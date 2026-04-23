package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSaturnQuakeFixtureExtractsMetadata(t *testing.T) {
	payload := loadSaturnFixture(t, "saturn_quake_usa.sav")

	parsed := parseSaturnContainer("Quake (USA).sav", payload)
	if parsed == nil {
		t.Fatal("expected Saturn container details")
	}
	if parsed.Details == nil {
		t.Fatal("expected Saturn container summary")
	}
	if parsed.Details.Format != "mister-combined-interleaved" {
		t.Fatalf("unexpected Saturn format: %q", parsed.Details.Format)
	}
	if parsed.Details.SaveEntries != 1 {
		t.Fatalf("expected one Saturn save entry, got %d", parsed.Details.SaveEntries)
	}
	if parsed.Details.DefaultVolume != "internal" {
		t.Fatalf("unexpected default volume: %q", parsed.Details.DefaultVolume)
	}
	if len(parsed.Details.Volumes) != 2 {
		t.Fatalf("expected internal + cartridge volume summaries, got %d", len(parsed.Details.Volumes))
	}
	if parsed.Internal == nil || !parsed.Internal.Summary.HeaderValid {
		t.Fatalf("expected valid internal Saturn volume, got %#v", parsed.Internal)
	}
	if parsed.Cartridge == nil {
		t.Fatal("expected optional cartridge summary")
	}
	entry := parsed.Details.Entries[0]
	if entry.Volume != "internal" {
		t.Fatalf("unexpected entry volume: %q", entry.Volume)
	}
	if entry.Filename != "LOBOQUAKE__" {
		t.Fatalf("unexpected entry filename: %q", entry.Filename)
	}
	if entry.Comment != "save games" {
		t.Fatalf("unexpected entry comment: %q", entry.Comment)
	}
	if entry.LanguageCode != "EN" {
		t.Fatalf("unexpected entry language: %q", entry.LanguageCode)
	}
	if entry.SaveSizeBytes != 1408 {
		t.Fatalf("unexpected entry payload size: %d", entry.SaveSizeBytes)
	}
	if entry.BlockCount <= 0 {
		t.Fatalf("expected positive block count, got %d", entry.BlockCount)
	}
	if entry.Date == "" {
		t.Fatal("expected parsed Saturn entry timestamp")
	}
}

func TestDetectSaveSystemRecognizesSaturnBackupRAM(t *testing.T) {
	payload := loadSaturnFixture(t, "saturn_quake_usa.sav")

	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename: "Quake (USA).sav",
		Payload:  payload,
	})
	if detected.Slug != "saturn" {
		t.Fatalf("expected saturn slug, got %q", detected.Slug)
	}
	if detected.System == nil || detected.System.Slug != "saturn" {
		t.Fatalf("expected Saturn system details, got %#v", detected.System)
	}
	if !detected.Evidence.Payload {
		t.Fatalf("expected payload evidence, got %#v", detected.Evidence)
	}
}

func TestNormalizeSaveInputAcceptsSaturnAndSetsParserMetadata(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename: "Quake (USA).sav",
		Payload:  loadSaturnFixture(t, "saturn_quake_usa.sav"),
		Game:     game{Name: "Quake (USA)"},
		ROMSHA1:  "saturn-quake-rom-sha1",
		SlotName: "default",
	})
	if result.Rejected {
		t.Fatalf("expected Saturn save to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.SystemSlug != "saturn" {
		t.Fatalf("expected saturn system slug, got %q", result.Input.SystemSlug)
	}
	if result.Input.Saturn == nil {
		t.Fatal("expected Saturn metadata on normalized input")
	}
	if !result.Input.Game.HasParser {
		t.Fatal("expected parser flag to be set")
	}
	if result.Input.DisplayTitle != "Quake" {
		t.Fatalf("expected cleaned display title, got %q", result.Input.DisplayTitle)
	}
	if result.Input.RegionCode != regionUS {
		t.Fatalf("expected US region, got %q", result.Input.RegionCode)
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected Saturn inspection metadata")
	}
	if result.Input.Inspection.ParserLevel != saveParserLevelStructural {
		t.Fatalf("unexpected parser level: %q", result.Input.Inspection.ParserLevel)
	}
	if result.Input.Inspection.ParserID != "saturn-backup-ram" {
		t.Fatalf("unexpected parser id: %q", result.Input.Inspection.ParserID)
	}
	meta, ok := result.Input.Metadata.(map[string]any)
	if !ok {
		t.Fatalf("expected metadata map, got %#v", result.Input.Metadata)
	}
	rsm := mustObject(t, meta["rsm"], "metadata.rsm")
	if _, ok := rsm["saturn"]; !ok {
		t.Fatalf("expected saturn metadata under rsm, got %v", rsm)
	}
}

