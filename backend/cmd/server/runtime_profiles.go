package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type downloadProfile struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	TargetExtension string `json:"targetExtension,omitempty"`
	Note            string `json:"note,omitempty"`
}

type runtimeProfileDefinition struct {
	ID              string
	SystemSlug      string
	Label           string
	TargetExtension string
	Note            string
}

var runtimeProfileDefinitions = []runtimeProfileDefinition{
	{ID: "psx/mister", SystemSlug: "psx", Label: "MiSTer", TargetExtension: ".mcr", Note: "PlayStation memory card image"},
	{ID: "psx/retroarch", SystemSlug: "psx", Label: "RetroArch", TargetExtension: ".mcr", Note: "PlayStation memory card image"},
	{ID: "ps2/pcsx2", SystemSlug: "ps2", Label: "PCSX2", TargetExtension: ".ps2", Note: "PlayStation 2 memory card image"},

	{ID: n64ProfileMister, SystemSlug: "n64", Label: "MiSTer", Note: "Nintendo 64 save projection"},
	{ID: n64ProfileRetroArch, SystemSlug: "n64", Label: "RetroArch", TargetExtension: ".srm", Note: "Nintendo 64 SRM projection"},
	{ID: n64ProfileProject64, SystemSlug: "n64", Label: "Project64", Note: "Nintendo 64 emulator projection"},
	{ID: n64ProfileMupenFamily, SystemSlug: "n64", Label: "Mupen / RMG", Note: "Nintendo 64 emulator projection"},
	{ID: n64ProfileEverDrive, SystemSlug: "n64", Label: "EverDrive", Note: "Flash cart projection"},

	{ID: "saturn/mister", SystemSlug: "saturn", Label: "MiSTer", TargetExtension: ".sav", Note: "Combined Saturn backup RAM image"},
	{ID: "saturn/internal-raw", SystemSlug: "saturn", Label: "Internal Raw", TargetExtension: ".bkr", Note: "Internal Saturn backup RAM"},
	{ID: "saturn/cartridge-raw", SystemSlug: "saturn", Label: "Cartridge Raw", TargetExtension: ".bcr", Note: "Cartridge backup RAM"},
	{ID: "saturn/mednafen", SystemSlug: "saturn", Label: "Mednafen", Note: "Mednafen-compatible backup RAM export"},
	{ID: "saturn/mednafen-internal", SystemSlug: "saturn", Label: "Mednafen Internal", TargetExtension: ".bkr", Note: "Internal Mednafen backup RAM"},
	{ID: "saturn/mednafen-cartridge", SystemSlug: "saturn", Label: "Mednafen Cartridge", TargetExtension: ".bcr.gz", Note: "Compressed cartridge backup RAM"},
	{ID: "saturn/yabause", SystemSlug: "saturn", Label: "Yabause", TargetExtension: ".sav", Note: "Byte-expanded Saturn backup RAM"},
	{ID: "saturn/yabasanshiro", SystemSlug: "saturn", Label: "Yaba Sanshiro", TargetExtension: ".sav", Note: "Extended internal backup RAM export"},
	{ID: "saturn/bup", SystemSlug: "saturn", Label: "BUP", TargetExtension: ".bup", Note: "Single Saturn entry export"},
	{ID: "saturn/ymir", SystemSlug: "saturn", Label: "Ymir BUP", TargetExtension: ".bup", Note: "Single Saturn entry export"},
	{ID: "saturn/ymbp", SystemSlug: "saturn", Label: "YmBP", TargetExtension: ".ymbp", Note: "Single Saturn entry export"},

	{ID: "snes/snes9x", SystemSlug: "snes", Label: "Snes9x", TargetExtension: ".srm", Note: "Raw SNES SRAM"},
	{ID: "snes/bsnes", SystemSlug: "snes", Label: "bsnes", TargetExtension: ".srm", Note: "Raw SNES SRAM"},
	{ID: "snes/retroarch-snes9x", SystemSlug: "snes", Label: "RetroArch (Snes9x)", TargetExtension: ".srm", Note: "Raw SNES SRAM"},
	{ID: "snes/mesen2", SystemSlug: "snes", Label: "Mesen 2", TargetExtension: ".srm", Note: "Raw SNES SRAM"},
	{ID: "snes/higan", SystemSlug: "snes", Label: "higan", TargetExtension: ".srm", Note: "Raw SNES SRAM"},

	{ID: "nes/mesen2", SystemSlug: "nes", Label: "Mesen 2", TargetExtension: ".sav", Note: "Raw NES save RAM"},
	{ID: "nes/fceux", SystemSlug: "nes", Label: "FCEUX", TargetExtension: ".sav", Note: "Raw NES save RAM"},
	{ID: "nes/nestopia-ue", SystemSlug: "nes", Label: "Nestopia UE", TargetExtension: ".sav", Note: "Raw NES save RAM"},
	{ID: "nes/punes", SystemSlug: "nes", Label: "puNES", TargetExtension: ".sav", Note: "Raw NES save RAM"},
	{ID: "nes/retroarch-nestopia", SystemSlug: "nes", Label: "RetroArch (Nestopia)", TargetExtension: ".sav", Note: "Raw NES save RAM"},
	{ID: "nes/retroarch-fceumm", SystemSlug: "nes", Label: "RetroArch (FCEUmm)", TargetExtension: ".sav", Note: "Raw NES save RAM"},

	{ID: "gba/mgba", SystemSlug: "gba", Label: "mGBA", TargetExtension: ".sav", Note: "Raw Game Boy Advance save"},
	{ID: "gba/vba-m", SystemSlug: "gba", Label: "VBA-M", TargetExtension: ".sav", Note: "Raw Game Boy Advance save"},
	{ID: "gba/nocashgba", SystemSlug: "gba", Label: "No$GBA", TargetExtension: ".sav", Note: "Raw Game Boy Advance save"},
	{ID: "gba/skyemu", SystemSlug: "gba", Label: "SkyEmu", TargetExtension: ".sav", Note: "Raw Game Boy Advance save"},
	{ID: "gba/retroarch-mgba", SystemSlug: "gba", Label: "RetroArch (mGBA)", TargetExtension: ".sav", Note: "Raw Game Boy Advance save"},
	{ID: "gba/retroarch-vbam", SystemSlug: "gba", Label: "RetroArch (VBA-M)", TargetExtension: ".sav", Note: "Raw Game Boy Advance save"},

	{ID: "sms/emulicious", SystemSlug: "master-system", Label: "Emulicious", TargetExtension: ".sav", Note: "Raw Master System SRAM"},
	{ID: "sms/meka", SystemSlug: "master-system", Label: "MEKA", TargetExtension: ".sav", Note: "Raw Master System SRAM"},
	{ID: "sms/genesis-plus-gx", SystemSlug: "master-system", Label: "Genesis Plus GX", TargetExtension: ".sav", Note: "Raw Master System SRAM"},
	{ID: "sms/retroarch-gearsystem", SystemSlug: "master-system", Label: "RetroArch (Gearsystem)", TargetExtension: ".sav", Note: "Raw Master System SRAM"},
	{ID: "sms/retroarch-genesis-plus-gx", SystemSlug: "master-system", Label: "RetroArch (Genesis Plus GX)", TargetExtension: ".sav", Note: "Raw Master System SRAM"},

	{ID: "genesis/blastem", SystemSlug: "genesis", Label: "BlastEm", TargetExtension: ".srm", Note: "Raw Genesis SRAM"},
	{ID: "genesis/genesis-plus-gx", SystemSlug: "genesis", Label: "Genesis Plus GX", TargetExtension: ".srm", Note: "Raw Genesis SRAM"},
	{ID: "genesis/picodrive", SystemSlug: "genesis", Label: "PicoDrive", TargetExtension: ".srm", Note: "Raw Genesis SRAM"},
	{ID: "genesis/retroarch-genesis-plus-gx", SystemSlug: "genesis", Label: "RetroArch (Genesis Plus GX)", TargetExtension: ".srm", Note: "Raw Genesis SRAM"},
	{ID: "genesis/retroarch-picodrive", SystemSlug: "genesis", Label: "RetroArch (PicoDrive)", TargetExtension: ".srm", Note: "Raw Genesis SRAM"},

	{ID: "gamegear/emulicious", SystemSlug: "game-gear", Label: "Emulicious", TargetExtension: ".sav", Note: "Raw Game Gear SRAM"},
	{ID: "gamegear/gearsystem", SystemSlug: "game-gear", Label: "Gearsystem", TargetExtension: ".sav", Note: "Raw Game Gear SRAM"},
	{ID: "gamegear/genesis-plus-gx", SystemSlug: "game-gear", Label: "Genesis Plus GX", TargetExtension: ".sav", Note: "Raw Game Gear SRAM"},
	{ID: "gamegear/retroarch-gearsystem", SystemSlug: "game-gear", Label: "RetroArch (Gearsystem)", TargetExtension: ".sav", Note: "Raw Game Gear SRAM"},
	{ID: "gamegear/retroarch-genesis-plus-gx", SystemSlug: "game-gear", Label: "RetroArch (Genesis Plus GX)", TargetExtension: ".sav", Note: "Raw Game Gear SRAM"},

	{ID: "dreamcast/mister", SystemSlug: "dreamcast", Label: "MiSTer", TargetExtension: ".bin", Note: "Dreamcast VMU image"},
	{ID: "dreamcast/flycast", SystemSlug: "dreamcast", Label: "Flycast", Note: "Dreamcast validated container"},
	{ID: "dreamcast/redream", SystemSlug: "dreamcast", Label: "Redream", Note: "Dreamcast validated container"},
	{ID: "dreamcast/retroarch-flycast", SystemSlug: "dreamcast", Label: "RetroArch (Flycast)", Note: "Dreamcast validated container"},
}

