package main

import (
	"os"
	"path/filepath"
	"strings"
)

type normalizedSaveInputResult struct {
	Input        saveCreateInput
	Detection    saveSystemDetectionResult
	Rejected     bool
	RejectReason string
}

type normalizeSaveInputOptions struct {
	StoredSystemFallbackOnly bool
}

func (a *app) normalizeSaveInput(input saveCreateInput) saveCreateInput {
	return a.normalizeSaveInputDetailed(input).Input
}

func (a *app) normalizeSaveInputDetailed(input saveCreateInput) normalizedSaveInputResult {
	return a.normalizeSaveInputDetailedWithOptions(input, normalizeSaveInputOptions{})
}

func (a *app) normalizeSaveInputDetailedWithOptions(input saveCreateInput, options normalizeSaveInputOptions) normalizedSaveInputResult {
	input.Filename = safeFilename(input.Filename)
	if strings.TrimSpace(input.Format) == "" {
		input.Format = inferSaveFormat(input.Filename)
	}

	detection := detectSaveSystem(saveSystemDetectionInput{
		Filename:             input.Filename,
		DisplayTitle:         firstNonEmpty(input.DisplayTitle, input.Game.DisplayTitle, input.Game.Name, strings.TrimSuffix(input.Filename, filepath.Ext(input.Filename))),
		Payload:              input.Payload,
		DeclaredSystemSlug:   input.SystemSlug,
		DeclaredSystem:       input.Game.System,
		DeclaredFallbackOnly: options.StoredSystemFallbackOnly,
	})

	normalized := deriveNormalizedSaveMetadata(input, input.Filename, detection)
	input.DisplayTitle = normalized.DisplayTitle
	input.RegionCode = normalizeRegionCode(normalized.RegionCode)
	input.RegionFlag = regionFlagFromCode(input.RegionCode)
	input.LanguageCodes = normalizeLanguageCodes(normalized.LanguageCodes)
	input.SystemPath = normalized.SystemPath
	input.GamePath = normalized.GamePath
	input.MemoryCard = normalized.MemoryCard
	if normalized.System != nil {
		input.Game.System = normalized.System
		input.SystemSlug = normalized.System.Slug
	} else {
		input.Game.System = nil
		input.SystemSlug = "unknown-system"
	}

	if input.RegionCode == regionUnknown {
		productCode := deriveProductCodeFromPayload(input.Payload)
		if regionFromCode := regionFromProductCode(productCode); normalizeRegionCode(regionFromCode) != regionUnknown {
			input.RegionCode = normalizeRegionCode(regionFromCode)
			input.RegionFlag = regionFlagFromCode(input.RegionCode)
		}
	}

	if a != nil && a.enricher != nil && normalized.System != nil && !normalized.IsPSMemoryCard {
		systemName := normalized.System.Name
		enriched := a.enricher.enrich(input.DisplayTitle, systemName, input.RegionCode)
		if normalizeRegionCode(input.RegionCode) == regionUnknown && normalizeRegionCode(enriched.RegionCode) != regionUnknown {
			input.RegionCode = normalizeRegionCode(enriched.RegionCode)
			input.RegionFlag = regionFlagFromCode(input.RegionCode)
		}
		if strings.TrimSpace(enriched.CoverArtURL) != "" {
			input.CoverArtURL = strings.TrimSpace(enriched.CoverArtURL)
		}
	}

	if strings.TrimSpace(input.CoverArtURL) == "" {
		input.CoverArtURL = strings.TrimSpace(input.Game.CoverArtURL)
	}
	if strings.TrimSpace(input.CoverArtURL) == "" && input.Game.BoxartThumb != nil {
		input.CoverArtURL = strings.TrimSpace(*input.Game.BoxartThumb)
	}
	if strings.TrimSpace(input.CoverArtURL) == "" && input.Game.Boxart != nil {
		input.CoverArtURL = strings.TrimSpace(*input.Game.Boxart)
	}

	rejectReason := ""
	rejected := false
	if normalized.ArtifactKind == saveArtifactUnsupported {
		rejected = true
		rejectReason = playStationRejectReason(normalized.System)
	}

	track := canonicalTrackFromInput(input)
	input.SystemSlug = canonicalSegment(track.SystemSlug, "unknown-system")
	input.DisplayTitle = track.DisplayTitle
	input.RegionCode = canonicalRegion(input.RegionCode, track.RegionCode)
	input.RegionFlag = regionFlagFromCode(input.RegionCode)
	input.Game.System = track.System
	input.SystemPath = sanitizeDisplayPathSegment(func() string {
		if track.System != nil {
			return track.System.Name
		}
		return "Unknown System"
	}(), "Unknown System")
	input.GamePath = sanitizeDisplayPathSegment(track.DisplayTitle, "Unknown Game")
	input.GameSlug = canonicalGameSlugForTrack(track)
	input.Game.ID = canonicalGameIDForTrack(track)
	input.Game.Name = track.DisplayTitle
	input.Game.DisplayTitle = track.DisplayTitle
	input.Game.RegionCode = input.RegionCode
	input.Game.RegionFlag = input.RegionFlag
	input.Game.LanguageCodes = input.LanguageCodes
	input.Game.CoverArtURL = input.CoverArtURL
	if strings.TrimSpace(input.CoverArtURL) != "" {
		thumb := input.CoverArtURL
		box := input.CoverArtURL
		input.Game.BoxartThumb = &thumb
		input.Game.Boxart = &box
	}

	if detection.System == nil || !isSupportedSystemSlug(input.SystemSlug) {
		rejected = true
		if rejectReason == "" {
			if strings.TrimSpace(detection.Reason) != "" {
				rejectReason = detection.Reason
			} else {
				rejectReason = errUnsupportedSaveFormat.Error()
			}
		}
	}
	if rejected {
		input.SystemSlug = "unknown-system"
		input.Game.System = nil
		input.MemoryCard = nil
	}

	return normalizedSaveInputResult{
		Input:        input,
		Detection:    detection,
		Rejected:     rejected,
		RejectReason: rejectReason,
	}
}

