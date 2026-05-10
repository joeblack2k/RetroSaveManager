package main

import (
	"net/http"
	"net/url"
	"testing"
)

func TestNativePortUploadUsesPortsTrackAndManifestMetadata(t *testing.T) {
	h := newContractHarness(t)

	body := uploadSave(t, h, "/saves", map[string]string{
		"system":           "ports",
		"rom_sha1":         "port:ship-of-harkinian",
		"slotName":         "file1",
		"runtimeProfile":   "port/ship-of-harkinian",
		"portId":           "ship-of-harkinian",
		"portName":         "The Legend of Zelda: Ocarina of Time (Ship of Harkinian)",
		"originSystemSlug": "n64",
		"portSaveKind":     "progress",
		"relativePath":     "Save/file1.sav",
		"rootRelativePath": "OcarinaOfTime/Save/file1.sav",
		"slotId":           "file1",
		"displayTitle":     "The Legend of Zelda: Ocarina of Time (Ship of Harkinian) - File 1",
	}, "file1.sav", []byte{1, 2, 3, 4})

	save := mustObject(t, body["save"], "save")
	saveID := mustString(t, save["id"], "save.id")

	list := h.request(http.MethodGet, "/saves?limit=10", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	items := mustArray(t, listBody["saves"], "saves")
	var found map[string]any
	for _, raw := range items {
		item := mustObject(t, raw, "save")
		if mustString(t, item["id"], "save.id") == saveID {
			found = item
			break
		}
	}
	if found == nil {
		t.Fatalf("native port save not found in list: %s", prettyJSON(listBody))
	}
	if mustString(t, found["systemSlug"], "systemSlug") != "ports" {
		t.Fatalf("expected ports system, got %s", prettyJSON(found))
	}
	if mustString(t, found["portId"], "portId") != "ship-of-harkinian" {
		t.Fatalf("expected port metadata on summary: %s", prettyJSON(found))
	}
	if mustString(t, found["originSystemSlug"], "originSystemSlug") != "n64" {
		t.Fatalf("expected origin system metadata: %s", prettyJSON(found))
	}

	latest := h.request(http.MethodGet, "/save/latest?romSha1=port:ship-of-harkinian&slotName="+url.QueryEscape("file1"), nil)
	assertStatus(t, latest, http.StatusOK)
	latestBody := decodeJSONMap(t, latest.Body)
	if mustString(t, latestBody["id"], "latest.id") != saveID {
		t.Fatalf("latest did not resolve native port track: %s", prettyJSON(latestBody))
	}
}

func TestNativePortUploadRejectsUnmanifestedPath(t *testing.T) {
	h := newContractHarness(t)

	rr := h.multipart("/saves", map[string]string{
		"system":           "ports",
		"rom_sha1":         "port:ship-of-harkinian",
		"slotName":         "bad",
		"runtimeProfile":   "port/ship-of-harkinian",
		"portId":           "ship-of-harkinian",
		"portName":         "The Legend of Zelda: Ocarina of Time (Ship of Harkinian)",
		"originSystemSlug": "n64",
		"portSaveKind":     "progress",
		"relativePath":     "mods/random.bin",
		"rootRelativePath": "OcarinaOfTime/mods/random.bin",
		"slotId":           "bad",
		"displayTitle":     "Bad Port Asset",
	}, "file", "random.bin", []byte{1, 2, 3, 4})
	assertStatus(t, rr, http.StatusUnprocessableEntity)
	body := decodeJSONMap(t, rr.Body)
	if mustString(t, body["reason"], "reason") == "" {
		t.Fatalf("expected manifest rejection reason: %s", prettyJSON(body))
	}
}
