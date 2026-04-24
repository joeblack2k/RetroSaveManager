package main

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

func countSaveLineRecords(records []saveRecord, romSHA1, slotName string) int {
	count := 0
	for _, record := range records {
		if strings.TrimSpace(record.ROMSHA1) != strings.TrimSpace(romSHA1) {
			continue
		}
		if normalizedSlot(record.SlotName) != normalizedSlot(slotName) {
			continue
		}
		count++
	}
	return count
}

func TestGenericUploadDuplicateLatestIsIgnored(t *testing.T) {
	h := newContractHarness(t)
	fields := map[string]string{
		"rom_sha1": "dedupe-latest-rom",
		"slotName": "slot-a",
		"system":   "n64",
	}
	first := uploadSave(t, h, "/saves", fields, "Wave Race 64 (USA).eep", buildTestN64Payload("eep", "generic-dedupe-latest"))
	firstSave := mustObject(t, first["save"], "save")
	firstID := mustString(t, firstSave["id"], "save.id")

	second := h.multipart("/saves", fields, "file", "Wave Race 64 (USA).eep", buildTestN64Payload("eep", "generic-dedupe-latest"))
	assertStatus(t, second, http.StatusOK)
	body := decodeJSONMap(t, second.Body)
	if !mustBool(t, body["duplicate"], "duplicate") {
		t.Fatalf("expected duplicate response, got %s", prettyJSON(body))
	}
	if got := mustString(t, body["duplicateDisposition"], "duplicateDisposition"); got != string(uploadDuplicateIgnoredLatest) {
		t.Fatalf("expected ignored-latest duplicate disposition, got %q", got)
	}
	secondSave := mustObject(t, body["save"], "save")
	if got := mustString(t, secondSave["id"], "save.id"); got != firstID {
		t.Fatalf("expected duplicate upload to reuse first save id %q, got %q", firstID, got)
	}
	if got := countSaveLineRecords(h.app.snapshotSaveRecords(), "dedupe-latest-rom", "slot-a"); got != 1 {
		t.Fatalf("expected 1 stored version after duplicate latest upload, got %d", got)
	}
}

func TestGenericUploadHistoricalDuplicateReturnsConflict(t *testing.T) {
	h := newContractHarness(t)
	fields := map[string]string{
		"rom_sha1": "dedupe-stale-rom",
		"slotName": "slot-a",
		"system":   "n64",
	}
	first := uploadSave(t, h, "/saves", fields, "Star Fox 64 (USA).eep", buildTestN64Payload("eep", "generic-dedupe-a"))
	firstID := mustString(t, mustObject(t, first["save"], "save")["id"], "save.id")
	second := uploadSave(t, h, "/saves", fields, "Star Fox 64 (USA).eep", buildTestN64Payload("eep", "generic-dedupe-b"))
	secondID := mustString(t, mustObject(t, second["save"], "save")["id"], "save.id")
	if firstID == secondID {
		t.Fatalf("expected second upload to create a distinct save record")
	}

	third := h.multipart("/saves", fields, "file", "Star Fox 64 (USA).eep", buildTestN64Payload("eep", "generic-dedupe-a"))
	assertStatus(t, third, http.StatusConflict)
	body := decodeJSONMap(t, third.Body)
	if got := mustString(t, body["reason"], "reason"); got != "stale_historical_duplicate" {
		t.Fatalf("expected stale_historical_duplicate reason, got %q", got)
	}
	latest := mustObject(t, body["latest"], "latest")
	if got := mustString(t, latest["id"], "latest.id"); got != secondID {
		t.Fatalf("expected latest save id %q, got %q", secondID, got)
	}
	if got := countSaveLineRecords(h.app.snapshotSaveRecords(), "dedupe-stale-rom", "slot-a"); got != 2 {
		t.Fatalf("expected 2 stored versions after stale duplicate rejection, got %d", got)
	}
}

