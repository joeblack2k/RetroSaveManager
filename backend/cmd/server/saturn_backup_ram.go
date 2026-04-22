package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

const (
	saturnHeaderMagic               = "BackUpRam Format"
	saturnMinimumMagicBytes         = 0x40
	saturnInternalRawSize           = 0x8000
	saturnInternalInterleavedSize   = saturnInternalRawSize * 2
	saturnCartridgeRawSize          = 0x80000
	saturnCartridgeInterleavedSize  = saturnCartridgeRawSize * 2
	saturnYabaSanshiroRawSize       = 0x400000
	saturnYabaSanshiroExpandedSize  = saturnYabaSanshiroRawSize * 2
	saturnCombinedRawSize           = saturnInternalRawSize + saturnCartridgeRawSize
	saturnCombinedInterleavedSize   = saturnInternalInterleavedSize + saturnCartridgeInterleavedSize
	saturnInternalBlockSize         = 0x40
	saturnCartridgeBlockSize        = 0x200
	saturnArchiveEntryMarker uint32 = 0x80000000
	saturnDataEntryMarker    uint32 = 0x00000000
)

var saturnKnownFormats = map[string]struct{}{
	"original":            {},
	"mister":              {},
	"internal-raw":        {},
	"cartridge-raw":       {},
	"mednafen":            {},
	"mednafen-internal":   {},
	"mednafen-cartridge":  {},
	"yabause":             {},
	"yabasanshiro":        {},
	"bup":                 {},
	"ymir":                {},
	"ymbp":                {},
}

type saturnDetails struct {
	Container       string         `json:"container,omitempty"`
	Format          string         `json:"format,omitempty"`
	TotalImageBytes int            `json:"totalImageBytes,omitempty"`
	SaveEntries     int            `json:"saveEntries,omitempty"`
	DefaultVolume   string         `json:"defaultVolume,omitempty"`
	SampleFilename  string         `json:"sampleFilename,omitempty"`
	SampleComment   string         `json:"sampleComment,omitempty"`
	Volumes         []saturnVolume `json:"volumes,omitempty"`
	Entries         []saturnEntry  `json:"entries,omitempty"`
}

type saturnVolume struct {
	Name             string `json:"name,omitempty"`
	Kind             string `json:"kind,omitempty"`
	Interleaved      bool   `json:"interleaved,omitempty"`
	EncodedSizeBytes int    `json:"encodedSizeBytes,omitempty"`
	RawSizeBytes     int    `json:"rawSizeBytes,omitempty"`
	BlockSize        int    `json:"blockSize,omitempty"`
	TotalBlocks      int    `json:"totalBlocks,omitempty"`
	UsedBlocks       int    `json:"usedBlocks,omitempty"`
	FreeBlocks       int    `json:"freeBlocks,omitempty"`
	HeaderValid      bool   `json:"headerValid,omitempty"`
	SaveEntries      int    `json:"saveEntries,omitempty"`
	Empty            bool   `json:"empty,omitempty"`
}

type saturnEntry struct {
	Volume        string `json:"volume,omitempty"`
	Filename      string `json:"filename,omitempty"`
	Comment       string `json:"comment,omitempty"`
	LanguageCode  string `json:"languageCode,omitempty"`
	Language      string `json:"language,omitempty"`
	DateRaw       uint32 `json:"dateRaw,omitempty"`
	Date          string `json:"date,omitempty"`
	SaveSizeBytes int    `json:"saveSizeBytes,omitempty"`
	BlockCount    int    `json:"blockCount,omitempty"`
	FirstBlock    int    `json:"firstBlock,omitempty"`
	BlockIndexes  []int  `json:"blockIndexes,omitempty"`
}

type saturnParsedContainer struct {
	Details         *saturnDetails
	Format          string
	Internal        *saturnParsedVolume
	Cartridge       *saturnParsedVolume
	OriginalPayload []byte
}

type saturnParsedVolume struct {
	Summary saturnVolume
	Entries []saturnParsedEntry
	Raw     []byte
}

type saturnParsedEntry struct {
	Summary saturnEntry
	Data    []byte
}

