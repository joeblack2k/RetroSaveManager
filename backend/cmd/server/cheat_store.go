package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	cheatPackSourceBuiltin  = "builtin"
	cheatPackSourceGithub   = "github"
	cheatPackSourceModule   = "module"
	cheatPackSourceUploaded = "uploaded"
	cheatPackSourceWorker   = "worker"

	cheatPackStatusActive   = "active"
	cheatPackStatusDisabled = "disabled"
	cheatPackStatusDeleted  = "deleted"
)

type cheatRuntimeStore struct {
	root          string
	packsRoot     string
	tombstoneRoot string
	libraryStatus string
}

func newCheatRuntimeStore(saveRoot string) (*cheatRuntimeStore, error) {
	root, err := safeJoinUnderRoot(saveRoot, "_rsm", "cheats")
	if err != nil {
		return nil, err
	}
	packsRoot := filepath.Join(root, "packs")
	tombstoneRoot := filepath.Join(root, "tombstones")
	for _, dir := range []string{root, packsRoot, tombstoneRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return &cheatRuntimeStore{
		root:          root,
		packsRoot:     packsRoot,
		tombstoneRoot: tombstoneRoot,
		libraryStatus: filepath.Join(root, "library-status.json"),
	}, nil
}

func (s *cheatRuntimeStore) listRuntimePacks() ([]cheatManagedPack, error) {
	if s == nil {
		return nil, nil
	}
	packs := make([]cheatManagedPack, 0, 8)
	err := filepath.WalkDir(s.packsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "manifest.json" {
			return nil
		}
		manifestData, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var manifest cheatPackManifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			return err
		}
		packPath := filepath.Join(filepath.Dir(path), "pack.yaml")
		pack, err := decodeCheatPackFile(packPath, false)
		if err != nil {
			return err
		}
		pack.PackID = firstNonEmpty(pack.PackID, manifest.PackID)
		pack.AdapterID = firstNonEmpty(pack.AdapterID, manifest.AdapterID, pack.EditorID)
		manifest.PackID = firstNonEmpty(manifest.PackID, pack.PackID)
		manifest.AdapterID = firstNonEmpty(manifest.AdapterID, pack.AdapterID, pack.EditorID)
		packs = append(packs, cheatManagedPack{
			Pack:           pack,
			Manifest:       manifest,
			Builtin:        false,
			SupportsSaveUI: manifest.Status == cheatPackStatusActive,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(packs, func(i, j int) bool {
		if packs[i].Manifest.UpdatedAt.Equal(packs[j].Manifest.UpdatedAt) {
			return packs[i].Manifest.PackID < packs[j].Manifest.PackID
		}
		return packs[i].Manifest.UpdatedAt.After(packs[j].Manifest.UpdatedAt)
	})
	return packs, nil
}

func (s *cheatRuntimeStore) loadTombstones() (map[string]cheatPackTombstone, error) {
	if s == nil {
		return map[string]cheatPackTombstone{}, nil
	}
	out := map[string]cheatPackTombstone{}
	err := filepath.WalkDir(s.tombstoneRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var tombstone cheatPackTombstone
		if err := json.Unmarshal(data, &tombstone); err != nil {
			return err
		}
		tombstone.PackID = strings.TrimSpace(tombstone.PackID)
		if tombstone.PackID != "" {
			out[tombstone.PackID] = tombstone
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *cheatRuntimeStore) writePack(pack cheatPack, manifest cheatPackManifest) (cheatManagedPack, error) {
	packID := canonicalCheatPackID(firstNonEmpty(pack.PackID, manifest.PackID, pack.GameID, pack.Title))
	pack.PackID = packID
	manifest.PackID = packID
	manifest.AdapterID = firstNonEmpty(manifest.AdapterID, pack.AdapterID, pack.EditorID)
	pack.AdapterID = firstNonEmpty(pack.AdapterID, manifest.AdapterID, pack.EditorID)
	now := time.Now().UTC()
	if manifest.CreatedAt.IsZero() {
		manifest.CreatedAt = now
	}
	manifest.UpdatedAt = now
	if manifest.Source == "" {
		manifest.Source = cheatPackSourceUploaded
	}
	if manifest.Status == "" {
		manifest.Status = cheatPackStatusActive
	}
	packDir := filepath.Join(s.packsRoot, packID)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return cheatManagedPack{}, err
	}
	packData, err := yaml.Marshal(pack)
	if err != nil {
		return cheatManagedPack{}, err
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return cheatManagedPack{}, err
	}
	if err := writeFileAtomic(filepath.Join(packDir, "pack.yaml"), packData, 0o644); err != nil {
		return cheatManagedPack{}, err
	}
	if err := writeFileAtomic(filepath.Join(packDir, "manifest.json"), manifestData, 0o644); err != nil {
		return cheatManagedPack{}, err
	}
	return cheatManagedPack{
		Pack:           pack,
		Manifest:       manifest,
		Builtin:        false,
		SupportsSaveUI: manifest.Status == cheatPackStatusActive,
	}, nil
}

func (s *cheatRuntimeStore) writeLibraryStatus(status cheatLibraryStatus) error {
	if s == nil {
		return nil
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.libraryStatus, data, 0o644)
}

func (s *cheatRuntimeStore) readLibraryStatus(config cheatLibraryConfig) (cheatLibraryStatus, error) {
	status := cheatLibraryStatus{
		Config:   config,
		Imported: []cheatLibraryImportedPack{},
		Errors:   []cheatLibrarySyncError{},
	}
	if s == nil {
		return status, nil
	}
	data, err := os.ReadFile(s.libraryStatus)
	if os.IsNotExist(err) {
		return status, nil
	}
	if err != nil {
		return status, err
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return cheatLibraryStatus{}, err
	}
	status.Config = config
	if status.Imported == nil {
		status.Imported = []cheatLibraryImportedPack{}
	}
	if status.Errors == nil {
		status.Errors = []cheatLibrarySyncError{}
	}
	status.ImportedCount = len(status.Imported)
	status.ErrorCount = len(status.Errors)
	return status, nil
}

func (s *cheatRuntimeStore) updateRuntimePackStatus(packID, status string) (cheatManagedPack, error) {
	packs, err := s.listRuntimePacks()
	if err != nil {
		return cheatManagedPack{}, err
	}
	for _, item := range packs {
		if item.Manifest.PackID != strings.TrimSpace(packID) {
			continue
		}
		item.Manifest.Status = status
		return s.writePack(item.Pack, item.Manifest)
	}
	return cheatManagedPack{}, fmt.Errorf("cheat pack %q not found", packID)
}

func (s *cheatRuntimeStore) writeTombstone(packID, status, deletedBy, source string) error {
	packID = canonicalCheatPackID(packID)
	if packID == "" {
		return fmt.Errorf("packId is required")
	}
	data, err := json.MarshalIndent(cheatPackTombstone{
		PackID:      packID,
		Status:      firstNonEmpty(status, cheatPackStatusDeleted),
		DeletedAt:   time.Now().UTC(),
		DeletedBy:   strings.TrimSpace(deletedBy),
		Source:      strings.TrimSpace(source),
		Description: "builtin pack suppressed by runtime management",
	}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(s.tombstoneRoot, packID+".json"), data, 0o644)
}

func (s *cheatRuntimeStore) clearTombstone(packID string) error {
	path := filepath.Join(s.tombstoneRoot, canonicalCheatPackID(packID)+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func canonicalCheatPackID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	replacer := strings.NewReplacer("/", "--", "\\", "--", ":", "-", ".", "-", "_", "-")
	out := strings.ToLower(replacer.Replace(raw))
	out = strings.Trim(out, "-")
	if out == "" {
		return "pack"
	}
	return out
}
