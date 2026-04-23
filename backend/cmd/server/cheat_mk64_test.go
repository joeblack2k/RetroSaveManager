package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestMK64CheatEditorReadAndApply(t *testing.T) {
	pack := mustLoadMK64CheatPack(t)
	payload := buildMK64FixturePayload()
	editor := mk64EEPROMCheatEditor{}

	state, err := editor.Read(pack, payload)
	if err != nil {
		t.Fatalf("read cheats: %v", err)
	}
	if soundMode, ok := state.Values["soundMode"].(string); !ok || soundMode != "stereo" {
		t.Fatalf("expected stereo sound mode, got %#v", state.Values["soundMode"])
	}
	if cupValue, ok := state.Values["gp100Flower"].(string); !ok || cupValue != "silver" {
		t.Fatalf("expected 100cc Flower Cup to be silver, got %#v", state.Values["gp100Flower"])
	}

	updated, changed, err := editor.Apply(pack, payload, "", map[string]any{
		"soundMode":    "headphones",
		"gp100Flower":  "gold",
		"gp150Special": "gold",
	})
	if err != nil {
		t.Fatalf("apply cheats: %v", err)
	}
	if _, ok := changed["gp100Flower"]; !ok {
		t.Fatalf("expected changed map to include gp100Flower")
	}
	parsed, err := parseMK64EEPROM(updated)
	if err != nil {
		t.Fatalf("parse updated payload: %v", err)
	}
	if parsed.Active.SoundMode != 1 {
		t.Fatalf("expected sound mode 1 after apply, got %d", parsed.Active.SoundMode)
	}
	if mk64CupPoints(parsed.Active.GrandPrixPoints[1], 1) != 3 {
		t.Fatalf("expected 100cc Flower Cup to become gold")
	}
	if mk64CupPoints(parsed.Active.GrandPrixPoints[2], 3) != 3 {
		t.Fatalf("expected 150cc Special Cup to become gold")
	}
	if !mk64VerifyStuff(updated[mk64MainOffset : mk64MainOffset+mk64StuffSize]) {
		t.Fatalf("expected rebuilt main save-info checksum to be valid")
	}
	if !mk64VerifyStuff(updated[mk64BackupOffset : mk64BackupOffset+mk64StuffSize]) {
		t.Fatalf("expected rebuilt backup save-info checksum to be valid")
	}
}

func TestSaveCheatEndpointsExposeMarioKart64Pack(t *testing.T) {
	h := newContractHarness(t)
	saveID := seedMK64Save(t, h, "/saves")

	root := h.request(http.MethodGet, "/save/cheats?saveId="+saveID, nil)
	v1 := h.request(http.MethodGet, "/v1/save/cheats?saveId="+saveID, nil)
	assertStatus(t, root, http.StatusOK)
	assertStatus(t, v1, http.StatusOK)
	assertJSONContentType(t, root)
	assertJSONContentType(t, v1)
	assertEqualJSONValue(t, normalizeForGolden(decodeJSONMap(t, root.Body)), normalizeForGolden(decodeJSONMap(t, v1.Body)), "mk64 save cheats")

	body := decodeJSONMap(t, root.Body)
	cheats := mustObject(t, body["cheats"], "cheats")
	if !mustBool(t, cheats["supported"], "cheats.supported") {
		t.Fatalf("expected supported cheats payload")
	}
	if mustString(t, cheats["editorId"], "cheats.editorId") != "mk64-eeprom" {
		t.Fatalf("expected mk64-eeprom editor: %s", root.Body.String())
	}
	presets := mustArray(t, cheats["presets"], "cheats.presets")
	foundUnlockExtra := false
	for _, preset := range presets {
		item := mustObject(t, preset, "preset")
		if mustString(t, item["id"], "preset.id") == "unlockExtraMode" {
			foundUnlockExtra = true
			break
		}
	}
	if !foundUnlockExtra {
		t.Fatalf("expected unlockExtraMode preset in response: %s", root.Body.String())
	}
}