var runtimeProfilesByID = func() map[string]runtimeProfileDefinition {
	out := make(map[string]runtimeProfileDefinition, len(runtimeProfileDefinitions))
	for _, definition := range runtimeProfileDefinitions {
		out[definition.ID] = definition
	}
	return out
}()

func requestedRuntimeProfile(values url.Values, systemSlug string) string {
	return canonicalRuntimeProfile(systemSlug, firstNonEmpty(
		values.Get("runtimeProfile"),
		values.Get("n64Profile"),
		values.Get("saturnFormat"),
	))
}

func requestedRuntimeProfileFromForm(formValue func(string) string, systemSlug string) string {
	if formValue == nil {
		return ""
	}
	return canonicalRuntimeProfile(systemSlug, firstNonEmpty(
		formValue("runtimeProfile"),
		formValue("n64Profile"),
		formValue("saturnFormat"),
	))
}

func isProjectionCapableSystem(systemSlug string) bool {
	switch canonicalSegment(systemSlug, "") {
	case "psx", "ps2", "n64", "saturn", "snes", "nes", "gba", "master-system", "genesis", "game-gear", "dreamcast":
		return true
	default:
		return false
	}
}

func requiresRuntimeProfileForHelper(systemSlug string, helper bool) bool {
	return helper && isProjectionCapableSystem(systemSlug)
}

