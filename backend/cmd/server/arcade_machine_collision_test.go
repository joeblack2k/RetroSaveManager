package main

import (
	"net/http"
	"strings"
	"testing"
)

// Two machines ship a file with the same name -- eeprom, game, bmcu_mcu -- and
// a MAME tree keeps them apart only by their directory. If neither upload
// carries a ROM hash the version key (rom:<sha>::slot:<slot>) is the same for
// both, so the second save is treated as a duplicate of the first and never
// stored. The machine is what distinguishes them.
func TestTwoArcadeMachinesWithTheSameFileNameAreTwoSaves(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	galaga := make([]byte, 4096)
	sfa3 := make([]byte, 4096)
	for i := range galaga {
		galaga[i] = byte(i * 3)
		sfa3[i] = byte(i*3 + 1)
	}
	uploads := []struct {
		machine string
		payload []byte
	}{{"galaga", galaga}, {"sfa3", sfa3}}
	for _, upload := range uploads {
		response := h.multipart("/saves", map[string]string{
			"rom_sha1":    "",
			"slotName":    "default",
			"system":      "arcade",
			"machine":     upload.machine,
			"device_type": "mister",
			"fingerprint": "arcade-collision-dev",
		}, "file", "eeprom", upload.payload)
		assertStatus(t, response, http.StatusOK)
	}

	list := h.request(http.MethodGet, "/api/saves?systemSlug=arcade&limit=100", nil)
	assertStatus(t, list, http.StatusOK)
	for _, machine := range []string{"galaga", "sfa3"} {
		if !strings.Contains(list.Body.String(), machine) {
			t.Fatalf("%s is missing from the arcade listing: %s", machine, list.Body.String())
		}
	}
}
