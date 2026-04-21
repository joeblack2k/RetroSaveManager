package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	ps1MemoryCardTotalSize   = 128 * 1024
	ps1DexDriveHeaderSize    = 3904
	ps1PSPVMPHeaderSize      = 0x80
	ps2MemoryCardHeaderMagic = "Sony PS2 Memory Card Format "
	neoGeoSaveRAMSize        = 0x10000
	neoGeoCardDataSize       = 0x2000
	neoGeoCompoundSaveSize   = neoGeoSaveRAMSize + neoGeoCardDataSize
)

var errUnsupportedSaveFormat = errors.New("unsupported or unrecognized save format; only known consoles/arcade are allowed")

var supportedSystemsBySlug = map[string]system{
	"arcade":        {ID: 900001, Name: "Arcade", Slug: "arcade", Manufacturer: "Arcade"},
	"game-gear":     {ID: 900002, Name: "Game Gear", Slug: "game-gear", Manufacturer: "Sega"},
	"gameboy":       {ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy", Manufacturer: "Nintendo"},
	"gba":           {ID: 24, Name: "Game Boy Advance", Slug: "gba", Manufacturer: "Nintendo"},
	"genesis":       {ID: 33, Name: "Sega Genesis/Mega Drive", Slug: "genesis", Manufacturer: "Sega"},
	"master-system": {ID: 900003, Name: "Master System", Slug: "master-system", Manufacturer: "Sega"},
	"n64":           {ID: 64, Name: "Nintendo 64", Slug: "n64", Manufacturer: "Nintendo"},
	"nds":           {ID: 900004, Name: "Nintendo DS", Slug: "nds", Manufacturer: "Nintendo"},
	"neogeo":        {ID: 900005, Name: "Neo Geo", Slug: "neogeo", Manufacturer: "SNK"},
	"nes":           {ID: 900006, Name: "Nintendo Entertainment System", Slug: "nes", Manufacturer: "Nintendo"},
	"ps2":           {ID: 900007, Name: "PlayStation 2", Slug: "ps2", Manufacturer: "Sony"},
	"ps3":           {ID: 900008, Name: "PlayStation 3", Slug: "ps3", Manufacturer: "Sony"},
	"ps4":           {ID: 900009, Name: "PlayStation 4", Slug: "ps4", Manufacturer: "Sony"},
	"ps5":           {ID: 900010, Name: "PlayStation 5", Slug: "ps5", Manufacturer: "Sony"},
	"psp":           {ID: 900011, Name: "PlayStation Portable", Slug: "psp", Manufacturer: "Sony"},
	"psvita":        {ID: 900012, Name: "PlayStation Vita", Slug: "psvita", Manufacturer: "Sony"},
	"psx":           {ID: 27, Name: "PlayStation", Slug: "psx", Manufacturer: "Sony"},
	"snes":          {ID: 26, Name: "Nintendo Super Nintendo Entertainment System", Slug: "snes", Manufacturer: "Nintendo"},
}

var systemLabelAliases = map[string]string{
	"arcade":               "arcade",
	"fbneo":                "arcade",
	"finalburn":            "arcade",
	"mame":                 "arcade",
	"gameboy":              "gameboy",
	"game boy":             "gameboy",
	"gb":                   "gameboy",
	"gameboyadvance":       "gba",
	"game boy advance":     "gba",
	"gba":                  "gba",
	"genesis":              "genesis",
	"mega drive":           "genesis",
	"megadrive":            "genesis",
	"mastersystem":         "master-system",
	"master system":        "master-system",
	"sms":                  "master-system",
	"game gear":            "game-gear",
	"n64":                  "n64",
	"nintendo64":           "n64",
	"nintendo ds":          "nds",
	"nds":                  "nds",
	"ds":                   "nds",
	"neogeo":               "neogeo",
	"neo geo":              "neogeo",
	"neo-geo":              "neogeo",
	"nes":                  "nes",
	"famicom":              "nes",
	"psx":                  "psx",
	"ps1":                  "psx",
	"playstation":          "psx",
	"playstation 1":        "psx",
	"playstation2":         "ps2",
	"playstation 2":        "ps2",
	"ps2":                  "ps2",
	"playstation3":         "ps3",
	"playstation 3":        "ps3",
	"ps3":                  "ps3",
	"playstation4":         "ps4",
	"playstation 4":        "ps4",
	"ps4":                  "ps4",
	"playstation5":         "ps5",
	"playstation 5":        "ps5",
	"ps5":                  "ps5",
	"psp":                  "psp",
	"playstation portable": "psp",
	"psvita":               "psvita",
	"ps vita":              "psvita",
	"vita":                 "psvita",
	"snes":                 "snes",
	"super nintendo":       "snes",
	"sfc":                  "snes",
}

var arcadeFilenameHints = []string{
	"mame", "fbneo", "finalburn", "model2", "naomi",
}

type saveSystemDetectionInput struct {
	Filename             string
	DisplayTitle         string
	Payload              []byte
	DeclaredSystemSlug   string
	DeclaredSystem       *system
	DeclaredFallbackOnly bool
}

type saveSystemDetectionResult struct {
	Slug       string
	System     *system
	Confidence int
	Reason     string
	Noise      bool
}

var exactTitleSystemHints = map[string]string{
	"new super mario bros": "nds",
}

var strictNeoGeoSetNames = map[string]struct{}{
	"doubledr": {},
}

var numberedSaveSlotTitlePattern = regexp.MustCompile(`^\s*[0-9]{1,3}\s*-\s*[A-Za-z]`)

func allSupportedSystems() []system {
	out := make([]system, 0, len(supportedSystemsBySlug))
	for _, candidate := range supportedSystemsBySlug {
		out = append(out, candidate)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Manufacturer == out[j].Manufacturer {
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		}
		return strings.ToLower(out[i].Manufacturer) < strings.ToLower(out[j].Manufacturer)
	})
	return out
}

