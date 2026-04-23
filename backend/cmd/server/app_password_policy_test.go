package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestAppPasswordNormalizeFormat(t *testing.T) {
	formatted, compact, ok := normalizeAppPasswordInput("asd-k9p")
	if !ok {
		t.Fatal("expected lowercase app password to normalize")
	}
	if formatted != "ASD-K9P" {
		t.Fatalf("unexpected formatted key: %q", formatted)
	}
	if compact != "ASDK9P" {
		t.Fatalf("unexpected compact key: %q", compact)
	}

	if _, _, ok := normalizeAppPasswordInput("too-short"); ok {
		t.Fatal("expected invalid app password length to fail")
	}
}

func TestAppPasswordBindsOnceAndRejectsOtherDevice(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "steamdeck")

	uploadSave(t, h, "/saves", map[string]string{
		"app_password":   helperKey,
		"rom_sha1":       "bind-once-rom",
		"slotName":       "default",
		"system":         "snes",
		"device_type":    "linux-x86",
		"fingerprint":    "deck-1",
		"runtimeProfile": "snes/snes9x",
	}, "Chrono Trigger.srm", []byte("bind-once"))

	conflict := h.multipart("/saves", map[string]string{
		"app_password":   helperKey,
		"rom_sha1":       "bind-once-rom-2",
		"slotName":       "default",
		"system":         "snes",
		"device_type":    "linux-x86",
		"fingerprint":    "deck-2",
		"runtimeProfile": "snes/snes9x",
	}, "file", "EarthBound.srm", []byte("bind-once-2"))
	assertStatus(t, conflict, http.StatusConflict)
	body := decodeJSONMap(t, conflict.Body)
	if !strings.Contains(strings.ToLower(mustString(t, body["message"], "message")), "bound") {
		t.Fatalf("expected bound-device conflict message, got %s", prettyJSON(body))
	}
	devicesResp := h.request(http.MethodGet, "/devices", nil)
	assertStatus(t, devicesResp, http.StatusOK)
	devices := mustArray(t, decodeJSONMap(t, devicesResp.Body)["devices"], "devices")
	found := false
	for _, raw := range devices {
		d := mustObject(t, raw, "device")
		if mustString(t, d["fingerprint"], "fingerprint") != "deck-1" {
			continue
		}
		found = true
		if d["boundAppPasswordId"] == nil {
			t.Fatalf("expected boundAppPasswordId for device: %s", prettyJSON(d))
		}
		if !mustBool(t, d["syncAll"], "syncAll") {
			t.Fatalf("expected syncAll=true by default for new device: %s", prettyJSON(d))
		}
	}
	if !found {
		t.Fatalf("expected helper-bound device in list: %s", prettyJSON(devices))
	}
}

