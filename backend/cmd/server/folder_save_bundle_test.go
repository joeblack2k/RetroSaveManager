package main

import (
	"bytes"
	"encoding/binary"
	"net/http"
	"testing"
	"unicode/utf16"
)

// Dolphin keeps a Wii title's save in its NAND (title/<hi>/<lo>/data/) and Vita3K
// a Vita title's under ux0/user/00/savedata/<TITLEID>/. These uploads are the
// flat zips a helper sends for such a folder, with folderSaveDir naming it.

func testWiiBanner(title string) []byte {
	banner := make([]byte, 0xF0A0)
	copy(banner[0:4], wiiBannerMagic)
	units := utf16.Encode([]rune(title))
	for i, unit := range units {
		binary.BigEndian.PutUint16(banner[wiiBannerTitleOffset+2*i:], unit)
	}
	return banner
}

func testWiiNANDBundle(t *testing.T, seed string) []byte {
	t.Helper()
	save := bytes.Repeat([]byte(seed), 4096)
	return buildTestPSPZip(t, []testPSPZipEntry{
		{name: "banner.bin", data: testWiiBanner("Cave Story")},
		{name: "savedata.dat", data: save},
	})
}

func testVitaBundle(t *testing.T, seed string) []byte {
	t.Helper()
	return buildTestPSPZip(t, []testPSPZipEntry{
		{name: "SAVEDATA.BIN", data: bytes.Repeat([]byte(seed), 2048)},
	})
}

func folderBundleFields(system, saveDir string) map[string]string {
	return map[string]string{
		"system":        system,
		"folderSaveDir": saveDir,
		"rom_sha1":      system + "-folder:" + saveDir,
		"slotName":      "default",
	}
}

func testFolderMetadataString(metadata any, key string) string {
	root, ok := metadata.(map[string]any)
	if !ok {
		return ""
	}
	rsm, ok := root["rsm"].(map[string]any)
	if !ok {
		return ""
	}
	folder, ok := rsm["folder"].(map[string]any)
	if !ok {
		return ""
	}
	value, _ := folder[key].(string)
	return value
}

// A Dolphin NAND title folder uploads as one wii record named after its banner,
// keyed by the NAND title, and downloads byte for byte.
func TestWiiNANDFolderBundleStoresOneRecordAndDownloadsIdentically(t *testing.T) {
	h := newContractHarness(t)
	before := len(h.app.snapshotSaveRecords())
	bundle := testWiiNANDBundle(t, "cave")

	body := uploadSave(t, h, "/saves", folderBundleFields("wii", "00010001/57435645"), "57435645.wiinand", bundle)
	saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
	if got := len(h.app.snapshotSaveRecords()) - before; got != 1 {
		t.Fatalf("expected exactly one new record for a NAND folder, got %d", got)
	}
	record := mustFindRecordByID(t, h, saveID)
	if record.Summary.SystemSlug != "wii" {
		t.Fatalf("expected wii system, got %q", record.Summary.SystemSlug)
	}
	if got := testFolderMetadataString(record.Summary.Metadata, "saveDir"); got != "00010001/57435645" {
		t.Fatalf("expected rsm.folder.saveDir=00010001/57435645, got %q (metadata=%+v)", got, record.Summary.Metadata)
	}
	if got := testFolderMetadataString(record.Summary.Metadata, "bundle"); got != folderSaveBundleKind {
		t.Fatalf("expected rsm.folder.bundle=%s, got %q", folderSaveBundleKind, got)
	}
	if record.Summary.DisplayTitle != "Cave Story" {
		t.Fatalf("expected the banner's title, got %q", record.Summary.DisplayTitle)
	}
	if got := canonicalListKeyForRecord(record); got != "wii::folder::00010001/57435645" {
		t.Fatalf("expected list key wii::folder::00010001/57435645, got %q", got)
	}
	download := h.request(http.MethodGet, "/saves/download?id="+saveID, nil)
	assertStatus(t, download, http.StatusOK)
	if !bytes.Equal(download.Body.Bytes(), bundle) {
		t.Fatalf("expected a byte-identical download (%d bytes), got %d", len(bundle), download.Body.Len())
	}
}