func parseSaturnContainer(filename string, payload []byte) *saturnParsedContainer {
	payload = append([]byte(nil), payload...)
	if len(payload) == 0 {
		return nil
	}

	if parsed := parseSaturnDirectPayload(payload); parsed != nil {
		parsed.OriginalPayload = payload
		return parsed
	}

	if isGzipPayload(payload) {
		inflated, err := gunzipBytes(payload)
		if err == nil {
			if parsed := parseSaturnDirectPayload(inflated); parsed != nil {
				parsed.Format = "mednafen-cartridge-gzip"
				if parsed.Details != nil {
					parsed.Details.Format = parsed.Format
					parsed.Details.TotalImageBytes = len(payload)
				}
				parsed.OriginalPayload = payload
				return parsed
			}
		}
	}

	return nil
}

func parseSaturnDirectPayload(payload []byte) *saturnParsedContainer {
	switch len(payload) {
	case saturnInternalRawSize:
		internal := parseSaturnVolume(payload, "internal", false)
		if internal == nil {
			return nil
		}
		return buildSaturnContainer("internal-raw", payload, internal, nil)
	case saturnCartridgeRawSize:
		cart := parseSaturnVolume(payload, "cartridge", false)
		if cart == nil {
			return nil
		}
		return buildSaturnContainer("cartridge-raw", payload, nil, cart)
	case saturnInternalInterleavedSize:
		internal := parseSaturnVolume(collapseByteExpanded(payload), "internal", true)
		if internal == nil {
			return nil
		}
		return buildSaturnContainer("mister-internal-interleaved", payload, internal, nil)
	case saturnCartridgeInterleavedSize:
		cart := parseSaturnVolume(collapseByteExpanded(payload), "cartridge", true)
		if cart == nil {
			return nil
		}
		return buildSaturnContainer("cartridge-interleaved", payload, nil, cart)
	case saturnCombinedRawSize:
		internal := parseSaturnVolume(payload[:saturnInternalRawSize], "internal", false)
		if internal == nil {
			return nil
		}
		cart := parseSaturnOptionalVolume(payload[saturnInternalRawSize:], "cartridge", false)
		return buildSaturnContainer("combined-raw", payload, internal, cart)
	case saturnCombinedInterleavedSize:
		internal := parseSaturnVolume(collapseByteExpanded(payload[:saturnInternalInterleavedSize]), "internal", true)
		if internal == nil {
			return nil
		}
		cart := parseSaturnOptionalVolume(collapseByteExpanded(payload[saturnInternalInterleavedSize:]), "cartridge", true)
		return buildSaturnContainer("mister-combined-interleaved", payload, internal, cart)
	case saturnYabaSanshiroRawSize:
		internal := parseSaturnVolume(payload, "internal", false)
		if internal == nil {
			return nil
		}
		return buildSaturnContainer("yabasanshiro-raw", payload, internal, nil)
	case saturnYabaSanshiroExpandedSize:
		internal := parseSaturnVolume(collapseByteExpanded(payload), "internal", true)
		if internal == nil {
			return nil
		}
		return buildSaturnContainer("yabasanshiro-interleaved", payload, internal, nil)
	default:
		return nil
	}
}

func buildSaturnContainer(format string, originalPayload []byte, internal, cart *saturnParsedVolume) *saturnParsedContainer {
	details := &saturnDetails{
		Container:       "backup-ram",
		Format:          format,
		TotalImageBytes: len(originalPayload),
		Volumes:         []saturnVolume{},
		Entries:         []saturnEntry{},
	}
	if internal != nil {
		details.Volumes = append(details.Volumes, internal.Summary)
		details.Entries = append(details.Entries, summarizeSaturnEntries(internal.Entries)...)
	}
	if cart != nil {
		details.Volumes = append(details.Volumes, cart.Summary)
		details.Entries = append(details.Entries, summarizeSaturnEntries(cart.Entries)...)
	}
	details.SaveEntries = len(details.Entries)
	if internal != nil && internal.Summary.SaveEntries > 0 {
		details.DefaultVolume = "internal"
	} else if cart != nil && cart.Summary.SaveEntries > 0 {
		details.DefaultVolume = "cartridge"
	}
	if len(details.Entries) > 0 {
		details.SampleFilename = details.Entries[0].Filename
		details.SampleComment = details.Entries[0].Comment
	}
	return &saturnParsedContainer{
		Details:         details,
		Format:          format,
		Internal:        internal,
		Cartridge:       cart,
		OriginalPayload: originalPayload,
	}
}

