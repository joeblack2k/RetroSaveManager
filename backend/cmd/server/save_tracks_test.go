package main

import (
	"net/http"
	"testing"
)

func TestCanonicalTrackDedupesFilenameVariantsInListAndHistory(t *testing.T) {
	h := newContractHarness(t)

	first := uploadSave(t, h, "/saves", map[string]string{}, "Star Fox 64 (USA).eep", []byte("star-fox-v1"))
	firstID := mustString(t, mustObject(t, first["save"], "save")["id"], "save.id")

	second := uploadSave(t, h, "/saves", map[string]string{}, "Star Fox 64 (USA) (Rev 1).eep", []byte("star-fox-v2"))
	secondID := mustString(t, mustObject(t, second["save"], "save")["id"], "save.id")

	list := h.request(http.MethodGet, "/saves?limit=20&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	saves := mustArray(t, listBody["saves"], "saves")

	matches := 0
	for _, item := range saves {
		summary := mustObject(t, item, "saves[n]")
		if mustString(t, summary["displayTitle"], "displayTitle") != "Star Fox 64" {
			continue
		}
		matches++
		if mustString(t, summary["regionCode"], "regionCode") != regionUS {
			t.Fatalf("expected US region for Star Fox summary, got %s", prettyJSON(summary))
		}
		if mustNumber(t, summary["saveCount"], "saveCount") != 2 {
			t.Fatalf("expected saveCount=2 for Star Fox summary, got %s", prettyJSON(summary))
		}
		if mustNumber(t, summary["latestVersion"], "latestVersion") != 2 {
			t.Fatalf("expected latestVersion=2 for Star Fox summary, got %s", prettyJSON(summary))
		}
		if mustString(t, summary["id"], "id") != secondID {
			t.Fatalf("expected newest save id to represent the track, got %s", prettyJSON(summary))
		}
	}
	if matches != 1 {
		t.Fatalf("expected exactly one Star Fox list row, got %d", matches)
	}

	history := h.request(http.MethodGet, "/save?saveId="+firstID, nil)
	assertStatus(t, history, http.StatusOK)
	historyBody := decodeJSONMap(t, history.Body)
	versions := mustArray(t, historyBody["versions"], "versions")
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions in Star Fox history, got %d", len(versions))
	}
	if mustString(t, mustObject(t, versions[0], "versions[0]")["id"], "versions[0].id") != secondID {
		t.Fatalf("expected newest Star Fox save first in history")
	}
}

func TestCanonicalTrackSeparatesRegions(t *testing.T) {
	h := newContractHarness(t)

	uploadSave(t, h, "/saves", map[string]string{}, "Wave Race 64 (USA).eep", []byte("wave-race-us"))
	uploadSave(t, h, "/saves", map[string]string{}, "Wave Race 64 (Japan).eep", []byte("wave-race-jp"))

	list := h.request(http.MethodGet, "/saves?limit=20&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	saves := mustArray(t, listBody["saves"], "saves")

	regions := map[string]struct{}{}
	for _, item := range saves {
		summary := mustObject(t, item, "saves[n]")
		if mustString(t, summary["displayTitle"], "displayTitle") != "Wave Race 64" {
			continue
		}
		regions[mustString(t, summary["regionCode"], "regionCode")] = struct{}{}
	}
	if _, ok := regions[regionUS]; !ok {
		t.Fatalf("expected US Wave Race track in list, got %#v", regions)
	}
	if _, ok := regions[regionJP]; !ok {
		t.Fatalf("expected JP Wave Race track in list, got %#v", regions)
	}
	if len(regions) != 2 {
		t.Fatalf("expected exactly 2 Wave Race region tracks, got %#v", regions)
	}
}
