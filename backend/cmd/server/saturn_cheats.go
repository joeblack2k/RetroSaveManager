package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const saturnEntryPayloadFormat = "saturn-entry"

type saturnEntryCheatTarget struct {
	Parsed  *saturnParsedContainer
	Entry   saturnParsedEntry
	Summary saveSummary
	Payload []byte
}

func (a *app) saturnEntryCheatTarget(record saveRecord, selectedEntry string) (saturnEntryCheatTarget, error) {
	payload, err := os.ReadFile(record.payloadPath)
	if err != nil {
		return saturnEntryCheatTarget{}, err
	}
	parsed := parseSaturnContainer(record.Summary.Filename, payload)
	if parsed == nil {
		return saturnEntryCheatTarget{}, fmt.Errorf("save is not a valid Saturn backup RAM image")
	}
	entry, err := selectSaturnExportEntry(parsed, selectedEntry)
	if err != nil {
		return saturnEntryCheatTarget{}, err
	}
	summary := buildSaturnEntryCheatSummary(record, parsed, entry)
	baseInspection := summary.Inspection
	if modules := a.moduleService(); modules != nil {
		if inspection, ok := modules.inspectSave(saveCreateInput{
			Filename:     summary.Filename,
			Payload:      entry.Data,
			Format:       summary.Format,
			SystemSlug:   summary.SystemSlug,
			DisplayTitle: firstNonEmpty(summary.DisplayTitle, summary.Game.DisplayTitle, summary.Game.Name),
			Game:         summary.Game,
			SlotName:     summary.CardSlot,
			Metadata:     summary.Metadata,
		}, baseInspection); ok {
			summary.Inspection = inspection
		}
	}
	if cheats := a.cheatService(); cheats != nil {
		summary.Cheats = cheats.capabilityForPayload(record, summary, entry.Data, true)
	}
	return saturnEntryCheatTarget{
		Parsed:  parsed,
		Entry:   entry,
		Summary: summary,
		Payload: append([]byte(nil), entry.Data...),
	}, nil
}

func buildSaturnEntryCheatSummary(record saveRecord, parsed *saturnParsedContainer, entry saturnParsedEntry) saveSummary {
	summary := canonicalSummaryForRecord(record)
	entryTitle := firstNonEmpty(strings.TrimSpace(entry.Summary.Comment), strings.TrimSpace(entry.Summary.Filename), summary.DisplayTitle, summary.Game.DisplayTitle, summary.Game.Name)
	summary.DisplayTitle = entryTitle
	summary.Filename = safeFilename(firstNonEmpty(entry.Summary.Filename, entryTitle, "saturn-entry") + ".saturn-entry")
	summary.Format = saturnEntryPayloadFormat
	summary.FileSize = len(entry.Data)
	summary.LatestSizeBytes = len(entry.Data)
	summary.TotalSizeBytes = len(entry.Data) * maxInt(summary.SaveCount, 1)
	summary.Saturn = &saturnDetails{
		Container:       "backup-ram-entry",
		Format:          saturnEntryPayloadFormat,
		TotalImageBytes: len(entry.Data),
		SaveEntries:     1,
		DefaultVolume:   entry.Summary.Volume,
		SampleFilename:  entry.Summary.Filename,
		SampleComment:   entry.Summary.Comment,
		Entries:         []saturnEntry{entry.Summary},
	}
	summary.Metadata = mergeSaturnEntryMetadata(summary.Metadata, parsed, entry)
	summary.Inspection = saturnEntryBaseInspection(parsed, entry)
	return summary
}

func saturnEntryBaseInspection(parsed *saturnParsedContainer, entry saturnParsedEntry) *saveInspection {
	format := ""
	if parsed != nil && parsed.Details != nil {
		format = parsed.Details.Format
	}
	return &saveInspection{
		ParserLevel:        saveParserLevelStructural,
		ParserID:           "saturn-backup-entry",
		ValidatedSystem:    "saturn",
		ValidatedGameID:    firstNonEmpty(entry.Summary.Filename, entry.Summary.Comment),
		ValidatedGameTitle: firstNonEmpty(entry.Summary.Comment, entry.Summary.Filename),
		TrustLevel:         "structure-verified",
		Evidence: []string{
			"saturn backup RAM entry extracted from container",
			"containerFormat=" + format,
			"entry=" + strings.TrimSpace(entry.Summary.Filename),
		},
		PayloadSizeBytes: len(entry.Data),
		SlotCount:        1,
		ActiveSlotIndexes: []int{
			entry.Summary.FirstBlock,
		},
		SemanticFields: map[string]any{
			"saturnEntryFilename": entry.Summary.Filename,
			"saturnEntryComment":  entry.Summary.Comment,
			"saturnEntryVolume":   entry.Summary.Volume,
			"saturnEntryBlocks":   entry.Summary.BlockCount,
			"saturnEntryDate":     entry.Summary.Date,
			"containerFormat":     format,
		},
	}
}