func TestSaveCheatApplyMarioKart64CreatesNewCurrentVersionOnRootAndV1(t *testing.T) {
	for _, prefix := range []string{"", "/v1"} {
		t.Run(firstNonEmpty(prefix, "root"), func(t *testing.T) {
			h := newContractHarness(t)
			saveID := seedMK64Save(t, h, prefix+"/saves")
			applyReq := fmt.Sprintf(`{"saveId":%q,"editorId":"mk64-eeprom","presetIds":["unlockExtraMode"],"updates":{"soundMode":"headphones"}}`, saveID)
			rr := h.json(http.MethodPost, prefix+"/save/cheats/apply", strings.NewReader(applyReq))
			assertStatus(t, rr, http.StatusOK)
			assertJSONContentType(t, rr)
			body := decodeJSONMap(t, rr.Body)
			if !mustBool(t, body["success"], "success") {
				t.Fatalf("expected success=true")
			}
			save := mustObject(t, body["save"], "save")
			newID := mustString(t, save["id"], "save.id")
			if mustNumber(t, save["version"], "save.version") != 2 {
				t.Fatalf("expected new version 2, got %s", rr.Body.String())
			}

			latest := h.request(http.MethodGet, prefix+"/save/latest?romSha1=mk64-rom&slotName=default", nil)
			assertStatus(t, latest, http.StatusOK)
			latestBody := decodeJSONMap(t, latest.Body)
			if mustString(t, latestBody["id"], "id") != newID {
				t.Fatalf("expected cheat-applied save to become latest: %s", latest.Body.String())
			}

			cheatState := h.request(http.MethodGet, prefix+"/save/cheats?saveId="+newID, nil)
			assertStatus(t, cheatState, http.StatusOK)
			cheatBody := decodeJSONMap(t, cheatState.Body)
			values := mustObject(t, mustObject(t, cheatBody["cheats"], "cheats")["values"], "cheats.values")
			if mustString(t, values["soundMode"], "values.soundMode") != "headphones" {
				t.Fatalf("expected sound mode headphones after apply: %s", cheatState.Body.String())
			}
			for _, fieldID := range []string{"gp150Mushroom", "gp150Flower", "gp150Star", "gp150Special"} {
				if mustString(t, values[fieldID], fieldID) != "gold" {
					t.Fatalf("expected %s to be gold after preset: %s", fieldID, cheatState.Body.String())
				}
			}
		})
	}
}

func mustLoadMK64CheatPack(t *testing.T) cheatPack {
	t.Helper()
	root, err := findCuratedCheatPackRoot()
	if err != nil {
		t.Fatalf("find curated cheat pack root: %v", err)
	}
	pack, err := loadCheatPackFile(filepath.Join(root, "n64", "mario-kart-64.yaml"))
	if err != nil {
		t.Fatalf("load mk64 cheat pack: %v", err)
	}
	return pack
}

func buildMK64FixturePayload() []byte {
	payload := make([]byte, mk64EEPROMSize)
	state := mk64ParsedStuff{SoundMode: 0}
	state.GrandPrixPoints[0] = mk64SetCupPoints(state.GrandPrixPoints[0], 0, 3)
	state.GrandPrixPoints[0] = mk64SetCupPoints(state.GrandPrixPoints[0], 1, 2)
	state.GrandPrixPoints[1] = mk64SetCupPoints(state.GrandPrixPoints[1], 1, 2)
	state.GrandPrixPoints[2] = mk64SetCupPoints(state.GrandPrixPoints[2], 0, 1)
	stuff := mk64BuildStuff(state)
	copy(payload[mk64MainOffset:mk64MainOffset+mk64StuffSize], stuff)
	copy(payload[mk64BackupOffset:mk64BackupOffset+mk64StuffSize], stuff)
	return payload
}

func seedMK64Save(t *testing.T, h *contractHarness, path string) string {
	t.Helper()
	body := uploadSave(t, h, path, map[string]string{
		"rom_sha1": "mk64-rom",
		"slotName": "default",
		"system":   "n64",
	}, "Mario Kart 64 (USA).eep", buildMK64FixturePayload())
	return mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
}
