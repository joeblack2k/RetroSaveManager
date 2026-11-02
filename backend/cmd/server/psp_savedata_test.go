package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"net/http"
	"strings"
	"testing"
)

type testPSPZipEntry struct {
	name string
	data []byte
}

type testPSPSFOEntry struct {
	key   string
	value string
}

// buildTestPSPParamSFO writes a minimal but valid PSF (PARAM.SFO) holding UTF-8 string entries.
func buildTestPSPParamSFO(entries []testPSPSFOEntry) []byte {
	const headerSize = 20
	const indexEntrySize = 16
	var keyTable bytes.Buffer
	var dataTable bytes.Buffer
	index := make([]byte, indexEntrySize*len(entries))
	for i, entry := range entries {
		value := append([]byte(entry.value), 0)
		maxLen := (len(value) + 3) &^ 3
		row := index[i*indexEntrySize:]
		binary.LittleEndian.PutUint16(row[0:2], uint16(keyTable.Len()))
		binary.LittleEndian.PutUint16(row[2:4], 0x0204)
		binary.LittleEndian.PutUint32(row[4:8], uint32(len(value)))
		binary.LittleEndian.PutUint32(row[8:12], uint32(maxLen))
		binary.LittleEndian.PutUint32(row[12:16], uint32(dataTable.Len()))
		keyTable.WriteString(entry.key)
		keyTable.WriteByte(0)
		dataTable.Write(value)
		dataTable.Write(make([]byte, maxLen-len(value)))
	}
	for keyTable.Len()%4 != 0 {
		keyTable.WriteByte(0)
	}
	keyStart := headerSize + len(index)
	dataStart := keyStart + keyTable.Len()
	out := make([]byte, headerSize, dataStart+dataTable.Len())
	copy(out[0:4], []byte("\x00PSF"))
	binary.LittleEndian.PutUint32(out[4:8], 0x00000101)
	binary.LittleEndian.PutUint32(out[8:12], uint32(keyStart))
	binary.LittleEndian.PutUint32(out[12:16], uint32(dataStart))
	binary.LittleEndian.PutUint32(out[16:20], uint32(len(entries)))
	out = append(out, index...)
	out = append(out, keyTable.Bytes()...)
	out = append(out, dataTable.Bytes()...)
	return out
}

