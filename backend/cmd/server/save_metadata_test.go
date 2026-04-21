package main

import (
	"strings"
	"testing"
)

func TestCleanupDisplayTitleAndRegion(t *testing.T) {
	title, region := cleanupDisplayTitleAndRegion("Yoshi's Story (USA) (En,Ja)")
	if title != "Yoshi's Story" {
		t.Fatalf("unexpected title: %q", title)
	}
	if region != regionUS {
		t.Fatalf("unexpected region: %q", region)
	}
}

func TestCleanupDisplayTitleRegionAndLanguages(t *testing.T) {
	title, region, languages := cleanupDisplayTitleRegionAndLanguages("Yoshi's Story (USA) (En,Ja)")
	if title != "Yoshi's Story" {
		t.Fatalf("unexpected title: %q", title)
	}
	if region != regionUS {
		t.Fatalf("unexpected region: %q", region)
	}
	if len(languages) != 2 || languages[0] != "EN" || languages[1] != "JA" {
		t.Fatalf("unexpected languages: %#v", languages)
	}
}

func TestParsePlayStationMemoryCard(t *testing.T) {
	payload := make([]byte, psMemoryCardBlockSize*2)
	dirOffset := psDirectoryEntrySize // slot 1
	payload[dirOffset] = 0x51
	copy(payload[dirOffset+0x0a:dirOffset+0x16], []byte("SCUS_941.63"))

	blockOffset := psMemoryCardBlockSize
	copy(payload[blockOffset+16:blockOffset+64], []byte("Final Fantasy VII Save"))

	card := parsePlayStationMemoryCard(payload, "Memory Card 1")
	if card == nil {
		t.Fatal("expected memory card details")
	}
	if card.Name != "Memory Card 1" {
		t.Fatalf("unexpected card name: %q", card.Name)
	}
	if len(card.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(card.Entries))
	}

	entry := card.Entries[0]
	if entry.Slot != 1 {
		t.Fatalf("unexpected slot: %d", entry.Slot)
	}
	if entry.RegionCode != regionUS {
		t.Fatalf("unexpected region: %q", entry.RegionCode)
	}
	if !strings.Contains(strings.ToLower(entry.Title), "final") {
		t.Fatalf("unexpected title: %q", entry.Title)
	}
}

func TestNormalizeSaveInputSetsDisplayAndCardPath(t *testing.T) {
	a := &app{}
	regular := a.normalizeSaveInput(saveCreateInput{
		Filename: "Yoshi Story (USA).eep",
		Payload:  []byte("save-data"),
		Game:     game{Name: "Yoshi Story (USA)"},
	})
	if regular.DisplayTitle != "Yoshi Story" {
		t.Fatalf("unexpected display title: %q", regular.DisplayTitle)
	}
	if regular.RegionCode != regionUS {
		t.Fatalf("unexpected region code: %q", regular.RegionCode)
	}
	if len(regular.LanguageCodes) != 0 {
		t.Fatalf("expected no language codes, got %#v", regular.LanguageCodes)
	}
	if regular.GamePath != "Yoshi Story" {
		t.Fatalf("unexpected game path: %q", regular.GamePath)
	}

	cardPayload := make([]byte, psMemoryCardBlockSize*2)
	cardOffset := psDirectoryEntrySize
	cardPayload[cardOffset] = 0x51
	copy(cardPayload[cardOffset+0x0a:cardOffset+0x16], []byte("SLES_123.45"))
	ps := a.normalizeSaveInput(saveCreateInput{
		Filename: "memory_card_1.mcr",
		Payload:  cardPayload,
		Game:     game{Name: "Final Fantasy VII (Europe)"},
	})
	if ps.GamePath != "Memory Card 1" {
		t.Fatalf("unexpected ps game path: %q", ps.GamePath)
	}
	if ps.MemoryCard == nil {
		t.Fatal("expected memory card details")
	}
	if ps.MemoryCard.Name != "Memory Card 1" {
		t.Fatalf("unexpected memory card name: %q", ps.MemoryCard.Name)
	}
}

func TestNormalizeSaveInputDetectsLanguageAndRegionFromPayload(t *testing.T) {
	a := &app{}
	input := a.normalizeSaveInput(saveCreateInput{
		Filename: "Yoshi's Story (En,Ja).eep",
		Payload:  []byte("header-SCUS_123.45-footer"),
		Game:     game{Name: "Yoshi's Story (En,Ja)"},
	})
	if input.DisplayTitle != "Yoshi's Story" {
		t.Fatalf("unexpected display title: %q", input.DisplayTitle)
	}
	if input.RegionCode != regionUS {
		t.Fatalf("expected US region from payload product code, got %q", input.RegionCode)
	}
	if len(input.LanguageCodes) != 2 || input.LanguageCodes[0] != "EN" || input.LanguageCodes[1] != "JA" {
		t.Fatalf("unexpected language codes: %#v", input.LanguageCodes)
	}
}
