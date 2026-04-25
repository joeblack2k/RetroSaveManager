package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestSF64CheatEditorReadAndApply(t *testing.T) {
	pack := mustLoadSF64CheatPack(t)
	payload := buildSF64FixturePayload()
	editor := sf64EEPROMCheatEditor{}

	state, err := editor.Read(pack, payload)
	if err != nil {
		t.Fatalf("read cheats: %v", err)
	}
	values := state.Values
	if soundMode, ok := values["soundMode"].(string); !ok || soundMode != "stereo" {
		t.Fatalf("expected stereo sound mode, got %#v", values["soundMode"])
	}
	if !stringSliceContains(mustStringSliceCheatValue(t, values["normalClearPlanets"], "normalClearPlanets"), "corneria") {
		t.Fatalf("expected Corneria normal clear in fixture, got %#v", values["normalClearPlanets"])
	}

	updated, changed, err := editor.Apply(pack, payload, "", map[string]any{
		"soundMode":          "headphones",
		"musicVolume":        80,
		"playedPlanets":      []string{"corneria", "venom2"},
		"normalClearPlanets": []string{"venom2"},
		"expertClearPlanets": []string{"venom2"},
		"normalMedalPlanets": []string{"corneria"},
		"expertMedalPlanets": []string{"venom2"},
	})
	if err != nil {
		t.Fatalf("apply cheats: %v", err)
	}
	if _, ok := changed["expertClearPlanets"]; !ok {
		t.Fatalf("expected changed map to include expertClearPlanets")
	}
	parsed, err := parseSF64EEPROM(updated)
	if err != nil {
		t.Fatalf("parse updated payload: %v", err)
	}
	save := parsed.Save
	if save[sf64SoundModeOffset] != 2 {
		t.Fatalf("expected headphones sound mode after apply, got %d", save[sf64SoundModeOffset])
	}
	if save[sf64MusicVolumeOffset] != 80 {
		t.Fatalf("expected music volume 80 after apply, got %d", save[sf64MusicVolumeOffset])
	}
	if save[sf64SaveSlotVenom2]&sf64PlanetFlagNormalClear == 0 {
		t.Fatalf("expected Venom 2 normal clear flag to unlock Landmaster")
	}
	if save[sf64SaveSlotVenom2]&sf64PlanetFlagExpertClear == 0 {
		t.Fatalf("expected Venom 2 expert clear flag to unlock on-foot mode")
	}
	if save[9]&sf64PlanetFlagNormalMedal == 0 {
		t.Fatalf("expected Corneria normal medal flag")
	}
	if save[9]&sf64PlanetFlagNormalClear != 0 {
		t.Fatalf("expected normal clear selection to replace the old Corneria clear flag")
	}
	if !sf64VerifySaveBlock(updated[:sf64SaveBlockSize]) {
		t.Fatalf("expected rebuilt primary checksum to be valid")
	}
	if !sf64VerifySaveBlock(updated[sf64SaveBlockSize : sf64SaveBlockSize*2]) {
		t.Fatalf("expected rebuilt backup checksum to be valid")
	}
}

func TestSF64CheatEditorUsesBackupWhenPrimaryIsInvalid(t *testing.T) {
	pack := mustLoadSF64CheatPack(t)
	payload := buildSF64FixturePayload()
	payload[0] ^= 0x7f
	editor := sf64EEPROMCheatEditor{}

	state, err := editor.Read(pack, payload)
	if err != nil {
		t.Fatalf("read cheats from backup: %v", err)
	}
	if soundMode, ok := state.Values["soundMode"].(string); !ok || soundMode != "stereo" {
		t.Fatalf("expected backup values after primary corruption, got %#v", state.Values["soundMode"])
	}

	updated, _, err := editor.Apply(pack, payload, "", map[string]any{
		"normalClearPlanets": []string{"venom2"},
	})
	if err != nil {
		t.Fatalf("apply cheats from backup: %v", err)
	}
	if !sf64VerifySaveBlock(updated[:sf64SaveBlockSize]) || !sf64VerifySaveBlock(updated[sf64SaveBlockSize:sf64SaveBlockSize*2]) {
		t.Fatalf("expected apply to repair both Star Fox 64 save mirrors")
	}
}

func TestSaveCheatEndpointsExposeStarFox64Pack(t *testing.T) {
	h := newContractHarness(t)
	saveID := seedSF64Save(t, h, "/saves")

	root := h.request(http.MethodGet, "/save/cheats?saveId="+saveID, nil)
	v1 := h.request(http.MethodGet, "/v1/save/cheats?saveId="+saveID, nil)
	assertStatus(t, root, http.StatusOK)
	assertStatus(t, v1, http.StatusOK)
	assertJSONContentType(t, root)
	assertJSONContentType(t, v1)
	assertEqualJSONValue(t, normalizeForGolden(decodeJSONMap(t, root.Body)), normalizeForGolden(decodeJSONMap(t, v1.Body)), "sf64 save cheats")

	body := decodeJSONMap(t, root.Body)
	cheats := mustObject(t, body["cheats"], "cheats")
	if !mustBool(t, cheats["supported"], "cheats.supported") {
		t.Fatalf("expected supported cheats payload")
	}
	if mustString(t, cheats["editorId"], "cheats.editorId") != "sf64-eeprom" {
		t.Fatalf("expected sf64-eeprom editor: %s", root.Body.String())
	}
	presets := mustArray(t, cheats["presets"], "cheats.presets")
	foundVersus := false
	for _, preset := range presets {
		item := mustObject(t, preset, "preset")
		if mustString(t, item["id"], "preset.id") == "unlockVersusVehicles" {
			foundVersus = true
			break
		}
	}
	if !foundVersus {
		t.Fatalf("expected unlockVersusVehicles preset in response: %s", root.Body.String())
	}
}

