package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"path/filepath"
	"strings"
)

const (
	dreamcastBlockSize      = 512
	dreamcastVMUBlockCount  = 256
	dreamcastVMUSize        = dreamcastBlockSize * dreamcastVMUBlockCount
	dreamcastRootBlock      = 255
	dreamcastDirEntrySize   = 32
	dreamcastDCIHeaderSize  = 32
	dreamcastFileTypeData   = 0x33
	dreamcastFileTypeGame   = 0xCC
	dreamcastFATFree        = 0xFFFC
	dreamcastFATEnd         = 0xFFFA
	dreamcastHeaderSize     = 0x80
	dreamcastIconWidth      = 32
	dreamcastIconHeight     = 32
	dreamcastIconFrameBytes = 512
	dreamcastEyecatchWidth  = 72
	dreamcastEyecatchHeight = 56
)

var dreamcastNVMEMMagic = []byte("KATANA_FLASH____")

type dreamcastDetails struct {
	Container             string           `json:"container,omitempty"`
	SlotName              string           `json:"slotName,omitempty"`
	SaveEntries           int              `json:"saveEntries,omitempty"`
	IconFrames            int              `json:"iconFrames,omitempty"`
	SampleTitle           string           `json:"sampleTitle,omitempty"`
	SampleAppID           string           `json:"sampleAppId,omitempty"`
	SampleIconDataURL     string           `json:"sampleIconDataUrl,omitempty"`
	SampleEyecatchDataURL string           `json:"sampleEyecatchDataUrl,omitempty"`
	Entries               []dreamcastEntry `json:"entries,omitempty"`
}

type dreamcastEntry struct {
	DirectoryName      string `json:"directoryName,omitempty"`
	FileType           string `json:"fileType,omitempty"`
	FirstBlock         int    `json:"firstBlock,omitempty"`
	FileSizeBlocks     int    `json:"fileSizeBlocks,omitempty"`
	HeaderOffsetBlocks int    `json:"headerOffsetBlocks,omitempty"`
	ShortDescription   string `json:"shortDescription,omitempty"`
	LongDescription    string `json:"longDescription,omitempty"`
	AppID              string `json:"appId,omitempty"`
	IconCount          int    `json:"iconCount,omitempty"`
	IconAnimationSpeed int    `json:"iconAnimationSpeed,omitempty"`
	EyecatchType       int    `json:"eyecatchType,omitempty"`
	EyecatchTypeLabel  string `json:"eyecatchTypeLabel,omitempty"`
	CRC                uint16 `json:"crc,omitempty"`
	CRCValid           *bool  `json:"crcValid,omitempty"`
	DataLength         int    `json:"dataLength,omitempty"`
	IconDataURL        string `json:"iconDataUrl,omitempty"`
	EyecatchDataURL    string `json:"eyecatchDataUrl,omitempty"`
}

type dreamcastRoot struct {
	fatLoc     int
	fatSize    int
	dirLoc     int
	dirSize    int
	userBlocks int
}

type dreamcastDirectoryEntry struct {
	fileType           byte
	firstBlock         int
	fileSizeBlocks     int
	headerOffsetBlocks int
	directoryName      string
}

type dreamcastPackage struct {
	shortDescription   string
	longDescription    string
	appID              string
	iconCount          int
	iconAnimationSpeed int
	eyecatchType       int
	eyecatchTypeLabel  string
	crc                uint16
	crcValid           *bool
	dataLength         int
	iconDataURL        string
	eyecatchDataURL    string
}

type dreamcastPackageParseOptions struct {
	headerOffset     int
	ignoreCRC        bool
	ignoreDataLength bool
}

func parseDreamcastContainer(filename string, payload []byte) *dreamcastDetails {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(filename))), ".")
	if ext == "bin" || len(payload) == dreamcastVMUSize {
		return parseDreamcastVMUImage(payload)
	}
	switch ext {
	case "dci":
		return parseDreamcastDCIPayload(payload)
	case "vms":
		return parseDreamcastVMSPayload(payload)
	}
	return nil
}