func runtimeProfilesForSystem(systemSlug string) []runtimeProfileDefinition {
	systemSlug = canonicalSegment(systemSlug, "")
	out := make([]runtimeProfileDefinition, 0, 8)
	for _, definition := range runtimeProfileDefinitions {
		if definition.SystemSlug == systemSlug {
			out = append(out, definition)
		}
	}
	return out
}

func canonicalRuntimeProfile(systemSlug, requested string) string {
	systemSlug = strings.ToLower(strings.TrimSpace(systemSlug))
	clean := strings.TrimSpace(requested)
	if clean == "" {
		return ""
	}
	if systemSlug == "n64" {
		if profile := canonicalN64Profile(clean); profile != "" {
			return profile
		}
	}
	if systemSlug == "saturn" {
		profile := strings.ToLower(strings.TrimSpace(clean))
		if strings.HasPrefix(profile, "saturn/") {
			if _, ok := runtimeProfilesByID[profile]; ok {
				return profile
			}
		}
		alias := "saturn/" + profile
		if _, ok := runtimeProfilesByID[alias]; ok {
			return alias
		}
	}
	clean = strings.ToLower(strings.TrimSpace(clean))
	if definition, ok := runtimeProfilesByID[clean]; ok {
		if systemSlug == "" || definition.SystemSlug == systemSlug {
			return definition.ID
		}
	}
	if systemSlug != "" && !strings.Contains(clean, "/") {
		for _, definition := range runtimeProfilesForSystem(systemSlug) {
			if strings.EqualFold(definition.ID, systemSlug+"/"+clean) || strings.EqualFold(strings.TrimPrefix(definition.ID, profileFamilyPrefix(systemSlug)), clean) {
				return definition.ID
			}
		}
	}
	return ""
}

