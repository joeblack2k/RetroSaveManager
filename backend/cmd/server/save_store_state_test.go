package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A rejected save is kept under `_rsm/quarantine`, and its metadata.json holds
// id, filename and payloadFile at the top level rather than under `summary`.
// The loader walks the whole store root, so each quarantine entry was indexed
// as a save with a blank id and name; a rescan then turned the blank name into
// save.bin, called it generic noise, and pruned the directory the save had been
// quarantined into. Nothing under the store's own state path is a save record.
func TestStoreDoesNotIndexQuarantinedRejections(t *testing.T) {
	h := newContractHarness(t)
	root := h.app.saveStore.root

	quarantineDir := filepath.Join(root, "_rsm", "quarantine", "20260101T000000.000000000Z-abcdef123456")
	if err := os.MkdirAll(quarantineDir, 0o755); err != nil {
		t.Fatalf("mkdir quarantine dir: %v", err)
	}
	payload := make([]byte, 64)
	for i := range payload {
		payload[i] = 0x11
	}
	if err := os.WriteFile(filepath.Join(quarantineDir, "payload.srm"), payload, 0o644); err != nil {
		t.Fatalf("write quarantine payload: %v", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"id":          filepath.Base(quarantineDir),
		"filename":    "Madden NFL 94.srm",
		"payloadFile": "payload.srm",
		"sizeBytes":   len(payload),
		"reason":      "genesis raw save size 2114 is not recognized",
	})
	if err != nil {
		t.Fatalf("encode quarantine metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(quarantineDir, "metadata.json"), metadata, 0o644); err != nil {
		t.Fatalf("write quarantine metadata: %v", err)
	}

	records, err := h.app.saveStore.load()
	if err != nil {
		t.Fatalf("load saves: %v", err)
	}
	for _, record := range records {
		if strings.Contains(record.dirPath, "_rsm") {
			t.Fatalf("the store indexed its own state as a save record: %s", record.dirPath)
		}
	}

	if _, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true}); err != nil {
		t.Fatalf("rescan saves: %v", err)
	}
	if _, err := os.Stat(quarantineDir); err != nil {
		t.Fatalf("a rescan pruned the quarantine directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(quarantineDir, "metadata.json")); err != nil {
		t.Fatalf("a rescan removed the quarantine metadata: %v", err)
	}
}