func (a *app) decorateLoadedRecord(record *saveRecord) {
	if record == nil {
		return
	}

	var payload []byte
	if strings.TrimSpace(record.payloadPath) != "" {
		if data, err := os.ReadFile(record.payloadPath); err == nil {
			payload = data
		}
	}

	normalized := a.normalizeSaveInputDetailedWithOptions(saveCreateInput{
		Filename:      record.Summary.Filename,
		Payload:       payload,
		Game:          record.Summary.Game,
		Format:        record.Summary.Format,
		Metadata:      record.Summary.Metadata,
		ROMSHA1:       record.ROMSHA1,
		ROMMD5:        record.ROMMD5,
		SlotName:      record.SlotName,
		SystemSlug:    firstNonEmpty(record.SystemSlug, record.Summary.SystemSlug),
		GameSlug:      record.GameSlug,
		SystemPath:    record.SystemPath,
		GamePath:      record.GamePath,
		DisplayTitle:  record.Summary.DisplayTitle,
		RegionCode:    record.Summary.RegionCode,
		RegionFlag:    record.Summary.RegionFlag,
		LanguageCodes: record.Summary.LanguageCodes,
		CoverArtURL:   record.Summary.CoverArtURL,
		MemoryCard:    record.Summary.MemoryCard,
		CreatedAt:     record.Summary.CreatedAt,
	}, normalizeSaveInputOptions{StoredSystemFallbackOnly: true})

	updated := applyNormalizedSaveToRecord(*record, normalized.Input)
	updated.payloadPath = record.payloadPath
	updated.dirPath = record.dirPath
	updated.PayloadFile = record.PayloadFile
	*record = updated
}