func isSupportedSystemSlug(raw string) bool {
	slug := canonicalSegment(raw, "")
	_, ok := supportedSystemsBySlug[slug]
	return ok
}

func supportedSystemFromSlug(raw string) *system {
	slug := canonicalSegment(raw, "")
	candidate, ok := supportedSystemsBySlug[slug]
	if !ok {
		return nil
	}
	copyCandidate := candidate
	return &copyCandidate
}

func supportedSystemSlugFromLabel(raw string) string {
	label := strings.ToLower(strings.TrimSpace(raw))
	if label == "" {
		return ""
	}
	if slug, ok := systemLabelAliases[label]; ok {
		return slug
	}
	compact := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(label, "-", ""), "_", ""), " ", "")
	if slug, ok := systemLabelAliases[compact]; ok {
		return slug
	}
	if isSupportedSystemSlug(label) {
		return canonicalSegment(label, "")
	}
	return ""
}

func normalizedSystemTitleHintKey(raw string) string {
	clean := strings.ToLower(strings.TrimSpace(raw))
	clean = strings.Trim(clean, " .,:;!?-_")
	if clean == "" {
		return ""
	}
	return strings.Join(strings.Fields(clean), " ")
}

func knownSystemSlugFromTitleHint(raw string) string {
	key := normalizedSystemTitleHintKey(raw)
	if key == "" {
		return ""
	}
	if slug, ok := exactTitleSystemHints[key]; ok && isSupportedSystemSlug(slug) {
		return slug
	}
	return ""
}

