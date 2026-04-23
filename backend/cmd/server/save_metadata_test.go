package main

import (
	"strings"
	"testing"
)

func TestCleanupDisplayTitleAndRegion(t *testing.T) {
	title, region := cleanupDisplayTitleAndRegion("Yoshi's Story (USA) (En,Ja)")
	if title != "Yoshi's Story" {
		t.Fatalf("unexpected title: %q", title)
	}
	if region != regionUS {
		t.Fatalf("unexpected region: %q", region)
	}
}

func TestCleanupDisplayTitleRegionAndLanguages(t *testing.T) {
	title, region, languages := cleanupDisplayTitleRegionAndLanguages("Yoshi's Story (USA) (En,Ja)")
	if title != "Yoshi's Story" {
		t.Fatalf("unexpected title: %q", title)
	}
	if region != regionUS {
		t.Fatalf("unexpected region: %q", region)
	}
	if len(languages) != 2 || languages[0] != "EN" || languages[1] != "JA" {
		t.Fatalf("unexpected languages: %#v", languages)
	}
}

func TestCleanupDisplayTitleRegionAndLanguagesStripsTrailingCounterNoise(t *testing.T) {
	title, region, languages := cleanupDisplayTitleRegionAndLanguages("The Legend of Zelda - A Link to the Past (USA)_1")
	if title != "The Legend of Zelda - A Link to the Past" {
		t.Fatalf("unexpected title: %q", title)
	}
	if region != regionUS {
		t.Fatalf("unexpected region: %q", region)
	}
	if len(languages) != 0 {
		t.Fatalf("expected no language codes, got %#v", languages)
	}
}

func TestResolveKnownSaveTitleAlias(t *testing.T) {
	if resolved := resolveKnownSaveTitleAlias("neogeo", "doubledr"); resolved != "Double Dragon" {
		t.Fatalf("expected Neo Geo alias to resolve, got %q", resolved)
	}
	if resolved := resolveKnownSaveTitleAlias("snes", "sm"); resolved != "sm" {
		t.Fatalf("expected unknown short title to stay untouched, got %q", resolved)
	}
}

func TestParsePlayStationMemoryCard(t *testing.T) {
	payload := make([]byte, ps1MemoryCardTotalSize)
	copy(payload[:2], []byte("MC"))
	dirOffset := psDirectoryEntrySize // slot 1
	payload[dirOffset] = 0x51
	copy(payload[dirOffset+0x0a:dirOffset+0x16], []byte("SCUS_941.63"))

	blockOffset := psMemoryCardBlockSize
	payload[blockOffset+0x60] = 0x1F
	payload[blockOffset+0x61] = 0x00
	payload[blockOffset+0x80] = 0x11
	copy(payload[blockOffset+16:blockOffset+64], []byte("Final Fantasy VII Save"))

	card := parsePlayStationMemoryCard(supportedSystemFromSlug("psx"), payload, "memory_card_1.mcr", "Memory Card 1")
	if card == nil {
		t.Fatal("expected memory card details")
	}
	if card.Name != "Memory Card 1" {
		t.Fatalf("unexpected card name: %q", card.Name)
	}
	if len(card.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(card.Entries))
	}

	entry := card.Entries[0]
	if entry.Slot != 1 {
		t.Fatalf("unexpected slot: %d", entry.Slot)
	}
	if entry.RegionCode != regionUS {
		t.Fatalf("unexpected region: %q", entry.RegionCode)
	}
	if !strings.Contains(strings.ToLower(entry.Title), "final") {
		t.Fatalf("unexpected title: %q", entry.Title)
	}
	if entry.IconDataURL == "" {
		t.Fatal("expected PS1 entry icon thumbnail")
	}
}

func TestNormalizeSaveInputSetsDisplayAndCardPath(t *testing.T) {
	a := &app{}
	regular := a.normalizeSaveInput(saveCreateInput{
		Filename: "Yoshi Story (USA).eep",
		Payload:  []byte("save-data"),
		Game:     game{Name: "Yoshi Story (USA)"},
	})
	if regular.DisplayTitle != "Yoshi Story" {
		t.Fatalf("unexpected display title: %q", regular.DisplayTitle)
	}
	if regular.RegionCode != regionUS {
		t.Fatalf("unexpected region code: %q", regular.RegionCode)
	}
	if len(regular.LanguageCodes) != 0 {
		t.Fatalf("expected no language codes, got %#v", regular.LanguageCodes)
	}
	if regular.GamePath != "Yoshi Story" {
		t.Fatalf("unexpected game path: %q", regular.GamePath)
	}

	cardPayload := make([]byte, ps1MemoryCardTotalSize)
	copy(cardPayload[:2], []byte("MC"))
	cardOffset := psDirectoryEntrySize
	cardPayload[cardOffset] = 0x51
	copy(cardPayload[cardOffset+0x0a:cardOffset+0x16], []byte("SLES_123.45"))
	ps := a.normalizeSaveInput(saveCreateInput{
		Filename: "memory_card_1.mcr",
		Payload:  cardPayload,
		Game:     game{Name: "Final Fantasy VII (Europe)"},
	})
	if ps.GamePath != "Memory Card 1" {
		t.Fatalf("unexpected ps game path: %q", ps.GamePath)
	}
	if ps.MemoryCard == nil {
		t.Fatal("expected memory card details")
	}
	if ps.MemoryCard.Name != "Memory Card 1" {
		t.Fatalf("unexpected memory card name: %q", ps.MemoryCard.Name)
	}
}

