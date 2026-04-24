package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	rawTrustLevelROMMediaVerified = "rom-media-verified"
)

type rawSaveStats struct {
	Size         int
	NonZero      int
	NonFF        int
	BlankZero    bool
	BlankFF      bool
	SparseCutoff int
}

type strictRawSaveValidationProfile struct {
	SystemSlug           string
	DisplayName          string
	ParserID             string
	AllowedExts          map[string]struct{}
	AllowedSizes         map[int]struct{}
	RequireROMSHA1       bool
	RequireTrustedMatch  bool
	RequireDeclared      bool
	RequireHelperOrStore bool
	RequireSignature     func([]byte) bool
	SignatureReason      string
	RejectBlank          bool
	SparseWarningCutoff  int
	Warning              string
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
	if profile.RequireROMSHA1 && strings.TrimSpace(input.ROMSHA1) == "" {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw saves require rom_sha1", profile.DisplayName),
		}
	}
	if profile.RequireDeclared && !detection.Evidence.Declared {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw saves require an explicit system declaration", profile.SystemSlug),
		}
	}
	if profile.RequireTrustedMatch && !detection.Evidence.Declared && !detection.Evidence.HelperTrusted && !detection.Evidence.StoredTrusted {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw saves require declared, helper, or stored system evidence", profile.DisplayName),
		}
	}
	if profile.RequireHelperOrStore && !detection.Evidence.HelperTrusted && !detection.Evidence.StoredTrusted {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw saves require trusted helper or stored system evidence", profile.SystemSlug),
		}
	}
	if profile.RequireSignature != nil && !profile.RequireSignature(input.Payload) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw save is missing a validated payload signature", profile.DisplayName),
		}
	}

	stats := analyzeRawSavePayload(input.Payload, profile.SparseWarningCutoff)
	if profile.RejectBlank && stats.BlankZero {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw save payload is blank (all 0x00)", profile.DisplayName),
		}
	}
	if profile.RejectBlank && stats.BlankFF {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("%s raw save payload is blank (all 0xFF)", profile.DisplayName),
		}
	}

	evidence := []string{
		"validated raw save media",
		"extension=" + ext,
		fmt.Sprintf("payloadSize=%d", stats.Size),
		fmt.Sprintf("nonZeroBytes=%d", stats.NonZero),
		fmt.Sprintf("nonFFBytes=%d", stats.NonFF),
		"blank check passed",
	}
	romLinked := strings.TrimSpace(input.ROMSHA1) != ""
	if romLinked {
		evidence = append(evidence, "romSha1 present")
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
	if stats.SparseCutoff > 0 && stats.NonZero <= stats.SparseCutoff {
		warnings = append(warnings, "Payload is extremely sparse and only raw media validation is available")
	}

	trustLevel := n64TrustLevelMediaOnly
	if strings.TrimSpace(input.ROMSHA1) != "" {
		trustLevel = rawTrustLevelROMMediaVerified
	}
	inspection := &saveInspection{
		ParserLevel:      saveParserLevelContainer,
		ParserID:         profile.ParserID,
		ValidatedSystem:  profile.SystemSlug,
		TrustLevel:       trustLevel,
		Evidence:         evidence,
		Warnings:         warnings,
		PayloadSizeBytes: stats.Size,
		SemanticFields: map[string]any{
			"extension":      ext,
			"rawSaveKind":    rawSaveKind(profile.SystemSlug, ext),
			"blankCheck":     "passed",
			"nonZeroBytes":   stats.NonZero,
			"nonFFBytes":     stats.NonFF,
			"romLinked":      romLinked,
			"romSha1Present": romLinked,
		},
	}
	return consoleValidationResult{Inspection: inspection}
}

func rawSaveKind(systemSlug, ext string) string {
	switch strings.TrimSpace(strings.ToLower(systemSlug)) {
	case "gameboy":
		switch strings.TrimSpace(strings.ToLower(ext)) {
		case "rtc":
			return "Game Boy real-time clock data"
		case "gme":
			return "Game Boy emulator save package"
		default:
			return "Game Boy cartridge SRAM"
		}
	case "gba":
		return "Game Boy Advance backup memory"
	case "nes":
		return "NES cartridge SRAM"
	case "snes":
		return "SNES cartridge SRAM"
	case "genesis":
		return "Genesis / Mega Drive cartridge SRAM"
	case "master-system":
		return "Master System cartridge SRAM"
	case "game-gear":
		return "Game Gear cartridge SRAM"
	default:
		return "Raw cartridge save media"
	}
}

func analyzeRawSavePayload(payload []byte, sparseCutoff int) rawSaveStats {
	stats := rawSaveStats{
		Size:         len(payload),
		SparseCutoff: sparseCutoff,
	}
	if len(payload) == 0 {
		stats.BlankZero = true
		stats.BlankFF = true
		return stats
	}
	stats.NonZero = countBytesNotEqual(payload, 0x00)
	stats.NonFF = countBytesNotEqual(payload, 0xFF)
	stats.BlankZero = stats.NonZero == 0
	stats.BlankFF = stats.NonFF == 0
	return stats
}
