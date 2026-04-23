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
	"dreamcast":     {ID: 900013, Name: "Sega Dreamcast", Slug: "dreamcast", Manufacturer: "Sega"},
	"master-system": {ID: 900003, Name: "Master System", Slug: "master-system", Manufacturer: "Sega"},
	"saturn":        {ID: 900014, Name: "Sega Saturn", Slug: "saturn", Manufacturer: "Sega"},
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
	"dreamcast":            "dreamcast",
	"sega dreamcast":       "dreamcast",
	"dc":                   "dreamcast",
	"saturn":               "saturn",
	"sega saturn":          "saturn",
	"ss":                   "saturn",
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

type saveSystemDetectionInput struct {
	Filename             string
	DisplayTitle         string
	Payload              []byte
	DeclaredSystemSlug   string
	DeclaredSystem       *system
	TrustedHelperSystem  bool
	DeclaredFallbackOnly bool
	TrustedStoredSystem  bool
}

type saveDetectionEvidence struct {
	Declared      bool `json:"declared,omitempty"`
	HelperTrusted bool `json:"helperTrusted,omitempty"`
	Payload       bool `json:"payload,omitempty"`
	PathHint      bool `json:"pathHint,omitempty"`
	FormatHint    bool `json:"formatHint,omitempty"`
	StoredTrusted bool `json:"storedTrusted,omitempty"`
	TitleHint     bool `json:"titleHint,omitempty"`
}

