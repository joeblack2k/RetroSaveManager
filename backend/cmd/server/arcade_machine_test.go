package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// A MAME save lives at nvram/<machine>/<file>, and the file name is the
// machine's own filename -- 29f016a.27h says nothing about which game it
// belongs to. The helper therefore sends the machine name alongside the file;
// without using it the record is named after the file, and a restore writes
// the save back to a directory no machine reads.
func TestArcadeMachineNamesTheGame(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i * 5)
	}
	response := h.multipart("/saves", map[string]string{
		"rom_sha1":       "arcade-machine-rom",
		"slotName":       "default",
		"system":         "arcade",
		"machine":        "galaga",
		"device_type":    "mister",
		"fingerprint":    "arcade-machine-dev",
		"runtimeProfile": "arcade/mister",
	}, "file", "29f016a.27h", payload)
	assertStatus(t, response, http.StatusOK)

	var body struct {
		Save struct {
			ID string `json:"id"`
		} `json:"save"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Save.ID == "" {
		t.Fatalf("arcade upload reported no stored save: %s", response.Body.String())
	}
	for _, record := range h.app.snapshotSaveRecords() {
		if record.Summary.ID != body.Save.ID {
			continue
		}
		if got := record.Summary.DisplayTitle; got != "galaga" {
			t.Fatalf("record is named %q, want the machine name", got)
		}
		if got := record.GamePath; got != "galaga" {
			t.Fatalf("record's game path is %q, want the machine name", got)
		}
		return
	}
	t.Fatalf("arcade upload reported save %q but no record holds it", body.Save.ID)
}
