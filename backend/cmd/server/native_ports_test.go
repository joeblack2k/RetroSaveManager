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

	portsOnly := h.request(http.MethodGet, "/saves?systemSlug=ports&limit=10", nil)
	assertStatus(t, portsOnly, http.StatusOK)
	portsBody := decodeJSONMap(t, portsOnly.Body)
	if total := int(mustNumber(t, portsBody["total"], "ports.total")); total != 1 {
		t.Fatalf("expected one filtered native port save, got %d: %s", total, prettyJSON(portsBody))
	}
	n64Only := h.request(http.MethodGet, "/saves?systemSlug=n64&limit=10", nil)
	assertStatus(t, n64Only, http.StatusOK)
	n64Body := decodeJSONMap(t, n64Only.Body)
	if total := int(mustNumber(t, n64Body["total"], "n64.total")); total != 0 {
		t.Fatalf("expected native port save to stay isolated from n64 list, got %d: %s", total, prettyJSON(n64Body))
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

func TestNativePortUploadAllowsManifestDefaultFilename(t *testing.T) {
	h := newContractHarness(t)

	payload := make([]byte, 512)
	payload[0] = 0x4d
	payload[1] = 0x4b

	body := uploadSave(t, h, "/saves", map[string]string{
		"system":           "ports",
		"rom_sha1":         "port:spaghettikart",
		"slotName":         "default",
		"runtimeProfile":   "port/spaghettikart",
		"portId":           "spaghettikart",
		"portName":         "Mario Kart 64 (SpaghettiKart)",
		"originSystemSlug": "n64",
		"portSaveKind":     "progress",
		"relativePath":     "default.sav",
		"rootRelativePath": "MarioKart64/default.sav",
		"slotId":           "default",
		"displayTitle":     "Mario Kart 64 (SpaghettiKart) - Default",
	}, "default.sav", payload)

	save := mustObject(t, body["save"], "save")
	saveID := mustString(t, save["id"], "save.id")
	if saveID == "" {
		t.Fatalf("expected manifest default save upload to succeed: %s", prettyJSON(body))
	}

	list := h.request(http.MethodGet, "/saves?systemSlug=ports&limit=10", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	items := mustArray(t, listBody["saves"], "saves")
	found := false
	for _, raw := range items {
		item := mustObject(t, raw, "save")
		if mustString(t, item["id"], "save.id") == saveID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("manifest default save was accepted but missing from ports list: %s", prettyJSON(listBody))
	}

	latest := h.request(http.MethodGet, "/save/latest?romSha1="+url.QueryEscape("port:spaghettikart")+"&slotName=default", nil)
	assertStatus(t, latest, http.StatusOK)
	latestBody := decodeJSONMap(t, latest.Body)
	if mustString(t, latestBody["id"], "latest.id") != saveID {
		t.Fatalf("latest did not resolve manifest default save: %s", prettyJSON(latestBody))
	}
}
