package main

import (
	"net/http"
	"testing"
)

func TestNormalizeSaveInputRejectsTrustedGenesisRawSaveWithoutROMSHA1(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Sonic the Hedgehog.srm",
		Payload:             buildNonBlankPayload(8192, 0x05),
		Game:                game{Name: "Sonic the Hedgehog"},
		Format:              "sram",
		SystemSlug:          "genesis",
		TrustedHelperSystem: true,
	})
	if !result.Rejected {
		t.Fatal("expected Genesis raw save without rom_sha1 to be rejected")
	}
	if result.RejectReason != "genesis raw saves require rom_sha1" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestNormalizeSaveInputAcceptsTrustedGenesisRawSaveWithInspection(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Sonic the Hedgehog.srm",
		Payload:             buildNonBlankPayload(8192, 0x05),
		Game:                game{Name: "Sonic the Hedgehog"},
		Format:              "sram",
		ROMSHA1:             "genesis-sonic-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "genesis",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected Genesis raw save to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.SystemSlug != "genesis" {
		t.Fatalf("expected Genesis system slug, got %q", result.Input.SystemSlug)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected Sega inspection metadata")
	}
	if result.Input.Inspection.ParserLevel != saveParserLevelContainer {
		t.Fatalf("expected parser level %q, got %q", saveParserLevelContainer, result.Input.Inspection.ParserLevel)
	}
	if result.Input.Inspection.ParserID != "sega-raw-sram" {
		t.Fatalf("unexpected parser id: %q", result.Input.Inspection.ParserID)
	}
	if result.Input.Inspection.ValidatedSystem != "genesis" {
		t.Fatalf("unexpected validated system: %q", result.Input.Inspection.ValidatedSystem)
	}
	if got := result.Input.Inspection.SemanticFields["rawSaveKind"]; got != "Genesis / Mega Drive cartridge SRAM" {
		t.Fatalf("expected Genesis raw save kind, got %+v", result.Input.Inspection.SemanticFields)
	}
	if got := result.Input.Inspection.SemanticFields["blankCheck"]; got != "passed" {
		t.Fatalf("expected blank check metadata, got %+v", result.Input.Inspection.SemanticFields)
	}
	if result.Input.Game.HasParser {
		t.Fatal("expected raw Sega validator to remain below structural parser level")
	}
}

func TestNormalizeSaveInputRejectsBlankGenesisRawSave(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Sonic the Hedgehog.srm",
		Payload:             make([]byte, 8192),
		Game:                game{Name: "Sonic the Hedgehog"},
		Format:              "sram",
		ROMSHA1:             "genesis-sonic-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "genesis",
		TrustedHelperSystem: true,
	})
	if !result.Rejected {
		t.Fatal("expected blank Genesis raw save to be rejected")
	}
	if result.RejectReason != "genesis raw save payload is blank (all 0x00)" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestNormalizeSaveInputAcceptsWeakGenesisSlugTitle(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "daytona.ram",
		Payload:             buildNonBlankPayload(16384, 0x17),
		Game:                game{Name: "daytona"},
		Format:              "ram",
		ROMSHA1:             "daytona-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "genesis",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected weak Genesis slug title to be preserved, got reject=%q", result.RejectReason)
	}
	if result.Input.DisplayTitle != "daytona" {
		t.Fatalf("expected original title to be preserved without a proven alias, got %q", result.Input.DisplayTitle)
	}
}

func TestContractSavesMultipartAcceptsGenesisWithInspection(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "genesis-helper")

	uploadSave(t, h, "/saves", map[string]string{
		"app_password":   helperKey,
		"rom_sha1":       "genesis-rom-sha1",
		"slotName":       "default",
		"system":         "genesis",
		"device_type":    "mister",
		"fingerprint":    "genesis-device",
		"runtimeProfile": "genesis/genesis-plus-gx",
	}, "Sonic the Hedgehog.srm", buildNonBlankPayload(8192, 0x05))

	list := h.request(http.MethodGet, "/saves?limit=10&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	body := decodeJSONMap(t, list.Body)
	items := mustArray(t, body["saves"], "saves")
	if len(items) == 0 {
		t.Fatal("expected uploaded Genesis save in list")
	}
	first := mustObject(t, items[0], "items[0]")
	if mustString(t, first["systemSlug"], "items[0].systemSlug") != "genesis" {
		t.Fatalf("unexpected system slug: %s", prettyJSON(first))
	}
	inspection := mustObject(t, first["inspection"], "items[0].inspection")
	if mustString(t, inspection["parserLevel"], "items[0].inspection.parserLevel") != saveParserLevelContainer {
		t.Fatalf("unexpected inspection payload: %s", prettyJSON(first))
	}
	if mustString(t, inspection["parserId"], "items[0].inspection.parserId") != "sega-raw-sram" {
		t.Fatalf("unexpected inspection payload: %s", prettyJSON(first))
	}
	if mustString(t, inspection["validatedSystem"], "items[0].inspection.validatedSystem") != "genesis" {
		t.Fatalf("unexpected inspection payload: %s", prettyJSON(first))
	}
}

func TestContractSavesMultipartRejectsGenesisWithoutROMSHA1(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "genesis-helper")

	rr := h.multipart("/saves", map[string]string{
		"app_password":   helperKey,
		"slotName":       "default",
		"system":         "genesis",
		"device_type":    "mister",
		"fingerprint":    "genesis-device",
		"runtimeProfile": "genesis/genesis-plus-gx",
	}, "file", "Sonic the Hedgehog.srm", buildNonBlankPayload(8192, 0x05))
	assertStatus(t, rr, http.StatusUnprocessableEntity)
	assertJSONContentType(t, rr)
}

func TestNormalizeSaveInputAcceptsWeakNeoGeoSlugTitleAndAppliesAlias(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "doubledr.sav",
		Payload:             buildValidNeoGeoCompoundPayload(),
		Game:                game{Name: "doubledr"},
		Format:              "sram",
		ROMSHA1:             "doubledr-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "neogeo",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected weak Neo Geo slug title to be preserved, got reject=%q", result.RejectReason)
	}
	if result.Input.DisplayTitle != "Double Dragon" {
		t.Fatalf("expected Neo Geo alias resolution, got %q", result.Input.DisplayTitle)
	}
}
