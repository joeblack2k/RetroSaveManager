package main

import (
	"net/http"
	"strings"
	"testing"
)

// A fresh save -- the file an emulator creates at first boot, before anything
// is written -- is a uniform run of 0x00 or 0xFF. It carries no signature
// either, so the signature check used to answer first and the reader was told
// the save was "missing a validated payload signature" when it simply held no
// save yet.
func TestBlankRawSaveIsRefusedAsBlank(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	for _, fill := range []byte{0xFF, 0x00} {
		payload := make([]byte, 512)
		for i := range payload {
			payload[i] = fill
		}
		response := h.multipart("/saves", map[string]string{
			"rom_sha1":       "gba-blank-rom",
			"slotName":       "default",
			"system":         "gba",
			"device_type":    "steamdeck",
			"fingerprint":    "steam-machine",
			"runtimeProfile": "gba/retroarch-mgba",
		}, "file", "NightFire (USA).sav", payload)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("fill %#x: got %d, want 422: %s", fill, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "blank (all 0x") {
			t.Fatalf("fill %#x: the refusal does not name blankness: %s", fill, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "signature") {
			t.Fatalf("fill %#x: a blank save is reported as a signature problem: %s", fill, response.Body.String())
		}
	}
}

// The blank check moving earlier must not change what a real, non-blank save
// does: one without the library signature footer is still accepted on helper
// trust.
func TestNonBlankSignaturelessRawSaveStillPassesOnHelperTrust(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	payload := make([]byte, 32768)
	for i := range payload {
		payload[i] = byte(i * 5)
	}
	response := h.multipart("/saves", map[string]string{
		"rom_sha1":       "gba-played-rom",
		"slotName":       "default",
		"system":         "gba",
		"device_type":    "steamdeck",
		"fingerprint":    "steam-machine",
		"runtimeProfile": "gba/retroarch-mgba",
	}, "file", "Golden Sun (USA).sav", payload)
	assertStatus(t, response, http.StatusOK)
}
