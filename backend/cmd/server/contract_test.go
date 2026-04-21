package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestContractAliasParityForStableEndpoints(t *testing.T) {
	h := newContractHarness(t)

	cases := []struct {
		name string
		path string
	}{
		{name: "auth me", path: "/auth/me"},
		{name: "save latest missing", path: "/save/latest?romSha1=missing-rom"},
		{name: "saves list envelope", path: "/saves?limit=2&offset=0"},
		{name: "devices list", path: "/devices"},
		{name: "conflicts check missing", path: "/conflicts/check?romSha1=missing-rom&slotName=default"},
		{name: "rom lookup", path: "/rom/lookup?filenameStem=Wario%20Land%20II"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rootResp := h.request(http.MethodGet, tc.path, nil)
			v1Resp := h.request(http.MethodGet, "/v1"+tc.path, nil)

			assertStatus(t, rootResp, http.StatusOK)
			assertStatus(t, v1Resp, http.StatusOK)
			if rootResp.Header().Get("Content-Type") != v1Resp.Header().Get("Content-Type") {
				t.Fatalf("content-type mismatch: root=%q v1=%q", rootResp.Header().Get("Content-Type"), v1Resp.Header().Get("Content-Type"))
			}

			rootBody := normalizeForGolden(decodeJSONMap(t, rootResp.Body))
			v1Body := normalizeForGolden(decodeJSONMap(t, v1Resp.Body))
			assertEqualJSONValue(t, rootBody, v1Body, tc.path)
		})
	}
}

func TestContractAuthMeNoAuthShape(t *testing.T) {
	h := newContractHarness(t)

	rr := h.request(http.MethodGet, "/auth/me", nil)
	assertStatus(t, rr, http.StatusOK)
	assertJSONContentType(t, rr)

	body := decodeJSONMap(t, rr.Body)
	if !mustBool(t, body["success"], "success") {
		t.Fatalf("expected success=true")
	}
	if mustString(t, body["message"], "message") != "Authenticated" {
		t.Fatalf("unexpected auth/me message: %s", rr.Body.String())
	}

	user := mustObject(t, body["user"], "user")
	if mustString(t, user["id"], "user.id") != internalPrincipalID() {
		t.Fatalf("unexpected user id: %s", prettyJSON(user))
	}
	if user["storageUsedBytes"] == nil || user["gameCount"] == nil || user["fileCount"] == nil {
		t.Fatalf("auth/me missing quota counts: %s", prettyJSON(body))
	}
	quota := mustObject(t, user["quota"], "user.quota")
	storage := mustObject(t, quota["storage"], "user.quota.storage")
	devices := mustObject(t, quota["devices"], "user.quota.devices")
	if mustString(t, storage["status"], "quota.storage.status") != "ok" {
		t.Fatalf("unexpected storage quota status: %s", prettyJSON(quota))
	}
	if mustString(t, devices["status"], "quota.devices.status") != "ok" {
		t.Fatalf("unexpected devices quota status: %s", prettyJSON(quota))
	}
}

func TestContractSaveLatestMissingAndSuccessShape(t *testing.T) {
	h := newContractHarness(t)

	missing := h.request(http.MethodGet, "/save/latest?romSha1=missing-rom&slotName=default", nil)
	assertStatus(t, missing, http.StatusOK)
	missingBody := decodeJSONMap(t, missing.Body)
	if !mustBool(t, missingBody["success"], "success") {
		t.Fatalf("expected success=true")
	}
	if mustBool(t, missingBody["exists"], "exists") {
		t.Fatalf("expected missing save latest result")
	}
	for _, key := range []string{"sha256", "version", "id"} {
		if missingBody[key] != nil {
			t.Fatalf("expected %s to be null, got %v", key, missingBody[key])
		}
	}

	uploadSave(t, h, "/saves", map[string]string{"rom_sha1": "rom-success", "slotName": "default"}, "slot1.srm", []byte("save-latest-success"))
	success := h.request(http.MethodGet, "/save/latest?romSha1=rom-success&slotName=default", nil)
	assertStatus(t, success, http.StatusOK)
	successBody := decodeJSONMap(t, success.Body)
	if !mustBool(t, successBody["success"], "success") || !mustBool(t, successBody["exists"], "exists") {
		t.Fatalf("unexpected success payload: %s", prettyJSON(successBody))
	}
	if mustString(t, successBody["sha256"], "sha256") == "" {
		t.Fatalf("expected sha256 on success payload")
	}
	if mustNumber(t, successBody["version"], "version") < 1 {
		t.Fatalf("expected positive version")
	}
	if mustString(t, successBody["id"], "id") == "" {
		t.Fatalf("expected non-empty save id")
	}
}

