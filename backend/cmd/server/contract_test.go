package main

import (
	"bytes"
	"encoding/base64"
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

	okBody := uploadSave(t, h, "/saves", map[string]string{
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
