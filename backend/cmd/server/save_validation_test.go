package main

import (
	"net/http"
	"testing"
)

func TestSaveUploadPreviewAcceptsAndRejectsWithoutPersisting(t *testing.T) {
	h := newContractHarness(t)

	accepted := h.multipart("/api/saves/preview", map[string]string{
		"system":   "gameboy",
		"slotName": "default",
		"rom_sha1": "preview-rom",
	}, "file", "Pokemon Red.sav", buildNonBlankPayload(8192, 0x19))
	assertStatus(t, accepted, http.StatusOK)
	acceptedBody := decodeJSONMap(t, accepted.Body)
	if mustNumber(t, acceptedBody["acceptedCount"], "acceptedCount") != 1 {
		t.Fatalf("expected one accepted preview: %s", prettyJSON(acceptedBody))
	}
	items := mustArray(t, acceptedBody["items"], "items")
	first := mustObject(t, items[0], "items[0]")
	if !mustBool(t, first["accepted"], "items[0].accepted") {
		t.Fatalf("expected accepted preview item: %s", prettyJSON(first))
	}

	rejected := h.multipart("/api/saves/preview", nil, "file", "notes.txt", []byte("not a save file"))
	assertStatus(t, rejected, http.StatusOK)
	rejectedBody := decodeJSONMap(t, rejected.Body)
	if mustNumber(t, rejectedBody["rejectedCount"], "rejectedCount") != 1 {
		t.Fatalf("expected one rejected preview: %s", prettyJSON(rejectedBody))
	}

	list := h.request(http.MethodGet, "/saves?limit=100&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	if len(mustArray(t, listBody["saves"], "saves")) != 1 {
		t.Fatalf("preview should not persist saves: %s", prettyJSON(listBody))
	}
}

func TestRejectedUploadIsQuarantinedAndListedInValidation(t *testing.T) {
	h := newContractHarness(t)

	upload := h.multipart("/saves", nil, "file", "notes.txt", []byte("not a save file"))
	assertStatus(t, upload, http.StatusUnprocessableEntity)

	status := h.request(http.MethodGet, "/api/validation", nil)
	assertStatus(t, status, http.StatusOK)
	body := decodeJSONMap(t, status.Body)
	validation := mustObject(t, body["validation"], "validation")
	if mustNumber(t, validation["quarantineCount"], "validation.quarantineCount") != 1 {
		t.Fatalf("expected one quarantined upload: %s", prettyJSON(validation))
	}
	quarantine := mustArray(t, validation["quarantine"], "validation.quarantine")
	first := mustObject(t, quarantine[0], "quarantine[0]")
	if mustString(t, first["filename"], "filename") != "notes.txt" {
		t.Fatalf("unexpected quarantine file: %s", prettyJSON(first))
	}
	if mustString(t, first["reason"], "reason") == "" {
		t.Fatalf("expected quarantine reason: %s", prettyJSON(first))
	}
}