func TestHelperAutoEnrollmentWindowAllowsNoKeyProvisioning(t *testing.T) {
	h := newContractHarness(t)

	unauthorized := h.multipart("/saves", map[string]string{
		"rom_sha1":       "auto-enroll-rom-unauthorized",
		"slotName":       "default",
		"system":         "snes",
		"device_type":    "linux-x86",
		"fingerprint":    "deck-auto",
		"runtimeProfile": "snes/snes9x",
	}, "file", "Super Metroid.srm", []byte("auto-unauthorized"))
	assertStatus(t, unauthorized, http.StatusUnauthorized)

	enable := h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`))
	assertStatus(t, enable, http.StatusOK)
	enableBody := decodeJSONMap(t, enable.Body)
	if !mustBool(t, enableBody["active"], "active") {
		t.Fatalf("expected auto-enroll window to be active: %s", prettyJSON(enableBody))
	}

	authorized := h.multipart("/saves", map[string]string{
		"rom_sha1":       "auto-enroll-rom-ok",
		"slotName":       "default",
		"system":         "snes",
		"device_type":    "linux-x86",
		"fingerprint":    "deck-auto",
		"hostname":       "mister-01.example.invalid",
		"helper_name":    "RSM Helper",
		"helper_version": "1.4.0",
		"platform":       "MiSTer",
		"sync_paths":     "/media/fat/saves/SNES;/media/fat/saves/PSX",
		"systems":        "snes,psx",
		"runtimeProfile": "snes/snes9x",
	}, "file", "Final Fantasy VI.srm", []byte("auto-authorized"))
	assertStatus(t, authorized, http.StatusOK)
	if headerValue := authorized.Header().Get("X-RSM-Auto-App-Password"); headerValue == "" {
		t.Fatalf("expected X-RSM-Auto-App-Password header for auto-provisioned helper request")
	}

	deviceID := findDeviceIDByFingerprint(t, h, "deck-auto")
	deviceResp := h.request(http.MethodGet, fmt.Sprintf("/devices/%d", deviceID), nil)
	assertStatus(t, deviceResp, http.StatusOK)
	deviceBody := decodeJSONMap(t, deviceResp.Body)
	deviceObject := mustObject(t, deviceBody["device"], "device")
	if deviceObject["boundAppPasswordId"] == nil {
		t.Fatalf("expected auto-provisioned device to have bound app password: %s", prettyJSON(deviceObject))
	}
	if mustString(t, deviceObject["hostname"], "hostname") != "mister-01.example.invalid" {
		t.Fatalf("expected hostname metadata to persist: %s", prettyJSON(deviceObject))
	}
	if mustString(t, deviceObject["helperName"], "helperName") != "RSM Helper" {
		t.Fatalf("expected helperName metadata to persist: %s", prettyJSON(deviceObject))
	}
	if mustString(t, deviceObject["helperVersion"], "helperVersion") != "1.4.0" {
		t.Fatalf("expected helperVersion metadata to persist: %s", prettyJSON(deviceObject))
	}
	if mustString(t, deviceObject["platform"], "platform") != "MiSTer" {
		t.Fatalf("expected platform metadata to persist: %s", prettyJSON(deviceObject))
	}
	if mustString(t, deviceObject["lastSeenIp"], "lastSeenIp") == "" {
		t.Fatalf("expected lastSeenIp metadata to persist: %s", prettyJSON(deviceObject))
	}
	syncPaths := mustArray(t, deviceObject["syncPaths"], "syncPaths")
	if len(syncPaths) != 2 {
		t.Fatalf("expected syncPaths metadata to persist: %s", prettyJSON(deviceObject))
	}
	reportedSystems := mustArray(t, deviceObject["reportedSystemSlugs"], "reportedSystemSlugs")
	if len(reportedSystems) != 2 {
		t.Fatalf("expected reportedSystemSlugs metadata to persist: %s", prettyJSON(deviceObject))
	}

	appPasswordsResp := h.request(http.MethodGet, "/auth/app-passwords", nil)
	assertStatus(t, appPasswordsResp, http.StatusOK)
	appPasswordsBody := decodeJSONMap(t, appPasswordsResp.Body)
	appPasswords := mustArray(t, appPasswordsBody["appPasswords"], "appPasswords")
	foundBoundRecord := false
	for _, raw := range appPasswords {
		record := mustObject(t, raw, "appPassword")
		if record["boundDeviceId"] == nil {
			continue
		}
		if int(mustNumber(t, record["boundDeviceId"], "boundDeviceId")) == deviceID {
			foundBoundRecord = true
			break
		}
	}
	if !foundBoundRecord {
		t.Fatalf("expected one app password bound to device %d: %s", deviceID, prettyJSON(appPasswordsBody))
	}
}

func TestHelperPolicyEnforcedForUploadLatestAndDownload(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "policy-test")

	blockedBeforePolicy := uploadSave(t, h, "/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "policy-n64-rom",
		"slotName":     "default",
		"system":       "n64",
		"device_type":  "linux-x86",
		"fingerprint":  "deck-policy",
		"n64Profile":   n64ProfileMister,
	}, "policy-n64.eep", buildTestN64Payload("eep", "n64-before-policy"))
	blockedBeforePolicyID := mustString(t, mustObject(t, blockedBeforePolicy["save"], "save")["id"], "save.id")

	deviceID := findDeviceIDByFingerprint(t, h, "deck-policy")
	patchBody := `{"alias":"SteamDeck","syncAll":false,"allowedSystemSlugs":["snes"]}`
	patchResp := h.json(http.MethodPatch, fmt.Sprintf("/devices/%d", deviceID), strings.NewReader(patchBody))
	assertStatus(t, patchResp, http.StatusOK)

	blockedUpload := h.multipart("/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "policy-n64-rom-2",
		"slotName":     "default",
		"system":       "n64",
		"device_type":  "linux-x86",
		"fingerprint":  "deck-policy",
		"n64Profile":   n64ProfileMister,
	}, "file", "policy-n64-2.eep", buildTestN64Payload("eep", "n64-after-policy"))
	assertStatus(t, blockedUpload, http.StatusForbidden)

	allowedUpload := uploadSave(t, h, "/saves", map[string]string{
		"app_password":   helperKey,
		"rom_sha1":       "policy-snes-rom",
		"slotName":       "default",
		"system":         "snes",
		"device_type":    "linux-x86",
		"fingerprint":    "deck-policy",
		"runtimeProfile": "snes/snes9x",
	}, "Chrono Trigger.srm", []byte("snes-after-policy"))
	allowedID := mustString(t, mustObject(t, allowedUpload["save"], "save")["id"], "save.id")

	latestBlocked := helperGET(t, h, "/save/latest?romSha1=policy-n64-rom&slotName=default&device_type=linux-x86&fingerprint=deck-policy&n64Profile="+n64ProfileMister, helperKey)
	assertStatus(t, latestBlocked, http.StatusOK)
	latestBlockedBody := decodeJSONMap(t, latestBlocked.Body)
	if mustBool(t, latestBlockedBody["exists"], "exists") {
		t.Fatalf("expected disallowed latest lookup to be filtered: %s", prettyJSON(latestBlockedBody))
	}

	latestAllowed := helperGET(t, h, "/save/latest?romSha1=policy-snes-rom&slotName=default&device_type=linux-x86&fingerprint=deck-policy&runtimeProfile=snes/snes9x", helperKey)
	assertStatus(t, latestAllowed, http.StatusOK)
	latestAllowedBody := decodeJSONMap(t, latestAllowed.Body)
	if !mustBool(t, latestAllowedBody["exists"], "exists") {
		t.Fatalf("expected allowed latest lookup to succeed: %s", prettyJSON(latestAllowedBody))
	}

	blockedDownload := helperGET(t, h, "/saves/download?id="+blockedBeforePolicyID+"&device_type=linux-x86&fingerprint=deck-policy&n64Profile="+n64ProfileMister, helperKey)
	assertStatus(t, blockedDownload, http.StatusForbidden)

	allowedDownload := helperGET(t, h, "/saves/download?id="+allowedID+"&device_type=linux-x86&fingerprint=deck-policy&runtimeProfile=snes/snes9x", helperKey)
	assertStatus(t, allowedDownload, http.StatusOK)
	if got := allowedDownload.Header().Get("Content-Type"); !strings.Contains(got, "application/octet-stream") {
		t.Fatalf("expected binary download for allowed save, got content-type %q", got)
	}
}

func TestAuthTokenAppPasswordAutoProvisionRequiresWindow(t *testing.T) {
	h := newContractHarness(t)

	blocked := h.json(http.MethodPost, "/auth/token/app-password", strings.NewReader(`{"name":"SteamDeck","deviceType":"linux-x86","fingerprint":"deck-token"}`))
	assertStatus(t, blocked, http.StatusForbidden)

	enable := h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`))
	assertStatus(t, enable, http.StatusOK)

	allowed := h.json(http.MethodPost, "/auth/token/app-password", strings.NewReader(`{"name":"SteamDeck","deviceType":"linux-x86","fingerprint":"deck-token"}`))
	assertStatus(t, allowed, http.StatusOK)
	allowedBody := decodeJSONMap(t, allowed.Body)
	plainTextKey := mustString(t, allowedBody["plainTextKey"], "plainTextKey")
	if _, _, ok := normalizeAppPasswordInput(plainTextKey); !ok {
		t.Fatalf("expected auth/token/app-password to return key format XXX-XXX, got %q", plainTextKey)
	}
}

