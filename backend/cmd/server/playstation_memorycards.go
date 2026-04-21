package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type saveArtifactKind string

const (
	saveArtifactGeneric        saveArtifactKind = "generic"
	saveArtifactPS1MemoryCard saveArtifactKind = "ps1-memory-card"
	saveArtifactPS2MemoryCard saveArtifactKind = "ps2-memory-card"
	saveArtifactUnsupported   saveArtifactKind = "unsupported"
)

const (
	ps2FATAllocatedBit   = 0x80000000
	ps2FATChainEnd       = 0xFFFFFFFF
	ps2DirectoryEntSize  = 512
	ps2IconTextureWidth  = 128
	ps2IconTextureHeight = 128
	ps2IconTextureSize   = ps2IconTextureWidth * ps2IconTextureHeight * 2
)

var ps2DirectoryCodePattern = regexp.MustCompile(`\b([A-Z]{4,8}[-_][A-Z0-9]{4,6})\b`)

type ps2DirectoryEntry struct {
	Mode         uint16
	Length       uint32
	FirstCluster uint32
	ParentEntry  uint32
	Name         string
}

type ps2MemoryCardReader struct {
	payload                  []byte
	pageSize                 int
	rawPageSize              int
	pagesPerCluster          int
	clusterSize              int
	allocatableClusterOffset int
	allocatableClusterEnd    int
	entriesPerCluster        int
	indirectFAT              []uint32
}

type ps2IconSys struct {
	TitleLineOne   string
	TitleLineTwo   string
	IconFileNormal string
}

func classifyPlayStationArtifact(sys *system, format, filename string, payload []byte) saveArtifactKind {
	if isConfirmedPS1MemoryCard(sys, format, filename, payload) {
		return saveArtifactPS1MemoryCard
	}
	if isConfirmedPS2MemoryCard(sys, format, filename, payload) {
		return saveArtifactPS2MemoryCard
	}
	if sys == nil {
		return saveArtifactGeneric
	}
	slug := supportedSystemSlugFromLabel(firstNonEmpty(sys.Slug, sys.Name))
	switch slug {
	case "psx", "ps2":
		return saveArtifactUnsupported
	default:
		return saveArtifactGeneric
	}
}

func playStationRejectReason(sys *system) string {
	slug := ""
	if sys != nil {
		slug = supportedSystemSlugFromLabel(firstNonEmpty(sys.Slug, sys.Name))
	}
	switch slug {
	case "ps2":
		return "only real PlayStation 2 memory card images are supported"
	case "psx":
		return "only real PlayStation memory card images are supported"
	default:
		return "only real PlayStation memory card images are supported for PS1/PS2 sync"
	}
}

func isPS2System(sys *system) bool {
	if sys == nil {
		return false
	}
	return supportedSystemSlugFromLabel(firstNonEmpty(sys.Slug, sys.Name)) == "ps2"
}

func isConfirmedPS2MemoryCard(sys *system, format, filename string, payload []byte) bool {
	if sys != nil && !isPS2System(sys) {
		return false
	}
	if !isLikelyPS2MemoryCard(payload) {
		return false
	}
	_ = format
	_ = filename
	return true
}

func parsePlayStationMemoryCard(sys *system, payload []byte, filename, cardName string) *memoryCardDetails {
	switch classifyPlayStationArtifact(sys, "", filename, payload) {
	case saveArtifactPS2MemoryCard:
		return parsePS2MemoryCard(payload, cardName)
	case saveArtifactPS1MemoryCard:
		return parsePS1MemoryCard(payload, filename, cardName)
	default:
		return nil
	}
}