func summarizeSaturnEntries(entries []saturnParsedEntry) []saturnEntry {
	out := make([]saturnEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Summary)
	}
	return out
}

func parseSaturnOptionalVolume(raw []byte, kind string, interleaved bool) *saturnParsedVolume {
	if len(raw) == 0 || isZeroFilled(raw) {
		blockSize := saturnBlockSizeForKind(kind)
		if blockSize == 0 {
			return nil
		}
		totalBlocks := len(raw) / blockSize
		return &saturnParsedVolume{Summary: saturnVolume{
			Name:             strings.Title(kind),
			Kind:             kind,
			Interleaved:      interleaved,
			EncodedSizeBytes: encodedSaturnSize(len(raw), interleaved),
			RawSizeBytes:     len(raw),
			BlockSize:        blockSize,
			TotalBlocks:      totalBlocks,
			FreeBlocks:       maxInt(totalBlocks-2, 0),
			HeaderValid:      false,
			SaveEntries:      0,
			Empty:            true,
		}}
	}
	parsed := parseSaturnVolume(raw, kind, interleaved)
	if parsed == nil {
		return &saturnParsedVolume{Summary: saturnVolume{
			Name:             strings.Title(kind),
			Kind:             kind,
			Interleaved:      interleaved,
			EncodedSizeBytes: encodedSaturnSize(len(raw), interleaved),
			RawSizeBytes:     len(raw),
			BlockSize:        saturnBlockSizeForKind(kind),
			TotalBlocks:      len(raw) / maxInt(saturnBlockSizeForKind(kind), 1),
			HeaderValid:      false,
			SaveEntries:      0,
			Empty:            true,
		}}
	}
	return parsed
}

func parseSaturnVolume(raw []byte, kind string, interleaved bool) *saturnParsedVolume {
	blockSize := saturnBlockSizeForRawSize(len(raw))
	if blockSize == 0 {
		blockSize = saturnBlockSizeForKind(kind)
	}
	if blockSize == 0 || len(raw) < blockSize*2 {
		return nil
	}
	if !saturnHeaderValid(raw, blockSize) {
		return nil
	}

	totalBlocks := len(raw) / blockSize
	used := map[int]struct{}{0: {}, 1: {}}
	entries := make([]saturnParsedEntry, 0)
	for block := 2; block < totalBlocks; block++ {
		offset := block * blockSize
		if offset+4 > len(raw) {
			break
		}
		if binary.BigEndian.Uint32(raw[offset:offset+4]) != saturnArchiveEntryMarker {
			continue
		}
		entry, blocks, ok := parseSaturnArchiveEntry(raw, blockSize, totalBlocks, kind, block)
		if !ok {
			continue
		}
		for _, blockIndex := range blocks {
			used[blockIndex] = struct{}{}
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Summary.FirstBlock == entries[j].Summary.FirstBlock {
			return entries[i].Summary.Filename < entries[j].Summary.Filename
		}
		return entries[i].Summary.FirstBlock < entries[j].Summary.FirstBlock
	})
	usedBlocks := len(used)
	freeBlocks := totalBlocks - usedBlocks
	if freeBlocks < 0 {
		freeBlocks = 0
	}
	return &saturnParsedVolume{
		Summary: saturnVolume{
			Name:             strings.Title(kind),
			Kind:             kind,
			Interleaved:      interleaved,
			EncodedSizeBytes: encodedSaturnSize(len(raw), interleaved),
			RawSizeBytes:     len(raw),
			BlockSize:        blockSize,
			TotalBlocks:      totalBlocks,
			UsedBlocks:       usedBlocks,
			FreeBlocks:       freeBlocks,
			HeaderValid:      true,
			SaveEntries:      len(entries),
			Empty:            len(entries) == 0,
		},
		Entries: entries,
		Raw:     append([]byte(nil), raw...),
	}
}

