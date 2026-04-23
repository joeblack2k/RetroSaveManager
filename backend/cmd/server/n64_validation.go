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
}

func validateN64Save(input saveCreateInput, detection saveSystemDetectionResult) consoleValidationResult {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(input.Filename))), ".")
	profile, ok := n64ValidationProfiles[ext]
	if !ok {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "n64 saves require .eep, .fla, .sra, or .mpk media",
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
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("n64 %s payload size %d is not recognized", profile.mediaType, len(input.Payload)),
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

	inspection := &saveInspection{
		ParserLevel:      saveParserLevelContainer,
		ParserID:         "n64-save-media",
		ValidatedSystem:  "n64",
		Evidence:         evidence,
		Warnings:         warnings,
		PayloadSizeBytes: len(input.Payload),
		SemanticFields: map[string]any{
			"extension":    ext,
			"mediaType":    profile.mediaType,
			"nonZeroBytes": nonZeroBytes,
		},
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
