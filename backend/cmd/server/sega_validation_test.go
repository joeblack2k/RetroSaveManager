package main

import (
	"encoding/binary"
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

func TestNormalizeSaveInputAcceptsHelperSegaCDAnd32XRawSaves(t *testing.T) {
	cases := []struct {
		system string
		name   string
		kind   string
	}{
		{system: "sega-cd", name: "Lunar.srm", kind: "Sega CD / Mega-CD backup RAM"},
		{system: "sega-32x", name: "Virtua Racing Deluxe.sav", kind: "Sega 32X cartridge SRAM"},
	}

	for _, tc := range cases {
		t.Run(tc.system, func(t *testing.T) {
			a := &app{}
			result := a.normalizeSaveInputDetailed(saveCreateInput{
				Filename:            tc.name,
				Payload:             buildNonBlankPayload(8192, 0x07),
				Game:                game{Name: tc.name},
				Format:              "sram",
				ROMSHA1:             tc.system + "-rom-sha1",
				SlotName:            "default",
				SystemSlug:          tc.system,
				TrustedHelperSystem: true,
			})
			if result.Rejected {
				t.Fatalf("expected %s raw save to be accepted, got reject=%q", tc.system, result.RejectReason)
			}
			if result.Input.SystemSlug != tc.system {
				t.Fatalf("expected system slug %q, got %q", tc.system, result.Input.SystemSlug)
			}
			if result.Input.Inspection == nil {
				t.Fatal("expected Sega inspection metadata")
			}
			if got := result.Input.Inspection.SemanticFields["rawSaveKind"]; got != tc.kind {
				t.Fatalf("expected raw save kind %q, got %+v", tc.kind, result.Input.Inspection.SemanticFields)
			}
		})
	}
}

func TestNormalizeSaveInputValidatesSonic3SRAMSemantics(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Sonic The Hedgehog 3 (USA).sav",
		Payload:             buildSonic3FixturePayload(),
		Game:                game{Name: "Sonic The Hedgehog 3"},
		Format:              "sram",
		ROMSHA1:             "genesis-sonic-3-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "genesis",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected Sonic 3 SRAM to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected Sonic 3 inspection metadata")
	}
	if result.Input.Inspection.ParserLevel != saveParserLevelSemantic {
		t.Fatalf("expected parser level %q, got %q", saveParserLevelSemantic, result.Input.Inspection.ParserLevel)
	}
	if result.Input.Inspection.ParserID != genesisSonicParserID {
		t.Fatalf("unexpected parser id: %q", result.Input.Inspection.ParserID)
	}
	if result.Input.Inspection.ValidatedGameTitle != "Sonic The Hedgehog 3" {
		t.Fatalf("expected validated Sonic 3 title, got %q", result.Input.Inspection.ValidatedGameTitle)
	}
	fields := result.Input.Inspection.SemanticFields
	if got := fields["stage"]; got != "Hydrocity Zone Act 1" {
		t.Fatalf("expected Hydrocity stage, got %+v", fields)
	}
	if got := fields["character"]; got != "Sonic & Tails" {
		t.Fatalf("expected Sonic & Tails character, got %+v", fields)
	}
	if got := fields["livesNote"]; got != "Not stored in Sonic 3 standalone SRAM" {
		t.Fatalf("expected explicit Sonic 3 lives note, got %+v", fields)
	}
	if !result.Input.Game.HasParser {
		t.Fatal("expected Sonic semantic parser to mark game as parser-backed")
	}
}