func TestNormalizeSaveInputRejectsEmptySaturnBackupRAM(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Fighting Vipers (USA) (6S).sav",
		Payload:             loadSaturnFixture(t, "saturn_fighting_vipers_usa_6s.sav"),
		Game:                game{Name: "Fighting Vipers (USA) (6S)"},
		SystemSlug:          "saturn",
		TrustedHelperSystem: true,
		ROMSHA1:             "saturn-fighting-vipers-rom-sha1",
		SlotName:            "default",
	})
	if !result.Rejected {
		t.Fatal("expected empty Saturn backup RAM image to be rejected")
	}
	if result.RejectReason != "saturn backup RAM image has no active save entries" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestSaturnDownloadPayloadExportsYabauseAndYabaSanshiro(t *testing.T) {
	payload := loadSaturnFixture(t, "saturn_quake_usa.sav")
	record := saveRecord{Summary: saveSummary{Filename: "Quake (USA).sav", DisplayTitle: "Quake"}}

	_, _, yabausePayload, err := saturnDownloadPayload(record, payload, "yabause", "")
	if err != nil {
		t.Fatalf("export yabause: %v", err)
	}
	if len(yabausePayload) != saturnInternalInterleavedSize {
		t.Fatalf("unexpected Yabause payload size: %d", len(yabausePayload))
	}
	if parsed := parseSaturnContainer("quake_yabause.sav", yabausePayload); parsed == nil || parsed.Details == nil || parsed.Details.SaveEntries != 1 {
		t.Fatalf("expected Yabause export to parse back into one Saturn save, got %#v", parsed)
	}

	_, _, yabaSanshiroPayload, err := saturnDownloadPayload(record, payload, "yabasanshiro", "")
	if err != nil {
		t.Fatalf("export yabasanshiro: %v", err)
	}
	if len(yabaSanshiroPayload) != saturnYabaSanshiroExpandedSize {
		t.Fatalf("unexpected Yaba Sanshiro payload size: %d", len(yabaSanshiroPayload))
	}
	if parsed := parseSaturnContainer("quake_yabasanshiro.sav", yabaSanshiroPayload); parsed == nil || parsed.Details == nil || parsed.Details.SaveEntries != 1 {
		t.Fatalf("expected Yaba Sanshiro export to parse back into one Saturn save, got %#v", parsed)
	}
}

func TestContractSavesMultipartAcceptsSaturnBackupRAMAndListsMetadata(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "saturn-helper")

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "saturn-quake-rom-sha1",
		"slotName":     "default",
		"system":       "saturn",
		"device_type":  "mister",
		"fingerprint":  "saturn-device",
		"runtimeProfile": "saturn/mister",
	}, "Quake (USA).sav", loadSaturnFixture(t, "saturn_quake_usa.sav"))

	save := mustObject(t, body["save"], "save")
	saveID := mustString(t, save["id"], "save.id")

	list := h.request(http.MethodGet, "/saves?limit=10&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	items := mustArray(t, listBody["saves"], "saves")
	if len(items) == 0 {
		t.Fatal("expected uploaded Saturn save in list")
	}
	first := mustObject(t, items[0], "items[0]")
	if mustString(t, first["id"], "items[0].id") != saveID {
		t.Fatalf("expected first save to be Saturn upload, got %s", prettyJSON(first))
	}
	if mustString(t, first["systemSlug"], "items[0].systemSlug") != "saturn" {
		t.Fatalf("unexpected system slug: %s", prettyJSON(first))
	}
	if mustString(t, first["displayTitle"], "items[0].displayTitle") != "Quake" {
		t.Fatalf("unexpected display title: %s", prettyJSON(first))
	}
	if mustString(t, first["regionCode"], "items[0].regionCode") != regionUS {
		t.Fatalf("unexpected region code: %s", prettyJSON(first))
	}
	saturn := mustObject(t, first["saturn"], "items[0].saturn")
	if mustString(t, saturn["container"], "items[0].saturn.container") != "backup-ram" {
		t.Fatalf("unexpected saturn payload: %s", prettyJSON(first))
	}
	if mustNumber(t, saturn["saveEntries"], "items[0].saturn.saveEntries") != 1 {
		t.Fatalf("unexpected Saturn save count: %s", prettyJSON(first))
	}
	inspection := mustObject(t, first["inspection"], "items[0].inspection")
	if mustString(t, inspection["parserId"], "items[0].inspection.parserId") != "saturn-backup-ram" {
		t.Fatalf("unexpected Saturn inspection payload: %s", prettyJSON(first))
	}
}