func parseDreamcastVMUImage(payload []byte) *dreamcastDetails {
	root, ok := parseDreamcastRoot(payload)
	if !ok {
		return nil
	}
	fat, ok := parseDreamcastFAT(payload, root)
	if !ok {
		return nil
	}

	directoryBlocks, ok := collectDreamcastBlockChain(
		root.dirLoc,
		root.dirSize,
		fat,
		root.userBlocks+root.dirSize+root.fatSize+4,
	)
	if !ok {
		return nil
	}

	details := &dreamcastDetails{Container: "vmu-bin"}
	entries := make([]dreamcastEntry, 0)
	for _, block := range directoryBlocks {
		start := block * dreamcastBlockSize
		end := start + dreamcastBlockSize
		if end > len(payload) {
			return nil
		}
		for _, chunk := range chunkDreamcastDirectoryBlock(payload[start:end]) {
			directory, ok := parseDreamcastDirectoryEntry(chunk)
			if !ok {
				return nil
			}
			if directory.fileType != dreamcastFileTypeData && directory.fileType != dreamcastFileTypeGame {
				continue
			}
			details.SaveEntries++
			entry := dreamcastEntry{
				DirectoryName:      directory.directoryName,
				FileType:           dreamcastFileTypeLabel(directory.fileType),
				FirstBlock:         directory.firstBlock,
				FileSizeBlocks:     directory.fileSizeBlocks,
				HeaderOffsetBlocks: directory.headerOffsetBlocks,
			}
			fileBytes, ok := collectDreamcastFileBytes(payload, fat, directory)
			if ok {
				pkg := parseDreamcastPackageFromFile(fileBytes, dreamcastPackageParseOptions{
					headerOffset:     directory.headerOffsetBlocks * dreamcastBlockSize,
					ignoreCRC:        directory.fileType == dreamcastFileTypeGame,
					ignoreDataLength: directory.fileType == dreamcastFileTypeGame,
				})
				if pkg != nil {
					mergeDreamcastEntryMetadata(&entry, pkg)
					details.IconFrames += pkg.iconCount
					if details.SampleTitle == "" {
						details.SampleTitle = dreamcastPackageDisplayTitle(pkg)
					}
					if details.SampleAppID == "" {
						details.SampleAppID = strings.TrimSpace(pkg.appID)
					}
					if details.SampleIconDataURL == "" {
						details.SampleIconDataURL = strings.TrimSpace(pkg.iconDataURL)
					}
					if details.SampleEyecatchDataURL == "" {
						details.SampleEyecatchDataURL = strings.TrimSpace(pkg.eyecatchDataURL)
					}
				}
			}
			entries = append(entries, entry)
		}
	}
	details.Entries = entries
	return details
}

func parseDreamcastDCIPayload(payload []byte) *dreamcastDetails {
	if len(payload) < dreamcastDCIHeaderSize+dreamcastBlockSize {
		return nil
	}
	directory, ok := parseDreamcastDirectoryEntry(payload[:dreamcastDCIHeaderSize])
	if !ok || (directory.fileType != dreamcastFileTypeData && directory.fileType != dreamcastFileTypeGame) {
		return nil
	}
	expectedLen := dreamcastDCIHeaderSize + directory.fileSizeBlocks*dreamcastBlockSize
	if directory.fileSizeBlocks <= 0 || expectedLen > len(payload) {
		return nil
	}
	fileBytes := make([]byte, 0, directory.fileSizeBlocks*dreamcastBlockSize)
	for offset := dreamcastDCIHeaderSize; offset < expectedLen; offset += dreamcastBlockSize {
		fileBytes = append(fileBytes, dreamcastUnswap32BitChunks(payload[offset:offset+dreamcastBlockSize])...)
	}
	pkg := parseDreamcastPackageFromFile(fileBytes, dreamcastPackageParseOptions{
		headerOffset:     directory.headerOffsetBlocks * dreamcastBlockSize,
		ignoreCRC:        directory.fileType == dreamcastFileTypeGame,
		ignoreDataLength: directory.fileType == dreamcastFileTypeGame,
	})
	if pkg == nil {
		return nil
	}
	entry := dreamcastEntry{
		DirectoryName:      directory.directoryName,
		FileType:           dreamcastFileTypeLabel(directory.fileType),
		FirstBlock:         directory.firstBlock,
		FileSizeBlocks:     directory.fileSizeBlocks,
		HeaderOffsetBlocks: directory.headerOffsetBlocks,
	}
	mergeDreamcastEntryMetadata(&entry, pkg)
	return &dreamcastDetails{
		Container:             "dci",
		SaveEntries:           1,
		IconFrames:            pkg.iconCount,
		SampleTitle:           dreamcastPackageDisplayTitle(pkg),
		SampleAppID:           strings.TrimSpace(pkg.appID),
		SampleIconDataURL:     strings.TrimSpace(pkg.iconDataURL),
		SampleEyecatchDataURL: strings.TrimSpace(pkg.eyecatchDataURL),
		Entries:               []dreamcastEntry{entry},
	}
}

