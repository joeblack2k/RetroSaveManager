package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
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

func TestHelperConfigReportScopesMisterAndBlocksWii(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "mister-config")

	report := h.json(http.MethodPost, "/devices/config/report", strings.NewReader(`{
		"deviceType":"mister",
		"fingerprint":"mister-config-1",
		"appPassword":"`+helperKey+`",
		"hostname":"mister.example.invalid",
		"helperName":"RSM MiSTer Helper",
		"helperVersion":"3.0.0",
		"platform":"MiSTer FPGA",
		"configRevision":"sha256:example",
		"sources":[
			{
				"id":"mister_default",
				"label":"MiSTer Default",
				"kind":"mister-fpga",
				"profile":"mister",
				"savePath":"/media/fat/saves",
				"romPath":"/media/fat/games",
				"recursive":true,
				"systems":["nes","snes","n64","wii","gbc","psx"],
				"managed":false,
				"origin":"manual"
			}
		]
	}`))
	assertStatus(t, report, http.StatusOK)
	body := decodeJSONMap(t, report.Body)
	deviceObject := mustObject(t, body["device"], "device")
	policy := mustObject(t, deviceObject["effectivePolicy"], "device.effectivePolicy")
	allowed := mustArray(t, policy["allowedSystemSlugs"], "effectivePolicy.allowedSystemSlugs")
	allowedSlugs := make([]string, 0, len(allowed))
	for _, raw := range allowed {
		allowedSlugs = append(allowedSlugs, mustString(t, raw, "allowedSystemSlugs[]"))
	}
	sort.Strings(allowedSlugs)
	if strings.Join(allowedSlugs, ",") != "gameboy,n64,nes,psx,snes" {
		t.Fatalf("unexpected effective MiSTer allowed systems: %v", allowedSlugs)
	}
	blocked := mustArray(t, policy["blocked"], "effectivePolicy.blocked")
	blockedReasons := map[string]string{}
	for _, raw := range blocked {
		item := mustObject(t, raw, "blocked[]")
		blockedReasons[mustString(t, item["system"], "blocked.system")] = mustString(t, item["reason"], "blocked.reason")
	}
	if !strings.Contains(blockedReasons["wii"], "not supported") {
		t.Fatalf("expected Wii to be blocked by MiSTer capability, got %s", prettyJSON(blockedReasons))
	}
	if _, ok := blockedReasons["gbc"]; ok {
		t.Fatalf("expected GBC alias to normalize to gameboy instead of being blocked, got %s", prettyJSON(blockedReasons))
	}

	blockedUpload := h.multipart("/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "mister-config-wii-rom",
		"slotName":     "default",
		"system":       "wii",
		"device_type":  "mister",
		"fingerprint":  "mister-config-1",
	}, "file", "data.bin", buildWiiDataBinFixture())
	assertStatus(t, blockedUpload, http.StatusForbidden)

	allowedUpload := h.multipart("/saves", map[string]string{
		"app_password":   helperKey,
		"rom_sha1":       "mister-config-snes-rom",
		"slotName":       "default",
		"system":         "snes",
		"device_type":    "mister",
		"fingerprint":    "mister-config-1",
		"runtimeProfile": "snes/snes9x",
	}, "file", "Super Metroid.srm", []byte("mister-config-snes"))
	assertStatus(t, allowedUpload, http.StatusOK)
}

