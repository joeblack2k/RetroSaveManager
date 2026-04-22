package main

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestPlayStationProjectionImportCreatesCrossProfilePS1Records(t *testing.T) {
	h := newContractHarness(t)
	payload := makeTestPS1Card(t, "SCUS_941.63", "Final Fantasy VII Save")
	input := saveCreateInput{
		Filename: "memory_card_1.mcr",
		Payload:  payload,
		Game:     game{Name: "PlayStation Save", System: supportedSystemFromSlug("psx")},
		Format:   "mcr",
		SlotName: "Memory Card 1",
	}
	preview := h.app.normalizeSaveInputDetailed(input)
	record, conflict, err := h.app.createPlayStationProjectionSave(input, preview, "retroarch", "deck-psx")
	if err != nil {
		t.Fatalf("create playstation projection: %v", err)
	}
	if conflict != nil {
		t.Fatalf("expected no projection conflict, got %#v", conflict)
	}
	if record.Summary.RuntimeProfile != "psx/retroarch" {
		t.Fatalf("unexpected runtime profile: %q", record.Summary.RuntimeProfile)
	}
	if record.Summary.CardSlot != "Memory Card 1" {
		t.Fatalf("unexpected card slot: %q", record.Summary.CardSlot)
	}
	if record.Summary.MemoryCard == nil || len(record.Summary.MemoryCard.Entries) != 1 {
		t.Fatalf("expected single PS1 entry, got %#v", record.Summary.MemoryCard)
	}
	if !strings.Contains(strings.ToLower(record.Summary.MemoryCard.Entries[0].Title), "final") {
		t.Fatalf("unexpected PS1 title: %q", record.Summary.MemoryCard.Entries[0].Title)
	}

	store := h.app.playStationSyncStore()
	saveID, _, ok := store.latestProjectionSaveRecord("psx/retroarch", "Memory Card 1")
	if !ok || saveID != record.Summary.ID {
		t.Fatalf("expected retroarch projection latest id %q, got %q ok=%v", record.Summary.ID, saveID, ok)
	}
	misterID, _, ok := store.latestProjectionSaveRecord("psx/mister", "Memory Card 1")
	if !ok || strings.TrimSpace(misterID) == "" {
		t.Fatalf("expected sister MiSTer projection to be generated")
	}
}

func TestPlayStationProjectionImportBuildsPS2Projection(t *testing.T) {
	h := newContractHarness(t)
	payload := mustDecodePS2Fixture(t)
	input := saveCreateInput{
		Filename: "Mcd001.ps2",
		Payload:  payload,
		Game:     game{Name: "PCSX2 Card", System: supportedSystemFromSlug("ps2")},
		Format:   "ps2",
		SlotName: "Memory Card 1",
	}
	preview := h.app.normalizeSaveInputDetailed(input)
	record, conflict, err := h.app.createPlayStationProjectionSave(input, preview, "pcsx2", "deck-ps2")
	if err != nil {
		t.Fatalf("create ps2 projection: %v", err)
	}
	if conflict != nil {
		t.Fatalf("expected no projection conflict, got %#v", conflict)
	}
	if record.Summary.RuntimeProfile != "ps2/pcsx2" {
		t.Fatalf("unexpected runtime profile: %q", record.Summary.RuntimeProfile)
	}
	if record.Summary.MemoryCard == nil || len(record.Summary.MemoryCard.Entries) != 2 {
		t.Fatalf("expected two PS2 entries, got %#v", record.Summary.MemoryCard)
	}
	if record.Summary.MemoryCard.Entries[0].Title != "Burnout 3" {
		t.Fatalf("unexpected first PS2 title: %q", record.Summary.MemoryCard.Entries[0].Title)
	}
	if record.Summary.MemoryCard.Entries[1].Title != "Mortal Kombat Shaolin Monks" {
		t.Fatalf("unexpected second PS2 title: %q", record.Summary.MemoryCard.Entries[1].Title)
	}

	store := h.app.playStationSyncStore()
	projection, ok := store.projectionForRuntime("ps2/pcsx2", "Memory Card 1")
	if !ok {
		t.Fatal("expected latest PS2 projection")
	}
	if projection.SaveRecordID != record.Summary.ID {
		t.Fatalf("unexpected PS2 projection save record id: %q", projection.SaveRecordID)
	}
}

func TestConflictCheckUsesPlayStationProjectionIdentity(t *testing.T) {
	h := newContractHarness(t)
	romKey := projectionConflictKey("psx/retroarch", "Memory Card 1")
	h.app.reportConflict(romKey, "Memory Card 1", "local-sha", "cloud-sha", "RetroArch Deck", "memory_card_1.mcr", ps1MemoryCardTotalSize)

	rr := h.request(http.MethodGet, "/conflicts/check?slotName=Memory%20Card%201&device_type=retroarch&fingerprint=deck-psx", nil)
	assertStatus(t, rr, http.StatusOK)
	body := decodeJSONMap(t, rr.Body)
	if !mustBool(t, body["exists"], "exists") {
		t.Fatalf("expected playstation projection conflict to be found: %s", rr.Body.String())
	}
}