func TestContractSavesMultipartRejectsEmptySaturnBackupRAM(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "saturn-helper")

	rr := h.multipart("/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "saturn-fighting-vipers-rom-sha1",
		"slotName":     "default",
		"system":       "saturn",
		"device_type":  "mister",
		"fingerprint":  "saturn-device",
		"runtimeProfile": "saturn/mister",
	}, "file", "Fighting Vipers (USA) (6S).sav", loadSaturnFixture(t, "saturn_fighting_vipers_usa_6s.sav"))
	assertStatus(t, rr, http.StatusUnprocessableEntity)
	assertJSONContentType(t, rr)
}

func TestContractSaturnDownloadAndLatestSupportFormatConversion(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "saturn-helper")
	payload := loadSaturnFixture(t, "saturn_quake_usa.sav")

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "saturn-quake-rom-sha1",
		"slotName":     "default",
		"system":       "saturn",
		"device_type":  "mister",
		"fingerprint":  "saturn-device",
		"runtimeProfile": "saturn/mister",
	}, "Quake (USA).sav", payload)
	save := mustObject(t, body["save"], "save")
	saveID := mustString(t, save["id"], "save.id")

	expectedInternal := collapseSaturnByteExpanded(payload[:saturnInternalInterleavedSize])
	download := h.request(http.MethodGet, "/saves/download?id="+saveID+"&saturnFormat=internal-raw", nil)
	assertStatus(t, download, http.StatusOK)
	if got := download.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("unexpected content type: %q", got)
	}
	if !bytes.Equal(download.Body.Bytes(), expectedInternal) {
		t.Fatal("expected internal-raw download to match collapsed Saturn internal backup RAM")
	}

	ymir := h.request(http.MethodGet, "/saves/download?id="+saveID+"&saturnFormat=ymir&saturnEntry=LOBOQUAKE__", nil)
	assertStatus(t, ymir, http.StatusOK)
	if !bytes.HasPrefix(ymir.Body.Bytes(), []byte("Vmem")) {
		t.Fatalf("expected Ymir/BUP export to begin with Vmem magic, got %q", ymir.Body.Bytes()[:min(4, len(ymir.Body.Bytes()))])
	}

	latest := h.request(http.MethodGet, "/save/latest?romSha1=saturn-quake-rom-sha1&slotName=default&saturnFormat=internal-raw", nil)
	assertStatus(t, latest, http.StatusOK)
	latestBody := decodeJSONMap(t, latest.Body)
	if !mustBool(t, latestBody["exists"], "exists") {
		t.Fatalf("expected latest Saturn save to exist: %s", prettyJSON(latestBody))
	}
	sum := sha256.Sum256(expectedInternal)
	expectedSHA := hex.EncodeToString(sum[:])
	if mustString(t, latestBody["sha256"], "sha256") != expectedSHA {
		t.Fatalf("unexpected latest sha: %s", prettyJSON(latestBody))
	}
}

func loadSaturnFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Saturn fixture %s: %v", name, err)
	}
	return payload
}

func collapseSaturnByteExpanded(payload []byte) []byte {
	out := make([]byte, len(payload)/2)
	for i := 0; i < len(out); i++ {
		out[i] = payload[i*2+1]
	}
	return out
}