func profileFamilyPrefix(systemSlug string) string {
	switch canonicalSegment(systemSlug, "") {
	case "master-system":
		return "sms/"
	case "game-gear":
		return "gamegear/"
	default:
		return canonicalSegment(systemSlug, "") + "/"
	}
}

func supportsRuntimeProfile(systemSlug, profile string) bool {
	profile = canonicalRuntimeProfile(systemSlug, profile)
	if profile == "" {
		return false
	}
	definition, ok := runtimeProfilesByID[profile]
	if !ok {
		return false
	}
	return definition.SystemSlug == canonicalSegment(systemSlug, "")
}

func applyProjectionUploadMetadata(input saveCreateInput, runtimeProfile string) saveCreateInput {
	capable := true
	input.ProjectionCapable = &capable
	input.SourceArtifactProfile = runtimeProfile
	input.RuntimeProfile = runtimeProfile
	return input
}

func normalizeProjectionUpload(input saveCreateInput, requestedProfile string) (saveCreateInput, error) {
	systemSlug := canonicalSegment(firstNonEmpty(input.SystemSlug, func() string {
		if input.Game.System != nil {
			return input.Game.System.Slug
		}
		return ""
	}()), "")
	profile := canonicalRuntimeProfile(systemSlug, requestedProfile)
	if profile == "" {
		return input, fmt.Errorf("runtimeProfile is required for %s helper uploads", systemSlug)
	}
	switch systemSlug {
	case "n64":
		return normalizeN64ProjectionUpload(input, profile)
	case "saturn", "snes", "nes", "gba", "master-system", "genesis", "game-gear", "dreamcast":
		return applyProjectionUploadMetadata(input, profile), nil
	default:
		return input, fmt.Errorf("runtimeProfile is not supported for %s saves", systemSlug)
	}
}

func downloadProfilesForSummary(summary saveSummary) []downloadProfile {
	if strings.TrimSpace(summary.LogicalKey) != "" {
		return []downloadProfile{originalDownloadProfile(summary)}
	}
	systemSlug := canonicalSegment(firstNonEmpty(summary.SystemSlug, func() string {
		if summary.Game.System != nil {
			return summary.Game.System.Slug
		}
		return ""
	}()), "")
	if !isProjectionCapableSystem(systemSlug) {
		return []downloadProfile{originalDownloadProfile(summary)}
	}
	definitions := compatibleDownloadProfiles(summary)
	if len(definitions) == 0 {
		return []downloadProfile{originalDownloadProfile(summary)}
	}
	profiles := make([]downloadProfile, 0, len(definitions))
	for _, definition := range definitions {
		profiles = append(profiles, downloadProfile{
			ID:              definition.ID,
			Label:           definition.Label,
			TargetExtension: runtimeProfileTargetExtension(summary, definition),
			Note:            definition.Note,
		})
	}
	return profiles
}