func parsePS1MemoryCard(payload []byte, filename, cardName string) *memoryCardDetails {
	imageData := normalizedPS1MemoryCardImage(payload, strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), "."))
	if len(imageData) < psMemoryCardBlockSize {
		return nil
	}

	entries := make([]memoryCardEntry, 0, 8)
	for dirIndex := 1; dirIndex <= psDirectoryEntries; dirIndex++ {
		offset := dirIndex * psDirectoryEntrySize
		if offset+psDirectoryEntrySize > len(imageData) {
			break
		}
		dirEntry := imageData[offset : offset+psDirectoryEntrySize]
		if !isPS1DirectoryStartEntry(dirEntry[0]) {
			continue
		}

		productCode := strings.TrimSpace(extractPrintableASCII(dirEntry[0x0a:0x16]))
		slot := dirIndex
		blocks := countDirectoryBlocks(imageData, dirIndex)
		if blocks <= 0 {
			blocks = 1
		}

		blockOffset := slot * psMemoryCardBlockSize
		if blockOffset+psMemoryCardBlockSize > len(imageData) {
			continue
		}
		block := imageData[blockOffset : blockOffset+psMemoryCardBlockSize]
		title := parsePS1MemoryCardEntryTitle(block, productCode)
		region := regionFromProductCode(productCode)
		if region == regionUnknown {
			region = detectRegionCode(title)
		}

		entries = append(entries, memoryCardEntry{
			Title:       title,
			Slot:        slot,
			Blocks:      blocks,
			ProductCode: productCode,
			RegionCode:  normalizeRegionCode(region),
			IconDataURL: parsePS1MemoryCardEntryIcon(block),
		})
	}

	if len(entries) == 0 {
		return &memoryCardDetails{Name: cardName}
	}
	return &memoryCardDetails{Name: cardName, Entries: entries}
}

func isPS1DirectoryStartEntry(state byte) bool {
	return state == 0x51
}

func parsePS1MemoryCardEntryTitle(block []byte, productCode string) string {
	if len(block) >= 68 {
		if title := decodeShiftJISText(block[4:68]); title != "" {
			return title
		}
	}
	if title := extractLongestReadableASCII(block[:minInt(len(block), 256)]); title != "" {
		return title
	}
	if productCode != "" {
		return productCode
	}
	return "Unknown Save"
}

func parsePS1MemoryCardEntryIcon(block []byte) string {
	if len(block) < 0x80+128 {
		return ""
	}
	paletteData := block[0x60:0x80]
	palette := make([]color.NRGBA, 16)
	allTransparent := true
	for i := 0; i < 16; i++ {
		value := binary.LittleEndian.Uint16(paletteData[i*2 : i*2+2])
		palette[i] = decodePSX1555Color(value)
		if palette[i].A != 0 {
			allTransparent = false
		}
	}
	if allTransparent {
		return ""
	}

	frame := block[0x80 : 0x80+128]
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for index, packed := range frame {
		px := (index * 2) % 16
		py := (index * 2) / 16
		colors := [2]uint8{packed & 0x0F, (packed >> 4) & 0x0F}
		for n := 0; n < 2; n++ {
			base := palette[colors[n]]
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					img.SetNRGBA(px*4+n*4+sx, py*4+sy, base)
				}
			}
		}
	}
	return pngDataURLFromImage(img)
}

func decodePSX1555Color(value uint16) color.NRGBA {
	if value == 0 {
		return color.NRGBA{A: 0}
	}
	return color.NRGBA{
		R: expand5BitColor(uint8(value & 0x1F)),
		G: expand5BitColor(uint8((value >> 5) & 0x1F)),
		B: expand5BitColor(uint8((value >> 10) & 0x1F)),
		A: 255,
	}
}