func parseSaturnArchiveEntry(raw []byte, blockSize, totalBlocks int, volume string, firstBlock int) (saturnParsedEntry, []int, bool) {
	offset := firstBlock * blockSize
	if offset+0x22 > len(raw) {
		return saturnParsedEntry{}, nil, false
	}
	languageRaw := int(raw[offset+0x0F])
	if languageRaw < 0 || languageRaw > 5 {
		return saturnParsedEntry{}, nil, false
	}
	filename := strings.TrimRight(extractPrintableASCII(raw[offset+0x04:offset+0x0F]), "\x00 ")
	comment := decodeShiftJISText(raw[offset+0x10 : offset+0x1A])
	dateRaw := binary.BigEndian.Uint32(raw[offset+0x1A : offset+0x1E])
	saveSize := int(binary.BigEndian.Uint32(raw[offset+0x1E : offset+0x22]))
	if saveSize < 0 || saveSize > len(raw) {
		return saturnParsedEntry{}, nil, false
	}
	blockIndexes, ok := saturnReadBlockList(raw, blockSize, totalBlocks, firstBlock)
	if !ok || len(blockIndexes) == 0 {
		return saturnParsedEntry{}, nil, false
	}
	data, ok := saturnExtractEntryData(raw, blockSize, blockIndexes, saveSize)
	if !ok {
		return saturnParsedEntry{}, nil, false
	}
	date := saturnBackupTime(dateRaw)
	entry := saturnParsedEntry{
		Summary: saturnEntry{
			Volume:        volume,
			Filename:      filename,
			Comment:       comment,
			LanguageCode:  saturnLanguageCode(languageRaw),
			Language:      saturnLanguageName(languageRaw),
			DateRaw:       dateRaw,
			Date:          date.Format(time.RFC3339),
			SaveSizeBytes: len(data),
			BlockCount:    len(blockIndexes),
			FirstBlock:    firstBlock,
			BlockIndexes:  append([]int(nil), blockIndexes...),
		},
		Data: data,
	}
	return entry, blockIndexes, true
}

func saturnReadBlockList(raw []byte, blockSize, totalBlocks, firstBlock int) ([]int, bool) {
	offset := firstBlock*blockSize + 0x22
	blocks := []int{firstBlock}
	listIndex := 1
	for {
		if offset+2 > len(raw) {
			return nil, false
		}
		nextBlock := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
		if nextBlock == 0 {
			break
		}
		if nextBlock < 0 || nextBlock >= totalBlocks {
			return nil, false
		}
		blocks = append(blocks, nextBlock)
		offset += 2
		if offset%(blockSize) == 0 {
			if listIndex >= len(blocks) {
				return nil, false
			}
			offset = blocks[listIndex]*blockSize + 4
			listIndex++
		}
	}
	return blocks, true
}

func saturnExtractEntryData(raw []byte, blockSize int, blocks []int, size int) ([]byte, bool) {
	blockListRemaining := len(blocks) * 2
	remaining := size
	data := make([]byte, 0, size)
	for blockIndex, block := range blocks {
		if remaining <= 0 {
			break
		}
		blockOffset := block * blockSize
		innerOffset := 0x04
		if blockIndex == 0 {
			innerOffset = 0x22
		}
		avail := blockSize - innerOffset
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
			return nil, false
		}
		if avail > remaining {
			avail = remaining
		}
		data = append(data, raw[blockOffset+innerOffset:blockOffset+innerOffset+avail]...)
		remaining -= avail
	}
	if remaining != 0 {
		return nil, false
	}
	return data, true
}

func saturnHeaderValid(raw []byte, blockSize int) bool {
	if len(raw) < blockSize*2 {
		return false
	}
	limit := saturnMinimumMagicBytes
	if limit > blockSize {
		limit = blockSize
	}
	for i := 0; i < limit; i++ {
		if raw[i] != saturnHeaderMagic[i%len(saturnHeaderMagic)] {
			return false
		}
	}
	for i := blockSize; i < blockSize*2; i++ {
		if raw[i] != 0x00 {
			return false
		}
	}
	return true
}

func collapseByteExpanded(payload []byte) []byte {
	if len(payload)%2 != 0 {
		return nil
	}
	out := make([]byte, len(payload)/2)
	for i := 0; i < len(out); i++ {
		out[i] = payload[i*2+1]
	}
	return out
}