func TestGenericUploadDuplicateAcrossROMVariantsIsIgnored(t *testing.T) {
	h := newContractHarness(t)
	payload := buildTestN64Payload("eep", "cross-rom-duplicate")
	first := uploadSave(t, h, "/saves", map[string]string{
		"rom_sha1": "star-fox-retail-rom",
		"slotName": "slot-a",
		"system":   "n64",
	}, "Star Fox 64 (USA).eep", payload)
	firstID := mustString(t, mustObject(t, first["save"], "save")["id"], "save.id")

	second := h.multipart("/saves", map[string]string{
		"rom_sha1": "star-fox-rev1-rom",
		"slotName": "slot-a",
		"system":   "n64",
	}, "file", "Star Fox 64 (USA) (Rev 1).eep", payload)
	assertStatus(t, second, http.StatusOK)
	body := decodeJSONMap(t, second.Body)
	if !mustBool(t, body["duplicate"], "duplicate") {
		t.Fatalf("expected cross-ROM duplicate response, got %s", prettyJSON(body))
	}
	if got := mustString(t, body["duplicateDisposition"], "duplicateDisposition"); got != string(uploadDuplicateIgnoredLatest) {
		t.Fatalf("expected ignored-latest duplicate disposition, got %q", got)
	}
	if got := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id"); got != firstID {
		t.Fatalf("expected cross-ROM duplicate upload to return %q, got %q", firstID, got)
	}

	list := h.request(http.MethodGet, "/saves?limit=50&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	saves := mustArray(t, decodeJSONMap(t, list.Body)["saves"], "saves")
	matches := 0
	for _, item := range saves {
		summary := mustObject(t, item, "saves[n]")
		if mustString(t, summary["displayTitle"], "displayTitle") != "Star Fox 64" {
			continue
		}
		matches++
		if got := mustNumber(t, summary["saveCount"], "saveCount"); got != 1 {
			t.Fatalf("expected one visible save after cross-ROM duplicate upload, got %s", prettyJSON(summary))
		}
	}
	if matches != 1 {
		t.Fatalf("expected one Star Fox row, got %d", matches)
	}

	history := h.request(http.MethodGet, "/save?saveId="+url.QueryEscape(firstID), nil)
	assertStatus(t, history, http.StatusOK)
	versions := mustArray(t, decodeJSONMap(t, history.Body)["versions"], "versions")
	if len(versions) != 1 {
		t.Fatalf("expected one history version after cross-ROM duplicate upload, got %d", len(versions))
	}
}

func TestPlayStationHistoricalDuplicateImportIsStale(t *testing.T) {
	h := newContractHarness(t)
	firstPayload := makeTestPS1Card(t, "SCUS_941.63", "Final Fantasy VII Save")
	secondPayload := append([]byte(nil), firstPayload...)
	secondPayload[psMemoryCardBlockSize+0x90] = 0x5A

	input := saveCreateInput{
		Filename: "memory_card_1.mcr",
		Payload:  firstPayload,
		Game:     game{Name: "PlayStation Save", System: supportedSystemFromSlug("psx")},
		Format:   "mcr",
		SlotName: "Memory Card 1",
	}
	preview := h.app.normalizeSaveInputDetailed(input)
	first, err := h.app.createPlayStationProjectionSaveDetailed(input, preview, "retroarch", "psx/retroarch", "deck-psx")
	if err != nil {
		t.Fatalf("create first playstation projection: %v", err)
	}
	if first.Disposition != uploadDuplicateNone {
		t.Fatalf("unexpected first import disposition: %q", first.Disposition)
	}

	secondInput := input
	secondInput.Payload = secondPayload
	secondPreview := h.app.normalizeSaveInputDetailed(secondInput)
	second, err := h.app.createPlayStationProjectionSaveDetailed(secondInput, secondPreview, "retroarch", "psx/retroarch", "deck-psx")
	if err != nil {
		t.Fatalf("create second playstation projection: %v", err)
	}

	third, err := h.app.createPlayStationProjectionSaveDetailed(input, preview, "retroarch", "psx/retroarch", "deck-psx")
	if err != nil {
		t.Fatalf("create third playstation projection: %v", err)
	}
	if third.Disposition != uploadDuplicateStaleHistorical {
		t.Fatalf("expected stale historical disposition, got %q", third.Disposition)
	}
	if got := third.Record.Summary.ID; got != second.Record.Summary.ID {
		t.Fatalf("expected stale import to return current latest save id %q, got %q", second.Record.Summary.ID, got)
	}

	logicalKey := strings.TrimSpace(second.Record.Summary.MemoryCard.Entries[0].LogicalKey)
	history := h.request(http.MethodGet, "/save?saveId="+url.QueryEscape(second.Record.Summary.ID)+"&psLogicalKey="+url.QueryEscape(logicalKey), nil)
	assertStatus(t, history, http.StatusOK)
	versions := mustArray(t, decodeJSONMap(t, history.Body)["versions"], "versions")
	if len(versions) != 2 {
		t.Fatalf("expected 2 logical revisions after stale historical import, got %d", len(versions))
	}
}