func TestContractSavesMultipartSuccessAndMissingFileFailure(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "contract-helper")

	okBody := uploadSave(t, h, "/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":    "upload-rom",
		"device_type": "web",
		"fingerprint": "abcdef12",
	}, "Wario Land II.srm", []byte("multipart-upload"))
	save := mustObject(t, okBody["save"], "save")
	if _, ok := save["id"]; !ok {
		t.Fatalf("expected save.id key")
	}
	if _, ok := save["sha256"]; !ok {
		t.Fatalf("expected save.sha256 key")
	}

	missing := h.multipart("/saves", map[string]string{"rom_sha1": "upload-rom"}, "", "", nil)
	assertStatus(t, missing, http.StatusBadRequest)
	missingBody := decodeJSONMap(t, missing.Body)
	if mustString(t, missingBody["message"], "message") != "File is required" {
		t.Fatalf("unexpected missing-file message: %s", prettyJSON(missingBody))
	}
}

func TestContractSavesMultipartRejectsUnknownSystemNoise(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "noise-helper")

	rejected := h.multipart("/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":    "noise-rom",
		"device_type": "linux-x86",
		"fingerprint": "noise-device",
	}, "notes.txt", []byte("this is plain text and not a valid save"))
	assertStatus(t, rejected, http.StatusUnprocessableEntity)
	assertJSONContentType(t, rejected)

	body := decodeJSONMap(t, rejected.Body)
	message := mustString(t, body["message"], "message")
	if !strings.Contains(strings.ToLower(message), "known consoles") {
		t.Fatalf("expected unsupported-format message, got %q", message)
	}
}

func TestContractSaveHistoryAndRollbackPromoteCopy(t *testing.T) {
	h := newContractHarness(t)

	first := uploadSave(t, h, "/saves", map[string]string{
		"rom_sha1": "rollback-rom",
		"slotName": "slot-a",
	}, "Yoshi's Story (USA) (En,Ja).eep", []byte("rollback-v1"))
	firstSave := mustObject(t, first["save"], "save")
	firstID := mustString(t, firstSave["id"], "save.id")

	second := uploadSave(t, h, "/saves", map[string]string{
		"rom_sha1": "rollback-rom",
		"slotName": "slot-a",
	}, "Yoshi's Story (USA) (En,Ja).eep", []byte("rollback-v2"))
	secondID := mustString(t, mustObject(t, second["save"], "save")["id"], "save.id")

	historyBefore := h.request(http.MethodGet, "/save?saveId="+firstID, nil)
	assertStatus(t, historyBefore, http.StatusOK)
	beforeBody := decodeJSONMap(t, historyBefore.Body)
	beforeVersions := mustArray(t, beforeBody["versions"], "versions")
	if len(beforeVersions) != 2 {
		t.Fatalf("expected 2 history versions before rollback, got %d", len(beforeVersions))
	}
	beforeSummary := mustObject(t, beforeBody["summary"], "summary")
	beforeLangs := mustArray(t, beforeSummary["languageCodes"], "summary.languageCodes")
	if len(beforeLangs) != 2 || mustString(t, beforeLangs[0], "summary.languageCodes[0]") != "EN" || mustString(t, beforeLangs[1], "summary.languageCodes[1]") != "JA" {
		t.Fatalf("unexpected language codes before rollback: %s", prettyJSON(beforeSummary))
	}
	latestBefore := mustObject(t, beforeVersions[0], "versions[0]")
	if mustString(t, latestBefore["id"], "versions[0].id") != secondID {
		t.Fatalf("expected newest history version to be second upload")
	}

	rollbackReq := fmt.Sprintf(`{"saveId":"%s"}`, firstID)
	rollbackResp := h.json(http.MethodPost, "/save/rollback", strings.NewReader(rollbackReq))
	assertStatus(t, rollbackResp, http.StatusOK)
	rollbackBody := decodeJSONMap(t, rollbackResp.Body)
	if !mustBool(t, rollbackBody["success"], "success") {
		t.Fatalf("expected rollback success")
	}
	if mustString(t, rollbackBody["sourceSaveId"], "sourceSaveId") != firstID {
		t.Fatalf("unexpected sourceSaveId: %s", prettyJSON(rollbackBody))
	}
	rollbackSave := mustObject(t, rollbackBody["save"], "save")
	rollbackSaveID := mustString(t, rollbackSave["id"], "save.id")
	if mustNumber(t, rollbackSave["version"], "save.version") != 3 {
		t.Fatalf("expected rollback to create version 3, got %s", prettyJSON(rollbackSave))
	}

	latest := h.request(http.MethodGet, "/save/latest?romSha1=rollback-rom&slotName=slot-a", nil)
	assertStatus(t, latest, http.StatusOK)
	latestBody := decodeJSONMap(t, latest.Body)
	if mustString(t, latestBody["id"], "id") != rollbackSaveID {
		t.Fatalf("expected rollback save to become latest: %s", prettyJSON(latestBody))
	}
	if mustNumber(t, latestBody["version"], "version") != 3 {
		t.Fatalf("expected latest version=3 after rollback: %s", prettyJSON(latestBody))
	}

	historyAfter := h.request(http.MethodGet, "/save?saveId="+firstID, nil)
	assertStatus(t, historyAfter, http.StatusOK)
	afterBody := decodeJSONMap(t, historyAfter.Body)
	afterVersions := mustArray(t, afterBody["versions"], "versions")
	if len(afterVersions) != 3 {
		t.Fatalf("expected 3 history versions after rollback, got %d", len(afterVersions))
	}
	newest := mustObject(t, afterVersions[0], "versions[0]")
	if mustString(t, newest["id"], "versions[0].id") != rollbackSaveID {
		t.Fatalf("expected rollback save as newest entry")
	}
	metadata := mustObject(t, newest["metadata"], "versions[0].metadata")
	rollbackMeta := mustObject(t, metadata["rollback"], "versions[0].metadata.rollback")
	if mustString(t, rollbackMeta["sourceSaveId"], "versions[0].metadata.rollback.sourceSaveId") != firstID {
		t.Fatalf("expected rollback metadata to reference source save")
	}
}

