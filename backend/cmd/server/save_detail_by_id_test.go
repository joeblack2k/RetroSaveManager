package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func uploadGameBoySave(t *testing.T, h *contractHarness, title, rom string, fill byte) string {
	t.Helper()
	payload := make([]byte, 8192)
	for i := range payload {
		payload[i] = fill
	}
	response := h.multipart("/saves", map[string]string{
		"rom_sha1":       rom,
		"slotName":       "default",
		"system":         "gameboy",
		"device_type":    "mister",
		"fingerprint":    "detail-dev",
		"runtimeProfile": "gameboy/mister",
	}, "file", title+".sav", payload)
	assertStatus(t, response, http.StatusOK)
	var body struct {
		Save struct {
			ID string `json:"id"`
		} `json:"save"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Save.ID == "" {
		t.Fatalf("upload of %s named no save: %s", title, response.Body.String())
	}
	return body.Save.ID
}

// A detail request answers with the record its id names. An id that names
// nothing is a 404 -- answering it with the whole store makes a stale or
// mistyped id look like a save of its own and hands the caller a record it
// never asked for.
func TestSaveDetailByNameServesOnlyThatRecord(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	firstID := uploadGameBoySave(t, h, "Tetris", "rom-tetris", 0x11)
	secondID := uploadGameBoySave(t, h, "Kirby's Dream Land 2", "rom-kirby", 0x22)
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	for _, tc := range []struct{ id, want, unwanted string }{
		{firstID, "Tetris", "Kirby"},
		{secondID, "Kirby", "Tetris"},
	} {
		detail := h.request(http.MethodGet, "/api/saves/"+tc.id, nil)
		assertStatus(t, detail, http.StatusOK)
		if !strings.Contains(detail.Body.String(), tc.want) || strings.Contains(detail.Body.String(), tc.unwanted) {
			t.Fatalf("detail for %s served the wrong record: %s", tc.id, detail.Body.String())
		}
	}

	missing := h.request(http.MethodGet, "/api/saves/totally-not-an-id", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown save id returned %d, want 404: %s", missing.Code, missing.Body.String())
	}
}
