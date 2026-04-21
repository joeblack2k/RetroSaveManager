package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type saveLayoutMove struct {
	SaveID string `json:"saveId"`
	From   string `json:"from"`
	To     string `json:"to"`
}

type saveLayoutManifest struct {
	GeneratedAt time.Time        `json:"generatedAt"`
	SaveRoot    string           `json:"saveRoot"`
	Moves       []saveLayoutMove `json:"moves"`
}

func runSaveLayoutMigration(args []string) error {
	fs := flag.NewFlagSet("migrate-save-layout", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	saveRoot := fs.String("save-root", "", "absolute or relative save root; defaults to SAVE_ROOT env")
	manifestPath := fs.String("manifest", "", "manifest path for migrate/rollback")
	dryRun := fs.Bool("dry-run", false, "plan only; do not move directories")
	rollback := fs.Bool("rollback", false, "rollback a previously generated manifest")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root := strings.TrimSpace(*saveRoot)
	if root == "" {
		root = strings.TrimSpace(os.Getenv("SAVE_ROOT"))
	}
	if root == "" {
		root = defaultSaveRoot
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve save root: %w", err)
	}

	manifest := strings.TrimSpace(*manifestPath)
	if manifest == "" {
		manifest = filepath.Join(absRoot, "save-layout-manifest.json")
	}
	manifest, err = filepath.Abs(manifest)
	if err != nil {
		return fmt.Errorf("resolve manifest path: %w", err)
	}

	if *rollback {
		return rollbackSaveLayout(absRoot, manifest, *dryRun)
	}
	return migrateSaveLayout(absRoot, manifest, *dryRun)
}

func migrateSaveLayout(saveRoot, manifestPath string, dryRun bool) error {
	store := &saveStore{root: saveRoot}
	records, err := store.load()
	if err != nil {
		return fmt.Errorf("load save records: %w", err)
	}

	moves := make([]saveLayoutMove, 0, len(records))
	for _, record := range records {
		systemPath := record.SystemPath
		if strings.TrimSpace(systemPath) == "" {
			systemName := "Unknown System"
			if record.Summary.Game.System != nil {
				systemName = record.Summary.Game.System.Name
			}
			systemPath = sanitizeDisplayPathSegment(systemName, "Unknown System")
		}

		gamePath := record.GamePath
		if strings.TrimSpace(gamePath) == "" {
			if record.Summary.MemoryCard != nil && strings.TrimSpace(record.Summary.MemoryCard.Name) != "" {
				gamePath = sanitizeDisplayPathSegment(record.Summary.MemoryCard.Name, "Memory Card 1")
			} else {
				gamePath = sanitizeDisplayPathSegment(record.Summary.DisplayTitle, "Unknown Game")
			}
		}

		targetDir, err := safeJoinUnderRoot(saveRoot, systemPath, gamePath, record.Summary.ID)
		if err != nil {
			return fmt.Errorf("build target dir for %s: %w", record.Summary.ID, err)
		}
		if filepath.Clean(targetDir) == filepath.Clean(record.dirPath) {
			continue
		}

		moves = append(moves, saveLayoutMove{
			SaveID: record.Summary.ID,
			From:   record.dirPath,
			To:     targetDir,
		})
	}

	sort.Slice(moves, func(i, j int) bool { return moves[i].From < moves[j].From })
	manifest := saveLayoutManifest{
		GeneratedAt: time.Now().UTC(),
		SaveRoot:    saveRoot,
		Moves:       moves,
	}
	if err := writeSaveLayoutManifest(manifestPath, manifest); err != nil {
		return err
	}

	fmt.Printf("manifest: %s\n", manifestPath)
	fmt.Printf("planned moves: %d\n", len(moves))
	if dryRun {
		return nil
	}

	for _, move := range moves {
		if err := applySaveLayoutMove(saveRoot, move); err != nil {
			return err
		}
	}
	return nil
}

func rollbackSaveLayout(saveRoot, manifestPath string, dryRun bool) error {
	manifest, err := readSaveLayoutManifest(manifestPath)
	if err != nil {
		return err
	}
	if manifest.SaveRoot != "" && filepath.Clean(manifest.SaveRoot) != filepath.Clean(saveRoot) {
		return fmt.Errorf("manifest saveRoot mismatch: manifest=%s requested=%s", manifest.SaveRoot, saveRoot)
	}

	fmt.Printf("rollback manifest: %s\n", manifestPath)
	fmt.Printf("rollback moves: %d\n", len(manifest.Moves))
	if dryRun {
		return nil
	}

	for i := len(manifest.Moves) - 1; i >= 0; i-- {
		move := manifest.Moves[i]
		reverse := saveLayoutMove{
			SaveID: move.SaveID,
			From:   move.To,
			To:     move.From,
		}
		if err := applySaveLayoutMove(saveRoot, reverse); err != nil {
			return err
		}
	}
	return nil
}

func applySaveLayoutMove(saveRoot string, move saveLayoutMove) error {
	from := filepath.Clean(move.From)
	to := filepath.Clean(move.To)

	if !isUnderRoot(saveRoot, from) || !isUnderRoot(saveRoot, to) {
		return fmt.Errorf("move escapes save root for %s", move.SaveID)
	}
	if from == to {
		return nil
	}

	fromExists := pathExists(from)
	toExists := pathExists(to)
	if !fromExists && toExists {
		return nil
	}
	if !fromExists && !toExists {
		return nil
	}
	if fromExists && toExists {
		return fmt.Errorf("both source and target exist for %s", move.SaveID)
	}

	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return fmt.Errorf("create target parent for %s: %w", move.SaveID, err)
	}
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("move %s: %w", move.SaveID, err)
	}
	cleanupEmptyParents(filepath.Dir(from), saveRoot)
	return nil
}

func writeSaveLayoutManifest(path string, manifest saveLayoutManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest parent: %w", err)
	}
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func readSaveLayoutManifest(path string) (saveLayoutManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return saveLayoutManifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest saveLayoutManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return saveLayoutManifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if len(manifest.Moves) == 0 {
		return saveLayoutManifest{}, errors.New("manifest has no moves")
	}
	return manifest, nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isUnderRoot(root, candidate string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

func cleanupEmptyParents(startDir, stopRoot string) {
	current := filepath.Clean(startDir)
	root := filepath.Clean(stopRoot)
	for current != root && current != "." && current != string(os.PathSeparator) {
		entries, err := os.ReadDir(current)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(current); err != nil {
			return
		}
		current = filepath.Dir(current)
	}
}