func expandByteExpanded(raw []byte) []byte {
	out := make([]byte, len(raw)*2)
	for i, b := range raw {
		out[i*2] = 0xFF
		out[i*2+1] = b
	}
	return out
}

func encodedSaturnSize(rawSize int, interleaved bool) int {
	if interleaved {
		return rawSize * 2
	}
	return rawSize
}

func saturnBlockSizeForKind(kind string) int {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "internal":
		return saturnInternalBlockSize
	case "cartridge":
		return saturnCartridgeBlockSize
	default:
		return 0
	}
}

func saturnBlockSizeForRawSize(size int) int {
	switch size {
	case saturnInternalRawSize:
		return saturnInternalBlockSize
	case saturnCartridgeRawSize:
		return saturnCartridgeBlockSize
	default:
		return 0
	}
}

func isZeroFilled(payload []byte) bool {
	for _, b := range payload {
		if b != 0x00 {
			return false
		}
	}
	return true
}

func saturnLanguageCode(raw int) string {
	switch raw {
	case 0:
		return "JP"
	case 1:
		return "EN"
	case 2:
		return "FR"
	case 3:
		return "DE"
	case 4:
		return "ES"
	case 5:
		return "IT"
	default:
		return "UNKNOWN"
	}
}

func saturnLanguageName(raw int) string {
	switch raw {
	case 0:
		return "Japanese"
	case 1:
		return "English"
	case 2:
		return "French"
	case 3:
		return "German"
	case 4:
		return "Spanish"
	case 5:
		return "Italian"
	default:
		return "Unknown"
	}
}

func saturnBackupTime(raw uint32) time.Time {
	origin := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	return origin.Add(time.Duration(raw) * time.Minute)
}