type saveSystemDetectionResult struct {
	Slug       string
	System     *system
	Confidence int
	Reason     string
	Noise      bool
	Evidence   saveDetectionEvidence
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

func detectSaveSystem(input saveSystemDetectionInput) saveSystemDetectionResult {
	filename := strings.TrimSpace(input.Filename)
	displayTitle := strings.TrimSpace(input.DisplayTitle)
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
		score         int
		reason        string
		declared      bool
		helperTrusted bool
		payload       bool
		pathHint      bool
		formatHint    bool
		storedTrusted bool
		titleHint     bool
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
		scoreValue := 69
		reason := "stored system fallback"
		if input.TrustedStoredSystem {
			scoreValue = 89
			reason = "stored trusted system evidence"
		}
		if declared := supportedSystemSlugFromLabel(input.DeclaredSystemSlug); declared != "" {
			setScore(declared, scoreValue, reason, func(candidate *detectionCandidate) {
				candidate.declared = true
				candidate.storedTrusted = input.TrustedStoredSystem
			})
		}
		if input.DeclaredSystem != nil {
			if declared := supportedSystemSlugFromLabel(input.DeclaredSystem.Slug); declared != "" {
				setScore(declared, scoreValue, reason, func(candidate *detectionCandidate) {
					candidate.declared = true
					candidate.storedTrusted = input.TrustedStoredSystem
				})
			}
			if declared := supportedSystemSlugFromLabel(input.DeclaredSystem.Name); declared != "" {
				setScore(declared, scoreValue, reason, func(candidate *detectionCandidate) {
					candidate.declared = true
					candidate.storedTrusted = input.TrustedStoredSystem
				})
			}
		}
	} else {
		if declared := supportedSystemSlugFromLabel(input.DeclaredSystemSlug); declared != "" {
			scoreValue := 92
			reason := "declared system slug"
			if input.TrustedHelperSystem {
				scoreValue = 96
				reason = "trusted helper declared system slug"
			}
			setScore(declared, scoreValue, reason, func(candidate *detectionCandidate) {
				candidate.declared = true
				candidate.helperTrusted = input.TrustedHelperSystem
			})
		}
		if input.DeclaredSystem != nil {
			if declared := supportedSystemSlugFromLabel(input.DeclaredSystem.Slug); declared != "" {
				scoreValue := 90
				reason := "declared system struct slug"
				if input.TrustedHelperSystem {
					scoreValue = 95
					reason = "trusted helper declared system struct slug"
				}
				setScore(declared, scoreValue, reason, func(candidate *detectionCandidate) {
					candidate.declared = true
					candidate.helperTrusted = input.TrustedHelperSystem
				})
			}
			if declared := supportedSystemSlugFromLabel(input.DeclaredSystem.Name); declared != "" {
				scoreValue := 88
				reason := "declared system name"
				if input.TrustedHelperSystem {
					scoreValue = 94
					reason = "trusted helper declared system name"
				}
				setScore(declared, scoreValue, reason, func(candidate *detectionCandidate) {
					candidate.declared = true
					candidate.helperTrusted = input.TrustedHelperSystem
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
	if parseDreamcastContainer(filename, payload) != nil {
		setScore("dreamcast", 99, "dreamcast vmu container signature", func(candidate *detectionCandidate) {
			candidate.payload = true
		})
	}
	if parseSaturnContainer(filename, payload) != nil {
		setScore("saturn", 99, "saturn backup ram signature", func(candidate *detectionCandidate) {
			candidate.payload = true
		})
	}
	if hasGBASignature(payload) {
		setScore("gba", 96, "gba backup signature", func(candidate *detectionCandidate) {
			candidate.payload = true
		})
	}
	if hasNewSuperMarioBrosNDSSignature(payload) {
		setScore("nds", 98, "new super mario bros nds save signature", func(candidate *detectionCandidate) {
			candidate.payload = true
		})
	}

	switch ext {
	case "dsv":
		setScore("nds", 90, "dsv extension", func(candidate *detectionCandidate) {
			candidate.formatHint = true
		})
	case "vms", "dci":
		setScore("dreamcast", 88, ext+" extension", func(candidate *detectionCandidate) {
			candidate.formatHint = true
		})
	case "bkr", "bcr", "bup":
		setScore("saturn", 84, ext+" extension", func(candidate *detectionCandidate) {
			candidate.formatHint = true
		})
	case "eep", "fla", "sra", "mpk":
		setScore("n64", 90, ext+" extension", func(candidate *detectionCandidate) {
			candidate.formatHint = true
		})
	case "mcr", "mc", "mcd", "gme", "vmp", "psv":
		setScore("psx", 90, ext+" extension", func(candidate *detectionCandidate) {
			candidate.formatHint = true
		})
	case "ps2":
		setScore("ps2", 90, "ps2 extension", func(candidate *detectionCandidate) {
			candidate.formatHint = true
		})
	case "sa1":
		setScore("snes", 74, "sa1 extension", func(candidate *detectionCandidate) {
			candidate.formatHint = true
		})
	case "nv", "nvram", "hi", "eeprom":
		setScore("arcade", 82, ext+" extension", func(candidate *detectionCandidate) {
			candidate.formatHint = true
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
		noise := isGenericSaveExtension(ext) || looksLikeMostlyTextPayload(payload)
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: bestCandidate.score,
			Reason:     "no supported console signature",
			Noise:      noise,
		}
	}

	if isGenericSaveExtension(ext) && !bestCandidate.payload && !bestCandidate.pathHint && !bestCandidate.formatHint && !bestCandidate.helperTrusted && !bestCandidate.storedTrusted {
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: bestCandidate.score,
			Reason:     "generic save extension without parser or helper evidence",
			Noise:      true,
		}
	}

	if isGenericSaveExtension(ext) && input.DeclaredFallbackOnly && bestCandidate.declared && !bestCandidate.payload && !bestCandidate.storedTrusted {
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: bestCandidate.score,
			Reason:     "generic save extension without trusted stored system evidence",
			Noise:      true,
		}
	}

	if input.DeclaredFallbackOnly && bestCandidate.declared && !bestCandidate.storedTrusted && !bestCandidate.payload && !bestCandidate.formatHint && !bestCandidate.pathHint {
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: bestCandidate.score,
			Reason:     "stored system fallback without trusted save evidence",
			Noise:      true,
		}
	}

	if isGenericSaveExtension(ext) && bestCandidate.declared && !bestCandidate.payload && !bestCandidate.pathHint && !bestCandidate.titleHint && !bestCandidate.storedTrusted {
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
	if looksLikeMostlyTextPayload(payload) && !bestCandidate.declared && !bestCandidate.payload && !bestCandidate.storedTrusted && bestCandidate.score < 85 {
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
		Evidence: saveDetectionEvidence{
			Declared:      bestCandidate.declared,
			HelperTrusted: bestCandidate.helperTrusted,
			Payload:       bestCandidate.payload,
			PathHint:      bestCandidate.pathHint,
			FormatHint:    bestCandidate.formatHint,
			StoredTrusted: bestCandidate.storedTrusted,
			TitleHint:     bestCandidate.titleHint,
		},
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

func hasGBASignature(payload []byte) bool {
	return hasGBABackupSignature(payload) || hasGBAEmbeddedGameHeader(payload)
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

func hasGBAEmbeddedGameHeader(payload []byte) bool {
	if len(payload) < 16 {
		return false
	}
	limit := len(payload)
	if limit > 128 {
		limit = 128
	}
	window := payload[:limit]
	for idx := 0; idx+9 <= len(window); idx++ {
		if window[idx] != 'A' || window[idx+1] != 'G' || window[idx+2] != 'B' {
			continue
		}
		printable := 0
		for j := idx + 3; j < len(window) && j < idx+24; j++ {
			if window[j] == 0x00 {
				break
			}
			if window[j] < 32 || window[j] > 126 {
				printable = 0
				break
			}
			printable++
		}
		if printable >= 6 {
			return true
		}
	}
	return false
}

func hasNewSuperMarioBrosNDSSignature(payload []byte) bool {
	if len(payload) != 8192 {
		return false
	}
	if looksLikeMostlyTextPayload(payload) {
		return false
	}
	return bytes.Count(payload, []byte("Mario2d")) >= 6
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
