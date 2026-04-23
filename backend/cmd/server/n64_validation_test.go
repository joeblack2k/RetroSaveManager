package main

import (
	"encoding/binary"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeSaveInputRejectsBlankN64EEPROM(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Wave Race 64 (USA).eep",
		Payload:             make([]byte, 512),
		Game:                game{Name: "Wave Race 64"},
		SystemSlug:          "n64",
		TrustedHelperSystem: true,
	})
	if !result.Rejected {
		t.Fatal("expected blank N64 EEPROM to be rejected")
	}
	if result.RejectReason != "n64 eeprom payload is blank (all 0x00)" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestNormalizeSaveInputRejectsBlankN64FlashRAM(t *testing.T) {
	a := &app{}
	payload := make([]byte, 131072)
	for i := range payload {
		payload[i] = 0xFF
	}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Paper Mario (USA).fla",
		Payload:             payload,
		Game:                game{Name: "Paper Mario"},
		SystemSlug:          "n64",
		TrustedHelperSystem: true,
	})
	if !result.Rejected {
		t.Fatal("expected blank N64 FlashRAM to be rejected")
	}
	if result.RejectReason != "n64 flashram payload is blank (all 0xFF)" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestNormalizeSaveInputAcceptsStructuredN64SaveWithInspection(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Banjo-Kazooie (USA).eep",
		Payload:             buildTestN64Payload("eep", "banjo-kazooie"),
		Game:                game{Name: "Banjo-Kazooie"},
		SystemSlug:          "n64",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected N64 save to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.SystemSlug != "n64" {
		t.Fatalf("expected n64 system slug, got %q", result.Input.SystemSlug)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected N64 inspection metadata")
	}
	if result.Input.Inspection.ParserLevel != saveParserLevelContainer {
		t.Fatalf("expected parser level %q, got %q", saveParserLevelContainer, result.Input.Inspection.ParserLevel)
	}
	if result.Input.Inspection.ParserID != "n64-save-media" {
		t.Fatalf("unexpected parser id: %q", result.Input.Inspection.ParserID)
	}
	if result.Input.Inspection.ValidatedSystem != "n64" {
		t.Fatalf("unexpected validated system: %q", result.Input.Inspection.ValidatedSystem)
	}
	if result.Input.Game.HasParser {
		t.Fatal("expected N64 media validator to stay below structural parser level")
	}
	if result.Input.Inspection.TrustLevel != n64TrustLevelMediaOnly {
		t.Fatalf("expected media-only trust for generic N64 save, got %q", result.Input.Inspection.TrustLevel)
	}
}

func TestNormalizeSaveInputRecognizesOOTWordSwappedSRAM(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Legend of Zelda, The - Ocarina of Time (USA).sra",
		Payload:             buildOOTWordSwappedHeaderOnlyFixture(),
		Game:                game{Name: "Legend of Zelda, The - Ocarina of Time"},
		SystemSlug:          "n64",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected OOT SRAM to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected OOT inspection metadata")
	}
	if result.Input.Inspection.ParserID != "n64-oot-sram" {
		t.Fatalf("unexpected parser id: %q", result.Input.Inspection.ParserID)
	}
	if result.Input.Inspection.ValidatedGameTitle != "The Legend of Zelda: Ocarina of Time" {
		t.Fatalf("unexpected validated game title: %+v", result.Input.Inspection)
	}
	if result.Input.Inspection.TrustLevel != n64TrustLevelGameValidated {
		t.Fatalf("unexpected trust level: %+v", result.Input.Inspection)
	}
	if result.Input.Inspection.ParserLevel != saveParserLevelStructural {
		t.Fatalf("unexpected parser level: %+v", result.Input.Inspection)
	}
	if !result.Input.Game.HasParser {
		t.Fatal("expected OOT parser to mark save as parsed")
	}
	if got, ok := result.Input.Inspection.SemanticFields["wordSwapped"].(bool); !ok || !got {
		t.Fatalf("expected wordSwapped semantic field, got %+v", result.Input.Inspection.SemanticFields)
	}
}

func TestNormalizeSaveInputRecognizesOOTSemanticSlotValidation(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Legend of Zelda, The - Ocarina of Time (USA).sra",
		Payload:             buildOOTWordSwappedPopulatedFixture(),
		Game:                game{Name: "Legend of Zelda, The - Ocarina of Time"},
		SystemSlug:          "n64",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected populated OOT SRAM to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected populated OOT inspection metadata")
	}
	if result.Input.Inspection.TrustLevel != n64TrustLevelSemanticVerified {
		t.Fatalf("unexpected trust level: %+v", result.Input.Inspection)
	}
	if result.Input.Inspection.ParserLevel != saveParserLevelSemantic {
		t.Fatalf("unexpected parser level: %+v", result.Input.Inspection)
	}
	if result.Input.Inspection.ChecksumValid == nil || !*result.Input.Inspection.ChecksumValid {
		t.Fatalf("expected checksumValid=true, got %+v", result.Input.Inspection.ChecksumValid)
	}
	if !reflect.DeepEqual(result.Input.Inspection.ActiveSlotIndexes, []int{1}) {
		t.Fatalf("unexpected active slots: %+v", result.Input.Inspection.ActiveSlotIndexes)
	}
}

