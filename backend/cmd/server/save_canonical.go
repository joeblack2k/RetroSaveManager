package main

import (
	"path/filepath"
	"strings"
)

type canonicalSaveTrack struct {
	SystemSlug     string
	System         *system
	DisplayTitle   string
	RegionCode     string
	IsMemoryCard   bool
	MemoryCardName string
	RuntimeProfile string
	PortID         string
	SlotID         string
}

func canonicalSystemForSave(existing *system, fallbackSlug string) (string, *system) {
	for _, candidate := range []string{
		fallbackSlug,
		func() string {
			if existing == nil {
				return ""
			}
			return existing.Slug
		}(),
		func() string {
			if existing == nil {
				return ""
			}
			return existing.Name
		}(),
	} {
		if slug := supportedSystemSlugFromLabel(candidate); slug != "" {
			return slug, supportedSystemFromSlug(slug)
		}
	}
	return "unknown-system", nil
}

func canonicalDisplayTitle(raw ...string) string {
	for _, candidate := range raw {
		title, _, _ := cleanupDisplayTitleRegionAndLanguages(candidate)
		if strings.TrimSpace(title) != "" && title != "Unknown Game" {
			return title
		}
	}
	return "Unknown Game"
}

func canonicalRegion(raw ...string) string {
	for _, candidate := range raw {
		if normalized := normalizeRegionCode(candidate); normalized != regionUnknown {
			return normalized
		}
	}
	return regionUnknown
}

func canonicalMemoryCardName(card *memoryCardDetails, slotName, filename string) string {
	if card != nil && strings.TrimSpace(card.Name) != "" {
		return strings.TrimSpace(card.Name)
	}
	return deriveMemoryCardName(slotName, filename)
}

func canonicalTrackFromInput(input saveCreateInput) canonicalSaveTrack {
	systemSlug, sys := canonicalSystemForSave(input.Game.System, input.SystemSlug)
	displayTitle := canonicalDisplayTitle(
		input.DisplayTitle,
		input.Game.DisplayTitle,
		input.Game.Name,
		strings.TrimSuffix(input.Filename, filepath.Ext(input.Filename)),
	)
	track := canonicalSaveTrack{
		SystemSlug:   systemSlug,
		System:       sys,
		DisplayTitle: displayTitle,
		RegionCode: canonicalRegion(
			input.RegionCode,
			input.Game.RegionCode,
		),
	}
	if (systemSlug == "psx" || systemSlug == "ps2") && input.MemoryCard != nil {
		track.IsMemoryCard = true
		track.MemoryCardName = canonicalMemoryCardName(input.MemoryCard, input.SlotName, input.Filename)
		track.DisplayTitle = track.MemoryCardName
		track.RuntimeProfile = strings.TrimSpace(input.RuntimeProfile)
	}
	track = canonicalNativePortTrackFromInput(track, input)
	if systemSlug == nativePortSystemSlug {
		if title := strings.TrimSpace(input.DisplayTitle); title != "" {
			track.DisplayTitle = title
		}
	}
	return track
}

func canonicalTrackFromSummary(summary saveSummary, fallbackSystemSlug string) canonicalSaveTrack {
	systemSlug, sys := canonicalSystemForSave(summary.Game.System, firstNonEmpty(summary.SystemSlug, fallbackSystemSlug))
	displayTitle := canonicalDisplayTitle(
		summary.DisplayTitle,
		summary.Game.DisplayTitle,
		summary.Game.Name,
		strings.TrimSuffix(summary.Filename, filepath.Ext(summary.Filename)),
	)
	track := canonicalSaveTrack{
		SystemSlug:   systemSlug,
		System:       sys,
		DisplayTitle: displayTitle,
		RegionCode: canonicalRegion(
			summary.RegionCode,
			summary.Game.RegionCode,
		),
	}
	if (systemSlug == "psx" || systemSlug == "ps2") && summary.MemoryCard != nil {
		track.IsMemoryCard = true
		track.MemoryCardName = canonicalMemoryCardName(summary.MemoryCard, "", summary.Filename)
		track.DisplayTitle = track.MemoryCardName
		track.RuntimeProfile = strings.TrimSpace(summary.RuntimeProfile)
	}
	track = canonicalNativePortTrackFromSummary(track, summary, "")
	if systemSlug == nativePortSystemSlug {
		if title := strings.TrimSpace(summary.DisplayTitle); title != "" {
			track.DisplayTitle = title
		}
	}
	return track
}

func canonicalTrackFromRecord(record saveRecord) canonicalSaveTrack {
	return canonicalNativePortTrackFromSummary(canonicalTrackFromSummary(record.Summary, record.SystemSlug), record.Summary, record.SlotName)
}

func canonicalNativePortTrackFromInput(track canonicalSaveTrack, input saveCreateInput) canonicalSaveTrack {
	return canonicalNativePortTrack(track, input.RuntimeProfile, input.PortID, input.SlotID, input.SlotName, input.DisplayTitle)
}

func canonicalNativePortTrackFromSummary(track canonicalSaveTrack, summary saveSummary, fallbackSlotName string) canonicalSaveTrack {
	return canonicalNativePortTrack(track, summary.RuntimeProfile, summary.PortID, summary.SlotID, fallbackSlotName, summary.DisplayTitle)
}

