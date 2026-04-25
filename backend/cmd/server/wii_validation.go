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
	wiiSMG2GameDataParserID      = "smg2-gamedata-bin"
	wiiDataBinBackupHeaderOffset = 0xF0C0
	wiiDataBinFileHeaderOffset   = 0xF140
	wiiDataBinFileHeaderMagic    = 0x03ADF17E
	wiiSMG2GameDataVersion       = 3
	wiiSMG2GameDataSize          = 0x30A0

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

type wiiSMG2GameDataDetails struct {
	EntryCount      int
	UserFileCount   int
	ConfigFileCount int
	SysconfPresent  bool
	Checksum        uint32
}

type wiiSMG2RawEntry struct {
	Name   string
	Kind   string
	Slot   int
	Offset int
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
		if rawDetails, rawErr := parseSMG2RawGameData(input.Payload); rawErr == nil {
			return validateSMG2RawGameData(input, detection, ext, rawDetails)
		}
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

func validateSMG2RawGameData(input saveCreateInput, detection saveSystemDetectionResult, ext string, details wiiSMG2GameDataDetails) consoleValidationResult {
	evidence := []string{
		"validated raw decrypted Super Mario Galaxy 2 GameData.bin",
		fmt.Sprintf("payloadSize=%d", len(input.Payload)),
		fmt.Sprintf("entryCount=%d", details.EntryCount),
		fmt.Sprintf("userFiles=%d", details.UserFileCount),
		"checksum valid",
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

	checksumValid := true
	inspection := &saveInspection{
		ParserLevel:        saveParserLevelStructural,
		ParserID:           wiiSMG2GameDataParserID,
		ValidatedSystem:    "wii",
		ValidatedGameID:    "wii/super-mario-galaxy-2",
		ValidatedGameTitle: "Super Mario Galaxy 2",
		TrustLevel:         wiiTrustLevelStructureVerified,
		Evidence:           evidence,
		Warnings: []string{
			"Raw decrypted GameData.bin is accepted for read-only parser verification; write support is not exposed for decrypted gameplay fields",
		},
		PayloadSizeBytes:  len(input.Payload),
		SlotCount:         details.UserFileCount,
		ActiveSlotIndexes: sequentialIndexes(details.UserFileCount),
		ChecksumValid:     &checksumValid,
		SemanticFields: map[string]any{
			"containerKind":      "smg2-raw-gamedata",
			"extension":          ext,
			"version":            wiiSMG2GameDataVersion,
			"entryCount":         details.EntryCount,
			"userFileCount":      details.UserFileCount,
			"configFileCount":    details.ConfigFileCount,
			"sysconfPresent":     details.SysconfPresent,
			"checksum":           fmt.Sprintf("0x%08X", details.Checksum),
			"encrypted":          false,
			"semanticVerified":   false,
			"cheatEditing":       "not available for raw GameData.bin; module parser exposes read-only semantic fields",
			"readOnlyGameData":   true,
			"semanticParserHint": "smg2-data-bin-wasm",
		},
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

func parseSMG2RawGameData(payload []byte) (wiiSMG2GameDataDetails, error) {
	if len(payload) != wiiSMG2GameDataSize {
		return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin payload size is invalid")
	}
	if binary.BigEndian.Uint32(payload[4:8]) != wiiSMG2GameDataVersion {
		return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin version is invalid")
	}
	entryCount := int(binary.BigEndian.Uint32(payload[8:12]))
	if entryCount <= 0 || entryCount > 14 {
		return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin entry count is invalid")
	}
	fileSize := int(binary.BigEndian.Uint32(payload[12:16]))
	if fileSize != len(payload) {
		return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin file size is invalid")
	}
	storedChecksum := binary.BigEndian.Uint32(payload[0:4])
	if computed := wiiGalaxyChecksum(payload[4:]); computed != storedChecksum {
		return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin checksum is invalid")
	}

	indexEnd := 0x10 + entryCount*0x10
	if indexEnd >= len(payload) {
		return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin index is invalid")
	}
	entries := make([]wiiSMG2RawEntry, 0, entryCount)
	seenUserSlots := map[int]struct{}{}
	seenConfigSlots := map[int]struct{}{}
	lastOffset := indexEnd
	userCount := 0
	configCount := 0
	sysconfPresent := false
	for idx := 0; idx < entryCount; idx++ {
		entry := payload[0x10+idx*0x10 : 0x20+idx*0x10]
		name := fixedASCIIName(entry[:12])
		if name == "" {
			return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin entry name is invalid")
		}
		offset := int(binary.BigEndian.Uint32(entry[12:16]))
		if offset < indexEnd || offset >= len(payload) || offset < lastOffset {
			return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin entry offset is invalid")
		}
		lastOffset = offset
		if slot, ok := smg2EntrySlot(name, "user"); ok {
			if _, exists := seenUserSlots[slot]; exists {
				return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin duplicate user slot")
			}
			seenUserSlots[slot] = struct{}{}
			userCount++
			entries = append(entries, wiiSMG2RawEntry{Name: name, Kind: "user", Slot: slot, Offset: offset})
			continue
		}
		if slot, ok := smg2EntrySlot(name, "config"); ok {
			if _, exists := seenConfigSlots[slot]; exists {
				return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin duplicate config slot")
			}
			seenConfigSlots[slot] = struct{}{}
			configCount++
			entries = append(entries, wiiSMG2RawEntry{Name: name, Kind: "config", Slot: slot, Offset: offset})
			continue
		}
		if name != "sysconf" {
			return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin entry name is unsupported")
		}
		if sysconfPresent {
			return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin duplicate sysconf entry")
		}
		sysconfPresent = true
		entries = append(entries, wiiSMG2RawEntry{Name: name, Kind: "sysconf", Offset: offset})
	}
	for idx, entry := range entries {
		end := len(payload)
		if idx+1 < len(entries) {
			end = entries[idx+1].Offset
		}
		minSize := 0
		switch entry.Kind {
		case "user":
			minSize = 0xF80
		case "config":
			minSize = 0x60
		case "sysconf":
			minSize = 0x80
		default:
			return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin entry name is unsupported")
		}
		if entry.Offset+minSize > end {
			return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin %s entry is truncated", entry.Name)
		}
		if payload[entry.Offset] != 2 || binary.BigEndian.Uint16(payload[entry.Offset+2:entry.Offset+4]) != 0 {
			return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin %s entry header is invalid", entry.Name)
		}
	}
	if userCount == 0 {
		return wiiSMG2GameDataDetails{}, fmt.Errorf("wii raw GameData.bin has no user save entries")
	}
	return wiiSMG2GameDataDetails{
		EntryCount:      entryCount,
		UserFileCount:   userCount,
		ConfigFileCount: configCount,
		SysconfPresent:  sysconfPresent,
		Checksum:        storedChecksum,
	}, nil
}

func smg2EntrySlot(name string, prefix string) (int, bool) {
	if len(name) != len(prefix)+1 || !strings.HasPrefix(name, prefix) {
		return 0, false
	}
	slot := int(name[len(prefix)] - '0')
	if slot < 1 || slot > 6 {
		return 0, false
	}
	return slot, true
}

func wiiGalaxyChecksum(payload []byte) uint32 {
	var sum uint16
	var invSum uint16
	for idx := 0; idx+1 < len(payload); idx += 2 {
		term := binary.BigEndian.Uint16(payload[idx : idx+2])
		sum += term
		invSum += ^term
	}
	return uint32(sum)<<16 | uint32(invSum)
}

func fixedASCIIName(raw []byte) string {
	end := 0
	for end < len(raw) && raw[end] != 0 {
		if raw[end] < 0x20 || raw[end] > 0x7E {
			return ""
		}
		end++
	}
	return strings.TrimSpace(string(raw[:end]))
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
