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
