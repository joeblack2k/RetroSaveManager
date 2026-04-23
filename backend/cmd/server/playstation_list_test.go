package main

import (
	"net/http"
	"testing"
	"time"
)

func filterSaveArrayBySystem(t *testing.T, saves []any, wantSystem string) []map[string]any {
	t.Helper()
	filtered := make([]map[string]any, 0, len(saves))
	for _, item := range saves {
		summary := mustObject(t, item, "save")
		if mustString(t, summary["systemSlug"], "systemSlug") != wantSystem {
			continue
		}
		filtered = append(filtered, summary)
	}
	return filtered
}

func TestPlayStationSavesListShowsLogicalSummaryAndSkipsLegacyRawCard(t *testing.T) {
	h := newContractHarness(t)
	payload := makeTestPS1Card(t, "SCUS_941.63", "Final Fantasy VII Save")

	_, err := h.app.createSave(saveCreateInput{
		Filename:   "memory_card_1.mcr",
		Payload:    payload,
		Game:       game{Name: "Legacy Card", System: supportedSystemFromSlug("psx")},
		Format:     "mcr",
		SystemSlug: "psx",
		SlotName:   "Memory Card 1",
		CreatedAt:  time.Unix(100, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("create legacy raw ps1 save: %v", err)
	}

	input := saveCreateInput{
		Filename:   "memory_card_1.mcr",
		Payload:    payload,
		Game:       game{Name: "PlayStation Save", System: supportedSystemFromSlug("psx")},
		Format:     "mcr",
		SystemSlug: "psx",
		SlotName:   "Memory Card 1",
		CreatedAt:  time.Unix(200, 0).UTC(),
	}
	preview := h.app.normalizeSaveInputDetailed(input)
	if preview.Rejected {
		t.Fatalf("unexpected preview rejection: %s", preview.RejectReason)
	}
	if _, _, err := h.app.createPlayStationProjectionSave(input, preview, "retroarch", "psx/retroarch", "deck-psx"); err != nil {
		t.Fatalf("create ps1 projection: %v", err)
	}

	list := h.request(http.MethodGet, "/saves?limit=20&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	body := decodeJSONMap(t, list.Body)
	psSaves := filterSaveArrayBySystem(t, mustArray(t, body["saves"], "saves"), "psx")
	if len(psSaves) != 1 {
		t.Fatalf("expected 1 logical ps1 row, got %d: %s", len(psSaves), prettyJSON(body))
	}

	summary := psSaves[0]
	if mustString(t, summary["displayTitle"], "displayTitle") != "Final Fantasy VII Save" {
		t.Fatalf("expected logical title, got %s", prettyJSON(summary))
	}
	if summary["memoryCard"] != nil {
		t.Fatalf("expected ps1 list summary to hide memoryCard details, got %s", prettyJSON(summary))
	}
	if mustString(t, summary["logicalKey"], "logicalKey") == "" {
		t.Fatalf("expected ps1 logicalKey on list summary, got %s", prettyJSON(summary))
	}
}

func TestPlayStationBackfilledPS2ListShowsLogicalGamesOnly(t *testing.T) {
	h := newContractHarness(t)
	payload := mustDecodePS2Fixture(t)

	_, err := h.app.createSave(saveCreateInput{
		Filename:   "Mcd001.ps2",
		Payload:    payload,
		Game:       game{Name: "Legacy PS2 Card", System: supportedSystemFromSlug("ps2")},
		Format:     "ps2",
		SystemSlug: "ps2",
		CreatedAt:  time.Unix(100, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("create legacy raw ps2 save: %v", err)
	}

	result, err := h.app.backfillPlayStation(playStationBackfillOptions{
		ReplaceRaw:         true,
		DefaultPS2CardSlot: "Memory Card 1",
	})
	if err != nil {
		t.Fatalf("backfill ps2: %v", err)
	}
	if result.Migrated != 1 {
		t.Fatalf("expected ps2 backfill to migrate 1 raw card, got %+v", result)
	}

	list := h.request(http.MethodGet, "/saves?limit=20&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	body := decodeJSONMap(t, list.Body)
	psSaves := filterSaveArrayBySystem(t, mustArray(t, body["saves"], "saves"), "ps2")
	if len(psSaves) != 2 {
		t.Fatalf("expected 2 logical ps2 rows, got %d: %s", len(psSaves), prettyJSON(body))
	}

	titles := map[string]bool{}
	for _, summary := range psSaves {
		title := mustString(t, summary["displayTitle"], "displayTitle")
		titles[title] = true
		if title == "Memory Card 1" {
			t.Fatalf("raw memory card leaked into ps2 list: %s", prettyJSON(summary))
		}
		if summary["memoryCard"] != nil {
			t.Fatalf("expected ps2 logical list summary to hide memoryCard details, got %s", prettyJSON(summary))
		}
		if mustString(t, summary["logicalKey"], "logicalKey") == "" {
			t.Fatalf("expected ps2 logicalKey on list summary, got %s", prettyJSON(summary))
		}
	}
	if !titles["Burnout 3"] || !titles["Mortal Kombat Shaolin Monks"] {
		t.Fatalf("unexpected ps2 logical titles: %#v", titles)
	}
}
