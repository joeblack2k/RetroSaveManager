package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestNormalizeIdempotencyKey(t *testing.T) {
	key, err := normalizeIdempotencyKey("  sync-123  ")
	if err != nil || key != "sync-123" {
		t.Fatalf("unexpected normalized key %q err=%v", key, err)
	}
	if _, err := normalizeIdempotencyKey(strings.Repeat("x", idempotencyKeyMaxLength+1)); err == nil {
		t.Fatal("expected overlong idempotency key to be rejected")
	}
	if _, err := normalizeIdempotencyKey("bad key"); err == nil {
		t.Fatal("expected whitespace inside key to be rejected")
	}
}

func TestSaveUploadRequestFingerprintIgnoresMultipartBoundaryAndFieldOrder(t *testing.T) {
	fieldsA := map[string]string{"system": "snes", "rom_sha1": "rom-a", "slotName": "default"}
	fieldsB := map[string]string{"slotName": "default", "rom_sha1": "rom-a", "system": "snes"}
	payload := buildNonBlankPayload(2048, 0x51)

	reqA := idempotencyMultipartRequest(t, "/saves", fieldsA, "file", "game.srm", payload)
	reqB := idempotencyMultipartRequest(t, "/saves", fieldsB, "file", "game.srm", payload)
	fpA, err := saveUploadRequestFingerprint(reqA)
	if err != nil {
		t.Fatalf("fingerprint A: %v", err)
	}
	fpB, err := saveUploadRequestFingerprint(reqB)
	if err != nil {
		t.Fatalf("fingerprint B: %v", err)
	}
	cleanupMultipartForm(reqA)
	cleanupMultipartForm(reqB)
	if fpA != fpB {
		t.Fatalf("semantic retries must share fingerprint: %s != %s", fpA, fpB)
	}
}

func TestSaveUploadIdempotencyReplaysSameResultWithoutNewRevision(t *testing.T) {
	h := newContractHarness(t)
	fields := map[string]string{
		"rom_sha1": "idempotent-rom",
		"slotName": "default",
		"system":   "snes",
	}
	payload := buildNonBlankPayload(2048, 0x61)

	firstReq := idempotencyMultipartRequest(t, "/saves", fields, "file", "Chrono Trigger.srm", payload)
	firstReq.Header.Set("Idempotency-Key", "sync-operation-1")
	first := h.do(firstReq)
	assertStatus(t, first, http.StatusOK)
	firstBody := decodeJSONMap(t, first.Body)
	firstID := mustString(t, mustObject(t, firstBody["save"], "save")["id"], "save.id")
	beforeCount := len(h.app.snapshotSaveRecords())

	secondReq := idempotencyMultipartRequest(t, "/saves", fields, "file", "Chrono Trigger.srm", payload)
	secondReq.Header.Set("Idempotency-Key", "sync-operation-1")
	second := h.do(secondReq)
	assertStatus(t, second, http.StatusOK)
	if second.Header().Get("X-RSM-Idempotent-Replay") != "true" {
		t.Fatal("expected replay marker on repeated idempotency key")
	}
	secondBody := decodeJSONMap(t, second.Body)
	secondID := mustString(t, mustObject(t, secondBody["save"], "save")["id"], "save.id")
	if secondID != firstID {
		t.Fatalf("replay returned different save id: %q != %q", secondID, firstID)
	}
	if afterCount := len(h.app.snapshotSaveRecords()); afterCount != beforeCount {
		t.Fatalf("replay mutated save history: before=%d after=%d", beforeCount, afterCount)
	}
}

func TestSaveUploadIdempotencyRejectsKeyReuseWithDifferentPayload(t *testing.T) {
	h := newContractHarness(t)
	fields := map[string]string{"rom_sha1": "idempotency-conflict-rom", "slotName": "default", "system": "snes"}

	firstReq := idempotencyMultipartRequest(t, "/saves", fields, "file", "game.srm", buildNonBlankPayload(2048, 0x71))
	firstReq.Header.Set("Idempotency-Key", "sync-operation-conflict")
	first := h.do(firstReq)
	assertStatus(t, first, http.StatusOK)

	secondReq := idempotencyMultipartRequest(t, "/saves", fields, "file", "game.srm", buildNonBlankPayload(2048, 0x72))
	secondReq.Header.Set("Idempotency-Key", "sync-operation-conflict")
	second := h.do(secondReq)
	assertStatus(t, second, http.StatusConflict)
	if !strings.Contains(strings.ToLower(second.Body.String()), "different save upload") {
		t.Fatalf("unexpected conflict body: %s", second.Body.String())
	}
}

func TestSaveUploadIdempotencySurvivesAppRestart(t *testing.T) {
	h := newContractHarness(t)
	fields := map[string]string{"rom_sha1": "idempotency-restart-rom", "slotName": "default", "system": "snes"}
	payload := buildNonBlankPayload(2048, 0x81)

	firstReq := idempotencyMultipartRequest(t, "/saves", fields, "file", "game.srm", payload)
	firstReq.Header.Set("Idempotency-Key", "sync-operation-restart")
	first := h.do(firstReq)
	assertStatus(t, first, http.StatusOK)
	firstBody := decodeJSONMap(t, first.Body)
	firstID := mustString(t, mustObject(t, firstBody["save"], "save")["id"], "save.id")

	app2 := newApp()
	app2.applyBootstrapDemoPolicy()
	if err := app2.initSaveStore(); err != nil {
		t.Fatalf("restart init save store: %v", err)
	}
	handler2 := newRouter(app2)
	secondReq := idempotencyMultipartRequest(t, "/saves", fields, "file", "game.srm", payload)
	secondReq.Header.Set("Idempotency-Key", "sync-operation-restart")
	second := httptest.NewRecorder()
	handler2.ServeHTTP(second, secondReq)
	assertStatus(t, second, http.StatusOK)
	if second.Header().Get("X-RSM-Idempotent-Replay") != "true" {
		t.Fatal("expected durable replay marker after app restart")
	}
	secondBody := decodeJSONMap(t, second.Body)
	if got := mustString(t, mustObject(t, secondBody["save"], "save")["id"], "save.id"); got != firstID {
		t.Fatalf("restart replay returned %q want %q", got, firstID)
	}
}

func TestSaveIdempotencyStateDoesNotPersistRawKey(t *testing.T) {
	h := newContractHarness(t)
	fields := map[string]string{"rom_sha1": "idempotency-secret-rom", "slotName": "default", "system": "snes"}
	key := "client-generated-operation-identifier"
	req := idempotencyMultipartRequest(t, "/saves", fields, "file", "game.srm", buildNonBlankPayload(2048, 0x91))
	req.Header.Set("Idempotency-Key", key)
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)

	data, err := os.ReadFile(saveIdempotencyStatePath())
	if err != nil {
		t.Fatalf("read idempotency state: %v", err)
	}
	if bytes.Contains(data, []byte(key)) {
		t.Fatal("raw Idempotency-Key must not be persisted")
	}
	if !bytes.Contains(data, []byte(idempotencyKeyHash(key))) {
		t.Fatal("expected hashed idempotency key in durable state")
	}
}

func idempotencyMultipartRequest(t *testing.T, path string, fields map[string]string, fileField, fileName string, payload []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, key := range keys {
		if err := writer.WriteField(key, fields[key]); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	part, err := writer.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Protection", "1")
	return req
}