func TestPlayStationBackfillMigratesLegacyPS1RawSave(t *testing.T) {
	h := newContractHarness(t)
	payload := makeTestPS1Card(t, "SCUS_941.63", "Final Fantasy VII Save")
	raw, err := h.app.createSave(saveCreateInput{
		Filename:   "psx.sav",
		Payload:    payload,
		Game:       game{Name: "Legacy Card", System: supportedSystemFromSlug("psx")},
		Format:     "sram",
		SystemSlug: "psx",
		ROMSHA1:    "legacy-psx-rom",
		SlotName:   "default",
	})
	if err != nil {
		t.Fatalf("create raw legacy ps1 save: %v", err)
	}

	result, err := h.app.backfillPlayStation(playStationBackfillOptions{
		PSXProfile:         "psx/mister",
		DefaultPSXCardSlot: "Memory Card 1",
		ReplaceRaw:         true,
	})
	if err != nil {
		t.Fatalf("backfill playstation: %v", err)
	}
	if result.Migrated != 1 {
		t.Fatalf("expected 1 migrated record, got %+v", result)
	}
	if _, err := os.Stat(raw.dirPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy raw save dir to be removed, stat err=%v", err)
	}

	records := h.app.snapshotSaveRecords()
	foundPrimary := false
	foundMirror := false
	for _, record := range records {
		switch record.Summary.RuntimeProfile {
		case "psx/mister":
			foundPrimary = true
		case "psx/retroarch":
			foundMirror = true
		}
	}
	if !foundPrimary {
		t.Fatal("expected migrated psx/mister projection record")
	}
	if !foundMirror {
		t.Fatal("expected mirrored psx/retroarch projection record")
	}
}

func TestPlayStationBackfillRequiresExplicitPS1Profile(t *testing.T) {
	h := newContractHarness(t)
	payload := makeTestPS1Card(t, "SCUS_941.63", "Final Fantasy VII Save")
	_, err := h.app.createSave(saveCreateInput{
		Filename:   "psx.sav",
		Payload:    payload,
		Game:       game{Name: "Legacy Card", System: supportedSystemFromSlug("psx")},
		Format:     "sram",
		SystemSlug: "psx",
		ROMSHA1:    "legacy-psx-rom",
		SlotName:   "default",
	})
	if err != nil {
		t.Fatalf("create raw legacy ps1 save: %v", err)
	}

	result, err := h.app.backfillPlayStation(playStationBackfillOptions{ReplaceRaw: true})
	if err != nil {
		t.Fatalf("backfill playstation: %v", err)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected one failure for missing psx profile, got %+v", result)
	}
	if !strings.Contains(result.Failures[0].Reason, "--psx-profile") {
		t.Fatalf("unexpected failure reason: %+v", result.Failures[0])
	}
}

func TestPlayStationBackfillMigratesLegacyPS2RawSave(t *testing.T) {
	h := newContractHarness(t)
	payload := mustDecodePS2Fixture(t)
	raw, err := h.app.createSave(saveCreateInput{
		Filename:   "Mcd001.ps2",
		Payload:    payload,
		Game:       game{Name: "Legacy PS2 Card", System: supportedSystemFromSlug("ps2")},
		Format:     "ps2",
		SystemSlug: "ps2",
		ROMSHA1:    "legacy-ps2-rom",
		SlotName:   "default",
	})
	if err != nil {
		t.Fatalf("create raw legacy ps2 save: %v", err)
	}

	result, err := h.app.backfillPlayStation(playStationBackfillOptions{ReplaceRaw: true})
	if err != nil {
		t.Fatalf("backfill playstation: %v", err)
	}
	if result.Migrated != 1 {
		t.Fatalf("expected 1 migrated record, got %+v", result)
	}
	if _, err := os.Stat(raw.dirPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy raw ps2 save dir to be removed, stat err=%v", err)
	}

	records := h.app.snapshotSaveRecords()
	found := false
	for _, record := range records {
		if record.Summary.RuntimeProfile == "ps2/pcsx2" && record.Summary.ProjectionID != "" {
			found = true
			if record.Summary.MemoryCard == nil || len(record.Summary.MemoryCard.Entries) != 2 {
				t.Fatalf("expected PS2 projection memory card entries, got %#v", record.Summary.MemoryCard)
			}
		}
	}
	if !found {
		t.Fatal("expected migrated ps2/pcsx2 projection record")
	}
}

func makeTestPS1Card(t *testing.T, productCode, title string) []byte {
	t.Helper()
	payload := make([]byte, ps1MemoryCardTotalSize)
	copy(payload[:2], []byte("MC"))
	dirOffset := psDirectoryEntrySize
	payload[dirOffset] = ps1DirectoryStateFirst
	copy(payload[dirOffset+0x0a:dirOffset+0x16], []byte(productCode))
	updatePS1DirectoryChecksum(payload[dirOffset : dirOffset+psDirectoryEntrySize])
	blockOffset := psMemoryCardBlockSize
	payload[blockOffset+0x60] = 0x1F
	payload[blockOffset+0x61] = 0x00
	payload[blockOffset+0x80] = 0x11
	copy(payload[blockOffset+4:blockOffset+4+len(title)], []byte(title))
	return payload
}
