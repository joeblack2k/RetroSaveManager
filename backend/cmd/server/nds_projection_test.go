package main

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestNDSRuntimeProfilesConvertDeSmuMERawAndNoGBA(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "nds-converters")
	raw := buildNDSFixturePayload(8 * 1024)
	dsv, err := buildDeSmuMEDSV(raw)
	if err != nil {
		t.Fatalf("build DeSmuME DSV fixture: %v", err)
	}

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password":   helperKey,
		"device_type":    "retroarch",
		"fingerprint":    "deck-nds",
		"system":         "nds",
		"slotName":       "default",
		"rom_sha1":       "nds-rom-1",
		"runtimeProfile": "nds/desmume",
		"displayTitle":   "Nintendo DS Converter Test",
	}, "Nintendo DS Converter Test.dsv", dsv)

	save := mustObject(t, body["save"], "save")
	saveID := mustString(t, save["id"], "save.id")
	if got := mustString(t, save["sha256"], "save.sha256"); got != payloadSHA256Hex(raw) {
		t.Fatalf("expected stored canonical raw payload SHA, got %s", prettyJSON(save))
	}

	list := h.request(http.MethodGet, "/saves?systemSlug=nds&limit=10", nil)
	assertStatus(t, list, http.StatusOK)
	item := mustObject(t, mustArray(t, decodeJSONMap(t, list.Body)["saves"], "saves")[0], "save")
	profiles := mustArray(t, item["downloadProfiles"], "downloadProfiles")
	for _, id := range []string{"nds/melonds", "nds/desmume", "nds/nogba", "nds/retroarch-melonds"} {
		if !downloadProfilePresent(t, profiles, id) {
			t.Fatalf("expected %s profile in NDS downloads: %s", id, prettyJSON(item))
		}
	}

	rawDownload := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape("nds/melonds"), nil)
	assertStatus(t, rawDownload, http.StatusOK)
	if got := rawDownload.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="Nintendo DS Converter Test.sav"`) {
		t.Fatalf("expected melonDS .sav filename, got %q", got)
	}
	if !bytes.Equal(rawDownload.Body.Bytes(), raw) {
		t.Fatal("expected melonDS projection to be canonical raw NDS save")
	}

	dsvDownload := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape("nds/desmume"), nil)
	assertStatus(t, dsvDownload, http.StatusOK)
	dsvRoundTrip, err := splitDeSmuMEDSV(dsvDownload.Body.Bytes())
	if err != nil {
		t.Fatalf("downloaded DSV should parse: %v", err)
	}
	if !bytes.Equal(dsvRoundTrip, raw) {
		t.Fatal("expected DeSmuME projection to round-trip canonical raw payload")
	}

	noGBADownload := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape("nds/nogba"), nil)
	assertStatus(t, noGBADownload, http.StatusOK)
	noGBARoundTrip, ok, err := splitNoGBANDSContainer(noGBADownload.Body.Bytes())
	if err != nil || !ok {
		t.Fatalf("downloaded No$GBA save should parse: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(noGBARoundTrip, raw) {
		t.Fatal("expected No$GBA projection to round-trip canonical raw payload")
	}

	latest := h.request(http.MethodGet, "/save/latest?romSha1=nds-rom-1&slotName=default&runtimeProfile="+url.QueryEscape("nds/desmume"), nil)
	assertStatus(t, latest, http.StatusOK)
	latestBody := decodeJSONMap(t, latest.Body)
	if mustString(t, latestBody["sha256"], "latest.sha256") != payloadSHA256Hex(dsvDownload.Body.Bytes()) {
		t.Fatalf("expected latest to compare projected DeSmuME DSV bytes: %s", prettyJSON(latestBody))
	}
}

func TestGameBoyRuntimeProfilesUseRawIdentityProjection(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "gameboy-converters")
	raw := buildNDSFixturePayload(8 * 1024)

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password":   helperKey,
		"device_type":    "retroarch",
		"fingerprint":    "deck-gameboy",
		"system":         "gameboy",
		"slotName":       "default",
		"rom_sha1":       "gb-rom-1",
		"runtimeProfile": "gameboy/retroarch-gambatte",
		"displayTitle":   "Game Boy Converter Test",
	}, "Game Boy Converter Test.srm", raw)

	saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
	list := h.request(http.MethodGet, "/saves?systemSlug=gameboy&limit=10", nil)
	assertStatus(t, list, http.StatusOK)
	item := mustObject(t, mustArray(t, decodeJSONMap(t, list.Body)["saves"], "saves")[0], "save")
	profiles := mustArray(t, item["downloadProfiles"], "downloadProfiles")
	for _, id := range []string{"gameboy/gambatte", "gameboy/sameboy", "gameboy/bgb", "gameboy/retroarch-gambatte"} {
		if !downloadProfilePresent(t, profiles, id) {
			t.Fatalf("expected %s profile in Game Boy downloads: %s", id, prettyJSON(item))
		}
	}

	bgb := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape("gameboy/bgb"), nil)
	assertStatus(t, bgb, http.StatusOK)
	if got := bgb.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="Game Boy Converter Test.sav"`) {
		t.Fatalf("expected BGB .sav filename, got %q", got)
	}
	if !bytes.Equal(bgb.Body.Bytes(), raw) {
		t.Fatal("expected Game Boy BGB projection to preserve raw SRAM")
	}

	retroArch := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape("gameboy/retroarch-gambatte"), nil)
	assertStatus(t, retroArch, http.StatusOK)
	if got := retroArch.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="Game Boy Converter Test.srm"`) {
		t.Fatalf("expected RetroArch .srm filename, got %q", got)
	}
	if !bytes.Equal(retroArch.Body.Bytes(), raw) {
		t.Fatal("expected Game Boy RetroArch projection to preserve raw SRAM")
	}
}

func buildNDSFixturePayload(size int) []byte {
	payload := bytes.Repeat([]byte{0xFF}, size)
	copy(payload[:], []byte("RSM-NDS-CONVERTER-FIXTURE"))
	for i := 64; i < len(payload); i += 257 {
		payload[i] = byte(i % 251)
	}
	return payload
}