func detectSaveSystem(input saveSystemDetectionInput) saveSystemDetectionResult {
	filename := strings.TrimSpace(input.Filename)
	displayTitle := strings.TrimSpace(input.DisplayTitle)
	lowerFilename := strings.ToLower(filename)
	lowerTitle := strings.ToLower(displayTitle)
	lowerName := strings.ToLower(filename + " " + displayTitle)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	payload := input.Payload

	if noisy, reason := isLikelyNoiseFilename(filename); noisy {
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: 0,
			Reason:     reason,
			Noise:      true,
		}
	}

	if looksLikeExecutableOrArchivePayload(payload) {
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: 0,
			Reason:     "payload looks like executable/archive",
			Noise:      true,
		}
	}

	type detectionCandidate struct {
		score     int
		reason    string
		declared  bool
		payload   bool
		pathHint  bool
		titleHint bool
	}

	score := map[string]detectionCandidate{}
	setScore := func(slug string, value int, why string, update func(*detectionCandidate)) {
		if !isSupportedSystemSlug(slug) {
			return
		}
		candidate := score[slug]
		if value > candidate.score {
			candidate.score = value
			candidate.reason = why
		}
		if update != nil {
			update(&candidate)
		}
		score[slug] = candidate
	}

	if input.DeclaredFallbackOnly {
		if declared := supportedSystemSlugFromLabel(input.DeclaredSystemSlug); declared != "" {
			setScore(declared, 69, "stored system fallback", func(candidate *detectionCandidate) {
				candidate.declared = true
			})
		}
		if input.DeclaredSystem != nil {
			if declared := supportedSystemSlugFromLabel(input.DeclaredSystem.Slug); declared != "" {
				setScore(declared, 69, "stored system fallback", func(candidate *detectionCandidate) {
					candidate.declared = true
				})
			}
			if declared := supportedSystemSlugFromLabel(input.DeclaredSystem.Name); declared != "" {
				setScore(declared, 69, "stored system fallback", func(candidate *detectionCandidate) {
					candidate.declared = true
				})
			}
		}
	} else {
		if declared := supportedSystemSlugFromLabel(input.DeclaredSystemSlug); declared != "" {
			setScore(declared, 92, "declared system slug", func(candidate *detectionCandidate) {
				candidate.declared = true
			})
		}
		if input.DeclaredSystem != nil {
			if declared := supportedSystemSlugFromLabel(input.DeclaredSystem.Slug); declared != "" {
				setScore(declared, 90, "declared system struct slug", func(candidate *detectionCandidate) {
					candidate.declared = true
				})
			}
			if declared := supportedSystemSlugFromLabel(input.DeclaredSystem.Name); declared != "" {
				setScore(declared, 88, "declared system name", func(candidate *detectionCandidate) {
					candidate.declared = true
				})
			}
		}
	}

	if isLikelyPS1MemoryCard(payload, ext) {
		setScore("psx", 100, "ps1 memory card signature", func(candidate *detectionCandidate) {
			candidate.payload = true
		})
	}
	if isLikelyPS2MemoryCard(payload) {
		setScore("ps2", 100, "ps2 memory card header", func(candidate *detectionCandidate) {
			candidate.payload = true
		})
	}
	if hasGBABackupSignature(payload) {
		setScore("gba", 96, "gba backup signature", func(candidate *detectionCandidate) {
			candidate.payload = true
		})
	}
	if isLikelyStrictNeoGeoSave(filename, displayTitle, payload) {
		setScore("neogeo", 97, "known neo geo set-name with strict save layout", func(candidate *detectionCandidate) {
			candidate.payload = true
		})
	}

	switch ext {
	case "dsv":
		setScore("nds", 90, "dsv extension", nil)
	case "eep", "fla", "sra", "mpk":
		setScore("n64", 90, ext+" extension", nil)
	case "mcr", "mc", "mcd", "gme", "vmp", "psv":
		setScore("psx", 90, ext+" extension", nil)
	case "ps2":
		setScore("ps2", 90, "ps2 extension", nil)
	case "sa1":
		setScore("snes", 74, "sa1 extension", nil)
	case "srm":
		setScore("snes", 68, "srm extension", nil)
		setScore("gameboy", 62, "srm extension", nil)
		setScore("genesis", 60, "srm extension", nil)
	case "sav":
		setScore("gameboy", 56, "sav extension", nil)
		setScore("gba", 56, "sav extension", nil)
		setScore("nds", 56, "sav extension", nil)
	case "ram":
		setScore("genesis", 60, "ram extension", nil)
		setScore("master-system", 56, "ram extension", nil)
		setScore("game-gear", 56, "ram extension", nil)
		setScore("neogeo", 62, "ram extension", nil)
		setScore("arcade", 58, "ram extension", nil)
	case "nv", "nvram", "hi", "eeprom":
		setScore("arcade", 82, ext+" extension", nil)
	}

	if containsAny(lowerFilename, []string{"game boy", "gameboy", "/gb/", "\\gb\\", ".gb"}) {
		setScore("gameboy", 70, "gameboy filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerFilename, []string{"gba", "game boy advance", "mgba", "visualboyadvance"}) {
		setScore("gba", 74, "gba filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerFilename, []string{"nds", "nintendo ds", "melonds", "desmume"}) {
		setScore("nds", 78, "nds filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerName, []string{"snes", "super nintendo", "sfc", "bsnes", "snes9x"}) {
		setScore("snes", 76, "snes filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerName, []string{"n64", "nintendo 64", "mupen", "project64"}) {
		setScore("n64", 78, "n64 filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerFilename, []string{"neogeo", "neo geo", "neo-geo", "/mvs/", "\\mvs\\", "/aes/", "\\aes\\"}) {
		setScore("neogeo", 82, "neo geo filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerName, []string{"genesis", "mega drive", "megadrive"}) {
		setScore("genesis", 74, "genesis filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerFilename, []string{"master system", "/sms/", "\\sms\\"}) {
		setScore("master-system", 76, "master system filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerFilename, []string{"game gear", "/gg/", "\\gg\\"}) {
		setScore("game-gear", 76, "game gear filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerName, []string{"playstation", "psx", "ps1", "duckstation", "pcsx"}) {
		setScore("psx", 78, "playstation filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerName, []string{"ps2", "pcsx2", "playstation 2"}) {
		setScore("ps2", 80, "ps2 filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerName, []string{"psp", "ppsspp", "playstation portable"}) {
		setScore("psp", 78, "psp filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerName, []string{"ps vita", "psvita", "vita3k"}) {
		setScore("psvita", 78, "psvita filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerName, []string{"ps3", "rpcs3"}) {
		setScore("ps3", 76, "ps3 filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerName, []string{"ps4"}) {
		setScore("ps4", 76, "ps4 filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerName, []string{"ps5"}) {
		setScore("ps5", 76, "ps5 filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerFilename, arcadeFilenameHints) {
		setScore("arcade", 84, "arcade filename hint", func(candidate *detectionCandidate) {
			candidate.pathHint = true
		})
	}
	if containsAny(lowerTitle, []string{"super nintendo", "snes"}) {
		setScore("snes", 66, "title hint", func(candidate *detectionCandidate) {
			candidate.titleHint = true
		})
	}
	if containsAny(lowerTitle, []string{"nintendo 64", "n64"}) {
		setScore("n64", 66, "title hint", func(candidate *detectionCandidate) {
			candidate.titleHint = true
		})
	}
	if hinted := knownSystemSlugFromTitleHint(displayTitle); hinted != "" {
		setScore(hinted, 80, "exact title hint", func(candidate *detectionCandidate) {
			candidate.titleHint = true
		})
	}

	bestSlug := ""
	bestCandidate := detectionCandidate{}
	for slug, candidate := range score {
		if candidate.score > bestCandidate.score {
			bestSlug = slug
			bestCandidate = candidate
		}
	}

	if bestSlug == "" || bestCandidate.score < 50 {
		noise := looksLikeMostlyTextPayload(payload)
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: bestCandidate.score,
			Reason:     "no supported console signature",
			Noise:      noise,
		}
	}

	if isGenericSaveExtension(ext) && !bestCandidate.declared && !bestCandidate.payload && !bestCandidate.pathHint && bestCandidate.score < 68 {
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: bestCandidate.score,
			Reason:     "generic save extension without trusted system evidence",
			Noise:      true,
		}
	}

	if isGenericSaveExtension(ext) && bestCandidate.declared && !bestCandidate.payload && !bestCandidate.pathHint && !bestCandidate.titleHint {
		if generic, reason := isLikelyGenericFallbackTitle(firstNonEmpty(displayTitle, strings.TrimSuffix(filename, filepath.Ext(filename)))); generic {
			return saveSystemDetectionResult{
				Slug:       "unknown-system",
				System:     nil,
				Confidence: bestCandidate.score,
				Reason:     reason,
				Noise:      true,
			}
		}
	}

	if bestSlug == "arcade" && !bestCandidate.payload && !bestCandidate.pathHint && !isDedicatedArcadeExtension(ext) {
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: bestCandidate.score,
			Reason:     "arcade requires dedicated header/extension or machine hint",
			Noise:      true,
		}
	}

	// Treat plain-text payload as noise unless the system evidence is strong
	// (explicit console signatures or declared trusted source).
	if looksLikeMostlyTextPayload(payload) && !bestCandidate.declared && !bestCandidate.payload && bestCandidate.score < 85 {
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: bestCandidate.score,
			Reason:     "payload looks like text/noise",
			Noise:      true,
		}
	}

	bestSystem := supportedSystemFromSlug(bestSlug)
	return saveSystemDetectionResult{
		Slug:       bestSlug,
		System:     bestSystem,
		Confidence: bestCandidate.score,
		Reason:     bestCandidate.reason,
		Noise:      false,
	}
}

func isLikelyPS2MemoryCard(payload []byte) bool {
	if len(payload) < len(ps2MemoryCardHeaderMagic) {
		return false
	}
	return string(payload[:len(ps2MemoryCardHeaderMagic)]) == ps2MemoryCardHeaderMagic
}

func isLikelyPS1MemoryCard(payload []byte, ext string) bool {
	switch ext {
	case "gme":
		return len(payload) == ps1MemoryCardTotalSize+ps1DexDriveHeaderSize
	case "vmp":
		if len(payload) != ps1MemoryCardTotalSize+ps1PSPVMPHeaderSize {
			return false
		}
		return len(payload) > 5 && payload[1] == 0x50 && payload[2] == 0x4D && payload[3] == 0x56
	case "mcr", "mcd", "mc", "psv":
		if len(payload) != ps1MemoryCardTotalSize {
			return false
		}
		return bytes.HasPrefix(payload, []byte("MC"))
	default:
		if len(payload) != ps1MemoryCardTotalSize {
			return false
		}
		return bytes.HasPrefix(payload, []byte("MC"))
	}
}

func hasGBABackupSignature(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}
	limit := len(payload)
	if limit > 512*1024 {
		limit = 512 * 1024
	}
	upper := strings.ToUpper(string(payload[:limit]))
	return strings.Contains(upper, "EEPROM_V") || strings.Contains(upper, "SRAM_V") || strings.Contains(upper, "FLASH_V") || strings.Contains(upper, "FLASH1M_V")
}

func isLikelyStrictNeoGeoSave(filename, displayTitle string, payload []byte) bool {
	if !isKnownNeoGeoSetName(filename, displayTitle) {
		return false
	}
	return hasStrictNeoGeoSaveLayout(payload)
}

func isKnownNeoGeoSetName(filename, displayTitle string) bool {
	candidates := []string{
		strings.TrimSuffix(filepath.Base(strings.TrimSpace(filename)), filepath.Ext(strings.TrimSpace(filename))),
		strings.TrimSpace(displayTitle),
	}
	for _, candidate := range candidates {
		key := canonicalSegment(candidate, "")
		if key == "" {
			continue
		}
		if _, ok := strictNeoGeoSetNames[key]; ok {
			return true
		}
	}
	return false
}

func hasStrictNeoGeoSaveLayout(payload []byte) bool {
	if len(payload) == 0 || looksLikeMostlyTextPayload(payload) {
		return false
	}

	switch len(payload) {
	case neoGeoSaveRAMSize:
		return countNonPaddingBytes(payload) >= 16
	case neoGeoCardDataSize:
		return countNonPaddingBytes(payload) >= 16
	case neoGeoCompoundSaveSize:
		saveRAM := payload[:neoGeoSaveRAMSize]
		cardData := payload[neoGeoSaveRAMSize:]
		if countNonPaddingBytes(cardData) < 16 {
			return false
		}
		return paddingRatio(saveRAM) >= 0.75
	default:
		return false
	}
}

func countNonPaddingBytes(payload []byte) int {
	count := 0
	for _, b := range payload {
		if b != 0x00 && b != 0xff {
			count++
		}
	}
	return count
}

func paddingRatio(payload []byte) float64 {
	if len(payload) == 0 {
		return 0
	}
	padding := 0
	for _, b := range payload {
		if b == 0x00 || b == 0xff {
			padding++
		}
	}
	return float64(padding) / float64(len(payload))
}

func looksLikeExecutableOrArchivePayload(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	header := payload
	if len(header) > 8 {
		header = header[:8]
	}
	return bytes.HasPrefix(header, []byte("MZ")) ||
		bytes.HasPrefix(header, []byte{0x7f, 'E', 'L', 'F'}) ||
		bytes.HasPrefix(header, []byte("PK\x03\x04")) ||
		bytes.HasPrefix(header, []byte("PK\x05\x06")) ||
		bytes.HasPrefix(header, []byte("PK\x07\x08")) ||
		bytes.HasPrefix(header, []byte{0x1f, 0x8b}) ||
		bytes.HasPrefix(header, []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C})
}

func looksLikeMostlyTextPayload(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}
	sample := payload
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	printable := 0
	zero := 0
	for _, b := range sample {
		if b == 0x00 {
			zero++
			continue
		}
		if b == '\n' || b == '\r' || b == '\t' || (b >= 32 && b <= 126) {
			printable++
		}
	}
	if zero > len(sample)/32 {
		return false
	}
	return printable >= (len(sample)*9)/10
}

func containsAny(haystack string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func isGenericSaveExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "sav", "srm", "ram":
		return true
	default:
		return false
	}
}

func isDedicatedArcadeExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "nv", "nvram", "hi", "eeprom":
		return true
	default:
		return false
	}
}

func isLikelyNoiseFilename(filename string) (bool, string) {
	stem := strings.TrimSpace(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	if stem == "" {
		return true, "empty filename"
	}

	normalized := canonicalSegment(stem, "")
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")

	exactDeny := map[string]string{
		"settings":                  "generic settings artifact",
		"manifest":                  "generic manifest artifact",
		"saveindex":                 "generic save index artifact",
		"save":                      "generic save artifact",
		"default":                   "generic default artifact",
		"usersettings":              "generic user settings artifact",
		"profileusersettings":       "generic profile user settings artifact",
		"enhancedinputusersettings": "generic enhanced input settings artifact",
		"savegamemanifest":          "generic save manifest artifact",
	}
	if reason, ok := exactDeny[normalized]; ok {
		return true, reason
	}
	if strings.Contains(normalized, "usersettings") {
		return true, "generic user settings artifact"
	}
	if strings.Contains(normalized, "saveindex") {
		return true, "generic save index artifact"
	}
	if strings.Contains(normalized, "savegamemanifest") || strings.Contains(normalized, "manifest") && strings.Contains(normalized, "save") {
		return true, "generic save manifest artifact"
	}
	if normalized == "ps3logo" && ext == "dat" {
		return true, "console logo artifact"
	}
	if normalized == "edge" && (ext == "dat" || ext == "sav") {
		return true, "browser/runtime artifact"
	}
	return false, ""
}

func isLikelyGenericFallbackTitle(raw string) (bool, string) {
	title := strings.TrimSpace(raw)
	if title == "" {
		return true, "empty save title"
	}

	normalized := canonicalSegment(title, "")
	exactDeny := map[string]string{
		"autosave":                 "generic autosave slot artifact",
		"save":                     "generic save slot artifact",
		"savefile":                 "generic save file artifact",
		"saveslot":                 "generic save slot artifact",
		"playerslot":               "generic player slot artifact",
		"spsaveslot":               "generic save slot artifact",
		"slot":                     "generic slot artifact",
		"inputsettings":            "generic input settings artifact",
		"inputsettingskeymappings": "generic input settings artifact",
		"optionsettings":           "generic option settings artifact",
		"optionpconly":             "generic option settings artifact",
		"saveslot0":                "generic save slot artifact",
		"save00":                   "generic save slot artifact",
	}
	if reason, ok := exactDeny[normalized]; ok {
		return true, reason
	}

	prefixDeny := map[string]string{
		"autosave":       "generic autosave slot artifact",
		"saveslot":       "generic save slot artifact",
		"playerslot":     "generic player slot artifact",
		"spsaveslot":     "generic save slot artifact",
		"savefile":       "generic save file artifact",
		"inputsettings":  "generic input settings artifact",
		"optionsettings": "generic option settings artifact",
		"optionpconly":   "generic option settings artifact",
	}
	for prefix, reason := range prefixDeny {
		if strings.HasPrefix(normalized, prefix) {
			return true, reason
		}
	}
	if strings.HasPrefix(normalized, "save-") || strings.HasPrefix(normalized, "slot-") {
		return true, "generic save slot artifact"
	}
	if numberedSaveSlotTitlePattern.MatchString(title) {
		return true, "numbered mission/save-slot artifact"
	}
	return false, ""
}
