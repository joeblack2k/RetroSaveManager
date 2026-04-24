package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

var strictRawGBSizes = map[int]struct{}{
	512:   {},
	1024:  {},
	2048:  {},
	4096:  {},
	8192:  {},
	16384: {},
	32768: {},
	65536: {},
}

var strictRawGBASizes = map[int]struct{}{
	512:    {},
	8192:   {},
	32768:  {},
	65536:  {},
	131072: {},
}

var strictRawSNESSizes = map[int]struct{}{
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

var strictRawNESSizes = map[int]struct{}{
	512:   {},
	1024:  {},
	2048:  {},
	4096:  {},
	8192:  {},
	16384: {},
	32768: {},
	65536: {},
}

func validateNintendoRawSave(input saveCreateInput, detection saveSystemDetectionResult, systemSlug string) consoleValidationResult {
	switch systemSlug {
	case "gameboy":
		return validateStrictRawSaveClass(input, detection, strictRawSaveValidationProfile{
			SystemSlug:          "gameboy",
			DisplayName:         "game boy",
			ParserID:            "gameboy-raw-sram",
			AllowedExts:         map[string]struct{}{"sav": {}, "srm": {}, "ram": {}, "rtc": {}, "gme": {}},
			AllowedSizes:        strictRawGBSizes,
			RequireROMSHA1:      true,
			RequireTrustedMatch: true,
			RejectBlank:         true,
			SparseWarningCutoff: 16,
			Warning:             "No structural Game Boy save decoder is available yet for this raw SRAM payload",
		})
	case "gba":
		return validateStrictRawSaveClass(input, detection, strictRawSaveValidationProfile{
			SystemSlug:          "gba",
			DisplayName:         "gba",
			ParserID:            "gba-raw-backup",
			AllowedExts:         map[string]struct{}{"sav": {}, "srm": {}, "sa1": {}},
			AllowedSizes:        strictRawGBASizes,
			RequireROMSHA1:      true,
			RequireTrustedMatch: true,
			RequireSignature:    hasGBASignature,
			SignatureReason:     "gba validated payload signature",
			RejectBlank:         true,
			SparseWarningCutoff: 16,
			Warning:             "No structural GBA save decoder is available yet beyond backup-library signature validation",
		})
	case "nes":
		return validateStrictRawSaveClass(input, detection, strictRawSaveValidationProfile{
			SystemSlug:          "nes",
			DisplayName:         "nes",
			ParserID:            "nes-raw-sram",
			AllowedExts:         map[string]struct{}{"sav": {}, "srm": {}, "ram": {}},
			AllowedSizes:        strictRawNESSizes,
			RequireROMSHA1:      true,
			RequireTrustedMatch: true,
			RejectBlank:         true,
			SparseWarningCutoff: 16,
			Warning:             "No structural NES save decoder is available yet for this raw SRAM payload",
		})
	case "snes":
		result := validateStrictRawSaveClass(input, detection, strictRawSaveValidationProfile{
			SystemSlug:          "snes",
			DisplayName:         "snes",
			ParserID:            "snes-raw-sram",
			AllowedExts:         map[string]struct{}{"sav": {}, "srm": {}, "sa1": {}},
			AllowedSizes:        strictRawSNESSizes,
			RequireROMSHA1:      true,
			RequireTrustedMatch: true,
			RejectBlank:         true,
			SparseWarningCutoff: 16,
			Warning:             "No structural SNES save decoder is available yet for this raw SRAM payload",
		})
		if result.Rejected || result.Inspection == nil {
			return result
		}
		if enriched, ok := validateSNESDKCFamilySave(input, result.Inspection); ok {
			result.Inspection = enriched
		}
		return result
	case "nds":
		return validateNintendoDSSave(input)
	case "wii":
		return validateWiiSave(input, detection)
	default:
		return consoleValidationResult{}
	}
}

func validateNintendoDSSave(input saveCreateInput) consoleValidationResult {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(input.Filename))), ".")
	if ext != "sav" && ext != "dsv" {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "nds saves require .sav or .dsv payloads",
		}
	}
	if len(input.Payload) == 0 {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "nds save payload is empty",
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
	if !hasNewSuperMarioBrosNDSSignature(input.Payload) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "nds save is not a validated supported Nintendo DS profile",
		}
	}

	signatureCount := bytesCount(input.Payload, []byte("Mario2d"))
	inspection := &saveInspection{
		ParserLevel:        saveParserLevelStructural,
		ParserID:           "nds-new-super-mario-bros",
		ValidatedSystem:    "nds",
		ValidatedGameID:    "nds/new-super-mario-bros",
		ValidatedGameTitle: "New Super Mario Bros.",
		TrustLevel:         n64TrustLevelGameValidated,
		Evidence: []string{
			"validated Nintendo DS save profile",
			"game=New Super Mario Bros.",
			fmt.Sprintf("payloadSize=%d", len(input.Payload)),
			fmt.Sprintf("signatureCount=%d", signatureCount),
		},
		PayloadSizeBytes: len(input.Payload),
		SemanticFields: map[string]any{
			"signature":      "Mario2d",
			"signatureCount": signatureCount,
		},
	}
	return consoleValidationResult{Inspection: inspection}
}

func bytesCount(payload []byte, needle []byte) int {
	if len(needle) == 0 || len(payload) < len(needle) {
		return 0
	}
	count := 0
	for idx := 0; idx <= len(payload)-len(needle); idx++ {
		if bytes.Equal(payload[idx:idx+len(needle)], needle) {
			count++
		}
	}
	return count
}
