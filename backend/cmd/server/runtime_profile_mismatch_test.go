package main

import (
	"strings"
	"testing"
)

func TestRuntimeProfileMismatchNamesBothSystems(t *testing.T) {
	// The save is recorded as game-gear; the client asks with a genesis
	// profile. Reporting that as "runtimeProfile is required" sends the client
	// looking for a missing field when what it sent is present and wrong.
	message := strings.ToLower(runtimeProfileMismatch("game-gear", "genesis/retroarch-genesis-plus-gx", "genesis"))
	if message == "" {
		t.Fatalf("expected a mismatch message, got none")
	}
	for _, want := range []string{"genesis", "game-gear", "genesis/retroarch-genesis-plus-gx"} {
		if !strings.Contains(message, want) {
			t.Errorf("message %q does not name %q", message, want)
		}
	}
}

func TestRuntimeProfileMismatchIsSilentWhenNothingWasSent(t *testing.T) {
	// Absent is absent: the callers keep their "is required" message for it.
	if message := runtimeProfileMismatch("game-gear", "  ", "genesis"); message != "" {
		t.Errorf("expected no message for an empty request, got %q", message)
	}
}

func TestRuntimeProfileMismatchNamesAnUnknownProfile(t *testing.T) {
	message := runtimeProfileMismatch("genesis", "not-a-profile", "genesis")
	if !strings.Contains(strings.ToLower(message), "not a known profile") {
		t.Errorf("expected an unknown-profile message, got %q", message)
	}
}

func TestRuntimeProfileMismatchNamesADeclarationThatDiffersFromTheProfile(t *testing.T) {
	// The request declares a third system, so neither side of the profile
	// sentence is the reader's. Say which one they sent.
	message := strings.ToLower(runtimeProfileMismatch("game-gear", "genesis/retroarch-genesis-plus-gx", "snes"))
	if !strings.Contains(message, "the request declares snes") {
		t.Errorf("message %q does not name the declared system", message)
	}
}

func TestRuntimeProfileMismatchOmitsTheDeclarationWhenItMatchesTheProfile(t *testing.T) {
	// Declared genesis, profile genesis: the profile sentence already tells the
	// reader which side is theirs, and a second "declares genesis" is noise.
	message := runtimeProfileMismatch("game-gear", "genesis/retroarch-genesis-plus-gx", "genesis")
	if strings.Contains(strings.ToLower(message), "the request declares") {
		t.Errorf("message %q repeats the declaration", message)
	}
}
