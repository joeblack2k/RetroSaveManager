package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"gopkg.in/yaml.v3"
)

const (
	gameModuleSchemaVersion = 1
	gameModuleABIVersion    = "rsm-wasm-json-v1"

	gameModuleStatusActive   = "active"
	gameModuleStatusDisabled = "disabled"
	gameModuleStatusDeleted  = "deleted"
	gameModuleStatusError    = "error"

	gameModuleSourceGithub   = "github"
	gameModuleSourceUploaded = "uploaded"

	defaultModuleLibraryRepo = "joeblack2k/RetroSaveManager"
	defaultModuleLibraryRef  = "main"
	defaultModuleLibraryPath = "modules"

	maxGameModuleZipBytes    = 8 << 20
	maxGameModuleFileBytes   = 4 << 20
	maxGameModuleWASMBytes   = 2 << 20
	maxGameModuleOutputBytes = 2 << 20
	maxGameModuleFiles       = 128
)

type gameModuleService struct {
	saveRoot string
	root     string
	modsRoot string
	status   string
}

type gameModuleManifest struct {
	ModuleID      string                  `json:"moduleId" yaml:"moduleId"`
	SchemaVersion int                     `json:"schemaVersion" yaml:"schemaVersion"`
	Version       string                  `json:"version" yaml:"version"`
	SystemSlug    string                  `json:"systemSlug" yaml:"systemSlug"`
	GameID        string                  `json:"gameId" yaml:"gameId"`
	Title         string                  `json:"title" yaml:"title"`
	ParserID      string                  `json:"parserId" yaml:"parserId"`
	WASMFile      string                  `json:"wasmFile" yaml:"wasmFile"`
	ABIVersion    string                  `json:"abiVersion" yaml:"abiVersion"`
	CheatPacks    []gameModuleCheatPack   `json:"cheatPacks,omitempty" yaml:"cheatPacks,omitempty"`
	Payload       gameModulePayloadPolicy `json:"payload" yaml:"payload"`
	TitleAliases  []string                `json:"titleAliases,omitempty" yaml:"titleAliases,omitempty"`
	ROMHashes     []string                `json:"romHashes,omitempty" yaml:"romHashes,omitempty"`
}

type gameModuleCheatPack struct {
	Path string `json:"path" yaml:"path"`
}

type gameModulePayloadPolicy struct {
	ExactSizes []int    `json:"exactSizes,omitempty" yaml:"exactSizes,omitempty"`
	Formats    []string `json:"formats,omitempty" yaml:"formats,omitempty"`
}

type gameModuleRecord struct {
	Manifest       gameModuleManifest `json:"manifest"`
	Status         string             `json:"status"`
	Source         string             `json:"source"`
	SourcePath     string             `json:"sourcePath,omitempty"`
	SourceRevision string             `json:"sourceRevision,omitempty"`
	SourceSHA256   string             `json:"sourceSha256,omitempty"`
	ImportedAt     time.Time          `json:"importedAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
	LastSyncedAt   *time.Time         `json:"lastSyncedAt,omitempty"`
	Errors         []string           `json:"errors,omitempty"`
	CheatPackIDs   []string           `json:"cheatPackIds,omitempty"`
}

type gameModuleLibraryConfig struct {
	Repo string `json:"repo"`
	Ref  string `json:"ref"`
	Path string `json:"path"`
}

type gameModuleSyncImported struct {
	Path       string `json:"path"`
	ModuleID   string `json:"moduleId,omitempty"`
	Title      string `json:"title,omitempty"`
	SystemSlug string `json:"systemSlug,omitempty"`
	Status     string `json:"status,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
}

type gameModuleSyncError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type gameModuleLibraryStatus struct {
	Config        gameModuleLibraryConfig  `json:"config"`
	LastSyncedAt  *time.Time               `json:"lastSyncedAt,omitempty"`
	ImportedCount int                      `json:"importedCount"`
	ErrorCount    int                      `json:"errorCount"`
	Imported      []gameModuleSyncImported `json:"imported"`
	Errors        []gameModuleSyncError    `json:"errors"`
}

type gameModuleListResponse struct {
	Success bool                    `json:"success"`
	Modules []gameModuleRecord      `json:"modules"`
	Library gameModuleLibraryStatus `json:"library"`
}

type gameModuleResponse struct {
	Success bool             `json:"success"`
	Module  gameModuleRecord `json:"module"`
}