func TestContractV1SavesListEnvelope(t *testing.T) {
	h := newContractHarness(t)

	rr := h.request(http.MethodGet, "/v1/saves?limit=5&offset=0", nil)
	assertStatus(t, rr, http.StatusOK)
	body := decodeJSONMap(t, rr.Body)
	if !mustBool(t, body["success"], "success") {
		t.Fatalf("expected success=true")
	}
	saves := mustArray(t, body["saves"], "saves")
	if len(saves) == 0 {
		t.Fatalf("expected seeded saves to be present")
	}
	first := mustObject(t, saves[0], "saves[0]")
	if mustString(t, first["displayTitle"], "saves[0].displayTitle") == "" {
		t.Fatalf("expected displayTitle to be present")
	}
	if mustString(t, first["regionCode"], "saves[0].regionCode") == "" {
		t.Fatalf("expected regionCode to be present")
	}
	if mustNumber(t, first["saveCount"], "saves[0].saveCount") < 1 {
		t.Fatalf("expected saveCount >= 1")
	}
	if mustNumber(t, first["totalSizeBytes"], "saves[0].totalSizeBytes") < 1 {
		t.Fatalf("expected totalSizeBytes >= 1")
	}
	if mustNumber(t, body["total"], "total") < 1 {
		t.Fatalf("expected total >= 1")
	}
	if mustNumber(t, body["limit"], "limit") != 5 {
		t.Fatalf("expected echoed limit=5")
	}
	if mustNumber(t, body["offset"], "offset") != 0 {
		t.Fatalf("expected echoed offset=0")
	}
}

func TestContractDevicesListAndMissingDevice404(t *testing.T) {
	h := newContractHarness(t)

	list := h.request(http.MethodGet, "/devices", nil)
	assertStatus(t, list, http.StatusOK)
	body := decodeJSONMap(t, list.Body)
	if !mustBool(t, body["success"], "success") {
		t.Fatalf("expected success=true")
	}
	devices := mustArray(t, body["devices"], "devices")
	if len(devices) == 0 {
		t.Fatalf("expected at least the seeded internal device")
	}

	missing := h.json(http.MethodPatch, "/v1/devices/9999", strings.NewReader(`{"alias":"Atlas"}`))
	assertStatus(t, missing, http.StatusNotFound)
	missingBody := decodeJSONMap(t, missing.Body)
	if mustString(t, missingBody["message"], "message") != "Device not found" {
		t.Fatalf("unexpected missing-device body: %s", prettyJSON(missingBody))
	}
}

