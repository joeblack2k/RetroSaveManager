package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const (
	wiiDataBinParserID           = "wii-data-bin"
	wiiDataBinBackupHeaderOffset = 0xF0C0
	wiiDataBinFileHeaderOffset   = 0xF140
	wiiDataBinFileHeaderMagic    = 0x03ADF17E

	wiiTrustLevelMediaVerified     = "media-verified"
	wiiTrustLevelROMVerified       = "rom-verified"
	wiiTrustLevelStructureVerified = "structure-verified"
	wiiTrustLevelSemanticVerified  = "semantic-verified"
)

var wiiTitleCodePattern = regexp.MustCompile(`(?i)(?:^|/)([A-Z0-9]{4})(?:/data\.bin$|$)`)

type wiiTitleCatalogEntry struct {
	GameID     string
	Title      string
	RegionCode string
}

type wiiDataBinDetails struct {
	HeaderOffset       int
	FileHeaderOffset   int
	FileCount          int
	DeclaredDataSize   int
	FileName           string
	FileSize           int
	CertificatePresent bool
}

var wiiTitleCatalog = map[string]wiiTitleCatalogEntry{
	"SB4P": {
		GameID:     "wii/super-mario-galaxy-2",
		Title:      "Super Mario Galaxy 2",
		RegionCode: regionEU,
	},
}

func validateWiiSave(input saveCreateInput, detection saveSystemDetectionResult) consoleValidationResult {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(input.Filename))), ".")
	if ext != "bin" {
		return consoleValidationResult{Rejected: true, RejectReason: "wii saves require an exported data.bin payload"}
	}
	if len(input.Payload) == 0 {
		return consoleValidationResult{Rejected: true, RejectReason: "wii data.bin payload is empty"}
	}
	if looksLikeExecutableOrArchivePayload(input.Payload) {
		return consoleValidationResult{Rejected: true, RejectReason: "payload looks like executable/archive"}
	}
	if looksLikeMostlyTextPayload(input.Payload) {
		return consoleValidationResult{Rejected: true, RejectReason: "payload looks like text/noise"}
	}
	if allBytesEqual(input.Payload, 0x00) {
		return consoleValidationResult{Rejected: true, RejectReason: "wii data.bin payload is blank (all 0x00)"}
	}
	if allBytesEqual(input.Payload, 0xFF) {
		return consoleValidationResult{Rejected: true, RejectReason: "wii data.bin payload is blank (all 0xFF)"}
	}

	details, err := parseWiiDataBin(input.Payload)
	if err != nil {
		return consoleValidationResult{Rejected: true, RejectReason: err.Error()}
	}

	titleCode := wiiTitleCodeFromSaveInput(input)
	catalog := wiiCatalogEntryForTitleCode(titleCode)
	regionCode := catalog.RegionCode
	if regionCode == "" {
		regionCode = regionFromWiiTitleCode(titleCode)
	}

	verificationLevels := []string{wiiTrustLevelMediaVerified, wiiTrustLevelStructureVerified}
	trustLevel := wiiTrustLevelStructureVerified
	if strings.TrimSpace(input.ROMSHA1) != "" {
		verificationLevels = appendUniqueString(verificationLevels, wiiTrustLevelROMVerified)
		trustLevel = wiiTrustLevelROMVerified
	}
	semanticVerified := false

	evidence := []string{
		"validated Wii SD data.bin media",
		"validated Wii backup header",
		fmt.Sprintf("backupHeaderOffset=0x%X", details.HeaderOffset),
		fmt.Sprintf("fileCount=%d", details.FileCount),
		fmt.Sprintf("payloadSize=%d", len(input.Payload)),
	}
	if titleCode != "" {
		evidence = append(evidence, "titleCode="+titleCode)
	}
	if details.FileName != "" {
		evidence = append(evidence, "embeddedFile="+details.FileName)
	}
	if details.CertificatePresent {
		evidence = append(evidence, "certificateChainPresent")
	}
	if detection.Evidence.Declared {
		evidence = append(evidence, "declared system evidence")
	}
	if detection.Evidence.HelperTrusted {
		evidence = append(evidence, "trusted helper system")
	}
	if detection.Evidence.StoredTrusted {
		evidence = append(evidence, "trusted stored system")
	}
	if strings.TrimSpace(input.ROMSHA1) != "" {
		evidence = append(evidence, "romSha1 present")
	}

	warnings := make([]string, 0, 2)
	if titleCode == "" {
		warnings = append(warnings, "Wii title code was not supplied by path, zip structure, or form metadata; game title enrichment is unavailable")
	}
	warnings = append(warnings, "Official Wii data.bin exports are encrypted; semantic gameplay fields and cheat editing require a verified decrypted-game decoder")

	semanticFields := map[string]any{
		"containerKind":        "wii-data-bin",
		"extension":            ext,
		"titleCode":            titleCode,
		"region":               regionCode,
		"embeddedFileName":     details.FileName,
		"embeddedFileSize":     details.FileSize,
		"fileCount":            details.FileCount,
		"declaredDataSize":     details.DeclaredDataSize,
		"backupHeaderOffset":   fmt.Sprintf("0x%X", details.HeaderOffset),
		"fileHeaderOffset":     fmt.Sprintf("0x%X", details.FileHeaderOffset),
		"certificatePresent":   details.CertificatePresent,
		"encrypted":            true,
		"mediaVerified":        true,
		"romVerified":          strings.TrimSpace(input.ROMSHA1) != "",
		"structureVerified":    true,
		"semanticVerified":     semanticVerified,
		"verificationLevels":   verificationLevels,
		"cheatEditing":         "not available until a semantic Super Mario Galaxy 2 decoder is verified",
		"romSha1Present":       strings.TrimSpace(input.ROMSHA1) != "",
		"semanticDecoderState": "encrypted-container-only",
	}
	if sourcePath := wiiSourcePathFromMetadata(input.Metadata); sourcePath != "" {
		semanticFields["sourcePath"] = sourcePath
	}

	inspection := &saveInspection{
		ParserLevel:        saveParserLevelStructural,
		ParserID:           wiiDataBinParserID,
		ValidatedSystem:    "wii",
		ValidatedGameID:    catalog.GameID,
		ValidatedGameTitle: catalog.Title,
		TrustLevel:         trustLevel,
		Evidence:           evidence,
		Warnings:           warnings,
		PayloadSizeBytes:   len(input.Payload),
		SlotCount:          details.FileCount,
		ActiveSlotIndexes:  sequentialIndexes(details.FileCount),
		ChecksumValid:      nil,
		SemanticFields:     semanticFields,
	}
	return consoleValidationResult{Inspection: inspection}
}