type gameModuleSyncResponse struct {
	Success bool                    `json:"success"`
	Library gameModuleLibraryStatus `json:"library"`
}

type gameModuleRescanResponse struct {
	Success bool             `json:"success"`
	Result  saveRescanResult `json:"result"`
}

type gameModuleSourceInfo struct {
	Source         string
	SourcePath     string
	SourceRevision string
	SourceSHA256   string
	LastSyncedAt   *time.Time
}

type gameModuleParseRequest struct {
	Payload      []byte         `json:"payload"`
	Filename     string         `json:"filename,omitempty"`
	Format       string         `json:"format,omitempty"`
	SystemSlug   string         `json:"systemSlug,omitempty"`
	DisplayTitle string         `json:"displayTitle,omitempty"`
	ROMSHA1      string         `json:"romSha1,omitempty"`
	ROMMD5       string         `json:"romMd5,omitempty"`
	SlotName     string         `json:"slotName,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type gameModuleParseResponse struct {
	Supported          bool           `json:"supported"`
	ParserLevel        string         `json:"parserLevel,omitempty"`
	ParserID           string         `json:"parserId,omitempty"`
	ValidatedSystem    string         `json:"validatedSystem,omitempty"`
	ValidatedGameID    string         `json:"validatedGameId,omitempty"`
	ValidatedGameTitle string         `json:"validatedGameTitle,omitempty"`
	TrustLevel         string         `json:"trustLevel,omitempty"`
	Evidence           []string       `json:"evidence,omitempty"`
	Warnings           []string       `json:"warnings,omitempty"`
	SlotCount          int            `json:"slotCount,omitempty"`
	ActiveSlotIndexes  []int          `json:"activeSlotIndexes,omitempty"`
	ChecksumValid      *bool          `json:"checksumValid,omitempty"`
	SemanticFields     map[string]any `json:"semanticFields,omitempty"`
}

type gameModuleCheatRequest struct {
	Payload    []byte          `json:"payload"`
	Pack       cheatPack       `json:"pack"`
	SlotID     string          `json:"slotId,omitempty"`
	Updates    map[string]any  `json:"updates,omitempty"`
	Summary    saveSummary     `json:"summary"`
	Inspection *saveInspection `json:"inspection,omitempty"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
}

type gameModuleCheatApplyResponse struct {
	Payload        []byte         `json:"payload"`
	Changed        map[string]any `json:"changed,omitempty"`
	IntegritySteps []string       `json:"integritySteps,omitempty"`
}

func newGameModuleService(saveRoot string) (*gameModuleService, error) {
	root, err := safeJoinUnderRoot(saveRoot, "_rsm", "modules")
	if err != nil {
		return nil, err
	}
	modsRoot := filepath.Join(root, "installed")
	for _, dir := range []string{root, modsRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return &gameModuleService{
		saveRoot: saveRoot,
		root:     root,
		modsRoot: modsRoot,
		status:   filepath.Join(root, "library-status.json"),
	}, nil
}

func (s *gameModuleService) listModules() ([]gameModuleRecord, error) {
	if s == nil {
		return nil, nil
	}
	items := make([]gameModuleRecord, 0, 8)
	err := filepath.WalkDir(s.modsRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "module.json" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var record gameModuleRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return err
		}
		items = append(items, record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Manifest.SystemSlug == items[j].Manifest.SystemSlug {
			return items[i].Manifest.Title < items[j].Manifest.Title
		}
		return items[i].Manifest.SystemSlug < items[j].Manifest.SystemSlug
	})
	return items, nil
}

func (s *gameModuleService) moduleDir(moduleID string) string {
	return filepath.Join(s.modsRoot, canonicalGameModuleID(moduleID))
}

func (s *gameModuleService) moduleRecordPath(moduleID string) string {
	return filepath.Join(s.moduleDir(moduleID), "module.json")
}

func (s *gameModuleService) readModule(moduleID string) (gameModuleRecord, error) {
	data, err := os.ReadFile(s.moduleRecordPath(moduleID))
	if err != nil {
		return gameModuleRecord{}, fmt.Errorf("module %q not found", moduleID)
	}
	var record gameModuleRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return gameModuleRecord{}, err
	}
	return record, nil
}

