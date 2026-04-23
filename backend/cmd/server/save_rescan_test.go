package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRescanSavesRehydratesSupportedSystem(t *testing.T) {
	h := newContractHarness(t)
	records := h.app.snapshotSaveRecords()
	if len(records) == 0 {
		t.Fatal("expected seeded save record")
	}

	record := records[0]
	record.SystemSlug = "unknown-system"
	record.Summary.SystemSlug = "unknown-system"
	if err := persistSaveRecordMetadata(record); err != nil {
		t.Fatalf("persist tampered metadata: %v", err)
	}
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload saves after tamper: %v", err)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan saves: %v", err)
	}
	if result.Rejected != 0 {
		t.Fatalf("expected no rejected saves during rehydrate, got %+v", result)
	}

	records = h.app.snapshotSaveRecords()
	if len(records) == 0 {
		t.Fatal("expected saves to remain after rescan")
	}
	if records[0].SystemSlug != "gameboy" {
		t.Fatalf("expected gameboy system slug after rescan, got %q", records[0].SystemSlug)
	}
}

func TestRescanSavesPrunesUnsupportedNoise(t *testing.T) {
	h := newContractHarness(t)
	records := h.app.snapshotSaveRecords()
	if len(records) == 0 {
		t.Fatal("expected seeded save record")
	}

	seed := records[0]
	noiseDir := filepath.Join(filepath.Dir(seed.dirPath), "Noise", "Unsupported", "noise-save-1")
	if err := os.MkdirAll(noiseDir, 0o755); err != nil {
		t.Fatalf("mkdir noise dir: %v", err)
	}
	payloadPath := filepath.Join(noiseDir, "payload.txt")
	if err := os.WriteFile(payloadPath, []byte("this is not a save"), 0o644); err != nil {
		t.Fatalf("write noise payload: %v", err)
	}

	noise := seed
	noise.dirPath = noiseDir
	noise.payloadPath = payloadPath
	noise.PayloadFile = "payload.txt"
	noise.SystemSlug = "unknown-system"
	noise.SystemPath = "Noise"
	noise.GamePath = "Unsupported"
	noise.GameSlug = "noise"
	noise.Summary.ID = "noise-save-1"
	noise.Summary.Filename = "notes.txt"
	noise.Summary.Format = "txt"
	noise.Summary.SystemSlug = "unknown-system"
	noise.Summary.Game.System = nil

	metadataPath := filepath.Join(noiseDir, "metadata.json")
	metadataBytes, err := json.MarshalIndent(noise, "", "  ")
	if err != nil {
		t.Fatalf("marshal noise metadata: %v", err)
	}
	if err := os.WriteFile(metadataPath, metadataBytes, 0o644); err != nil {
		t.Fatalf("write noise metadata: %v", err)
	}

	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload with noise: %v", err)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan saves: %v", err)
	}
	if result.Removed < 1 {
		t.Fatalf("expected at least one removed save, got %+v", result)
	}
	if len(result.Rejections) == 0 || result.Rejections[0].Reason == "" {
		t.Fatalf("expected rejection reasons for pruned saves, got %+v", result.Rejections)
	}
	if _, err := os.Stat(noiseDir); !os.IsNotExist(err) {
		t.Fatalf("expected noise dir removed, stat err=%v", err)
	}
}