func buildTestPSPZip(t *testing.T, entries []testPSPZipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, entry := range entries {
		part, err := writer.Create(entry.name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", entry.name, err)
		}
		if _, err := part.Write(entry.data); err != nil {
			t.Fatalf("write zip entry %s: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func testPSPSaveData(seed string) []byte {
	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i*7) ^ 0x5A
	}
	copy(payload[16:], []byte(seed))
	return payload
}

// buildTestPSPBundle returns a SAVEDATA bundle for saveDir whose PARAM.SFO declares sfoDir.
func buildTestPSPBundle(t *testing.T, saveDir, sfoDir, title, seed string) []byte {
	t.Helper()
	sfo := buildTestPSPParamSFO([]testPSPSFOEntry{
		{key: "SAVEDATA_DETAIL", value: "Story mode progress"},
		{key: "SAVEDATA_DIRECTORY", value: sfoDir},
		{key: "SAVEDATA_TITLE", value: "System Data"},
		{key: "TITLE", value: title},
	})
	return buildTestPSPZip(t, []testPSPZipEntry{
		{name: "PARAM.SFO", data: sfo},
		{name: "ICON0.PNG", data: []byte("\x89PNG\r\n\x1a\nfake-icon-" + saveDir)},
		{name: "DISSIDIA.BIN", data: testPSPSaveData(seed)},
	})
}

func pspBundleFields(saveDir string) map[string]string {
	return map[string]string{
		"system":     "psp",
		"pspSaveDir": saveDir,
		"rom_sha1":   "psp-savedir:" + strings.ToUpper(saveDir),
		"slotName":   "default",
	}
}

func pspRecordsBySaveDir(records []saveRecord, saveDir string) []saveRecord {
	out := []saveRecord{}
	for _, record := range records {
		if testPSPMetadataString(record.Summary.Metadata, "saveDir") == saveDir {
			out = append(out, record)
		}
	}
	return out
}

func testPSPMetadataString(metadata any, key string) string {
	root, ok := metadata.(map[string]any)
	if !ok {
		return ""
	}
	rsm, ok := root["rsm"].(map[string]any)
	if !ok {
		return ""
	}
	psp, ok := rsm["psp"].(map[string]any)
	if !ok {
		return ""
	}
	value, _ := psp[key].(string)
	return value
}

func mustFindRecordByID(t *testing.T, h *contractHarness, id string) saveRecord {
	t.Helper()
	record, ok := h.app.findSaveRecordByID(id)
	if !ok {
		t.Fatalf("expected save record %q", id)
	}
	return record
}

func TestPSPSaveDataBundleUploadStoresOneRecord(t *testing.T) {
	h := newContractHarness(t)
	before := len(h.app.snapshotSaveRecords())
	bundle := buildTestPSPBundle(t, "ULUS10437DATA00", "ULUS10437DATA00", "DISSIDIA FINAL FANTASY", "slot-a")

	body := uploadSave(t, h, "/saves", pspBundleFields("ULUS10437DATA00"), "ULUS10437DATA00.pspsave", bundle)
	saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")

	records := h.app.snapshotSaveRecords()
	if got := len(records) - before; got != 1 {
		t.Fatalf("expected exactly one new record for a bundle, got %d", got)
	}
	record := mustFindRecordByID(t, h, saveID)
	if got := testPSPMetadataString(record.Summary.Metadata, "saveDir"); got != "ULUS10437DATA00" {
		t.Fatalf("expected rsm.psp.saveDir=ULUS10437DATA00, got %q (metadata=%+v)", got, record.Summary.Metadata)
	}
	if got := testPSPMetadataString(record.Summary.Metadata, "bundle"); got != "savedata-zip" {
		t.Fatalf("expected rsm.psp.bundle=savedata-zip, got %q", got)
	}
	if got := testPSPMetadataString(record.Summary.Metadata, "gameId"); got != "ULUS10437" {
		t.Fatalf("expected rsm.psp.gameId=ULUS10437, got %q", got)
	}
	if record.Summary.SystemSlug != "psp" {
		t.Fatalf("expected psp system, got %q", record.Summary.SystemSlug)
	}
	if record.Summary.DisplayTitle != "DISSIDIA FINAL FANTASY" {
		t.Fatalf("expected SFO title as display title, got %q", record.Summary.DisplayTitle)
	}
	if record.Summary.RegionCode != regionUS {
		t.Fatalf("expected US region from ULUS game id, got %q", record.Summary.RegionCode)
	}
}

func TestPSPSaveDataBundleDownloadIsByteIdentical(t *testing.T) {
	h := newContractHarness(t)
	bundle := buildTestPSPBundle(t, "ULUS10437DATA00", "ULUS10437DATA00", "DISSIDIA FINAL FANTASY", "download")

	body := uploadSave(t, h, "/saves", pspBundleFields("ULUS10437DATA00"), "ULUS10437DATA00.pspsave", bundle)
	saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")

	download := h.request(http.MethodGet, "/saves/download?id="+saveID, nil)
	assertStatus(t, download, http.StatusOK)
	if !bytes.Equal(download.Body.Bytes(), bundle) {
		t.Fatalf("expected byte-identical bundle download (%d bytes), got %d bytes", len(bundle), download.Body.Len())
	}
	if got := download.Header().Get("Content-Disposition"); !strings.Contains(got, "ULUS10437DATA00.pspsave") {
		t.Fatalf("expected bundle filename in Content-Disposition, got %q", got)
	}
}

func TestPSPSaveDataBundleRejectsDirectoryMismatch(t *testing.T) {
	h := newContractHarness(t)
	before := len(h.app.snapshotSaveRecords())
	bundle := buildTestPSPBundle(t, "ULUS10437DATA00", "ULUS10437DATA99", "DISSIDIA FINAL FANTASY", "mismatch")

	rr := h.multipart("/saves", pspBundleFields("ULUS10437DATA00"), "file", "ULUS10437DATA00.pspsave", bundle)
	assertStatus(t, rr, http.StatusUnprocessableEntity)
	if !strings.Contains(rr.Body.String(), "SAVEDATA_DIRECTORY") {
		t.Fatalf("expected SAVEDATA_DIRECTORY mismatch reason, got %s", rr.Body.String())
	}
	if got := len(h.app.snapshotSaveRecords()) - before; got != 0 {
		t.Fatalf("expected no record for a rejected bundle, got %d", got)
	}
}

func TestPSPSaveDataBundleRejectsUnsafeEntryName(t *testing.T) {
	h := newContractHarness(t)
	before := len(h.app.snapshotSaveRecords())
	sfo := buildTestPSPParamSFO([]testPSPSFOEntry{
		{key: "SAVEDATA_DIRECTORY", value: "ULUS10437DATA00"},
		{key: "TITLE", value: "DISSIDIA FINAL FANTASY"},
	})
	bundle := buildTestPSPZip(t, []testPSPZipEntry{
		{name: "PARAM.SFO", data: sfo},
		{name: "DISSIDIA.BIN", data: testPSPSaveData("unsafe")},
		{name: "../x", data: []byte("escape")},
	})

	rr := h.multipart("/saves", pspBundleFields("ULUS10437DATA00"), "file", "ULUS10437DATA00.pspsave", bundle)
	assertStatus(t, rr, http.StatusUnprocessableEntity)
	if !strings.Contains(rr.Body.String(), "../x") {
		t.Fatalf("expected unsafe entry name in reason, got %s", rr.Body.String())
	}
	if got := len(h.app.snapshotSaveRecords()) - before; got != 0 {
		t.Fatalf("expected no record for a rejected bundle, got %d", got)
	}
}

func TestPSPSaveDataBundleAndLooseRecordUseDifferentKeys(t *testing.T) {
	h := newContractHarness(t)
	loose := uploadSave(t, h, "/saves", map[string]string{"system": "psp"}, "DISSIDIA FINAL FANTASY (USA).bin", testPSPSaveData("loose"))
	looseRecord := mustFindRecordByID(t, h, mustString(t, mustObject(t, loose["save"], "save")["id"], "save.id"))

	bundle := buildTestPSPBundle(t, "ULUS10437DATA00", "ULUS10437DATA00", "DISSIDIA FINAL FANTASY", "bundle")
	body := uploadSave(t, h, "/saves", pspBundleFields("ULUS10437DATA00"), "ULUS10437DATA00.pspsave", bundle)
	if body["duplicate"] == true {
		t.Fatalf("bundle upload must not be treated as a duplicate of the loose record: %s", prettyJSON(body))
	}
	bundleRecord := mustFindRecordByID(t, h, mustString(t, mustObject(t, body["save"], "save")["id"], "save.id"))

	if looseRecord.Summary.DisplayTitle != bundleRecord.Summary.DisplayTitle {
		t.Fatalf("fixture expects matching titles, got %q vs %q", looseRecord.Summary.DisplayTitle, bundleRecord.Summary.DisplayTitle)
	}
	if a, b := canonicalListKeyForRecord(looseRecord), canonicalListKeyForRecord(bundleRecord); a == b {
		t.Fatalf("expected different list keys for loose and bundle records, both %q", a)
	}
	if a, b := canonicalHistoryKeyForRecord(looseRecord), canonicalHistoryKeyForRecord(bundleRecord); a == b {
		t.Fatalf("expected different track keys for loose and bundle records, both %q", a)
	}
	if got := canonicalListKeyForRecord(bundleRecord); got != "psp::savedir::ULUS10437DATA00" {
		t.Fatalf("expected bundle list key psp::savedir::ULUS10437DATA00, got %q", got)
	}
}

func TestPSPSaveDataBundlesWithSameTitleStaySeparateTracks(t *testing.T) {
	h := newContractHarness(t)
	first := buildTestPSPBundle(t, "ULUS10437DATA00", "ULUS10437DATA00", "DISSIDIA FINAL FANTASY", "first")
	second := buildTestPSPBundle(t, "ULUS10437DATA01", "ULUS10437DATA01", "DISSIDIA FINAL FANTASY", "second")

	firstBody := uploadSave(t, h, "/saves", pspBundleFields("ULUS10437DATA00"), "ULUS10437DATA00.pspsave", first)
	secondBody := uploadSave(t, h, "/saves", pspBundleFields("ULUS10437DATA01"), "ULUS10437DATA01.pspsave", second)
	firstRecord := mustFindRecordByID(t, h, mustString(t, mustObject(t, firstBody["save"], "save")["id"], "save.id"))
	secondRecord := mustFindRecordByID(t, h, mustString(t, mustObject(t, secondBody["save"], "save")["id"], "save.id"))

	if a, b := canonicalListKeyForRecord(firstRecord), canonicalListKeyForRecord(secondRecord); a == b {
		t.Fatalf("expected two folders of one game to stay separate tracks, both %q", a)
	}
	if firstRecord.Summary.Game.ID == secondRecord.Summary.Game.ID {
		t.Fatalf("expected distinct game ids per save folder, both %d", firstRecord.Summary.Game.ID)
	}
	if got := len(pspRecordsBySaveDir(h.app.snapshotSaveRecords(), "ULUS10437DATA01")); got != 1 {
		t.Fatalf("expected one record for the second folder, got %d", got)
	}
}

func TestPSPSaveDataBundleDuplicateUploadIsIgnored(t *testing.T) {
	h := newContractHarness(t)
	bundle := buildTestPSPBundle(t, "ULUS10437DATA00", "ULUS10437DATA00", "DISSIDIA FINAL FANTASY", "dedupe")
	fields := pspBundleFields("ULUS10437DATA00")

	first := uploadSave(t, h, "/saves", fields, "ULUS10437DATA00.pspsave", bundle)
	firstID := mustString(t, mustObject(t, first["save"], "save")["id"], "save.id")

	second := h.multipart("/saves", fields, "file", "ULUS10437DATA00.pspsave", bundle)
	assertStatus(t, second, http.StatusOK)
	body := decodeJSONMap(t, second.Body)
	if !mustBool(t, body["duplicate"], "duplicate") {
		t.Fatalf("expected duplicate response, got %s", prettyJSON(body))
	}
	if got := mustString(t, body["duplicateDisposition"], "duplicateDisposition"); got != string(uploadDuplicateIgnoredLatest) {
		t.Fatalf("expected ignored-latest duplicate disposition, got %q", got)
	}
	if got := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id"); got != firstID {
		t.Fatalf("expected duplicate to reuse save id %q, got %q", firstID, got)
	}
	if got := countSaveLineRecords(h.app.snapshotSaveRecords(), "psp-savedir:ULUS10437DATA00", "default"); got != 1 {
		t.Fatalf("expected 1 stored version after duplicate upload, got %d", got)
	}
}