func (s *gameModuleService) writeModuleRecord(record gameModuleRecord) error {
	moduleID := canonicalGameModuleID(record.Manifest.ModuleID)
	if moduleID == "" {
		return errors.New("moduleId is required")
	}
	record.Manifest.ModuleID = moduleID
	if record.ImportedAt.IsZero() {
		record.ImportedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.moduleDir(moduleID), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(s.moduleRecordPath(moduleID), data, 0o644)
}

func (s *gameModuleService) importZip(ctx context.Context, zipData []byte, source gameModuleSourceInfo) (gameModuleRecord, error) {
	if s == nil {
		return gameModuleRecord{}, errors.New("module service is not initialized")
	}
	if len(zipData) == 0 {
		return gameModuleRecord{}, errors.New("module zip is empty")
	}
	if len(zipData) > maxGameModuleZipBytes {
		return gameModuleRecord{}, fmt.Errorf("module zip exceeds %d bytes", maxGameModuleZipBytes)
	}
	files, err := readGameModuleZip(zipData)
	if err != nil {
		return gameModuleRecord{}, err
	}
	manifestData := files["rsm-module.yaml"]
	if len(manifestData) == 0 {
		return gameModuleRecord{}, errors.New("rsm-module.yaml is required")
	}
	var manifest gameModuleManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		return gameModuleRecord{}, fmt.Errorf("decode rsm-module.yaml: %w", err)
	}
	manifest, err = normalizeGameModuleManifest(manifest, files)
	if err != nil {
		return gameModuleRecord{}, err
	}
	wasmData := files[manifest.WASMFile]
	if len(wasmData) == 0 {
		return gameModuleRecord{}, fmt.Errorf("wasmFile %q not found", manifest.WASMFile)
	}
	if err := validateGameModuleWASM(ctx, manifest.ModuleID, wasmData); err != nil {
		return gameModuleRecord{}, err
	}
	packs, packPaths, err := loadGameModuleCheatPacks(manifest, files)
	if err != nil {
		return gameModuleRecord{}, err
	}
	existing, _ := s.readModule(manifest.ModuleID)
	status := gameModuleStatusActive
	if existing.Status == gameModuleStatusDisabled || existing.Status == gameModuleStatusDeleted {
		status = existing.Status
	}
	record := gameModuleRecord{
		Manifest:       manifest,
		Status:         status,
		Source:         firstNonEmpty(source.Source, gameModuleSourceUploaded),
		SourcePath:     source.SourcePath,
		SourceRevision: source.SourceRevision,
		SourceSHA256:   firstNonEmpty(source.SourceSHA256, sha256Hex(zipData)),
		ImportedAt:     existing.ImportedAt,
		LastSyncedAt:   source.LastSyncedAt,
		CheatPackIDs:   make([]string, 0, len(packs)),
	}
	if record.ImportedAt.IsZero() {
		record.ImportedAt = time.Now().UTC()
	}
	moduleDir := s.moduleDir(manifest.ModuleID)
	if err := os.RemoveAll(moduleDir); err != nil {
		return gameModuleRecord{}, err
	}
	if err := os.MkdirAll(filepath.Join(moduleDir, "cheats"), 0o755); err != nil {
		return gameModuleRecord{}, err
	}
	manifestYAML, err := yaml.Marshal(manifest)
	if err != nil {
		return gameModuleRecord{}, err
	}
	if err := writeFileAtomic(filepath.Join(moduleDir, "rsm-module.yaml"), manifestYAML, 0o644); err != nil {
		return gameModuleRecord{}, err
	}
	wasmTarget := filepath.Join(moduleDir, manifest.WASMFile)
	if err := os.MkdirAll(filepath.Dir(wasmTarget), 0o755); err != nil {
		return gameModuleRecord{}, err
	}
	if err := writeFileAtomic(wasmTarget, wasmData, 0o644); err != nil {
		return gameModuleRecord{}, err
	}
	for idx, pack := range packs {
		packID := canonicalCheatPackID(firstNonEmpty(pack.PackID, pack.GameID, pack.Title))
		pack.PackID = packID
		record.CheatPackIDs = append(record.CheatPackIDs, packID)
		packData, err := yaml.Marshal(pack)
		if err != nil {
			return gameModuleRecord{}, err
		}
		name := safeFilename(filepath.Base(packPaths[idx]))
		if strings.TrimSpace(name) == "" || name == "save.bin" {
			name = packID + ".yaml"
		}
		if err := writeFileAtomic(filepath.Join(moduleDir, "cheats", name), packData, 0o644); err != nil {
			return gameModuleRecord{}, err
		}
	}
	if err := s.writeModuleRecord(record); err != nil {
		return gameModuleRecord{}, err
	}
	return record, nil
}

