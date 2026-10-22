package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func installCheatService(h *contractHarness) {
	h.app.mu.Lock()
	h.app.cheats = &cheatService{}
	h.app.mu.Unlock()
}

func countCheatPayloadReads(t *testing.T) func() int {
	t.Helper()
	previous := cheatPayloadReader
	reads := 0
	cheatPayloadReader = func(path string) ([]byte, error) {
		reads++
		return previous(path)
	}
	t.Cleanup(func() { cheatPayloadReader = previous })
	return func() int { return reads }
}

func seedSave(t *testing.T, h *contractHarness, title, rom string, fill byte) string {
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
		"fingerprint":    "listing-dev",
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

// Listing reads no payloads. Cheat availability is detail-level: resolving it
// opens the record's payload, and doing that per record made /api/saves and
// /api/systems cost O(records) disk reads on every request -- seconds locally,
// a timeout over NFS.
func TestListingDoesNotReadPayloads(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	ids := []string{
		seedSave(t, h, "Tetris", "list-rom-tetris", 0x11),
		seedSave(t, h, "Kirby's Dream Land 2", "list-rom-kirby", 0x22),
		seedSave(t, h, "Wario Land", "list-rom-wario", 0x33),
	}
	// Installed before the reload so the startup walk is covered: decoration
	// used to resolve cheat capability for every record as it loaded them.
	installCheatService(h)
	reads := countCheatPayloadReads(t)
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reads(); got != 0 {
		t.Fatalf("startup read %d payload(s) for cheat capability; decoration must not resolve cheats", got)
	}

	for _, path := range []string{
		"/api/saves?limit=100",
		"/api/saves?limit=1",
		"/api/systems",
		"/saves?limit=100",
	} {
		response := h.request(http.MethodGet, path, nil)
		assertStatus(t, response, http.StatusOK)
		if got := reads(); got != 0 {
			t.Fatalf("%s read %d payload(s); a listing must read none", path, got)
		}
	}

	detail := h.request(http.MethodGet, "/api/saves/"+ids[0], nil)
	assertStatus(t, detail, http.StatusOK)
	if reads() == 0 {
		t.Fatalf("the detail endpoint did not resolve cheat capability; the detail path must still read the payload")
	}
}