func TestPSPSaveDataBundlePreviewIsOneItem(t *testing.T) {
	h := newContractHarness(t)
	bundle := buildTestPSPBundle(t, "ULUS10437DATA00", "ULUS10437DATA00", "DISSIDIA FINAL FANTASY", "preview")

	rr := h.multipart("/api/saves/preview", pspBundleFields("ULUS10437DATA00"), "file", "ULUS10437DATA00.pspsave", bundle)
	assertStatus(t, rr, http.StatusOK)
	body := decodeJSONMap(t, rr.Body)
	items := mustArray(t, body["items"], "items")
	if len(items) != 1 {
		t.Fatalf("expected one preview item for a bundle, got %d: %s", len(items), prettyJSON(body))
	}
	item := mustObject(t, items[0], "items[0]")
	if !mustBool(t, item["accepted"], "items[0].accepted") {
		t.Fatalf("expected bundle preview to be accepted, got %s", prettyJSON(item))
	}
	if got := mustString(t, item["displayTitle"], "items[0].displayTitle"); got != "DISSIDIA FINAL FANTASY" {
		t.Fatalf("expected SFO title in preview, got %q", got)
	}
}

func TestPSPLooseUploadWithoutSaveDirIsUnchanged(t *testing.T) {
	h := newContractHarness(t)
	payload := testPSPSaveData("loose-only")
	body := uploadSave(t, h, "/saves", map[string]string{"system": "psp", "rom_sha1": "psp-loose-rom"}, "Lumines (USA).bin", payload)
	record := mustFindRecordByID(t, h, mustString(t, mustObject(t, body["save"], "save")["id"], "save.id"))

	if record.Summary.SystemSlug != "psp" {
		t.Fatalf("expected psp system, got %q", record.Summary.SystemSlug)
	}
	if record.Summary.Filename != "Lumines (USA).bin" {
		t.Fatalf("expected loose filename preserved, got %q", record.Summary.Filename)
	}
	if got := testPSPMetadataString(record.Summary.Metadata, "saveDir"); got != "" {
		t.Fatalf("expected no rsm.psp metadata on a loose upload, got saveDir %q", got)
	}
	if got := canonicalListKeyForRecord(record); got != "psp::lumines::"+regionUS {
		t.Fatalf("expected title-keyed list key for loose upload, got %q", got)
	}
	download := h.request(http.MethodGet, "/saves/download?id="+record.Summary.ID, nil)
	assertStatus(t, download, http.StatusOK)
	if !bytes.Equal(download.Body.Bytes(), payload) {
		t.Fatalf("expected loose download to return the stored bytes")
	}
}

