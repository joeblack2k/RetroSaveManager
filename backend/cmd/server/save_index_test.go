package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveRecordIndexLookupParity(t *testing.T) {
	root := t.TempDir()
	older := indexedSaveRecord(t, root, "save-old", "rom-a", "slot-1", "Pokemon Noise Loop.srm", "Pokemon Noise Loop", time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), true)
	latestReadable := indexedSaveRecord(t, root, "save-readable", "rom-a", "slot-1", "Pokemon Noise Loop.srm", "Pokemon Noise Loop", time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC), true)
	brokenLatest := indexedSaveRecord(t, root, "save-broken", "rom-a", "slot-1", "Pokemon Noise Loop.srm", "Pokemon Noise Loop", time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC), false)

	index := buildSaveRecordIndex([]saveRecord{older, latestReadable, brokenLatest})

	if got, ok := index.recordByID("save-readable"); !ok || got.Summary.ID != "save-readable" {
		t.Fatalf("recordByID mismatch: ok=%v id=%q", ok, got.Summary.ID)
	}
	if got, ok := index.latestByROMSlot("rom-a", "slot-1"); !ok || got.Summary.ID != "save-broken" {
		t.Fatalf("latestByROMSlot should include metadata-only records: ok=%v id=%q", ok, got.Summary.ID)
	}
	if got, ok := index.latestReadableByROMSlot("rom-a", "slot-1"); !ok || got.Summary.ID != "save-readable" {
		t.Fatalf("latestReadableByROMSlot should skip missing payloads: ok=%v id=%q", ok, got.Summary.ID)
	}
	if got, ok := index.latestReadableByTrackContext("Pokemon Noise Loop.srm", "gameboy", "Pokemon Noise Loop", "US", "", ""); !ok || got.Summary.ID != "save-readable" {
		t.Fatalf("latestReadableByTrackContext should match canonical track: ok=%v id=%q", ok, got.Summary.ID)
	}

	selected := index.recordsByIDs([]string{"save-old", "save-readable", "save-old", "missing"})
	if len(selected) != 2 {
		t.Fatalf("recordsByIDs should dedupe known ids, got %d", len(selected))
	}
	if selected[0].Summary.ID != "save-readable" || selected[1].Summary.ID != "save-old" {
		t.Fatalf("recordsByIDs should sort newest first, got %q then %q", selected[0].Summary.ID, selected[1].Summary.ID)
	}
}

func indexedSaveRecord(t *testing.T, root, id, romSHA1, slotName, filename, title string, createdAt time.Time, payloadExists bool) saveRecord {
	t.Helper()

	payloadPath := filepath.Join(root, id+".sav")
	if payloadExists {
		if err := os.WriteFile(payloadPath, []byte(id), 0o644); err != nil {
			t.Fatalf("write payload: %v", err)
		}
	}

	return saveRecord{
		Summary: saveSummary{
			ID:           id,
			Game:         game{Name: title, DisplayTitle: title, RegionCode: "US", System: supportedSystemFromSlug("gameboy")},
			DisplayTitle: title,
			SystemSlug:   "gameboy",
			RegionCode:   "US",
			Filename:     filename,
			FileSize:     len(id),
			Version:      1,
			SHA256:       id + "-sha",
			CreatedAt:    createdAt,
		},
		ROMSHA1:     romSHA1,
		SlotName:    slotName,
		SystemSlug:  "gameboy",
		PayloadFile: filepath.Base(payloadPath),
		payloadPath: payloadPath,
	}
}
