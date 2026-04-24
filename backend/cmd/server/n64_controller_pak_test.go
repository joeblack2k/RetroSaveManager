package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/clktmr/n64/drivers/controller/pakfs"
)

type testN64ControllerPakEntry struct {
	Name          string
	GameCode      string
	PublisherCode string
	Payload       []byte
}

func makeTestN64ControllerPak(t *testing.T, entries ...testN64ControllerPakEntry) []byte {
	t.Helper()
	buf := make(n64ControllerPakMemBuffer, n64RetroArchControllerPakSize)
	n64InitControllerPak(buf)
	fsys, err := pakfs.Read(buf)
	if err != nil {
		t.Fatalf("read initialized controller pak: %v", err)
	}
	for _, entry := range entries {
		file, err := fsys.Create(entry.Name)
		if err != nil {
			t.Fatalf("create controller pak entry %q: %v", entry.Name, err)
		}
		var gameCode [4]byte
		copy(gameCode[:], []byte(strings.ToUpper(strings.TrimSpace(entry.GameCode))))
		if err := file.SetGameCode(gameCode); err != nil {
			t.Fatalf("set game code for %q: %v", entry.Name, err)
		}
		var publisherCode [2]byte
		copy(publisherCode[:], []byte(strings.ToUpper(strings.TrimSpace(entry.PublisherCode))))
		if err := file.SetCompanyCode(publisherCode); err != nil {
			t.Fatalf("set publisher code for %q: %v", entry.Name, err)
		}
		payload := append([]byte(nil), entry.Payload...)
		if len(payload) == 0 {
			payload = buildTestN64Payload("eep", entry.Name)
		}
		if rem := len(payload) % 256; rem != 0 {
			padding := make([]byte, 256-rem)
			payload = append(payload, padding...)
		}
		if _, err := file.WriteAt(payload, 0); err != nil {
			t.Fatalf("write controller pak payload for %q: %v", entry.Name, err)
		}
	}
	if count, err := countN64ControllerPakEntries(buf); err != nil {
		t.Fatalf("validate built controller pak: %v", err)
	} else if count != len(entries) {
		t.Fatalf("expected %d controller pak entries, got %d", len(entries), count)
	}
	return append([]byte(nil), buf...)
}

func TestValidateN64ControllerPakRejectsEmptyCPK(t *testing.T) {
	input := saveCreateInput{
		Filename:   "empty.cpk",
		Payload:    makeTestN64ControllerPak(t),
		SystemSlug: "n64",
	}
	result := validateN64Save(input, saveSystemDetectionResult{
		Slug:   "n64",
		System: supportedSystemFromSlug("n64"),
	})
	if !result.Rejected {
		t.Fatal("expected empty controller pak to be rejected")
	}
	if !strings.Contains(strings.ToLower(result.RejectReason), "does not contain any save entries") {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestN64ControllerPakUploadListsLogicalEntriesInsteadOfProjectionRows(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "n64-cpk-list")
	pak := makeTestN64ControllerPak(t,
		testN64ControllerPakEntry{Name: "MK64A", GameCode: "NKTE", PublisherCode: "01", Payload: buildTestN64Payload("eep", "mk64-entry")},
		testN64ControllerPakEntry{Name: "OOTA", GameCode: "CZLE", PublisherCode: "01", Payload: buildTestN64Payload("eep", "oot-entry")},
	)

	fields := map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "n64-cpk-rom",
		"slotName":     "controller-1",
		"system":       "n64",
		"device_type":  "mister",
		"fingerprint":  "mister-cpk-1",
		"n64Profile":   n64ProfileMister,
	}
	upload := h.multipart("/saves", fields, "file", "MARIO.cpk", pak)
	assertStatus(t, upload, http.StatusOK)

	list := h.request(http.MethodGet, "/saves?romSha1=n64-cpk-rom&limit=50&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	body := decodeJSONMap(t, list.Body)
	saves := mustArray(t, body["saves"], "saves")
	if len(saves) != 2 {
		t.Fatalf("expected 2 logical controller pak saves, got %d: %s", len(saves), prettyJSON(body))
	}

	titles := map[string]map[string]any{}
	for _, raw := range saves {
		summary := mustObject(t, raw, "save")
		titles[mustString(t, summary["displayTitle"], "displayTitle")] = summary
		if mustString(t, summary["mediaType"], "mediaType") != "controller-pak" {
			t.Fatalf("expected controller-pak media type, got %s", prettyJSON(summary))
		}
		if mustString(t, summary["logicalKey"], "logicalKey") == "" {
			t.Fatalf("expected logicalKey on controller pak summary: %s", prettyJSON(summary))
		}
		entry := mustObject(t, summary["controllerPakEntry"], "controllerPakEntry")
		if mustString(t, entry["gameCode"], "controllerPakEntry.gameCode") == "" {
			t.Fatalf("expected gameCode on controller pak entry: %s", prettyJSON(summary))
		}
	}
	if _, ok := titles["Mario Kart 64"]; !ok {
		t.Fatalf("expected Mario Kart 64 in logical summaries, got %v", mapsKeys(titles))
	}
	if _, ok := titles["The Legend of Zelda: Ocarina of Time"]; !ok {
		t.Fatalf("expected Ocarina of Time in logical summaries, got %v", mapsKeys(titles))
	}
}

func TestN64ControllerPakHistoryAndRuntimeDownloadUseLogicalEntry(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "n64-cpk-history")
	pak := makeTestN64ControllerPak(t,
		testN64ControllerPakEntry{Name: "MK64A", GameCode: "NKTE", PublisherCode: "01", Payload: buildTestN64Payload("eep", "mk64-history")},
		testN64ControllerPakEntry{Name: "OOTA", GameCode: "CZLE", PublisherCode: "01", Payload: buildTestN64Payload("eep", "oot-history")},
	)
	fields := map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "n64-cpk-history-rom",
		"slotName":     "controller-1",
		"system":       "n64",
		"device_type":  "mister",
		"fingerprint":  "mister-cpk-2",
		"n64Profile":   n64ProfileMister,
	}
	upload := h.multipart("/saves", fields, "file", "history.cpk", pak)
	assertStatus(t, upload, http.StatusOK)

	list := h.request(http.MethodGet, "/saves?romSha1=n64-cpk-history-rom&limit=50&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	saves := mustArray(t, decodeJSONMap(t, list.Body)["saves"], "saves")
	first := mustObject(t, saves[0], "saves[0]")
	saveID := mustString(t, first["id"], "id")
	logicalKey := mustString(t, first["logicalKey"], "logicalKey")

	history := h.request(http.MethodGet, "/save?saveId="+url.QueryEscape(saveID)+"&psLogicalKey="+url.QueryEscape(logicalKey), nil)
	assertStatus(t, history, http.StatusOK)
	historyBody := decodeJSONMap(t, history.Body)
	versions := mustArray(t, historyBody["versions"], "versions")
	if len(versions) != 1 {
		t.Fatalf("expected 1 logical version, got %d: %s", len(versions), prettyJSON(historyBody))
	}

	download := helperGET(t, h, "/saves/download?id="+url.QueryEscape(saveID)+"&psLogicalKey="+url.QueryEscape(logicalKey)+"&device_type=mister&fingerprint=mister-cpk-2&n64Profile="+url.QueryEscape(n64ProfileRetroArch), helperKey)
	assertStatus(t, download, http.StatusOK)
	if got := download.Header().Get("Content-Disposition"); !strings.Contains(got, ".srm") {
		t.Fatalf("expected retroarch srm download, got %q", got)
	}
	if len(download.Body.Bytes()) != n64RetroArchSRMSize {
		t.Fatalf("unexpected retroarch controller pak size: %d", len(download.Body.Bytes()))
	}
}