func parseDreamcastVMSPayload(payload []byte) *dreamcastDetails {
	for _, options := range []dreamcastPackageParseOptions{
		{headerOffset: 0},
		{headerOffset: dreamcastBlockSize, ignoreCRC: true, ignoreDataLength: true},
	} {
		pkg := parseDreamcastPackageFromFile(payload, options)
		if pkg == nil {
			continue
		}
		entry := dreamcastEntry{}
		mergeDreamcastEntryMetadata(&entry, pkg)
		return &dreamcastDetails{
			Container:             "vms",
			SaveEntries:           1,
			IconFrames:            pkg.iconCount,
			SampleTitle:           dreamcastPackageDisplayTitle(pkg),
			SampleAppID:           strings.TrimSpace(pkg.appID),
			SampleIconDataURL:     strings.TrimSpace(pkg.iconDataURL),
			SampleEyecatchDataURL: strings.TrimSpace(pkg.eyecatchDataURL),
			Entries:               []dreamcastEntry{entry},
		}
	}
	return nil
}

func parseDreamcastPackageFromFile(fileBytes []byte, options dreamcastPackageParseOptions) *dreamcastPackage {
	if options.headerOffset < 0 || options.headerOffset+dreamcastHeaderSize > len(fileBytes) {
		return nil
	}
	data := fileBytes[options.headerOffset:]
	if len(data) < dreamcastHeaderSize {
		return nil
	}

	iconCount := int(binary.LittleEndian.Uint16(data[0x40:0x42]))
	if iconCount < 0 || iconCount > 16 {
		return nil
	}
	eyecatchType := int(binary.LittleEndian.Uint16(data[0x44:0x46]))
	eyecatchSize, ok := dreamcastEyecatchDataSize(eyecatchType)
	if !ok {
		return nil
	}
	iconBytes := iconCount * dreamcastIconFrameBytes
	required := dreamcastHeaderSize + iconBytes + eyecatchSize
	if required > len(data) {
		return nil
	}

	pkg := &dreamcastPackage{
		shortDescription:   decodeDreamcastText(data[0x00:0x10]),
		longDescription:    decodeDreamcastText(data[0x10:0x30]),
		appID:              decodeDreamcastText(data[0x30:0x40]),
		iconCount:          iconCount,
		iconAnimationSpeed: int(binary.LittleEndian.Uint16(data[0x42:0x44])),
		eyecatchType:       eyecatchType,
		eyecatchTypeLabel:  dreamcastEyecatchTypeLabel(eyecatchType),
		crc:                binary.LittleEndian.Uint16(data[0x46:0x48]),
		dataLength:         int(binary.LittleEndian.Uint32(data[0x48:0x4C])),
	}

	iconPalette := data[0x60:0x80]
	iconStart := dreamcastHeaderSize
	if pkg.iconCount > 0 {
		pkg.iconDataURL = parseDreamcastIconDataURL(iconPalette, data[iconStart:iconStart+dreamcastIconFrameBytes])
	}
	eyecatchStart := iconStart + iconBytes
	if eyecatchSize > 0 {
		pkg.eyecatchDataURL = parseDreamcastEyecatchDataURL(eyecatchType, data[eyecatchStart:eyecatchStart+eyecatchSize])
	}

	if !options.ignoreDataLength {
		totalSize := options.headerOffset + required + pkg.dataLength
		if pkg.dataLength < 0 || totalSize > len(fileBytes) {
			return nil
		}
		if !options.ignoreCRC {
			crc := dreamcastCRCForPayload(fileBytes[:totalSize], options.headerOffset)
			valid := crc == pkg.crc
			pkg.crcValid = &valid
		}
	}

	return pkg
}

