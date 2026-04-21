package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	if _, err := os.Stat(noiseDir); !os.IsNotExist(err) {
		t.Fatalf("expected noise dir removed, stat err=%v", err)
	}
}
