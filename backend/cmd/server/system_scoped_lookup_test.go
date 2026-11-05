package main

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

// One ROM stem, three systems: NBA Jam - Tournament Edition exists for the
// Genesis, the 32X and the Game Gear. Each system's save is keyed on its own ROM.
func seedThreeSystemStem(t *testing.T, h *contractHarness) map[string]string {
	t.Helper()
	const stem = "NBA Jam - Tournament Edition (World)"
	keys := map[string]string{}
	for i, sys := range []string{"genesis", "sega-32x", "game-gear"} {
		lookup := h.request(http.MethodGet, "/rom/lookup?filenameStem="+url.QueryEscape(stem)+"&system="+sys, nil)
		assertStatus(t, lookup, http.StatusOK)
		rom := mustObject(t, decodeJSONMap(t, lookup.Body)["rom"], "rom")
		sha1 := mustString(t, rom["sha1"], "rom.sha1")
		keys[sys] = sha1
		if _, err := h.app.createSave(saveCreateInput{
			Filename:            stem + ".srm",
			Payload:             buildNonBlankPayload(8192, byte(0x11+i)),
			Game:                game{Name: stem, System: supportedSystemFromSlug(sys)},
			Format:              "sram",
			ROMSHA1:             sha1,
			SystemSlug:          sys,
			SlotName:            "default",
			TrustedHelperSystem: true,
			CreatedAt:           time.Unix(int64(100+i), 0).UTC(),
		}); err != nil {
			t.Fatalf("seed %s: %v", sys, err)
		}
	}
	return keys
}

// A lookup that names its system keys on that system's ROM, so three systems
// sharing a stem do not share one key.
func TestRomLookupWithSystemKeysEachSystemApart(t *testing.T) {
	h := newContractHarness(t)
	keys := seedThreeSystemStem(t, h)
	if keys["genesis"] == keys["sega-32x"] || keys["genesis"] == keys["game-gear"] || keys["sega-32x"] == keys["game-gear"] {
		t.Fatalf("three systems sharing a stem got overlapping ROM keys: %v", keys)
	}
}

// /save/latest with system=genesis returns only the Genesis record and never
// the 32X or Game Gear record that shares the stem; with no Genesis record it
// returns nothing rather than another system's.
func TestSaveLatestWithSystemNeverOffersAnotherSystemsRecord(t *testing.T) {
	h := newContractHarness(t)
	const stem = "NBA Jam - Tournament Edition (World)"
	keys := seedThreeSystemStem(t, h)
	plain := h.request(http.MethodGet, "/rom/lookup?filenameStem="+url.QueryEscape(stem), nil)
	shared := mustString(t, mustObject(t, decodeJSONMap(t, plain.Body)["rom"], "rom")["sha1"], "rom.sha1")

	// The shared (stem-only) key, asked for as genesis, must never come back as
	// the 32X or Game Gear save: it is the Genesis save or nothing.
	genesisSHA := latestSHA(t, h, keys["genesis"], "genesis")
	if genesisSHA == "" {
		t.Fatalf("system=genesis did not find the genesis record by its own key")
	}
	if got := latestSHA(t, h, shared, "genesis"); got != "" && got != genesisSHA {
		t.Fatalf("system=genesis on the shared key was offered another system's save (sha256 %s)", got)
	}
	// A 32X key asked for as genesis must come back empty, not as the 32X save.
	if got := latestSHA(t, h, keys["sega-32x"], "genesis"); got != "" {
		t.Fatalf("system=genesis was offered the 32X record (sha256 %s)", got)
	}
}

func latestSHA(t *testing.T, h *contractHarness, romSHA1, system string) string {
	t.Helper()
	rr := h.request(http.MethodGet, "/save/latest?romSha1="+romSHA1+"&slotName=default&system="+system, nil)
	assertStatus(t, rr, http.StatusOK)
	body := decodeJSONMap(t, rr.Body)
	if exists, _ := body["exists"].(bool); !exists {
		return ""
	}
	sha, _ := body["sha256"].(string)
	return sha
}

// Without system, lookup and latest behave exactly as before.
func TestRomLookupWithoutSystemIsUnchanged(t *testing.T) {
	h := newContractHarness(t)
	rr := h.request(http.MethodGet, "/rom/lookup?filenameStem=Wario%20Land%20II", nil)
	rom := mustObject(t, decodeJSONMap(t, rr.Body)["rom"], "rom")
	if got, want := mustString(t, rom["sha1"], "rom.sha1"), deterministicSHA1Hex("rom:Wario Land II"); got != want {
		t.Fatalf("lookup without system changed its key: got %s want %s", got, want)
	}
}