func TestNormalizeSaveInputDoesNotTreatNonPS1SonySaveAsMemoryCard(t *testing.T) {
	a := &app{}
	normalized := a.normalizeSaveInput(saveCreateInput{
		Filename:   "PS3LOGO.DAT",
		Payload:    []byte{0x01, 0x02, 0x03, 0x04},
		Game:       game{Name: "PS3LOGO"},
		SystemSlug: "ps3",
	})
	if normalized.MemoryCard != nil {
		t.Fatalf("expected non-PS1 save to avoid memory card metadata, got %#v", normalized.MemoryCard)
	}
	if normalized.DisplayTitle == "Memory Card 1" {
		t.Fatalf("unexpected memory card title for non-PS1 save")
	}
}

func TestNormalizeSaveInputRejectsPlayStationSaveStateNoise(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:   "Castlevania - Symphony of the Night (USA)_1.ss",
		Payload:    make([]byte, 4*1024*1024),
		Game:       game{Name: "Castlevania - Symphony of the Night"},
		SystemSlug: "psx",
	})
	if !result.Rejected {
		t.Fatal("expected PS1 save state to be rejected")
	}
	if result.RejectReason == "" {
		t.Fatal("expected PS1 save state rejection reason")
	}
}

func TestNormalizeSaveInputAcceptsRawPS1CardWithSavExtension(t *testing.T) {
	a := &app{}
	payload := make([]byte, ps1MemoryCardTotalSize)
	copy(payload[:2], []byte("MC"))
	dirOffset := psDirectoryEntrySize
	payload[dirOffset] = 0x51
	copy(payload[dirOffset+0x0a:dirOffset+0x16], []byte("SCUS_941.63"))
	blockOffset := psMemoryCardBlockSize
	payload[blockOffset+2] = 0x11
	payload[blockOffset+0x60] = 0x1F
	payload[blockOffset+0x61] = 0x00
	payload[blockOffset+0x80] = 0x11
	copy(payload[blockOffset+4:blockOffset+4+len("Final Fantasy VII")], []byte("Final Fantasy VII"))
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename: "psx.sav",
		Payload:  payload,
		Game:     game{Name: "PlayStation Save"},
	})
	if result.Rejected {
		t.Fatalf("expected raw PS1 card to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.SystemSlug != "psx" {
		t.Fatalf("expected PS1 card system slug, got %q", result.Input.SystemSlug)
	}
	if result.Input.MemoryCard == nil || len(result.Input.MemoryCard.Entries) != 1 {
		t.Fatalf("expected parsed PS1 memory card entries, got %#v", result.Input.MemoryCard)
	}
}

func TestNormalizeSaveInputDetectsLanguageAndRegionFromPayload(t *testing.T) {
	a := &app{}
	input := a.normalizeSaveInput(saveCreateInput{
		Filename: "Yoshi's Story (En,Ja).eep",
		Payload:  []byte("header-SCUS_123.45-footer"),
		Game:     game{Name: "Yoshi's Story (En,Ja)"},
	})
	if input.DisplayTitle != "Yoshi's Story" {
		t.Fatalf("unexpected display title: %q", input.DisplayTitle)
	}
	if input.RegionCode != regionUS {
		t.Fatalf("expected US region from payload product code, got %q", input.RegionCode)
	}
	if len(input.LanguageCodes) != 2 || input.LanguageCodes[0] != "EN" || input.LanguageCodes[1] != "JA" {
		t.Fatalf("unexpected language codes: %#v", input.LanguageCodes)
	}
}

