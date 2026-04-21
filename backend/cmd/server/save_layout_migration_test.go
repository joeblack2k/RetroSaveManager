package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLayoutMigrationAndRollback(t *testing.T) {
	saveRoot := filepath.Join(t.TempDir(), "saves")
	oldDir := filepath.Join(saveRoot, "snes", "yoshi-story-usa", "save-1")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("mkdir old dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "payload.srm"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	record := saveRecord{
		Summary: saveSummary{
			ID:           "save-1",
			DisplayTitle: "Yoshi Story",
			Game: game{
				ID:           1001,
				Name:         "Yoshi Story",
				DisplayTitle: "Yoshi Story",
				System: &system{
					ID:   26,
					Name: "Super Nintendo",
					Slug: "snes",
				},
			},
			Filename:  "Yoshi Story (USA).srm",
			FileSize:  7,
			Format:    "sram",
			Version:   1,
			SHA256:    "abc",
			CreatedAt: time.Now().UTC(),
		},
		SystemSlug:  "snes",
		GameSlug:    "yoshi-story-usa",
		PayloadFile: "payload.srm",
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "metadata.json"), data, 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	manifest := filepath.Join(t.TempDir(), "save-layout-manifest.json")
	if err := migrateSaveLayout(saveRoot, manifest, false); err != nil {
		t.Fatalf("migrate save layout: %v", err)
	}

	newDir := filepath.Join(saveRoot, "Nintendo Super Nintendo Entertainment System", "Yoshi Story", "save-1")
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("expected new dir: %v", err)
	}

	if err := rollbackSaveLayout(saveRoot, manifest, false); err != nil {
		t.Fatalf("rollback save layout: %v", err)
	}
	if _, err := os.Stat(oldDir); err != nil {
		t.Fatalf("expected original dir after rollback: %v", err)
	}
}