func parseWiiDataBin(payload []byte) (wiiDataBinDetails, error) {
	if len(payload) < wiiDataBinFileHeaderOffset+0x80 {
		return wiiDataBinDetails{}, fmt.Errorf("wii data.bin payload is too small")
	}
	if !bytes.Equal(payload[wiiDataBinBackupHeaderOffset+4:wiiDataBinBackupHeaderOffset+8], []byte{'B', 'k', 0x00, 0x01}) {
		return wiiDataBinDetails{}, fmt.Errorf("wii data.bin backup header magic is missing")
	}
	headerSize := binary.BigEndian.Uint32(payload[wiiDataBinBackupHeaderOffset : wiiDataBinBackupHeaderOffset+4])
	if headerSize != 0x70 {
		return wiiDataBinDetails{}, fmt.Errorf("wii data.bin backup header size is invalid")
	}
	fileCount := int(binary.BigEndian.Uint32(payload[wiiDataBinBackupHeaderOffset+0x0C : wiiDataBinBackupHeaderOffset+0x10]))
	if fileCount <= 0 || fileCount > 64 {
		return wiiDataBinDetails{}, fmt.Errorf("wii data.bin file count is invalid")
	}
	declaredDataSize := int(binary.BigEndian.Uint32(payload[wiiDataBinBackupHeaderOffset+0x10 : wiiDataBinBackupHeaderOffset+0x14]))
	if declaredDataSize <= 0 || declaredDataSize > len(payload) {
		return wiiDataBinDetails{}, fmt.Errorf("wii data.bin declared data size is invalid")
	}
	fileHeaderMagic := binary.BigEndian.Uint32(payload[wiiDataBinFileHeaderOffset : wiiDataBinFileHeaderOffset+4])
	if fileHeaderMagic != wiiDataBinFileHeaderMagic {
		return wiiDataBinDetails{}, fmt.Errorf("wii data.bin file header magic is missing")
	}
	fileSize := int(binary.BigEndian.Uint32(payload[wiiDataBinFileHeaderOffset+4 : wiiDataBinFileHeaderOffset+8]))
	if fileSize <= 0 || fileSize > len(payload) {
		return wiiDataBinDetails{}, fmt.Errorf("wii data.bin embedded file size is invalid")
	}
	return wiiDataBinDetails{
		HeaderOffset:       wiiDataBinBackupHeaderOffset,
		FileHeaderOffset:   wiiDataBinFileHeaderOffset,
		FileCount:          fileCount,
		DeclaredDataSize:   declaredDataSize,
		FileName:           extractWiiEmbeddedFileName(payload[wiiDataBinFileHeaderOffset:minInt(len(payload), wiiDataBinFileHeaderOffset+0x80)]),
		FileSize:           fileSize,
		CertificatePresent: bytes.Contains(payload, []byte("Root-CA")) || bytes.Contains(payload, []byte("AP000")),
	}, nil
}

func isLikelyWiiDataBin(payload []byte) bool {
	_, err := parseWiiDataBin(payload)
	return err == nil
}