func TestRescanSavesPrunesFalsePositiveMemoryCardRecord(t *testing.T) {
	h := newContractHarness(t)
	records := h.app.snapshotSaveRecords()
	if len(records) == 0 {
		t.Fatal("expected seeded save record")
	}

	seed := records[0]
	badDir := filepath.Join(filepath.Dir(seed.dirPath), "Sony", "Memory Card 1", "ps3logo-save-1")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("mkdir bad dir: %v", err)
	}
	payloadPath := filepath.Join(badDir, "payload.dat")
	if err := os.WriteFile(payloadPath, []byte{0x01, 0x02, 0x03, 0x04}, 0o644); err != nil {
		t.Fatalf("write bad payload: %v", err)
	}

	bad := seed
	bad.dirPath = badDir
	bad.payloadPath = payloadPath
	bad.PayloadFile = "payload.dat"
	bad.SystemSlug = "ps3"
	bad.SystemPath = "Sony"
	bad.GamePath = "Memory Card 1"
	bad.GameSlug = "memory-card-1"
	bad.Summary.ID = "ps3logo-save-1"
	bad.Summary.Filename = "PS3LOGO.DAT"
	bad.Summary.DisplayTitle = "Memory Card 1"
	bad.Summary.Format = "dat"
	bad.Summary.SystemSlug = "ps3"
	bad.Summary.Game = game{
		ID:           999,
		Name:         "Memory Card 1",
		DisplayTitle: "Memory Card 1",
		System:       &system{ID: 900008, Name: "PlayStation 3", Slug: "ps3"},
	}
	bad.Summary.MemoryCard = &memoryCardDetails{Name: "Memory Card 1"}

	metadataPath := filepath.Join(badDir, "metadata.json")
	metadataBytes, err := json.MarshalIndent(bad, "", "  ")
	if err != nil {
		t.Fatalf("marshal false positive metadata: %v", err)
	}
	if err := os.WriteFile(metadataPath, metadataBytes, 0o644); err != nil {
		t.Fatalf("write false positive metadata: %v", err)
	}

	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload with false positive card: %v", err)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan false positive card: %v", err)
	}
	if result.Removed < 1 {
		t.Fatalf("expected false positive memory card to be removed, got %+v", result)
	}
	if _, err := os.Stat(badDir); !os.IsNotExist(err) {
		t.Fatalf("expected false positive dir removed, stat err=%v", err)
	}
}

func TestRescanSavesPrunesPlayStationSaveStateNoise(t *testing.T) {
	h := newContractHarness(t)
	records := h.app.snapshotSaveRecords()
	if len(records) == 0 {
		t.Fatal("expected seeded save record")
	}

	seed := records[0]
	badDir := filepath.Join(filepath.Dir(seed.dirPath), "PlayStation", "Memory Card 1", "psx-state-noise")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("mkdir bad dir: %v", err)
	}
	payloadPath := filepath.Join(badDir, "payload.ss")
	if err := os.WriteFile(payloadPath, make([]byte, 4*1024*1024), 0o644); err != nil {
		t.Fatalf("write bad payload: %v", err)
	}

	bad := seed
	bad.dirPath = badDir
	bad.payloadPath = payloadPath
	bad.PayloadFile = "payload.ss"
	bad.SystemSlug = "psx"
	bad.SystemPath = "PlayStation"
	bad.GamePath = "Memory Card 1"
	bad.GameSlug = "memory-card-1"
	bad.Summary.ID = "psx-state-noise"
	bad.Summary.Filename = "Castlevania - Symphony of the Night (USA)_1.ss"
	bad.Summary.DisplayTitle = "Memory Card 1"
	bad.Summary.Format = "ss"
	bad.Summary.SystemSlug = "psx"
	bad.Summary.Game = game{
		ID:           999,
		Name:         "Memory Card 1",
		DisplayTitle: "Memory Card 1",
		System:       &system{ID: 27, Name: "PlayStation", Slug: "psx"},
	}
	bad.Summary.MemoryCard = &memoryCardDetails{Name: "Memory Card 1"}

	metadataPath := filepath.Join(badDir, "metadata.json")
	metadataBytes, err := json.MarshalIndent(bad, "", "  ")
	if err != nil {
		t.Fatalf("marshal save state metadata: %v", err)
	}
	if err := os.WriteFile(metadataPath, metadataBytes, 0o644); err != nil {
		t.Fatalf("write save state metadata: %v", err)
	}

	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload with save state noise: %v", err)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan PS save state noise: %v", err)
	}
	if result.Removed < 1 {
		t.Fatalf("expected PS save state noise to be removed, got %+v", result)
	}
	if _, err := os.Stat(badDir); !os.IsNotExist(err) {
		t.Fatalf("expected PS save state dir removed, stat err=%v", err)
	}
}