func compatibleDownloadProfiles(summary saveSummary) []runtimeProfileDefinition {
	systemSlug := canonicalSegment(firstNonEmpty(summary.SystemSlug, func() string {
		if summary.Game.System != nil {
			return summary.Game.System.Slug
		}
		return ""
	}()), "")
	definitions := runtimeProfilesForSystem(systemSlug)
	if len(definitions) == 0 {
		return nil
	}
	out := make([]runtimeProfileDefinition, 0, len(definitions))
	for _, definition := range definitions {
		switch systemSlug {
		case "dreamcast":
			if dreamcastRuntimeProfileCompatible(summary, definition.ID) {
				out = append(out, definition)
			}
		case "saturn":
			if saturnRuntimeProfileCompatible(summary, definition.ID) {
				out = append(out, definition)
			}
		default:
			out = append(out, definition)
		}
	}
	return out
}

func originalDownloadProfile(summary saveSummary) downloadProfile {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(summary.Filename)))
	return downloadProfile{
		ID:              "original",
		Label:           "Original file",
		TargetExtension: ext,
		Note:            "Stored payload without projection",
	}
}

func runtimeProfileTargetExtension(summary saveSummary, definition runtimeProfileDefinition) string {
	if definition.TargetExtension != "" {
		return definition.TargetExtension
	}
	if definition.SystemSlug == "dreamcast" {
		switch strings.ToLower(strings.TrimSpace(definition.ID)) {
		case "dreamcast/mister":
			return ".bin"
		default:
			container := ""
			if summary.Dreamcast != nil {
				container = strings.ToLower(strings.TrimSpace(summary.Dreamcast.Container))
			}
			switch container {
			case "vms":
				return ".vms"
			case "dci":
				return ".dci"
			default:
				return ".bin"
			}
		}
	}
	return strings.ToLower(strings.TrimSpace(filepath.Ext(summary.Filename)))
}

func dreamcastRuntimeProfileCompatible(summary saveSummary, profile string) bool {
	container := ""
	if summary.Dreamcast != nil {
		container = strings.ToLower(strings.TrimSpace(summary.Dreamcast.Container))
	}
	switch profile {
	case "dreamcast/mister":
		return container == "" || container == "bin"
	case "dreamcast/flycast", "dreamcast/redream", "dreamcast/retroarch-flycast":
		return container == "" || container == "bin" || container == "vms" || container == "dci"
	default:
		return false
	}
}

func saturnRuntimeProfileCompatible(summary saveSummary, profile string) bool {
	if !strings.HasPrefix(profile, "saturn/") {
		return false
	}
	format := strings.TrimPrefix(profile, "saturn/")
	switch format {
	case "bup", "ymir", "ymbp":
		if summary.Saturn == nil {
			return false
		}
		return len(summary.Saturn.Entries) == 1
	default:
		return true
	}
}

func projectPayloadForRuntime(a *app, record saveRecord, payload []byte, requestedProfile, saturnEntry string) (string, string, []byte, error) {
	systemSlug := canonicalSegment(saveRecordSystemSlug(record), "")
	profile := canonicalRuntimeProfile(systemSlug, requestedProfile)
	if strings.TrimSpace(requestedProfile) != "" && profile == "" {
		return "", "", nil, fmt.Errorf("unsupported runtimeProfile %q", requestedProfile)
	}
	if profile == "" || profile == "original" {
		return record.Summary.Filename, "application/octet-stream", payload, nil
	}
	switch systemSlug {
	case "n64":
		return projectN64Payload(record.Summary, payload, profile)
	case "saturn":
		return saturnDownloadPayload(record, payload, strings.TrimPrefix(profile, "saturn/"), saturnEntry)
	case "psx", "ps2":
		return a.projectPlayStationProjectionPayload(record, profile)
	case "dreamcast":
		return projectDreamcastPayload(record.Summary, payload, profile)
	case "snes", "nes", "gba", "master-system", "genesis", "game-gear":
		return projectIdentityRuntimePayload(record.Summary, payload, profile)
	default:
		return record.Summary.Filename, "application/octet-stream", payload, nil
	}
}

