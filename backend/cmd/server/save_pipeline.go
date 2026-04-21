package main

import "strings"

func (a *app) normalizeSaveInput(input saveCreateInput) saveCreateInput {
	input.Filename = safeFilename(input.Filename)
	if strings.TrimSpace(input.Format) == "" {
		input.Format = inferSaveFormat(input.Filename)
	}

	normalized := deriveNormalizedSaveMetadata(input, input.Filename)
	input.DisplayTitle = normalized.DisplayTitle
	input.RegionCode = normalizeRegionCode(normalized.RegionCode)
	input.RegionFlag = regionFlagFromCode(input.RegionCode)
	input.LanguageCodes = normalizeLanguageCodes(normalized.LanguageCodes)
	input.SystemPath = normalized.SystemPath
	input.GamePath = normalized.GamePath
	input.MemoryCard = normalized.MemoryCard
	if normalized.System != nil {
		input.Game.System = normalized.System
	}

	if strings.TrimSpace(input.SystemSlug) == "" && input.Game.System != nil {
		input.SystemSlug = canonicalSegment(input.Game.System.Slug, "unknown-system")
	}
	if input.Game.System != nil && strings.TrimSpace(input.Game.System.Slug) != "" {
		input.SystemSlug = canonicalSegment(input.Game.System.Slug, "unknown-system")
	}
	if strings.TrimSpace(input.SystemSlug) == "" {
		input.SystemSlug = canonicalSegment(normalized.SystemPath, "unknown-system")
	}

	if normalized.IsPSMemoryCard {
		input.GameSlug = canonicalSegment(input.GamePath, "memory-card")
	} else {
		input.GameSlug = canonicalSegment(input.DisplayTitle, "unknown-game")
	}

	input.Game.Name = input.DisplayTitle
	input.Game.DisplayTitle = input.DisplayTitle
	input.Game.RegionCode = input.RegionCode
	input.Game.RegionFlag = input.RegionFlag
	input.Game.LanguageCodes = input.LanguageCodes

	if input.Game.ID == 0 {
		if normalized.IsPSMemoryCard {
			input.Game.ID = deterministicGameID(input.SystemSlug + ":" + input.GamePath)
		} else {
			input.Game.ID = deterministicGameID(input.DisplayTitle)
		}
	}

	if input.RegionCode == regionUnknown {
		productCode := deriveProductCodeFromPayload(input.Payload)
		if regionFromCode := regionFromProductCode(productCode); normalizeRegionCode(regionFromCode) != regionUnknown {
			input.RegionCode = normalizeRegionCode(regionFromCode)
			input.RegionFlag = regionFlagFromCode(input.RegionCode)
			input.Game.RegionCode = input.RegionCode
			input.Game.RegionFlag = input.RegionFlag
		}
	}

	if a != nil && a.enricher != nil {
		systemName := ""
		if input.Game.System != nil {
			systemName = input.Game.System.Name
		}
		enriched := a.enricher.enrich(input.DisplayTitle, systemName, input.RegionCode)
		if normalizeRegionCode(input.RegionCode) == regionUnknown && normalizeRegionCode(enriched.RegionCode) != regionUnknown {
			input.RegionCode = normalizeRegionCode(enriched.RegionCode)
			input.RegionFlag = regionFlagFromCode(input.RegionCode)
			input.Game.RegionCode = input.RegionCode
			input.Game.RegionFlag = input.RegionFlag
		}
		if strings.TrimSpace(enriched.CoverArtURL) != "" {
			input.CoverArtURL = strings.TrimSpace(enriched.CoverArtURL)
			input.Game.CoverArtURL = input.CoverArtURL
			thumb := input.CoverArtURL
			box := input.CoverArtURL
			input.Game.BoxartThumb = &thumb
			input.Game.Boxart = &box
		}
	}

	return input
}

