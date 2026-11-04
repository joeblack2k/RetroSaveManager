package main

import (
	"net/http"
	"strings"
	"testing"
)

// RetroArch's Beetle PSX and the MiSTer keep one memory card per game, so one
// device uploads several cards to the same Memory Card 1 line, each holding
// only its own game's save. A card that lacks game A's save is game B's card,
// not a deletion of game A.
func TestPS1PerGameCardsFromOneDeviceKeepEachOther(t *testing.T) {
	h := newContractHarness(t)
	upload := func(filename, slot, productCode, title string) {
		t.Helper()
		input := saveCreateInput{
			Filename: filename,
			Payload:  makeTestPS1Card(t, productCode, title),
			Game:     game{Name: "PlayStation Save", System: supportedSystemFromSlug("psx")},
			Format:   "srm",
			SlotName: slot,
		}
		preview := h.app.normalizeSaveInputDetailed(input)
		if _, _, err := h.app.createPlayStationProjectionSave(input, preview, "retroarch", "psx/retroarch", "deck-psx"); err != nil {
			t.Fatalf("upload %s: %v", filename, err)
		}
	}
	store := h.app.playStationSyncStore()
	upload("Final Fantasy VII (USA) (Disc 1).srm", "Game: Final Fantasy VII (USA) (Disc 1)", "SCUS_941.63", "Final Fantasy VII Save")
	// The helper restores the line's projection, which records it as this
	// device's baseline: the next upload is compared against it.
	store.markProjectionDownloaded("psx/retroarch", "Game: Final Fantasy VII (USA) (Disc 1)", "deck-psx")
	upload("Tekken 3 (USA).srm", "Game: Tekken 3 (USA)", "SLUS_004.02", "Tekken 3 Save")

	store.mu.Lock()
	var titles []string
	for _, logical := range store.state.LogicalSaves {
		if _, gone := store.state.Tombstones[psTombstoneKey(psScopeKey(logical.Portable, logical.SyncLineKey, logical.ProjectionLineKey), logical.Key)]; !gone {
			titles = append(titles, logical.DisplayTitle)
		}
	}
	store.mu.Unlock()
	joined := strings.ToLower(strings.Join(titles, " | "))
	if !strings.Contains(joined, "final fantasy") {
		t.Fatalf("game A's save was tombstoned by game B's card; live saves: %q", titles)
	}
	if !strings.Contains(joined, "tekken") {
		t.Fatalf("game B's save is missing; live saves: %q", titles)
	}
}

// A per-game card's save downloads under its game's name, the card the
// emulator reads, not under the title written inside the save.
func TestPS1PerGameCardDownloadsUnderTheGameName(t *testing.T) {
	h := newContractHarness(t)
	input := saveCreateInput{
		Filename: "Tekken 3 (USA).srm",
		Payload:  makeTestPS1Card(t, "SLUS_004.02", "Tekken 3 Save"),
		Game:     game{Name: "PlayStation Save", System: supportedSystemFromSlug("psx")},
		Format:   "srm",
		SlotName: "Game: Tekken 3 (USA)",
	}
	preview := h.app.normalizeSaveInputDetailed(input)
	if _, _, err := h.app.createPlayStationProjectionSave(input, preview, "retroarch", "psx/retroarch", "deck-psx"); err != nil {
		t.Fatalf("upload: %v", err)
	}
	list := h.request(http.MethodGet, "/saves?limit=20&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	var names []string
	for _, item := range filterSaveArrayBySystem(t, mustArray(t, decodeJSONMap(t, list.Body)["saves"], "saves"), "psx") {
		name, _ := item["filename"].(string)
		names = append(names, name)
	}
	found := false
	for _, name := range names {
		if strings.HasPrefix(name, "Tekken 3 (USA).") {
			found = true
		}
		if strings.Contains(strings.ToLower(name), "save") {
			t.Fatalf("a record is named after the save's title: %q", names)
		}
	}
	if !found {
		t.Fatalf("no record is named after the game's card: %q", names)
	}
}

// A save that moves to another per-game line must stop being served by the
// line it left. The card it was uploaded on is still a real file on disk, and
// its projection still holds the save, so a device that restores that line
// brings the save back as that game's card.
func TestPS1PerGameCardLeavesTheLineItVacated(t *testing.T) {
	h := newContractHarness(t)
	upload := func(filename, slot, productCode, title string) {
		t.Helper()
		input := saveCreateInput{
			Filename: filename,
			Payload:  makeTestPS1Card(t, productCode, title),
			Game:     game{Name: "PlayStation Save", System: supportedSystemFromSlug("psx")},
			Format:   "srm",
			SlotName: slot,
		}
		preview := h.app.normalizeSaveInputDetailed(input)
		if _, _, err := h.app.createPlayStationProjectionSave(input, preview, "retroarch", "psx/retroarch", "deck-psx"); err != nil {
			t.Fatalf("upload %s: %v", filename, err)
		}
	}
	// The save is on Final Fantasy's own card, uploaded last so that line's
	// latest projection holds it.
	upload("Tekken 3 (USA).srm", "Game: Tekken 3 (USA)", "SLUS_004.02", "Tekken 3 Save")
	upload("Final Fantasy VII (USA) (Disc 1).srm", "Game: Final Fantasy VII (USA) (Disc 1)", "SCUS_941.63", "Final Fantasy VII Save")
	// It then arrives on Tekken's card, so it belongs to that line now and
	// Final Fantasy's card is never re-uploaded.
	upload("Tekken 3 (USA).srm", "Game: Tekken 3 (USA)", "SCUS_941.63", "Final Fantasy VII Save")

	store := h.app.playStationSyncStore()
	store.mu.Lock()
	defer store.mu.Unlock()
	// The line Final Fantasy's card was uploaded on, found by slot rather
	// than by key: a correct fix retires the line, so the key may be gone.
	var serves []string
	for _, line := range store.state.ProjectionLines {
		if line.SystemSlug != "psx" || !strings.Contains(strings.ToLower(line.CardSlot), "final fantasy") {
			continue
		}
		projection, ok := store.state.Projections[strings.TrimSpace(line.LatestProjectionID)]
		if !ok {
			continue
		}
		for _, entry := range projection.Manifest {
			serves = append(serves, entry.DisplayTitle)
		}
	}
	for _, title := range serves {
		if strings.Contains(strings.ToLower(title), "final fantasy") {
			t.Fatalf("the vacated line still serves %q; a device restoring it writes the save onto the wrong card", title)
		}
	}
}