func parsePS2MemoryCard(payload []byte, cardName string) *memoryCardDetails {
	reader, err := newPS2MemoryCardReader(payload)
	if err != nil {
		return &memoryCardDetails{Name: cardName}
	}
	rootEntries, err := reader.readRootDirectoryEntries()
	if err != nil {
		return &memoryCardDetails{Name: cardName}
	}

	entries := make([]memoryCardEntry, 0, len(rootEntries))
	for _, dirEntry := range rootEntries {
		if !ps2ModeIsDir(dirEntry.Mode) {
			continue
		}
		if dirEntry.Name == "." || dirEntry.Name == ".." || strings.TrimSpace(dirEntry.Name) == "" {
			continue
		}

		children, err := reader.readDirectoryEntries(dirEntry.FirstCluster, dirEntry.Length)
		if err != nil {
			continue
		}
		iconSysBytes, ok := reader.readDirectoryFile(children, "icon.sys")
		if !ok || len(iconSysBytes) < 964 {
			continue
		}
		iconSys, err := parsePS2IconSys(iconSysBytes[:964])
		if err != nil {
			continue
		}

		title := combinePS2Title(iconSys.TitleLineOne, iconSys.TitleLineTwo)
		if title == "" {
			title = dirEntry.Name
		}
		if isPS2SystemConfigurationEntry(dirEntry.Name, title) {
			continue
		}

		productCode := derivePS2DirectoryProductCode(dirEntry.Name)
		region := regionFromProductCode(productCode)
		if region == regionUnknown {
			region = detectRegionCode(title)
		}
		sizeBytes := reader.directorySizeBytes(children, dirEntry)
		blocks := 0
		if sizeBytes > 0 {
			blocks = (sizeBytes + reader.clusterSize - 1) / reader.clusterSize
		}

		iconDataURL := ""
		if strings.TrimSpace(iconSys.IconFileNormal) != "" {
			if iconBytes, ok := reader.readDirectoryFile(children, iconSys.IconFileNormal); ok {
				iconDataURL = parsePS2IconTextureDataURL(iconBytes)
			}
		}

		entries = append(entries, memoryCardEntry{
			Title:         title,
			Slot:          len(entries) + 1,
			Blocks:        blocks,
			ProductCode:   productCode,
			RegionCode:    normalizeRegionCode(region),
			DirectoryName: dirEntry.Name,
			IconDataURL:   iconDataURL,
			SizeBytes:     sizeBytes,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Title == entries[j].Title {
			return entries[i].DirectoryName < entries[j].DirectoryName
		}
		return entries[i].Title < entries[j].Title
	})
	for i := range entries {
		entries[i].Slot = i + 1
	}

	if len(entries) == 0 {
		return &memoryCardDetails{Name: cardName}
	}
	return &memoryCardDetails{Name: cardName, Entries: entries}
}

func newPS2MemoryCardReader(payload []byte) (*ps2MemoryCardReader, error) {
	if len(payload) < 0x154 || !isLikelyPS2MemoryCard(payload) {
		return nil, fmt.Errorf("not a PS2 memory card image")
	}

	pageSize := int(binary.LittleEndian.Uint16(payload[40:42]))
	pagesPerCluster := int(binary.LittleEndian.Uint16(payload[42:44]))
	if pageSize <= 0 || pagesPerCluster <= 0 {
		return nil, fmt.Errorf("invalid PS2 memory card geometry")
	}
	spareSize := ((pageSize + 127) / 128) * 4
	rawPageSize := pageSize
	if pageSize > 0 && len(payload)%(pageSize+spareSize) == 0 {
		rawPageSize = pageSize + spareSize
	}

	indirectFatBytes := payload[80:208]
	indirectFAT := make([]uint32, len(indirectFatBytes)/4)
	for i := range indirectFAT {
		indirectFAT[i] = binary.LittleEndian.Uint32(indirectFatBytes[i*4 : i*4+4])
	}

	reader := &ps2MemoryCardReader{
		payload:                  payload,
		pageSize:                 pageSize,
		rawPageSize:              rawPageSize,
		pagesPerCluster:          pagesPerCluster,
		clusterSize:              pageSize * pagesPerCluster,
		allocatableClusterOffset: int(binary.LittleEndian.Uint32(payload[52:56])),
		allocatableClusterEnd:    int(binary.LittleEndian.Uint32(payload[56:60])),
		entriesPerCluster:        (pageSize * pagesPerCluster) / 4,
		indirectFAT:              indirectFAT,
	}
	if reader.clusterSize <= 0 || reader.entriesPerCluster <= 0 {
		return nil, fmt.Errorf("invalid PS2 cluster size")
	}
	return reader, nil
}

func (r *ps2MemoryCardReader) readRootDirectoryEntries() ([]ps2DirectoryEntry, error) {
	cluster, err := r.readAllocatableCluster(0)
	if err != nil {
		return nil, err
	}
	if len(cluster) < ps2DirectoryEntSize {
		return nil, fmt.Errorf("root directory too small")
	}
	dot := parsePS2DirectoryEntry(cluster[:ps2DirectoryEntSize])
	if dot.Name != "." || !ps2ModeIsDir(dot.Mode) {
		return nil, fmt.Errorf("root directory damaged")
	}
	return r.readDirectoryEntries(0, dot.Length)
}

func (r *ps2MemoryCardReader) readDirectoryEntries(firstCluster, length uint32) ([]ps2DirectoryEntry, error) {
	if length == 0 {
		return nil, nil
	}
	data, err := r.readFile(firstCluster, int(length)*ps2DirectoryEntSize)
	if err != nil {
		return nil, err
	}
	entries := make([]ps2DirectoryEntry, 0, len(data)/ps2DirectoryEntSize)
	for offset := 0; offset+ps2DirectoryEntSize <= len(data); offset += ps2DirectoryEntSize {
		entries = append(entries, parsePS2DirectoryEntry(data[offset:offset+ps2DirectoryEntSize]))
	}
	return entries, nil
}

func parsePS2DirectoryEntry(data []byte) ps2DirectoryEntry {
	if len(data) < ps2DirectoryEntSize {
		return ps2DirectoryEntry{}
	}
	return ps2DirectoryEntry{
		Mode:         binary.LittleEndian.Uint16(data[0:2]),
		Length:       binary.LittleEndian.Uint32(data[4:8]),
		FirstCluster: binary.LittleEndian.Uint32(data[16:20]),
		ParentEntry:  binary.LittleEndian.Uint32(data[20:24]),
		Name:         zeroTerminateASCII(data[64:]),
	}
}

func (r *ps2MemoryCardReader) readDirectoryFile(entries []ps2DirectoryEntry, targetName string) ([]byte, bool) {
	for _, entry := range entries {
		if !ps2ModeIsFile(entry.Mode) {
			continue
		}
		if strings.EqualFold(entry.Name, targetName) {
			data, err := r.readFile(entry.FirstCluster, int(entry.Length))
			if err != nil {
				return nil, false
			}
			return data, true
		}
	}
	return nil, false
}

func (r *ps2MemoryCardReader) directorySizeBytes(entries []ps2DirectoryEntry, dirEntry ps2DirectoryEntry) int {
	total := roundUpInt(int(dirEntry.Length)*ps2DirectoryEntSize, r.clusterSize)
	for _, child := range entries {
		switch {
		case ps2ModeIsFile(child.Mode):
			total += roundUpInt(int(child.Length), r.clusterSize)
		case ps2ModeIsDir(child.Mode) && child.Name != "." && child.Name != "..":
			nested, err := r.readDirectoryEntries(child.FirstCluster, child.Length)
			if err == nil {
				total += r.directorySizeBytes(nested, child)
			}
		}
	}
	return total
}

func (r *ps2MemoryCardReader) readFile(firstCluster uint32, length int) ([]byte, error) {
	if length <= 0 {
		return nil, nil
	}
	chain, err := r.fatChain(firstCluster)
	if err != nil {
		return nil, err
	}
	remaining := length
	out := make([]byte, 0, minInt(length, len(chain)*r.clusterSize))
	for _, cluster := range chain {
		buf, err := r.readAllocatableCluster(int(cluster))
		if err != nil {
			return nil, err
		}
		chunk := minInt(remaining, len(buf))
		out = append(out, buf[:chunk]...)
		remaining -= chunk
		if remaining <= 0 {
			break
		}
	}
	if remaining > 0 {
		return nil, fmt.Errorf("unexpected EOF in PS2 memory card file")
	}
	return out, nil
}

func (r *ps2MemoryCardReader) fatChain(first uint32) ([]uint32, error) {
	visited := map[uint32]struct{}{}
	chain := make([]uint32, 0, 4)
	current := first
	for {
		if _, exists := visited[current]; exists {
			return chain, nil
		}
		visited[current] = struct{}{}
		chain = append(chain, current)
		next, err := r.lookupFAT(current)
		if err != nil {
			return nil, err
		}
		if next == ps2FATChainEnd {
			return chain, nil
		}
		if next&ps2FATAllocatedBit == 0 {
			return chain, nil
		}
		current = next &^ ps2FATAllocatedBit
	}
}

func (r *ps2MemoryCardReader) lookupFAT(cluster uint32) (uint32, error) {
	if int(cluster) < 0 || int(cluster) >= r.allocatableClusterEnd {
		return 0, fmt.Errorf("PS2 FAT cluster out of range: %d", cluster)
	}
	fatOffset := cluster % uint32(r.entriesPerCluster)
	fatIndex := cluster / uint32(r.entriesPerCluster)
	fatCluster, err := r.readFATCluster(fatIndex)
	if err != nil {
		return 0, err
	}
	if int(fatOffset) >= len(fatCluster) {
		return 0, fmt.Errorf("PS2 FAT entry out of range")
	}
	return fatCluster[fatOffset], nil
}

func (r *ps2MemoryCardReader) readFATCluster(index uint32) ([]uint32, error) {
	indirectOffset := index % uint32(r.entriesPerCluster)
	indirectIndex := index / uint32(r.entriesPerCluster)
	if int(indirectIndex) >= len(r.indirectFAT) {
		return nil, fmt.Errorf("PS2 indirect FAT index out of range")
	}
	indirectCluster := r.indirectFAT[indirectIndex]
	if indirectCluster == 0 {
		return nil, fmt.Errorf("PS2 indirect FAT cluster missing")
	}
	indirectData, err := r.readCluster(int(indirectCluster))
	if err != nil {
		return nil, err
	}
	indirectTable := uint32SliceFromBytes(indirectData)
	if int(indirectOffset) >= len(indirectTable) {
		return nil, fmt.Errorf("PS2 indirect FAT offset out of range")
	}
	cluster := indirectTable[indirectOffset]
	if cluster == 0 {
		return nil, fmt.Errorf("PS2 FAT cluster missing")
	}
	fatData, err := r.readCluster(int(cluster))
	if err != nil {
		return nil, err
	}
	return uint32SliceFromBytes(fatData), nil
}

func (r *ps2MemoryCardReader) readAllocatableCluster(index int) ([]byte, error) {
	return r.readCluster(index + r.allocatableClusterOffset)
}

func (r *ps2MemoryCardReader) readCluster(index int) ([]byte, error) {
	if index < 0 {
		return nil, fmt.Errorf("negative cluster index")
	}
	if r.rawPageSize == r.pageSize {
		start := index * r.clusterSize
		end := start + r.clusterSize
		if start < 0 || end > len(r.payload) {
			return nil, fmt.Errorf("cluster %d out of range", index)
		}
		return append([]byte(nil), r.payload[start:end]...), nil
	}
	startPage := index * r.pagesPerCluster
	cluster := make([]byte, 0, r.clusterSize)
	for page := 0; page < r.pagesPerCluster; page++ {
		pageData, err := r.readPage(startPage + page)
		if err != nil {
			return nil, err
		}
		cluster = append(cluster, pageData...)
	}
	return cluster, nil
}

func (r *ps2MemoryCardReader) readPage(index int) ([]byte, error) {
	start := index * r.rawPageSize
	end := start + r.pageSize
	if start < 0 || end > len(r.payload) {
		return nil, fmt.Errorf("page %d out of range", index)
	}
	return append([]byte(nil), r.payload[start:end]...), nil
}

func parsePS2IconSys(data []byte) (ps2IconSys, error) {
	if len(data) < 964 || string(data[:4]) != "PS2D" {
		return ps2IconSys{}, fmt.Errorf("invalid icon.sys")
	}
	titleOffset := int(binary.LittleEndian.Uint16(data[6:8]))
	titleRaw := data[192:260]
	if titleOffset < 0 {
		titleOffset = 0
	}
	if titleOffset > len(titleRaw) {
		titleOffset = len(titleRaw)
	}
	return ps2IconSys{
		TitleLineOne:   decodeShiftJISText(titleRaw[:titleOffset]),
		TitleLineTwo:   decodeShiftJISText(titleRaw[titleOffset:]),
		IconFileNormal: zeroTerminateASCII(data[260:324]),
	}, nil
}

func combinePS2Title(lineOne, lineTwo string) string {
	lineOne = strings.TrimSpace(normalizeWideASCII(lineOne))
	lineTwo = strings.TrimSpace(normalizeWideASCII(lineTwo))
	switch {
	case lineOne == "" && lineTwo == "":
		return ""
	case lineOne == "":
		return lineTwo
	case lineTwo == "":
		return lineOne
	case strings.HasSuffix(lineOne, "-") || strings.HasSuffix(lineOne, "/"):
		return strings.TrimSpace(lineOne + lineTwo)
	default:
		return strings.TrimSpace(lineOne + " " + lineTwo)
	}
}

func parsePS2IconTextureDataURL(data []byte) string {
	texture, err := decodePS2IconTexture(data)
	if err != nil || len(texture) != ps2IconTextureSize {
		return ""
	}
	img := image.NewNRGBA(image.Rect(0, 0, ps2IconTextureWidth, ps2IconTextureHeight))
	for y := 0; y < ps2IconTextureHeight; y++ {
		for x := 0; x < ps2IconTextureWidth; x++ {
			offset := (y*ps2IconTextureWidth + x) * 2
			value := binary.LittleEndian.Uint16(texture[offset : offset+2])
			pixel := decodePSX1555Color(value)
			if pixel.A == 0 && value != 0 {
				pixel.A = 255
			}
			img.SetNRGBA(x, y, pixel)
		}
	}
	return pngDataURLFromImage(img)
}

func decodePS2IconTexture(data []byte) ([]byte, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("icon file too small")
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != 0x010000 {
		return nil, fmt.Errorf("invalid PS2 icon magic")
	}
	animationShapes := int(binary.LittleEndian.Uint32(data[4:8]))
	texType := binary.LittleEndian.Uint32(data[8:12])
	vertexCount := int(binary.LittleEndian.Uint32(data[16:20]))
	if animationShapes <= 0 || vertexCount < 0 {
		return nil, fmt.Errorf("invalid PS2 icon geometry")
	}

	offset := 20
	// Each vertex stores N animated positions (8 bytes each), one normal
	// vector (8 bytes), texture coordinates (4 bytes), and RGBA (4 bytes).
	stride := animationShapes*8 + 16
	geometrySize := vertexCount * stride
	if offset+geometrySize > len(data) {
		return nil, fmt.Errorf("truncated PS2 icon geometry")
	}
	offset += geometrySize

	if offset+20 > len(data) {
		return nil, fmt.Errorf("truncated PS2 icon animation header")
	}
	if binary.LittleEndian.Uint32(data[offset:offset+4]) != 0x01 {
		return nil, fmt.Errorf("invalid PS2 icon animation header")
	}
	frameCount := int(binary.LittleEndian.Uint32(data[offset+16 : offset+20]))
	offset += 20
	for i := 0; i < frameCount; i++ {
		if offset+16 > len(data) {
			return nil, fmt.Errorf("truncated PS2 icon frame header")
		}
		keyCount := int(binary.LittleEndian.Uint32(data[offset+4:offset+8])) - 1
		offset += 16
		if keyCount < 0 {
			keyCount = 0
		}
		keyBytes := keyCount * 8
		if offset+keyBytes > len(data) {
			return nil, fmt.Errorf("truncated PS2 icon frame keys")
		}
		offset += keyBytes
	}

	if offset >= len(data) {
		return nil, fmt.Errorf("PS2 icon has no texture")
	}
	if texType&0x4 == 0 {
		return nil, fmt.Errorf("PS2 icon has no texture segment")
	}
	if texType&0x8 == 0 {
		if offset+ps2IconTextureSize > len(data) {
			return nil, fmt.Errorf("truncated PS2 icon texture")
		}
		return append([]byte(nil), data[offset:offset+ps2IconTextureSize]...), nil
	}
	if offset+4 > len(data) {
		return nil, fmt.Errorf("truncated PS2 icon compressed texture header")
	}
	compressedSize := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if compressedSize < 0 || offset+compressedSize > len(data) {
		return nil, fmt.Errorf("truncated PS2 icon compressed texture")
	}
	return decompressPS2IconTexture(data[offset : offset+compressedSize])
}

func decompressPS2IconTexture(data []byte) ([]byte, error) {
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("odd PS2 texture payload")
	}
	texture := make([]byte, 0, ps2IconTextureSize)
	for offset := 0; offset < len(data); {
		if offset+2 > len(data) {
			return nil, fmt.Errorf("truncated PS2 texture RLE code")
		}
		code := binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2
		switch {
		case code&0xFF00 == 0xFF00:
			rawBytes := int(0x10000-uint32(code)) * 2
			if offset+rawBytes > len(data) {
				return nil, fmt.Errorf("truncated PS2 texture raw payload")
			}
			texture = append(texture, data[offset:offset+rawBytes]...)
			offset += rawBytes
		default:
			if offset+2 > len(data) {
				return nil, fmt.Errorf("truncated PS2 texture repeat payload")
			}
			pair := data[offset : offset+2]
			offset += 2
			for i := 0; i < int(code); i++ {
				texture = append(texture, pair...)
			}
		}
		if len(texture) > ps2IconTextureSize {
			return nil, fmt.Errorf("PS2 texture decompressed beyond limit")
		}
	}
	if len(texture) != ps2IconTextureSize {
		return nil, fmt.Errorf("unexpected PS2 texture size: %d", len(texture))
	}
	return texture, nil
}

