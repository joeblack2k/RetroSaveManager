package main

import (
	"fmt"
	"net/http"
	"net/url"
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
	record, conflict, err := h.app.createPlayStationProjectionSave(input, preview, "retroarch", "psx/retroarch", "deck-psx")
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

func TestPlayStationProjectionDownloadReturnsValidPS1RawCard(t *testing.T) {
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
	record, _, err := h.app.createPlayStationProjectionSave(input, preview, "retroarch", "psx/retroarch", "deck-psx")
	if err != nil {
		t.Fatalf("create playstation projection: %v", err)
	}

	download := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(record.Summary.ID), nil)
	assertStatus(t, download, http.StatusOK)
	if err := validatePS1RawCard(download.Body.Bytes()); err != nil {
		t.Fatalf("expected valid helper-restorable PS1 raw card, got %v", err)
	}
}

func TestPlayStationProjectionUploadRepairsStaleSaveRecordPointer(t *testing.T) {
	h := newContractHarness(t)
	payload := makeTestPS1Card(t, "SCUS_941.63", "Final Fantasy VII Save")
	input := saveCreateInput{
		Filename: "psx.sav",
		Payload:  payload,
		Game:     game{Name: "PlayStation Save", System: supportedSystemFromSlug("psx")},
		Format:   "mcr",
		SlotName: "Memory Card 1",
	}
	preview := h.app.normalizeSaveInputDetailed(input)
	record, _, err := h.app.createPlayStationProjectionSave(input, preview, "mister", "psx/mister", "mister-psx")
	if err != nil {
		t.Fatalf("create playstation projection: %v", err)
	}
	oldID := record.Summary.ID
	removeSaveRecordFromAppForTest(t, h.app, oldID)

	repaired, _, err := h.app.createPlayStationProjectionSave(input, preview, "mister", "psx/mister", "mister-psx")
	if err != nil {
		t.Fatalf("expected stale projection pointer to repair, got %v", err)
	}
	if repaired.Summary.ID == oldID {
		t.Fatalf("expected repaired projection to create a new save record, got stale id %q", repaired.Summary.ID)
	}
	if !saveRecordPayloadExists(repaired) {
		t.Fatalf("expected repaired projection payload to exist")
	}

	store := h.app.playStationSyncStore()
	latestID, _, ok := store.latestProjectionSaveRecord("psx/mister", "Memory Card 1")
	if !ok {
		t.Fatal("expected latest MiSTer projection after repair")
	}
	if latestID != repaired.Summary.ID {
		t.Fatalf("expected projection pointer to be repaired to %q, got %q", repaired.Summary.ID, latestID)
	}
	download := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(repaired.Summary.ID), nil)
	assertStatus(t, download, http.StatusOK)
	if err := validatePS1RawCard(download.Body.Bytes()); err != nil {
		t.Fatalf("expected repaired PS1 projection to remain valid, got %v", err)
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
	record, conflict, err := h.app.createPlayStationProjectionSave(input, preview, "pcsx2", "ps2/pcsx2", "deck-ps2")
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

func TestPlayStationLogicalHistoryDownloadAndDeletePS2Entry(t *testing.T) {
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
	record, _, err := h.app.createPlayStationProjectionSave(input, preview, "pcsx2", "ps2/pcsx2", "deck-ps2")
	if err != nil {
		t.Fatalf("create ps2 projection: %v", err)
	}
	if record.Summary.MemoryCard == nil || len(record.Summary.MemoryCard.Entries) < 2 {
		t.Fatalf("expected ps2 logical entries on projection, got %#v", record.Summary.MemoryCard)
	}
	logicalKey := strings.TrimSpace(record.Summary.MemoryCard.Entries[0].LogicalKey)
	if logicalKey == "" {
		t.Fatal("expected first ps2 entry to expose logicalKey")
	}

	historyPath := "/save?saveId=" + url.QueryEscape(record.Summary.ID) + "&psLogicalKey=" + url.QueryEscape(logicalKey)
	history := h.request(http.MethodGet, historyPath, nil)
	assertStatus(t, history, http.StatusOK)
	historyBody := decodeJSONMap(t, history.Body)
	if mustString(t, historyBody["displayTitle"], "displayTitle") != "Burnout 3" {
		t.Fatalf("expected Burnout 3 logical history, got %s", prettyJSON(historyBody))
	}
	versions := mustArray(t, historyBody["versions"], "versions")
	if len(versions) != 1 {
		t.Fatalf("expected 1 logical version, got %d", len(versions))
	}

	download := h.request(http.MethodGet, "/saves/download?id="+url.QueryEscape(record.Summary.ID)+"&psLogicalKey="+url.QueryEscape(logicalKey), nil)
	assertStatus(t, download, http.StatusOK)
	if got := download.Header().Get("Content-Type"); !strings.Contains(got, "application/zip") {
		t.Fatalf("expected zip download for ps2 logical save, got %q", got)
	}

	deleted := h.request(http.MethodDelete, "/save?id="+url.QueryEscape(record.Summary.ID)+"&psLogicalKey="+url.QueryEscape(logicalKey), nil)
	assertStatus(t, deleted, http.StatusOK)

	store := h.app.playStationSyncStore()
	latestID, _, ok := store.latestProjectionSaveRecord("ps2/pcsx2", "Memory Card 1")
	if !ok {
		t.Fatal("expected latest ps2 projection after logical delete")
	}
	latest, ok := h.app.findSaveRecordByID(latestID)
	if !ok {
		t.Fatalf("expected latest ps2 projection save record %q", latestID)
	}
	if latest.Summary.MemoryCard == nil || len(latest.Summary.MemoryCard.Entries) != 1 {
		t.Fatalf("expected deleted logical save to disappear from projection, got %#v", latest.Summary.MemoryCard)
	}
	if latest.Summary.MemoryCard.Entries[0].Title != "Mortal Kombat Shaolin Monks" {
		t.Fatalf("unexpected remaining ps2 entry after logical delete: %#v", latest.Summary.MemoryCard.Entries)
	}
}

func TestPlayStationLogicalRollbackPromotesHistoricalRevision(t *testing.T) {
	h := newContractHarness(t)
	firstPayload := makeTestPS1Card(t, "SCUS_941.63", "Final Fantasy VII Save")
	secondPayload := append([]byte(nil), firstPayload...)
	secondPayload[psMemoryCardBlockSize+0x90] = 0x7A

	input := saveCreateInput{
		Filename: "memory_card_1.mcr",
		Payload:  firstPayload,
		Game:     game{Name: "PlayStation Save", System: supportedSystemFromSlug("psx")},
		Format:   "mcr",
		SlotName: "Memory Card 1",
	}
	preview := h.app.normalizeSaveInputDetailed(input)
	record, _, err := h.app.createPlayStationProjectionSave(input, preview, "retroarch", "psx/retroarch", "deck-psx")
	if err != nil {
		t.Fatalf("create first ps1 projection: %v", err)
	}

	secondInput := input
	secondInput.Payload = secondPayload
	secondPreview := h.app.normalizeSaveInputDetailed(secondInput)
	record, _, err = h.app.createPlayStationProjectionSave(secondInput, secondPreview, "retroarch", "psx/retroarch", "deck-psx")
	if err != nil {
		t.Fatalf("create second ps1 projection: %v", err)
	}

	logicalKey := strings.TrimSpace(record.Summary.MemoryCard.Entries[0].LogicalKey)
	if logicalKey == "" {
		t.Fatal("expected ps1 entry logicalKey")
	}

	historyPath := "/save?saveId=" + url.QueryEscape(record.Summary.ID) + "&psLogicalKey=" + url.QueryEscape(logicalKey)
	before := h.request(http.MethodGet, historyPath, nil)
	assertStatus(t, before, http.StatusOK)
	beforeBody := decodeJSONMap(t, before.Body)
	beforeVersions := mustArray(t, beforeBody["versions"], "versions")
	if len(beforeVersions) != 2 {
		t.Fatalf("expected 2 ps1 logical versions before rollback, got %d", len(beforeVersions))
	}
	rollbackRevisionID := mustString(t, mustObject(t, beforeVersions[1], "versions[1]")["id"], "versions[1].id")
	rollbackReq := `{"saveId":"` + record.Summary.ID + `","psLogicalKey":"` + logicalKey + `","revisionId":"` + rollbackRevisionID + `"}`
	rollback := h.json(http.MethodPost, "/save/rollback", strings.NewReader(rollbackReq))
	assertStatus(t, rollback, http.StatusOK)

	after := h.request(http.MethodGet, historyPath, nil)
	assertStatus(t, after, http.StatusOK)
	afterBody := decodeJSONMap(t, after.Body)
	afterVersions := mustArray(t, afterBody["versions"], "versions")
	if len(afterVersions) != 3 {
		t.Fatalf("expected 3 ps1 logical versions after rollback, got %d", len(afterVersions))
	}
	newest := mustObject(t, afterVersions[0], "versions[0]")
	if mustNumber(t, newest["version"], "versions[0].version") != 3 {
		t.Fatalf("expected rollback to create logical version 3, got %s", prettyJSON(newest))
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

func removeSaveRecordFromAppForTest(t *testing.T, a *app, saveID string) {
	t.Helper()
	record, found := a.findSaveRecordByID(saveID)
	if !found {
		t.Fatalf("save record %q not found before removal", saveID)
	}
	if record.dirPath != "" {
		if err := os.RemoveAll(record.dirPath); err != nil {
			t.Fatalf("remove save record dir: %v", err)
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	saveRecords := a.saveRecords[:0]
	for _, candidate := range a.saveRecords {
		if candidate.Summary.ID != saveID {
			saveRecords = append(saveRecords, candidate)
		}
	}
	a.saveRecords = saveRecords
	summaries := a.saves[:0]
	for _, candidate := range a.saves {
		if candidate.ID != saveID {
			summaries = append(summaries, candidate)
		}
	}
	a.saves = summaries
}

func makeTestPS1Card(t *testing.T, productCode, title string) []byte {
	t.Helper()
	payload := blankPS1CardTemplate()
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

func validatePS1RawCard(payload []byte) error {
	raw := normalizedPS1MemoryCardImage(payload, "mcr")
	if len(raw) != ps1MemoryCardTotalSize {
		return fmt.Errorf("unexpected PS1 raw size %d", len(raw))
	}
	if err := validatePS1Frame(raw, 0, true); err != nil {
		return fmt.Errorf("frame 0 invalid: %w", err)
	}
	for frameIndex := 1; frameIndex <= psDirectoryEntries; frameIndex++ {
		if err := validatePS1Frame(raw, frameIndex, false); err != nil {
			return fmt.Errorf("directory frame %d invalid: %w", frameIndex, err)
		}
	}
	if err := validatePS1Frame(raw, 63, true); err != nil {
		return fmt.Errorf("frame 63 invalid: %w", err)
	}
	return nil
}

func validatePS1Frame(payload []byte, frameIndex int, expectMarker bool) error {
	offset := frameIndex * psDirectoryEntrySize
	if offset < 0 || offset+psDirectoryEntrySize > len(payload) {
		return fmt.Errorf("frame out of range")
	}
	frame := payload[offset : offset+psDirectoryEntrySize]
	if expectMarker && string(frame[:2]) != "MC" {
		return fmt.Errorf("missing MC marker")
	}
	checksum := byte(0)
	for i := 0; i < psDirectoryEntrySize-1; i++ {
		checksum ^= frame[i]
	}
	if frame[psDirectoryEntrySize-1] != checksum {
		return fmt.Errorf("checksum mismatch: got %d want %d", frame[psDirectoryEntrySize-1], checksum)
	}
	return nil
}