func readGameModuleZip(zipData []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("read module zip: %w", err)
	}
	if len(reader.File) > maxGameModuleFiles {
		return nil, fmt.Errorf("module zip contains too many files")
	}
	files := map[string][]byte{}
	total := 0
	for _, entry := range reader.File {
		clean, ok := cleanGameModuleZipPath(entry.Name)
		if !ok {
			return nil, fmt.Errorf("unsafe module path %q", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.FileInfo().Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlinks are not allowed in module zip: %s", entry.Name)
		}
		if entry.UncompressedSize64 > maxGameModuleFileBytes {
			return nil, fmt.Errorf("module file %s exceeds %d bytes", clean, maxGameModuleFileBytes)
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(rc, maxGameModuleFileBytes+1))
		closeErr := rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(data) > maxGameModuleFileBytes {
			return nil, fmt.Errorf("module file %s exceeds %d bytes", clean, maxGameModuleFileBytes)
		}
		total += len(data)
		if total > maxGameModuleZipBytes {
			return nil, fmt.Errorf("module zip expanded content exceeds %d bytes", maxGameModuleZipBytes)
		}
		files[clean] = data
	}
	return files, nil
}

func cleanGameModuleZipPath(raw string) (string, bool) {
	name := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	if name == "" || strings.HasPrefix(name, "/") {
		return "", false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == ".." {
			return "", false
		}
	}
	clean := path.Clean(name)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", false
	}
	return clean, true
}

func normalizeGameModuleManifest(manifest gameModuleManifest, files map[string][]byte) (gameModuleManifest, error) {
	manifest.ModuleID = canonicalGameModuleID(manifest.ModuleID)
	if manifest.ModuleID == "" {
		return manifest, errors.New("moduleId is required")
	}
	if manifest.SchemaVersion != gameModuleSchemaVersion {
		return manifest, fmt.Errorf("schemaVersion must be %d", gameModuleSchemaVersion)
	}
	manifest.SystemSlug = canonicalSegment(manifest.SystemSlug, "")
	if manifest.SystemSlug == "" || !isSupportedSystemSlug(manifest.SystemSlug) {
		return manifest, fmt.Errorf("supported systemSlug is required")
	}
	manifest.GameID = strings.TrimSpace(manifest.GameID)
	manifest.Title = strings.TrimSpace(manifest.Title)
	if manifest.GameID == "" || manifest.Title == "" {
		return manifest, errors.New("gameId and title are required")
	}
	manifest.ParserID = strings.TrimSpace(manifest.ParserID)
	if manifest.ParserID == "" {
		return manifest, errors.New("parserId is required")
	}
	manifest.ABIVersion = strings.TrimSpace(manifest.ABIVersion)
	if manifest.ABIVersion != gameModuleABIVersion {
		return manifest, fmt.Errorf("abiVersion must be %q", gameModuleABIVersion)
	}
	wasmPath, ok := cleanGameModuleZipPath(firstNonEmpty(manifest.WASMFile, "parser.wasm"))
	if !ok {
		return manifest, errors.New("wasmFile path is unsafe")
	}
	manifest.WASMFile = wasmPath
	if len(manifest.Payload.ExactSizes) == 0 {
		return manifest, errors.New("payload.exactSizes is required")
	}
	manifest.Payload.ExactSizes = normalizeGameModuleSizes(manifest.Payload.ExactSizes)
	if len(manifest.Payload.ExactSizes) == 0 {
		return manifest, errors.New("payload.exactSizes must contain positive sizes")
	}
	if len(manifest.Payload.Formats) == 0 {
		return manifest, errors.New("payload.formats is required")
	}
	manifest.Payload.Formats = normalizeStringList(manifest.Payload.Formats)
	manifest.TitleAliases = normalizeStringList(append([]string{manifest.Title}, manifest.TitleAliases...))
	manifest.ROMHashes = normalizeStringList(manifest.ROMHashes)
	if len(manifest.CheatPacks) == 0 {
		paths := make([]string, 0, 4)
		for name := range files {
			if strings.HasPrefix(name, "cheats/") {
				ext := strings.ToLower(filepath.Ext(name))
				if ext == ".yaml" || ext == ".yml" {
					paths = append(paths, name)
				}
			}
		}
		sort.Strings(paths)
		for _, name := range paths {
			manifest.CheatPacks = append(manifest.CheatPacks, gameModuleCheatPack{Path: name})
		}
	}
	for idx := range manifest.CheatPacks {
		clean, ok := cleanGameModuleZipPath(manifest.CheatPacks[idx].Path)
		if !ok || !strings.HasPrefix(clean, "cheats/") {
			return manifest, fmt.Errorf("cheatPacks[%d].path must be under cheats/", idx)
		}
		manifest.CheatPacks[idx].Path = clean
	}
	return manifest, nil
}