func extractWiiEmbeddedFileName(header []byte) string {
	best := ""
	start := -1
	flush := func(end int) {
		if start < 0 || end <= start {
			return
		}
		candidate := strings.TrimSpace(string(header[start:end]))
		start = -1
		if len(candidate) < 3 {
			return
		}
		if !strings.Contains(candidate, ".") && !strings.Contains(candidate, "/") {
			return
		}
		if best == "" || len(candidate) > len(best) {
			best = candidate
		}
	}
	for idx, value := range header {
		if value >= 0x20 && value <= 0x7E {
			if start < 0 {
				start = idx
			}
			continue
		}
		flush(idx)
	}
	flush(len(header))
	return best
}

func wiiCatalogEntryForTitleCode(titleCode string) wiiTitleCatalogEntry {
	code := normalizeWiiTitleCode(titleCode)
	if code == "" {
		return wiiTitleCatalogEntry{}
	}
	if entry, ok := wiiTitleCatalog[code]; ok {
		return entry
	}
	return wiiTitleCatalogEntry{
		GameID:     "wii/" + strings.ToLower(code),
		Title:      "Wii Save " + code,
		RegionCode: regionFromWiiTitleCode(code),
	}
}

func wiiGameFromTitleCode(titleCode string) game {
	entry := wiiCatalogEntryForTitleCode(titleCode)
	if strings.TrimSpace(entry.Title) == "" {
		return fallbackGameFromFilename("data.bin")
	}
	return game{
		ID:            deterministicGameID(entry.GameID),
		Name:          entry.Title,
		DisplayTitle:  entry.Title,
		RegionCode:    normalizeRegionCode(entry.RegionCode),
		RegionFlag:    regionFlagFromCode(entry.RegionCode),
		LanguageCodes: nil,
		Boxart:        nil,
		BoxartThumb:   nil,
		HasParser:     true,
		System:        supportedSystemFromSlug("wii"),
	}
}

func normalizeWiiTitleCode(raw string) string {
	clean := strings.ToUpper(strings.TrimSpace(raw))
	if len(clean) != 4 {
		return ""
	}
	for _, r := range clean {
		if !unicode.IsDigit(r) && (r < 'A' || r > 'Z') {
			return ""
		}
	}
	return clean
}

func regionFromWiiTitleCode(titleCode string) string {
	code := normalizeWiiTitleCode(titleCode)
	if len(code) != 4 {
		return regionUnknown
	}
	switch code[3] {
	case 'E':
		return regionUS
	case 'P', 'D', 'F', 'H', 'I', 'L', 'M', 'N', 'S', 'U', 'X', 'Y':
		return regionEU
	case 'J':
		return regionJP
	default:
		return regionUnknown
	}
}

func wiiTitleCodeFromSaveInput(input saveCreateInput) string {
	if code := wiiTitleCodeFromMetadata(input.Metadata); code != "" {
		return code
	}
	for _, candidate := range []string{input.Filename, input.SlotName, input.DisplayTitle, input.Game.DisplayTitle, input.Game.Name} {
		if code := normalizeWiiTitleCode(candidate); code != "" {
			return code
		}
	}
	return ""
}

func wiiTitleCodeFromMetadata(metadata any) string {
	wii := wiiMetadataMap(metadata)
	if wii == nil {
		return ""
	}
	for _, key := range []string{"titleCode", "gameCode", "wiiTitleId", "wiiTitleID"} {
		if value, ok := wii[key].(string); ok {
			if code := normalizeWiiTitleCode(value); code != "" {
				return code
			}
		}
	}
	return ""
}

func wiiSourcePathFromMetadata(metadata any) string {
	wii := wiiMetadataMap(metadata)
	if wii == nil {
		return ""
	}
	if value, ok := wii["sourcePath"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func wiiMetadataMap(metadata any) map[string]any {
	root, ok := metadata.(map[string]any)
	if !ok {
		return nil
	}
	rsm, ok := root["rsm"].(map[string]any)
	if !ok {
		return nil
	}
	wii, ok := rsm["wii"].(map[string]any)
	if !ok {
		return nil
	}
	return wii
}

func wiiTitleCodeFromPath(path string) string {
	clean := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"))
	if clean == "" {
		return ""
	}
	if matches := wiiTitleCodePattern.FindStringSubmatch(clean); len(matches) == 2 {
		return normalizeWiiTitleCode(matches[1])
	}
	parts := strings.Split(clean, "/")
	for idx, part := range parts {
		if normalizeWiiTitleCode(part) == "" {
			continue
		}
		if idx+1 < len(parts) && strings.EqualFold(parts[idx+1], "data.bin") {
			return normalizeWiiTitleCode(part)
		}
	}
	return ""
}

func looksLikeWiiDataBinPath(path string) bool {
	clean := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"))
	return strings.HasSuffix(clean, "/data.bin") || clean == "data.bin"
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func sequentialIndexes(count int) []int {
	if count <= 0 {
		return nil
	}
	out := make([]int, 0, count)
	for i := 1; i <= count; i++ {
		out = append(out, i)
	}
	return out
}
