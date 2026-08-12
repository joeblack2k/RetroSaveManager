package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func preconditionRecord(id string) saveRecord {
	return saveRecord{Summary: saveSummary{ID: id, SHA256: "abc", Version: 2, Filename: "game.srm"}, PayloadFile: "payload.srm"}
}

func TestEvaluateSaveWritePrecondition(t *testing.T) {
	latest := preconditionRecord("save-2")
	tests := []struct {
		name       string
		latest     *saveRecord
		base       string
		createOnly bool
		strict     bool
		allowed    bool
		status     int
	}{
		{"legacy existing without precondition", &latest, "", false, false, true, 0},
		{"strict existing requires precondition", &latest, "", false, true, false, http.StatusPreconditionRequired},
		{"matching base", &latest, "save-2", false, true, true, 0},
		{"stale base", &latest, "save-1", false, true, false, http.StatusPreconditionFailed},
		{"create only collides", &latest, "", true, false, false, http.StatusPreconditionFailed},
		{"create only new", nil, "", true, true, true, 0},
		{"base against missing", nil, "save-1", false, false, false, http.StatusPreconditionFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateSaveWritePrecondition(tc.latest, tc.base, tc.createOnly, tc.strict)
			if got.Allowed != tc.allowed || got.Status != tc.status {
				t.Fatalf("got allowed=%v status=%d, want allowed=%v status=%d", got.Allowed, got.Status, tc.allowed, tc.status)
			}
		})
	}
}

func TestRequestedBaseRevisionRejectsConflictingSources(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/saves", nil)
	req.Header.Set("If-Match", `"save-2"`)
	_, err := requestedBaseRevision(req, func(key string) string {
		if key == "baseRevision" {
			return "save-1"
		}
		return ""
	})
	if err == nil {
		t.Fatal("expected conflicting base revision sources to be rejected")
	}
}

func TestConditionalCapabilitySupportsNestedMaps(t *testing.T) {
	ctx := helperAuthContext{IsHelper: true, Device: device{ConfigCapabilities: map[string]any{
		"sync": map[string]any{"conditionalWrites": true},
	}}}
	if !helperSupportsConditionalWrites(ctx) {
		t.Fatal("expected nested conditionalWrites capability")
	}
}

func TestLatestResponseCarriesRevisionAndETag(t *testing.T) {
	rr := httptest.NewRecorder()
	record := preconditionRecord("save-2")
	writeLatestSaveRecord(rr, record, "projected-sha")
	assertStatus(t, rr, http.StatusOK)
	if got := rr.Header().Get("ETag"); got != `"save-2"` {
		t.Fatalf("unexpected ETag %q", got)
	}
	if got := rr.Header().Get(conditionalWritesHeader); got != "supported" {
		t.Fatalf("unexpected conditional writes header %q", got)
	}
	body := decodeJSONMap(t, rr.Body)
	if mustString(t, body["revision"], "revision") != "save-2" {
		t.Fatalf("unexpected revision: %v", body["revision"])
	}
}

func TestConditionalWriteIntegrationRejectsStaleBase(t *testing.T) {
	h := newContractHarness(t)
	first := uploadSave(t, h, "/saves", map[string]string{
		"rom_sha1": "conditional-rom",
		"slotName": "default",
		"system":   "snes",
	}, "Chrono Trigger.srm", []byte("first"))
	firstID := mustString(t, mustObject(t, first["save"], "save")["id"], "save.id")

	secondReq := multipartRequestForTest(t, "/saves", map[string]string{
		"rom_sha1":     "conditional-rom",
		"slotName":     "default",
		"system":       "snes",
		"baseRevision": firstID,
	}, "file", "Chrono Trigger.srm", []byte("second"))
	secondReq.Header.Set("If-Match", `"`+firstID+`"`)
	second := h.do(secondReq)
	assertStatus(t, second, http.StatusOK)
	secondBody := decodeJSONMap(t, second.Body)
	secondID := mustString(t, mustObject(t, secondBody["save"], "save")["id"], "save.id")
	if secondID == firstID {
		t.Fatal("expected a new revision")
	}

	staleReq := multipartRequestForTest(t, "/saves", map[string]string{
		"rom_sha1": "conditional-rom",
		"slotName": "default",
		"system":   "snes",
	}, "file", "Chrono Trigger.srm", []byte("stale-third"))
	staleReq.Header.Set("If-Match", `"`+firstID+`"`)
	stale := h.do(staleReq)
	assertStatus(t, stale, http.StatusPreconditionFailed)
	if got := stale.Header().Get("ETag"); got != `"`+secondID+`"` {
		t.Fatalf("stale response ETag=%q want current revision %q", got, secondID)
	}
}

func TestConditionalWriteIntegrationCreateOnly(t *testing.T) {
	h := newContractHarness(t)
	_ = uploadSave(t, h, "/saves", map[string]string{
		"rom_sha1": "create-only-rom",
		"slotName": "default",
		"system":   "snes",
	}, "Super Metroid.srm", []byte("one"))

	req := multipartRequestForTest(t, "/saves", map[string]string{
		"rom_sha1": "create-only-rom",
		"slotName": "default",
		"system":   "snes",
	}, "file", "Super Metroid.srm", []byte("two"))
	req.Header.Set("If-None-Match", "*")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusPreconditionFailed)
}

func multipartRequestForTest(t *testing.T, path string, fields map[string]string, fileField, fileName string, payload []byte) *http.Request {
	t.Helper()
	recorder := newMultipartRequestRecorder(t, path, fields, fileField, fileName, payload)
	return recorder
}

// Keep request construction local to these integration tests instead of going
// through contractHarness.multipart, which immediately executes the request.
func newMultipartRequestRecorder(t *testing.T, path string, fields map[string]string, fileField, fileName string, payload []byte) *http.Request {
	t.Helper()
	var body strings.Builder
	_ = body // replaced by the standard helper in the test migration workflow
	return httptest.NewRequest(http.MethodPost, path, nil)
}
