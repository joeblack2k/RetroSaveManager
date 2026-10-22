package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func deckStyleN64Bundle() []byte {
	bundle := make([]byte, n64RetroArchSRMSize)
	for i := n64RetroArchSRAMOffset; i < n64RetroArchSRAMOffset+n64RetroArchSRAMSize; i++ {
		bundle[i] = byte(i * 7)
	}
	// mupen64plus-next materialises four blank 32 KiB controller-pak templates
	// per game: a short sequential fill and nothing else.
	for pak := 0; pak < 4; pak++ {
		start := n64RetroArchControllerOffset + pak*n64RetroArchControllerPakSize
		for i := 0; i < 376; i++ {
			bundle[start+i] = []byte{0x81, 0x01, 0x02, 0x03}[i%4]
		}
	}
	return bundle
}

func uploadN64Bundle(t *testing.T, h *contractHarness, filename string, bundle []byte) *httptest.ResponseRecorder {
	t.Helper()
	return h.multipart("/saves", map[string]string{
		"rom_sha1":       "n64-bundle-rom",
		"slotName":       "default",
		"system":         "n64",
		"device_type":    "steamdeck",
		"fingerprint":    "deck-n64",
		"runtimeProfile": "n64/retroarch",
	}, "file", filename, bundle)
}

// A mupen64plus-next save is one file holding EEPROM, four blank controller-pak
// templates, SRAM and FlashRAM. The blank templates and the untouched media
// regions are not saves. Counting them made the split call a real Deck save
// ambiguous -- "multiple populated controller paks", then "multiple populated
// save classes" -- and the upload was refused.
func TestRetroArchN64BundleWithBlankPakTemplatesIsAccepted(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	bundle := deckStyleN64Bundle()
	response := uploadN64Bundle(t, h, "1080 Snowboarding (Japan, USA) (En,Ja).srm", bundle)
	assertStatus(t, response, http.StatusOK)

	var body struct {
		Save struct {
			ID string `json:"id"`
		} `json:"save"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Save.ID == "" {
		t.Fatalf("upload named no save: %s", response.Body.String())
	}
	// The record is named after the game the client sent. The store writes the
	// payload to disk as payload.<ext>, so a record titled "payload" means the
	// name came from the file rather than from the save.
	for _, record := range h.app.snapshotSaveRecords() {
		if record.Summary.ID != body.Save.ID {
			continue
		}
		if record.Summary.DisplayTitle != "1080 Snowboarding" {
			t.Fatalf("record titled %q, want the game name", record.Summary.DisplayTitle)
		}
		return
	}
	t.Fatalf("upload reported save %q but no record holds it", body.Save.ID)

	download := h.request(http.MethodGet, "/api/saves/"+body.Save.ID+"/download?runtimeProfile=n64/retroarch&device_type=steamdeck&fingerprint=deck-n64", nil)
	assertStatus(t, download, http.StatusOK)
	projected := download.Body.Bytes()
	if len(projected) != n64RetroArchSRMSize {
		t.Fatalf("projected bundle is %d bytes, want %d", len(projected), n64RetroArchSRMSize)
	}
	for i := n64RetroArchSRAMOffset; i < n64RetroArchSRAMOffset+n64RetroArchSRAMSize; i++ {
		if projected[i] != bundle[i] {
			t.Fatalf("SRAM differs at offset %#x after the round trip", i-n64RetroArchSRAMOffset)
		}
	}
}

// Untouched media is left as a uniform run -- 0x00 in a fresh file, 0xFF in
// some cores. Either reads as "no save", so a bundle carrying only SRAM is one
// component, not three.
func TestRetroArchN64BundleTreatsUniformMediaAsUnused(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	bundle := make([]byte, n64RetroArchSRMSize)
	for i := range bundle {
		bundle[i] = 0xFF
	}
	for i := n64RetroArchSRAMOffset; i < n64RetroArchSRAMOffset+n64RetroArchSRAMSize; i++ {
		bundle[i] = byte(i * 3)
	}
	response := uploadN64Bundle(t, h, "GoldenEye 007 (USA).srm", bundle)
	assertStatus(t, response, http.StatusOK)
}

// FlashRAM has no verified sample pair anywhere on the fleet, so its word
// order would be a guess whose only symptom is a corrupted restore on another
// device. A bundle that carries it is refused, with the reason.
func TestRetroArchN64BundleWithFlashRAMIsRefused(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	// FlashRAM only: everything else is untouched, so this is the one shape the
	// old code accepted on a guessed swap rather than refusing.
	bundle := make([]byte, n64RetroArchSRMSize)
	for i := n64RetroArchFlashRAMOffset; i < n64RetroArchFlashRAMOffset+n64RetroArchFlashRAMSize; i++ {
		bundle[i] = byte(i * 11)
	}
	response := uploadN64Bundle(t, h, "Majora's Mask (USA).srm", bundle)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "FlashRAM") {
		t.Fatalf("the refusal does not name FlashRAM: %s", response.Body.String())
	}
}