func TestContractConflictsCheckMissingAndReportMissingFileFailure(t *testing.T) {
	h := newContractHarness(t)

	check := h.request(http.MethodGet, "/conflicts/check?romSha1=missing-rom&slotName=slot-a", nil)
	assertStatus(t, check, http.StatusOK)
	body := decodeJSONMap(t, check.Body)
	if mustBool(t, body["exists"], "exists") {
		t.Fatalf("expected missing conflict record")
	}
	for _, key := range []string{"conflictId", "status", "cloudSha256", "cloudVersion", "cloudSaveId"} {
		if body[key] != nil {
			t.Fatalf("expected %s to be null, got %v", key, body[key])
		}
	}

	missing := h.multipart("/conflicts/report", map[string]string{
		"romSha1":     "conflict-rom",
		"slotName":    "default",
		"localSha256": "local",
		"cloudSha256": "cloud",
	}, "", "", nil)
	assertStatus(t, missing, http.StatusBadRequest)
	missingBody := decodeJSONMap(t, missing.Body)
	if mustString(t, missingBody["message"], "message") != "file is required" {
		t.Fatalf("unexpected report missing-file body: %s", prettyJSON(missingBody))
	}
}

func TestContractRomLookupSuccessShape(t *testing.T) {
	h := newContractHarness(t)

	rr := h.request(http.MethodGet, "/rom/lookup?filenameStem=Wario%20Land%20II", nil)
	assertStatus(t, rr, http.StatusOK)
	body := decodeJSONMap(t, rr.Body)
	if !mustBool(t, body["success"], "success") {
		t.Fatalf("expected success=true")
	}
	if mustNumber(t, body["count"], "count") != 1 {
		t.Fatalf("expected count=1")
	}
	rom := mustObject(t, body["rom"], "rom")
	if mustString(t, rom["sha1"], "rom.sha1") == "" || mustString(t, rom["md5"], "rom.md5") == "" {
		t.Fatalf("expected rom hashes in payload")
	}
	if mustString(t, rom["fileName"], "rom.fileName") != "Wario Land II.srm" {
		t.Fatalf("unexpected rom filename: %s", prettyJSON(rom))
	}
	game := mustObject(t, rom["game"], "rom.game")
	if mustString(t, game["name"], "rom.game.name") != "Wario Land II" {
		t.Fatalf("unexpected game name: %s", prettyJSON(game))
	}
	if game["boxart"] != nil || game["boxartThumb"] != nil {
		t.Fatalf("expected null boxart fields: %s", prettyJSON(game))
	}
}

func TestContractEventsPreludeAndContentType(t *testing.T) {
	h := newContractHarness(t)

	rr := h.ssePrelude("/events")
	assertStatus(t, rr, http.StatusOK)
	if got := rr.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("unexpected SSE content type: %q", got)
	}
	if !strings.HasPrefix(rr.Body.String(), ": connected\n\n") {
		t.Fatalf("expected connected SSE prelude, got %q", rr.Body.String())
	}
}

func TestContractBatchUploadRootAndV1AliasParity(t *testing.T) {
	payload := `{"items":[{"filename":"Wario Land II.srm","game":{"type":"name","value":{"name":"Wario Land II"}},"data":"` + base64.StdEncoding.EncodeToString([]byte("batch-save")) + `"}]}`

	rootHarness := newContractHarness(t)
	rootResp := rootHarness.json(http.MethodPost, "/saves", bytes.NewBufferString(payload))
	assertStatus(t, rootResp, http.StatusOK)

	v1Harness := newContractHarness(t)
	v1Resp := v1Harness.json(http.MethodPost, "/v1/saves", bytes.NewBufferString(payload))
	assertStatus(t, v1Resp, http.StatusOK)

	rootBody := normalizeForGolden(decodeJSONMap(t, rootResp.Body))
	v1Body := normalizeForGolden(decodeJSONMap(t, v1Resp.Body))
	assertEqualJSONValue(t, rootBody, v1Body, "batch upload alias parity")
}
