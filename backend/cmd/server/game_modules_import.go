package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// importZip validates and installs a community module bundle.
// Zip paths are normalized, symlinks rejected, file sizes capped, and raw Go
// source in src/ is ignored by the production runtime.
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

// readGameModuleZip expands only safe, bounded files from an uploaded bundle.
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

// normalizeGameModuleManifest is the activation gate for module metadata.
// Unsupported systems, unsafe paths, missing payload policy, or wrong ABI fail here.
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

// loadGameModuleCheatPacks adapts module YAML to the normal cheat-pack pipeline.
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