func mergeDreamcastEntryMetadata(entry *dreamcastEntry, pkg *dreamcastPackage) {
	if entry == nil || pkg == nil {
		return
	}
	entry.ShortDescription = strings.TrimSpace(pkg.shortDescription)
	entry.LongDescription = strings.TrimSpace(pkg.longDescription)
	entry.AppID = strings.TrimSpace(pkg.appID)
	entry.IconCount = pkg.iconCount
	entry.IconAnimationSpeed = pkg.iconAnimationSpeed
	entry.EyecatchType = pkg.eyecatchType
	entry.EyecatchTypeLabel = pkg.eyecatchTypeLabel
	entry.CRC = pkg.crc
	entry.CRCValid = pkg.crcValid
	entry.DataLength = pkg.dataLength
	entry.IconDataURL = strings.TrimSpace(pkg.iconDataURL)
	entry.EyecatchDataURL = strings.TrimSpace(pkg.eyecatchDataURL)
}

func dreamcastPackageDisplayTitle(pkg *dreamcastPackage) string {
	if pkg == nil {
		return ""
	}
	if title := strings.TrimSpace(pkg.shortDescription); title != "" {
		return title
	}
	return strings.TrimSpace(pkg.longDescription)
}

func parseDreamcastRoot(payload []byte) (*dreamcastRoot, bool) {
	if len(payload) != dreamcastVMUSize || bytes.HasPrefix(payload, dreamcastNVMEMMagic) {
		return nil, false
	}
	rootStart := dreamcastRootBlock * dreamcastBlockSize
	rootEnd := rootStart + dreamcastBlockSize
	rootBytes := payload[rootStart:rootEnd]
	for _, value := range rootBytes[:16] {
		if value != 0x55 {
			return nil, false
		}
	}

	root := &dreamcastRoot{
		fatLoc:     int(binary.LittleEndian.Uint16(rootBytes[0x46:0x48])),
		fatSize:    int(binary.LittleEndian.Uint16(rootBytes[0x48:0x4A])),
		dirLoc:     int(binary.LittleEndian.Uint16(rootBytes[0x4A:0x4C])),
		dirSize:    int(binary.LittleEndian.Uint16(rootBytes[0x4C:0x4E])),
		userBlocks: int(binary.LittleEndian.Uint16(rootBytes[0x50:0x52])),
	}
	if root.fatSize <= 0 || root.dirSize <= 0 || root.fatLoc < 0 || root.fatLoc >= dreamcastVMUBlockCount {
		return nil, false
	}
	if root.dirLoc < 0 || root.dirLoc >= dreamcastVMUBlockCount || root.userBlocks <= 0 || root.userBlocks > 200 {
		return nil, false
	}
	return root, true
}

func parseDreamcastFAT(payload []byte, root *dreamcastRoot) ([]uint16, bool) {
	if root == nil {
		return nil, false
	}
	fatStart := root.fatLoc * dreamcastBlockSize
	fatLen := root.fatSize * dreamcastBlockSize
	fatEnd := fatStart + fatLen
	if fatStart < 0 || fatEnd > len(payload) {
		return nil, false
	}
	fatBytes := payload[fatStart:fatEnd]
	if len(fatBytes) < dreamcastVMUBlockCount*2 {
		return nil, false
	}
	fat := make([]uint16, dreamcastVMUBlockCount)
	for i := range fat {
		offset := i * 2
		fat[i] = binary.LittleEndian.Uint16(fatBytes[offset : offset+2])
	}
	return fat, true
}

func chunkDreamcastDirectoryBlock(block []byte) [][]byte {
	chunks := make([][]byte, 0, len(block)/dreamcastDirEntrySize)
	for offset := 0; offset+dreamcastDirEntrySize <= len(block); offset += dreamcastDirEntrySize {
		chunks = append(chunks, block[offset:offset+dreamcastDirEntrySize])
	}
	return chunks
}

