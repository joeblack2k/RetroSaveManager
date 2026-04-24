package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type n64ValidationProfile struct {
	mediaType    string
	allowedSizes map[int]struct{}
}

var n64ValidationProfiles = map[string]n64ValidationProfile{
	"eep": {
		mediaType: "eeprom",
		allowedSizes: map[int]struct{}{
			512:  {},
			2048: {},
		},
	},
	"fla": {
		mediaType: "flashram",
		allowedSizes: map[int]struct{}{
			131072: {},
		},
	},
	"sra": {
		mediaType: "sram",
		allowedSizes: map[int]struct{}{
			32768: {},
		},
	},
	"mpk": {
		mediaType: "controller-pak",
		allowedSizes: map[int]struct{}{
			32768: {},
		},
	},
	"cpk": {
		mediaType: "controller-pak",
		allowedSizes: map[int]struct{}{
			32768: {},
		},
	},
}

func validateN64Save(input saveCreateInput, detection saveSystemDetectionResult) consoleValidationResult {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(input.Filename))), ".")
	controllerPakPayload := input.Payload
	runtimeProfile := canonicalN64Profile(firstNonEmpty(input.RuntimeProfile, input.SourceArtifactProfile))
	projectionWrapperAccepted := false
	var projectionMediaInfo n64MediaInfo
	profile, ok := n64ValidationProfiles[ext]
	if runtimeProfile != "" && strings.TrimSpace(input.ProjectionID) != "" {
		if normalizedPayload, mediaInfo, err := normalizeN64UploadPayload(runtimeProfile, ext, input.Payload); err == nil {
			projectionWrapperAccepted = true
			projectionMediaInfo = mediaInfo
			if mediaInfo.MediaType == "controller-pak" {
				controllerPakPayload = normalizedPayload
			}
			if !ok {
				profile = n64ValidationProfile{
					mediaType: mediaInfo.MediaType,
					allowedSizes: map[int]struct{}{
						len(input.Payload): {},
					},
				}
				ok = true
			}
		}
	}
	if !ok {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "n64 saves require .eep, .fla, .sra, .mpk, or .cpk media",
		}
	}
	if len(input.Payload) == 0 {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("n64 %s payload is empty", profile.mediaType),
		}
	}
	if looksLikeExecutableOrArchivePayload(input.Payload) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "payload looks like executable/archive",
		}
	}
	if looksLikeMostlyTextPayload(input.Payload) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "payload looks like text/noise",
		}
	}
	if _, ok := profile.allowedSizes[len(input.Payload)]; !ok {
		if projectionWrapperAccepted && projectionMediaInfo.MediaType == profile.mediaType {
			profile.allowedSizes[len(input.Payload)] = struct{}{}
		}
	}
	if _, ok := profile.allowedSizes[len(input.Payload)]; !ok {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("n64 %s payload size %d is not recognized", profile.mediaType, len(input.Payload)),
		}
	}
	entryCount := 0
	if profile.mediaType == "controller-pak" {
		var err error
		entryCount, err = countN64ControllerPakEntries(controllerPakPayload)
		if err != nil {
			return consoleValidationResult{
				Rejected:     true,
				RejectReason: fmt.Sprintf("n64 controller-pak filesystem is invalid: %v", err),
			}
		}
		if entryCount == 0 {
			return consoleValidationResult{
				Rejected:     true,
				RejectReason: "n64 controller-pak does not contain any save entries",
			}
		}
	}
	if allBytesEqual(input.Payload, 0x00) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("n64 %s payload is blank (all 0x00)", profile.mediaType),
		}
	}
	if allBytesEqual(input.Payload, 0xFF) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("n64 %s payload is blank (all 0xFF)", profile.mediaType),
		}
	}

	nonZeroBytes := countBytesNotEqual(input.Payload, 0x00)
	evidence := []string{
		"validated N64 save media",
		"mediaType=" + profile.mediaType,
		"extension=" + ext,
		fmt.Sprintf("payloadSize=%d", len(input.Payload)),
		fmt.Sprintf("nonZeroBytes=%d", nonZeroBytes),
	}
	if detection.Evidence.Declared {
		evidence = append(evidence, "declared system evidence")
	}
	if detection.Evidence.HelperTrusted {
		evidence = append(evidence, "trusted helper system")
	}
	if detection.Evidence.StoredTrusted {
		evidence = append(evidence, "trusted stored system")
	}

	warnings := make([]string, 0, 1)
	if nonZeroBytes <= 16 {
		warnings = append(warnings, "Payload is extremely sparse; title-specific semantic validation is not available yet")
	}

	parserID := "n64-save-media"
	parserLevel := saveParserLevelContainer
	trustLevel := n64TrustLevelMediaOnly
	slotCount := 0
	activeSlotIndexes := []int(nil)
	var checksumValid *bool
	if profile.mediaType == "controller-pak" {
		parserID = "n64-controller-pak"
		parserLevel = saveParserLevelStructural
		slotCount = entryCount
		activeSlotIndexes = make([]int, 0, entryCount)
		for i := 1; i <= entryCount; i++ {
			activeSlotIndexes = append(activeSlotIndexes, i)
		}
		checksumValid = boolPtr(true)
		evidence = append(evidence, fmt.Sprintf("controllerPakEntries=%d", entryCount))
	}
	semanticFields := map[string]any{
		"extension":    ext,
		"mediaType":    profile.mediaType,
		"nonZeroBytes": nonZeroBytes,
	}
	if profile.mediaType == "controller-pak" {
		semanticFields["entryCount"] = entryCount
	}
	validatedGameID := ""
	validatedGameTitle := ""
	for _, validator := range n64GameValidators {
		result, ok := validator.Validate(n64ValidationContext{
			Extension: ext,
			MediaType: profile.mediaType,
			Filename:  input.Filename,
			ROMSHA1:   input.ROMSHA1,
		}, input.Payload)
		if !ok {
			continue
		}
		parserID = validator.ID()
		parserLevel = result.ParserLevel
		trustLevel = firstNonEmpty(result.TrustLevel, n64TrustLevelGameValidated)
		validatedGameID = strings.TrimSpace(result.GameID)
		validatedGameTitle = strings.TrimSpace(result.GameTitle)
		evidence = append(evidence, result.Evidence...)
		warnings = append(warnings, result.Warnings...)
		slotCount = result.SlotCount
		activeSlotIndexes = append([]int(nil), result.ActiveSlotIndexes...)
		checksumValid = result.ChecksumValid
		for key, value := range result.SemanticFields {
			semanticFields[key] = value
		}
		break
	}

	inspection := &saveInspection{
		ParserLevel:        parserLevel,
		ParserID:           parserID,
		ValidatedSystem:    "n64",
		ValidatedGameID:    validatedGameID,
		ValidatedGameTitle: validatedGameTitle,
		TrustLevel:         trustLevel,
		Evidence:           evidence,
		Warnings:           warnings,
		PayloadSizeBytes:   len(input.Payload),
		SlotCount:          slotCount,
		ActiveSlotIndexes:  activeSlotIndexes,
		ChecksumValid:      checksumValid,
		SemanticFields:     semanticFields,
	}
	return consoleValidationResult{Inspection: inspection}
}

func allBytesEqual(payload []byte, want byte) bool {
	if len(payload) == 0 {
		return false
	}
	for _, value := range payload {
		if value != want {
			return false
		}
	}
	return true
}

func countBytesNotEqual(payload []byte, skip byte) int {
	total := 0
	for _, value := range payload {
		if value != skip {
			total++
		}
	}
	return total
}
