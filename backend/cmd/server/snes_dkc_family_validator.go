package main

import "strings"

const (
	snesDKCFamilyParserID = "snes-dkc-family"
)

type snesDKC3Signature struct {
	Offset int
	Value  string
}

var snesDKC3Signatures = []snesDKC3Signature{
	{Offset: 0x13, Value: "CRANK"},
	{Offset: 0x23, Value: "FUNK"},
	{Offset: 0x33, Value: "SWANK"},
	{Offset: 0x43, Value: "WRINKL"},
	{Offset: 0x68, Value: "DIXY"},
}

func validateSNESDKCFamilySave(input saveCreateInput, base *saveInspection) (*saveInspection, bool) {
	if base == nil || len(input.Payload) != dkcSRAMSize {
		return nil, false
	}
	if inspection, ok := validateSNESDKC1Save(input, base); ok {
		return inspection, true
	}
	if inspection, ok := validateSNESDKC3Save(input, base); ok {
		return inspection, true
	}
	return nil, false
}

func validateSNESDKC1Save(input saveCreateInput, base *saveInspection) (*saveInspection, bool) {
	parsed, err := parseDKCSRAM(input.Payload)
	if err != nil {
		return nil, false
	}
	activeSlots := make([]int, 0, dkcSlotCount)
	for idx, block := range parsed.Slots {
		if block != nil {
			activeSlots = append(activeSlots, idx+1)
		}
	}
	if len(activeSlots) == 0 {
		return nil, false
	}
	inspection := cloneSaveInspection(base)
	inspection.ParserLevel = saveParserLevelSemantic
	inspection.ParserID = snesDKCFamilyParserID
	inspection.ValidatedSystem = "snes"
	inspection.ValidatedGameID = "snes/donkey-kong-country"
	inspection.ValidatedGameTitle = "Donkey Kong Country"
	inspection.TrustLevel = n64TrustLevelSemanticVerified
	inspection.Evidence = append(cloneEvidence(base.Evidence), "validated Donkey Kong Country SRAM magic/checksum")
	inspection.Warnings = filterSNESRawGenericWarnings(base.Warnings)
	inspection.PayloadSizeBytes = len(input.Payload)
	inspection.SlotCount = len(activeSlots)
	inspection.ActiveSlotIndexes = activeSlots
	inspection.ChecksumValid = boolPtr(true)
	inspection.SemanticFields = mergeSemanticFields(base.SemanticFields, map[string]any{
		"family":      "donkey-kong-country",
		"variant":     "dkc1",
		"activeSlots": activeSlots,
	})
	return inspection, true
}

func validateSNESDKC3Save(input saveCreateInput, base *saveInspection) (*saveInspection, bool) {
	matches := make([]string, 0, len(snesDKC3Signatures))
	for _, signature := range snesDKC3Signatures {
		if hasASCIIAt(input.Payload, signature.Offset, signature.Value) {
			matches = append(matches, signature.Value)
		}
	}
	if len(matches) < 4 {
		return nil, false
	}
	inspection := cloneSaveInspection(base)
	inspection.ParserLevel = saveParserLevelStructural
	inspection.ParserID = snesDKCFamilyParserID
	inspection.ValidatedSystem = "snes"
	inspection.ValidatedGameID = "snes/donkey-kong-country-3"
	inspection.ValidatedGameTitle = "Donkey Kong Country 3 - Dixie Kong's Double Trouble!"
	inspection.TrustLevel = n64TrustLevelGameValidated
	inspection.Evidence = append(cloneEvidence(base.Evidence), "validated Donkey Kong Country 3 SRAM character-name table")
	for _, match := range matches {
		inspection.Evidence = append(inspection.Evidence, "signature="+match)
	}
	inspection.Warnings = filterSNESRawGenericWarnings(base.Warnings)
	inspection.PayloadSizeBytes = len(input.Payload)
	inspection.SemanticFields = mergeSemanticFields(base.SemanticFields, map[string]any{
		"family":     "donkey-kong-country",
		"variant":    "dkc3",
		"signatures": matches,
	})
	return inspection, true
}

func cloneSaveInspection(source *saveInspection) *saveInspection {
	if source == nil {
		return &saveInspection{}
	}
	clone := *source
	clone.Evidence = append([]string(nil), source.Evidence...)
	clone.Warnings = append([]string(nil), source.Warnings...)
	clone.ActiveSlotIndexes = append([]int(nil), source.ActiveSlotIndexes...)
	clone.SemanticFields = mergeSemanticFields(source.SemanticFields, nil)
	return &clone
}

func mergeSemanticFields(base map[string]any, extra map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func cloneEvidence(values []string) []string {
	return append([]string(nil), values...)
}

func filterSNESRawGenericWarnings(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.Contains(value, "No structural SNES save decoder") {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func hasASCIIAt(payload []byte, offset int, value string) bool {
	if offset < 0 || offset+len(value) > len(payload) {
		return false
	}
	return string(payload[offset:offset+len(value)]) == value
}