func TestRescanSavesReclassifiesNintendoDSStoredFallbackWhenPayloadMatches(t *testing.T) {
	h := newContractHarness(t)

	record, err := h.app.createSave(saveCreateInput{
		Filename:            "New Super Mario Bros. (USA).sav",
		Payload:             buildNSMBNDSSaveFixture(),
		Game:                game{Name: "New Super Mario Bros."},
		Format:              "sram",
		ROMSHA1:             "nds-rom-sha1",
		ROMMD5:              "nds-rom-md5",
		SlotName:            "default",
		SystemSlug:          "gameboy",
		GameSlug:            "new-super-mario-bros",
		TrustedHelperSystem: true,
	})
	if err != nil {
		t.Fatalf("create stale ds save: %v", err)
	}
	record.SystemSlug = "gameboy"
	record.SystemPath = "Nintendo Game Boy"
	record.GamePath = "New Super Mario Bros"
	record.GameSlug = "new-super-mario-bros"
	record.Summary.SystemSlug = "gameboy"
	record.Summary.Game.System = &system{ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy", Manufacturer: "Nintendo"}
	record.Summary.Metadata = nil
	if err := persistSaveRecordMetadata(record); err != nil {
		t.Fatalf("persist stale ds metadata: %v", err)
	}

	h.app.mu.Lock()
	for i := range h.app.saveRecords {
		if h.app.saveRecords[i].Summary.ID == record.Summary.ID {
			h.app.saveRecords[i] = record
			break
		}
	}
	for i := range h.app.saves {
		if h.app.saves[i].ID == record.Summary.ID {
			h.app.saves[i] = record.Summary
			break
		}
	}
	h.app.mu.Unlock()

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan ds save: %v", err)
	}
	if result.Updated < 1 {
		t.Fatalf("expected stale DS fallback save to be reclassified, got %+v", result)
	}

	records := h.app.snapshotSaveRecords()
	for _, candidate := range records {
		if candidate.Summary.DisplayTitle != "New Super Mario Bros." {
			continue
		}
		if candidate.SystemSlug != "nds" {
			t.Fatalf("expected NSMB save to be reclassified to nds, got %+v", candidate)
		}
		if candidate.Summary.Inspection == nil || candidate.Summary.Inspection.ParserID != "nds-new-super-mario-bros" {
			t.Fatalf("expected NSMB DS inspection metadata, got %+v", candidate.Summary.Inspection)
		}
		return
	}
	t.Fatalf("expected stale DS record %s after rescan", record.Summary.ID)
}

func TestRescanSavesPrunesGenericStoredFallbackSlots(t *testing.T) {
	h := newContractHarness(t)
	records := h.app.snapshotSaveRecords()
	if len(records) == 0 {
		t.Fatal("expected seeded save record")
	}

	seed := records[0]
	cases := []struct {
		id       string
		title    string
		filename string
	}{
		{id: "nds-autosave-1", title: "Autosave6", filename: "Autosave6.sav"},
		{id: "nds-slot-1", title: "01 - Channel 9 Headquarters", filename: "01 - Channel 9 Headquarters.sav"},
	}

	for _, tc := range cases {
		dir := filepath.Join(filepath.Dir(seed.dirPath), "Nintendo DS", tc.title, tc.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", tc.id, err)
		}
		payloadPath := filepath.Join(dir, "payload.sav")
		if err := os.WriteFile(payloadPath, make([]byte, 8192), 0o644); err != nil {
			t.Fatalf("write %s payload: %v", tc.id, err)
		}

		record := seed
		record.dirPath = dir
		record.payloadPath = payloadPath
		record.PayloadFile = "payload.sav"
		record.SystemSlug = "nds"
		record.SystemPath = "Nintendo DS"
		record.GamePath = tc.title
		record.GameSlug = canonicalSegment(tc.title, "unknown-game")
		record.Summary.ID = tc.id
		record.Summary.Filename = tc.filename
		record.Summary.DisplayTitle = tc.title
		record.Summary.SystemSlug = "nds"
		record.Summary.Metadata = nil
		record.Summary.Game = game{
			ID:           deterministicGameID(tc.title),
			Name:         tc.title,
			DisplayTitle: tc.title,
			System:       &system{ID: 900004, Name: "Nintendo DS", Slug: "nds"},
		}

		metadataBytes, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s metadata: %v", tc.id, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "metadata.json"), metadataBytes, 0o644); err != nil {
			t.Fatalf("write %s metadata: %v", tc.id, err)
		}
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan generic fallback slots: %v", err)
	}
	if result.Removed < len(cases) {
		t.Fatalf("expected at least %d removed fallback slot saves, got %+v", len(cases), result)
	}

	for _, tc := range cases {
		dir := filepath.Join(filepath.Dir(seed.dirPath), "Nintendo DS", tc.title, tc.id)
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be pruned, stat err=%v", tc.id, err)
		}
	}
}