func TestAuthTokenAppPasswordPersistsHelperMetadata(t *testing.T) {
	h := newContractHarness(t)

	enable := h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`))
	assertStatus(t, enable, http.StatusOK)

	allowed := h.json(http.MethodPost, "/auth/token/app-password", strings.NewReader(`{
		"name":"Living Room MiSTer",
		"deviceType":"mister",
		"fingerprint":"mister-token",
		"hostname":"mister-02.example.invalid",
		"helperName":"RSM Helper",
		"helperVersion":"2.0.1",
		"platform":"MiSTer",
		"syncPaths":["/media/fat/saves/SNES","/media/fat/saves/PSX"],
		"systems":["snes","psx"]
	}`))
	assertStatus(t, allowed, http.StatusOK)

	deviceID := findDeviceIDByFingerprint(t, h, "mister-token")
	deviceResp := h.request(http.MethodGet, fmt.Sprintf("/devices/%d", deviceID), nil)
	assertStatus(t, deviceResp, http.StatusOK)
	deviceBody := decodeJSONMap(t, deviceResp.Body)
	deviceObject := mustObject(t, deviceBody["device"], "device")
	if mustString(t, deviceObject["hostname"], "hostname") != "mister-02.example.invalid" {
		t.Fatalf("expected hostname metadata on token auto-provisioned device: %s", prettyJSON(deviceObject))
	}
	if mustString(t, deviceObject["helperVersion"], "helperVersion") != "2.0.1" {
		t.Fatalf("expected helperVersion metadata on token auto-provisioned device: %s", prettyJSON(deviceObject))
	}
	if mustString(t, deviceObject["platform"], "platform") != "MiSTer" {
		t.Fatalf("expected platform metadata on token auto-provisioned device: %s", prettyJSON(deviceObject))
	}
	if len(mustArray(t, deviceObject["reportedSystemSlugs"], "reportedSystemSlugs")) != 2 {
		t.Fatalf("expected reported systems on token auto-provisioned device: %s", prettyJSON(deviceObject))
	}
}

func TestAppPasswordAndDevicePolicyPersistAcrossRestart(t *testing.T) {
	saveRoot := filepath.Join(t.TempDir(), "saves")
	stateRoot := filepath.Join(t.TempDir(), "state")

	h1 := newContractHarnessWithRoots(t, saveRoot, stateRoot)
	appPasswordID, helperKey := createHelperAppPasswordRecord(t, h1, "", "persisted-key")
	uploadSave(t, h1, "/saves", map[string]string{
		"app_password":   helperKey,
		"rom_sha1":       "persist-rom",
		"slotName":       "default",
		"system":         "snes",
		"device_type":    "linux-x86",
		"fingerprint":    "deck-persist",
		"runtimeProfile": "snes/snes9x",
	}, "Chrono Trigger.srm", []byte("persist"))
	deviceID := findDeviceIDByFingerprint(t, h1, "deck-persist")
	patchBody := `{"alias":"SteamDeck Persist","syncAll":false,"allowedSystemSlugs":["snes","gba"]}`
	patchResp := h1.json(http.MethodPatch, fmt.Sprintf("/devices/%d", deviceID), strings.NewReader(patchBody))
	assertStatus(t, patchResp, http.StatusOK)

	h2 := newContractHarnessWithRoots(t, saveRoot, stateRoot)
	listResp := h2.request(http.MethodGet, "/auth/app-passwords", nil)
	assertStatus(t, listResp, http.StatusOK)
	listBody := decodeJSONMap(t, listResp.Body)
	passwords := mustArray(t, listBody["appPasswords"], "appPasswords")
	persistedFound := false
	for _, raw := range passwords {
		record := mustObject(t, raw, "appPassword")
		if mustString(t, record["id"], "id") != appPasswordID {
			continue
		}
		persistedFound = true
		if mustString(t, record["name"], "name") != "persisted-key" {
			t.Fatalf("unexpected app password name after restart: %s", prettyJSON(record))
		}
		if mustBool(t, record["syncAll"], "syncAll") {
			t.Fatalf("expected syncAll=false after restart: %s", prettyJSON(record))
		}
		allowed := mustArray(t, record["allowedSystemSlugs"], "allowedSystemSlugs")
		gotAllowed := []string{}
		for _, item := range allowed {
			gotAllowed = append(gotAllowed, mustString(t, item, "allowedSystemSlugs[]"))
		}
		sort.Strings(gotAllowed)
		if strings.Join(gotAllowed, ",") != "gba,snes" {
			t.Fatalf("unexpected allowedSystemSlugs after restart: %v", gotAllowed)
		}
	}
	if !persistedFound {
		t.Fatalf("expected persisted app password %s after restart: %s", appPasswordID, prettyJSON(listBody))
	}

	rootDeviceGet := h2.request(http.MethodGet, fmt.Sprintf("/devices/%d", deviceID), nil)
	assertStatus(t, rootDeviceGet, http.StatusOK)
	v1DeviceGet := h2.request(http.MethodGet, fmt.Sprintf("/v1/devices/%d", deviceID), nil)
	assertStatus(t, v1DeviceGet, http.StatusOK)
	rootDeviceBody := normalizeForGolden(decodeJSONMap(t, rootDeviceGet.Body))
	v1DeviceBody := normalizeForGolden(decodeJSONMap(t, v1DeviceGet.Body))
	assertEqualJSONValue(t, rootDeviceBody, v1DeviceBody, "devices get alias parity")

	deleteResp := h2.request(http.MethodDelete, "/auth/app-passwords/"+appPasswordID, nil)
	assertStatus(t, deleteResp, http.StatusOK)

	h3 := newContractHarnessWithRoots(t, saveRoot, stateRoot)
	listAfterDelete := h3.request(http.MethodGet, "/auth/app-passwords", nil)
	assertStatus(t, listAfterDelete, http.StatusOK)
	listAfterDeleteBody := decodeJSONMap(t, listAfterDelete.Body)
	afterDeletePasswords := mustArray(t, listAfterDeleteBody["appPasswords"], "appPasswords")
	for _, raw := range afterDeletePasswords {
		record := mustObject(t, raw, "appPassword")
		if mustString(t, record["id"], "id") == appPasswordID {
			t.Fatalf("expected app password %s to be removed after restart", appPasswordID)
		}
	}
}

func newContractHarnessWithRoots(t *testing.T, saveRoot, stateRoot string) *contractHarness {
	t.Helper()
	t.Setenv("SAVE_ROOT", saveRoot)
	t.Setenv("STATE_ROOT", stateRoot)
	t.Setenv("BOOTSTRAP_DEMO_DATA", "true")

	app := newApp()
	if err := app.initSaveStore(); err != nil {
		t.Fatalf("init save store: %v", err)
	}

	return &contractHarness{
		t:       t,
		app:     app,
		handler: newRouter(app),
	}
}

func helperGET(t *testing.T, h *contractHarness, path, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("X-CSRF-Protection", "1")
	return h.do(req)
}

func findDeviceIDByFingerprint(t *testing.T, h *contractHarness, fingerprint string) int {
	t.Helper()
	resp := h.request(http.MethodGet, "/devices", nil)
	assertStatus(t, resp, http.StatusOK)
	devices := mustArray(t, decodeJSONMap(t, resp.Body)["devices"], "devices")
	for _, raw := range devices {
		d := mustObject(t, raw, "device")
		if mustString(t, d["fingerprint"], "fingerprint") == fingerprint {
			return int(mustNumber(t, d["id"], "id"))
		}
	}
	t.Fatalf("device with fingerprint %q not found", fingerprint)
	return 0
}
