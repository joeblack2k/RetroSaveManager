package main

import (
	"fmt"
	"strings"
)

var strictSegaRawSaveSizes = map[int]struct{}{
	64:     {},
	128:    {},
	256:    {},
	512:    {},
	1024:   {},
	2048:   {},
	4096:   {},
	8192:   {},
	16384:  {},
	32768:  {},
	65536:  {},
	131072: {},
}

type consoleValidationResult struct {
	Inspection   *saveInspection
	Rejected     bool
	RejectReason string
}

func validateConsoleSpecificSave(input saveCreateInput, detection saveSystemDetectionResult, normalized normalizedSaveMetadata) consoleValidationResult {
	systemSlug := canonicalSegment(firstNonEmpty(func() string {
		if normalized.System == nil {
			return ""
		}
		return normalized.System.Slug
	}(), input.SystemSlug, detection.Slug), "")

	switch systemSlug {
	case "dreamcast":
		return validateDreamcastSave(input, normalized)
	case "n64":
		return validateN64Save(input, detection)
	case "saturn":
		return validateSaturnSave(input, normalized)
	case "gameboy", "gba", "nes", "snes", "nds":
		return validateNintendoRawSave(input, detection, systemSlug)
	case "neogeo":
		return validateNeoGeoSave(input, detection)
	case "genesis", "master-system", "game-gear":
		return validateStrictSegaRawSave(input, detection, systemSlug)
	default:
		return consoleValidationResult{}
	}
}

func validateSaturnSave(input saveCreateInput, normalized normalizedSaveMetadata) consoleValidationResult {
	details := normalized.Saturn
	if details == nil {
		if parsed := parseSaturnContainer(input.Filename, input.Payload); parsed != nil {
			details = parsed.Details
		}
	}
	if details == nil {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "saturn requires a validated backup RAM image",
		}
	}
	if details.SaveEntries <= 0 {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "saturn backup RAM image has no active save entries",
		}
	}

	evidence := []string{
		"validated Saturn backup RAM image",
		"format=" + strings.TrimSpace(details.Format),
		fmt.Sprintf("entries=%d", details.SaveEntries),
	}
	warnings := make([]string, 0, 2)
	activeSlots := make([]int, 0, len(details.Entries))
	for _, entry := range details.Entries {
		activeSlots = append(activeSlots, entry.FirstBlock)
	}
	for _, volume := range details.Volumes {
		evidence = append(evidence, fmt.Sprintf("%sBlocks=%d", volume.Kind, volume.TotalBlocks))
		if volume.Empty {
			warnings = append(warnings, fmt.Sprintf("%s volume is empty", volume.Kind))
		}
	}

	inspection := &saveInspection{
		ParserLevel:       saveParserLevelStructural,
		ParserID:          "saturn-backup-ram",
		ValidatedSystem:   "saturn",
		Evidence:          evidence,
		Warnings:          warnings,
		PayloadSizeBytes:  len(input.Payload),
		SlotCount:         details.SaveEntries,
		ActiveSlotIndexes: activeSlots,
		SemanticFields: map[string]any{
			"format":        details.Format,
			"defaultVolume": details.DefaultVolume,
		},
	}
	return consoleValidationResult{Inspection: inspection}
}

func validateDreamcastSave(input saveCreateInput, normalized normalizedSaveMetadata) consoleValidationResult {
	details := normalized.Dreamcast
	if details == nil {
		details = parseDreamcastContainer(input.Filename, input.Payload)
	}
	if details == nil {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "dreamcast requires a validated VMU/VMS/DCI container",
		}
	}
	if details.SaveEntries <= 0 {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "dreamcast container has no active save entries",
		}
	}

	evidence := []string{
		"validated Dreamcast container",
		"container=" + strings.TrimSpace(details.Container),
		fmt.Sprintf("activeEntries=%d", details.SaveEntries),
	}
	if slotName := strings.TrimSpace(details.SlotName); slotName != "" {
		evidence = append(evidence, "slotName="+slotName)
	}
	if details.IconFrames > 0 {
		evidence = append(evidence, fmt.Sprintf("iconFrames=%d", details.IconFrames))
	}

	warnings := make([]string, 0, 1)
	var checksumValid *bool
	knownChecksums := 0
	invalidChecksums := 0
	for _, entry := range details.Entries {
		if entry.CRCValid == nil {
			continue
		}
		knownChecksums++
		if !*entry.CRCValid {
			invalidChecksums++
		}
	}
	if knownChecksums > 0 {
		valid := invalidChecksums == 0
		checksumValid = &valid
		if !valid {
			warnings = append(warnings, "One or more Dreamcast entries failed CRC validation")
		}
	}

	inspection := &saveInspection{
		ParserLevel:      saveParserLevelStructural,
		ParserID:         "dreamcast-vmu",
		ValidatedSystem:  "dreamcast",
		Evidence:         evidence,
		Warnings:         warnings,
		PayloadSizeBytes: len(input.Payload),
		SlotCount:        details.SaveEntries,
		ChecksumValid:    checksumValid,
	}
	return consoleValidationResult{Inspection: inspection}
}

func validateStrictSegaRawSave(input saveCreateInput, detection saveSystemDetectionResult, systemSlug string) consoleValidationResult {
	return validateStrictRawSaveClass(input, detection, strictRawSaveValidationProfile{
		SystemSlug:           systemSlug,
		DisplayName:          systemSlug,
		ParserID:             "sega-raw-sram",
		AllowedExts:          map[string]struct{}{"sav": {}, "srm": {}, "ram": {}},
		AllowedSizes:         strictSegaRawSaveSizes,
		RequireROMSHA1:       true,
		RequireDeclared:      true,
		RequireHelperOrStore: true,
		RejectBlank:          true,
		SparseWarningCutoff:  16,
		Warning:              "No structural decoder is available yet for this Sega raw save",
	})
}

func isPlausibleStrictSegaRawSaveSize(size int) bool {
	_, ok := strictSegaRawSaveSizes[size]
	return ok
}

func isStrictSegaRawExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "sav", "srm", "ram":
		return true
	default:
		return false
	}
}