func TestDetectSaveSystemDoesNotPromoteArcadeFromTitleOnly(t *testing.T) {
	daytona := detectSaveSystem(saveSystemDetectionInput{
		Filename:     "daytona.ram",
		DisplayTitle: "Daytona USA",
		Payload:      []byte{0x01, 0x02, 0x03, 0x04, 0x05},
	})
	if daytona.System != nil {
		t.Fatalf("expected no trusted system for title-only arcade hint, got %q", daytona.Slug)
	}
	if !daytona.Noise {
		t.Fatalf("expected title-only generic ram file to be rejected as noise")
	}

	ghostHouse := detectSaveSystem(saveSystemDetectionInput{
		Filename:     "Ghost House.sav",
		DisplayTitle: "Ghost House",
		Payload:      []byte{0x12, 0x99, 0x44, 0x88},
	})
	if ghostHouse.System != nil {
		t.Fatalf("expected no trusted system for title-only arcade hint, got %q", ghostHouse.Slug)
	}
}

func TestDetectSaveSystemRejectsDeclaredSNESForGenericExtensionWithoutHelperEvidence(t *testing.T) {
	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename:           "The Legend of Zelda - A Link to the Past (USA).sav",
		DisplayTitle:       "The Legend of Zelda - A Link to the Past",
		Payload:            []byte{0x01, 0x00, 0x01, 0x00},
		DeclaredSystemSlug: "snes",
	})
	if detected.Slug != "unknown-system" {
		t.Fatalf("expected generic SNES save to be rejected without helper evidence, got %q", detected.Slug)
	}
	if detected.System != nil {
		t.Fatalf("expected no supported system details, got %#v", detected.System)
	}
}

func TestDetectSaveSystemAcceptsDeclaredSNESForGenericExtensionWithHelperEvidence(t *testing.T) {
	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename:            "The Legend of Zelda - A Link to the Past (USA).sav",
		DisplayTitle:        "The Legend of Zelda - A Link to the Past",
		Payload:             []byte{0x01, 0x00, 0x01, 0x00},
		DeclaredSystemSlug:  "snes",
		TrustedHelperSystem: true,
	})
	if detected.Slug != "snes" {
		t.Fatalf("expected helper-trusted SNES system to win, got %q", detected.Slug)
	}
	if detected.System == nil || detected.System.Slug != "snes" {
		t.Fatalf("expected SNES system details, got %#v", detected.System)
	}
}

func TestDetectSaveSystemRejectsNintendoDSTitleHintWhenStoredSystemFallsBack(t *testing.T) {
	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename:             "New Super Mario Bros. (USA).sav",
		DisplayTitle:         "New Super Mario Bros.",
		Payload:              make([]byte, 8192),
		DeclaredSystemSlug:   "gameboy",
		DeclaredSystem:       &system{ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy"},
		DeclaredFallbackOnly: true,
	})
	if detected.Slug != "unknown-system" {
		t.Fatalf("expected Nintendo DS title hint to be rejected without trusted evidence, got %q", detected.Slug)
	}
	if detected.System != nil {
		t.Fatalf("expected no trusted system details, got %#v", detected.System)
	}
}

func TestDetectSaveSystemKeepsTrustedStoredSystemForGenericExtension(t *testing.T) {
	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename:             "Wario Land II.srm",
		DisplayTitle:         "Wario Land II",
		Payload:              []byte("trusted-stored-save"),
		DeclaredSystemSlug:   "gameboy",
		DeclaredSystem:       &system{ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy"},
		DeclaredFallbackOnly: true,
		TrustedStoredSystem:  true,
	})
	if detected.Slug != "gameboy" {
		t.Fatalf("expected trusted stored system to remain gameboy, got %q", detected.Slug)
	}
	if detected.System == nil || detected.System.Slug != "gameboy" {
		t.Fatalf("expected Nintendo Game Boy system details, got %#v", detected.System)
	}
}

func TestDetectSaveSystemRejectsGenericFallbackSlotTitles(t *testing.T) {
	tests := []struct {
		filename string
		title    string
	}{
		{filename: "Autosave6.sav", title: "Autosave6"},
		{filename: "01 - Channel 9 Headquarters.sav", title: "01 - Channel 9 Headquarters"},
	}

	for _, tc := range tests {
		detected := detectSaveSystem(saveSystemDetectionInput{
			Filename:             tc.filename,
			DisplayTitle:         tc.title,
			Payload:              make([]byte, 8192),
			DeclaredSystemSlug:   "nds",
			DeclaredSystem:       &system{ID: 900004, Name: "Nintendo DS", Slug: "nds"},
			DeclaredFallbackOnly: true,
		})
		if detected.System != nil || detected.Slug != "unknown-system" {
			t.Fatalf("expected %q to be rejected as fallback noise, got slug=%q system=%#v", tc.filename, detected.Slug, detected.System)
		}
		if !detected.Noise {
			t.Fatalf("expected %q to be flagged as noise", tc.filename)
		}
	}
}

