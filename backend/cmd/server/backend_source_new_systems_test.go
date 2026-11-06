package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// syncWithDefaults posts a helper config sync whose capabilities give the
// mister-fpga kind these default systems, and returns the mister_default
// source's systems from the policy the backend sends back.
func syncWithDefaults(t *testing.T, h *contractHarness, helperKey string, defaults []string) []string {
	t.Helper()
	quoted := make([]string, len(defaults))
	for i, slug := range defaults {
		quoted[i] = `"` + slug + `"`
	}
	payload := `{
		"schemaVersion":1,
		"helper":{"name":"sgm-mister-helper","version":"0.4.16","deviceType":"mister","defaultKind":"mister-fpga","hostname":"new-systems-device","platform":"linux","arch":"arm"},
		"capabilities":{"sourceKinds":[{"kind":"mister-fpga","deviceType":"mister","defaultProfile":"mister","defaultSystems":[` + strings.Join(quoted, ",") + `]}]},
		"config":{"appPasswordConfigured":true,"sources":[{"id":"mister_default","label":"MiSTer Default","kind":"mister-fpga","profile":"mister","savePaths":["/media/fat/saves"],"romPaths":["/media/fat/games"],"recursive":true,"systems":["nes","snes"],"origin":"backend"}]}
	}`
	req := httptest.NewRequest(http.MethodPost, "/helpers/config/sync", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Protection", "1")
	req.Header.Set("X-RSM-App-Password", helperKey)
	resp := h.do(req)
	assertStatus(t, resp, http.StatusOK)
	body := decodeJSONMap(t, resp.Body)
	for _, raw := range mustArray(t, mustObject(t, body["policy"], "policy")["sources"], "policy.sources") {
		source := mustObject(t, raw, "policy.sources[]")
		if mustString(t, source["id"], "source.id") != "mister_default" {
			continue
		}
		var out []string
		for _, s := range mustArray(t, source["systems"], "source.systems") {
			out = append(out, mustString(t, s, "system"))
		}
		return out
	}
	t.Fatalf("no mister_default source in policy: %s", prettyJSON(body))
	return nil
}

func setupBackendMisterSource(t *testing.T, fingerprint string) (*contractHarness, string) {
	t.Helper()
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", fingerprint)
	report := h.json(http.MethodPost, "/devices/config/report", strings.NewReader(`{
		"deviceType":"mister","fingerprint":"new-systems-device","appPassword":"`+helperKey+`",
		"sources":[{"id":"mister_default","label":"MiSTer Default","kind":"mister-fpga","profile":"mister","savePath":"/media/fat/saves","romPath":"/media/fat/games","systems":["nes","snes"],"origin":"manual"}]
	}`))
	assertStatus(t, report, http.StatusOK)
	deviceID := findDeviceIDByFingerprint(t, h, "new-systems-device")
	patch := h.json(http.MethodPatch, fmt.Sprintf("/devices/%d", deviceID), strings.NewReader(`{
		"configSources":[{"id":"mister_default","label":"MiSTer Default","kind":"mister-fpga","profile":"mister","savePaths":["/media/fat/saves"],"romPaths":["/media/fat/games"],"recursive":true,"systems":["nes","snes"]}]
	}`))
	assertStatus(t, patch, http.StatusOK)
	return h, helperKey
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// A system a later helper build defaults to reaches a device whose source the
// backend already holds, with no edit to that device's config.
func TestBackendSourceGainsASystemANewHelperDefaultsTo(t *testing.T) {
	h, key := setupBackendMisterSource(t, "new-systems")
	first := syncWithDefaults(t, h, key, []string{"nes", "snes"})
	if contains(first, "wonderswan") {
		t.Fatalf("wonderswan before any helper defaulted to it: %v", first)
	}
	second := syncWithDefaults(t, h, key, []string{"nes", "snes", "wonderswan", "atari-lynx"})
	for _, want := range []string{"nes", "snes", "wonderswan", "atari-lynx"} {
		if !contains(second, want) {
			t.Fatalf("%s did not reach the backend source after the helper defaulted to it: %v", want, second)
		}
	}
}

// A system the helper defaulted to before, and that is no longer in the
// source, was taken out on purpose: a later sync doesn't put it back.
func TestBackendSourceKeepsARemovedDefaultOut(t *testing.T) {
	h, key := setupBackendMisterSource(t, "removed-default")
	syncWithDefaults(t, h, key, []string{"nes", "snes"})
	syncWithDefaults(t, h, key, []string{"nes", "snes", "wonderswan"})
	deviceID := findDeviceIDByFingerprint(t, h, "new-systems-device")
	patch := h.json(http.MethodPatch, fmt.Sprintf("/devices/%d", deviceID), strings.NewReader(`{
		"configSources":[{"id":"mister_default","label":"MiSTer Default","kind":"mister-fpga","profile":"mister","savePaths":["/media/fat/saves"],"romPaths":["/media/fat/games"],"recursive":true,"systems":["nes","snes"],"knownDefaultSystems":["nes","snes","wonderswan"]}]
	}`))
	assertStatus(t, patch, http.StatusOK)
	after := syncWithDefaults(t, h, key, []string{"nes", "snes", "wonderswan"})
	if contains(after, "wonderswan") {
		t.Fatalf("a removed default came back: %v", after)
	}
}

// A default the device kind may not sync stays out (the MiSTer has no PS2 core).
func TestBackendSourceGainsOnlySystemsTheDeviceKindAllows(t *testing.T) {
	h, key := setupBackendMisterSource(t, "kind-filter")
	syncWithDefaults(t, h, key, []string{"nes", "snes"})
	after := syncWithDefaults(t, h, key, []string{"nes", "snes", "ps2"})
	if contains(after, "ps2") {
		t.Fatalf("ps2 reached a MiSTer source: %v", after)
	}
}
