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
	trailingTagPattern     = regexp.MustCompile(`\s*\(([^)]*)\)\s*$`)
	trailingCounterPattern = regexp.MustCompile(`(?i)(?:[_\.-]+|\s+(?:slot|save)\s+)#?([0-9]{1,3})$`)
	cardNumberPattern      = regexp.MustCompile(`(?i)(memory[\s_-]*card|card)[\s_-]*([0-9]{1,2})`)
	mcdNumberPattern       = regexp.MustCompile(`(?i)mcd0*([0-9]{1,2})`)
	spacePattern           = regexp.MustCompile(`\s+`)
	productCodePattern     = regexp.MustCompile(`\b([A-Z]{4}[-_][0-9]{3}\.[0-9]{2}|[A-Z]{4}[0-9]{5})\b`)
)

var saveTitleAliasesBySystem = map[string]map[string]string{
	"neogeo": {
		"doubledr": "Double Dragon",
	},
}

type normalizedSaveMetadata struct {
	DisplayTitle   string
	RegionCode     string
	RegionFlag     string
	LanguageCodes  []string
	System         *system
	SystemPath     string
	GamePath       string
	CoverArtURL    string
	Metadata       any
	ArtifactKind   saveArtifactKind
	IsPSMemoryCard bool
	MemoryCard     *memoryCardDetails
	Dreamcast      *dreamcastDetails
	Saturn         *saturnDetails
}

