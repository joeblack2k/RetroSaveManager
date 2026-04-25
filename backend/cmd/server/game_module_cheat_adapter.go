package main

import (
	"fmt"
	"strings"
)

// gameModuleCheatAdapter bridges a runtime-loaded WASM module into the normal
// cheatAdapter interface. This keeps My Saves, Details, and the cheat APIs using
// the same code path for built-in and community modules.
type gameModuleCheatAdapter struct {
	service *gameModuleService
	record  gameModuleRecord
}

func (a gameModuleCheatAdapter) ID() string                    { return strings.TrimSpace(a.record.Manifest.ParserID) }
func (a gameModuleCheatAdapter) Kind() string                  { return "wasm-module" }
func (a gameModuleCheatAdapter) Family() string                { return a.record.Manifest.ModuleID }
func (a gameModuleCheatAdapter) SystemSlug() string            { return a.record.Manifest.SystemSlug }
func (a gameModuleCheatAdapter) RequiredParserID() string      { return a.record.Manifest.ParserID }
func (a gameModuleCheatAdapter) MinimumParserLevel() string    { return saveParserLevelStructural }
func (a gameModuleCheatAdapter) SupportsRuntimeProfiles() bool { return true }
func (a gameModuleCheatAdapter) SupportsLogicalSaves() bool    { return true }
func (a gameModuleCheatAdapter) SupportsLiveUpload() bool      { return true }
func (a gameModuleCheatAdapter) MatchKeys() []string {
	return []string{"moduleId", "parserId", "systemSlug", "titleAliases", "payloadSize", "format"}
}
func (a gameModuleCheatAdapter) IntegrityHooks() []cheatPayloadTransform { return nil }

// Supports accepts either parser-backed inspection evidence or conservative
// title aliases from the module manifest.
func (a gameModuleCheatAdapter) Supports(summary saveSummary, inspection *saveInspection) bool {
	if a.record.Status != gameModuleStatusActive {
		return false
	}
	if canonicalSegment(summary.SystemSlug, "") != a.record.Manifest.SystemSlug {
		return false
	}
	if inspection != nil {
		if strings.TrimSpace(inspection.ParserID) == a.record.Manifest.ParserID {
			return true
		}
		if strings.TrimSpace(inspection.ValidatedGameID) != "" && strings.EqualFold(strings.TrimSpace(inspection.ValidatedGameID), a.record.Manifest.GameID) {
			return true
		}
	}
	for _, alias := range a.record.Manifest.TitleAliases {
		if cheatTitleKey(alias) != "" && (cheatTitleKey(alias) == cheatTitleKey(summary.DisplayTitle) || cheatTitleKey(alias) == cheatTitleKey(summary.Game.Name)) {
			return true
		}
	}
	return false
}

func (a gameModuleCheatAdapter) FieldCatalog() map[string]cheatField {
	out := map[string]cheatField{}
	if a.service == nil {
		return out
	}
	packs, err := a.service.moduleCheatPacks(a.record)
	if err != nil {
		return out
	}
	for _, pack := range packs {
		for _, section := range pack.Sections {
			for _, field := range section.Fields {
				out[field.ID] = field
				if strings.TrimSpace(field.Ref) != "" {
					out[field.Ref] = field
				}
			}
		}
	}
	return out
}

func (a gameModuleCheatAdapter) PreparePack(pack cheatPack) (cheatPack, error) {
	pack.PackID = canonicalCheatPackID(firstNonEmpty(pack.PackID, pack.GameID, pack.Title))
	pack.SchemaVersion = maxInt(pack.SchemaVersion, 1)
	pack.SystemSlug = canonicalSegment(firstNonEmpty(pack.SystemSlug, a.record.Manifest.SystemSlug), a.record.Manifest.SystemSlug)
	pack.GameID = firstNonEmpty(pack.GameID, a.record.Manifest.GameID)
	pack.Title = firstNonEmpty(pack.Title, a.record.Manifest.Title)
	pack.AdapterID = firstNonEmpty(pack.AdapterID, a.record.Manifest.ParserID)
	pack.EditorID = firstNonEmpty(pack.EditorID, a.record.Manifest.ParserID)
	if len(pack.Match.TitleAliases) == 0 {
		pack.Match.TitleAliases = append([]string(nil), a.record.Manifest.TitleAliases...)
	}
	if len(pack.Payload.ExactSizes) == 0 {
		pack.Payload.ExactSizes = append([]int(nil), a.record.Manifest.Payload.ExactSizes...)
	}
	if len(pack.Payload.Formats) == 0 {
		pack.Payload.Formats = append([]string(nil), a.record.Manifest.Payload.Formats...)
	}
	return normalizeCheatPack(pack, true)
}

// Read sends the canonical payload to the module. Logical save hints such as
// psLogicalKey and Saturn entry names are passed as metadata, not guessed here.
func (a gameModuleCheatAdapter) Read(ctx cheatAdapterContext, pack cheatPack, payload []byte) (saveCheatEditorState, error) {
	if a.service == nil {
		return saveCheatEditorState{}, fmt.Errorf("module service is not initialized")
	}
	var state saveCheatEditorState
	err := a.service.callWASM(a.record, "readCheats", gameModuleCheatRequest{
		Payload:     payload,
		Pack:        pack,
		LogicalKey:  strings.TrimSpace(ctx.Summary.LogicalKey),
		SaturnEntry: saturnEntryNameFromSummary(ctx.Summary),
		Summary:     ctx.Summary,
		Inspection:  ctx.Inspection,
		Metadata:    metadataMap(ctx.Summary.Metadata),
	}, &state)
	if err != nil {
		return saveCheatEditorState{}, err
	}
	return state, nil
}

// Apply returns a patched payload only; version creation and projection rebuilds
// stay owned by the core cheat service.
func (a gameModuleCheatAdapter) Apply(ctx cheatAdapterContext, pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error) {
	if a.service == nil {
		return nil, nil, fmt.Errorf("module service is not initialized")
	}
	var response gameModuleCheatApplyResponse
	err := a.service.callWASM(a.record, "applyCheats", gameModuleCheatRequest{
		Payload:     payload,
		Pack:        pack,
		LogicalKey:  strings.TrimSpace(ctx.Summary.LogicalKey),
		SaturnEntry: saturnEntryNameFromSummary(ctx.Summary),
		SlotID:      strings.TrimSpace(slotID),
		Updates:     updates,
		Summary:     ctx.Summary,
		Inspection:  ctx.Inspection,
		Metadata:    metadataMap(ctx.Summary.Metadata),
	}, &response)
	if err != nil {
		return nil, nil, err
	}
	if len(response.Payload) == 0 {
		return nil, nil, fmt.Errorf("module did not return patched payload")
	}
	changed := response.Changed
	if changed == nil {
		changed = map[string]any{}
	}
	if len(response.IntegritySteps) > 0 {
		changed["moduleIntegritySteps"] = append([]string(nil), response.IntegritySteps...)
	}
	return response.Payload, changed, nil
}