func derivePS2DirectoryProductCode(raw string) string {
	match := ps2DirectoryCodePattern.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(raw)))
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func isPS2SystemConfigurationEntry(directoryName, title string) bool {
	name := strings.ToUpper(strings.TrimSpace(directoryName))
	combined := strings.ToLower(strings.TrimSpace(title + " " + directoryName))
	if strings.Contains(name, "DATA-SYSTEM") {
		return true
	}
	return strings.Contains(combined, "system configuration")
}

func decodeShiftJISText(data []byte) string {
	trimmed := bytes.Trim(data, "\x00")
	if len(trimmed) == 0 {
		return ""
	}
	decoded, _, err := transform.String(japanese.ShiftJIS.NewDecoder(), string(trimmed))
	if err != nil && decoded == "" {
		return strings.TrimSpace(extractPrintableASCII(trimmed))
	}
	decoded = strings.ReplaceAll(decoded, "\x00", "")
	decoded = normalizeWideASCII(decoded)
	decoded = spacePattern.ReplaceAllString(decoded, " ")
	return strings.TrimSpace(decoded)
}

func normalizeWideASCII(raw string) string {
	var out strings.Builder
	for _, r := range raw {
		switch {
		case r == 0x3000:
			out.WriteByte(' ')
		case r >= 0xFF01 && r <= 0xFF5E:
			out.WriteRune(r - 0xFEE0)
		case r == 0x30FC:
			out.WriteByte('-')
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

func pngDataURLFromImage(img image.Image) string {
	if img == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func expand5BitColor(value uint8) uint8 {
	return (value << 3) | (value >> 2)
}

func uint32SliceFromBytes(data []byte) []uint32 {
	count := len(data) / 4
	out := make([]uint32, count)
	for i := 0; i < count; i++ {
		out[i] = binary.LittleEndian.Uint32(data[i*4 : i*4+4])
	}
	return out
}

func zeroTerminateASCII(data []byte) string {
	if idx := bytes.IndexByte(data, 0); idx >= 0 {
		data = data[:idx]
	}
	return strings.TrimSpace(string(data))
}

func ps2ModeIsFile(mode uint16) bool {
	return mode&(0x0010|0x0020|0x8000) == (0x0010 | 0x8000)
}

func ps2ModeIsDir(mode uint16) bool {
	return mode&(0x0010|0x0020|0x8000) == (0x0020 | 0x8000)
}

func roundUpInt(value, alignment int) int {
	if alignment <= 0 {
		return value
	}
	mod := value % alignment
	if mod == 0 {
		return value
	}
	return value + alignment - mod
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
