package main

import (
	"path/filepath"
	"strings"
)

// inspectSave lets modules add semantic facts such as lives, stage, map,
// inventory, or checksum state to save details without changing core ingest code.
func (s *gameModuleService) inspectSave(input saveCreateInput, base *saveInspection) (*saveInspection, bool) {
	if s == nil || len(input.Payload) == 0 {
		return nil, false
	}
	modules, err := s.listModules()
	if err != nil {
		return nil, false
	}
	for _, record := range modules {
		if record.Status != gameModuleStatusActive || !gameModuleMatchesInput(record.Manifest, input) {
			continue
		}
		var response gameModuleParseResponse
		err := s.callWASM(record, "parse", gameModuleParseRequest{
			Payload:      input.Payload,
			Filename:     input.Filename,
			Format:       input.Format,
			SystemSlug:   input.SystemSlug,
			DisplayTitle: firstNonEmpty(input.DisplayTitle, input.Game.DisplayTitle, input.Game.Name),
			ROMSHA1:      input.ROMSHA1,
			ROMMD5:       input.ROMMD5,
			SlotName:     input.SlotName,
			Metadata:     metadataMap(input.Metadata),
		}, &response)
		if err != nil || !response.Supported {
			continue
		}
		inspection := cloneSaveInspection(base)
		inspection.ParserLevel = firstNonEmpty(response.ParserLevel, saveParserLevelSemantic)
		inspection.ParserID = firstNonEmpty(response.ParserID, record.Manifest.ParserID)
		inspection.ValidatedSystem = firstNonEmpty(response.ValidatedSystem, record.Manifest.SystemSlug)
		inspection.ValidatedGameID = firstNonEmpty(response.ValidatedGameID, record.Manifest.GameID)
		inspection.ValidatedGameTitle = firstNonEmpty(response.ValidatedGameTitle, record.Manifest.Title)
		inspection.TrustLevel = firstNonEmpty(response.TrustLevel, "module-semantic-verified")
		inspection.Evidence = append(cloneEvidence(inspection.Evidence), response.Evidence...)
		inspection.Evidence = append(inspection.Evidence, "runtime module="+record.Manifest.ModuleID)
		inspection.Warnings = append(append([]string(nil), inspection.Warnings...), response.Warnings...)
		inspection.PayloadSizeBytes = len(input.Payload)
		if response.SlotCount > 0 {
			inspection.SlotCount = response.SlotCount
		}
		if len(response.ActiveSlotIndexes) > 0 {
			inspection.ActiveSlotIndexes = append([]int(nil), response.ActiveSlotIndexes...)
		}
		if response.ChecksumValid != nil {
			inspection.ChecksumValid = response.ChecksumValid
		}
		inspection.SemanticFields = mergeSemanticFields(inspection.SemanticFields, response.SemanticFields)
		return inspection, true
	}
	return nil, false
}

// gameModuleMatchesInput is intentionally strict: system, size, format, and
// optional ROM hash/title aliases must match before a module can parse a save.
func gameModuleMatchesInput(manifest gameModuleManifest, input saveCreateInput) bool {
	if canonicalSegment(input.SystemSlug, "") != manifest.SystemSlug {
		return false
	}
	if len(manifest.Payload.ExactSizes) > 0 {
		matched := false
		for _, size := range manifest.Payload.ExactSizes {
			if len(input.Payload) == size {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(manifest.Payload.Formats) > 0 {
		format := strings.ToLower(strings.TrimSpace(input.Format))
		matched := false
		for _, candidate := range manifest.Payload.Formats {
			if strings.ToLower(strings.TrimSpace(candidate)) == format {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(manifest.ROMHashes) > 0 {
		rom := strings.ToLower(strings.TrimSpace(firstNonEmpty(input.ROMSHA1, input.ROMMD5)))
		matched := false
		for _, candidate := range manifest.ROMHashes {
			if strings.ToLower(strings.TrimSpace(candidate)) == rom {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	title := cheatTitleKey(firstNonEmpty(input.DisplayTitle, input.Game.DisplayTitle, input.Game.Name, strings.TrimSuffix(input.Filename, filepath.Ext(input.Filename))))
	for _, alias := range manifest.TitleAliases {
		if title != "" && title == cheatTitleKey(alias) {
			return true
		}
	}
	return len(manifest.ROMHashes) > 0
}