func TestN64ControllerPakLatestDuplicateUploadIsIgnored(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "n64-dedupe-latest")
	pak := makeTestN64ControllerPak(t,
		testN64ControllerPakEntry{Name: "MK64A", GameCode: "NKTE", PublisherCode: "01", Payload: buildTestN64Payload("eep", "n64-cpk-dedupe-latest")},
	)
	fields := map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "n64-cpk-dedupe-rom",
		"slotName":     "controller-1",
		"system":       "n64",
		"device_type":  "mister",
		"fingerprint":  "mister-cpk-dedupe",
		"n64Profile":   n64ProfileMister,
	}
	first := h.multipart("/saves", fields, "file", "latest.cpk", pak)
	assertStatus(t, first, http.StatusOK)
	firstBody := decodeJSONMap(t, first.Body)
	firstID := mustString(t, mustObject(t, firstBody["save"], "save")["id"], "save.id")

	second := h.multipart("/saves", fields, "file", "latest.cpk", pak)
	assertStatus(t, second, http.StatusOK)
	secondBody := decodeJSONMap(t, second.Body)
	if !mustBool(t, secondBody["duplicate"], "duplicate") {
		t.Fatalf("expected duplicate latest response, got %s", prettyJSON(secondBody))
	}
	if got := mustString(t, secondBody["duplicateDisposition"], "duplicateDisposition"); got != string(uploadDuplicateIgnoredLatest) {
		t.Fatalf("expected ignored-latest disposition, got %q", got)
	}
	if got := mustString(t, mustObject(t, secondBody["save"], "save")["id"], "save.id"); got != firstID {
		t.Fatalf("expected duplicate latest upload to return %q, got %q", firstID, got)
	}

	list := h.request(http.MethodGet, "/saves?romSha1=n64-cpk-dedupe-rom&limit=50&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	saves := mustArray(t, decodeJSONMap(t, list.Body)["saves"], "saves")
	if len(saves) != 1 {
		t.Fatalf("expected 1 logical controller pak save, got %d", len(saves))
	}
	firstSummary := mustObject(t, saves[0], "saves[0]")
	history := h.request(http.MethodGet, "/save?saveId="+url.QueryEscape(mustString(t, firstSummary["id"], "id"))+"&psLogicalKey="+url.QueryEscape(mustString(t, firstSummary["logicalKey"], "logicalKey")), nil)
	assertStatus(t, history, http.StatusOK)
	versions := mustArray(t, decodeJSONMap(t, history.Body)["versions"], "versions")
	if len(versions) != 1 {
		t.Fatalf("expected 1 logical revision after duplicate latest upload, got %d", len(versions))
	}
}

