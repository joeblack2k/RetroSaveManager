package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type cheatEditor interface {
	ID() string
	Read(pack cheatPack, payload []byte) (saveCheatEditorState, error)
	Apply(pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error)
}

type cheatService struct {
	saveRoot      string
	curatedRoot   string
	packsBySystem map[string][]cheatPack
	editors       map[string]cheatEditor
}

func newCheatService(saveRoot string) (*cheatService, error) {
	curatedRoot, err := findCuratedCheatPackRoot()
	if err != nil {
		return nil, err
	}
	packs, err := loadCuratedCheatPacks(curatedRoot)
	if err != nil {
		return nil, err
	}
	packsBySystem := map[string][]cheatPack{}
	for _, pack := range packs {
		slug := canonicalSegment(pack.SystemSlug, "")
		packsBySystem[slug] = append(packsBySystem[slug], pack)
	}
	return &cheatService{
		saveRoot:      saveRoot,
		curatedRoot:   curatedRoot,
		packsBySystem: packsBySystem,
		editors: map[string]cheatEditor{
			"dkc-sram":    dkcSRAMCheatEditor{},
			"sm64-eeprom": sm64EEPROMCheatEditor{},
			"mk64-eeprom": mk64EEPROMCheatEditor{},
		},
	}, nil
}

