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
	saveRoot     string
	curatedRoot  string
	runtimeStore *cheatRuntimeStore
	modules      *gameModuleService
	builtinPacks []cheatPack
	adapters     map[string]cheatAdapter
	adapterList  []cheatAdapter
}

func (s *cheatService) setModuleService(modules *gameModuleService) {
	if s != nil {
		s.modules = modules
	}
}

type resolvedCheatPack struct {
	Managed cheatManagedPack
	Logic   cheatPack
	Payload []byte
	Adapter cheatAdapter
	Record  saveRecord
	Summary saveSummary
	Context cheatAdapterContext
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
	runtimeStore, err := newCheatRuntimeStore(saveRoot)
	if err != nil {
		return nil, err
	}
	editors := builtinCheatEditors()
	adapters := map[string]cheatAdapter{}
	for i := range packs {
		packs[i].PackID = firstNonEmpty(packs[i].PackID, canonicalCheatPackID(firstNonEmpty(packs[i].GameID, packs[i].Title)))
		packs[i].SchemaVersion = maxInt(packs[i].SchemaVersion, 1)
		packs[i].AdapterID = firstNonEmpty(strings.TrimSpace(packs[i].AdapterID), strings.TrimSpace(packs[i].EditorID))
		editor := editors[packs[i].EditorID]
		if editor == nil {
			continue
		}
		if _, exists := adapters[packs[i].AdapterID]; exists {
			continue
		}
		adapters[packs[i].AdapterID] = legacyAdapterFromPack(packs[i], editor)
	}
	adapterList := make([]cheatAdapter, 0, len(adapters))
	for _, adapter := range adapters {
		adapterList = append(adapterList, adapter)
	}
	sortCheatAdapters(adapterList)
	return &cheatService{
		saveRoot:     saveRoot,
		curatedRoot:  curatedRoot,
		runtimeStore: runtimeStore,
		builtinPacks: packs,
		adapters:     adapters,
		adapterList:  adapterList,
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
	return decodeCheatPackData(data, requireIdentity)
}

func decodeCheatPackData(data []byte, requireIdentity bool) (cheatPack, error) {
	var pack cheatPack
	if err := yaml.Unmarshal(data, &pack); err != nil {
		return cheatPack{}, err
	}
	return normalizeCheatPack(pack, requireIdentity)
}

func normalizeCheatPack(pack cheatPack, requireIdentity bool) (cheatPack, error) {
	pack.PackID = canonicalCheatPackID(pack.PackID)
	pack.SchemaVersion = maxInt(pack.SchemaVersion, 0)
	pack.AdapterID = strings.TrimSpace(pack.AdapterID)
	pack.GameID = strings.TrimSpace(pack.GameID)
	pack.SystemSlug = canonicalSegment(pack.SystemSlug, "")
	pack.EditorID = strings.TrimSpace(pack.EditorID)
	pack.Title = strings.TrimSpace(pack.Title)
	if pack.AdapterID == "" {
		pack.AdapterID = strings.TrimSpace(pack.EditorID)
	}
	if pack.PackID == "" {
		pack.PackID = canonicalCheatPackID(firstNonEmpty(pack.GameID, pack.Title))
	}
	if requireIdentity && (pack.GameID == "" || pack.SystemSlug == "" || firstNonEmpty(pack.AdapterID, pack.EditorID) == "") {
		return cheatPack{}, fmt.Errorf("pack is missing gameId, systemSlug, or adapter/editor identity")
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
			field.Ref = strings.TrimSpace(firstNonEmpty(field.Ref, field.ID))
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
			if field.ID == "" || field.Label == "" || field.Type == "" {
				return cheatPack{}, fmt.Errorf("field is missing id, label, or type")
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
		AvailableCount: cheatAvailableCount(resolved.Managed.Pack),
		EditorID:       resolved.Logic.EditorID,
		AdapterID:      resolved.Adapter.ID(),
		PackID:         resolved.Managed.Manifest.PackID,
	}
}

func cheatAvailableCount(pack cheatPack) int {
	count := len(pack.Presets)
	for _, section := range pack.Sections {
		count += len(section.Fields)
	}
	return count
}

func (s *cheatService) resolve(record saveRecord) (*resolvedCheatPack, error) {
	summary := canonicalSummaryForRecord(record)
	inspection := summary.Inspection
	adapter := s.resolveAdapter(summary)
	if adapter == nil {
		return nil, nil
	}
	payload, err := os.ReadFile(record.payloadPath)
	if err != nil {
		return nil, err
	}
	managedPacks, err := s.listManagedPacks()
	if err != nil {
		return nil, err
	}
	ctx := cheatAdapterContext{
		Record:     record,
		Summary:    summary,
		Inspection: inspection,
	}
	for _, managed := range managedPacks {
		if managed.Manifest.Status != cheatPackStatusActive {
			continue
		}
		if strings.TrimSpace(managed.Pack.AdapterID) != adapter.ID() {
			continue
		}
		if !packMatchesSummary(managed.Pack, summary) {
			continue
		}
		logicPack := managed.Pack
		if override, ok := s.loadLocalOverride(record); ok {
			combined, mergeErr := mergeCheatPacks(logicPack, override)
			if mergeErr == nil {
				logicPack = combined
			}
		}
		if !packMatchesPayload(logicPack, record.Summary.Format, payload) {
			continue
		}
		prepared, prepErr := adapter.PreparePack(logicPack)
		if prepErr != nil {
			continue
		}
		if _, err := adapter.Read(ctx, prepared, payload); err != nil {
			continue
		}
		effectiveManaged := managed
		effectiveManaged.Pack = prepared
		return &resolvedCheatPack{
			Managed: effectiveManaged,
			Logic:   prepared,
			Payload: payload,
			Adapter: adapter,
			Record:  record,
			Summary: summary,
			Context: ctx,
		}, nil
	}
	return nil, nil
}

func (s *cheatService) resolveAdapter(summary saveSummary) cheatAdapter {
	for _, adapter := range s.allAdapterList() {
		if adapter.Supports(summary, summary.Inspection) {
			return adapter
		}
	}
	return nil
}

func (s *cheatService) allAdapterList() []cheatAdapter {
	if s == nil {
		return nil
	}
	items := append([]cheatAdapter{}, s.adapterList...)
	if s.modules != nil {
		items = append(items, s.modules.moduleAdapters()...)
	}
	sortCheatAdapters(items)
	return items
}

func (s *cheatService) adapterByID(id string) cheatAdapter {
	target := strings.TrimSpace(id)
	if target == "" || s == nil {
		return nil
	}
	if adapter := s.adapters[target]; adapter != nil {
		return adapter
	}
	if s.modules != nil {
		for _, adapter := range s.modules.moduleAdapters() {
			if adapter.ID() == target {
				return adapter
			}
		}
	}
	return nil
}

func packMatchesSummary(pack cheatPack, summary saveSummary) bool {
	if canonicalSegment(pack.SystemSlug, "") != canonicalSegment(summary.SystemSlug, "") {
		return false
	}
	if summary.Inspection != nil && strings.TrimSpace(pack.GameID) != "" && strings.TrimSpace(summary.Inspection.ValidatedGameID) != "" {
		if strings.EqualFold(strings.TrimSpace(pack.GameID), strings.TrimSpace(summary.Inspection.ValidatedGameID)) {
			return true
		}
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
	if strings.TrimSpace(override.PackID) != "" {
		merged.PackID = canonicalCheatPackID(override.PackID)
	}
	if override.SchemaVersion > 0 {
		merged.SchemaVersion = override.SchemaVersion
	}
	if strings.TrimSpace(override.GameID) != "" {
		merged.GameID = strings.TrimSpace(override.GameID)
	}
	if strings.TrimSpace(override.SystemSlug) != "" {
		merged.SystemSlug = canonicalSegment(override.SystemSlug, merged.SystemSlug)
	}
	if strings.TrimSpace(override.EditorID) != "" {
		merged.EditorID = strings.TrimSpace(override.EditorID)
	}
	if strings.TrimSpace(override.AdapterID) != "" {
		merged.AdapterID = strings.TrimSpace(override.AdapterID)
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
	state, err := resolved.Adapter.Read(resolved.Context, resolved.Logic, resolved.Payload)
	if err != nil {
		return saveCheatEditorState{}, err
	}
	state.Supported = true
	state.GameID = resolved.Managed.Pack.GameID
	state.SystemSlug = resolved.Managed.Pack.SystemSlug
	state.EditorID = resolved.Logic.EditorID
	state.AdapterID = resolved.Adapter.ID()
	state.PackID = resolved.Managed.Manifest.PackID
	state.Title = firstNonEmpty(resolved.Managed.Pack.Title, resolved.Summary.DisplayTitle)
	state.AvailableCount = cheatAvailableCount(resolved.Managed.Pack)
	state.Selector = resolved.Managed.Pack.Selector
	state.Sections = resolved.Managed.Pack.Sections
	state.Presets = resolved.Managed.Pack.Presets
	return state, nil
}

func (s *cheatService) apply(record saveRecord, req saveCheatApplyRequest) ([]byte, map[string]any, []string, *resolvedCheatPack, error) {
	resolved, err := s.resolve(record)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if resolved == nil {
		return nil, nil, nil, nil, errors.New("cheats are not available for this save")
	}
	requestedAdapterID := strings.TrimSpace(firstNonEmpty(req.AdapterID, req.EditorID))
	if requestedAdapterID == "" {
		return nil, nil, nil, nil, errors.New("adapterId or editorId is required")
	}
	if requestedAdapterID != resolved.Adapter.ID() && requestedAdapterID != resolved.Logic.EditorID {
		return nil, nil, nil, nil, fmt.Errorf("unsupported adapter/editor %q", requestedAdapterID)
	}
	updates, err := resolveCheatUpdates(resolved.Logic, req)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if len(updates) == 0 {
		return nil, nil, nil, nil, errors.New("no cheat updates were provided")
	}
	payload, changed, err := resolved.Adapter.Apply(resolved.Context, resolved.Logic, resolved.Payload, strings.TrimSpace(req.SlotID), updates)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	integritySteps := make([]string, 0, 4)
	for _, transform := range resolved.Adapter.IntegrityHooks() {
		payload, err = transform.Apply(payload, resolved.Context)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		integritySteps = append(integritySteps, transform.Name())
	}
	return payload, changed, integritySteps, resolved, nil
}

func (s *cheatService) listManagedPacks() ([]cheatManagedPack, error) {
	runtimePacks, err := s.runtimeStore.listRuntimePacks()
	if err != nil {
		return nil, err
	}
	modulePacks := []cheatManagedPack{}
	if s.modules != nil {
		modulePacks, err = s.modules.managedCheatPacks()
		if err != nil {
			return nil, err
		}
	}
	tombstones, err := s.runtimeStore.loadTombstones()
	if err != nil {
		return nil, err
	}
	out := make([]cheatManagedPack, 0, len(runtimePacks)+len(modulePacks)+len(s.builtinPacks))
	seenBuiltin := map[string]struct{}{}
	for _, item := range runtimePacks {
		item.Pack.PackID = firstNonEmpty(item.Pack.PackID, item.Manifest.PackID)
		item.Pack.AdapterID = firstNonEmpty(item.Pack.AdapterID, item.Manifest.AdapterID, item.Pack.EditorID)
		item.Manifest.PackID = firstNonEmpty(item.Manifest.PackID, item.Pack.PackID)
		item.Manifest.AdapterID = firstNonEmpty(item.Manifest.AdapterID, item.Pack.AdapterID, item.Pack.EditorID)
		item.SupportsSaveUI = item.Manifest.Status == cheatPackStatusActive
		out = append(out, item)
		seenBuiltin[item.Manifest.PackID] = struct{}{}
	}
	for _, item := range modulePacks {
		item.Pack.PackID = firstNonEmpty(item.Pack.PackID, item.Manifest.PackID)
		item.Pack.AdapterID = firstNonEmpty(item.Pack.AdapterID, item.Manifest.AdapterID, item.Pack.EditorID)
		item.Manifest.PackID = firstNonEmpty(item.Manifest.PackID, item.Pack.PackID)
		item.Manifest.AdapterID = firstNonEmpty(item.Manifest.AdapterID, item.Pack.AdapterID, item.Pack.EditorID)
		if _, exists := seenBuiltin[item.Manifest.PackID]; exists {
			continue
		}
		out = append(out, item)
		seenBuiltin[item.Manifest.PackID] = struct{}{}
	}
	for _, pack := range s.builtinPacks {
		packID := firstNonEmpty(pack.PackID, canonicalCheatPackID(firstNonEmpty(pack.GameID, pack.Title)))
		if _, ok := seenBuiltin[packID]; ok {
			continue
		}
		status := cheatPackStatusActive
		if tombstone, ok := tombstones[packID]; ok {
			status = firstNonEmpty(tombstone.Status, cheatPackStatusDeleted)
		}
		out = append(out, cheatManagedPack{
			Pack: pack,
			Manifest: cheatPackManifest{
				PackID:    packID,
				AdapterID: firstNonEmpty(pack.AdapterID, pack.EditorID),
				Source:    cheatPackSourceBuiltin,
				Status:    status,
			},
			Builtin:        true,
			SupportsSaveUI: status == cheatPackStatusActive,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Builtin != out[j].Builtin {
			return !out[i].Builtin
		}
		if out[i].Manifest.UpdatedAt.Equal(out[j].Manifest.UpdatedAt) {
			return out[i].Manifest.PackID < out[j].Manifest.PackID
		}
		return out[i].Manifest.UpdatedAt.After(out[j].Manifest.UpdatedAt)
	})
	return out, nil
}

func (s *cheatService) getManagedPack(packID string) (cheatManagedPack, error) {
	packs, err := s.listManagedPacks()
	if err != nil {
		return cheatManagedPack{}, err
	}
	target := canonicalCheatPackID(packID)
	for _, item := range packs {
		if canonicalCheatPackID(item.Manifest.PackID) == target {
			return item, nil
		}
	}
	return cheatManagedPack{}, fmt.Errorf("cheat pack %q not found", packID)
}

func (s *cheatService) createManagedPack(req cheatPackCreateRequest, principal map[string]any) (cheatManagedPack, error) {
	pack, err := decodeCheatPackData([]byte(req.YAML), true)
	if err != nil {
		return cheatManagedPack{}, err
	}
	if _, err := s.validateLiveCheatPack(pack); err != nil {
		return cheatManagedPack{}, err
	}
	publishedBy := strings.TrimSpace(firstNonEmpty(req.PublishedBy, principalString(principal, "email"), principalString(principal, "displayName")))
	source := normalizeCheatPackSource(req.Source)
	managed, err := s.runtimeStore.writePack(pack, cheatPackManifest{
		PackID:      pack.PackID,
		AdapterID:   pack.AdapterID,
		Source:      source,
		Status:      cheatPackStatusActive,
		PublishedBy: publishedBy,
		Notes:       strings.TrimSpace(req.Notes),
	})
	if err != nil {
		return cheatManagedPack{}, err
	}
	return managed, nil
}

func (s *cheatService) validateLiveCheatPack(pack cheatPack) (cheatPack, error) {
	if pack.SchemaVersion <= 0 {
		return cheatPack{}, errors.New("schemaVersion is required")
	}
	if strings.TrimSpace(pack.AdapterID) == "" {
		return cheatPack{}, errors.New("adapterId is required")
	}
	adapter := s.adapterByID(strings.TrimSpace(pack.AdapterID))
	if adapter == nil {
		return cheatPack{}, fmt.Errorf("unknown adapterId %q", pack.AdapterID)
	}
	if !adapter.SupportsLiveUpload() {
		return cheatPack{}, fmt.Errorf("adapter %q does not accept live uploads", pack.AdapterID)
	}
	if len(pack.Sections) == 0 {
		return cheatPack{}, errors.New("pack must contain at least one section")
	}
	if err := validateCheatPackForAdapter(adapter, pack); err != nil {
		return cheatPack{}, err
	}
	prepared, err := adapter.PreparePack(pack)
	if err != nil {
		return cheatPack{}, err
	}
	return prepared, nil
}

func (s *cheatService) deleteManagedPack(packID string, principal map[string]any) (cheatManagedPack, error) {
	item, err := s.getManagedPack(packID)
	if err != nil {
		return cheatManagedPack{}, err
	}
	if item.Manifest.Source == cheatPackSourceModule && s.modules != nil {
		if _, ok, moduleErr := s.modules.setStatusByPackID(item.Manifest.PackID, gameModuleStatusDeleted); moduleErr != nil {
			return cheatManagedPack{}, moduleErr
		} else if ok {
			return s.getManagedPack(packID)
		}
	}
	if item.Builtin {
		if err := s.runtimeStore.writeTombstone(item.Manifest.PackID, cheatPackStatusDeleted, principalString(principal, "email"), item.Manifest.Source); err != nil {
			return cheatManagedPack{}, err
		}
		return s.getManagedPack(packID)
	}
	return s.runtimeStore.updateRuntimePackStatus(item.Manifest.PackID, cheatPackStatusDeleted)
}

func (s *cheatService) disableManagedPack(packID string, principal map[string]any) (cheatManagedPack, error) {
	item, err := s.getManagedPack(packID)
	if err != nil {
		return cheatManagedPack{}, err
	}
	if item.Manifest.Source == cheatPackSourceModule && s.modules != nil {
		if _, ok, moduleErr := s.modules.setStatusByPackID(item.Manifest.PackID, gameModuleStatusDisabled); moduleErr != nil {
			return cheatManagedPack{}, moduleErr
		} else if ok {
			return s.getManagedPack(packID)
		}
	}
	if item.Builtin {
		if err := s.runtimeStore.writeTombstone(item.Manifest.PackID, cheatPackStatusDisabled, principalString(principal, "email"), item.Manifest.Source); err != nil {
			return cheatManagedPack{}, err
		}
		return s.getManagedPack(packID)
	}
	return s.runtimeStore.updateRuntimePackStatus(item.Manifest.PackID, cheatPackStatusDisabled)
}

func (s *cheatService) enableManagedPack(packID string) (cheatManagedPack, error) {
	item, err := s.getManagedPack(packID)
	if err != nil {
		return cheatManagedPack{}, err
	}
	if item.Manifest.Source == cheatPackSourceModule && s.modules != nil {
		if _, ok, moduleErr := s.modules.setStatusByPackID(item.Manifest.PackID, gameModuleStatusActive); moduleErr != nil {
			return cheatManagedPack{}, moduleErr
		} else if ok {
			return s.getManagedPack(packID)
		}
	}
	if item.Builtin {
		if err := s.runtimeStore.clearTombstone(item.Manifest.PackID); err != nil {
			return cheatManagedPack{}, err
		}
		return s.getManagedPack(packID)
	}
	return s.runtimeStore.updateRuntimePackStatus(item.Manifest.PackID, cheatPackStatusActive)
}

func (s *cheatService) listAdapterDescriptors() []cheatAdapterDescriptor {
	adapters := s.allAdapterList()
	items := make([]cheatAdapterDescriptor, 0, len(adapters))
	for _, adapter := range adapters {
		items = append(items, cheatAdapterDescriptor{
			ID:                     adapter.ID(),
			Kind:                   adapter.Kind(),
			Family:                 adapter.Family(),
			SystemSlug:             adapter.SystemSlug(),
			RequiredParserID:       adapter.RequiredParserID(),
			MinimumParserLevel:     adapter.MinimumParserLevel(),
			SupportsRuntimeProfile: adapter.SupportsRuntimeProfiles(),
			SupportsLogicalSaves:   adapter.SupportsLogicalSaves(),
			SupportsLiveUpload:     adapter.SupportsLiveUpload(),
			MatchKeys:              adapter.MatchKeys(),
		})
	}
	return items
}

func (s *cheatService) adapterDescriptor(id string) (cheatAdapterDescriptor, error) {
	target := strings.TrimSpace(id)
	for _, item := range s.listAdapterDescriptors() {
		if item.ID == target {
			return item, nil
		}
	}
	return cheatAdapterDescriptor{}, fmt.Errorf("adapter %q not found", id)
}

func validateCheatPackForAdapter(adapter cheatAdapter, pack cheatPack) error {
	fieldCatalog := adapter.FieldCatalog()
	if len(fieldCatalog) == 0 {
		return fmt.Errorf("adapter %q has no field catalog", adapter.ID())
	}
	for _, section := range pack.Sections {
		if strings.TrimSpace(section.ID) == "" {
			return errors.New("section id is required")
		}
		if len(section.Fields) == 0 {
			return fmt.Errorf("section %q has no fields", section.ID)
		}
		for _, field := range section.Fields {
			ref := strings.TrimSpace(firstNonEmpty(field.Ref, field.ID))
			if _, ok := fieldCatalog[ref]; !ok {
				return fmt.Errorf("unknown field ref %q for adapter %q", ref, adapter.ID())
			}
		}
	}
	return nil
}

func normalizeCheatPackSource(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case cheatPackSourceWorker:
		return cheatPackSourceWorker
	case cheatPackSourceBuiltin:
		return cheatPackSourceBuiltin
	case cheatPackSourceGithub:
		return cheatPackSourceGithub
	default:
		return cheatPackSourceUploaded
	}
}

func principalString(principal map[string]any, key string) string {
	if principal == nil {
		return ""
	}
	if value, ok := principal[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
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
		values, err := normalizeCheatStringArray(value)
		if err != nil {
			return nil, err
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

func normalizeCheatStringArray(value any) ([]string, error) {
	if values, ok := value.([]string); ok {
		return values, nil
	}
	if values, ok := value.([]any); ok {
		out := make([]string, 0, len(values))
		for _, item := range values {
			text, ok := item.(string)
			if !ok {
				return nil, errors.New("expected string array")
			}
			out = append(out, text)
		}
		return out, nil
	}
	return nil, errors.New("expected string array")
}