func TestDetectSaveSystemRejectsStoredFallbackArcadeWithoutMachineEvidence(t *testing.T) {
	tests := []struct {
		filename string
		title    string
	}{
		{filename: "daytona.ram", title: "daytona"},
		{filename: "Ghost House.sav", title: "Ghost House"},
		{filename: "Arcade.sav", title: "Arcade"},
	}

	for _, tc := range tests {
		detected := detectSaveSystem(saveSystemDetectionInput{
			Filename:             tc.filename,
			DisplayTitle:         tc.title,
			Payload:              make([]byte, 16384),
			DeclaredSystemSlug:   "arcade",
			DeclaredSystem:       &system{ID: 900001, Name: "Arcade", Slug: "arcade"},
			DeclaredFallbackOnly: true,
		})
		if detected.System != nil || detected.Slug != "unknown-system" {
			t.Fatalf("expected %q to be rejected without arcade machine evidence, got slug=%q system=%#v", tc.filename, detected.Slug, detected.System)
		}
		if !detected.Noise {
			t.Fatalf("expected %q to be flagged as noise", tc.filename)
		}
	}
}

func TestDetectSaveSystemDoesNotPromoteNeoGeoFromPayloadAlone(t *testing.T) {
	payload := make([]byte, neoGeoCompoundSaveSize)
	for i := 0; i < neoGeoSaveRAMSize; i++ {
		payload[i] = 0xff
	}
	for i := neoGeoSaveRAMSize; i < len(payload); i += 2 {
		payload[i] = byte((i / 2) % 251)
	}

	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename:     "doubledr.sav",
		DisplayTitle: "doubledr",
		Payload:      payload,
	})
	if detected.System != nil || detected.Slug != "unknown-system" {
		t.Fatalf("expected neo geo payload without trusted console evidence to stay unknown, got slug=%q system=%#v", detected.Slug, detected.System)
	}
}

func TestDetectSaveSystemDoesNotPromoteNeoGeoWithoutKnownSetName(t *testing.T) {
	payload := make([]byte, neoGeoCompoundSaveSize)
	for i := 0; i < neoGeoSaveRAMSize; i++ {
		payload[i] = 0xff
	}
	for i := neoGeoSaveRAMSize; i < len(payload); i += 2 {
		payload[i] = byte((i / 2) % 251)
	}

	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename:     "mystery.sav",
		DisplayTitle: "mystery",
		Payload:      payload,
	})
	if detected.System != nil || detected.Slug != "unknown-system" {
		t.Fatalf("expected unknown system without trusted neo geo evidence, got slug=%q system=%#v", detected.Slug, detected.System)
	}
}

func TestDetectSaveSystemRecognizesNewSuperMarioBrosNDSSignature(t *testing.T) {
	payload := make([]byte, 8192)
	copy(payload[2:], []byte("Mario2d"))
	copy(payload[258:], []byte("Mario2d"))
	copy(payload[898:], []byte("Mario2d"))
	copy(payload[1538:], []byte("Mario2d"))
	copy(payload[4098:], []byte("Mario2d"))
	copy(payload[4354:], []byte("Mario2d"))

	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename:             "New Super Mario Bros. (USA).sav",
		DisplayTitle:         "New Super Mario Bros.",
		Payload:              payload,
		DeclaredSystemSlug:   "gameboy",
		DeclaredSystem:       &system{ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy"},
		DeclaredFallbackOnly: true,
		TrustedStoredSystem:  true,
	})
	if detected.Slug != "nds" {
		t.Fatalf("expected NSMB DS payload to override stale gameboy fallback, got %q", detected.Slug)
	}
	if detected.System == nil || detected.System.Slug != "nds" {
		t.Fatalf("expected Nintendo DS system details, got %#v", detected.System)
	}
}

func TestDetectSaveSystemKeepsDedicatedArcadeExtension(t *testing.T) {
	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename:     "outrun.nvram",
		DisplayTitle: "OutRun",
		Payload:      make([]byte, 8192),
	})
	if detected.Slug != "arcade" {
		t.Fatalf("expected arcade for dedicated arcade extension, got %q", detected.Slug)
	}
	if detected.System == nil || detected.System.Slug != "arcade" {
		t.Fatalf("expected arcade system details, got %#v", detected.System)
	}
}

func TestFallbackGameFromFilenameDoesNotInjectConsoleFromTitle(t *testing.T) {
	game := fallbackGameFromFilename("Wario Land II.srm")
	if game.System != nil {
		t.Fatalf("expected filename fallback to avoid setting a console, got %#v", game.System)
	}
	if game.Name != "Wario Land II" {
		t.Fatalf("expected cleaned fallback title, got %q", game.Name)
	}
}
