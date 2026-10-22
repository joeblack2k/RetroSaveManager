package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// An arcade save arrives as a raw NVRAM/EEPROM dump whose extension is not one
// of the four the detector recognises as arcade-specific (.nv, .nvram, .hi,
// .eeprom) -- a MAME tree holds <machine>/<file> pairs whose file name carries
// no arcade extension at all. The detector already accepts a trusted helper's
// declaration as the strongest evidence there is for every other system; the
// arcade guard ignored it and rejected the upload as "arcade requires
// dedicated header/extension or machine hint" even though the helper had
// declared arcade.
func TestTrustedHelperArcadeUploadIsNotRejectedForItsExtension(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i * 3)
	}
	response := h.multipart("/saves", map[string]string{
		"rom_sha1":       "arcade-rom-1",
		"slotName":       "default",
		"system":         "arcade",
		"device_type":    "mister",
		"fingerprint":    "arcade-dev",
		"runtimeProfile": "arcade/mister",
	}, "file", "galaga.sav", payload)
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
		if slug := saveRecordSystemSlug(record); slug != "arcade" {
			t.Fatalf("trusted helper's arcade upload stored as %q, want arcade", slug)
		}
		return
	}
	t.Fatalf("arcade upload reported save %q but no record holds it", body.Save.ID)
}
