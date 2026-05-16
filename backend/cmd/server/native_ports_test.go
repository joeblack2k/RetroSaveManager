package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
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

func TestNativePortRuntimeProfileUploadsAsOriginN64AndProjectsBack(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "steamdeck-port-projection")
	portPayload := buildMK64FixturePayload()

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password":     helperKey,
		"device_type":      "steamdeck",
		"fingerprint":      "deck-port-projection",
		"system":           "n64",
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
	}, "default.sav", portPayload)

	saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
	list := h.request(http.MethodGet, "/saves?systemSlug=n64&limit=10", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	items := mustArray(t, listBody["saves"], "saves")
	if len(items) != 1 {
		t.Fatalf("expected one origin N64 row, got %s", prettyJSON(listBody))
	}
	item := mustObject(t, items[0], "save")
	if mustString(t, item["displayTitle"], "displayTitle") != "Mario Kart 64" {
		t.Fatalf("expected port upload to normalize under origin game: %s", prettyJSON(item))
	}
	if mustString(t, item["systemSlug"], "systemSlug") != "n64" {
		t.Fatalf("expected origin system n64: %s", prettyJSON(item))
	}
	profiles := mustArray(t, item["downloadProfiles"], "downloadProfiles")
	if !downloadProfilePresent(t, profiles, "port/spaghettikart") {
		t.Fatalf("expected SpaghettiKart download target in profiles: %s", prettyJSON(item))
	}

	portDownload := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape("port/spaghettikart"), nil)
	assertStatus(t, portDownload, http.StatusOK)
	if got := portDownload.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="default.sav"`) {
		t.Fatalf("expected SpaghettiKart default.sav filename, got %q", got)
	}
	if !bytes.Equal(portDownload.Body.Bytes(), portPayload) {
		t.Fatal("expected SpaghettiKart projection to match original 512-byte port payload")
	}

	retroArchDownload := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape(n64ProfileRetroArch), nil)
	assertStatus(t, retroArchDownload, http.StatusOK)
	if len(retroArchDownload.Body.Bytes()) != n64RetroArchSRMSize {
		t.Fatalf("expected RetroArch N64 SRM size, got %d", len(retroArchDownload.Body.Bytes()))
	}
	canonical, info, err := splitRetroArchN64SRM(retroArchDownload.Body.Bytes())
	if err != nil {
		t.Fatalf("split projected RetroArch SRM: %v", err)
	}
	if info.MediaType != "eeprom" {
		t.Fatalf("expected EEPROM media, got %q", info.MediaType)
	}
	window, err := n64SmallEEPROMWindow(canonical, "Mario Kart 64")
	if err != nil {
		t.Fatalf("extract canonical EEPROM window: %v", err)
	}
	if !bytes.Equal(window, portPayload) {
		t.Fatal("expected RetroArch projection to carry the SpaghettiKart EEPROM window")
	}

	latest := h.request(http.MethodGet, "/save/latest?slotName=default&filename="+url.QueryEscape("Mario Kart 64.eep")+"&system=n64&displayTitle="+url.QueryEscape("Mario Kart 64")+"&runtimeProfile="+url.QueryEscape("port/spaghettikart"), nil)
	assertStatus(t, latest, http.StatusOK)
	latestBody := decodeJSONMap(t, latest.Body)
	if mustString(t, latestBody["sha256"], "latest.sha256") != payloadSHA256Hex(portPayload) {
		t.Fatalf("expected latest to compare projected SpaghettiKart bytes: %s", prettyJSON(latestBody))
	}
}

func TestNativePortRuntimeProfileProjectsStarshipBothDirections(t *testing.T) {
	portPayload := buildSF64FixturePayload()
	canonical := normalizeN64EEPROM(portPayload)
	summary := saveSummary{
		DisplayTitle: "Star Fox 64",
		SystemSlug:   "n64",
		Filename:     "Star Fox 64.eep",
		MediaType:    "eeprom",
		FileSize:     len(canonical),
	}

	filename, _, projected, err := projectN64ToNativePortPayload(summary, canonical, "port/starship")
	if err != nil {
		t.Fatalf("project Star Fox 64 to Starship: %v", err)
	}
	if filename != "default.sav" {
		t.Fatalf("unexpected Starship filename: %q", filename)
	}
	if !bytes.Equal(projected, portPayload) {
		t.Fatal("expected Starship projection to be the 512-byte Star Fox 64 EEPROM")
	}

	portSummary := saveSummary{
		DisplayTitle:     "Star Fox 64 (Starship) - Default",
		SystemSlug:       "ports",
		Filename:         "default.sav",
		RuntimeProfile:   "port/starship",
		PortID:           "starship",
		OriginSystemSlug: "n64",
		MediaType:        "eeprom",
		FileSize:         len(portPayload),
	}
	filename, _, retroarch, err := projectNativePortPayload(portSummary, portPayload, n64ProfileRetroArch)
	if err != nil {
		t.Fatalf("project Starship to RetroArch: %v", err)
	}
	if filename != "Star Fox 64.srm" {
		t.Fatalf("unexpected RetroArch filename: %q", filename)
	}
	roundTrip, info, err := splitRetroArchN64SRM(retroarch)
	if err != nil {
		t.Fatalf("split Starship RetroArch projection: %v", err)
	}
	if info.MediaType != "eeprom" {
		t.Fatalf("expected EEPROM media, got %q", info.MediaType)
	}
	window, err := n64SmallEEPROMWindow(roundTrip, "Star Fox 64")
	if err != nil {
		t.Fatalf("extract Star Fox EEPROM window: %v", err)
	}
	if !bytes.Equal(window, portPayload) {
		t.Fatal("expected Starship -> RetroArch projection to preserve EEPROM window")
	}
}

func TestShipOfHarkinianRuntimeProfileConvertsToOOTSRAMAndBack(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "soh-port-projection")
	ootFixture := buildOOTCheatFixturePayload()
	filename, sohPayload, err := ootSRAMToShipOfHarkinian(saveSummary{
		DisplayTitle: "The Legend of Zelda: Ocarina of Time",
		SystemSlug:   "n64",
		Filename:     "The Legend of Zelda - Ocarina of Time.sra",
		MediaType:    "sram",
		FileSize:     ootSramSize,
	}, ootFixture)
	if err != nil {
		t.Fatalf("build Ship fixture: %v", err)
	}
	if filename != "file1.sav" {
		t.Fatalf("expected file1.sav fixture, got %q", filename)
	}

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password":     helperKey,
		"device_type":      "steamdeck",
		"fingerprint":      "deck-soh-port",
		"system":           "n64",
		"slotName":         "file1",
		"runtimeProfile":   "port/ship-of-harkinian",
		"portId":           "ship-of-harkinian",
		"portName":         "The Legend of Zelda: Ocarina of Time (Ship of Harkinian)",
		"originSystemSlug": "n64",
		"portSaveKind":     "progress",
		"relativePath":     "Save/file1.sav",
		"rootRelativePath": "OcarinaOfTime/Save/file1.sav",
		"slotId":           "file1",
		"displayTitle":     "The Legend of Zelda: Ocarina of Time",
	}, "file1.sav", sohPayload)

	save := mustObject(t, body["save"], "save")
	saveID := mustString(t, save["id"], "save.id")

	list := h.request(http.MethodGet, "/saves?systemSlug=n64&limit=10", nil)
	assertStatus(t, list, http.StatusOK)
	item := mustObject(t, mustArray(t, decodeJSONMap(t, list.Body)["saves"], "saves")[0], "save")
	if mustString(t, item["displayTitle"], "displayTitle") != "The Legend of Zelda: Ocarina of Time" {
		t.Fatalf("expected origin game title: %s", prettyJSON(item))
	}
	if got := mustString(t, item["mediaType"], "mediaType"); got != "sram" {
		t.Fatalf("expected Ship upload to normalize as OOT SRAM, got %q: %s", got, prettyJSON(item))
	}
	profiles := mustArray(t, item["downloadProfiles"], "downloadProfiles")
	if !downloadProfilePresent(t, profiles, "port/ship-of-harkinian") {
		t.Fatalf("expected Ship of Harkinian target: %s", prettyJSON(item))
	}
	if !downloadProfilePresent(t, profiles, n64ProfileRetroArch) {
		t.Fatalf("expected N64 emulator targets to remain available: %s", prettyJSON(item))
	}

	retroArch := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape(n64ProfileRetroArch), nil)
	assertStatus(t, retroArch, http.StatusOK)
	canonical, info, err := splitRetroArchN64SRM(retroArch.Body.Bytes())
	if err != nil {
		t.Fatalf("split OOT RetroArch projection: %v", err)
	}
	if info.MediaType != "sram" {
		t.Fatalf("expected OOT SRAM media, got %q", info.MediaType)
	}
	parsed, err := parseOOTSRAM(canonical)
	if err != nil {
		t.Fatalf("projected OOT SRAM should parse: %v", err)
	}
	if !parsed.Slots[0].Present {
		t.Fatal("expected converted OOT SRAM slot 1 to be present")
	}

	portDownload := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape("port/ship-of-harkinian"), nil)
	assertStatus(t, portDownload, http.StatusOK)
	if got := portDownload.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="file1.sav"`) {
		t.Fatalf("expected Ship file1.sav filename, got %q", got)
	}
	var ship map[string]any
	if err := json.Unmarshal(portDownload.Body.Bytes(), &ship); err != nil {
		t.Fatalf("Ship projection should be JSON: %v", err)
	}
	base := mustObject(t, mustObject(t, mustObject(t, ship["sections"], "sections")["base"], "base")["data"], "base.data")
	if got := int(mustNumber(t, base["rupees"], "rupees")); got != 123 {
		t.Fatalf("expected rupees to round-trip, got %d in %s", got, prettyJSON(base))
	}
}

