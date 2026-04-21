package main

import (
	"encoding/binary"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	regionUS      = "US"
	regionEU      = "EU"
	regionJP      = "JP"
	regionUnknown = "UNKNOWN"
)

const (
	psMemoryCardBlockSize = 8192
	psDirectoryEntrySize  = 128
	psDirectoryEntries    = 15
)

var (
	trailingTagPattern = regexp.MustCompile(`\s*\(([^)]*)\)\s*$`)
	trailingCounterPattern = regexp.MustCompile(`(?i)(?:[_\.-]+|\s+(?:slot|save)\s+)#?([0-9]{1,3})$`)
	cardNumberPattern  = regexp.MustCompile(`(?i)(memory[\s_-]*card|card)[\s_-]*([0-9]{1,2})`)
	spacePattern       = regexp.MustCompile(`\s+`)
	productCodePattern = regexp.MustCompile(`\b([A-Z]{4}[-_][0-9]{3}\.[0-9]{2}|[A-Z]{4}[0-9]{5})\b`)
)

type normalizedSaveMetadata struct {
	DisplayTitle   string
	RegionCode     string
	RegionFlag     string
	LanguageCodes  []string
	System         *system
	SystemPath     string
	GamePath       string
	IsPSMemoryCard bool
	MemoryCard     *memoryCardDetails
}

func deriveNormalizedSaveMetadata(input saveCreateInput, filename string) normalizedSaveMetadata {
	originalTitle := strings.TrimSpace(input.Game.Name)
	if originalTitle == "" {
		originalTitle = strings.TrimSpace(strings.TrimSuffix(filename, filepath.Ext(filename)))
	}
	displayTitle, regionCode, languageCodes := cleanupDisplayTitleRegionAndLanguages(originalTitle)
	if displayTitle == "" {
		displayTitle = "Unknown Game"
	}
	_, filenameRegion, filenameLanguages := cleanupDisplayTitleRegionAndLanguages(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if normalizeRegionCode(regionCode) == regionUnknown {
		regionCode = filenameRegion
	}
	languageCodes = normalizeLanguageCodes(append(languageCodes, filenameLanguages...))

	sys := normalizeSystemForSave(input.Game.System, input.SystemSlug, filename, input.Format, input.Payload, originalTitle)
	regionCode = normalizeRegionCode(regionCode)
	if input.RegionCode != "" {
		regionCode = normalizeRegionCode(input.RegionCode)
	}
	if regionCode == regionUnknown {
		productCode := deriveProductCodeFromPayload(input.Payload)
		if detected := normalizeRegionCode(regionFromProductCode(productCode)); detected != regionUnknown {
			regionCode = detected
		}
	}
	if len(input.LanguageCodes) > 0 {
		languageCodes = normalizeLanguageCodes(append(input.LanguageCodes, languageCodes...))
	}

	isPS := isPlayStationSave(sys, input.Format, filename)
	cardName := ""
	var memoryCard *memoryCardDetails
	gamePath := displayTitle
	if isPS {
		cardName = deriveMemoryCardName(input.SlotName, filename)
		displayTitle = cardName
		gamePath = cardName
		memoryCard = parsePlayStationMemoryCard(input.Payload, cardName)
		if memoryCard == nil {
			memoryCard = &memoryCardDetails{Name: cardName}
		}
		if regionCode == regionUnknown {
			for _, entry := range memoryCard.Entries {
				if normalizeRegionCode(entry.RegionCode) != regionUnknown {
					regionCode = normalizeRegionCode(entry.RegionCode)
					break
				}
			}
		}
	}

	return normalizedSaveMetadata{
		DisplayTitle:   displayTitle,
		RegionCode:     regionCode,
		RegionFlag:     regionFlagFromCode(regionCode),
		LanguageCodes:  languageCodes,
		System:         sys,
		SystemPath:     sanitizeDisplayPathSegment(sys.Name, "Unknown System"),
		GamePath:       sanitizeDisplayPathSegment(gamePath, "Unknown Game"),
		IsPSMemoryCard: isPS,
		MemoryCard:     memoryCard,
	}
}

func cleanupDisplayTitleAndRegion(raw string) (string, string) {
	title, region, _ := cleanupDisplayTitleRegionAndLanguages(raw)
	return title, region
}

func cleanupDisplayTitleRegionAndLanguages(raw string) (string, string, []string) {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "Unknown Game", regionUnknown, nil
	}

	detectedRegion := detectRegionCode(raw)
	detectedLanguages := make([]string, 0, 4)
	for {
		match := trailingTagPattern.FindStringSubmatchIndex(title)
		if match == nil {
			break
		}
		tag := title[match[2]:match[3]]
		if !looksLikeRomMetadataTag(tag) {
			break
		}
		if detectedRegion == regionUnknown {
			detectedRegion = detectRegionCode(tag)
		}
		detectedLanguages = append(detectedLanguages, extractLanguageCodes(tag)...)
		title = strings.TrimSpace(title[:match[0]])
	}

	title = strings.TrimSpace(strings.Trim(title, "-_"))
	for {
		match := trailingCounterPattern.FindStringSubmatchIndex(title)
		if match == nil {
			break
		}
		title = strings.TrimSpace(title[:match[0]])
	}
	title = strings.TrimSpace(strings.Trim(title, "-_"))
	if title == "" {
		title = "Unknown Game"
	}
	return title, normalizeRegionCode(detectedRegion), normalizeLanguageCodes(detectedLanguages)
}

