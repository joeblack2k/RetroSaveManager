package main

import (
	"net/http"
	"strings"
	"testing"
)

// The write path accepts a raw GBA payload on helper trust: admission
// signatures are advisory for an authenticated helper because RetroArch's
// libretro-mGBA core never writes them. The rescan path re-validates the same
// record from the payload alone, with no helper flag on the request, so it
// rejects the record the write path certified -- and with pruning on, deletes
// it. Every GBA save is stored and then erased by the first rescan.
func TestRescanKeepsATrustedHelpersRawSave(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	payload := make([]byte, 131072)
	for i := range payload {
		payload[i] = byte(i * 11)
	}
	upload := h.multipart("/saves", map[string]string{
		"rom_sha1":       "gba-rescan-rom",
		"slotName":       "default",
		"system":         "gba",
		"device_type":    "mister",
		"fingerprint":    "gba-rescan-dev",
		"runtimeProfile": "gba/retroarch-mgba",
	}, "file", "Metroid - Zero Mission (USA).sav", payload)
	assertStatus(t, upload, http.StatusOK)

	result, err := h.app.rescanSaves(saveRescanOptions{PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan: %v", err)
	}
	for _, rejection := range result.Rejections {
		t.Logf("rejected %s: %s", rejection.Filename, rejection.Reason)
	}
	if result.Rejected != 0 {
		t.Fatalf("rescan rejected %d record(s), want 0", result.Rejected)
	}

	list := h.request(http.MethodGet, "/api/saves?systemSlug=gba&limit=100", nil)
	assertStatus(t, list, http.StatusOK)
	if !strings.Contains(list.Body.String(), "Zero Mission") {
		t.Fatalf("the record is missing after rescan: %s", list.Body.String())
	}
}
