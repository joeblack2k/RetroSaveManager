package main

import (
	"path/filepath"
	"testing"
)

func TestTask4PathSafetyEvidence(t *testing.T) {
	root := t.TempDir()

	normalPath, err := safeJoinUnderRoot(root, "gameboy", "wario-land-ii", "save-1")
	if err != nil {
		t.Fatalf("normal safe join failed: %v", err)
	}
	expectedNormal := filepath.Join(root, "gameboy", "wario-land-ii", "save-1")
	if normalPath != expectedNormal {
		t.Fatalf("normal path mismatch: got %q want %q", normalPath, expectedNormal)
	}
	t.Logf("normal path=%s", normalPath)

	_, err = safeJoinUnderRoot(root, "..", "etc", "passwd")
	if err == nil {
		t.Fatal("expected parent traversal to fail")
	}
	t.Logf("escape-parent error=%v", err)

	_, err = safeJoinUnderRoot(root, "gameboy", "..", "..", "evil")
	if err == nil {
		t.Fatal("expected nested traversal to fail")
	}
	t.Logf("escape-nested error=%v", err)

	systemSlug := canonicalSegment("../../SNES Saves", "unknown-system")
	if systemSlug != "snes-saves" {
		t.Fatalf("unexpected system slug: %q", systemSlug)
	}
	t.Logf("canonical bad system=%s", systemSlug)

	gameSlug := canonicalSegment(`..\..\Chrono Trigger`, "unknown-game")
	if gameSlug != "chrono-trigger" {
		t.Fatalf("unexpected game slug: %q", gameSlug)
	}
	t.Logf("canonical bad game=%s", gameSlug)

	filename := safeFilename("../../secret/slot1.srm")
	if filename != "slot1.srm" {
		t.Fatalf("unexpected safe filename: %q", filename)
	}
	t.Logf("safe filename=%s", filename)
}
