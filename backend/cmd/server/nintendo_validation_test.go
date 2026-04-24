package main

import "testing"

func TestNormalizeSaveInputAcceptsTrustedSNESRawSaveWithInspection(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Super Mario World (USA).srm",
		Payload:             buildNonBlankPayload(2048, 0x03),
		Game:                game{Name: "Super Mario World"},
		Format:              "sram",
		ROMSHA1:             "smw-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "snes",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected SNES raw save to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected SNES inspection metadata")
	}
	if result.Input.Inspection.ParserID != "snes-raw-sram" {
		t.Fatalf("unexpected parser id: %+v", result.Input.Inspection)
	}
	if result.Input.Inspection.ValidatedSystem != "snes" {
		t.Fatalf("unexpected validated system: %+v", result.Input.Inspection)
	}
	if got := result.Input.Inspection.SemanticFields["rawSaveKind"]; got != "SNES cartridge SRAM" {
		t.Fatalf("expected SNES raw save kind, got %+v", result.Input.Inspection.SemanticFields)
	}
	if got := result.Input.Inspection.SemanticFields["blankCheck"]; got != "passed" {
		t.Fatalf("expected blank check metadata, got %+v", result.Input.Inspection.SemanticFields)
	}
}

func TestNormalizeSaveInputRejectsSNESRawSaveWithoutROMSHA1(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Super Mario World (USA).srm",
		Payload:             buildNonBlankPayload(2048, 0x03),
		Game:                game{Name: "Super Mario World"},
		Format:              "sram",
		SystemSlug:          "snes",
		TrustedHelperSystem: true,
	})
	if !result.Rejected {
		t.Fatal("expected SNES raw save without rom_sha1 to be rejected")
	}
	if result.RejectReason != "snes raw saves require rom_sha1" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestNormalizeSaveInputAcceptsWeakSNESSlugTitle(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "sm.srm",
		Payload:             buildNonBlankPayload(8192, 0x03),
		Game:                game{Name: "sm"},
		Format:              "sram",
		ROMSHA1:             "sm-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "snes",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected weak SNES slug title to be preserved, got reject=%q", result.RejectReason)
	}
	if result.Input.DisplayTitle != "sm" {
		t.Fatalf("expected original title to be preserved without a proven alias, got %q", result.Input.DisplayTitle)
	}
}

func TestNormalizeSaveInputRejectsBlankSNESRawSave(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Super Mario Kart (USA).srm",
		Payload:             make([]byte, 2048),
		Game:                game{Name: "Super Mario Kart"},
		Format:              "sram",
		ROMSHA1:             "super-mario-kart-rom",
		SlotName:            "default",
		SystemSlug:          "snes",
		TrustedHelperSystem: true,
	})
	if !result.Rejected {
		t.Fatal("expected blank SNES raw save to be rejected")
	}
	if result.RejectReason != "snes raw save payload is blank (all 0x00)" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestNormalizeSaveInputValidatesDKC3FamilySRAM(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Donkey Kong Country 3 - Dixie Kong's Double Trouble! (USA).srm",
		Payload:             buildDKC3FixturePayload(),
		Game:                game{Name: "Donkey Kong Country 3 - Dixie Kong's Double Trouble!"},
		Format:              "sram",
		ROMSHA1:             "dkc3-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "snes",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected DKC3 SRAM to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected DKC3 inspection metadata")
	}
	if result.Input.Inspection.ParserID != snesDKCFamilyParserID {
		t.Fatalf("unexpected parser id: %+v", result.Input.Inspection)
	}
	if result.Input.Inspection.ParserLevel != saveParserLevelStructural {
		t.Fatalf("unexpected parser level: %+v", result.Input.Inspection)
	}
	if result.Input.Inspection.ValidatedGameTitle != "Donkey Kong Country 3 - Dixie Kong's Double Trouble!" {
		t.Fatalf("unexpected validated game title: %+v", result.Input.Inspection)
	}
	if result.Input.DisplayTitle != "Donkey Kong Country 3 - Dixie Kong's Double Trouble!" {
		t.Fatalf("expected validated DKC3 title, got %q", result.Input.DisplayTitle)
	}
}

func TestNormalizeSaveInputAcceptsNESRawSaveWithInspection(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "The Legend of Zelda (USA).sav",
		Payload:             buildNonBlankPayload(8192, 0x07),
		Game:                game{Name: "The Legend of Zelda"},
		Format:              "sram",
		ROMSHA1:             "zelda-nes-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "nes",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected NES raw save to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil || result.Input.Inspection.ParserID != "nes-raw-sram" {
		t.Fatalf("expected NES inspection metadata, got %+v", result.Input.Inspection)
	}
	if got := result.Input.Inspection.SemanticFields["rawSaveKind"]; got != "NES cartridge SRAM" {
		t.Fatalf("expected NES raw save kind, got %+v", result.Input.Inspection.SemanticFields)
	}
}