func TestNormalizeSaveInputValidatesSonic3KnucklesLives(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Sonic 3 & Knuckles.srm",
		Payload:             buildSonicSKFixturePayload(),
		Game:                game{Name: "Sonic 3 & Knuckles"},
		Format:              "sram",
		ROMSHA1:             "genesis-sonic-sk-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "genesis",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected Sonic 3 & Knuckles SRAM to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected Sonic 3 & Knuckles inspection metadata")
	}
	if result.Input.Inspection.ValidatedGameTitle != "Sonic 3 & Knuckles" {
		t.Fatalf("expected validated Sonic 3 & Knuckles title, got %q", result.Input.Inspection.ValidatedGameTitle)
	}
	fields := result.Input.Inspection.SemanticFields
	if got := fields["stage"]; got != "Mushroom Hill Zone Act 1" {
		t.Fatalf("expected Mushroom Hill stage, got %+v", fields)
	}
	if got := fields["character"]; got != "Knuckles" {
		t.Fatalf("expected Knuckles character, got %+v", fields)
	}
	if got := fields["lives"]; got != 7 {
		t.Fatalf("expected lives to be decoded, got %+v", fields)
	}
	if got := fields["continues"]; got != 2 {
		t.Fatalf("expected continues to be decoded, got %+v", fields)
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

func TestNormalizeSaveInputDoesNotPromoteInvalidSonicLikeSRAM(t *testing.T) {
	a := &app{}
	payload := buildSonic3FixturePayload()
	payload[sonicS3RegionOffset+sonicS3RegionSize-1] ^= 0x7f
	payload[sonicS3BackupOffset+sonicS3RegionSize-1] ^= 0x7f
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Sonic The Hedgehog 3 (USA).sav",
		Payload:             payload,
		Game:                game{Name: "Sonic The Hedgehog 3"},
		Format:              "sram",
		ROMSHA1:             "genesis-sonic-3-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "genesis",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected raw Genesis save to remain accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected raw inspection metadata")
	}
	if result.Input.Inspection.ParserID == genesisSonicParserID {
		t.Fatalf("expected invalid Sonic checksum not to promote semantic parser: %+v", result.Input.Inspection)
	}
	if result.Input.Inspection.ParserLevel != saveParserLevelContainer {
		t.Fatalf("expected fallback raw parser level, got %q", result.Input.Inspection.ParserLevel)
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

func buildSonic3FixturePayload() []byte {
	payload := buildBlankGenesisSRAMFixture()
	region := make([]byte, sonicS3RegionSize)
	for idx := 0; idx < sonicS3SlotCount; idx++ {
		region[idx*sonicS3SlotSize] = 0x80
	}
	copy(region[:sonicS3SlotSize], []byte{0x00, 0x00, 0x00, 0x01, 0x02, 0x01, 0x80, 0x00})
	writeSonicRegion(payload, sonicS3RegionOffset, region, sonicS3IntegrityWord)
	writeSonicRegion(payload, sonicS3BackupOffset, region, sonicS3IntegrityWord)
	return payload
}

func buildSonicSKFixturePayload() []byte {
	payload := buildBlankGenesisSRAMFixture()
	region := make([]byte, sonicSKRegionSize)
	for idx := 0; idx < sonicSKSlotCount; idx++ {
		region[idx*sonicSKSlotSize] = 0x80
	}
	copy(region[:sonicSKSlotSize], []byte{0x00, 0x00, 0x30, 0x06, 0x00, 0x00, 0xff, 0xff, 0x07, 0x02})
	writeSonicRegion(payload, sonicSKRegionOffset, region, sonicSKIntegrityWord)
	writeSonicRegion(payload, sonicSKBackupOffset, region, sonicSKIntegrityWord)
	return payload
}

func buildBlankGenesisSRAMFixture() []byte {
	payload := make([]byte, 65536)
	for idx := range payload {
		payload[idx] = 0xff
	}
	return payload
}

func writeSonicRegion(payload []byte, offset int, region []byte, integrity uint16) {
	binary.BigEndian.PutUint16(region[len(region)-4:len(region)-2], integrity)
	binary.BigEndian.PutUint16(region[len(region)-2:], sonicSRAMChecksum(region[:len(region)-2]))
	copy(payload[offset:], region)
}
