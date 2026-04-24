package main

import (
	"fmt"
	"sort"
	"strings"
)

type cheatAdapterContext struct {
	Record     saveRecord
	Summary    saveSummary
	Inspection *saveInspection
}

type cheatPayloadTransform interface {
	Name() string
	Apply(payload []byte, ctx cheatAdapterContext) ([]byte, error)
}

type cheatAdapter interface {
	ID() string
	Kind() string
	Family() string
	SystemSlug() string
	RequiredParserID() string
	MinimumParserLevel() string
	SupportsRuntimeProfiles() bool
	SupportsLogicalSaves() bool
	SupportsLiveUpload() bool
	MatchKeys() []string
	Supports(summary saveSummary, inspection *saveInspection) bool
	FieldCatalog() map[string]cheatField
	PreparePack(pack cheatPack) (cheatPack, error)
	Read(ctx cheatAdapterContext, pack cheatPack, payload []byte) (saveCheatEditorState, error)
	Apply(ctx cheatAdapterContext, pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error)
	IntegrityHooks() []cheatPayloadTransform
}

type legacyCheatAdapter struct {
	id                     string
	kind                   string
	family                 string
	systemSlug             string
	requiredParserID       string
	minimumParserLevel     string
	supportsRuntimeProfile bool
	supportsLogicalSaves   bool
	supportsLiveUpload     bool
	matchKeys              []string
	editor                 cheatEditor
	basePack               cheatPack
}

func (a legacyCheatAdapter) ID() string                              { return a.id }
func (a legacyCheatAdapter) Kind() string                            { return a.kind }
func (a legacyCheatAdapter) Family() string                          { return a.family }
func (a legacyCheatAdapter) SystemSlug() string                      { return a.systemSlug }
func (a legacyCheatAdapter) RequiredParserID() string                { return a.requiredParserID }
func (a legacyCheatAdapter) MinimumParserLevel() string              { return a.minimumParserLevel }
func (a legacyCheatAdapter) SupportsRuntimeProfiles() bool           { return a.supportsRuntimeProfile }
func (a legacyCheatAdapter) SupportsLogicalSaves() bool              { return a.supportsLogicalSaves }
func (a legacyCheatAdapter) SupportsLiveUpload() bool                { return a.supportsLiveUpload }
func (a legacyCheatAdapter) MatchKeys() []string                     { return append([]string(nil), a.matchKeys...) }
func (a legacyCheatAdapter) IntegrityHooks() []cheatPayloadTransform { return nil }

func (a legacyCheatAdapter) Supports(summary saveSummary, inspection *saveInspection) bool {
	if canonicalSegment(summary.SystemSlug, "") != canonicalSegment(a.systemSlug, "") {
		return false
	}
	if !packMatchesSummary(a.basePack, summary) {
		return false
	}
	if inspection == nil {
		return true
	}
	if !parserLevelAtLeast(inspection.ParserLevel, a.minimumParserLevel) {
		return false
	}
	if strings.TrimSpace(a.requiredParserID) == "" {
		return true
	}
	return strings.TrimSpace(inspection.ParserID) == strings.TrimSpace(a.requiredParserID)
}

func (a legacyCheatAdapter) FieldCatalog() map[string]cheatField {
	out := map[string]cheatField{}
	for _, section := range a.basePack.Sections {
		for _, field := range section.Fields {
			out[field.ID] = field
		}
	}
	return out
}

func (a legacyCheatAdapter) PreparePack(pack cheatPack) (cheatPack, error) {
	return hydrateLegacyCheatPack(a.basePack, pack)
}

func (a legacyCheatAdapter) Read(_ cheatAdapterContext, pack cheatPack, payload []byte) (saveCheatEditorState, error) {
	return a.editor.Read(pack, payload)
}

func (a legacyCheatAdapter) Apply(_ cheatAdapterContext, pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error) {
	return a.editor.Apply(pack, payload, slotID, updates)
}

func parserLevelAtLeast(level, minimum string) bool {
	order := map[string]int{
		"":                        0,
		saveParserLevelNone:       0,
		saveParserLevelContainer:  1,
		saveParserLevelStructural: 2,
		saveParserLevelSemantic:   3,
	}
	return order[strings.TrimSpace(level)] >= order[strings.TrimSpace(minimum)]
}