func parseDreamcastDirectoryEntry(data []byte) (dreamcastDirectoryEntry, bool) {
	if len(data) != dreamcastDirEntrySize {
		return dreamcastDirectoryEntry{}, false
	}
	fileType := data[0]
	if fileType == 0 {
		return dreamcastDirectoryEntry{fileType: fileType}, true
	}
	entry := dreamcastDirectoryEntry{
		fileType:           fileType,
		firstBlock:         int(binary.LittleEndian.Uint16(data[0x02:0x04])),
		fileSizeBlocks:     int(binary.LittleEndian.Uint16(data[0x18:0x1A])),
		headerOffsetBlocks: int(binary.LittleEndian.Uint16(data[0x1A:0x1C])),
		directoryName:      strings.TrimSpace(string(bytes.Trim(data[0x04:0x10], "\x00 "))),
	}
	if entry.firstBlock < 0 || entry.firstBlock >= dreamcastVMUBlockCount || entry.fileSizeBlocks <= 0 {
		return dreamcastDirectoryEntry{}, false
	}
	if entry.headerOffsetBlocks < 0 || entry.headerOffsetBlocks > entry.fileSizeBlocks {
		return dreamcastDirectoryEntry{}, false
	}
	return entry, true
}

func collectDreamcastFileBytes(payload []byte, fat []uint16, entry dreamcastDirectoryEntry) ([]byte, bool) {
	chain, ok := collectDreamcastBlockChain(entry.firstBlock, entry.fileSizeBlocks, fat, entry.fileSizeBlocks+2)
	if !ok || len(chain) == 0 || len(chain) > entry.fileSizeBlocks {
		return nil, false
	}
	out := make([]byte, 0, len(chain)*dreamcastBlockSize)
	for _, block := range chain {
		if block < 0 || block >= 200 {
			return nil, false
		}
		start := block * dreamcastBlockSize
		end := start + dreamcastBlockSize
		if end > len(payload) {
			return nil, false
		}
		out = append(out, payload[start:end]...)
	}
	return out, true
}

func collectDreamcastBlockChain(startBlock, expectedBlocks int, fat []uint16, hardLimit int) ([]int, bool) {
	if startBlock < 0 || startBlock >= len(fat) || expectedBlocks <= 0 {
		return nil, false
	}
	seen := make([]bool, dreamcastVMUBlockCount)
	chain := make([]int, 0, expectedBlocks)
	current := startBlock
	for len(chain) < hardLimit && current >= 0 && current < len(fat) {
		if current >= dreamcastVMUBlockCount || seen[current] {
			return nil, false
		}
		seen[current] = true
		chain = append(chain, current)
		if len(chain) >= expectedBlocks {
			break
		}
		next := fat[current]
		if next == dreamcastFATEnd {
			break
		}
		if next == dreamcastFATFree {
			return nil, false
		}
		current = int(next)
	}
	if len(chain) == 0 {
		return nil, false
	}
	return chain, true
}

func dreamcastUnswap32BitChunks(data []byte) []byte {
	out := append([]byte(nil), data...)
	for offset := 0; offset+4 <= len(out); offset += 4 {
		out[offset+0], out[offset+1], out[offset+2], out[offset+3] = out[offset+3], out[offset+2], out[offset+1], out[offset+0]
	}
	return out
}

func decodeDreamcastText(data []byte) string {
	trimmed := bytes.TrimRight(data, "\x00 ")
	if len(trimmed) == 0 {
		return ""
	}
	if decoded := decodeShiftJISText(trimmed); decoded != "" {
		return decoded
	}
	return strings.TrimSpace(extractPrintableASCII(trimmed))
}

func parseDreamcastIconDataURL(paletteBytes, bitmap []byte) string {
	if len(paletteBytes) < 32 || len(bitmap) < dreamcastIconFrameBytes {
		return ""
	}
	img := image.NewNRGBA(image.Rect(0, 0, dreamcastIconWidth, dreamcastIconHeight))
	palette := dreamcastPaletteFromARGB4444(paletteBytes, 16)
	for y := 0; y < dreamcastIconHeight; y++ {
		for x := 0; x < dreamcastIconWidth; x += 2 {
			offset := (y*dreamcastIconWidth + x) / 2
			b := bitmap[offset]
			left := int(b >> 4)
			right := int(b & 0x0F)
			img.Set(x, y, palette[left])
			img.Set(x+1, y, palette[right])
		}
	}
	return pngDataURLFromImage(img)
}