func saturnDownloadPayload(record saveRecord, payload []byte, format, selectedEntry string) (string, string, []byte, error) {
	cleanFormat := strings.ToLower(strings.TrimSpace(format))
	if cleanFormat == "" || cleanFormat == "original" {
		return record.Summary.Filename, "application/octet-stream", payload, nil
	}
	if _, ok := saturnKnownFormats[cleanFormat]; !ok {
		return "", "", nil, fmt.Errorf("unsupported saturnFormat %q", format)
	}
	parsed := parseSaturnContainer(record.Summary.Filename, payload)
	if parsed == nil {
		return "", "", nil, fmt.Errorf("save is not a valid Saturn backup RAM image")
	}

	switch cleanFormat {
	case "mister":
		encoded, err := buildSaturnMisterPayload(parsed)
		if err != nil {
			return "", "", nil, err
		}
		return saturnDownloadName(record, ".sav"), "application/octet-stream", encoded, nil
	case "internal-raw":
		if parsed.Internal == nil || len(parsed.Internal.Raw) == 0 || !parsed.Internal.Summary.HeaderValid {
			return "", "", nil, fmt.Errorf("Saturn save does not contain internal backup RAM")
		}
		return saturnDownloadName(record, ".bkr"), "application/octet-stream", append([]byte(nil), parsed.Internal.Raw...), nil
	case "cartridge-raw":
		if parsed.Cartridge == nil || len(parsed.Cartridge.Raw) == 0 || !parsed.Cartridge.Summary.HeaderValid {
			return "", "", nil, fmt.Errorf("Saturn save does not contain cartridge backup RAM")
		}
		return saturnDownloadName(record, ".bcr"), "application/octet-stream", append([]byte(nil), parsed.Cartridge.Raw...), nil
	case "mednafen":
		if parsed.Internal != nil && len(parsed.Internal.Raw) > 0 && parsed.Internal.Summary.HeaderValid {
			return saturnDownloadName(record, ".bkr"), "application/octet-stream", append([]byte(nil), parsed.Internal.Raw...), nil
		}
		if parsed.Cartridge != nil && len(parsed.Cartridge.Raw) > 0 && parsed.Cartridge.Summary.HeaderValid {
			compressed, err := gzipBytes(parsed.Cartridge.Raw)
			if err != nil {
				return "", "", nil, err
			}
			return saturnDownloadName(record, ".bcr.gz"), "application/gzip", compressed, nil
		}
		return "", "", nil, fmt.Errorf("Saturn save does not contain a Mednafen-compatible backup RAM volume")
	case "mednafen-internal":
		if parsed.Internal == nil || len(parsed.Internal.Raw) == 0 || !parsed.Internal.Summary.HeaderValid {
			return "", "", nil, fmt.Errorf("Saturn save does not contain internal backup RAM")
		}
		return saturnDownloadName(record, ".bkr"), "application/octet-stream", append([]byte(nil), parsed.Internal.Raw...), nil
	case "mednafen-cartridge":
		if parsed.Cartridge == nil || len(parsed.Cartridge.Raw) == 0 || !parsed.Cartridge.Summary.HeaderValid {
			return "", "", nil, fmt.Errorf("Saturn save does not contain cartridge backup RAM")
		}
		compressed, err := gzipBytes(parsed.Cartridge.Raw)
		if err != nil {
			return "", "", nil, err
		}
		return saturnDownloadName(record, ".bcr.gz"), "application/gzip", compressed, nil
	case "yabause":
		if parsed.Internal == nil || len(parsed.Internal.Raw) == 0 || !parsed.Internal.Summary.HeaderValid {
			return "", "", nil, fmt.Errorf("Saturn save does not contain internal backup RAM")
		}
		return saturnDownloadName(record, ".sav"), "application/octet-stream", expandByteExpanded(parsed.Internal.Raw), nil
	case "yabasanshiro":
		if parsed.Internal == nil || len(parsed.Internal.Raw) == 0 || !parsed.Internal.Summary.HeaderValid {
			return "", "", nil, fmt.Errorf("Saturn save does not contain internal backup RAM")
		}
		raw := buildSaturnExtendedInternalRaw(parsed.Internal.Raw, saturnYabaSanshiroRawSize)
		return saturnDownloadName(record, ".sav"), "application/octet-stream", expandByteExpanded(raw), nil
	case "bup", "ymir", "ymbp":
		entry, err := selectSaturnExportEntry(parsed, selectedEntry)
		if err != nil {
			return "", "", nil, err
		}
		var built []byte
		filenameExt := ".bup"
		if cleanFormat == "ymbp" {
			built, err = buildSaturnYmBP(entry)
			filenameExt = ".ymbp"
		} else {
			built, err = buildSaturnBUP(entry)
		}
		if err != nil {
			return "", "", nil, err
		}
		return safeFilename(saturnEntryBaseName(entry)+filenameExt), "application/octet-stream", built, nil
	default:
		return "", "", nil, fmt.Errorf("unsupported saturnFormat %q", format)
	}
}

func buildSaturnMisterPayload(parsed *saturnParsedContainer) ([]byte, error) {
	if parsed == nil || parsed.Internal == nil || len(parsed.Internal.Raw) == 0 {
		return nil, fmt.Errorf("Saturn MiSTer payload requires internal backup RAM")
	}
	internal := expandByteExpanded(parsed.Internal.Raw)
	if parsed.Cartridge == nil || len(parsed.Cartridge.Raw) == 0 || !parsed.Cartridge.Summary.HeaderValid || parsed.Cartridge.Summary.Empty {
		return internal, nil
	}
	cart := expandByteExpanded(parsed.Cartridge.Raw)
	return append(internal, cart...), nil
}

func buildSaturnExtendedInternalRaw(raw []byte, size int) []byte {
	if len(raw) >= size {
		return append([]byte(nil), raw[:size]...)
	}
	out := make([]byte, size)
	copy(out, raw)
	return out
}

func selectSaturnExportEntry(parsed *saturnParsedContainer, selected string) (saturnParsedEntry, error) {
	entries := make([]saturnParsedEntry, 0)
	if parsed.Internal != nil {
		entries = append(entries, parsed.Internal.Entries...)
	}
	if parsed.Cartridge != nil {
		entries = append(entries, parsed.Cartridge.Entries...)
	}
	if len(entries) == 0 {
		return saturnParsedEntry{}, fmt.Errorf("Saturn backup RAM image has no save entries")
	}
	cleanSelected := strings.TrimSpace(selected)
	if cleanSelected != "" {
		for _, entry := range entries {
			if strings.EqualFold(strings.TrimSpace(entry.Summary.Filename), cleanSelected) {
				return entry, nil
			}
		}
		return saturnParsedEntry{}, fmt.Errorf("Saturn save entry %q not found", selected)
	}
	if len(entries) != 1 {
		return saturnParsedEntry{}, fmt.Errorf("Saturn image contains multiple save entries; specify saturnEntry")
	}
	return entries[0], nil
}