func mergeSaturnEntryMetadata(existing any, parsed *saturnParsedContainer, entry saturnParsedEntry) any {
	containerFormat := ""
	if parsed != nil && parsed.Details != nil {
		containerFormat = parsed.Details.Format
	}
	entryAudit := map[string]any{
		"filename":        strings.TrimSpace(entry.Summary.Filename),
		"comment":         strings.TrimSpace(entry.Summary.Comment),
		"volume":          strings.TrimSpace(entry.Summary.Volume),
		"firstBlock":      entry.Summary.FirstBlock,
		"blockCount":      entry.Summary.BlockCount,
		"containerFormat": containerFormat,
	}
	if existing == nil {
		return map[string]any{"saturnEntry": entryAudit}
	}
	if existingMap, ok := existing.(map[string]any); ok {
		merged := make(map[string]any, len(existingMap)+1)
		for key, value := range existingMap {
			merged[key] = value
		}
		merged["saturnEntry"] = entryAudit
		return merged
	}
	return map[string]any{
		"saturnEntry":    entryAudit,
		"sourceMetadata": existing,
	}
}

func (a *app) enrichSaturnSummary(record saveRecord, summary saveSummary) saveSummary {
	if canonicalSegment(summary.SystemSlug, "") != "saturn" {
		return summary
	}
	target, err := a.saturnEntryCheatTarget(record, "")
	if err != nil {
		return summary
	}
	if target.Summary.Cheats == nil {
		return summary
	}
	summary.Cheats = target.Summary.Cheats
	summary.Inspection = target.Summary.Inspection
	return summary
}

func (a *app) promoteSaturnEntryCheatPayload(sourceRecord saveRecord, selectedEntry string, patchedEntry []byte, metadata any) (saveRecord, error) {
	sourcePayload, err := os.ReadFile(sourceRecord.payloadPath)
	if err != nil {
		return saveRecord{}, err
	}
	fullPayload, err := replaceSaturnEntryPayload(sourceRecord.Summary.Filename, sourcePayload, selectedEntry, patchedEntry)
	if err != nil {
		return saveRecord{}, err
	}
	newRecord, err := a.createSave(saveCreateInput{
		Filename:              sourceRecord.Summary.Filename,
		Payload:               fullPayload,
		Game:                  sourceRecord.Summary.Game,
		Format:                sourceRecord.Summary.Format,
		Metadata:              metadata,
		ROMSHA1:               sourceRecord.ROMSHA1,
		ROMMD5:                sourceRecord.ROMMD5,
		SlotName:              sourceRecord.SlotName,
		SystemSlug:            sourceRecord.SystemSlug,
		GameSlug:              sourceRecord.GameSlug,
		SystemPath:            sourceRecord.SystemPath,
		GamePath:              sourceRecord.GamePath,
		TrustedHelperSystem:   metadataHasTrustedSystemEvidence(sourceRecord.Summary.Metadata),
		DisplayTitle:          sourceRecord.Summary.DisplayTitle,
		RegionCode:            sourceRecord.Summary.RegionCode,
		RegionFlag:            sourceRecord.Summary.RegionFlag,
		LanguageCodes:         sourceRecord.Summary.LanguageCodes,
		CoverArtURL:           sourceRecord.Summary.CoverArtURL,
		Saturn:                sourceRecord.Summary.Saturn,
		Inspection:            sourceRecord.Summary.Inspection,
		MediaType:             sourceRecord.Summary.MediaType,
		ProjectionCapable:     sourceRecord.Summary.ProjectionCapable,
		SourceArtifactProfile: sourceRecord.Summary.SourceArtifactProfile,
		RuntimeProfile:        sourceRecord.Summary.RuntimeProfile,
		CardSlot:              sourceRecord.Summary.CardSlot,
		ProjectionID:          sourceRecord.Summary.ProjectionID,
		SourceImportID:        sourceRecord.Summary.SourceImportID,
		Portable:              sourceRecord.Summary.Portable,
		CreatedAt:             time.Now().UTC(),
	})
	if err != nil {
		return saveRecord{}, err
	}
	return newRecord, nil
}