func TestSuperMetroidNativePortUsesIdentityRuntimeConversion(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "super-metroid-port")
	payload := buildNonBlankPayload(8192, 0x42)

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password":     helperKey,
		"device_type":      "steamdeck",
		"fingerprint":      "deck-super-metroid-port",
		"system":           "snes",
		"rom_sha1":         "port:super-metroid-native:test",
		"slotName":         "sm",
		"runtimeProfile":   "port/super-metroid-native",
		"portId":           "super-metroid-native",
		"portName":         "Super Metroid (Native Port)",
		"originSystemSlug": "snes",
		"portSaveKind":     "progress",
		"relativePath":     "saves/sm.srm",
		"rootRelativePath": "SuperMetroid/saves/sm.srm",
		"slotId":           "sm",
		"displayTitle":     "Super Metroid",
	}, "sm.srm", payload)

	saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
	list := h.request(http.MethodGet, "/saves?systemSlug=snes&limit=10", nil)
	assertStatus(t, list, http.StatusOK)
	item := mustObject(t, mustArray(t, decodeJSONMap(t, list.Body)["saves"], "saves")[0], "save")
	profiles := mustArray(t, item["downloadProfiles"], "downloadProfiles")
	if !downloadProfilePresent(t, profiles, "port/super-metroid-native") {
		t.Fatalf("expected Super Metroid native target: %s", prettyJSON(item))
	}
	if !downloadProfilePresent(t, profiles, "snes/retroarch-snes9x") {
		t.Fatalf("expected SNES emulator targets for raw-compatible port: %s", prettyJSON(item))
	}

	portDownload := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape("port/super-metroid-native"), nil)
	assertStatus(t, portDownload, http.StatusOK)
	if got := portDownload.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="sm.srm"`) {
		t.Fatalf("expected native Super Metroid filename, got %q", got)
	}
	if !bytes.Equal(portDownload.Body.Bytes(), payload) {
		t.Fatal("expected native Super Metroid port projection to preserve raw SRAM")
	}

	retroArch := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape("snes/retroarch-snes9x"), nil)
	assertStatus(t, retroArch, http.StatusOK)
	if !bytes.Equal(retroArch.Body.Bytes(), payload) {
		t.Fatal("expected Super Metroid port save to export unchanged to SNES emulator SRAM")
	}

	emulatorPayload := buildNonBlankPayload(8192, 0x24)
	emulatorBody := uploadSave(t, h, "/saves", map[string]string{
		"app_password":   helperKey,
		"device_type":    "steamdeck",
		"fingerprint":    "deck-super-metroid-port",
		"system":         "snes",
		"rom_sha1":       "rom:super-metroid:emulator",
		"slotName":       "default",
		"runtimeProfile": "snes/retroarch-snes9x",
	}, "Super Metroid.srm", emulatorPayload)
	emulatorSaveID := mustString(t, mustObject(t, emulatorBody["save"], "save")["id"], "save.id")
	nativeDownload := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(emulatorSaveID)+"&runtimeProfile="+url.QueryEscape("port/super-metroid-native"), nil)
	assertStatus(t, nativeDownload, http.StatusOK)
	if got := nativeDownload.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="sm.srm"`) {
		t.Fatalf("expected native Super Metroid filename for emulator export, got %q", got)
	}
	if !bytes.Equal(nativeDownload.Body.Bytes(), emulatorPayload) {
		t.Fatal("expected SNES emulator SRAM to export unchanged to Super Metroid native port")
	}
}

