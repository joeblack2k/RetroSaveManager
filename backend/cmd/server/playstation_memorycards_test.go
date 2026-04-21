package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"testing"
)

func TestParsePS2MemoryCardListsOnlyRealSaveDirs(t *testing.T) {
	payload := mustDecodePS2Fixture(t)
	card := parsePlayStationMemoryCard(supportedSystemFromSlug("ps2"), payload, "Mcd001.ps2", "Memory Card 1")
	if card == nil {
		t.Fatal("expected PS2 memory card details")
	}
	if card.Name != "Memory Card 1" {
		t.Fatalf("unexpected card name: %q", card.Name)
	}
	if len(card.Entries) != 2 {
		t.Fatalf("expected 2 PS2 game entries, got %d", len(card.Entries))
	}

	if card.Entries[0].Title != "Burnout 3" {
		t.Fatalf("unexpected first PS2 title: %q", card.Entries[0].Title)
	}
	if card.Entries[1].Title != "Mortal Kombat Shaolin Monks" {
		t.Fatalf("unexpected second PS2 title: %q", card.Entries[1].Title)
	}
	for _, entry := range card.Entries {
		if entry.DirectoryName == "BADATA-SYSTEM" {
			t.Fatalf("system configuration entry leaked into PS2 card list: %#v", entry)
		}
		if entry.IconDataURL == "" {
			t.Fatalf("expected icon preview for %q", entry.Title)
		}
		if entry.SizeBytes <= 0 || entry.Blocks <= 0 {
			t.Fatalf("expected size and block stats for %q, got %+v", entry.Title, entry)
		}
	}
}

func mustDecodePS2Fixture(t *testing.T) []byte {
	t.Helper()
	compressed, err := os.ReadFile("testdata/ps2_memory_card_fixture.ps2.gz")
	if err != nil {
		t.Fatalf("read PS2 fixture: %v", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("open PS2 fixture gzip: %v", err)
	}
	defer zr.Close()
	payload, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read PS2 fixture payload: %v", err)
	}
	return payload
}