func deriveNormalizedSaveMetadata(input saveCreateInput, filename string, detection saveSystemDetectionResult) normalizedSaveMetadata {
	originalTitle := strings.TrimSpace(input.Game.Name)
	if originalTitle == "" {
		originalTitle = strings.TrimSpace(strings.TrimSuffix(filename, filepath.Ext(filename)))
	}
	displayTitle, regionCode, languageCodes := cleanupDisplayTitleRegionAndLanguages(originalTitle)
	displayTitle = resolveKnownSaveTitleAlias(detection.Slug, displayTitle)
	if displayTitle == "" {
		displayTitle = "Unknown Game"
	}
	_, filenameRegion, filenameLanguages := cleanupDisplayTitleRegionAndLanguages(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if normalizeRegionCode(regionCode) == regionUnknown {
		regionCode = filenameRegion
	}
	languageCodes = normalizeLanguageCodes(append(languageCodes, filenameLanguages...))

	sys := detection.System
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

	artifactKind := classifyPlayStationArtifact(sys, input.Format, filename, input.Payload)
	isPS := artifactKind == saveArtifactPS1MemoryCard || artifactKind == saveArtifactPS2MemoryCard
	cardName := ""
	var memoryCard *memoryCardDetails
	var dreamcast *dreamcastDetails
	var saturn *saturnDetails
	coverArtURL := strings.TrimSpace(input.CoverArtURL)
	metadata := input.Metadata
	gamePath := displayTitle
	if isPS {
		cardName = canonicalMemoryCardName(input.MemoryCard, input.SlotName, filename)
		displayTitle = cardName
		gamePath = cardName
		memoryCard = parsePlayStationMemoryCard(sys, input.Payload, filename, cardName)
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
	if !isPS && sys != nil && supportedSystemSlugFromLabel(firstNonEmpty(sys.Slug, sys.Name)) == "dreamcast" {
		dreamcast = parseDreamcastContainer(filename, input.Payload)
		if dreamcast != nil {
			dreamcast.SlotName = normalizeDreamcastSlotName(input.SlotName, filename)
			metadata = mergeRSMMetadata(metadata, "dreamcast", dreamcast)
			if dreamcast.SaveEntries == 1 {
				if title := strings.TrimSpace(dreamcast.SampleTitle); title != "" {
					displayTitle = title
					gamePath = title
				}
			}
			if coverArtURL == "" {
				coverArtURL = strings.TrimSpace(firstNonEmpty(dreamcast.SampleIconDataURL, dreamcast.SampleEyecatchDataURL))
			}
		}
	}
	if !isPS && sys != nil && supportedSystemSlugFromLabel(firstNonEmpty(sys.Slug, sys.Name)) == "saturn" {
		parsedSaturn := parseSaturnContainer(filename, input.Payload)
		if parsedSaturn != nil {
			saturn = parsedSaturn.Details
			metadata = mergeRSMMetadata(metadata, "saturn", saturn)
		}
	}

	return normalizedSaveMetadata{
		DisplayTitle:  displayTitle,
		RegionCode:    regionCode,
		RegionFlag:    regionFlagFromCode(regionCode),
		LanguageCodes: languageCodes,
		System:        sys,
		SystemPath: sanitizeDisplayPathSegment(func() string {
			if sys != nil {
				return sys.Name
			}
			return "Unknown System"
		}(), "Unknown System"),
		CoverArtURL:    coverArtURL,
		Metadata:       metadata,
		ArtifactKind:   artifactKind,
		GamePath:       sanitizeDisplayPathSegment(gamePath, "Unknown Game"),
		IsPSMemoryCard: isPS,
		MemoryCard:     memoryCard,
		Dreamcast:      dreamcast,
		Saturn:         saturn,
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
	stripTrailingTags := func() {
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
	}

	stripTrailingTags()

	title = strings.TrimSpace(strings.Trim(title, "-_"))
	for {
		match := trailingCounterPattern.FindStringSubmatchIndex(title)
		if match == nil {
			break
		}
		title = strings.TrimSpace(title[:match[0]])
	}
	// Removing trailing counters can expose a metadata tag such as "(USA)_1".
	stripTrailingTags()
	title = strings.TrimSpace(strings.Trim(title, "-_"))
	if title == "" {
		title = "Unknown Game"
	}
	return title, normalizeRegionCode(detectedRegion), normalizeLanguageCodes(detectedLanguages)
}

func resolveKnownSaveTitleAlias(systemSlug, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return title
	}

	systemSlug = canonicalSegment(systemSlug, "")
	if aliases, ok := saveTitleAliasesBySystem[systemSlug]; ok {
		if resolved, ok := aliases[canonicalSegment(title, "")]; ok && strings.TrimSpace(resolved) != "" {
			return strings.TrimSpace(resolved)
		}
	}
	return title
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

func isPS1System(sys *system) bool {
	if sys != nil {
		slug := supportedSystemSlugFromLabel(firstNonEmpty(sys.Slug, sys.Name))
		if slug == "psx" {
			return true
		}
	}
	return false
}

func isConfirmedPS1MemoryCard(sys *system, format, filename string, payload []byte) bool {
	if sys != nil && !isPS1System(sys) {
		return false
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if !isPlayStationExt(ext) {
		if strings.Contains(strings.ToLower(strings.TrimSpace(format)), "mcr") {
			ext = "mcr"
		}
	}
	return isLikelyPS1MemoryCard(payload, ext)
}

func isPlayStationExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "mcr", "mcd", "gme", "mc", "vmp", "psv":
		return true
	default:
		return false
	}
}

func deriveMemoryCardName(slotName, filename string) string {
	candidates := []string{slotName, filename}
	for _, candidate := range candidates {
		if match := mcdNumberPattern.FindStringSubmatch(candidate); len(match) >= 2 {
			index, err := strconv.Atoi(match[1])
			if err == nil && index > 0 {
				return "Memory Card " + strconv.Itoa(index)
			}
		}
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

func normalizeDreamcastSlotName(slotName, filename string) string {
	candidates := []string{slotName, filename}
	for _, candidate := range candidates {
		upper := strings.ToUpper(strings.TrimSpace(candidate))
		if upper == "" {
			continue
		}
		for _, bank := range []string{"A", "B", "C", "D"} {
			for _, slot := range []string{"1", "2", "3", "4"} {
				needle := bank + slot
				if strings.Contains(upper, needle) {
					return needle
				}
			}
		}
	}
	return ""
}

func normalizedPS1MemoryCardImage(payload []byte, ext string) []byte {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "gme":
		if len(payload) == ps1MemoryCardTotalSize+ps1DexDriveHeaderSize {
			return payload[ps1DexDriveHeaderSize:]
		}
	case "vmp":
		if len(payload) == ps1MemoryCardTotalSize+ps1PSPVMPHeaderSize && len(payload) > ps1PSPVMPHeaderSize {
			return payload[ps1PSPVMPHeaderSize:]
		}
	case "mcr", "mcd", "mc", "psv":
		if len(payload) == ps1MemoryCardTotalSize {
			return payload
		}
	default:
		if len(payload) == ps1MemoryCardTotalSize {
			return payload
		}
	}
	return nil
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
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(code), "_", ""), "-", ""))
	if normalized == "" {
		return regionUnknown
	}
	switch {
	case strings.HasPrefix(normalized, "SCUS"), strings.HasPrefix(normalized, "SLUS"), strings.HasPrefix(normalized, "BASLUS"), strings.HasPrefix(normalized, "BASCUS"):
		return regionUS
	case strings.HasPrefix(normalized, "SLES"), strings.HasPrefix(normalized, "SCES"), strings.HasPrefix(normalized, "SLED"), strings.HasPrefix(normalized, "SCED"), strings.HasPrefix(normalized, "BESLES"), strings.HasPrefix(normalized, "BESCES"):
		return regionEU
	case strings.HasPrefix(normalized, "SLPS"), strings.HasPrefix(normalized, "SCPS"), strings.HasPrefix(normalized, "SLPM"), strings.HasPrefix(normalized, "SCPM"), strings.HasPrefix(normalized, "PAPX"), strings.HasPrefix(normalized, "BISLPS"), strings.HasPrefix(normalized, "BASLPM"):
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
