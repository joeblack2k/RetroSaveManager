package main

import (
	"os"
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
		Filename:            "Mario Kart 64 (USA).eep",
		Payload:             buildTestN64Payload("eep", "mario-kart-64"),
		Game:                game{Name: "Mario Kart 64"},
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
