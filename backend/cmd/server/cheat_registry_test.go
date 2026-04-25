package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheatEditorRegistryContainsKnownEditors(t *testing.T) {
	required := []string{
		"alttp-sram",
		"dkc3-sram",
		"dkc-sram",
		"dkr-eeprom",
		"mk64-eeprom",
		"oot-sram",
		"sf64-eeprom",
		"sm64-eeprom",
	}
	registered := map[string]bool{}
	for _, id := range builtinCheatEditorIDs() {
		registered[id] = true
	}
	for _, id := range required {
		if !registered[id] {
			t.Fatalf("required cheat editor %q is not registered; registered=%v", id, builtinCheatEditorIDs())
		}
	}
}

func TestCheatLibraryPacksValidateAgainstRegisteredAdapters(t *testing.T) {
	service, err := newCheatService(t.TempDir())
	if err != nil {
		t.Fatalf("newCheatService: %v", err)
	}
	root := findCheatLibraryPackRootForTest(t)
	checked := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
			t.Errorf("load %s: %v", path, loadErr)
			return nil
		}
		if _, validateErr := service.validateLiveCheatPack(pack); validateErr != nil {
			t.Errorf("validate %s: %v", path, validateErr)
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatalf("walk cheat library: %v", err)
	}
	if checked == 0 {
		t.Fatalf("no cheat packs found under %s", root)
	}
}

func findCheatLibraryPackRootForTest(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	dir := cwd
	for depth := 0; depth < 8; depth++ {
		candidate := filepath.Join(dir, "cheats", "packs")
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("cheats/packs root not found from %s", cwd)
	return ""
}
