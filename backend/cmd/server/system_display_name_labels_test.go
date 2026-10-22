package main

import (
	"strings"
	"testing"
)

// Labels are derived from the system table, so a row added there is a label
// here without a second list to keep in step.
func TestSystemDisplayNamesResolveToTheirSlug(t *testing.T) {
	// A client may declare a system by the name the store itself uses for it.
	// When that resolves to nothing the declaration is silently discarded and a
	// lower-confidence rule decides instead, which is how a Genesis save was
	// stored as game-gear.
	for slug, sys := range supportedSystemsBySlug {
		for _, label := range []string{sys.Name, strings.ReplaceAll(sys.Name, "/", " ")} {
			if got := supportedSystemSlugFromLabel(label); got != slug {
				t.Errorf("display name %q resolved to %q, want %q", label, got, slug)
			}
		}
	}
}

func TestTrustedHelperDisplayNameBeatsTitleOverride(t *testing.T) {
	payload := make([]byte, 65536)
	for index := range payload {
		payload[index] = byte(index * 7)
	}

	// "defenders of oasis" is in knownTitleSystemOverrides as game-gear. A
	// trusted helper declaring Genesis outranks it at 96 against 93 -- but only
	// once the declared label can be read.
	for _, declared := range []string{"genesis", "Sega Genesis", "Sega Genesis/Mega Drive"} {
		got := detectSaveSystem(saveSystemDetectionInput{
			Filename:            "Defenders of Oasis (USA, Europe).srm",
			DisplayTitle:        "Defenders of Oasis",
			Payload:             payload,
			DeclaredSystemSlug:  declared,
			TrustedHelperSystem: true,
		})
		if got.Slug != "genesis" {
			t.Errorf("declared %q: got %q (%s), want genesis", declared, got.Slug, got.Reason)
		}
	}
}