func TestSaveCheatApplyStarFox64CreatesNewCurrentVersionOnRootAndV1(t *testing.T) {
	for _, prefix := range []string{"", "/v1"} {
		t.Run(firstNonEmpty(prefix, "root"), func(t *testing.T) {
			h := newContractHarness(t)
			saveID := seedSF64Save(t, h, prefix+"/saves")
			applyReq := fmt.Sprintf(`{"saveId":%q,"editorId":"sf64-eeprom","presetIds":["unlockVersusVehicles"],"updates":{"soundMode":"mono","musicVolume":75}}`, saveID)
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

			latest := h.request(http.MethodGet, prefix+"/save/latest?romSha1=sf64-rom&slotName=default", nil)
			assertStatus(t, latest, http.StatusOK)
			latestBody := decodeJSONMap(t, latest.Body)
			if mustString(t, latestBody["id"], "id") != newID {
				t.Fatalf("expected cheat-applied save to become latest: %s", latest.Body.String())
			}

			cheatState := h.request(http.MethodGet, prefix+"/save/cheats?saveId="+newID, nil)
			assertStatus(t, cheatState, http.StatusOK)
			cheatBody := decodeJSONMap(t, cheatState.Body)
			values := mustObject(t, mustObject(t, cheatBody["cheats"], "cheats")["values"], "cheats.values")
			if mustString(t, values["soundMode"], "values.soundMode") != "mono" {
				t.Fatalf("expected sound mode mono after apply: %s", cheatState.Body.String())
			}
			if mustNumber(t, values["musicVolume"], "values.musicVolume") != 75 {
				t.Fatalf("expected music volume 75 after apply: %s", cheatState.Body.String())
			}
			normalClear := mustArray(t, values["normalClearPlanets"], "values.normalClearPlanets")
			expertClear := mustArray(t, values["expertClearPlanets"], "values.expertClearPlanets")
			if !jsonStringArrayContains(normalClear, "venom2") {
				t.Fatalf("expected Venom 2 normal clear after preset: %s", cheatState.Body.String())
			}
			if !jsonStringArrayContains(expertClear, "venom2") {
				t.Fatalf("expected Venom 2 expert clear after preset: %s", cheatState.Body.String())
			}
		})
	}
}

func mustLoadSF64CheatPack(t *testing.T) cheatPack {
	t.Helper()
	root, err := findCuratedCheatPackRoot()
	if err != nil {
		t.Fatalf("find curated cheat pack root: %v", err)
	}
	pack, err := loadCheatPackFile(filepath.Join(root, "n64", "star-fox-64.yaml"))
	if err != nil {
		t.Fatalf("load sf64 cheat pack: %v", err)
	}
	return pack
}

func buildSF64FixturePayload() []byte {
	payload := make([]byte, sf64EEPROMSize)
	save := buildSF64FixtureSaveBlock()
	copy(payload[:sf64SaveBlockSize], save)
	copy(payload[sf64SaveBlockSize:sf64SaveBlockSize*2], save)
	return payload
}

func buildSF64FixtureSaveBlock() []byte {
	save := make([]byte, sf64SaveBlockSize)
	save[9] = sf64PlanetFlagPlayed | sf64PlanetFlagNormalClear
	save[sf64SaveSlotVenom2] = sf64PlanetFlagPlayed
	save[sf64SoundModeOffset] = 0
	save[sf64MusicVolumeOffset] = 99
	save[sf64VoiceVolumeOffset] = 99
	save[sf64SFXVolumeOffset] = 99
	sf64WriteChecksum(save)
	return save
}

func seedSF64Save(t *testing.T, h *contractHarness, path string) string {
	t.Helper()
	body := uploadSave(t, h, path, map[string]string{
		"rom_sha1": "sf64-rom",
		"slotName": "default",
		"system":   "n64",
	}, "Star Fox 64 (USA).eep", buildSF64FixturePayload())
	return mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
}

func mustStringSliceCheatValue(t *testing.T, value any, field string) []string {
	t.Helper()
	values, ok := value.([]string)
	if !ok {
		t.Fatalf("expected %s to be []string, got %#v", field, value)
	}
	return values
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func jsonStringArrayContains(values []any, want string) bool {
	for _, value := range values {
		if text, ok := value.(string); ok && text == want {
			return true
		}
	}
	return false
}