func buildSaturnBUP(entry saturnParsedEntry) ([]byte, error) {
	comment, err := encodeShiftJISFixed(entry.Summary.Comment, 10)
	if err != nil {
		return nil, err
	}
	filename := encodeASCIIFixed(entry.Summary.Filename, 11)
	out := make([]byte, 0x40+len(entry.Data))
	copy(out[:4], []byte("Vmem"))
	copy(out[0x10:0x1B], filename)
	copy(out[0x1C:0x26], comment)
	out[0x27] = saturnLanguageRaw(entry.Summary.LanguageCode)
	binary.BigEndian.PutUint32(out[0x28:0x2C], entry.Summary.DateRaw)
	binary.BigEndian.PutUint32(out[0x2C:0x30], uint32(len(entry.Data)))
	binary.BigEndian.PutUint16(out[0x30:0x32], uint16(entry.Summary.BlockCount))
	binary.BigEndian.PutUint32(out[0x34:0x38], entry.Summary.DateRaw)
	copy(out[0x40:], entry.Data)
	return out, nil
}

func buildSaturnYmBP(entry saturnParsedEntry) ([]byte, error) {
	comment, err := encodeShiftJISFixed(entry.Summary.Comment, 10)
	if err != nil {
		return nil, err
	}
	filename := encodeASCIIFixed(entry.Summary.Filename, 11)
	out := make([]byte, 0x22+len(entry.Data))
	copy(out[:4], []byte("YmBP"))
	copy(out[0x04:0x0F], filename)
	out[0x0F] = saturnLanguageRaw(entry.Summary.LanguageCode)
	copy(out[0x10:0x1A], comment)
	binary.LittleEndian.PutUint32(out[0x1A:0x1E], entry.Summary.DateRaw)
	binary.LittleEndian.PutUint32(out[0x1E:0x22], uint32(len(entry.Data)))
	copy(out[0x22:], entry.Data)
	return out, nil
}

func saturnLanguageRaw(code string) byte {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "JP":
		return 0
	case "EN":
		return 1
	case "FR":
		return 2
	case "DE":
		return 3
	case "ES":
		return 4
	case "IT":
		return 5
	default:
		return 1
	}
}

func encodeASCIIFixed(value string, size int) []byte {
	out := make([]byte, size)
	copy(out, []byte(extractPrintableASCII([]byte(value))))
	return out
}

func encodeShiftJISFixed(value string, size int) ([]byte, error) {
	out := make([]byte, size)
	clean := strings.TrimSpace(value)
	if clean == "" {
		return out, nil
	}
	encoded, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(clean))
	if err != nil {
		fallback := []byte(extractPrintableASCII([]byte(clean)))
		copy(out, fallback)
		return out, nil
	}
	copy(out, encoded)
	return out, nil
}

func saturnEntryBaseName(entry saturnParsedEntry) string {
	for _, candidate := range []string{entry.Summary.Filename, entry.Summary.Comment} {
		clean := strings.TrimSpace(candidate)
		if clean != "" {
			return safeFilename(clean)
		}
	}
	return "saturn-save"
}

func saturnDownloadName(record saveRecord, ext string) string {
	base := strings.TrimSuffix(strings.TrimSpace(record.Summary.Filename), filepath.Ext(strings.TrimSpace(record.Summary.Filename)))
	if base == "" {
		base = strings.TrimSpace(record.Summary.DisplayTitle)
	}
	if base == "" {
		base = "saturn-save"
	}
	return safeFilename(base + ext)
}

func isGzipPayload(payload []byte) bool {
	return len(payload) >= 2 && payload[0] == 0x1f && payload[1] == 0x8b
}

func gunzipBytes(payload []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func gzipBytes(payload []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(payload); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