func TestNativePortRuntimeProfileUploadsAsOriginGenesisWithoutEmulatorProjection(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "sonic-port-projection")
	portPayload := []byte("selected=1\nslot=2\n")

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password":     helperKey,
		"device_type":      "steamdeck",
		"fingerprint":      "deck-sonic-port",
		"system":           "genesis",
		"slotName":         "savesel",
		"runtimeProfile":   "port/sonic1-forever",
		"portId":           "sonic1-forever",
		"portName":         "Sonic 1 Forever",
		"originSystemSlug": "genesis",
		"portSaveKind":     "progress",
		"relativePath":     "Scripts/Save/SaveSel.txt",
		"rootRelativePath": "Sonic1Forever/Scripts/Save/SaveSel.txt",
		"slotId":           "savesel",
		"displayTitle":     "Sonic the Hedgehog",
	}, "SaveSel.txt", portPayload)

	saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
	slotPayload := []byte("slot=2\ncontinues=3\n")
	slotBody := uploadSave(t, h, "/saves", map[string]string{
		"app_password":     helperKey,
		"device_type":      "steamdeck",
		"fingerprint":      "deck-sonic-port",
		"system":           "genesis",
		"slotName":         "saveslot",
		"runtimeProfile":   "port/sonic1-forever",
		"portId":           "sonic1-forever",
		"portName":         "Sonic 1 Forever",
		"originSystemSlug": "genesis",
		"portSaveKind":     "progress",
		"relativePath":     "Scripts/Save/SaveSlot.txt",
		"rootRelativePath": "Sonic1Forever/Scripts/Save/SaveSlot.txt",
		"slotId":           "saveslot",
		"displayTitle":     "Sonic the Hedgehog",
	}, "SaveSlot.txt", slotPayload)
	slotSaveID := mustString(t, mustObject(t, slotBody["save"], "save")["id"], "slotSave.id")

	list := h.request(http.MethodGet, "/saves?systemSlug=genesis&limit=10", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	items := mustArray(t, listBody["saves"], "saves")
	if len(items) != 1 {
		t.Fatalf("expected Sonic port files to group under one origin Genesis row, got %s", prettyJSON(listBody))
	}
	item := mustObject(t, items[0], "save")
	if mustString(t, item["displayTitle"], "displayTitle") != "Sonic the Hedgehog" {
		t.Fatalf("expected port upload to normalize under origin game: %s", prettyJSON(item))
	}
	if got := int(mustNumber(t, item["saveCount"], "saveCount")); got != 2 {
		t.Fatalf("expected both Sonic port artifacts to stay visible under one game row, got saveCount=%d: %s", got, prettyJSON(item))
	}
	profiles := mustArray(t, item["downloadProfiles"], "downloadProfiles")
	if !downloadProfilePresent(t, profiles, "port/sonic1-forever") {
		t.Fatalf("expected Sonic port download target in profiles: %s", prettyJSON(item))
	}
	if downloadProfilePresent(t, profiles, "genesis/retroarch") {
		t.Fatalf("Sonic port text save must not advertise emulator projection without an adapter: %s", prettyJSON(item))
	}

	portDownload := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(saveID)+"&runtimeProfile="+url.QueryEscape("port/sonic1-forever"), nil)
	assertStatus(t, portDownload, http.StatusOK)
	if got := portDownload.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="SaveSel.txt"`) {
		t.Fatalf("expected Sonic port filename, got %q", got)
	}
	if !bytes.Equal(portDownload.Body.Bytes(), portPayload) {
		t.Fatal("expected Sonic port projection to return original text payload")
	}

	slotDownload := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(slotSaveID)+"&runtimeProfile="+url.QueryEscape("port/sonic1-forever"), nil)
	assertStatus(t, slotDownload, http.StatusOK)
	if got := slotDownload.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="SaveSlot.txt"`) {
		t.Fatalf("expected Sonic slot filename, got %q", got)
	}
	if !bytes.Equal(slotDownload.Body.Bytes(), slotPayload) {
		t.Fatal("expected Sonic slot port projection to return original text payload")
	}

	detail := h.request(http.MethodGet, "/save?saveId="+url.QueryEscape(slotSaveID), nil)
	assertStatus(t, detail, http.StatusOK)
	detailBody := decodeJSONMap(t, detail.Body)
	versions := mustArray(t, detailBody["versions"], "versions")
	if len(versions) != 2 {
		t.Fatalf("expected Sonic detail to expose both port artifacts, got %s", prettyJSON(detailBody))
	}
	seenFiles := map[string]bool{}
	for _, raw := range versions {
		version := mustObject(t, raw, "version")
		seenFiles[mustString(t, version["filename"], "version.filename")] = true
	}
	if !seenFiles["SaveSel.txt"] || !seenFiles["SaveSlot.txt"] {
		t.Fatalf("expected SaveSel and SaveSlot in detail versions, got %s", prettyJSON(detailBody))
	}

	latestSelect := h.request(http.MethodGet, "/save/latest?slotName=savesel&filename="+url.QueryEscape("Sonic the Hedgehog.srm")+"&system=genesis&displayTitle="+url.QueryEscape("Sonic the Hedgehog")+"&runtimeProfile="+url.QueryEscape("port/sonic1-forever"), nil)
	assertStatus(t, latestSelect, http.StatusOK)
	latestSelectBody := decodeJSONMap(t, latestSelect.Body)
	if mustString(t, latestSelectBody["id"], "latestSelect.id") != saveID || mustString(t, latestSelectBody["sha256"], "latestSelect.sha256") != payloadSHA256Hex(portPayload) {
		t.Fatalf("expected SaveSel latest lookup to stay on the SaveSel port slot: %s", prettyJSON(latestSelectBody))
	}

	latestSlot := h.request(http.MethodGet, "/save/latest?slotName=saveslot&filename="+url.QueryEscape("Sonic the Hedgehog.srm")+"&system=genesis&displayTitle="+url.QueryEscape("Sonic the Hedgehog")+"&runtimeProfile="+url.QueryEscape("port/sonic1-forever"), nil)
	assertStatus(t, latestSlot, http.StatusOK)
	latestSlotBody := decodeJSONMap(t, latestSlot.Body)
	if mustString(t, latestSlotBody["id"], "latestSlot.id") != slotSaveID || mustString(t, latestSlotBody["sha256"], "latestSlot.sha256") != payloadSHA256Hex(slotPayload) {
		t.Fatalf("expected SaveSlot latest lookup to stay on the SaveSlot port slot: %s", prettyJSON(latestSlotBody))
	}
}

func downloadProfilePresent(t *testing.T, profiles []any, id string) bool {
	t.Helper()
	for _, raw := range profiles {
		profile := mustObject(t, raw, "downloadProfile")
		if mustString(t, profile["id"], "downloadProfile.id") == id {
			return true
		}
	}
	return false
}