func TestRescanSavesPrunesStoredFallbackArcadeGuesses(t *testing.T) {
	h := newContractHarness(t)
	records := h.app.snapshotSaveRecords()
	if len(records) == 0 {
		t.Fatal("expected seeded save record")
	}

	seed := records[0]
	cases := []struct {
		id       string
		title    string
		filename string
		ext      string
		size     int
	}{
		{id: "arcade-daytona-1", title: "daytona", filename: "daytona.ram", ext: "ram", size: 16384},
		{id: "arcade-ghost-house-1", title: "Ghost House", filename: "Ghost House.sav", ext: "sav", size: 16384},
	}

	for _, tc := range cases {
		dir := filepath.Join(filepath.Dir(seed.dirPath), "Arcade", tc.title, tc.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", tc.id, err)
		}
		payloadPath := filepath.Join(dir, "payload."+tc.ext)
		if err := os.WriteFile(payloadPath, make([]byte, tc.size), 0o644); err != nil {
			t.Fatalf("write %s payload: %v", tc.id, err)
		}

		record := seed
		record.dirPath = dir
		record.payloadPath = payloadPath
		record.PayloadFile = "payload." + tc.ext
		record.SystemSlug = "arcade"
		record.SystemPath = "Arcade"
		record.GamePath = tc.title
		record.GameSlug = canonicalSegment(tc.title, "unknown-game")
		record.Summary.ID = tc.id
		record.Summary.Filename = tc.filename
		record.Summary.DisplayTitle = tc.title
		record.Summary.Format = inferSaveFormat(tc.filename)
		record.Summary.SystemSlug = "arcade"
		record.Summary.Metadata = nil
		record.Summary.Game = game{
			ID:           deterministicGameID(tc.title),
			Name:         tc.title,
			DisplayTitle: tc.title,
			System:       &system{ID: 900001, Name: "Arcade", Slug: "arcade"},
		}

		metadataBytes, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s metadata: %v", tc.id, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "metadata.json"), metadataBytes, 0o644); err != nil {
			t.Fatalf("write %s metadata: %v", tc.id, err)
		}
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan fallback arcade guesses: %v", err)
	}
	if result.Removed < len(cases) {
		t.Fatalf("expected at least %d removed arcade guesses, got %+v", len(cases), result)
	}

	for _, tc := range cases {
		dir := filepath.Join(filepath.Dir(seed.dirPath), "Arcade", tc.title, tc.id)
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be pruned, stat err=%v", tc.id, err)
		}
	}
}

func TestRescanSavesPrunesBlankTrustedNeoGeoSave(t *testing.T) {
	h := newContractHarness(t)
	record, err := h.app.createSave(saveCreateInput{
		Filename:            "doubledr.sav",
		Payload:             buildValidNeoGeoCompoundPayload(),
		Game:                game{Name: "doubledr"},
		Format:              "sram",
		ROMSHA1:             "neogeo-doubledr-rom",
		SlotName:            "default",
		SystemSlug:          "neogeo",
		GameSlug:            "doubledr",
		TrustedHelperSystem: true,
	})
	if err != nil {
		t.Fatalf("create trusted neo geo save: %v", err)
	}
	blank := make([]byte, neoGeoCompoundSaveSize)
	for i := range blank {
		blank[i] = 0xff
	}
	if err := os.WriteFile(record.payloadPath, blank, 0o644); err != nil {
		t.Fatalf("overwrite neo geo payload with blank media: %v", err)
	}
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload neo geo saves: %v", err)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan blank neo geo save: %v", err)
	}
	if result.Removed < 1 {
		t.Fatalf("expected blank neo geo save to be pruned, got %+v", result)
	}
	for _, candidate := range h.app.snapshotSaveRecords() {
		if candidate.Summary.ID == record.Summary.ID {
			t.Fatalf("expected blank neo geo save to be pruned, still found %+v", candidate)
		}
	}
	foundRejection := false
	for _, rejection := range result.Rejections {
		if rejection.SaveID != record.Summary.ID {
			continue
		}
		foundRejection = true
		if !strings.Contains(strings.ToLower(rejection.Reason), "neo geo") {
			t.Fatalf("expected neo geo rejection reason, got %+v", rejection)
		}
	}
	if !foundRejection {
		t.Fatalf("expected rejection entry for %s, got %+v", record.Summary.ID, result.Rejections)
	}
}