func hydrateLegacyCheatPack(base cheatPack, overlay cheatPack) (cheatPack, error) {
	fieldCatalog := map[string]cheatField{}
	for _, section := range base.Sections {
		for _, field := range section.Fields {
			fieldCatalog[field.ID] = field
		}
	}
	out := overlay
	if strings.TrimSpace(out.GameID) == "" {
		out.GameID = base.GameID
	}
	if strings.TrimSpace(out.SystemSlug) == "" {
		out.SystemSlug = base.SystemSlug
	}
	if strings.TrimSpace(out.EditorID) == "" {
		out.EditorID = base.EditorID
	}
	if strings.TrimSpace(out.AdapterID) == "" {
		out.AdapterID = firstNonEmpty(base.AdapterID, base.EditorID)
	}
	if len(out.Match.TitleAliases) == 0 {
		out.Match.TitleAliases = append([]string(nil), base.Match.TitleAliases...)
	}
	if len(out.Payload.ExactSizes) == 0 {
		out.Payload.ExactSizes = append([]int(nil), base.Payload.ExactSizes...)
	}
	if len(out.Payload.Formats) == 0 {
		out.Payload.Formats = append([]string(nil), base.Payload.Formats...)
	}
	if out.Selector == nil && base.Selector != nil {
		selectorCopy := *base.Selector
		out.Selector = &selectorCopy
	}
	sections := make([]cheatSection, 0, len(overlay.Sections))
	for _, section := range overlay.Sections {
		hydrated := cheatSection{
			ID:    strings.TrimSpace(section.ID),
			Title: strings.TrimSpace(firstNonEmpty(section.Title, section.ID)),
		}
		for _, field := range section.Fields {
			ref := strings.TrimSpace(firstNonEmpty(field.Ref, field.ID))
			baseField, ok := fieldCatalog[ref]
			if !ok {
				return cheatPack{}, fmt.Errorf("unknown field ref %q for adapter %q", ref, out.AdapterID)
			}
			hydratedField := baseField
			hydratedField.ID = strings.TrimSpace(firstNonEmpty(field.ID, ref))
			hydratedField.Ref = ref
			hydratedField.Label = strings.TrimSpace(firstNonEmpty(field.Label, baseField.Label))
			hydratedField.Description = strings.TrimSpace(firstNonEmpty(field.Description, baseField.Description))
			if strings.TrimSpace(field.Type) != "" {
				hydratedField.Type = strings.TrimSpace(field.Type)
			}
			if field.Min != nil {
				hydratedField.Min = field.Min
			}
			if field.Max != nil {
				hydratedField.Max = field.Max
			}
			if field.Step != nil {
				hydratedField.Step = field.Step
			}
			if len(field.Options) > 0 {
				hydratedField.Options = append([]cheatOption(nil), field.Options...)
			}
			if len(field.Bits) > 0 {
				hydratedField.Bits = append([]cheatBitOption(nil), field.Bits...)
			}
			hydrated.Fields = append(hydrated.Fields, hydratedField)
		}
		sections = append(sections, hydrated)
	}
	out.Sections = sections
	return normalizeCheatPack(out, true)
}

func legacyAdapterFromPack(pack cheatPack, editor cheatEditor) legacyCheatAdapter {
	adapterID := firstNonEmpty(strings.TrimSpace(pack.AdapterID), strings.TrimSpace(pack.EditorID), strings.TrimSpace(editor.ID()))
	matchKeys := []string{"systemSlug", "titleAliases", "format", "payloadSize"}
	return legacyCheatAdapter{
		id:                     adapterID,
		kind:                   "legacy",
		family:                 adapterID,
		systemSlug:             pack.SystemSlug,
		minimumParserLevel:     saveParserLevelContainer,
		supportsRuntimeProfile: false,
		supportsLogicalSaves:   false,
		supportsLiveUpload:     true,
		matchKeys:              matchKeys,
		editor:                 editor,
		basePack:               pack,
	}
}

func sortCheatAdapters(adapters []cheatAdapter) {
	sort.SliceStable(adapters, func(i, j int) bool {
		left := adapters[i]
		right := adapters[j]
		if left.SystemSlug() == right.SystemSlug() {
			return left.ID() < right.ID()
		}
		return left.SystemSlug() < right.SystemSlug()
	})
}
