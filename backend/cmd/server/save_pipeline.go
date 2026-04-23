package main

import (
	"fmt"
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
		TrustedHelperSystem:  input.TrustedHelperSystem,
		DeclaredFallbackOnly: options.StoredSystemFallbackOnly,
		TrustedStoredSystem:  metadataHasTrustedSystemEvidence(input.Metadata),
	})
	input.Metadata = mergeSystemDetectionMetadata(input.Metadata, detection)

	normalized := deriveNormalizedSaveMetadata(input, input.Filename, detection)
	input.DisplayTitle = normalized.DisplayTitle
	input.RegionCode = normalizeRegionCode(normalized.RegionCode)
	input.RegionFlag = regionFlagFromCode(input.RegionCode)
	input.LanguageCodes = normalizeLanguageCodes(normalized.LanguageCodes)
	input.SystemPath = normalized.SystemPath
	input.GamePath = normalized.GamePath
	if normalized.Metadata != nil {
		input.Metadata = normalized.Metadata
	}
	if normalized.Dreamcast != nil {
		input.Dreamcast = normalized.Dreamcast
		input.Game.HasParser = true
	}
	if normalized.Saturn != nil {
		input.Saturn = normalized.Saturn
		input.Game.HasParser = true
	}
	if strings.TrimSpace(normalized.CoverArtURL) != "" {
		input.CoverArtURL = strings.TrimSpace(normalized.CoverArtURL)
	}
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

	if a != nil && a.enricher != nil && normalized.System != nil && !normalized.IsPSMemoryCard && strings.TrimSpace(input.CoverArtURL) == "" {
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
	consoleValidation := validateConsoleSpecificSave(input, detection, normalized)
	if consoleValidation.Inspection != nil {
		input.Inspection = consoleValidation.Inspection
		if input.Game.HasParser || consoleValidation.Inspection.ParserLevel == saveParserLevelStructural || consoleValidation.Inspection.ParserLevel == saveParserLevelSemantic {
			input.Game.HasParser = true
		}
		if title := strings.TrimSpace(consoleValidation.Inspection.ValidatedGameTitle); title != "" && !normalized.IsPSMemoryCard {
			input.DisplayTitle = canonicalDisplayTitle(title)
			input.Game.Name = input.DisplayTitle
			input.Game.DisplayTitle = input.DisplayTitle
		}
	}
	decorateN64ProjectionFields(&input)
	if consoleValidation.Rejected {
		rejected = true
		if rejectReason == "" {
			rejectReason = consoleValidation.RejectReason
		}
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
	if placeholderReason := projectionPlaceholderRejectReason(input); placeholderReason != "" {
		rejected = true
		if rejectReason == "" {
			rejectReason = placeholderReason
		}
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
		input.Dreamcast = nil
		input.Inspection = nil
	}

	return normalizedSaveInputResult{
		Input:        input,
		Detection:    detection,
		Rejected:     rejected,
		RejectReason: rejectReason,
	}
}

func projectionPlaceholderRejectReason(input saveCreateInput) string {
	systemSlug := canonicalSegment(firstNonEmpty(input.SystemSlug, func() string {
		if input.Game.System != nil {
			return input.Game.System.Slug
		}
		return ""
	}()), "")
	if systemSlug == "" || systemSlug == "psx" || systemSlug == "ps2" {
		return ""
	}

	profile := canonicalRuntimeProfile(systemSlug, firstNonEmpty(input.RuntimeProfile, input.SourceArtifactProfile))
	if profile == "" {
		return ""
	}
	if input.Inspection != nil && strings.TrimSpace(input.Inspection.ValidatedGameTitle) != "" {
		return ""
	}

	nonEmpty := 0
	for _, candidate := range []string{
		input.DisplayTitle,
		input.Game.DisplayTitle,
		input.Game.Name,
		strings.TrimSuffix(input.Filename, filepath.Ext(input.Filename)),
	} {
		normalized := canonicalSegment(candidate, "")
		if normalized == "" {
			continue
		}
		nonEmpty++
		if !isProjectionPlaceholderTitle(systemSlug, profile, normalized) {
			return ""
		}
	}
	if nonEmpty == 0 {
		return ""
	}

	return fmt.Sprintf("%s save title resolves to a runtime profile placeholder, not a verified game title", systemSlug)
}

func isProjectionPlaceholderTitle(systemSlug, profile, normalizedTitle string) bool {
	normalizedTitle = canonicalSegment(normalizedTitle, "")
	if normalizedTitle == "" {
		return false
	}
	if !strings.HasPrefix(normalizedTitle, "profile-") {
		return false
	}

	shortProfile := strings.TrimPrefix(canonicalRuntimeProfile(systemSlug, profile), profileFamilyPrefix(systemSlug))
	shortProfile = canonicalSegment(shortProfile, "")
	fullProfile := canonicalSegment(strings.ReplaceAll(profile, "/", "-"), "")
	if shortProfile != "" && normalizedTitle == "profile-"+shortProfile {
		return true
	}
	if fullProfile != "" && normalizedTitle == "profile-"+fullProfile {
		return true
	}
	return false
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
		Filename:              record.Summary.Filename,
		Payload:               payload,
		Game:                  record.Summary.Game,
		Format:                record.Summary.Format,
		Metadata:              record.Summary.Metadata,
		ROMSHA1:               record.ROMSHA1,
		ROMMD5:                record.ROMMD5,
		SlotName:              record.SlotName,
		SystemSlug:            firstNonEmpty(record.SystemSlug, record.Summary.SystemSlug),
		GameSlug:              record.GameSlug,
		SystemPath:            record.SystemPath,
		GamePath:              record.GamePath,
		DisplayTitle:          record.Summary.DisplayTitle,
		RegionCode:            record.Summary.RegionCode,
		RegionFlag:            record.Summary.RegionFlag,
		LanguageCodes:         record.Summary.LanguageCodes,
		CoverArtURL:           record.Summary.CoverArtURL,
		MemoryCard:            record.Summary.MemoryCard,
		Dreamcast:             record.Summary.Dreamcast,
		Saturn:                record.Summary.Saturn,
		Inspection:            record.Summary.Inspection,
		MediaType:             record.Summary.MediaType,
		ProjectionCapable:     record.Summary.ProjectionCapable,
		SourceArtifactProfile: record.Summary.SourceArtifactProfile,
		RuntimeProfile:        record.Summary.RuntimeProfile,
		CardSlot:              record.Summary.CardSlot,
		ProjectionID:          record.Summary.ProjectionID,
		SourceImportID:        record.Summary.SourceImportID,
		Portable:              record.Summary.Portable,
		CreatedAt:             record.Summary.CreatedAt,
	}, normalizeSaveInputOptions{StoredSystemFallbackOnly: true})

	updated := applyNormalizedSaveToRecord(*record, normalized.Input)
	updated.payloadPath = record.payloadPath
	updated.dirPath = record.dirPath
	updated.PayloadFile = record.PayloadFile
	if cheats := a.cheatService(); cheats != nil {
		updated.Summary.Cheats = cheats.capabilityForRecord(updated)
	}
	*record = updated
}