func TestNormalizeSaveInputRecognizesYoshisStoryMirroredEEPROM(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Yoshi's Story (USA) (En,Ja).eep",
		Payload:             buildYoshisStoryFixture(),
		Game:                game{Name: "Yoshi's Story"},
		SystemSlug:          "n64",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected Yoshi's Story EEPROM to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected Yoshi inspection metadata")
	}
	if result.Input.Inspection.ParserID != "n64-yoshis-story-eeprom" {
		t.Fatalf("unexpected parser id: %q", result.Input.Inspection.ParserID)
	}
	if result.Input.Inspection.ValidatedGameTitle != "Yoshi's Story" {
		t.Fatalf("unexpected validated title: %+v", result.Input.Inspection)
	}
	if result.Input.Inspection.TrustLevel != n64TrustLevelGameValidated {
		t.Fatalf("unexpected trust level: %+v", result.Input.Inspection)
	}
	if result.Input.Inspection.ParserLevel != saveParserLevelStructural {
		t.Fatalf("unexpected parser level: %+v", result.Input.Inspection)
	}
	if !reflect.DeepEqual(result.Input.Inspection.ActiveSlotIndexes, []int{1, 2}) {
		t.Fatalf("unexpected mirrored copy indexes: %+v", result.Input.Inspection.ActiveSlotIndexes)
	}
}

func TestRescanSavesPrunesBlankN64SaveMedia(t *testing.T) {
	h := newContractHarness(t)

	created, err := h.app.createSave(saveCreateInput{
		Filename:            "Star Fox 64 (USA).eep",
		Payload:             buildTestN64Payload("eep", "star-fox"),
		Game:                game{Name: "Star Fox 64"},
		ROMSHA1:             "star-fox-rom",
		SlotName:            "default",
		SystemSlug:          "n64",
		TrustedHelperSystem: true,
	})
	if err != nil {
		t.Fatalf("create N64 save: %v", err)
	}

	if err := os.WriteFile(created.payloadPath, make([]byte, 512), 0o644); err != nil {
		t.Fatalf("overwrite payload with blank media: %v", err)
	}
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload saves after tamper: %v", err)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan N64 saves: %v", err)
	}
	if result.Removed < 1 {
		t.Fatalf("expected blank N64 save to be removed, got %+v", result)
	}
	found := false
	for _, rejection := range result.Rejections {
		if rejection.SaveID != created.Summary.ID {
			continue
		}
		found = true
		if !strings.Contains(rejection.Reason, "blank") {
			t.Fatalf("expected blank-media rejection reason, got %+v", rejection)
		}
	}
	if !found {
		t.Fatalf("expected rejection entry for %s, got %+v", created.Summary.ID, result.Rejections)
	}
}

const (
	ootFixtureSlotStart   = 0x20
	ootFixtureSlotStride  = 0x1450
	ootFixtureSaveSize    = 0x1354
	ootFixtureChecksumOff = 0x1352
)

func buildOOTWordSwappedHeaderOnlyFixture() []byte {
	payload := make([]byte, ootSramSize)
	copy(payload[:len(ootHeaderMagic)], ootHeaderMagic)
	return n64Swap32Words(payload)
}

func buildOOTWordSwappedPopulatedFixture() []byte {
	payload := make([]byte, ootSramSize)
	copy(payload[:len(ootHeaderMagic)], ootHeaderMagic)
	slot := make([]byte, ootFixtureSaveSize)
	copy(slot[0x1C:0x22], []byte("ZELDAZ"))
	slot[0x24] = 0x12
	slot[0x25] = 0x34
	slot[0x2E] = 0x00
	slot[0x2F] = 0x30
	checksum := ootChecksum(slot)
	binary.BigEndian.PutUint16(slot[ootFixtureChecksumOff:ootFixtureChecksumOff+2], checksum)
	copy(payload[ootFixtureSlotStart:ootFixtureSlotStart+ootFixtureSaveSize], slot)
	copy(payload[ootFixtureSlotStart+ootFixtureSlotStride*3:ootFixtureSlotStart+ootFixtureSlotStride*3+ootFixtureSaveSize], slot)
	return n64Swap32Words(payload)
}

func ootChecksum(block []byte) uint16 {
	scratch := append([]byte(nil), block...)
	scratch[ootFixtureChecksumOff] = 0
	scratch[ootFixtureChecksumOff+1] = 0
	var checksum uint32
	for i := 0; i < len(scratch); i += 2 {
		checksum += uint32(binary.BigEndian.Uint16(scratch[i : i+2]))
	}
	return uint16(checksum & 0xffff)
}

func buildYoshisStoryFixture() []byte {
	payload := make([]byte, yoshisStoryEEPROMSize)
	buffer := make([]byte, yoshisStoryBufferSize)
	binary.BigEndian.PutUint32(buffer[yoshisStoryMagicOffset:yoshisStoryMagicOffset+4], yoshisStoryMagic)
	copy(payload[:yoshisStoryBufferSize], buffer)
	copy(payload[yoshisStoryBufferSize:], buffer)
	return payload
}