// A Vita3K savedata folder uploads as one psvita record keyed by its title ID.
func TestVitaFolderBundleStoresOneRecordKeyedByTitleID(t *testing.T) {
	h := newContractHarness(t)
	bundle := testVitaBundle(t, "vita")
	body := uploadSave(t, h, "/saves", folderBundleFields("psvita", "PCSE00746"), "PCSE00746.vitasave", bundle)
	record := mustFindRecordByID(t, h, mustString(t, mustObject(t, body["save"], "save")["id"], "save.id"))
	if record.Summary.SystemSlug != "psvita" {
		t.Fatalf("expected psvita system, got %q", record.Summary.SystemSlug)
	}
	if got := canonicalListKeyForRecord(record); got != "psvita::folder::pcse00746" {
		t.Fatalf("expected list key psvita::folder::pcse00746, got %q", got)
	}
}

// A folder id that is not the system's form is refused, so a restore can never be
// pointed outside the emulator's own folder.
func TestFolderBundleRejectsAnInvalidFolderID(t *testing.T) {
	h := newContractHarness(t)
	for _, tc := range []struct{ system, dir string }{
		{"wii", "../../etc"},
		{"wii", "57435645"},
		{"psvita", "PCSE0074"},
		{"psvita", "../PCSE00746"},
	} {
		rr := h.multipart("/saves", folderBundleFields(tc.system, tc.dir), "file", "x.zip", testVitaBundle(t, "x"))
		assertStatus(t, rr, http.StatusUnprocessableEntity)
	}
}

// A folder bundle for a system that keeps no folder saves is refused, not stored loose.
func TestFolderBundleRejectsASystemWithoutFolderSaves(t *testing.T) {
	h := newContractHarness(t)
	rr := h.multipart("/saves", folderBundleFields("snes", "00010001/57435645"), "file", "x.zip", testWiiNANDBundle(t, "x"))
	assertStatus(t, rr, http.StatusUnprocessableEntity)
}

// A folder whose every file is all zero is a folder an emulator made and no game
// wrote: refused as blank.
func TestFolderBundleRejectsABlankFolder(t *testing.T) {
	h := newContractHarness(t)
	blank := buildTestPSPZip(t, []testPSPZipEntry{{name: "savedata.dat", data: make([]byte, 4096)}})
	rr := h.multipart("/saves", folderBundleFields("wii", "00010001/57435645"), "file", "57435645.wiinand", blank)
	assertStatus(t, rr, http.StatusUnprocessableEntity)
}

// A Wii data.bin upload, with no folderSaveDir, is handled exactly as before.
func TestWiiDataBinWithoutFolderSaveDirIsUnchanged(t *testing.T) {
	if _, ok, _ := folderSaveBundleFromForm([]byte("x"), func(key string) string {
		if key == "system" {
			return "wii"
		}
		return ""
	}); ok {
		t.Fatal("an upload without folderSaveDir must not be read as a folder bundle")
	}
}

// A bundle keeps its folder key after a rescan rewrites the store.
func TestFolderBundleSurvivesRescan(t *testing.T) {
	h := newContractHarness(t)
	body := uploadSave(t, h, "/saves", folderBundleFields("wii", "00010001/57435645"), "57435645.wiinand", testWiiNANDBundle(t, "rescan"))
	saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
	if _, err := h.app.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: true}); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	if err := h.app.reloadSavesFromDisk(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := canonicalListKeyForRecord(mustFindRecordByID(t, h, saveID)); got != "wii::folder::00010001/57435645" {
		t.Fatalf("expected the folder key to survive a rescan, got %q", got)
	}
}