func TestRescanSavesRebuildsGenesisInspectionFromTrustedMetadata(t *testing.T) {
	h := newContractHarness(t)

	record, err := h.app.createSave(saveCreateInput{
		Filename:            "Sonic the Hedgehog.srm",
		Payload:             make([]byte, 8192),
		Game:                game{Name: "Sonic the Hedgehog"},
		Format:              "sram",
		ROMSHA1:             "genesis-sonic-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "genesis",
		GameSlug:            "sonic-the-hedgehog",
		TrustedHelperSystem: true,
	})
	if err != nil {
		t.Fatalf("create trusted Genesis save: %v", err)
	}
	record.Summary.Inspection = nil
	if err := persistSaveRecordMetadata(record); err != nil {
		t.Fatalf("persist Genesis metadata without inspection: %v", err)
	}
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload Genesis save: %v", err)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan Genesis save: %v", err)
	}
	if result.Updated < 1 {
		t.Fatalf("expected Genesis save inspection to be rebuilt, got %+v", result)
	}

	records := h.app.snapshotSaveRecords()
	for _, candidate := range records {
		if candidate.Summary.ID != record.Summary.ID {
			continue
		}
		if candidate.Summary.Inspection == nil {
			t.Fatalf("expected rebuilt Genesis inspection metadata, got %+v", candidate.Summary)
		}
		if candidate.Summary.Inspection.ParserID != "sega-raw-sram" {
			t.Fatalf("unexpected Genesis inspection payload: %+v", candidate.Summary.Inspection)
		}
		return
	}
	t.Fatalf("expected Genesis record %s after rescan", record.Summary.ID)
}

func TestRescanSavesPrunesTrustedGenesisRawSaveWithoutROMSHA1(t *testing.T) {
	h := newContractHarness(t)

	record, err := h.app.createSave(saveCreateInput{
		Filename:            "Sonic the Hedgehog.srm",
		Payload:             make([]byte, 8192),
		Game:                game{Name: "Sonic the Hedgehog"},
		Format:              "sram",
		ROMSHA1:             "genesis-sonic-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "genesis",
		GameSlug:            "sonic-the-hedgehog",
		TrustedHelperSystem: true,
	})
	if err != nil {
		t.Fatalf("create trusted Genesis save: %v", err)
	}
	record.ROMSHA1 = ""
	if err := persistSaveRecordMetadata(record); err != nil {
		t.Fatalf("persist Genesis metadata without romSha1: %v", err)
	}
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload Genesis save without romSha1: %v", err)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan Genesis save without romSha1: %v", err)
	}
	if result.Removed < 1 {
		t.Fatalf("expected trusted Genesis save without romSha1 to be pruned, got %+v", result)
	}

	for _, candidate := range h.app.snapshotSaveRecords() {
		if candidate.Summary.ID == record.Summary.ID {
			t.Fatalf("expected Genesis save without romSha1 to be pruned, still found %+v", candidate)
		}
	}
	for _, rejection := range result.Rejections {
		if rejection.SaveID != record.Summary.ID {
			continue
		}
		if rejection.Reason != "genesis raw saves require rom_sha1" {
			t.Fatalf("unexpected Genesis prune reason: %q", rejection.Reason)
		}
		return
	}
	t.Fatalf("expected rejection entry for pruned Genesis save, got %+v", result.Rejections)
}
