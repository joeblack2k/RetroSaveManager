package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func validateNeoGeoSave(input saveCreateInput, detection saveSystemDetectionResult) consoleValidationResult {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(input.Filename))), ".")
	switch ext {
	case "sav", "srm", "ram":
	default:
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "neo geo saves require .sav, .srm, or .ram",
		}
	}
	if len(input.Payload) == 0 {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "neo geo save payload is empty",
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
	if strings.TrimSpace(input.ROMSHA1) == "" {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "neo geo saves require rom_sha1",
		}
	}
	if !detection.Evidence.Declared && !detection.Evidence.HelperTrusted && !detection.Evidence.StoredTrusted {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "neo geo saves require declared, helper, or stored system evidence",
		}
	}
	if !hasStrictNeoGeoSaveLayout(input.Payload) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "neo geo payload does not match a validated save layout",
		}
	}
	if allBytesEqual(input.Payload, 0x00) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "neo geo payload is blank (all 0x00)",
		}
	}
	if allBytesEqual(input.Payload, 0xFF) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "neo geo payload is blank (all 0xFF)",
		}
	}

	layout := "compound"
	switch len(input.Payload) {
	case neoGeoSaveRAMSize:
		layout = "saveram"
	case neoGeoCardDataSize:
		layout = "card-data"
	}
	evidence := []string{
		"validated neo geo save layout",
		"extension=" + ext,
		fmt.Sprintf("payloadSize=%d", len(input.Payload)),
		"layout=" + layout,
		fmt.Sprintf("nonPaddingBytes=%d", countNonPaddingBytes(input.Payload)),
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

	inspection := &saveInspection{
		ParserLevel:      saveParserLevelContainer,
		ParserID:         "neogeo-raw-save",
		ValidatedSystem:  "neogeo",
		TrustLevel:       n64TrustLevelMediaOnly,
		Evidence:         evidence,
		Warnings:         []string{"No title-specific Neo Geo save decoder is available yet"},
		PayloadSizeBytes: len(input.Payload),
		SemanticFields: map[string]any{
			"extension":       ext,
			"layout":          layout,
			"nonPaddingBytes": countNonPaddingBytes(input.Payload),
		},
	}
	return consoleValidationResult{Inspection: inspection}
}