func looksLikeRomMetadataTag(tag string) bool {
	compact := strings.ToLower(strings.TrimSpace(tag))
	if compact == "" {
		return false
	}

	regionHints := []string{
		"usa", "us", "u.s.a", "europe", "eu", "japan", "jp", "pal", "ntsc",
		"world", "australia", "france", "germany", "spain", "italy", "korea",
	}
	for _, hint := range regionHints {
		if strings.Contains(compact, hint) {
			return true
		}
	}

	metaHints := []string{"rev", "proto", "beta", "demo", "sample", "v", "en", "fr", "de", "es", "it", "ja"}
	for _, hint := range metaHints {
		if strings.Contains(compact, hint) {
			return true
		}
	}

	if strings.ContainsAny(compact, ",+") {
		return true
	}
	for _, r := range compact {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return len(compact) <= 4
}

func detectRegionCode(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return regionUnknown
	}

	usHints := []string{"usa", "u.s.a", "north america", "ntsc-u", "ntsc u", "(us)", " us "}
	euHints := []string{"europe", "pal", "eur", "(eu)", " eu "}
	jpHints := []string{"japan", "ntsc-j", "ntsc j", "(jp)", " jp "}

	for _, hint := range usHints {
		if strings.Contains(value, hint) {
			return regionUS
		}
	}
	for _, hint := range euHints {
		if strings.Contains(value, hint) {
			return regionEU
		}
	}
	for _, hint := range jpHints {
		if strings.Contains(value, hint) {
			return regionJP
		}
	}
	return regionUnknown
}

func normalizeRegionCode(code string) string {
	clean := strings.ToUpper(strings.TrimSpace(code))
	switch clean {
	case "US", "USA":
		return regionUS
	case "EU", "EUR":
		return regionEU
	case "JP", "JPN":
		return regionJP
	default:
		return regionUnknown
	}
}

func normalizeLanguageCode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "en", "eng", "english":
		return "EN"
	case "ja", "jp", "jpn", "jap", "japanese":
		return "JA"
	case "fr", "fre", "fra", "french":
		return "FR"
	case "de", "ger", "deu", "german":
		return "DE"
	case "es", "spa", "spanish":
		return "ES"
	case "it", "ita", "italian":
		return "IT"
	case "pt", "por", "portuguese", "pt-br", "ptbr":
		return "PT"
	case "nl", "dut", "nld", "dutch":
		return "NL"
	case "ko", "kor", "korean":
		return "KO"
	case "zh", "chi", "zho", "chinese":
		return "ZH"
	case "ru", "rus", "russian":
		return "RU"
	default:
		return ""
	}
}

