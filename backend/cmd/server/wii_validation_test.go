package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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

func TestNormalizeSaveInputAcceptsSMG2RawGameDataReadOnly(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:     "GameData.bin",
		Payload:      buildSMG2RawGameDataFixture(),
		Format:       "bin",
		SystemSlug:   "wii",
		DisplayTitle: "Super Mario Galaxy 2",
	})
	if result.Rejected {
		t.Fatalf("expected raw GameData.bin to be accepted, got reject=%q", result.RejectReason)
	}
	inspection := result.Input.Inspection
	if inspection == nil {
		t.Fatal("expected raw GameData.bin inspection")
	}
	if inspection.ParserID != wiiSMG2GameDataParserID || inspection.ParserLevel != saveParserLevelStructural {
		t.Fatalf("unexpected inspection: %+v", inspection)
	}
	if inspection.ValidatedGameID != "wii/super-mario-galaxy-2" {
		t.Fatalf("expected SMG2 game id, got %q", inspection.ValidatedGameID)
	}
	if inspection.ChecksumValid == nil || !*inspection.ChecksumValid {
		t.Fatalf("expected checksum valid, got %+v", inspection.ChecksumValid)
	}
	if got := inspection.SemanticFields["readOnlyGameData"]; got != true {
		t.Fatalf("expected readOnlyGameData=true, got %+v", inspection.SemanticFields)
	}
}