func replaceSaturnEntryPayload(filename string, payload []byte, selectedEntry string, patchedEntry []byte) ([]byte, error) {
	parsed := parseSaturnContainer(filename, payload)
	if parsed == nil {
		return nil, fmt.Errorf("save is not a valid Saturn backup RAM image")
	}
	entry, err := selectSaturnExportEntry(parsed, selectedEntry)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(strings.TrimSpace(entry.Summary.Volume)) {
	case "internal":
		if parsed.Internal == nil {
			return nil, fmt.Errorf("Saturn save entry %q is missing its internal volume", entry.Summary.Filename)
		}
		raw, err := replaceSaturnEntryInVolume(parsed.Internal.Raw, parsed.Internal.Summary.BlockSize, entry, patchedEntry)
		if err != nil {
			return nil, err
		}
		parsed.Internal.Raw = raw
	case "cartridge":
		if parsed.Cartridge == nil {
			return nil, fmt.Errorf("Saturn save entry %q is missing its cartridge volume", entry.Summary.Filename)
		}
		raw, err := replaceSaturnEntryInVolume(parsed.Cartridge.Raw, parsed.Cartridge.Summary.BlockSize, entry, patchedEntry)
		if err != nil {
			return nil, err
		}
		parsed.Cartridge.Raw = raw
	default:
		return nil, fmt.Errorf("unsupported Saturn entry volume %q", entry.Summary.Volume)
	}
	return rebuildSaturnOriginalPayload(parsed)
}

func replaceSaturnEntryInVolume(raw []byte, blockSize int, entry saturnParsedEntry, patchedEntry []byte) ([]byte, error) {
	if blockSize <= 0 {
		return nil, fmt.Errorf("Saturn entry has invalid block size")
	}
	blocks := append([]int(nil), entry.Summary.BlockIndexes...)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("Saturn entry %q has no block chain", entry.Summary.Filename)
	}
	capacity, ok := saturnEntryDataCapacity(blockSize, blocks)
	if !ok {
		return nil, fmt.Errorf("Saturn entry %q has invalid block chain capacity", entry.Summary.Filename)
	}
	if len(patchedEntry) > capacity {
		return nil, fmt.Errorf("patched Saturn entry is %d bytes but existing block chain only fits %d bytes", len(patchedEntry), capacity)
	}
	out := append([]byte(nil), raw...)
	firstOffset := blocks[0] * blockSize
	if firstOffset+0x22 > len(out) {
		return nil, fmt.Errorf("Saturn entry %q first block is out of range", entry.Summary.Filename)
	}
	binaryBigEndianPutUint32(out[firstOffset+0x1E:firstOffset+0x22], uint32(len(patchedEntry)))
	if !saturnWriteEntryData(out, blockSize, blocks, patchedEntry) {
		return nil, fmt.Errorf("could not write patched Saturn entry %q", entry.Summary.Filename)
	}
	return out, nil
}

func saturnEntryDataCapacity(blockSize int, blocks []int) (int, bool) {
	blockListRemaining := len(blocks) * 2
	capacity := 0
	for blockIndex := range blocks {
		innerOffset := 0x04
		if blockIndex == 0 {
			innerOffset = 0x22
		}
		avail := blockSize - innerOffset
		if avail < 0 {
			return 0, false
		}
		if blockListRemaining >= avail {
			blockListRemaining -= avail
			continue
		}
		if blockListRemaining > 0 {
			avail -= blockListRemaining
			blockListRemaining = 0
		}
		capacity += avail
	}
	return capacity, true
}