func TestHelperN64ControllerPakLatestUsesRequestedProjectionProfile(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "n64-cpk-latest")
	pak := makeTestN64ControllerPak(t,
		testN64ControllerPakEntry{Name: "MK64A", GameCode: "NKTE", PublisherCode: "01", Payload: buildTestN64Payload("eep", "mk64-latest")},
	)
	fields := map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "n64-cpk-latest-rom",
		"slotName":     "controller-1",
		"system":       "n64",
		"device_type":  "mister",
		"fingerprint":  "mister-cpk-3",
		"n64Profile":   n64ProfileMister,
	}
	upload := h.multipart("/saves", fields, "file", "latest.cpk", pak)
	assertStatus(t, upload, http.StatusOK)
	uploadBody := decodeJSONMap(t, upload.Body)
	sourceSaveID := mustString(t, mustObject(t, uploadBody["save"], "save")["id"], "save.id")

	latest := helperGET(t, h, "/save/latest?romSha1=n64-cpk-latest-rom&slotName=controller-1&device_type=mister&fingerprint=mister-cpk-3&n64Profile="+url.QueryEscape(n64ProfileRetroArch), helperKey)
	assertStatus(t, latest, http.StatusOK)
	latestBody := decodeJSONMap(t, latest.Body)
	if !mustBool(t, latestBody["exists"], "exists") {
		t.Fatalf("expected latest projection to exist: %s", prettyJSON(latestBody))
	}
	if got := mustString(t, latestBody["id"], "id"); got == "" || got == sourceSaveID {
		t.Fatalf("expected latest to resolve retroarch projection record, got %q", got)
	}
}

func TestN64ControllerPakMPKUploadStillWorks(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "n64-mpk")
	pak := makeTestN64ControllerPak(t,
		testN64ControllerPakEntry{Name: "MK64A", GameCode: "NKTE", PublisherCode: "01", Payload: buildTestN64Payload("eep", "mk64-mpk")},
	)
	fields := map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "n64-mpk-rom",
		"slotName":     "controller-1",
		"system":       "n64",
		"device_type":  "project64",
		"fingerprint":  "project64-cpk-1",
		"n64Profile":   n64ProfileProject64,
	}
	upload := h.multipart("/saves", fields, "file", "legacy.mpk", pak)
	assertStatus(t, upload, http.StatusOK)

	list := h.request(http.MethodGet, "/saves?romSha1=n64-mpk-rom&limit=50&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	saves := mustArray(t, decodeJSONMap(t, list.Body)["saves"], "saves")
	if len(saves) != 1 {
		t.Fatalf("expected 1 logical save after .mpk upload, got %d", len(saves))
	}
}

func mapsKeys(in map[string]map[string]any) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	return keys
}
