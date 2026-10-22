package main

import (
	"net/http"
	"strings"
	"testing"
)

// A projection-capable upload whose payload carries no console signature was
// accepted at write time -- the helper is trusted -- and then rejected again
// when the record was decorated for the API, because decoration runs the same
// pipeline without the helper's trust flag. The record was relabelled
// unknown-system, which hides it from every listing and from the by-id lookup.
func TestProjectionCapableUploadStaysVisible(t *testing.T) {
	h := newContractHarness(t)
	assertStatus(t, h.json(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`)), http.StatusOK)

	payload := make([]byte, 131072)
	for i := range payload {
		payload[i] = byte(i * 7)
	}
	upload := h.multipart("/saves", map[string]string{
		"rom_sha1":       "gba-rom-1",
		"slotName":       "default",
		"system":         "gba",
		"device_type":    "mister",
		"fingerprint":    "gba-dev",
		"runtimeProfile": "gba/retroarch-mgba",
	}, "file", "Metroid Fusion (USA).sav", payload)
	assertStatus(t, upload, http.StatusOK)

	for _, record := range h.app.snapshotSaveRecords() {
		if record.ROMSHA1 == "gba-rom-1" && saveRecordSystemSlug(record) != "gba" {
			t.Fatalf("record stored as %q, want gba", saveRecordSystemSlug(record))
		}
	}

	list := h.request(http.MethodGet, "/api/saves?systemSlug=gba&limit=100", nil)
	assertStatus(t, list, http.StatusOK)
	if !strings.Contains(list.Body.String(), "Metroid Fusion") {
		t.Fatalf("projection-capable upload is not listed: %s", list.Body.String())
	}

	// The same record must survive a reload, which decorates it from disk.
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	again := h.request(http.MethodGet, "/api/saves?systemSlug=gba&limit=100", nil)
	if !strings.Contains(again.Body.String(), "Metroid Fusion") {
		t.Fatalf("record disappeared after reload: %s", again.Body.String())
	}
}
