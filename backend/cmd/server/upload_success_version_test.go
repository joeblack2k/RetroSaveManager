package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Every upload response that reports a stored save must carry the version the
// record was written with. The helper keeps a per-save version to detect
// conflicts; a success body without one forces it to treat a stored save as
// unknown and re-upload, and hides the version from anything that reads the
// response to decide whether the cloud copy is newer.
func TestUploadSuccessBodyCarriesTheStoredVersion(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	payload := make([]byte, 131072)
	for i := range payload {
		payload[i] = byte(i * 13)
	}
	response := h.multipart("/saves", map[string]string{
		"rom_sha1":       "gba-version-rom",
		"slotName":       "default",
		"system":         "gba",
		"device_type":    "mister",
		"fingerprint":    "gba-version-dev",
		"runtimeProfile": "gba/retroarch-mgba",
	}, "file", "Golden Sun (USA).sav", payload)
	assertStatus(t, response, http.StatusOK)

	var body struct {
		Success bool `json:"success"`
		Save    struct {
			ID      string `json:"id"`
			SHA256  string `json:"sha256"`
			Version int    `json:"version"`
		} `json:"save"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("upload response is not JSON: %v (%s)", err, response.Body.String())
	}
	if body.Save.ID == "" {
		t.Fatalf("upload response names no save: %s", response.Body.String())
	}

	var storedVersion int
	found := false
	for _, record := range h.app.snapshotSaveRecords() {
		if record.Summary.ID == body.Save.ID {
			storedVersion = record.Summary.Version
			found = true
		}
	}
	if !found {
		t.Fatalf("upload reported save %q but no record holds it", body.Save.ID)
	}
	if body.Save.Version != storedVersion || body.Save.Version == 0 {
		t.Fatalf("upload success body carries version %d, record holds %d: %s",
			body.Save.Version, storedVersion, response.Body.String())
	}
}
