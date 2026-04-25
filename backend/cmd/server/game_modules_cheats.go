package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// moduleCheatPacks loads the installed YAML packs for one module.
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

// managedCheatPacks exposes module-backed packs beside bundled and GitHub packs.
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

// moduleAdapters registers active WASM modules as cheat adapters at runtime.
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