func TestNormalizeSaveInputAcceptsGBABackupSignatureWithInspection(t *testing.T) {
	a := &app{}
	payload := make([]byte, 32768)
	copy(payload[:16], []byte("SRAM_V113"))
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Wario Land 4 (USA).srm",
		Payload:             payload,
		Game:                game{Name: "Wario Land 4"},
		Format:              "sram",
		ROMSHA1:             "wario-land-4-rom",
		SlotName:            "default",
		SystemSlug:          "gba",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected GBA save to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected GBA inspection metadata")
	}
	if result.Input.Inspection.ParserID != "gba-raw-backup" {
		t.Fatalf("unexpected parser id: %+v", result.Input.Inspection)
	}
	if got := result.Input.Inspection.SemanticFields["rawSaveKind"]; got != "Game Boy Advance backup memory" {
		t.Fatalf("expected GBA raw save kind, got %+v", result.Input.Inspection.SemanticFields)
	}
}

func TestNormalizeSaveInputReclassifiesNSMBSaveToNDS(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailedWithOptions(saveCreateInput{
		Filename:     "New Super Mario Bros. (USA).sav",
		Payload:      buildNSMBNDSSaveFixture(),
		Game:         game{Name: "New Super Mario Bros."},
		Format:       "sram",
		ROMSHA1:      "nsmb-rom",
		SlotName:     "default",
		SystemSlug:   "gameboy",
		DisplayTitle: "New Super Mario Bros.",
		GameSlug:     "new-super-mario-bros",
		Metadata: map[string]any{
			"rsm": map[string]any{
				"systemDetection": map[string]any{
					"slug":          "gameboy",
					"reason":        "stored trusted system evidence",
					"trustedSystem": true,
					"evidence": map[string]any{
						"declared":      true,
						"storedTrusted": true,
					},
				},
			},
		},
	}, normalizeSaveInputOptions{StoredSystemFallbackOnly: true})
	if result.Rejected {
		t.Fatalf("expected NSMB save to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.SystemSlug != "nds" {
		t.Fatalf("expected NSMB save to be reclassified to nds, got %q", result.Input.SystemSlug)
	}
	if result.Input.Inspection == nil || result.Input.Inspection.ParserID != "nds-new-super-mario-bros" {
		t.Fatalf("expected NSMB-specific inspection metadata, got %+v", result.Input.Inspection)
	}
	if result.Input.DisplayTitle != "New Super Mario Bros." {
		t.Fatalf("expected validated title to become canonical display title, got %q", result.Input.DisplayTitle)
	}
}

func TestNormalizeSaveInputRejectsBlankTrustedNeoGeoSave(t *testing.T) {
	a := &app{}
	payload := make([]byte, neoGeoCompoundSaveSize)
	for i := range payload {
		payload[i] = 0xff
	}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "doubledr.sav",
		Payload:             payload,
		Game:                game{Name: "doubledr"},
		Format:              "sram",
		ROMSHA1:             "neogeo-doubledr-rom",
		SlotName:            "default",
		SystemSlug:          "neogeo",
		TrustedHelperSystem: true,
	})
	if !result.Rejected {
		t.Fatal("expected blank Neo Geo payload to be rejected")
	}
	if result.RejectReason != "neo geo payload does not match a validated save layout" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func buildNSMBNDSSaveFixture() []byte {
	payload := make([]byte, 8192)
	positions := []int{2, 258, 898, 1538, 2178, 4098, 4354, 4994, 5634, 6274}
	for idx, pos := range positions {
		copy(payload[pos:], []byte("Mario2d"))
		if pos+11 <= len(payload) {
			copy(payload[pos+7:], []byte{byte('0' + (idx % 10)), byte('0' + ((idx + 3) % 10)), byte('0' + ((idx + 6) % 10)), byte('0' + ((idx + 9) % 10))})
		}
	}
	return payload
}

func buildValidNeoGeoCompoundPayload() []byte {
	payload := make([]byte, neoGeoCompoundSaveSize)
	for i := 0; i < neoGeoSaveRAMSize; i++ {
		payload[i] = 0xff
	}
	for i := neoGeoSaveRAMSize; i < len(payload); i += 2 {
		payload[i] = byte((i / 2) % 251)
	}
	return payload
}

func buildNonBlankPayload(size int, value byte) []byte {
	payload := make([]byte, size)
	for idx := range payload {
		payload[idx] = 0x00
	}
	for idx := 0; idx < size && idx < 32; idx++ {
		payload[idx] = value
	}
	return payload
}

func buildDKC3FixturePayload() []byte {
	payload := make([]byte, dkcSRAMSize)
	for _, signature := range snesDKC3Signatures {
		copy(payload[signature.Offset:], []byte(signature.Value))
	}
	payload[0x0a] = 0x69
	payload[0x0e] = 0xaf
	payload[0xd2] = 0x59
	return payload
}