func normalizeGameModuleSizes(values []int) []int {
	out := make([]int, 0, len(values))
	seen := map[int]struct{}{}
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func loadGameModuleCheatPacks(manifest gameModuleManifest, files map[string][]byte) ([]cheatPack, []string, error) {
	packs := make([]cheatPack, 0, len(manifest.CheatPacks))
	paths := make([]string, 0, len(manifest.CheatPacks))
	for _, ref := range manifest.CheatPacks {
		data := files[ref.Path]
		if len(data) == 0 {
			return nil, nil, fmt.Errorf("cheat pack %q not found", ref.Path)
		}
		pack, err := decodeCheatPackData(data, true)
		if err != nil {
			return nil, nil, fmt.Errorf("decode %s: %w", ref.Path, err)
		}
		pack.PackID = canonicalCheatPackID(firstNonEmpty(pack.PackID, pack.GameID, pack.Title))
		pack.SchemaVersion = maxInt(pack.SchemaVersion, 1)
		pack.SystemSlug = canonicalSegment(firstNonEmpty(pack.SystemSlug, manifest.SystemSlug), manifest.SystemSlug)
		pack.GameID = firstNonEmpty(pack.GameID, manifest.GameID)
		pack.Title = firstNonEmpty(pack.Title, manifest.Title)
		pack.EditorID = firstNonEmpty(pack.EditorID, manifest.ParserID)
		pack.AdapterID = firstNonEmpty(pack.AdapterID, manifest.ParserID)
		if len(pack.Match.TitleAliases) == 0 {
			pack.Match.TitleAliases = append([]string(nil), manifest.TitleAliases...)
		}
		if len(pack.Payload.ExactSizes) == 0 {
			pack.Payload.ExactSizes = append([]int(nil), manifest.Payload.ExactSizes...)
		}
		if len(pack.Payload.Formats) == 0 {
			pack.Payload.Formats = append([]string(nil), manifest.Payload.Formats...)
		}
		if len(pack.Sections) == 0 {
			return nil, nil, fmt.Errorf("%s must contain at least one section", ref.Path)
		}
		packs = append(packs, pack)
		paths = append(paths, ref.Path)
	}
	return packs, paths, nil
}

func validateGameModuleWASM(ctx context.Context, moduleID string, wasm []byte) error {
	if len(wasm) < 8 || !bytes.Equal(wasm[:4], []byte{0x00, 0x61, 0x73, 0x6d}) || !bytes.Equal(wasm[4:8], []byte{0x01, 0x00, 0x00, 0x00}) {
		return errors.New("parser.wasm is not a WebAssembly v1 module")
	}
	if len(wasm) > maxGameModuleWASMBytes {
		return fmt.Errorf("parser.wasm exceeds %d bytes", maxGameModuleWASMBytes)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(16).WithCloseOnContextDone(true))
	defer runtime.Close(ctx)
	compiled, err := runtime.CompileModule(ctx, wasm)
	if err != nil {
		return fmt.Errorf("compile parser.wasm: %w", err)
	}
	defer compiled.Close(ctx)
	capabilities, err := callGameModuleWASM(ctx, wasm, moduleID, "capabilities", []byte(`{"abiVersion":"rsm-wasm-json-v1"}`))
	if err != nil {
		return fmt.Errorf("call capabilities: %w", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(capabilities, &decoded); err != nil {
		return fmt.Errorf("decode capabilities response: %w", err)
	}
	return nil
}

func (s *gameModuleService) moduleWASMPath(record gameModuleRecord) string {
	return filepath.Join(s.moduleDir(record.Manifest.ModuleID), record.Manifest.WASMFile)
}

func (s *gameModuleService) moduleCheatPacks(record gameModuleRecord) ([]cheatPack, error) {
	moduleDir := s.moduleDir(record.Manifest.ModuleID)
	packs := make([]cheatPack, 0, len(record.CheatPackIDs))
	err := filepath.WalkDir(filepath.Join(moduleDir, "cheats"), func(p string, d os.DirEntry, err error) error {
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
		pack, err := loadCheatPackFile(p)
		if err != nil {
			return err
		}
		pack.AdapterID = firstNonEmpty(pack.AdapterID, record.Manifest.ParserID)
		pack.EditorID = firstNonEmpty(pack.EditorID, record.Manifest.ParserID)
		packs = append(packs, pack)
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.SliceStable(packs, func(i, j int) bool { return packs[i].PackID < packs[j].PackID })
	return packs, nil
}

func (s *gameModuleService) managedCheatPacks() ([]cheatManagedPack, error) {
	modules, err := s.listModules()
	if err != nil {
		return nil, err
	}
	items := make([]cheatManagedPack, 0, len(modules))
	for _, record := range modules {
		packs, err := s.moduleCheatPacks(record)
		if err != nil {
			return nil, err
		}
		for _, pack := range packs {
			packID := canonicalCheatPackID(firstNonEmpty(pack.PackID, pack.GameID, pack.Title))
			items = append(items, cheatManagedPack{
				Pack: pack,
				Manifest: cheatPackManifest{
					PackID:         packID,
					AdapterID:      firstNonEmpty(pack.AdapterID, record.Manifest.ParserID),
					Source:         cheatPackSourceModule,
					Status:         record.Status,
					PublishedBy:    "Game support module",
					SourcePath:     record.SourcePath,
					SourceRevision: record.SourceRevision,
					SourceSHA256:   record.SourceSHA256,
					LastSyncedAt:   record.LastSyncedAt,
				},
				Builtin:        false,
				SupportsSaveUI: record.Status == gameModuleStatusActive,
			})
		}
	}
	return items, nil
}

func (s *gameModuleService) moduleAdapters() []cheatAdapter {
	modules, err := s.listModules()
	if err != nil {
		return nil
	}
	adapters := make([]cheatAdapter, 0, len(modules))
	for _, record := range modules {
		if record.Status != gameModuleStatusActive {
			continue
		}
		adapters = append(adapters, gameModuleCheatAdapter{service: s, record: record})
	}
	return adapters
}

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

func (s *gameModuleService) callWASM(record gameModuleRecord, command string, request any, response any) error {
	wasm, err := os.ReadFile(s.moduleWASMPath(record))
	if err != nil {
		return err
	}
	input, err := json.Marshal(request)
	if err != nil {
		return err
	}
	output, err := callGameModuleWASM(context.Background(), wasm, record.Manifest.ModuleID, command, input)
	if err != nil {
		return err
	}
	if len(output) > maxGameModuleOutputBytes {
		return fmt.Errorf("module output exceeds %d bytes", maxGameModuleOutputBytes)
	}
	if err := json.Unmarshal(output, response); err != nil {
		return fmt.Errorf("decode module %s response: %w", command, err)
	}
	return nil
}

func callGameModuleWASM(ctx context.Context, wasm []byte, moduleID, command string, input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(16).WithCloseOnContextDone(true))
	defer runtime.Close(ctx)
	_, _ = wasi_snapshot_preview1.Instantiate(ctx, runtime)
	compiled, err := runtime.CompileModule(ctx, wasm)
	if err != nil {
		return nil, err
	}
	defer compiled.Close(ctx)
	mod, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("").WithStartFunctions())
	if err != nil {
		return nil, err
	}
	defer mod.Close(ctx)
	alloc := mod.ExportedFunction("rsm_alloc")
	call := mod.ExportedFunction("rsm_call")
	memory := mod.Memory()
	if alloc == nil || call == nil || memory == nil {
		return nil, fmt.Errorf("module %s must export memory, rsm_alloc, and rsm_call", moduleID)
	}
	cmdBytes := []byte(command)
	cmdPtr, err := wasmAlloc(ctx, alloc, memory, cmdBytes)
	if err != nil {
		return nil, err
	}
	inputPtr, err := wasmAlloc(ctx, alloc, memory, input)
	if err != nil {
		return nil, err
	}
	result, err := call.Call(ctx, uint64(cmdPtr), uint64(len(cmdBytes)), uint64(inputPtr), uint64(len(input)))
	if err != nil {
		return nil, err
	}
	if len(result) != 1 {
		return nil, errors.New("rsm_call must return one i64 pointer/length value")
	}
	ptr := uint32(result[0] >> 32)
	length := uint32(result[0] & 0xffffffff)
	if length > maxGameModuleOutputBytes {
		return nil, fmt.Errorf("module response exceeds %d bytes", maxGameModuleOutputBytes)
	}
	data, ok := memory.Read(ptr, length)
	if !ok {
		return nil, errors.New("module response points outside memory")
	}
	return append([]byte(nil), data...), nil
}

func wasmAlloc(ctx context.Context, alloc wazeroFunction, memory wazeroMemory, data []byte) (uint32, error) {
	result, err := alloc.Call(ctx, uint64(len(data)))
	if err != nil {
		return 0, err
	}
	if len(result) != 1 {
		return 0, errors.New("rsm_alloc must return one pointer")
	}
	ptr := uint32(result[0])
	if !memory.Write(ptr, data) {
		return 0, errors.New("rsm_alloc returned out-of-range pointer")
	}
	return ptr, nil
}

type wazeroFunction interface {
	Call(context.Context, ...uint64) ([]uint64, error)
}

type wazeroMemory interface {
	Write(uint32, []byte) bool
	Read(uint32, uint32) ([]byte, bool)
}

func metadataMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func canonicalGameModuleID(raw string) string {
	return canonicalSegment(raw, "")
}

func (s *gameModuleService) setStatus(moduleID, status string) (gameModuleRecord, error) {
	record, err := s.readModule(moduleID)
	if err != nil {
		return gameModuleRecord{}, err
	}
	record.Status = status
	if err := s.writeModuleRecord(record); err != nil {
		return gameModuleRecord{}, err
	}
	return record, nil
}

func (s *gameModuleService) setStatusByPackID(packID, status string) (gameModuleRecord, bool, error) {
	modules, err := s.listModules()
	if err != nil {
		return gameModuleRecord{}, false, err
	}
	target := canonicalCheatPackID(packID)
	for _, record := range modules {
		for _, id := range record.CheatPackIDs {
			if canonicalCheatPackID(id) == target {
				updated, err := s.setStatus(record.Manifest.ModuleID, status)
				return updated, true, err
			}
		}
	}
	return gameModuleRecord{}, false, nil
}

func (s *gameModuleService) libraryStatus() (gameModuleLibraryStatus, error) {
	return s.readLibraryStatus(gameModuleLibraryConfigFromEnv())
}

func (s *gameModuleService) readLibraryStatus(config gameModuleLibraryConfig) (gameModuleLibraryStatus, error) {
	status := gameModuleLibraryStatus{Config: config, Imported: []gameModuleSyncImported{}, Errors: []gameModuleSyncError{}}
	if s == nil {
		return status, nil
	}
	data, err := os.ReadFile(s.status)
	if os.IsNotExist(err) {
		return status, nil
	}
	if err != nil {
		return status, err
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return gameModuleLibraryStatus{}, err
	}
	status.Config = config
	status.ImportedCount = len(status.Imported)
	status.ErrorCount = len(status.Errors)
	if status.Imported == nil {
		status.Imported = []gameModuleSyncImported{}
	}
	if status.Errors == nil {
		status.Errors = []gameModuleSyncError{}
	}
	return status, nil
}

func (s *gameModuleService) writeLibraryStatus(status gameModuleLibraryStatus) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.status, data, 0o644)
}

func (s *gameModuleService) syncLibrary(ctx context.Context) (gameModuleLibraryStatus, error) {
	config := gameModuleLibraryConfigFromEnv()
	now := time.Now().UTC()
	status := gameModuleLibraryStatus{Config: config, LastSyncedAt: &now, Imported: []gameModuleSyncImported{}, Errors: []gameModuleSyncError{}}
	files, err := fetchGameModuleLibraryTree(ctx, config)
	if err != nil {
		status.Errors = append(status.Errors, gameModuleSyncError{Path: config.Path, Message: err.Error()})
		status.ErrorCount = len(status.Errors)
		_ = s.writeLibraryStatus(status)
		return status, nil
	}
	for _, file := range files {
		sourcePath := file.Path
		data, err := fetchGameModuleLibraryRaw(ctx, config, sourcePath, file.SHA)
		if err != nil {
			status.Errors = append(status.Errors, gameModuleSyncError{Path: sourcePath, Message: err.Error()})
			continue
		}
		sourceHash := sha256Hex(data)
		record, err := s.importZip(ctx, data, gameModuleSourceInfo{Source: gameModuleSourceGithub, SourcePath: sourcePath, SourceRevision: config.Ref, SourceSHA256: sourceHash, LastSyncedAt: &now})
		if err != nil {
			status.Errors = append(status.Errors, gameModuleSyncError{Path: sourcePath, Message: err.Error()})
			continue
		}
		status.Imported = append(status.Imported, gameModuleSyncImported{Path: sourcePath, ModuleID: record.Manifest.ModuleID, Title: record.Manifest.Title, SystemSlug: record.Manifest.SystemSlug, Status: record.Status, SHA256: sourceHash})
	}
	status.ImportedCount = len(status.Imported)
	status.ErrorCount = len(status.Errors)
	sort.SliceStable(status.Imported, func(i, j int) bool { return status.Imported[i].Path < status.Imported[j].Path })
	sort.SliceStable(status.Errors, func(i, j int) bool { return status.Errors[i].Path < status.Errors[j].Path })
	if err := s.writeLibraryStatus(status); err != nil {
		return gameModuleLibraryStatus{}, err
	}
	return status, nil
}

func gameModuleLibraryConfigFromEnv() gameModuleLibraryConfig {
	return gameModuleLibraryConfig{
		Repo: firstNonEmpty(strings.TrimSpace(os.Getenv("MODULE_LIBRARY_REPO")), defaultModuleLibraryRepo),
		Ref:  firstNonEmpty(strings.TrimSpace(os.Getenv("MODULE_LIBRARY_REF")), defaultModuleLibraryRef),
		Path: cleanCheatLibraryPath(firstNonEmpty(strings.TrimSpace(os.Getenv("MODULE_LIBRARY_PATH")), defaultModuleLibraryPath)),
	}
}

func fetchGameModuleLibraryTree(ctx context.Context, config gameModuleLibraryConfig) ([]githubLibraryFile, error) {
	var tree githubTreeResponse
	if err := fetchCheatLibraryJSON(ctx, gameModuleTreeURL(config), &tree); err != nil {
		return nil, err
	}
	prefix := strings.Trim(config.Path, "/")
	if prefix != "" {
		prefix += "/"
	}
	files := make([]githubLibraryFile, 0, len(tree.Tree))
	for _, item := range tree.Tree {
		if strings.TrimSpace(item.Type) != "blob" {
			continue
		}
		itemPath := strings.Trim(item.Path, "/")
		if prefix != "" && !strings.HasPrefix(itemPath, prefix) {
			continue
		}
		if strings.EqualFold(filepath.Ext(itemPath), ".zip") && strings.HasSuffix(strings.ToLower(itemPath), ".rsmodule.zip") {
			files = append(files, githubLibraryFile{Path: itemPath, SHA: strings.TrimSpace(item.SHA)})
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func fetchGameModuleLibraryRaw(ctx context.Context, config gameModuleLibraryConfig, sourcePath, cacheKey string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gameModuleRawURL(config, sourcePath, cacheKey), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "RetroSaveManager")
	resp, err := cheatLibraryHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub raw returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGameModuleZipBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("empty module zip")
	}
	if len(data) > maxGameModuleZipBytes {
		return nil, fmt.Errorf("module zip exceeds %d bytes", maxGameModuleZipBytes)
	}
	return data, nil
}

func gameModuleTreeURL(config gameModuleLibraryConfig) string {
	apiBase := strings.TrimRight(firstNonEmpty(strings.TrimSpace(os.Getenv("MODULE_LIBRARY_API_BASE")), "https://api.github.com/repos"), "/")
	return apiBase + "/" + urlPathEscapeSegments(config.Repo) + "/git/trees/" + url.PathEscape(config.Ref) + "?recursive=1"
}

func gameModuleRawURL(config gameModuleLibraryConfig, sourcePath, cacheKey string) string {
	rawBase := strings.TrimRight(firstNonEmpty(strings.TrimSpace(os.Getenv("MODULE_LIBRARY_RAW_BASE")), "https://raw.githubusercontent.com"), "/")
	return withRawCacheBuster(rawBase+"/"+urlPathEscapeSegments(config.Repo)+"/"+url.PathEscape(config.Ref)+"/"+urlPathEscapeSegments(sourcePath), cacheKey)
}