func (a *app) decorateLoadedRecord(record *saveRecord) {
	if record == nil {
		return
	}

	cleanTitle, regionFromTitle, languageCodesFromTitle := cleanupDisplayTitleRegionAndLanguages(record.Summary.DisplayTitle)
	if cleanTitle == "" || cleanTitle == "Unknown Game" {
		cleanTitle, regionFromTitle, languageCodesFromTitle = cleanupDisplayTitleRegionAndLanguages(record.Summary.Game.DisplayTitle)
	}
	if cleanTitle == "" || cleanTitle == "Unknown Game" {
		cleanTitle, regionFromTitle, languageCodesFromTitle = cleanupDisplayTitleRegionAndLanguages(record.Summary.Game.Name)
	}
	if cleanTitle == "" || cleanTitle == "Unknown Game" {
		cleanTitle, regionFromTitle, languageCodesFromTitle = cleanupDisplayTitleRegionAndLanguages(record.Summary.Filename)
	}
	if strings.TrimSpace(cleanTitle) == "" {
		cleanTitle = "Unknown Game"
	}
	record.Summary.DisplayTitle = cleanTitle

	record.Summary.RegionCode = normalizeRegionCode(record.Summary.RegionCode)
	if record.Summary.RegionCode == regionUnknown {
		record.Summary.RegionCode = normalizeRegionCode(record.Summary.Game.RegionCode)
	}
	if record.Summary.RegionCode == regionUnknown {
		record.Summary.RegionCode = normalizeRegionCode(regionFromTitle)
	}
	record.Summary.RegionFlag = regionFlagFromCode(record.Summary.RegionCode)
	record.Summary.LanguageCodes = normalizeLanguageCodes(record.Summary.LanguageCodes)
	if len(record.Summary.LanguageCodes) == 0 {
		record.Summary.LanguageCodes = normalizeLanguageCodes(record.Summary.Game.LanguageCodes)
	}
	if len(record.Summary.LanguageCodes) == 0 {
		record.Summary.LanguageCodes = normalizeLanguageCodes(languageCodesFromTitle)
	}
	if len(record.Summary.LanguageCodes) == 0 {
		_, _, detectedLangs := cleanupDisplayTitleRegionAndLanguages(record.Summary.Filename)
		record.Summary.LanguageCodes = normalizeLanguageCodes(detectedLangs)
	}
	record.Summary.Game.Name = record.Summary.DisplayTitle
	record.Summary.Game.DisplayTitle = record.Summary.DisplayTitle
	record.Summary.Game.RegionCode = record.Summary.RegionCode
	record.Summary.Game.RegionFlag = record.Summary.RegionFlag
	record.Summary.Game.LanguageCodes = record.Summary.LanguageCodes

	if a == nil || a.enricher == nil {
		return
	}
	if strings.TrimSpace(record.Summary.CoverArtURL) != "" && normalizeRegionCode(record.Summary.RegionCode) != regionUnknown {
		return
	}

	systemName := ""
	if record.Summary.Game.System != nil {
		systemName = record.Summary.Game.System.Name
	}
	enriched := a.enricher.enrich(record.Summary.DisplayTitle, systemName, record.Summary.RegionCode)
	if strings.TrimSpace(record.Summary.CoverArtURL) == "" && strings.TrimSpace(enriched.CoverArtURL) != "" {
		record.Summary.CoverArtURL = strings.TrimSpace(enriched.CoverArtURL)
		record.Summary.Game.CoverArtURL = record.Summary.CoverArtURL
		thumb := record.Summary.CoverArtURL
		box := record.Summary.CoverArtURL
		record.Summary.Game.BoxartThumb = &thumb
		record.Summary.Game.Boxart = &box
	}
	if normalizeRegionCode(record.Summary.RegionCode) == regionUnknown && normalizeRegionCode(enriched.RegionCode) != regionUnknown {
		record.Summary.RegionCode = normalizeRegionCode(enriched.RegionCode)
		record.Summary.RegionFlag = regionFlagFromCode(record.Summary.RegionCode)
		record.Summary.Game.RegionCode = record.Summary.RegionCode
		record.Summary.Game.RegionFlag = record.Summary.RegionFlag
	}
}
