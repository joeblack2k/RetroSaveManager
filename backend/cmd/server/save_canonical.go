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
	IsPS1Card      bool
	MemoryCardName string
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
	if systemSlug == "psx" && input.MemoryCard != nil {
		track.IsPS1Card = true
		track.MemoryCardName = canonicalMemoryCardName(input.MemoryCard, input.SlotName, input.Filename)
		track.DisplayTitle = track.MemoryCardName
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
	if systemSlug == "psx" && summary.MemoryCard != nil {
		track.IsPS1Card = true
		track.MemoryCardName = canonicalMemoryCardName(summary.MemoryCard, "", summary.Filename)
		track.DisplayTitle = track.MemoryCardName
	}
	return track
}

func canonicalTrackFromRecord(record saveRecord) canonicalSaveTrack {
	return canonicalTrackFromSummary(record.Summary, record.SystemSlug)
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
	if track.IsPS1Card {
		cardName := strings.ToLower(strings.TrimSpace(spacePattern.ReplaceAllString(track.MemoryCardName, " ")))
		if cardName == "" {
			cardName = "memory card 1"
		}
		return systemSlug + "::memory-card::" + cardName
	}
	return systemSlug + "::" + canonicalTrackTitleKey(track.DisplayTitle) + "::" + normalizeRegionCode(track.RegionCode)
}

func canonicalGameSlugForTrack(track canonicalSaveTrack) string {
	if track.IsPS1Card {
		return canonicalSegment(track.MemoryCardName, "memory-card")
	}
	return canonicalSegment(track.DisplayTitle, "unknown-game")
}

func canonicalGameIDForTrack(track canonicalSaveTrack) int {
	return deterministicGameID("track:" + canonicalTrackKey(track))
}

func canonicalHistoryKeyForRecord(record saveRecord) string {
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
	return "track:" + canonicalTrackKey(canonicalTrackFromInput(input))
}