func TestPSPSaveDataBundleSurvivesRescan(t *testing.T) {
	h := newContractHarness(t)
	bundle := buildTestPSPBundle(t, "ULUS10437DATA00", "ULUS10437DATA00", "DISSIDIA FINAL FANTASY", "rescan")
	body := uploadSave(t, h, "/saves", pspBundleFields("ULUS10437DATA00"), "ULUS10437DATA00.pspsave", bundle)
	saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")

	if _, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true}); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	record := mustFindRecordByID(t, h, saveID)
	if got := canonicalListKeyForRecord(record); got != "psp::savedir::ULUS10437DATA00" {
		t.Fatalf("expected bundle to keep its save-folder key after rescan, got %q", got)
	}
}

// PPSSPP's folders are mixed case (ULUS10437GameData00), and a client restores into the
// folder named by rsm.psp.saveDir, so the stored name keeps the case the helper sent, on
// upload and after a rescan. Comparisons and track keys stay case-insensitive.
func TestPSPSaveDataBundleKeepsSaveDirCase(t *testing.T) {
	h := newContractHarness(t)
	const dir = "ULUS10437GameData00"
	bundle := buildTestPSPBundle(t, dir, dir, "DISSIDIA FINAL FANTASY", "case")
	body := uploadSave(t, h, "/saves", pspBundleFields(dir), dir+".pspsave", bundle)
	saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")

	record := mustFindRecordByID(t, h, saveID)
	if got := testPSPMetadataString(record.Summary.Metadata, "saveDir"); got != dir {
		t.Fatalf("expected rsm.psp.saveDir=%s after upload, got %q", dir, got)
	}
	if got := canonicalListKeyForRecord(record); got != "psp::savedir::ULUS10437GAMEDATA00" {
		t.Fatalf("expected a case-insensitive track key, got %q", got)
	}

	if _, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true}); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	record = mustFindRecordByID(t, h, saveID)
	if got := testPSPMetadataString(record.Summary.Metadata, "saveDir"); got != dir {
		t.Fatalf("expected rsm.psp.saveDir=%s after rescan, got %q", dir, got)
	}
}