func canonicalNativePortTrack(track canonicalSaveTrack, runtimeProfile, portID, slotID, slotName, explicitTitle string) canonicalSaveTrack {
	profile := strings.TrimSpace(runtimeProfile)
	cleanPortID := canonicalOptionalSegment(portID)
	manifest, hasManifest := canonicalNativePortManifest(profile, cleanPortID)
	if !hasManifest && cleanPortID == "" && !strings.HasPrefix(strings.ToLower(profile), "port/") && canonicalSegment(track.SystemSlug, "") != nativePortSystemSlug {
		return track
	}
	if hasManifest {
		profile = manifest.RuntimeProfile
		cleanPortID = manifest.ID
		if canonicalSegment(track.SystemSlug, "") != nativePortSystemSlug && strings.TrimSpace(explicitTitle) == "" && strings.TrimSpace(manifest.OriginGameTitle) != "" {
			track.DisplayTitle = manifest.OriginGameTitle
		}
	}
	if profile == "" && cleanPortID != "" {
		profile = "port/" + cleanPortID
	}
	track.RuntimeProfile = profile
	track.PortID = firstNonEmpty(cleanPortID, canonicalOptionalSegment(strings.TrimPrefix(profile, "port/")))
	track.SlotID = firstNonEmpty(canonicalOptionalSegment(slotID), canonicalOptionalSegment(slotName), "default")
	return track
}

func canonicalNativePortManifest(runtimeProfile, portID string) (nativePortManifest, bool) {
	for _, candidate := range []string{runtimeProfile, func() string {
		if portID == "" {
			return ""
		}
		return "port/" + portID
	}()} {
		if manifest, ok := nativePortManifestForRuntimeProfile(candidate); ok {
			return manifest, true
		}
	}
	return nativePortManifest{}, false
}

func canonicalTrackTitleKey(title string) string {
	clean := canonicalDisplayTitle(title)
	clean = strings.ToLower(strings.TrimSpace(spacePattern.ReplaceAllString(clean, " ")))
	if clean == "" {
		return "unknown game"
	}
	return clean
}

func canonicalTrackKey(track canonicalSaveTrack) string {
	systemSlug := canonicalSegment(track.SystemSlug, "unknown-system")
	if track.IsMemoryCard {
		cardName := strings.ToLower(strings.TrimSpace(spacePattern.ReplaceAllString(track.MemoryCardName, " ")))
		if cardName == "" {
			cardName = "memory card 1"
		}
		if runtimeProfile := canonicalOptionalSegment(track.RuntimeProfile); runtimeProfile != "" {
			return systemSlug + "::" + runtimeProfile + "::memory-card::" + cardName
		}
		return systemSlug + "::memory-card::" + cardName
	}
	if systemSlug == nativePortSystemSlug {
		portID := canonicalSegment(track.PortID, "unknown-port")
		slotID := canonicalSegment(track.SlotID, "default")
		return systemSlug + "::" + portID + "::" + slotID
	}
	return systemSlug + "::" + canonicalTrackTitleKey(track.DisplayTitle) + "::" + normalizeRegionCode(track.RegionCode)
}

func canonicalArtifactKeyForTrack(track canonicalSaveTrack) string {
	base := canonicalTrackKey(track)
	systemSlug := canonicalSegment(track.SystemSlug, "unknown-system")
	if track.IsMemoryCard || systemSlug == nativePortSystemSlug || !canonicalTrackHasNativePortIdentity(track) {
		return base
	}
	portID := canonicalOptionalSegment(firstNonEmpty(track.PortID, strings.TrimPrefix(track.RuntimeProfile, "port/")))
	if portID == "" {
		portID = "unknown-port"
	}
	return base + "::port::" + canonicalSegment(portID, "unknown-port") + "::slot::" + canonicalSegment(track.SlotID, "default")
}

func canonicalTrackHasNativePortIdentity(track canonicalSaveTrack) bool {
	if canonicalOptionalSegment(track.PortID) != "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(track.RuntimeProfile)), "port/")
}

func canonicalGameSlugForTrack(track canonicalSaveTrack) string {
	if track.IsMemoryCard {
		return canonicalSegment(track.MemoryCardName, "memory-card")
	}
	if canonicalSegment(track.SystemSlug, "") == nativePortSystemSlug {
		return canonicalSegment(firstNonEmpty(track.PortID, track.DisplayTitle), "unknown-port") + "-" + canonicalSegment(track.SlotID, "default")
	}
	return canonicalSegment(track.DisplayTitle, "unknown-game")
}

func canonicalGameIDForTrack(track canonicalSaveTrack) int {
	return deterministicGameID("track:" + canonicalTrackKey(track))
}

func canonicalHistoryKeyForRecord(record saveRecord) string {
	return canonicalArtifactKeyForTrack(canonicalTrackFromRecord(record))
}

func canonicalListKeyForRecord(record saveRecord) string {
	if runtimeProfile, cardSlot, _, ok := playStationProjectionInfoFromRecord(record); ok {
		_ = runtimeProfile
		systemSlug := canonicalOptionalSegment(saveRecordSystemSlug(record))
		if systemSlug == "" {
			systemSlug = canonicalOptionalSegment(record.Summary.SystemSlug)
		}
		if systemSlug == "" {
			systemSlug = "unknown-system"
		}
		return canonicalSegment(systemSlug, "unknown-system") + "::card::" + canonicalSegment(cardSlot, "memory-card-1")
	}
	return canonicalTrackKey(canonicalTrackFromRecord(record))
}

func canonicalVersionKeyForRecord(record saveRecord) string {
	if rom := strings.TrimSpace(record.ROMSHA1); rom != "" {
		return "rom:" + rom + "::slot:" + normalizedSlot(record.SlotName)
	}
	return "track:" + canonicalHistoryKeyForRecord(record)
}

func canonicalVersionKeyForInput(input saveCreateInput, filename string) string {
	if rom := strings.TrimSpace(input.ROMSHA1); rom != "" {
		return "rom:" + rom + "::slot:" + normalizedSlot(input.SlotName)
	}
	if strings.TrimSpace(input.Filename) == "" {
		input.Filename = filename
	}
	return "track:" + canonicalArtifactKeyForTrack(canonicalTrackFromInput(input))
}
