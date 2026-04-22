package main

import (
	"path/filepath"
	"testing"
)

func TestInitSaveStoreDoesNotSeedDemoSaveByDefault(t *testing.T) {
	t.Setenv("SAVE_ROOT", filepath.Join(t.TempDir(), "saves"))
	t.Setenv("STATE_ROOT", filepath.Join(t.TempDir(), "state"))
	t.Setenv("BOOTSTRAP_DEMO_DATA", "")

	app := newApp()
	if err := app.initSaveStore(); err != nil {
		t.Fatalf("init save store: %v", err)
	}

	if len(app.saves) != 0 {
		t.Fatalf("expected empty save store by default, got %d saves", len(app.saves))
	}
}

func TestInitSaveStoreCanSeedDemoSaveWhenExplicitlyEnabled(t *testing.T) {
	t.Setenv("SAVE_ROOT", filepath.Join(t.TempDir(), "saves"))
	t.Setenv("STATE_ROOT", filepath.Join(t.TempDir(), "state"))
	t.Setenv("BOOTSTRAP_DEMO_DATA", "true")

	app := newApp()
	if err := app.initSaveStore(); err != nil {
		t.Fatalf("init save store: %v", err)
	}

	if len(app.saves) != 1 {
		t.Fatalf("expected 1 seeded save when demo bootstrap enabled, got %d", len(app.saves))
	}
	if app.saves[0].Filename != "Wario Land II.srm" {
		t.Fatalf("expected seeded Wario Land II save, got %q", app.saves[0].Filename)
	}
}
