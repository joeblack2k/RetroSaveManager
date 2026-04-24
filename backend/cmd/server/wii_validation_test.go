package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"testing"
)

func TestNormalizeSaveInputAcceptsWiiDataBinWithTitleCodeEvidence(t *testing.T) {
	a := &app{}
	metadata, titleCode := wiiUploadMetadata(nil, "Super Mario Galaxy 2/SB4P/data.bin", nil)
	if titleCode != "SB4P" {
		t.Fatalf("expected title code from path, got %q", titleCode)
	}

	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:   "data.bin",
		Payload:    buildWiiDataBinFixture(),
		Game:       wiiGameFromTitleCode(titleCode),
		Format:     "bin",
		Metadata:   metadata,
		SlotName:   "Super Mario Galaxy 2/SB4P/data.bin",
		SystemSlug: "wii",
	})
	if result.Rejected {
		t.Fatalf("expected Wii data.bin to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.SystemSlug != "wii" {
		t.Fatalf("expected wii system, got %q", result.Input.SystemSlug)
	}
	if result.Input.DisplayTitle != "Super Mario Galaxy 2" {
		t.Fatalf("expected catalog title, got %q", result.Input.DisplayTitle)
	}
	if result.Input.RegionCode != regionEU {
		t.Fatalf("expected EU region from SB4P, got %q", result.Input.RegionCode)
	}
	inspection := result.Input.Inspection
	if inspection == nil {
		t.Fatal("expected Wii inspection metadata")
	}
	if inspection.ParserID != wiiDataBinParserID || inspection.ParserLevel != saveParserLevelStructural {
		t.Fatalf("unexpected inspection: %+v", inspection)
	}
	if inspection.TrustLevel != wiiTrustLevelStructureVerified {
		t.Fatalf("expected structure trust, got %q", inspection.TrustLevel)
	}
	if got := inspection.SemanticFields["titleCode"]; got != "SB4P" {
		t.Fatalf("expected titleCode semantic field, got %+v", inspection.SemanticFields)
	}
	if got := inspection.SemanticFields["semanticVerified"]; got != false {
		t.Fatalf("expected semanticVerified=false for encrypted data.bin, got %+v", inspection.SemanticFields)
	}
}

func TestNormalizeSaveInputMarksWiiROMVerifiedWhenROMSHA1Present(t *testing.T) {
	a := &app{}
	metadata, _ := wiiUploadMetadata(nil, "private/wii/title/SB4P/data.bin", nil)
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:   "data.bin",
		Payload:    buildWiiDataBinFixture(),
		Game:       wiiGameFromTitleCode("SB4P"),
		Format:     "bin",
		Metadata:   metadata,
		ROMSHA1:    "0123456789abcdef0123456789abcdef01234567",
		SystemSlug: "wii",
	})
	if result.Rejected {
		t.Fatalf("expected Wii data.bin to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.Inspection == nil || result.Input.Inspection.TrustLevel != wiiTrustLevelROMVerified {
		t.Fatalf("expected rom-verified trust, got %+v", result.Input.Inspection)
	}
	levels, ok := result.Input.Inspection.SemanticFields["verificationLevels"].([]string)
	if !ok {
		t.Fatalf("expected verification levels, got %+v", result.Input.Inspection.SemanticFields["verificationLevels"])
	}
	if !containsString(levels, wiiTrustLevelROMVerified) {
		t.Fatalf("expected rom-verified level in %+v", levels)
	}
}

func TestNormalizeSaveInputRejectsInvalidWiiDataBin(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:   "data.bin",
		Payload:    buildNonBlankPayload(4096, 0x44),
		Game:       wiiGameFromTitleCode("SB4P"),
		Format:     "bin",
		SystemSlug: "wii",
	})
	if !result.Rejected {
		t.Fatal("expected invalid Wii data.bin to be rejected")
	}
	if result.RejectReason != "wii data.bin payload is too small" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestMultipartZipUploadImportsWiiDataBin(t *testing.T) {
	h := newContractHarness(t)
	payload := buildZipFixture(t, map[string][]byte{
		"Super Mario Galaxy 2/.DS_Store":     []byte("noise"),
		"Super Mario Galaxy 2/SB4P/data.bin": buildWiiDataBinFixture(),
	})

	rr := h.multipart("/saves", map[string]string{}, "file", "smg2.zip", payload)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected zip upload 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["success"] != true {
		t.Fatalf("expected success response, got %+v", response)
	}
	items := h.app.snapshotSaveRecords()
	if len(items) == 0 {
		t.Fatal("expected imported save record")
	}
	found := false
	for _, record := range items {
		if record.Summary.SystemSlug == "wii" && record.Summary.DisplayTitle == "Super Mario Galaxy 2" {
			found = true
			if record.Summary.Inspection == nil || record.Summary.Inspection.ParserID != wiiDataBinParserID {
				t.Fatalf("expected Wii inspection on record, got %+v", record.Summary.Inspection)
			}
		}
	}
	if !found {
		t.Fatalf("expected Super Mario Galaxy 2 Wii record, got %+v", items)
	}
}

func buildWiiDataBinFixture() []byte {
	payload := buildNonBlankPayload(75200, 0x5A)
	for i := range payload {
		payload[i] = byte((i*31 + 7) & 0xFF)
	}
	binary.BigEndian.PutUint32(payload[wiiDataBinBackupHeaderOffset:wiiDataBinBackupHeaderOffset+4], 0x70)
	copy(payload[wiiDataBinBackupHeaderOffset+4:wiiDataBinBackupHeaderOffset+8], []byte{'B', 'k', 0x00, 0x01})
	binary.BigEndian.PutUint32(payload[wiiDataBinBackupHeaderOffset+0x0C:wiiDataBinBackupHeaderOffset+0x10], 1)
	binary.BigEndian.PutUint32(payload[wiiDataBinBackupHeaderOffset+0x10:wiiDataBinBackupHeaderOffset+0x14], 0x3140)
	binary.BigEndian.PutUint32(payload[wiiDataBinFileHeaderOffset:wiiDataBinFileHeaderOffset+4], wiiDataBinFileHeaderMagic)
	binary.BigEndian.PutUint32(payload[wiiDataBinFileHeaderOffset+4:wiiDataBinFileHeaderOffset+8], 0x30A0)
	copy(payload[wiiDataBinFileHeaderOffset+0x0B:wiiDataBinFileHeaderOffset+0x0B+len("GameData.bin")], []byte("GameData.bin"))
	copy(payload[len(payload)-640:], []byte("Root-CA00000001-MS00000002-NG02"))
	return payload
}

func buildZipFixture(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, payload := range files {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip part: %v", err)
		}
		if _, err := part.Write(payload); err != nil {
			t.Fatalf("write zip part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
