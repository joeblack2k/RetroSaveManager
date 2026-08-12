package main

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

type failingEntropyReader struct{}

func (failingEntropyReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy unavailable")
}

func TestRandomHexFromFailsClosed(t *testing.T) {
	value, err := randomHexFrom(failingEntropyReader{}, 32)
	if err == nil {
		t.Fatal("expected entropy failure")
	}
	if value != "" {
		t.Fatalf("expected no fallback token, got %q", value)
	}
}

func TestGenerateAppPasswordCompactFromFailsClosed(t *testing.T) {
	value, err := generateAppPasswordCompactFrom(failingEntropyReader{})
	if err == nil {
		t.Fatal("expected entropy failure")
	}
	if value != "" {
		t.Fatalf("expected no fallback app password, got %q", value)
	}
}

func TestCreateAppPasswordEntropyFailureDoesNotMutateState(t *testing.T) {
	a := newApp()
	a.applyBootstrapDemoPolicy()
	beforeNext := a.nextAppPasswordID
	beforeCount := len(a.appPasswords)

	record, plainText, err := a.createAppPasswordLockedWithReader("test", time.Now().UTC(), failingEntropyReader{})
	if err == nil {
		t.Fatal("expected credential generation failure")
	}
	if record.ID != "" || plainText != "" {
		t.Fatalf("failure must not return credential material: record=%+v plaintext=%q", record, plainText)
	}
	if a.nextAppPasswordID != beforeNext {
		t.Fatalf("failed generation consumed an app-password ID: got %d want %d", a.nextAppPasswordID, beforeNext)
	}
	if len(a.appPasswords) != beforeCount {
		t.Fatalf("failed generation mutated app-password state: got %d want %d", len(a.appPasswords), beforeCount)
	}
}

func TestCreateAppPasswordSaltFailureDoesNotMutateState(t *testing.T) {
	a := newApp()
	a.applyBootstrapDemoPolicy()
	beforeNext := a.nextAppPasswordID
	beforeCount := len(a.appPasswords)

	reader := io.MultiReader(bytes.NewReader(make([]byte, 6)), failingEntropyReader{})
	_, _, err := a.createAppPasswordLockedWithReader("test", time.Now().UTC(), reader)
	if err == nil {
		t.Fatal("expected salt generation failure")
	}
	if a.nextAppPasswordID != beforeNext {
		t.Fatalf("salt failure consumed an app-password ID: got %d want %d", a.nextAppPasswordID, beforeNext)
	}
	if len(a.appPasswords) != beforeCount {
		t.Fatalf("salt failure mutated app-password state: got %d want %d", len(a.appPasswords), beforeCount)
	}
}

func TestCreateAppPasswordCollisionExhaustionDoesNotMutateState(t *testing.T) {
	a := newApp()
	a.applyBootstrapDemoPolicy()
	salt := "existing-salt"
	a.appPasswords["existing"] = appPassword{
		ID:      "existing",
		Name:    "existing",
		KeySalt: salt,
		KeyHash: hashAppPasswordCompact(salt, "AAAAAA"),
	}
	beforeNext := a.nextAppPasswordID
	beforeCount := len(a.appPasswords)

	// Zero bytes map to 'A'. Supply exactly 64 identical candidates so every
	// attempt collides with the existing AAAAAA credential.
	reader := bytes.NewReader(make([]byte, 6*64))
	_, _, err := a.createAppPasswordLockedWithReader("test", time.Now().UTC(), reader)
	if !errors.Is(err, errAppPasswordCollisionExhausted) {
		t.Fatalf("expected collision exhaustion, got %v", err)
	}
	if a.nextAppPasswordID != beforeNext {
		t.Fatalf("collision exhaustion consumed an app-password ID: got %d want %d", a.nextAppPasswordID, beforeNext)
	}
	if len(a.appPasswords) != beforeCount {
		t.Fatalf("collision exhaustion mutated app-password state: got %d want %d", len(a.appPasswords), beforeCount)
	}
}
