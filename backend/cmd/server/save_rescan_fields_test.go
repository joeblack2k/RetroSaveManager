package main

import (
	"testing"
)

// A record carries fields the rescan never had to derive -- the runtime
// profile it was uploaded with, the projection and import ids that point at
// its payload, the card slot it came from. Building the normaliser's input
// without them wrote the empty values back, so a rescan dropped every one of
// them from every record that had them, and reported each as updated.
func TestRescanKeepsTheFieldsARecordCarries(t *testing.T) {
	h := newContractHarness(t)
	records := h.app.snapshotSaveRecords()
	if len(records) == 0 {
		t.Fatal("expected a seeded save record")
	}
	record := records[0]
	portable := true
	record.Summary.RuntimeProfile = "n64/retroarch"
	record.Summary.PortID = "port-1"
	record.Summary.PortName = "OpenPlaudit"
	record.Summary.OriginSystemSlug = "n64"
	record.Summary.PortSaveKind = "progress"
	record.Summary.RelativePath = "n64/port/progress.sav"
	record.Summary.RootRelativePath = "n64/port/progress.sav"
	record.Summary.SlotID = "1"
	record.Summary.CardSlot = "Memory Card 1"
	record.Summary.ProjectionID = "ps-projection-20260607090025.338399310"
	record.Summary.SourceImportID = "repair:stale-save-record-pointer"
	record.Summary.Portable = &portable
	if err := persistSaveRecordMetadata(record); err != nil {
		t.Fatalf("persist record: %v", err)
	}
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	if _, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true}); err != nil {
		t.Fatalf("rescan: %v", err)
	}

	after := h.app.snapshotSaveRecords()[0]
	checks := map[string]string{
		"runtimeProfile":   after.Summary.RuntimeProfile,
		"portId":           after.Summary.PortID,
		"portName":         after.Summary.PortName,
		"originSystemSlug": after.Summary.OriginSystemSlug,
		"portSaveKind":     after.Summary.PortSaveKind,
		"relativePath":     after.Summary.RelativePath,
		"rootRelativePath": after.Summary.RootRelativePath,
		"slotId":           after.Summary.SlotID,
		"cardSlot":         after.Summary.CardSlot,
		"projectionId":     after.Summary.ProjectionID,
		"sourceImportId":   after.Summary.SourceImportID,
	}
	for field, value := range checks {
		if value == "" {
			t.Fatalf("a rescan dropped %s from the record", field)
		}
	}
	if after.Summary.Portable == nil || !*after.Summary.Portable {
		t.Fatal("a rescan dropped portable from the record")
	}
}
