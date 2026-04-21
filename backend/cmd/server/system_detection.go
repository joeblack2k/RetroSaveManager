package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ps1MemoryCardTotalSize   = 128 * 1024
	ps1DexDriveHeaderSize    = 3904
	ps1PSPVMPHeaderSize      = 0x80
	ps2MemoryCardHeaderMagic = "Sony PS2 Memory Card Format "
)

var errUnsupportedSaveFormat = errors.New("unsupported or unrecognized save format; only known consoles/arcade are allowed")

var supportedSystemsBySlug = map[string]system{
	"arcade":       {ID: 900001, Name: "Arcade", Slug: "arcade", Manufacturer: "Arcade"},
	"game-gear":    {ID: 900002, Name: "Game Gear", Slug: "game-gear", Manufacturer: "Sega"},
	"gameboy":      {ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy", Manufacturer: "Nintendo"},
	"gba":          {ID: 24, Name: "Game Boy Advance", Slug: "gba", Manufacturer: "Nintendo"},
	"genesis":      {ID: 33, Name: "Sega Genesis/Mega Drive", Slug: "genesis", Manufacturer: "Sega"},
	"master-system": {ID: 900003, Name: "Master System", Slug: "master-system", Manufacturer: "Sega"},
	"n64":          {ID: 64, Name: "Nintendo 64", Slug: "n64", Manufacturer: "Nintendo"},
	"nds":          {ID: 900004, Name: "Nintendo DS", Slug: "nds", Manufacturer: "Nintendo"},
	"neogeo":       {ID: 900005, Name: "Neo Geo", Slug: "neogeo", Manufacturer: "SNK"},
	"nes":          {ID: 900006, Name: "Nintendo Entertainment System", Slug: "nes", Manufacturer: "Nintendo"},
	"ps2":          {ID: 900007, Name: "PlayStation 2", Slug: "ps2", Manufacturer: "Sony"},
	"ps3":          {ID: 900008, Name: "PlayStation 3", Slug: "ps3", Manufacturer: "Sony"},
	"ps4":          {ID: 900009, Name: "PlayStation 4", Slug: "ps4", Manufacturer: "Sony"},
	"ps5":          {ID: 900010, Name: "PlayStation 5", Slug: "ps5", Manufacturer: "Sony"},
	"psp":          {ID: 900011, Name: "PlayStation Portable", Slug: "psp", Manufacturer: "Sony"},
	"psvita":       {ID: 900012, Name: "PlayStation Vita", Slug: "psvita", Manufacturer: "Sony"},
	"psx":          {ID: 27, Name: "PlayStation", Slug: "psx", Manufacturer: "Sony"},
	"snes":         {ID: 26, Name: "Nintendo Super Nintendo Entertainment System", Slug: "snes", Manufacturer: "Nintendo"},
}

var systemLabelAliases = map[string]string{
	"arcade":              "arcade",
	"fbneo":               "arcade",
	"finalburn":           "arcade",
	"mame":                "arcade",
	"gameboy":             "gameboy",
	"game boy":            "gameboy",
	"gb":                  "gameboy",
	"gameboyadvance":      "gba",
	"game boy advance":    "gba",
	"gba":                 "gba",
	"genesis":             "genesis",
	"mega drive":          "genesis",
	"megadrive":           "genesis",
	"mastersystem":        "master-system",
	"master system":       "master-system",
	"sms":                 "master-system",
	"game gear":           "game-gear",
	"n64":                 "n64",
	"nintendo64":          "n64",
	"nintendo ds":         "nds",
	"nds":                 "nds",
	"ds":                  "nds",
	"neogeo":              "neogeo",
	"neo geo":             "neogeo",
	"neo-geo":             "neogeo",
	"nes":                 "nes",
	"famicom":             "nes",
	"psx":                 "psx",
	"ps1":                 "psx",
	"playstation":         "psx",
	"playstation 1":       "psx",
	"playstation2":        "ps2",
	"playstation 2":       "ps2",
	"ps2":                 "ps2",
	"playstation3":        "ps3",
	"playstation 3":       "ps3",
	"ps3":                 "ps3",
	"playstation4":        "ps4",
	"playstation 4":       "ps4",
	"ps4":                 "ps4",
	"playstation5":        "ps5",
	"playstation 5":       "ps5",
	"ps5":                 "ps5",
	"psp":                 "psp",
	"playstation portable": "psp",
	"psvita":              "psvita",
	"ps vita":             "psvita",
	"vita":                "psvita",
	"snes":                "snes",
	"super nintendo":      "snes",
	"sfc":                 "snes",
}

var arcadeFilenameHints = []string{
	"arcade", "mame", "fbneo", "finalburn", "model2", "naomi", "daytona", "ghost house",
}

type saveSystemDetectionInput struct {
	Filename           string
	DisplayTitle       string
	Payload            []byte
	DeclaredSystemSlug string
	DeclaredSystem     *system
}

type saveSystemDetectionResult struct {
	Slug       string
	System     *system
	Confidence int
	Reason     string
	Noise      bool
}

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
	lowerName := strings.ToLower(filename + " " + displayTitle)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	payload := input.Payload

	if looksLikeExecutableOrArchivePayload(payload) {
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: 0,
			Reason:     "payload looks like executable/archive",
			Noise:      true,
		}
	}

	score := map[string]int{}
	reason := map[string]string{}
	setScore := func(slug string, value int, why string) {
		if !isSupportedSystemSlug(slug) {
			return
		}
		if value > score[slug] {
			score[slug] = value
			reason[slug] = why
		}
	}

	if declared := supportedSystemSlugFromLabel(input.DeclaredSystemSlug); declared != "" {
		setScore(declared, 92, "declared system slug")
	}
	if input.DeclaredSystem != nil {
		if declared := supportedSystemSlugFromLabel(input.DeclaredSystem.Slug); declared != "" {
			setScore(declared, 90, "declared system struct slug")
		}
		if declared := supportedSystemSlugFromLabel(input.DeclaredSystem.Name); declared != "" {
			setScore(declared, 88, "declared system name")
		}
	}

	if isLikelyPS1MemoryCard(payload, ext) {
		setScore("psx", 100, "ps1 memory card signature")
	}
	if isLikelyPS2MemoryCard(payload) {
		setScore("ps2", 100, "ps2 memory card header")
	}
	if hasGBABackupSignature(payload) {
		setScore("gba", 96, "gba backup signature")
	}

	switch ext {
	case "dsv":
		setScore("nds", 90, "dsv extension")
	case "eep", "fla", "sra", "mpk":
		setScore("n64", 90, ext+" extension")
	case "mcr", "mc", "mcd", "gme", "vmp", "psv":
		setScore("psx", 90, ext+" extension")
	case "ps2":
		setScore("ps2", 90, "ps2 extension")
	case "sa1":
		setScore("snes", 74, "sa1 extension")
	case "srm":
		setScore("snes", 68, "srm extension")
		setScore("gameboy", 62, "srm extension")
		setScore("genesis", 60, "srm extension")
	case "sav":
		setScore("gameboy", 56, "sav extension")
		setScore("gba", 56, "sav extension")
		setScore("nds", 56, "sav extension")
	case "ram":
		setScore("genesis", 60, "ram extension")
		setScore("master-system", 56, "ram extension")
		setScore("game-gear", 56, "ram extension")
		setScore("neogeo", 62, "ram extension")
		setScore("arcade", 58, "ram extension")
	case "nv", "nvram", "hi", "eeprom":
		setScore("arcade", 82, ext+" extension")
	}

	if containsAny(lowerName, []string{"game boy", "gameboy", "/gb/", "\\gb\\", ".gb"}) {
		setScore("gameboy", 70, "gameboy filename hint")
	}
	if containsAny(lowerName, []string{"gba", "game boy advance", "mgba", "visualboyadvance"}) {
		setScore("gba", 74, "gba filename hint")
	}
	if containsAny(lowerName, []string{"nds", "nintendo ds", "melonds", "desmume"}) {
		setScore("nds", 78, "nds filename hint")
	}
	if containsAny(lowerName, []string{"snes", "super nintendo", "sfc", "bsnes", "snes9x"}) {
		setScore("snes", 76, "snes filename hint")
	}
	if containsAny(lowerName, []string{"n64", "nintendo 64", "mupen", "project64"}) {
		setScore("n64", 78, "n64 filename hint")
	}
	if containsAny(lowerName, []string{"neogeo", "neo geo", "neo-geo", "/mvs/", "\\mvs\\", "/aes/", "\\aes\\"}) {
		setScore("neogeo", 82, "neo geo filename hint")
	}
	if containsAny(lowerName, []string{"genesis", "mega drive", "megadrive"}) {
		setScore("genesis", 74, "genesis filename hint")
	}
	if containsAny(lowerName, []string{"master system", "/sms/", "\\sms\\"}) {
		setScore("master-system", 76, "master system filename hint")
	}
	if containsAny(lowerName, []string{"game gear", "/gg/", "\\gg\\"}) {
		setScore("game-gear", 76, "game gear filename hint")
	}
	if containsAny(lowerName, []string{"playstation", "psx", "ps1", "duckstation", "pcsx"}) {
		setScore("psx", 78, "playstation filename hint")
	}
	if containsAny(lowerName, []string{"ps2", "pcsx2", "playstation 2"}) {
		setScore("ps2", 80, "ps2 filename hint")
	}
	if containsAny(lowerName, []string{"psp", "ppsspp", "playstation portable"}) {
		setScore("psp", 78, "psp filename hint")
	}
	if containsAny(lowerName, []string{"ps vita", "psvita", "vita3k"}) {
		setScore("psvita", 78, "psvita filename hint")
	}
	if containsAny(lowerName, []string{"ps3", "rpcs3"}) {
		setScore("ps3", 76, "ps3 filename hint")
	}
	if containsAny(lowerName, []string{"ps4"}) {
		setScore("ps4", 76, "ps4 filename hint")
	}
	if containsAny(lowerName, []string{"ps5"}) {
		setScore("ps5", 76, "ps5 filename hint")
	}
	if containsAny(lowerName, arcadeFilenameHints) {
		setScore("arcade", 84, "arcade filename hint")
	}

	bestSlug := ""
	bestScore := 0
	bestReason := ""
	for slug, candidateScore := range score {
		if candidateScore > bestScore {
			bestSlug = slug
			bestScore = candidateScore
			bestReason = reason[slug]
		}
	}

	if bestSlug == "" || bestScore < 50 {
		noise := looksLikeMostlyTextPayload(payload)
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: bestScore,
			Reason:     "no supported console signature",
			Noise:      noise,
		}
	}

	// Treat plain-text payload as noise unless the system evidence is strong
	// (explicit console signatures or declared trusted source).
	if looksLikeMostlyTextPayload(payload) && bestScore < 85 {
		return saveSystemDetectionResult{
			Slug:       "unknown-system",
			System:     nil,
			Confidence: bestScore,
			Reason:     "payload looks like text/noise",
			Noise:      true,
		}
	}

	bestSystem := supportedSystemFromSlug(bestSlug)
	return saveSystemDetectionResult{
		Slug:       bestSlug,
		System:     bestSystem,
		Confidence: bestScore,
		Reason:     bestReason,
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