func TestSMG2ModuleParsesRawGameDataReadOnly(t *testing.T) {
	root := findRepositoryModuleRoot(t)
	zipData, err := os.ReadFile(filepath.Join(root, "wii-super-mario-galaxy-2.rsmodule.zip"))
	if err != nil {
		t.Fatalf("read SMG2 module: %v", err)
	}
	service, err := newGameModuleService(t.TempDir())
	if err != nil {
		t.Fatalf("new module service: %v", err)
	}
	if _, err := service.importZip(context.Background(), zipData, gameModuleSourceInfo{Source: gameModuleSourceUploaded, SourcePath: "wii-super-mario-galaxy-2.rsmodule.zip"}); err != nil {
		t.Fatalf("import SMG2 module: %v", err)
	}
	inspection, ok := service.inspectSave(saveCreateInput{
		Filename:     "GameData.bin",
		Payload:      buildSMG2RawGameDataFixture(),
		Format:       "bin",
		SystemSlug:   "wii",
		DisplayTitle: "Super Mario Galaxy 2",
	}, nil)
	if !ok {
		t.Fatal("expected SMG2 module inspection")
	}
	if inspection.ParserID != "smg2-data-bin-wasm" || inspection.ParserLevel != saveParserLevelSemantic {
		t.Fatalf("unexpected module inspection: %+v", inspection)
	}
	if got := inspection.SemanticFields["readOnlyGameData"]; got != true {
		t.Fatalf("expected readOnlyGameData=true, got %+v", inspection.SemanticFields)
	}
	totals, ok := inspection.SemanticFields["totals"].(map[string]any)
	if !ok {
		t.Fatalf("expected totals map, got %+v", inspection.SemanticFields["totals"])
	}
	if got := totals["powerStars"]; got != float64(1) {
		t.Fatalf("expected one read-only power star aggregate, got %+v", totals)
	}
	if got := totals["cometMedals"]; got != float64(1) {
		t.Fatalf("expected one read-only comet medal aggregate, got %+v", totals)
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

func buildSMG2RawGameDataFixture() []byte {
	payload := make([]byte, wiiSMG2GameDataSize)
	binary.BigEndian.PutUint32(payload[4:8], wiiSMG2GameDataVersion)
	binary.BigEndian.PutUint32(payload[8:12], 3)
	binary.BigEndian.PutUint32(payload[12:16], uint32(len(payload)))
	writeSMG2Entry(payload, 0, "user1", 0x40)
	writeSMG2Entry(payload, 1, "config1", 0xFC0)
	writeSMG2Entry(payload, 2, "sysconf", 0x1020)
	payload[0x40] = 2
	payload[0x41] = 4
	copy(payload[0x44:0x48], []byte("PLAY"))
	binary.BigEndian.PutUint32(payload[0x48:0x4C], 0x20)
	binary.BigEndian.PutUint32(payload[0x4C:0x50], 44)
	binary.BigEndian.PutUint16(payload[0x50:0x52], 5)
	binary.BigEndian.PutUint16(payload[0x52:0x54], 8)
	writeSMG2Attr(payload, 0x54, 0, 0x4ED5, 0)
	writeSMG2Attr(payload, 0x54, 1, 0xE352, 1)
	writeSMG2Attr(payload, 0x54, 2, 0x450D, 3)
	writeSMG2Attr(payload, 0x54, 3, 0x23EC, 5)
	writeSMG2Attr(payload, 0x54, 4, 0x7579, 7)
	payload[0x68] = 4
	binary.BigEndian.PutUint16(payload[0x69:0x6B], 1234)
	binary.BigEndian.PutUint16(payload[0x6B:0x6D], 56)
	binary.BigEndian.PutUint16(payload[0x6D:0x6F], 50)
	payload[0x6F] = 1
	copy(payload[0x70:0x74], []byte("GALA"))
	binary.BigEndian.PutUint32(payload[0x74:0x78], 0x20)
	binary.BigEndian.PutUint32(payload[0x78:0x7C], 67)
	binary.BigEndian.PutUint16(payload[0x7C:0x7E], 1)
	binary.BigEndian.PutUint16(payload[0x7E:0x80], 5)
	binary.BigEndian.PutUint16(payload[0x80:0x82], 7)
	writeSMG2Attr(payload, 0x82, 0, 0x8208, 0)
	writeSMG2Attr(payload, 0x82, 1, 0x0658, 2)
	writeSMG2Attr(payload, 0x82, 2, 0x6729, 4)
	writeSMG2Attr(payload, 0x82, 3, 0xACB4, 5)
	writeSMG2Attr(payload, 0x82, 4, 0x7579, 6)
	binary.BigEndian.PutUint16(payload[0x96:0x98], 3)
	binary.BigEndian.PutUint16(payload[0x98:0x9A], 6)
	writeSMG2Attr(payload, 0x9A, 0, 0xCFBD, 0)
	writeSMG2Attr(payload, 0x9A, 1, 0xF25E, 1)
	writeSMG2Attr(payload, 0x9A, 2, 0x7579, 5)
	binary.BigEndian.PutUint16(payload[0xA6:0xA8], 0x1111)
	binary.BigEndian.PutUint16(payload[0xA8:0xAA], 13)
	payload[0xAA] = 1
	payload[0xAB] = 2
	payload[0xAC] = 1
	binary.BigEndian.PutUint32(payload[0xAE:0xB2], 123)
	payload[0xB2] = 0x03
	copy(payload[0xB3:0xB7], []byte("FLG1"))
	binary.BigEndian.PutUint32(payload[0xB7:0xBB], 0x65020442)
	binary.BigEndian.PutUint32(payload[0xBB:0xBF], 16)
	binary.BigEndian.PutUint16(payload[0xBF:0xC1], 0x8001)
	binary.BigEndian.PutUint16(payload[0xC1:0xC3], 0x0002)
	copy(payload[0xC3:0xC7], []byte("VLE1"))
	binary.BigEndian.PutUint32(payload[0xC7:0xCB], 0x564C4531)
	binary.BigEndian.PutUint32(payload[0xCB:0xCF], 16)
	binary.BigEndian.PutUint16(payload[0xCF:0xD1], 0x1234)
	binary.BigEndian.PutUint16(payload[0xD1:0xD3], 9)
	payload[0xFC0] = 2
	payload[0xFC1] = 3
	copy(payload[0xFC4:0xFC8], []byte("CONF"))
	binary.BigEndian.PutUint32(payload[0xFC8:0xFCC], 0x2432DA)
	binary.BigEndian.PutUint32(payload[0xFCC:0xFD0], 13)
	payload[0xFD0] = 0xFF
	copy(payload[0xFD1:0xFD5], []byte("MII "))
	binary.BigEndian.PutUint32(payload[0xFD5:0xFD9], 0x2836E9)
	binary.BigEndian.PutUint32(payload[0xFD9:0xFDD], 22)
	payload[0xFDD] = 0x02
	payload[0xFE6] = 1
	copy(payload[0xFE7:0xFEB], []byte("MISC"))
	binary.BigEndian.PutUint32(payload[0xFEB:0xFEF], 0x1)
	binary.BigEndian.PutUint32(payload[0xFEF:0xFF3], 20)
	payload[0x1020] = 2
	payload[0x1021] = 1
	copy(payload[0x1024:0x1028], []byte("SYSC"))
	binary.BigEndian.PutUint32(payload[0x1028:0x102C], 0x3)
	binary.BigEndian.PutUint32(payload[0x102C:0x1030], 64)
	binary.BigEndian.PutUint16(payload[0x1030:0x1032], 7)
	binary.BigEndian.PutUint16(payload[0x1032:0x1034], 20)
	writeSMG2Attr(payload, 0x1034, 0, 0x9B5F, 0)
	writeSMG2Attr(payload, 0x1034, 1, 0x0F92, 1)
	writeSMG2Attr(payload, 0x1034, 2, 0x49C6, 9)
	writeSMG2Attr(payload, 0x1034, 3, 0x3D13, 13)
	writeSMG2Attr(payload, 0x1034, 4, 0x36F1, 15)
	writeSMG2Attr(payload, 0x1034, 5, 0x3044, 17)
	writeSMG2Attr(payload, 0x1034, 6, 0xEA91, 18)
	binary.BigEndian.PutUint16(payload[0x105D:0x105F], 500)
	binary.BigEndian.PutUint16(payload[0x105F:0x1061], 999)
	payload[0x1061] = 3
	checksum := wiiGalaxyChecksum(payload[4:])
	binary.BigEndian.PutUint32(payload[0:4], checksum)
	return payload
}

func writeSMG2Attr(payload []byte, base int, index int, hash uint16, offset uint16) {
	start := base + index*4
	binary.BigEndian.PutUint16(payload[start:start+2], hash)
	binary.BigEndian.PutUint16(payload[start+2:start+4], offset)
}

func writeSMG2Entry(payload []byte, index int, name string, offset int) {
	start := 0x10 + index*0x10
	copy(payload[start:start+12], []byte(name))
	binary.BigEndian.PutUint32(payload[start+12:start+16], uint32(offset))
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