func TestRescanCleanupRemovesDuplicateVersionsButKeepsRollbackAudit(t *testing.T) {
	h := newContractHarness(t)
	rom := "cleanup-dedupe-rom"
	slot := "slot-a"
	createdAt := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)

	first, err := h.app.createSave(saveCreateInput{
		Filename:   "Yoshi's Story (USA).eep",
		Payload:    buildTestN64Payload("eep", "cleanup-a"),
		Game:       fallbackGameFromFilename("Yoshi's Story (USA).eep"),
		Format:     "eep",
		ROMSHA1:    rom,
		SlotName:   slot,
		SystemSlug: "n64",
		CreatedAt:  createdAt,
	})
	if err != nil {
		t.Fatalf("create first save: %v", err)
	}
	_, err = h.app.createSave(saveCreateInput{
		Filename:   "Yoshi's Story (USA).eep",
		Payload:    buildTestN64Payload("eep", "cleanup-b"),
		Game:       fallbackGameFromFilename("Yoshi's Story (USA).eep"),
		Format:     "eep",
		ROMSHA1:    rom,
		SlotName:   slot,
		SystemSlug: "n64",
		CreatedAt:  createdAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("create second save: %v", err)
	}
	_, err = h.app.createSave(saveCreateInput{
		Filename:   "Yoshi's Story (USA).eep",
		Payload:    buildTestN64Payload("eep", "cleanup-a"),
		Game:       fallbackGameFromFilename("Yoshi's Story (USA).eep"),
		Format:     "eep",
		ROMSHA1:    rom,
		SlotName:   slot,
		SystemSlug: "n64",
		CreatedAt:  createdAt.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create third duplicate save: %v", err)
	}
	_, err = h.app.createSave(saveCreateInput{
		Filename:   "Yoshi's Story (USA).eep",
		Payload:    buildTestN64Payload("eep", "cleanup-a"),
		Game:       fallbackGameFromFilename("Yoshi's Story (USA).eep"),
		Format:     "eep",
		Metadata:   mergeRollbackMetadata(first),
		ROMSHA1:    rom,
		SlotName:   slot,
		SystemSlug: "n64",
		CreatedAt:  createdAt.Add(3 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create rollback audit save: %v", err)
	}

	if got := countSaveLineRecords(h.app.snapshotSaveRecords(), rom, slot); got != 4 {
		t.Fatalf("expected 4 versions before cleanup, got %d", got)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan with duplicate cleanup: %v", err)
	}
	if result.DuplicateGroups < 1 {
		t.Fatalf("expected duplicate groups to be reported, got %+v", result)
	}
	if result.DuplicateVersionsRemoved < 1 {
		t.Fatalf("expected duplicate versions to be removed, got %+v", result)
	}

	line := make([]saveRecord, 0)
	for _, record := range h.app.snapshotSaveRecords() {
		if strings.TrimSpace(record.ROMSHA1) == rom && normalizedSlot(record.SlotName) == slot {
			line = append(line, record)
		}
	}
	if len(line) != 3 {
		t.Fatalf("expected 3 surviving versions after cleanup, got %d", len(line))
	}
	sort.Slice(line, func(i, j int) bool { return saveRecordSortsAfter(line[i], line[j]) })
	if !saveRecordIsRollback(line[0]) {
		t.Fatalf("expected rollback audit version to remain latest after cleanup")
	}
	versions := []int{line[0].Summary.Version, line[1].Summary.Version, line[2].Summary.Version}
	if versions[0] != 3 || versions[1] != 2 || versions[2] != 1 {
		t.Fatalf("expected contiguous renumbered versions [3 2 1], got %v", versions)
	}
}

func TestRescanCleanupRemovesDuplicateAcrossROMVariants(t *testing.T) {
	h := newContractHarness(t)
	payload := buildTestN64Payload("eep", "cleanup-cross-rom")
	createdAt := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	first, err := h.app.createSave(saveCreateInput{
		Filename:   "Star Fox 64 (USA).eep",
		Payload:    payload,
		Game:       fallbackGameFromFilename("Star Fox 64 (USA).eep"),
		Format:     "eep",
		ROMSHA1:    "cleanup-starfox-retail",
		SlotName:   "slot-a",
		SystemSlug: "n64",
		CreatedAt:  createdAt,
	})
	if err != nil {
		t.Fatalf("create first cross-ROM save: %v", err)
	}
	_, err = h.app.createSave(saveCreateInput{
		Filename:   "Star Fox 64 (USA) (Rev 1).eep",
		Payload:    payload,
		Game:       fallbackGameFromFilename("Star Fox 64 (USA) (Rev 1).eep"),
		Format:     "eep",
		ROMSHA1:    "cleanup-starfox-rev1",
		SlotName:   "slot-a",
		SystemSlug: "n64",
		CreatedAt:  createdAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("create second cross-ROM save: %v", err)
	}
	if got := countRecordsByDuplicateTrackAndSHA(h.app.snapshotSaveRecords(), canonicalDuplicateTrackKeyForRecord(first), first.Summary.SHA256); got != 2 {
		t.Fatalf("expected 2 cross-ROM duplicate records before cleanup, got %d", got)
	}

	result, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true})
	if err != nil {
		t.Fatalf("rescan with cross-ROM duplicate cleanup: %v", err)
	}
	if result.DuplicateGroups < 1 || result.DuplicateVersionsRemoved < 1 {
		t.Fatalf("expected duplicate cleanup to report cross-ROM duplicate, got %+v", result)
	}
	records := h.app.snapshotSaveRecords()
	matching := make([]saveRecord, 0, 1)
	for _, record := range records {
		if canonicalDuplicateTrackKeyForRecord(record) == canonicalDuplicateTrackKeyForRecord(first) && record.Summary.SHA256 == first.Summary.SHA256 {
			matching = append(matching, record)
		}
	}
	if len(matching) != 1 {
		t.Fatalf("expected one surviving cross-ROM duplicate record after cleanup, got %d", len(matching))
	}
	if matching[0].Summary.SHA256 != first.Summary.SHA256 {
		t.Fatalf("expected surviving record to keep the duplicate payload")
	}
}

func countRecordsByDuplicateTrackAndSHA(records []saveRecord, key, sha string) int {
	count := 0
	for _, record := range records {
		if canonicalDuplicateTrackKeyForRecord(record) != key {
			continue
		}
		if strings.TrimSpace(record.Summary.SHA256) != strings.TrimSpace(sha) {
			continue
		}
		count++
	}
	return count
}
