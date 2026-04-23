package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"testing"
)

func TestNormalizeN64ProjectionUploadRetroArchSRMToCanonicalSRAM(t *testing.T) {
	canonical := buildTestN64Payload("sra", "retroarch-sram")
	input := saveCreateInput{
		Filename:   "Mario Kart 64.srm",
		Payload:    buildRetroArchN64SRM(n64CanonicalMediaByType["sram"], canonical),
		SystemSlug: "n64",
	}
	got, err := normalizeN64ProjectionUpload(input, n64ProfileRetroArch)
	if err != nil {
		t.Fatalf("normalize retroarch upload: %v", err)
	}
	if got.Filename != "Mario Kart 64.sra" {
		t.Fatalf("unexpected canonical filename: %q", got.Filename)
	}
	if got.MediaType != "sram" {
		t.Fatalf("unexpected media type: %q", got.MediaType)
	}
	if got.SourceArtifactProfile != n64ProfileRetroArch {
		t.Fatalf("unexpected source artifact profile: %q", got.SourceArtifactProfile)
	}
	if got.ProjectionCapable == nil || !*got.ProjectionCapable {
		t.Fatalf("expected projection-capable flag, got %+v", got.ProjectionCapable)
	}
	if string(got.Payload) != string(canonical) {
		t.Fatal("expected retroarch SRM upload to normalize back to canonical SRAM bytes")
	}
}

func TestProjectN64PayloadRetroArchBuildsSRM(t *testing.T) {
	canonical := buildTestN64Payload("sra", "retroarch-download")
	summary := saveSummary{Filename: "Wave Race 64.sra", MediaType: "sram", FileSize: len(canonical)}
	filename, _, projected, err := projectN64Payload(summary, canonical, n64ProfileRetroArch)
	if err != nil {
		t.Fatalf("project retroarch payload: %v", err)
	}
	if filename != "Wave Race 64.srm" {
		t.Fatalf("unexpected projected filename: %q", filename)
	}
	if len(projected) != n64RetroArchSRMSize {
		t.Fatalf("unexpected SRM size: %d", len(projected))
	}
	roundTrip, info, err := splitRetroArchN64SRM(projected)
	if err != nil {
		t.Fatalf("split projected SRM: %v", err)
	}
	if info.MediaType != "sram" {
		t.Fatalf("unexpected split media type: %q", info.MediaType)
	}
	if string(roundTrip) != string(canonical) {
		t.Fatal("expected retroarch projection to round-trip to canonical SRAM")
	}
}

func TestHelperN64DownloadRequiresProfile(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "n64-download")
	record, err := h.app.createSave(saveCreateInput{
		Filename:            "Mario Kart 64.eep",
		Payload:             normalizeN64EEPROM(buildTestN64Payload("eep", "mk64-helper")),
		Game:                game{Name: "Mario Kart 64"},
		ROMSHA1:             "mk64-rom",
		SlotName:            "default",
		SystemSlug:          "n64",
		TrustedHelperSystem: true,
		MediaType:           "eeprom",
	})
	if err != nil {
		t.Fatalf("create N64 save: %v", err)
	}
	rr := helperGET(t, h, "/saves/download?id="+url.QueryEscape(record.Summary.ID)+"&device_type=mister&fingerprint=test-device", helperKey)
	assertStatus(t, rr, http.StatusBadRequest)
}

func TestHelperN64LatestAndDownloadUseRequestedProfile(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "n64-projection")
	fields := map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "n64-profile-rom",
		"slotName":     "default",
		"system":       "n64",
		"device_type":  "mister",
		"fingerprint":  "mister-n64-1",
		"n64Profile":   n64ProfileMister,
	}
	upload := h.multipart("/saves", fields, "file", "Star Fox 64.sra", buildTestN64Payload("sra", "star-fox-mister"))
	assertStatus(t, upload, http.StatusOK)
	uploadBody := decodeJSONMap(t, upload.Body)
	saveObj := mustObject(t, uploadBody["save"], "save")
	saveID := mustString(t, saveObj["id"], "save.id")

	download := helperGET(t, h, "/saves/download?id="+url.QueryEscape(saveID)+"&device_type=mister&fingerprint=mister-n64-1&n64Profile="+url.QueryEscape(n64ProfileRetroArch), helperKey)
	assertStatus(t, download, http.StatusOK)
	if got := download.Header().Get("Content-Disposition"); got == "" || got == "attachment; filename=\"Star Fox 64.sra\"" {
		t.Fatalf("expected projected retroarch filename, got %q", got)
	}
	if len(download.Body.Bytes()) != n64RetroArchSRMSize {
		t.Fatalf("unexpected retroarch download size: %d", len(download.Body.Bytes()))
	}
	sum := sha256.Sum256(download.Body.Bytes())
	wantSHA := hex.EncodeToString(sum[:])

	latest := helperGET(t, h, "/save/latest?romSha1=n64-profile-rom&slotName=default&device_type=mister&fingerprint=mister-n64-1&n64Profile="+url.QueryEscape(n64ProfileRetroArch), helperKey)
	assertStatus(t, latest, http.StatusOK)
	latestBody := decodeJSONMap(t, latest.Body)
	if got := mustString(t, latestBody["sha256"], "sha256"); got != wantSHA {
		t.Fatalf("unexpected latest projected sha256: got %s want %s", got, wantSHA)
	}
}