func (a *app) projectPlayStationProjectionPayload(record saveRecord, requestedProfile string) (string, string, []byte, error) {
	profile := canonicalRuntimeProfile(saveRecordSystemSlug(record), requestedProfile)
	if profile == "" {
		return "", "", nil, fmt.Errorf("unsupported runtimeProfile %q", requestedProfile)
	}
	currentProfile, cardSlot, _, ok := playStationProjectionInfoFromRecord(record)
	if !ok {
		return "", "", nil, fmt.Errorf("save is not a playstation projection")
	}
	targetRecord := record
	if profile != currentProfile {
		store := a.playStationSyncStore()
		if store == nil {
			return "", "", nil, fmt.Errorf("playstation store is not initialized")
		}
		saveID, _, exists := store.latestProjectionSaveRecord(profile, cardSlot)
		if !exists {
			return "", "", nil, fmt.Errorf("playstation projection %q is not available for %s", profile, cardSlot)
		}
		resolved, found := a.findSaveRecordByID(saveID)
		if !found {
			return "", "", nil, fmt.Errorf("playstation projection save record not found")
		}
		targetRecord = resolved
	}
	payload, err := os.ReadFile(targetRecord.payloadPath)
	if err != nil {
		return "", "", nil, err
	}
	return targetRecord.Summary.Filename, "application/octet-stream", payload, nil
}

func projectIdentityRuntimePayload(summary saveSummary, payload []byte, requestedProfile string) (string, string, []byte, error) {
	systemSlug := canonicalSegment(firstNonEmpty(summary.SystemSlug, func() string {
		if summary.Game.System != nil {
			return summary.Game.System.Slug
		}
		return ""
	}()), "")
	profile := canonicalRuntimeProfile(systemSlug, requestedProfile)
	definition, ok := runtimeProfilesByID[profile]
	if !ok || definition.SystemSlug != systemSlug {
		return "", "", nil, fmt.Errorf("unsupported runtimeProfile %q", requestedProfile)
	}
	stem := strings.TrimSpace(strings.TrimSuffix(summary.Filename, filepath.Ext(summary.Filename)))
	if stem == "" {
		stem = canonicalSegment(summary.DisplayTitle, systemSlug+"-save")
	}
	if stem == "" {
		stem = systemSlug + "-save"
	}
	ext := runtimeProfileTargetExtension(summary, definition)
	if ext == "" {
		ext = strings.TrimSpace(filepath.Ext(summary.Filename))
	}
	return safeFilename(stem + ext), "application/octet-stream", append([]byte(nil), payload...), nil
}

func projectDreamcastPayload(summary saveSummary, payload []byte, requestedProfile string) (string, string, []byte, error) {
	profile := canonicalRuntimeProfile("dreamcast", requestedProfile)
	if profile == "" {
		return "", "", nil, fmt.Errorf("unsupported runtimeProfile %q", requestedProfile)
	}
	if !dreamcastRuntimeProfileCompatible(summary, profile) {
		return "", "", nil, fmt.Errorf("runtimeProfile %q is not compatible with this Dreamcast container", requestedProfile)
	}
	stem := strings.TrimSpace(strings.TrimSuffix(summary.Filename, filepath.Ext(summary.Filename)))
	if stem == "" {
		stem = canonicalSegment(summary.DisplayTitle, "dreamcast-save")
	}
	if stem == "" {
		stem = "dreamcast-save"
	}
	definition := runtimeProfilesByID[profile]
	ext := runtimeProfileTargetExtension(summary, definition)
	if ext == "" {
		ext = strings.TrimSpace(filepath.Ext(summary.Filename))
	}
	return safeFilename(stem + ext), "application/octet-stream", append([]byte(nil), payload...), nil
}

func summaryWithDownloadProfiles(summary saveSummary) saveSummary {
	summary.DownloadProfiles = downloadProfilesForSummary(summary)
	return summary
}