func findCuratedCheatPackRoot() (string, error) {
	candidates := make([]string, 0, 12)
	if envRoot := strings.TrimSpace(os.Getenv("CHEAT_PACK_ROOT")); envRoot != "" {
		candidates = append(candidates, envRoot)
	}
	candidates = append(candidates, filepath.Join(string(filepath.Separator), "app", "contracts", "cheats", "packs"))
	cwd, err := os.Getwd()
	if err == nil {
		dir := cwd
		for depth := 0; depth < 8; depth++ {
			candidates = append(candidates, filepath.Join(dir, "contracts", "cheats", "packs"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		info, statErr := os.Stat(candidate)
		if statErr != nil || !info.IsDir() {
			continue
		}
		abs, absErr := filepath.Abs(candidate)
		if absErr != nil {
			return "", absErr
		}
		return abs, nil
	}
	return "", fmt.Errorf("curated cheat pack root not found")
}

func loadCuratedCheatPacks(root string) ([]cheatPack, error) {
	packs := make([]cheatPack, 0, 8)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		pack, loadErr := loadCheatPackFile(path)
		if loadErr != nil {
			return fmt.Errorf("load cheat pack %s: %w", path, loadErr)
		}
		packs = append(packs, pack)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(packs, func(i, j int) bool {
		if packs[i].SystemSlug == packs[j].SystemSlug {
			return packs[i].GameID < packs[j].GameID
		}
		return packs[i].SystemSlug < packs[j].SystemSlug
	})
	return packs, nil
}

func loadCheatPackFile(path string) (cheatPack, error) {
	return decodeCheatPackFile(path, true)
}

func loadCheatPackOverrideFile(path string) (cheatPack, error) {
	return decodeCheatPackFile(path, false)
}

func decodeCheatPackFile(path string, requireIdentity bool) (cheatPack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cheatPack{}, err
	}
	var pack cheatPack
	if err := yaml.Unmarshal(data, &pack); err != nil {
		return cheatPack{}, err
	}
	normalized, err := normalizeCheatPack(pack, requireIdentity)
	if err != nil {
		return cheatPack{}, err
	}
	return normalized, nil
}

func normalizeCheatPack(pack cheatPack, requireIdentity bool) (cheatPack, error) {
	pack.GameID = strings.TrimSpace(pack.GameID)
	pack.SystemSlug = canonicalSegment(pack.SystemSlug, "")
	pack.EditorID = strings.TrimSpace(pack.EditorID)
	pack.Title = strings.TrimSpace(pack.Title)
	if requireIdentity && (pack.GameID == "" || pack.SystemSlug == "" || pack.EditorID == "") {
		return cheatPack{}, fmt.Errorf("pack is missing gameId, systemSlug, or editorId")
	}
	pack.Match.TitleAliases = normalizeStringList(pack.Match.TitleAliases)
	if len(pack.Match.TitleAliases) == 0 && pack.Title != "" {
		pack.Match.TitleAliases = []string{pack.Title}
	}
	pack.Payload.Formats = normalizeStringList(pack.Payload.Formats)
	if pack.Selector != nil {
		pack.Selector.ID = strings.TrimSpace(pack.Selector.ID)
		pack.Selector.Label = strings.TrimSpace(pack.Selector.Label)
		pack.Selector.Type = strings.TrimSpace(pack.Selector.Type)
		for i := range pack.Selector.Options {
			pack.Selector.Options[i].ID = strings.TrimSpace(pack.Selector.Options[i].ID)
			pack.Selector.Options[i].Label = strings.TrimSpace(pack.Selector.Options[i].Label)
		}
	}
	sections := make([]cheatSection, 0, len(pack.Sections))
	for _, section := range pack.Sections {
		section.ID = strings.TrimSpace(section.ID)
		section.Title = strings.TrimSpace(section.Title)
		fields := make([]cheatField, 0, len(section.Fields))
		for _, field := range section.Fields {
			field.ID = strings.TrimSpace(field.ID)
			field.Label = strings.TrimSpace(field.Label)
			field.Description = strings.TrimSpace(field.Description)
			field.Type = strings.TrimSpace(field.Type)
			field.Op.Kind = strings.TrimSpace(field.Op.Kind)
			field.Op.Flag = strings.TrimSpace(field.Op.Flag)
			field.Op.Course = strings.TrimSpace(field.Op.Course)
			field.Op.Mode = strings.TrimSpace(field.Op.Mode)
			field.Op.Cup = strings.TrimSpace(field.Op.Cup)
			field.Op.Field = strings.TrimSpace(field.Op.Field)
			for i := range field.Options {
				field.Options[i].ID = strings.TrimSpace(field.Options[i].ID)
				field.Options[i].Label = strings.TrimSpace(field.Options[i].Label)
			}
			field.BitLabels = normalizeStringList(field.BitLabels)
			if len(field.Bits) == 0 && len(field.BitLabels) > 0 {
				field.Bits = make([]cheatBitOption, 0, len(field.BitLabels))
				for i, label := range field.BitLabels {
					field.Bits = append(field.Bits, cheatBitOption{ID: fmt.Sprintf("bit%d", i+1), Bit: i, Label: label})
				}
			}
			for i := range field.Bits {
				field.Bits[i].ID = strings.TrimSpace(field.Bits[i].ID)
				field.Bits[i].Label = strings.TrimSpace(field.Bits[i].Label)
			}
			if field.ID == "" || field.Label == "" || field.Type == "" || field.Op.Kind == "" {
				return cheatPack{}, fmt.Errorf("field is missing id, label, type, or op.kind")
			}
			fields = append(fields, field)
		}
		section.Fields = fields
		sections = append(sections, section)
	}
	pack.Sections = sections
	presets := make([]cheatPreset, 0, len(pack.Presets))
	for _, preset := range pack.Presets {
		preset.ID = strings.TrimSpace(preset.ID)
		preset.Label = strings.TrimSpace(preset.Label)
		preset.Description = strings.TrimSpace(preset.Description)
		if preset.ID == "" || preset.Label == "" {
			return cheatPack{}, fmt.Errorf("preset is missing id or label")
		}
		if preset.Updates == nil {
			preset.Updates = map[string]any{}
		}
		presets = append(presets, preset)
	}
	pack.Presets = presets
	return pack, nil
}

func normalizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func (s *cheatService) capabilityForRecord(record saveRecord) *cheatCapability {
	resolved, err := s.resolve(record)
	if err != nil || resolved == nil {
		return nil
	}
	return &cheatCapability{
		Supported:      true,
		AvailableCount: cheatAvailableCount(resolved.Pack),
		EditorID:       resolved.Pack.EditorID,
	}
}

func cheatAvailableCount(pack cheatPack) int {
	count := len(pack.Presets)
	for _, section := range pack.Sections {
		count += len(section.Fields)
	}
	return count
}

type resolvedCheatPack struct {
	Pack    cheatPack
	Payload []byte
	Editor  cheatEditor
	Record  saveRecord
	Summary saveSummary
}

func (s *cheatService) resolve(record saveRecord) (*resolvedCheatPack, error) {
	summary := canonicalSummaryForRecord(record)
	systemSlug := canonicalSegment(summary.SystemSlug, "")
	candidates := s.packsBySystem[systemSlug]
	if len(candidates) == 0 {
		return nil, nil
	}
	matchingPacks := make([]cheatPack, 0, len(candidates))
	for _, candidate := range candidates {
		if !packMatchesSummary(candidate, summary) {
			continue
		}
		merged := candidate
		if override, ok := s.loadLocalOverride(record); ok {
			combined, err := mergeCheatPacks(candidate, override)
			if err == nil {
				merged = combined
			}
		}
		if !packMatchesSummary(merged, summary) {
			continue
		}
		matchingPacks = append(matchingPacks, merged)
	}
	if len(matchingPacks) == 0 {
		return nil, nil
	}
	payload, err := os.ReadFile(record.payloadPath)
	if err != nil {
		return nil, err
	}
	for _, candidate := range matchingPacks {
		if !packMatchesPayload(candidate, record.Summary.Format, payload) {
			continue
		}
		editor := s.editors[candidate.EditorID]
		if editor == nil {
			continue
		}
		if _, err := editor.Read(candidate, payload); err != nil {
			continue
		}
		return &resolvedCheatPack{
			Pack:    candidate,
			Payload: payload,
			Editor:  editor,
			Record:  record,
			Summary: summary,
		}, nil
	}
	return nil, nil
}

func packMatchesSummary(pack cheatPack, summary saveSummary) bool {
	if canonicalSegment(pack.SystemSlug, "") != canonicalSegment(summary.SystemSlug, "") {
		return false
	}
	titles := []string{
		summary.DisplayTitle,
		summary.Game.DisplayTitle,
		summary.Game.Name,
	}
	for _, alias := range pack.Match.TitleAliases {
		for _, title := range titles {
			if cheatTitleKey(alias) != "" && cheatTitleKey(alias) == cheatTitleKey(title) {
				return true
			}
		}
	}
	return false
}

func cheatTitleKey(value string) string {
	display, _, _ := cleanupDisplayTitleRegionAndLanguages(value)
	if strings.TrimSpace(display) == "" {
		display = strings.TrimSpace(value)
	}
	return canonicalSegment(display, "")
}

func packMatchesPayload(pack cheatPack, format string, payload []byte) bool {
	if len(pack.Payload.ExactSizes) > 0 {
		matched := false
		for _, size := range pack.Payload.ExactSizes {
			if len(payload) == size {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(pack.Payload.Formats) == 0 {
		return true
	}
	normalizedFormat := strings.TrimSpace(strings.ToLower(format))
	for _, candidate := range pack.Payload.Formats {
		if strings.ToLower(candidate) == normalizedFormat {
			return true
		}
	}
	return false
}

func (s *cheatService) loadLocalOverride(record saveRecord) (cheatPack, bool) {
	if s == nil || strings.TrimSpace(s.saveRoot) == "" {
		return cheatPack{}, false
	}
	path, err := safeJoinUnderRoot(s.saveRoot, record.SystemPath, record.GamePath, "_rsm", "cheats.local.yaml")
	if err != nil {
		return cheatPack{}, false
	}
	info, statErr := os.Stat(path)
	if statErr != nil || info.IsDir() {
		return cheatPack{}, false
	}
	pack, err := loadCheatPackOverrideFile(path)
	if err != nil {
		return cheatPack{}, false
	}
	return pack, true
}

func mergeCheatPacks(base cheatPack, override cheatPack) (cheatPack, error) {
	merged := base
	if strings.TrimSpace(override.GameID) != "" {
		merged.GameID = strings.TrimSpace(override.GameID)
	}
	if strings.TrimSpace(override.SystemSlug) != "" {
		merged.SystemSlug = canonicalSegment(override.SystemSlug, merged.SystemSlug)
	}
	if strings.TrimSpace(override.EditorID) != "" {
		merged.EditorID = strings.TrimSpace(override.EditorID)
	}
	if strings.TrimSpace(override.Title) != "" {
		merged.Title = strings.TrimSpace(override.Title)
	}
	merged.Match.TitleAliases = normalizeStringList(append(append([]string{}, base.Match.TitleAliases...), override.Match.TitleAliases...))
	if len(override.Payload.ExactSizes) > 0 {
		merged.Payload.ExactSizes = append([]int{}, override.Payload.ExactSizes...)
	}
	if len(override.Payload.Formats) > 0 {
		merged.Payload.Formats = normalizeStringList(override.Payload.Formats)
	}
	if override.Selector != nil {
		selectorCopy := *override.Selector
		merged.Selector = &selectorCopy
	}
	sectionByID := map[string]int{}
	sections := append([]cheatSection{}, base.Sections...)
	for idx, section := range sections {
		sectionByID[section.ID] = idx
	}
	for _, section := range override.Sections {
		idx, ok := sectionByID[section.ID]
		if !ok {
			sections = append(sections, section)
			sectionByID[section.ID] = len(sections) - 1
			continue
		}
		mergedSection := sections[idx]
		if strings.TrimSpace(section.Title) != "" {
			mergedSection.Title = section.Title
		}
		fieldByID := map[string]int{}
		for fieldIdx, field := range mergedSection.Fields {
			fieldByID[field.ID] = fieldIdx
		}
		for _, field := range section.Fields {
			fieldIdx, fieldOK := fieldByID[field.ID]
			if !fieldOK {
				mergedSection.Fields = append(mergedSection.Fields, field)
				fieldByID[field.ID] = len(mergedSection.Fields) - 1
				continue
			}
			mergedSection.Fields[fieldIdx] = field
		}
		sections[idx] = mergedSection
	}
	merged.Sections = sections
	presetByID := map[string]int{}
	presets := append([]cheatPreset{}, base.Presets...)
	for idx, preset := range presets {
		presetByID[preset.ID] = idx
	}
	for _, preset := range override.Presets {
		if idx, ok := presetByID[preset.ID]; ok {
			presets[idx] = preset
			continue
		}
		presets = append(presets, preset)
	}
	merged.Presets = presets
	return normalizeCheatPack(merged, true)
}

func (s *cheatService) get(record saveRecord) (saveCheatEditorState, error) {
	resolved, err := s.resolve(record)
	if err != nil {
		return saveCheatEditorState{}, err
	}
	if resolved == nil {
		return saveCheatEditorState{Supported: false}, nil
	}
	state, err := resolved.Editor.Read(resolved.Pack, resolved.Payload)
	if err != nil {
		return saveCheatEditorState{}, err
	}
	state.Supported = true
	state.GameID = resolved.Pack.GameID
	state.SystemSlug = resolved.Pack.SystemSlug
	state.EditorID = resolved.Pack.EditorID
	state.Title = firstNonEmpty(resolved.Pack.Title, resolved.Summary.DisplayTitle)
	state.AvailableCount = cheatAvailableCount(resolved.Pack)
	state.Selector = resolved.Pack.Selector
	state.Sections = resolved.Pack.Sections
	state.Presets = resolved.Pack.Presets
	return state, nil
}

func (s *cheatService) apply(record saveRecord, req saveCheatApplyRequest) ([]byte, map[string]any, error) {
	resolved, err := s.resolve(record)
	if err != nil {
		return nil, nil, err
	}
	if resolved == nil {
		return nil, nil, errors.New("cheats are not available for this save")
	}
	if strings.TrimSpace(req.EditorID) != resolved.Pack.EditorID {
		return nil, nil, fmt.Errorf("unsupported editorId %q", req.EditorID)
	}
	updates, err := resolveCheatUpdates(resolved.Pack, req)
	if err != nil {
		return nil, nil, err
	}
	if len(updates) == 0 {
		return nil, nil, errors.New("no cheat updates were provided")
	}
	return resolved.Editor.Apply(resolved.Pack, resolved.Payload, strings.TrimSpace(req.SlotID), updates)
}

func resolveCheatUpdates(pack cheatPack, req saveCheatApplyRequest) (map[string]any, error) {
	fieldIndex := map[string]cheatField{}
	presetIndex := map[string]cheatPreset{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			fieldIndex[field.ID] = field
		}
	}
	for _, preset := range pack.Presets {
		presetIndex[preset.ID] = preset
	}
	merged := map[string]any{}
	for _, presetID := range req.PresetIDs {
		preset, ok := presetIndex[strings.TrimSpace(presetID)]
		if !ok {
			return nil, fmt.Errorf("unknown presetId %q", presetID)
		}
		for fieldID, value := range preset.Updates {
			merged[fieldID] = value
		}
	}
	for fieldID, raw := range req.Updates {
		field, ok := fieldIndex[fieldID]
		if !ok {
			return nil, fmt.Errorf("unknown field %q", fieldID)
		}
		decoded, err := decodeCheatFieldValue(field, raw)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", fieldID, err)
		}
		merged[fieldID] = decoded
	}
	validated := map[string]any{}
	for fieldID, value := range merged {
		field := fieldIndex[fieldID]
		normalized, err := normalizeCheatFieldValue(field, value)
		if err != nil {
			return nil, fmt.Errorf("validate %s: %w", fieldID, err)
		}
		validated[fieldID] = normalized
	}
	return validated, nil
}

func decodeCheatFieldValue(field cheatField, raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, errors.New("empty value")
	}
	switch field.Type {
	case "boolean":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	case "integer":
		var value int
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	case "enum":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	case "bitmask":
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, err
		}
		return values, nil
	default:
		return nil, fmt.Errorf("unsupported field type %q", field.Type)
	}
}

func normalizeCheatFieldValue(field cheatField, value any) (any, error) {
	switch field.Type {
	case "boolean":
		boolValue, ok := value.(bool)
		if !ok {
			return nil, errors.New("expected boolean")
		}
		return boolValue, nil
	case "integer":
		intValue, ok := value.(int)
		if !ok {
			return nil, errors.New("expected integer")
		}
		if field.Min != nil && intValue < *field.Min {
			return nil, fmt.Errorf("must be >= %d", *field.Min)
		}
		if field.Max != nil && intValue > *field.Max {
			return nil, fmt.Errorf("must be <= %d", *field.Max)
		}
		return intValue, nil
	case "enum":
		stringValue, ok := value.(string)
		if !ok {
			return nil, errors.New("expected string")
		}
		for _, option := range field.Options {
			if option.ID == stringValue {
				return stringValue, nil
			}
		}
		return nil, fmt.Errorf("invalid option %q", stringValue)
	case "bitmask":
		values, ok := value.([]string)
		if !ok {
			return nil, errors.New("expected string array")
		}
		allowed := map[string]struct{}{}
		for _, bit := range field.Bits {
			allowed[bit.ID] = struct{}{}
		}
		seen := map[string]struct{}{}
		out := make([]string, 0, len(values))
		for _, item := range values {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := allowed[item]; !ok {
				return nil, fmt.Errorf("invalid bit %q", item)
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
		sort.Strings(out)
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported field type %q", field.Type)
	}
}