func normalizeLanguageCodes(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		normalized := normalizeLanguageCode(entry)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func extractLanguageCodes(tag string) []string {
	clean := strings.TrimSpace(strings.ToLower(tag))
	if clean == "" {
		return nil
	}
	normalized := strings.NewReplacer(",", " ", "/", " ", "+", " ", ";", " ", "_", " ", "-", " ").Replace(clean)
	parts := strings.Fields(normalized)
	langCodes := make([]string, 0, len(parts))
	for _, part := range parts {
		if code := normalizeLanguageCode(part); code != "" {
			langCodes = append(langCodes, code)
		}
	}
	return normalizeLanguageCodes(langCodes)
}

func regionFlagFromCode(code string) string {
	switch normalizeRegionCode(code) {
	case regionUS:
		return "us"
	case regionEU:
		return "eu"
	case regionJP:
		return "jp"
	default:
		return "unknown"
	}
}

func normalizeSystemForSave(existing *system, fallbackSlug, filename, format string, payload []byte, displayTitle string) *system {
	detection := detectSaveSystem(saveSystemDetectionInput{
		Filename:           filename,
		DisplayTitle:       displayTitle,
		Payload:            payload,
		DeclaredSystemSlug: fallbackSlug,
		DeclaredSystem:     existing,
	})
	if detection.System != nil {
		return detection.System
	}

	if existing != nil && strings.TrimSpace(existing.Name) != "" {
		existingSlug := canonicalSegment(firstNonEmpty(existing.Slug, existing.Name), "unknown-system")
		existingID := existing.ID
		if existingID == 0 {
			existingID = deterministicSystemID(existing.Name)
		}
		return &system{
			ID:           existingID,
			Name:         strings.TrimSpace(existing.Name),
			Slug:         existingSlug,
			Manufacturer: manufacturerForSystem(existingSlug, existing.Name),
		}
	}

	display := strings.TrimSpace(fallbackSlug)
	if display == "" {
		display = "Unknown System"
	}
	slug := canonicalSegment(display, "unknown-system")
	return &system{
		ID:           deterministicSystemID(display),
		Name:         toDisplayWords(display),
		Slug:         slug,
		Manufacturer: manufacturerForSystem(slug, display),
	}
}

func deterministicSystemID(name string) int {
	return deterministicGameID("system:" + strings.TrimSpace(name))
}

func toDisplayWords(raw string) string {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return ""
	}
	clean = strings.ReplaceAll(clean, "-", " ")
	clean = strings.ReplaceAll(clean, "_", " ")
	clean = spacePattern.ReplaceAllString(clean, " ")
	parts := strings.Split(clean, " ")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if len(part) <= 2 {
			parts[i] = strings.ToUpper(part)
		} else {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}

func sanitizeDisplayPathSegment(value, fallback string) string {
	clean := strings.TrimSpace(value)
	if clean == "" {
		clean = fallback
	}
	if clean == "" {
		clean = "Unknown"
	}
	clean = strings.ReplaceAll(clean, "/", "-")
	clean = strings.ReplaceAll(clean, `\`, "-")

	var b strings.Builder
	for _, r := range clean {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
			continue
		}
		switch r {
		case '-', '_', '.', '(', ')', '&', '\'', '+':
			b.WriteRune(r)
		}
	}
	clean = strings.TrimSpace(spacePattern.ReplaceAllString(b.String(), " "))
	clean = strings.Trim(clean, ".")
	if clean == "" {
		clean = fallback
	}
	if clean == "" {
		clean = "Unknown"
	}
	return clean
}

func isPlayStationSave(sys *system, format, filename string) bool {
	if sys != nil {
		slug := strings.ToLower(strings.TrimSpace(sys.Slug))
		name := strings.ToLower(strings.TrimSpace(sys.Name))
		if strings.Contains(slug, "psx") || strings.Contains(slug, "ps1") || strings.Contains(slug, "playstation") {
			return true
		}
		if strings.Contains(name, "playstation") {
			return true
		}
	}

	if strings.Contains(strings.ToLower(strings.TrimSpace(format)), "mcr") {
		return true
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	return isPlayStationExt(ext)
}

func isPlayStationExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "mcr", "mcd", "gme", "mc":
		return true
	default:
		return false
	}
}

func deriveMemoryCardName(slotName, filename string) string {
	candidates := []string{slotName, filename}
	for _, candidate := range candidates {
		match := cardNumberPattern.FindStringSubmatch(candidate)
		if len(match) >= 3 {
			index, err := strconv.Atoi(match[2])
			if err == nil && index > 0 {
				return "Memory Card " + strconv.Itoa(index)
			}
		}
	}
	return "Memory Card 1"
}

func parsePlayStationMemoryCard(payload []byte, cardName string) *memoryCardDetails {
	if len(payload) < psMemoryCardBlockSize {
		return nil
	}

	entries := make([]memoryCardEntry, 0, 8)
	for dirIndex := 1; dirIndex <= psDirectoryEntries; dirIndex++ {
		offset := dirIndex * psDirectoryEntrySize
		if offset+psDirectoryEntrySize > len(payload) {
			break
		}
		entry := payload[offset : offset+psDirectoryEntrySize]
		state := entry[0]
		productCode := extractPrintableASCII(entry[0x0a:0x16])
		if !isLikelyUsedDirectoryEntry(state, productCode) {
			continue
		}

		slot := dirIndex
		blocks := countDirectoryBlocks(payload, dirIndex)
		if blocks <= 0 {
			blocks = 1
		}
		title := parseMemoryCardEntryTitle(payload, slot, productCode)
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
		})
	}

	if len(entries) == 0 {
		return &memoryCardDetails{Name: cardName}
	}
	return &memoryCardDetails{Name: cardName, Entries: entries}
}

func isLikelyUsedDirectoryEntry(state byte, productCode string) bool {
	if strings.TrimSpace(productCode) != "" {
		return true
	}
	switch state {
	case 0x51, 0x52, 0x53, 0xA1, 0xA2, 0xA3:
		return true
	default:
		return false
	}
}

func countDirectoryBlocks(payload []byte, start int) int {
	visited := map[int]struct{}{}
	count := 0
	current := start
	for current >= 1 && current <= psDirectoryEntries {
		if _, exists := visited[current]; exists {
			break
		}
		visited[current] = struct{}{}
		count++

		offset := current * psDirectoryEntrySize
		if offset+10 > len(payload) {
			break
		}
		next := int(binary.LittleEndian.Uint16(payload[offset+8 : offset+10]))
		if next == 0xFFFF || next == 0 {
			break
		}
		current = next
	}
	return count
}

func parseMemoryCardEntryTitle(payload []byte, slot int, productCode string) string {
	offset := slot * psMemoryCardBlockSize
	if offset >= len(payload) {
		if productCode != "" {
			return productCode
		}
		return "Unknown Save"
	}

	end := offset + 256
	if end > len(payload) {
		end = len(payload)
	}
	header := payload[offset:end]
	title := extractLongestReadableASCII(header)
	if title == "" {
		if productCode != "" {
			return productCode
		}
		return "Save Slot " + strconv.Itoa(slot)
	}
	return title
}

func extractLongestReadableASCII(data []byte) string {
	best := ""
	current := strings.Builder{}
	flush := func() {
		candidate := strings.TrimSpace(current.String())
		if len(candidate) > len(best) {
			best = candidate
		}
		current.Reset()
	}

	for _, b := range data {
		if b >= 32 && b <= 126 {
			current.WriteByte(b)
			continue
		}
		flush()
	}
	flush()

	if len(best) > 64 {
		best = best[:64]
	}
	return strings.TrimSpace(best)
}

func extractPrintableASCII(data []byte) string {
	var b strings.Builder
	for _, c := range data {
		if c >= 32 && c <= 126 {
			b.WriteByte(c)
		}
	}
	out := strings.TrimSpace(b.String())
	out = strings.Trim(out, "\x00")
	return out
}

func regionFromProductCode(code string) string {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "_", ""))
	if normalized == "" {
		return regionUnknown
	}
	switch {
	case strings.HasPrefix(normalized, "SCUS"), strings.HasPrefix(normalized, "SLUS"):
		return regionUS
	case strings.HasPrefix(normalized, "SLES"), strings.HasPrefix(normalized, "SCES"), strings.HasPrefix(normalized, "SLED"), strings.HasPrefix(normalized, "SCED"):
		return regionEU
	case strings.HasPrefix(normalized, "SLPS"), strings.HasPrefix(normalized, "SCPS"), strings.HasPrefix(normalized, "SLPM"), strings.HasPrefix(normalized, "SCPM"), strings.HasPrefix(normalized, "PAPX"):
		return regionJP
	default:
		return regionUnknown
	}
}

func deriveProductCodeFromPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	text := strings.ToUpper(extractPrintableASCII(payload))
	match := productCodePattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