func parseDreamcastEyecatchDataURL(mode int, data []byte) string {
	img := image.NewNRGBA(image.Rect(0, 0, dreamcastEyecatchWidth, dreamcastEyecatchHeight))
	switch mode {
	case 0:
		return ""
	case 1:
		if len(data) < dreamcastEyecatchWidth*dreamcastEyecatchHeight*2 {
			return ""
		}
		for y := 0; y < dreamcastEyecatchHeight; y++ {
			for x := 0; x < dreamcastEyecatchWidth; x++ {
				offset := (y*dreamcastEyecatchWidth + x) * 2
				value := binary.LittleEndian.Uint16(data[offset : offset+2])
				img.Set(x, y, dreamcastColorFromARGB4444(value))
			}
		}
	case 2:
		if len(data) < 512+dreamcastEyecatchWidth*dreamcastEyecatchHeight {
			return ""
		}
		palette := dreamcastPaletteFromARGB4444(data[:512], 256)
		bitmap := data[512:]
		for y := 0; y < dreamcastEyecatchHeight; y++ {
			for x := 0; x < dreamcastEyecatchWidth; x++ {
				img.Set(x, y, palette[int(bitmap[y*dreamcastEyecatchWidth+x])])
			}
		}
	case 3:
		if len(data) < 32+(dreamcastEyecatchWidth*dreamcastEyecatchHeight)/2 {
			return ""
		}
		palette := dreamcastPaletteFromARGB4444(data[:32], 16)
		bitmap := data[32:]
		for y := 0; y < dreamcastEyecatchHeight; y++ {
			for x := 0; x < dreamcastEyecatchWidth; x += 2 {
				offset := (y*dreamcastEyecatchWidth + x) / 2
				b := bitmap[offset]
				left := int(b >> 4)
				right := int(b & 0x0F)
				img.Set(x, y, palette[left])
				img.Set(x+1, y, palette[right])
			}
		}
	default:
		return ""
	}
	return pngDataURLFromImage(img)
}

func dreamcastPaletteFromARGB4444(data []byte, entries int) []color.NRGBA {
	palette := make([]color.NRGBA, entries)
	for i := 0; i < entries; i++ {
		offset := i * 2
		if offset+2 > len(data) {
			break
		}
		palette[i] = dreamcastColorFromARGB4444(binary.LittleEndian.Uint16(data[offset : offset+2]))
	}
	return palette
}

func dreamcastColorFromARGB4444(value uint16) color.NRGBA {
	a := uint8((value >> 12) & 0x0F)
	r := uint8((value >> 8) & 0x0F)
	g := uint8((value >> 4) & 0x0F)
	b := uint8(value & 0x0F)
	return color.NRGBA{R: r<<4 | r, G: g<<4 | g, B: b<<4 | b, A: a<<4 | a}
}

func dreamcastEyecatchDataSize(mode int) (int, bool) {
	switch mode {
	case 0:
		return 0, true
	case 1:
		return dreamcastEyecatchWidth * dreamcastEyecatchHeight * 2, true
	case 2:
		return 512 + dreamcastEyecatchWidth*dreamcastEyecatchHeight, true
	case 3:
		return 32 + (dreamcastEyecatchWidth*dreamcastEyecatchHeight)/2, true
	default:
		return 0, false
	}
}

func dreamcastEyecatchTypeLabel(mode int) string {
	switch mode {
	case 0:
		return "none"
	case 1:
		return "argb4444"
	case 2:
		return "palette-256"
	case 3:
		return "palette-16"
	default:
		return "unknown"
	}
}

func dreamcastFileTypeLabel(fileType byte) string {
	switch fileType {
	case dreamcastFileTypeData:
		return "data"
	case dreamcastFileTypeGame:
		return "game"
	default:
		return "unknown"
	}
}

func dreamcastCRCForPayload(payload []byte, headerOffset int) uint16 {
	if headerOffset < 0 || headerOffset+0x48 > len(payload) {
		return 0
	}
	copyPayload := append([]byte(nil), payload...)
	copyPayload[headerOffset+0x46] = 0
	copyPayload[headerOffset+0x47] = 0
	var value uint32
	for _, b := range copyPayload {
		value ^= uint32(b) << 8
		for i := 0; i < 8; i++ {
			if value&0x8000 != 0 {
				value = (value << 1) ^ 0x1021
			} else {
				value <<= 1
			}
		}
	}
	return uint16(value & 0xFFFF)
}