func saturnWriteEntryData(raw []byte, blockSize int, blocks []int, data []byte) bool {
	blockListRemaining := len(blocks) * 2
	remaining := data
	for blockIndex, block := range blocks {
		blockOffset := block * blockSize
		innerOffset := 0x04
		if blockIndex == 0 {
			innerOffset = 0x22
		}
		avail := blockSize - innerOffset
		if blockOffset < 0 || blockOffset+innerOffset > len(raw) || avail < 0 {
			return false
		}
		if blockListRemaining >= avail {
			blockListRemaining -= avail
			continue
		}
		if blockListRemaining > 0 {
			innerOffset += blockListRemaining
			avail -= blockListRemaining
			blockListRemaining = 0
		}
		if blockOffset+innerOffset+avail > len(raw) {
			return false
		}
		window := raw[blockOffset+innerOffset : blockOffset+innerOffset+avail]
		for i := range window {
			window[i] = 0
		}
		n := min(len(remaining), len(window))
		copy(window, remaining[:n])
		remaining = remaining[n:]
	}
	return len(remaining) == 0
}

func rebuildSaturnOriginalPayload(parsed *saturnParsedContainer) ([]byte, error) {
	if parsed == nil {
		return nil, fmt.Errorf("Saturn container is missing")
	}
	switch strings.ToLower(strings.TrimSpace(parsed.Format)) {
	case "internal-raw":
		return append([]byte(nil), parsed.Internal.Raw...), nil
	case "cartridge-raw":
		return append([]byte(nil), parsed.Cartridge.Raw...), nil
	case "mister-internal-interleaved":
		return expandByteExpanded(parsed.Internal.Raw), nil
	case "cartridge-interleaved":
		return expandByteExpanded(parsed.Cartridge.Raw), nil
	case "combined-raw":
		out := append([]byte(nil), parsed.Internal.Raw...)
		out = append(out, saturnCartridgeRawForRebuild(parsed)...)
		return out, nil
	case "mister-combined-interleaved":
		out := expandByteExpanded(parsed.Internal.Raw)
		out = append(out, saturnCartridgeInterleavedForRebuild(parsed)...)
		return out, nil
	case "yabasanshiro-raw":
		return buildSaturnExtendedInternalRaw(parsed.Internal.Raw, saturnYabaSanshiroRawSize), nil
	case "yabasanshiro-interleaved":
		return expandByteExpanded(buildSaturnExtendedInternalRaw(parsed.Internal.Raw, saturnYabaSanshiroRawSize)), nil
	case "mednafen-cartridge-gzip":
		if parsed.Cartridge == nil {
			return nil, fmt.Errorf("Saturn gzip rebuild requires cartridge backup RAM")
		}
		return gzipBytes(parsed.Cartridge.Raw)
	default:
		return nil, fmt.Errorf("unsupported Saturn container rebuild format %q", parsed.Format)
	}
}

func saturnCartridgeRawForRebuild(parsed *saturnParsedContainer) []byte {
	if parsed != nil && parsed.Cartridge != nil && len(parsed.Cartridge.Raw) > 0 {
		return append([]byte(nil), parsed.Cartridge.Raw...)
	}
	if parsed != nil && len(parsed.OriginalPayload) >= saturnCombinedRawSize {
		return append([]byte(nil), parsed.OriginalPayload[saturnInternalRawSize:saturnCombinedRawSize]...)
	}
	return make([]byte, saturnCartridgeRawSize)
}

func saturnCartridgeInterleavedForRebuild(parsed *saturnParsedContainer) []byte {
	if parsed != nil && parsed.Cartridge != nil && len(parsed.Cartridge.Raw) > 0 {
		return expandByteExpanded(parsed.Cartridge.Raw)
	}
	if parsed != nil && len(parsed.OriginalPayload) >= saturnCombinedInterleavedSize {
		return append([]byte(nil), parsed.OriginalPayload[saturnInternalInterleavedSize:saturnCombinedInterleavedSize]...)
	}
	return make([]byte, saturnCartridgeInterleavedSize)
}

func saturnEntryNameFromSummary(summary saveSummary) string {
	metadata, ok := summary.Metadata.(map[string]any)
	if !ok {
		return ""
	}
	entry, ok := metadata["saturnEntry"].(map[string]any)
	if !ok {
		return ""
	}
	value, _ := entry["filename"].(string)
	return strings.TrimSpace(value)
}

func binaryBigEndianPutUint32(out []byte, value uint32) {
	out[0] = byte(value >> 24)
	out[1] = byte(value >> 16)
	out[2] = byte(value >> 8)
	out[3] = byte(value)
}
