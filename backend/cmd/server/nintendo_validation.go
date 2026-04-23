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
			RequireSignature:    hasGBABackupSignature,
			SignatureReason:     "gba backup signature",
			Warning:             "No structural GBA save decoder is available yet beyond backup-library signature validation",
		})
	case "snes":
		return validateStrictRawSaveClass(input, detection, strictRawSaveValidationProfile{
			SystemSlug:          "snes",
			DisplayName:         "snes",
			ParserID:            "snes-raw-sram",
			AllowedExts:         map[string]struct{}{"sav": {}, "srm": {}, "sa1": {}},
			AllowedSizes:        strictRawSNESSizes,
			RequireROMSHA1:      true,
			RequireTrustedMatch: true,
			Warning:             "No structural SNES save decoder is available yet for this raw SRAM payload",
		})
	case "nds":
		return validateNintendoDSSave(input)
	default:
		return consoleValidationResult{}
	}
}

type strictRawSaveValidationProfile struct {
	SystemSlug          string
	DisplayName         string
	ParserID            string
	AllowedExts         map[string]struct{}
	AllowedSizes        map[int]struct{}
	RequireROMSHA1      bool
	RequireTrustedMatch bool
	RequireSignature    func([]byte) bool
	SignatureReason     string
	Warning             string
}

func validateStrictRawSaveClass(input saveCreateInput, detection saveSystemDetectionResult, profile strictRawSaveValidationProfile) consoleValidationResult {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(input.Filename))), ".")
	if _, ok := profile.AllowedExts[ext]; !ok {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw saves require one of the supported raw-save extensions", profile.DisplayName),
		}
	}
	if len(input.Payload) == 0 {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw save payload is empty", profile.DisplayName),
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
	if _, ok := profile.AllowedSizes[len(input.Payload)]; !ok {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw save size %d is not recognized", profile.DisplayName, len(input.Payload)),
		}
	}
	if allBytesEqual(input.Payload, 0x00) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw save payload is blank (all 0x00)", profile.DisplayName),
		}
	}
	if allBytesEqual(input.Payload, 0xFF) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw save payload is blank (all 0xFF)", profile.DisplayName),
		}
	}
	if profile.RequireROMSHA1 && strings.TrimSpace(input.ROMSHA1) == "" {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw saves require rom_sha1", profile.DisplayName),
		}
	}
	if profile.RequireTrustedMatch && !detection.Evidence.Declared && !detection.Evidence.HelperTrusted && !detection.Evidence.StoredTrusted {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw saves require declared, helper, or stored system evidence", profile.DisplayName),
		}
	}
	if profile.RequireSignature != nil && !profile.RequireSignature(input.Payload) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw save is missing a validated payload signature", profile.DisplayName),
		}
	}

	nonZeroBytes := countBytesNotEqual(input.Payload, 0x00)
	evidence := []string{
		"validated raw save class",
		"extension=" + ext,
		fmt.Sprintf("payloadSize=%d", len(input.Payload)),
		fmt.Sprintf("nonZeroBytes=%d", nonZeroBytes),
	}
	if detection.Evidence.HelperTrusted {
		evidence = append(evidence, "trusted helper system")
	}
	if detection.Evidence.StoredTrusted {
		evidence = append(evidence, "trusted stored system")
	}
	if detection.Evidence.Declared && !detection.Evidence.HelperTrusted && !detection.Evidence.StoredTrusted {
		evidence = append(evidence, "declared system")
	}
	if profile.RequireSignature != nil && strings.TrimSpace(profile.SignatureReason) != "" {
		evidence = append(evidence, profile.SignatureReason)
	}

	warnings := []string(nil)
	if strings.TrimSpace(profile.Warning) != "" {
		warnings = append(warnings, profile.Warning)
	}
	if nonZeroBytes <= 16 {
		warnings = append(warnings, "Payload is extremely sparse and only raw media validation is available")
	}

	inspection := &saveInspection{
		ParserLevel:      saveParserLevelContainer,
		ParserID:         profile.ParserID,
		ValidatedSystem:  profile.SystemSlug,
		TrustLevel:       n64TrustLevelMediaOnly,
		Evidence:         evidence,
		Warnings:         warnings,
		PayloadSizeBytes: len(input.Payload),
		SemanticFields: map[string]any{
			"extension":    ext,
			"nonZeroBytes": nonZeroBytes,
		},
	}
	return consoleValidationResult{Inspection: inspection}
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
