package main

import "testing"

func TestNumberedSaveSlotTitlePattern(t *testing.T) {
	// True positives — real numbered slot/mission artifacts that we DO want to drop.
	wantNoise := []string{
		"1 - Save",
		"01 - Save Slot",
		"02 - Backup",
		"3 - Mission",
		"4 - Auto",
		"05 - Autosave",
		"7 - Player Slot",
		"008 - File 8",
		"01 - State",
		"02 - Memory Card",
		"01 - SP0",
		"02 - SP12",
	}
	for _, title := range wantNoise {
		t.Run("noise/"+title, func(t *testing.T) {
			if !numberedSaveSlotTitlePattern.MatchString(title) {
				t.Errorf("expected %q to match numberedSaveSlotTitlePattern (real slot artifact)", title)
			}
		})
	}

	// False positives — real game titles that previously got rejected and shouldn't.
	// See: 2026-06-07 burn where Deck saves for 007 - Everything or Nothing (GBA)
	// were rejected with status=422 because the title prefix "007 - " matched the
	// over-broad original pattern `^\s*[0-9]{1,3}\s*-\s*[A-Za-z]`.
	wantGame := []string{
		"007 - Everything or Nothing",
		"007 - The World Is Not Enough",
		"007 - GoldenEye",
		"007 - Agent Under Fire",
		"3 - Count Bout",                      // real arcade game, no dash
		"19 - Centerfold",                     // real game
		"99 - The Last Homeland",              // hypothetical real game
		"4 - Wheel Thunder",                   // SNK Dreamcast game
		"007 - James Bond: Quantum of Solace", // real game
		"42 - The Answer",                     // hypothetical, real-game-looking
	}
	for _, title := range wantGame {
		t.Run("game/"+title, func(t *testing.T) {
			if numberedSaveSlotTitlePattern.MatchString(title) {
				t.Errorf("expected %q to NOT match numberedSaveSlotTitlePattern (real game title)", title)
			}
		})
	}
}

func TestDecorationDoesNotApplyATitleOverrideToADeclaredSystem(t *testing.T) {
	h := newContractHarness(t)
	key := createHelperAppPassword(t, h, "", "p-helper")
	uploadSave(t, h, "/saves", map[string]string{
		"app_password":   key,
		"rom_sha1":       "decoration-rom",
		"slotName":       "default",
		"system":         "genesis",
		"device_type":    "mister",
		"fingerprint":    "p-dev",
		"runtimeProfile": "genesis/retroarch-genesis-plus-gx",
	}, "Defenders of Oasis (USA, Europe).srm", buildNonBlankPayload(65536, 0x21))

	// The client's declaration here is deliberately the false one: Defenders of
	// Oasis shipped only on Game Gear, and a client that files it under
	// MegaDrive is the case being tested. The fixture must keep that wrong
	// declaration, because the title override only fires for this title and a
	// correct declaration would make the test pass on main too.
	//
	// What is under test: the upload stores the declared system and the disk
	// holds it; reading the record back enriches it and must not re-decide the
	// system, or the API answers with a system the disk does not have and a
	// client holding the disk's answer is told its profile is missing.
	record, found := h.app.latestReadableSaveRecord("decoration-rom", "default")
	if !found {
		t.Fatalf("no record after upload")
	}
	if got := saveRecordSystemSlug(record); got != "genesis" {
		t.Errorf("record read back as %q, want genesis", got)
	}

	stored, err := h.app.saveStore.load()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	for _, candidate := range stored {
		if candidate.ROMSHA1 == "decoration-rom" && saveRecordSystemSlug(candidate) != "genesis" {
			t.Errorf("record on disk reads as %q, want genesis", saveRecordSystemSlug(candidate))
		}
	}
}

func TestDecorationLeavesAnUnoverriddenRecordAlone(t *testing.T) {
	// Same read path, title with no entry in knownTitleSystemOverrides: the
	// stored system is preserved, which is what a decorated record did before
	// and still does. This is the case the new flag must not disturb.
	h := newContractHarness(t)
	key := createHelperAppPassword(t, h, "", "p-helper")
	uploadSave(t, h, "/saves", map[string]string{
		"app_password":   key,
		"rom_sha1":       "plain-rom",
		"slotName":       "default",
		"system":         "genesis",
		"device_type":    "mister",
		"fingerprint":    "p-dev",
		"runtimeProfile": "genesis/retroarch-genesis-plus-gx",
	}, "Shining Force (USA).srm", buildNonBlankPayload(65536, 0x21))

	record, found := h.app.latestReadableSaveRecord("plain-rom", "default")
	if !found {
		t.Fatalf("no record after upload")
	}
	if got := saveRecordSystemSlug(record); got != "genesis" {
		t.Errorf("record read back as %q, want genesis", got)
	}
}
