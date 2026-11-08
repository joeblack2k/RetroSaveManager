package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// A rescan resolves a save's system from the store's own evidence rather than
// from the helper that uploaded it. When that resolves to the system the record
// already carries, there is nothing to write: the normalised record differs
// only in its provenance label ("trusted helper system" becomes "trusted stored
// system") and its confidence, so a rescan rewrites every save in the store and
// reports them all as updated.
func TestRescanKeepsStoredEvidenceWhenTheSystemIsUnchanged(t *testing.T) {
	h := newContractHarness(t)
	records, err := h.app.rawSaveRecordsForRescan()
	if err != nil {
		t.Fatalf("load saves: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected a seeded save record")
	}

	metadataPath := filepath.Join(records[0].dirPath, "metadata.json")
	before, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan saves: %v", err)
	}
	if result.Updated != 0 {
		t.Fatalf("a rescan rewrote %d record(s) whose system did not change", result.Updated)
	}

	after, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata after rescan: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("the rescan rewrote the metadata of a record whose system did not change")
	}

	second, err := h.app.rescanSaves(saveRescanOptions{DryRun: true, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("second rescan: %v", err)
	}
	if second.Updated != 0 {
		t.Fatalf("a second dry run reports updated=%d", second.Updated)
	}
}

// The read path normalises a record the same way a rescan does. It shares the
// rule that an unchanged system keeps the evidence the record already holds, so
// a read cannot report a weaker provenance than the file it came from.
func TestReadingARecordKeepsItsOwnSystemEvidence(t *testing.T) {
	h := newContractHarness(t)
	created, err := h.app.createSave(saveCreateInput{
		Filename:            "Sonic the Hedgehog.srm",
		Payload:             buildNonBlankPayload(8192, 0x05),
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
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload saves: %v", err)
	}

	records, err := h.app.rawSaveRecordsForRescan()
	if err != nil {
		t.Fatalf("load saves: %v", err)
	}
	var target *saveRecord
	for i := range records {
		if records[i].Summary.ID == created.Summary.ID {
			target = &records[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("created save %s is not in the store", created.Summary.ID)
	}
	if !recordCarriesHelperTrust(*target) {
		t.Fatal("the stored record lost the helper's declaration before the read")
	}

	h.app.decorateLoadedRecord(target)
	if !recordCarriesHelperTrust(*target) {
		t.Fatal("a read re-stamped the record's system evidence")
	}
}

func recordCarriesHelperTrust(record saveRecord) bool {
	root, ok := record.Summary.Metadata.(map[string]any)
	if !ok {
		return false
	}
	rsm, ok := root["rsm"].(map[string]any)
	if !ok {
		return false
	}
	detection, ok := rsm["systemDetection"].(map[string]any)
	if !ok {
		return false
	}
	evidence, ok := detection["evidence"].(map[string]any)
	if !ok {
		return false
	}
	helper, _ := evidence["helperTrusted"].(bool)
	return helper
}