func TestHelperConfigSyncMatchesHelperContract(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "helper-config-sync")

	payload := `{
		"schemaVersion":1,
		"helper":{
			"name":"sgm-mister-helper",
			"version":"0.4.12",
			"deviceType":"mister",
			"defaultKind":"mister-fpga",
			"hostname":"mister-contract.example.invalid",
			"platform":"linux",
			"arch":"arm",
			"configPath":"/media/fat/1retro/config.ini",
			"binaryDir":"/media/fat/1retro"
		},
		"config":{
			"url":"rsm.example.invalid",
			"port":80,
			"baseUrl":"http://rsm.example.invalid:80",
			"appPasswordConfigured":true,
			"root":"/media/fat",
			"stateDir":"./state",
			"watch":false,
			"watchInterval":30,
			"forceUpload":false,
			"dryRun":false,
			"routePrefix":"",
			"sources":[
				{
					"id":"mister_default",
					"label":"MiSTer Default",
					"kind":"mister-fpga",
					"profile":"mister",
					"savePaths":["/media/fat/saves"],
					"romPaths":["/media/fat/games"],
					"recursive":true,
					"systems":["snes","n64","wii","psx","sega-cd","sega-32x"],
					"createMissingSystemDirs":false,
					"managed":false,
					"origin":"manual"
				}
			]
		},
		"capabilities":{
			"policy":{"supportsSystemsAllowList":true}
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/helpers/config/sync", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Protection", "1")
	req.Header.Set("X-RSM-App-Password", helperKey)
	resp := h.do(req)
	assertStatus(t, resp, http.StatusOK)
	body := decodeJSONMap(t, resp.Body)
	if !mustBool(t, body["accepted"], "accepted") {
		t.Fatalf("expected accepted helper config response: %s", prettyJSON(body))
	}
	policy := mustObject(t, body["policy"], "policy")
	sources := mustArray(t, policy["sources"], "policy.sources")
	if len(sources) != 1 {
		t.Fatalf("expected one source policy: %s", prettyJSON(body))
	}
	sourcePolicy := mustObject(t, sources[0], "policy.sources[0]")
	systemsRaw := mustArray(t, sourcePolicy["systems"], "policy.sources[0].systems")
	systems := make([]string, 0, len(systemsRaw))
	for _, raw := range systemsRaw {
		systems = append(systems, mustString(t, raw, "system"))
	}
	sort.Strings(systems)
	if strings.Join(systems, ",") != "n64,psx,sega-32x,sega-cd,snes" {
		t.Fatalf("expected helper runtime policy to remove Wii, got %v", systems)
	}
	if mustBool(t, sourcePolicy["createMissingSystemDirs"], "createMissingSystemDirs") {
		t.Fatalf("expected createMissingSystemDirs=false to round-trip: %s", prettyJSON(sourcePolicy))
	}
	globalPolicy := mustObject(t, policy["global"], "policy.global")
	if mustBool(t, globalPolicy["forceUpload"], "policy.global.forceUpload") {
		t.Fatalf("expected forceUpload=false to round-trip: %s", prettyJSON(globalPolicy))
	}
	if mustBool(t, globalPolicy["dryRun"], "policy.global.dryRun") {
		t.Fatalf("expected dryRun=false to round-trip: %s", prettyJSON(globalPolicy))
	}
	if mustString(t, globalPolicy["url"], "policy.global.url") != "rsm.example.invalid" {
		t.Fatalf("expected URL to round-trip for helper writeback: %s", prettyJSON(globalPolicy))
	}
	if mustString(t, globalPolicy["root"], "policy.global.root") != "/media/fat" {
		t.Fatalf("expected root to round-trip for helper writeback: %s", prettyJSON(globalPolicy))
	}

	deviceID := findDeviceIDByFingerprint(t, h, "mister-contract.example.invalid")
	deviceResp := h.request(http.MethodGet, fmt.Sprintf("/devices/%d", deviceID), nil)
	assertStatus(t, deviceResp, http.StatusOK)
	device := mustObject(t, decodeJSONMap(t, deviceResp.Body)["device"], "device")
	if mustString(t, device["helperName"], "helperName") != "sgm-mister-helper" {
		t.Fatalf("expected helper metadata to persist: %s", prettyJSON(device))
	}
	configGlobal := mustObject(t, device["configGlobal"], "device.configGlobal")
	if mustString(t, configGlobal["root"], "configGlobal.root") != "/media/fat" {
		t.Fatalf("expected global root to persist: %s", prettyJSON(configGlobal))
	}
	syncPaths := mustArray(t, device["syncPaths"], "device.syncPaths")
	syncPathValues := make([]string, 0, len(syncPaths))
	for _, raw := range syncPaths {
		syncPathValues = append(syncPathValues, mustString(t, raw, "syncPaths[]"))
	}
	sort.Strings(syncPathValues)
	if strings.Join(syncPathValues, ",") != "/media/fat/games,/media/fat/saves" {
		t.Fatalf("expected sync paths to contain source save/ROM paths only, got %v", syncPathValues)
	}
}

func TestHelperHeartbeatStoresServiceSensorsAndConfig(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "helper-heartbeat")

	payload := `{
		"schemaVersion":1,
		"helper":{
			"name":"sgm-mister-helper",
			"version":"0.4.13",
			"deviceType":"mister",
			"defaultKind":"mister-fpga",
			"hostname":"mister-heartbeat.example.invalid",
			"platform":"linux",
			"arch":"arm",
			"pid":1234,
			"startedAt":"2026-04-25T12:00:00Z",
			"uptimeSeconds":61,
			"binaryPath":"/media/fat/1retro/sgm-mister-helper",
			"binaryDir":"/media/fat/1retro",
			"configPath":"/media/fat/1retro/config.ini",
			"stateDir":"/media/fat/1retro/state"
		},
		"service":{
			"mode":"daemon",
			"status":"idle",
			"loop":"sse-plus-periodic-reconcile",
			"heartbeatInterval":30,
			"reconcileInterval":1800,
			"controlChannel":"GET /events",
			"lastSyncStartedAt":"2026-04-25T12:00:01Z",
			"lastSyncFinishedAt":"2026-04-25T12:00:04Z",
			"lastSyncOk":true,
			"lastError":null,
			"lastEvent":"startup",
			"syncCycles":1
		},
		"sensors":{
			"online":true,
			"authenticated":true,
			"configHash":"sha256-example",
			"configReadable":true,
			"configError":null,
			"sourceCount":1,
			"savePathCount":1,
			"romPathCount":1,
			"configuredSystems":["snes","n64","psx"],
			"supportedSystems":["nes","snes","gameboy","gba","n64","genesis","master-system","game-gear","sega-cd","sega-32x","saturn","neogeo","psx"],
			"syncLockPresent":false,
			"lastSync":{"scanned":24,"uploaded":1,"downloaded":0,"inSync":23,"conflicts":0,"skipped":0,"errors":0}
		},
		"config":{
			"url":"rsm.example.invalid",
			"port":80,
			"baseUrl":"http://rsm.example.invalid:80",
			"appPasswordConfigured":true,
			"root":"/media/fat",
			"stateDir":"./state",
			"watch":false,
			"watchInterval":30,
			"forceUpload":false,
			"dryRun":false,
			"routePrefix":"",
			"sources":[
				{"id":"mister_default","label":"MiSTer Default","kind":"mister-fpga","profile":"mister","savePaths":["/media/fat/saves"],"romPaths":["/media/fat/games"],"recursive":true,"systems":["snes","n64","psx"],"createMissingSystemDirs":false,"managed":false,"origin":"manual"}
			]
		},
		"capabilities":{"serviceRun":true,"serviceInstall":true,"backendPolicyWins":true}
	}`

	req := httptest.NewRequest(http.MethodPost, "/helpers/heartbeat", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Protection", "1")
	req.Header.Set("X-RSM-App-Password", helperKey)
	resp := h.do(req)
	assertStatus(t, resp, http.StatusOK)
	body := decodeJSONMap(t, resp.Body)
	if !mustBool(t, body["accepted"], "accepted") {
		t.Fatalf("expected heartbeat accepted: %s", prettyJSON(body))
	}
	device := mustObject(t, body["device"], "device")
	service := mustObject(t, device["service"], "device.service")
	if mustString(t, service["mode"], "service.mode") != "daemon" {
		t.Fatalf("expected daemon service state: %s", prettyJSON(service))
	}
	if mustString(t, service["freshness"], "service.freshness") != "online" {
		t.Fatalf("expected fresh heartbeat to be online: %s", prettyJSON(service))
	}
	if !mustBool(t, service["online"], "service.online") {
		t.Fatalf("expected service.online=true: %s", prettyJSON(service))
	}
	sensors := mustObject(t, device["sensors"], "device.sensors")
	if mustString(t, sensors["configHash"], "sensors.configHash") != "sha256-example" {
		t.Fatalf("expected config hash sensor: %s", prettyJSON(sensors))
	}
	lastSync := mustObject(t, sensors["lastSync"], "sensors.lastSync")
	if mustNumber(t, lastSync["uploaded"], "lastSync.uploaded") != 1 {
		t.Fatalf("expected last sync counters: %s", prettyJSON(lastSync))
	}
}

func TestBackendManagedSourcesSurviveHelperConfigSync(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "source-merge")
	report := h.json(http.MethodPost, "/devices/config/report", strings.NewReader(`{
		"deviceType":"mister",
		"fingerprint":"source-merge-device",
		"appPassword":"`+helperKey+`",
		"sources":[
			{"id":"mister_default","label":"MiSTer Default","kind":"mister-fpga","profile":"mister","savePath":"/media/fat/saves","romPath":"/media/fat/games","systems":["n64"],"origin":"manual"}
		]
	}`))
	assertStatus(t, report, http.StatusOK)
	deviceID := findDeviceIDByFingerprint(t, h, "source-merge-device")

	patch := h.json(http.MethodPatch, fmt.Sprintf("/devices/%d", deviceID), strings.NewReader(`{
		"configSources":[
			{"id":"mister_default","label":"MiSTer Default","kind":"mister-fpga","profile":"mister","savePaths":["/media/fat/saves"],"romPaths":["/media/fat/games"],"recursive":true,"systems":["n64"],"createMissingSystemDirs":false},
			{"id":"backend-snes-snes9x","label":"Super Nintendo Snes9x","kind":"custom","profile":"snes9x","savePaths":["/media/snes9x/saves"],"romPaths":["/media/snes9x/roms"],"recursive":true,"systems":["snes"],"createMissingSystemDirs":false}
		]
	}`))
	assertStatus(t, patch, http.StatusOK)

	payload := `{
		"schemaVersion":1,
		"helper":{"name":"sgm-mister-helper","version":"0.4.13","deviceType":"mister","defaultKind":"mister-fpga","hostname":"source-merge-device","platform":"linux","arch":"arm","configPath":"/media/fat/1retro/config.ini","binaryDir":"/media/fat/1retro"},
		"config":{"appPasswordConfigured":true,"sources":[{"id":"mister_default","label":"MiSTer Default","kind":"mister-fpga","profile":"mister","savePaths":["/media/fat/saves"],"romPaths":["/media/fat/games"],"recursive":true,"systems":["n64"],"createMissingSystemDirs":false,"managed":false,"origin":"manual"}]}
	}`
	req := httptest.NewRequest(http.MethodPost, "/helpers/config/sync", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Protection", "1")
	req.Header.Set("X-RSM-App-Password", helperKey)
	resp := h.do(req)
	assertStatus(t, resp, http.StatusOK)
	body := decodeJSONMap(t, resp.Body)
	sources := mustArray(t, mustObject(t, body["policy"], "policy")["sources"], "policy.sources")
	foundBackendSNES := false
	for _, raw := range sources {
		source := mustObject(t, raw, "policy.sources[]")
		if mustString(t, source["id"], "source.id") != "backend-snes-snes9x" {
			continue
		}
		foundBackendSNES = true
		systems := mustArray(t, source["systems"], "source.systems")
		if len(systems) != 1 || mustString(t, systems[0], "source.systems[0]") != "snes" {
			t.Fatalf("expected backend SNES source to remain in runtime policy: %s", prettyJSON(source))
		}
	}
	if !foundBackendSNES {
		t.Fatalf("expected backend-managed SNES source to survive helper config sync: %s", prettyJSON(body))
	}
}

func TestDeviceCommandPublishesHelperEvent(t *testing.T) {
	h := newContractHarness(t)
	subscriberID, events := h.app.subscribeEvents()
	defer h.app.unsubscribeEvents(subscriberID)

	resp := h.json(http.MethodPost, "/devices/1/command", strings.NewReader(`{"command":"scan","reason":"test"}`))
	assertStatus(t, resp, http.StatusOK)
	body := decodeJSONMap(t, resp.Body)
	if mustString(t, body["event"], "event") != "scan.requested" {
		t.Fatalf("unexpected command response: %s", prettyJSON(body))
	}

	select {
	case event := <-events:
		if event.Type != "scan.requested" {
			t.Fatalf("unexpected event type %q", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("expected scan.requested event")
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

func TestHelperConfigReportPersistsAcrossRestart(t *testing.T) {
	saveRoot := filepath.Join(t.TempDir(), "saves")
	stateRoot := filepath.Join(t.TempDir(), "state")

	h1 := newContractHarnessWithRoots(t, saveRoot, stateRoot)
	_, helperKey := createHelperAppPasswordRecord(t, h1, "", "persisted-config")
	report := h1.json(http.MethodPost, "/devices/config/report", strings.NewReader(`{
		"deviceType":"mister",
		"fingerprint":"mister-persist-config",
		"appPassword":"`+helperKey+`",
		"configRevision":"sha256:persisted-example",
		"sources":[
			{
				"id":"mister_default",
				"label":"MiSTer Default",
				"kind":"mister-fpga",
				"profile":"mister",
				"savePath":"/media/fat/saves",
				"romPath":"/media/fat/games",
				"systems":["snes","wii"],
				"origin":"manual"
			}
		]
	}`))
	assertStatus(t, report, http.StatusOK)
	deviceID := findDeviceIDByFingerprint(t, h1, "mister-persist-config")

	h2 := newContractHarnessWithRoots(t, saveRoot, stateRoot)
	deviceResp := h2.request(http.MethodGet, fmt.Sprintf("/devices/%d", deviceID), nil)
	assertStatus(t, deviceResp, http.StatusOK)
	deviceObject := mustObject(t, decodeJSONMap(t, deviceResp.Body)["device"], "device")
	if mustString(t, deviceObject["configRevision"], "configRevision") != "sha256:persisted-example" {
		t.Fatalf("expected config revision after restart: %s", prettyJSON(deviceObject))
	}
	sources := mustArray(t, deviceObject["configSources"], "configSources")
	if len(sources) != 1 {
		t.Fatalf("expected one config source after restart: %s", prettyJSON(deviceObject))
	}
	policy := mustObject(t, deviceObject["effectivePolicy"], "effectivePolicy")
	blocked := mustArray(t, policy["blocked"], "effectivePolicy.blocked")
	foundWiiBlock := false
	for _, raw := range blocked {
		item := mustObject(t, raw, "blocked[]")
		if mustString(t, item["system"], "blocked.system") == "wii" {
			foundWiiBlock = true
			break
		}
	}
	if !foundWiiBlock {
		t.Fatalf("expected persisted MiSTer policy to continue blocking Wii: %s", prettyJSON(policy))
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
