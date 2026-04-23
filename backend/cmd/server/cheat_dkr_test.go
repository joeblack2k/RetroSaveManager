package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestDKRCheatEditorReadAndApply(t *testing.T) {
	pack := mustLoadDKRCheatPack(t)
	payload := buildDKRFixturePayload()
	editor := dkrEEPROMCheatEditor{}

	state, err := editor.Read(pack, payload)
	if err != nil {
		t.Fatalf("read cheats: %v", err)
	}
	slotA := state.SlotValues["A"]
	if slotA == nil {
		t.Fatalf("expected slot A values")
	}
	if total, ok := slotA["balloonsTotal"].(int); !ok || total != 2 {
		t.Fatalf("expected total balloons 2, got %#v", slotA["balloonsTotal"])
	}
	if status, ok := slotA["ancientLakeStatus"].(string); !ok || status != "completed" {
		t.Fatalf("expected Ancient Lake completed, got %#v", slotA["ancientLakeStatus"])
	}

	updated, changed, err := editor.Apply(pack, payload, "A", map[string]any{
		"balloonsTotal":     10,
		"keys":              []string{"dinoDomainKey", "sherbetIslandKey"},
		"defeatedBosses":    []string{"tricky1", "wizpig1"},
		"tajChallenges":     []string{"carChallengeUnlocked"},
		"ttAmulet":          2,
		"dinoDomainTrophy":  "firstPlace",
		"ancientLakeStatus": "silver",
	})
	if err != nil {
		t.Fatalf("apply cheats: %v", err)
	}
	if _, ok := changed["keys"]; !ok {
		t.Fatalf("expected changed map to include keys")
	}
	parsed, err := parseDKREEPROM(updated)
	if err != nil {
		t.Fatalf("parse updated payload: %v", err)
	}
	slot := parsed.Slots[0]
	if slot.Balloons[0] != 10 {
		t.Fatalf("expected total balloons 10, got %d", slot.Balloons[0])
	}
	if slot.Keys&(1<<1) == 0 || slot.Keys&(1<<2) == 0 {
		t.Fatalf("expected Dino and Sherbet keys to be set, got 0x%02x", slot.Keys)
	}
	if slot.Bosses&(1<<0) == 0 || slot.Bosses&(1<<1) == 0 {
		t.Fatalf("expected Wizpig 1 and Tricky 1 bits, got 0x%03x", slot.Bosses)
	}
	if slot.TajFlags != 1 {
		t.Fatalf("expected Taj flags 1, got 0x%02x", slot.TajFlags)
	}
	if slot.TTAmulet != 2 {
		t.Fatalf("expected TT amulet 2, got %d", slot.TTAmulet)
	}
	if slot.Trophies[0] != 3 {
		t.Fatalf("expected Dino trophy first place, got %d", slot.Trophies[0])
	}
	if slot.CourseStatus[3] != 3 {
		t.Fatalf("expected Ancient Lake silver status, got %d", slot.CourseStatus[3])
	}
	if slot.StoredChecksum != dkrSlotChecksum(updated[:dkrSlotSize]) {
		t.Fatalf("expected checksum to be rebuilt")
	}
}

func TestSaveCheatEndpointsExposeDKRPack(t *testing.T) {
	h := newContractHarness(t)
	saveID := seedDKRSave(t, h, "/saves")

	root := h.request(http.MethodGet, "/save/cheats?saveId="+saveID, nil)
	v1 := h.request(http.MethodGet, "/v1/save/cheats?saveId="+saveID, nil)
	assertStatus(t, root, http.StatusOK)
	assertStatus(t, v1, http.StatusOK)
	assertJSONContentType(t, root)
	assertJSONContentType(t, v1)
	assertEqualJSONValue(t, normalizeForGolden(decodeJSONMap(t, root.Body)), normalizeForGolden(decodeJSONMap(t, v1.Body)), "dkr save cheats")

	body := decodeJSONMap(t, root.Body)
	cheats := mustObject(t, body["cheats"], "cheats")
	if !mustBool(t, cheats["supported"], "cheats.supported") {
		t.Fatalf("expected supported cheats payload")
	}
	if mustString(t, cheats["editorId"], "cheats.editorId") != "dkr-eeprom" {
		t.Fatalf("expected dkr-eeprom editor: %s", root.Body.String())
	}
	presets := mustArray(t, cheats["presets"], "cheats.presets")
	foundKeys := false
	for _, preset := range presets {
		item := mustObject(t, preset, "preset")
		if mustString(t, item["id"], "preset.id") == "unlockAllKeys" {
			foundKeys = true
			break
		}
	}
	if !foundKeys {
		t.Fatalf("expected unlockAllKeys preset in response: %s", root.Body.String())
	}
}

func TestSaveCheatApplyDKRCreatesNewCurrentVersion(t *testing.T) {
	h := newContractHarness(t)
	saveID := seedDKRSave(t, h, "/saves")
	applyReq := fmt.Sprintf(`{"saveId":%q,"editorId":"dkr-eeprom","slotId":"A","presetIds":["unlockAllKeys","allFirstPlaceTrophies"],"updates":{"balloonsTotal":10,"ttAmulet":4}}`, saveID)
	rr := h.json(http.MethodPost, "/save/cheats/apply", strings.NewReader(applyReq))
	assertStatus(t, rr, http.StatusOK)
	assertJSONContentType(t, rr)

	body := decodeJSONMap(t, rr.Body)
	if !mustBool(t, body["success"], "success") {
		t.Fatalf("expected success=true")
	}
	save := mustObject(t, body["save"], "save")
	newID := mustString(t, save["id"], "save.id")
	if mustString(t, mustObject(t, save["cheats"], "save.cheats")["editorId"], "save.cheats.editorId") != "dkr-eeprom" {
		t.Fatalf("expected DKR cheat capability on new version: %s", rr.Body.String())
	}

	cheatState := h.request(http.MethodGet, "/save/cheats?saveId="+newID, nil)
	assertStatus(t, cheatState, http.StatusOK)
	cheatBody := decodeJSONMap(t, cheatState.Body)
	slotA := mustObject(t, mustObject(t, mustObject(t, cheatBody["cheats"], "cheats")["slotValues"], "slotValues")["A"], "slotValues.A")
	if mustNumber(t, slotA["balloonsTotal"], "slotA.balloonsTotal") != 10 {
		t.Fatalf("expected total balloons 10 after apply: %s", cheatState.Body.String())
	}
	keys := mustArray(t, slotA["keys"], "slotA.keys")
	if len(keys) != 4 {
		t.Fatalf("expected 4 unlocked keys after preset: %s", cheatState.Body.String())
	}
	for _, fieldID := range []string{
		"dinoDomainTrophy",
		"sherbetIslandTrophy",
		"snowflakeMountTrophy",
		"dragonForestTrophy",
		"futureFunLandTrophy",
	} {
		if mustString(t, slotA[fieldID], fieldID) != "firstPlace" {
			t.Fatalf("expected %s to be firstPlace after preset: %s", fieldID, cheatState.Body.String())
		}
	}
	if mustNumber(t, slotA["ttAmulet"], "slotA.ttAmulet") != 4 {
		t.Fatalf("expected ttAmulet 4 after apply: %s", cheatState.Body.String())
	}
}

func mustLoadDKRCheatPack(t *testing.T) cheatPack {
	t.Helper()
	root, err := findCuratedCheatPackRoot()
	if err != nil {
		t.Fatalf("find curated cheat pack root: %v", err)
	}
	pack, err := loadCheatPackFile(filepath.Join(root, "n64", "diddy-kong-racing.yaml"))
	if err != nil {
		t.Fatalf("load dkr cheat pack: %v", err)
	}
	return pack
}

func buildDKRFixturePayload() []byte {
	payload := make([]byte, dkrEEPROMSize)
	slot := dkrParsedSlot{
		Present:       true,
		CutsceneFlags: 0x40000,
		Name:          0x0c7c,
	}
	slot.CourseStatus[3] = 2
	slot.Balloons[0] = 2
	slot.Balloons[1] = 1
	slot.FlagsWorld[0] = 0x403
	slot.FlagsWorld[1] = 0x36
	copy(payload[:dkrSlotSize], dkrEncodeSlot(slot))
	for slotIndex := 1; slotIndex < dkrSlotCount; slotIndex++ {
		start := dkrSlotOffset(slotIndex)
		end := start + dkrSlotSize
		for i := start; i < end; i++ {
			payload[i] = 0xff
		}
	}
	return payload
}

func seedDKRSave(t *testing.T, h *contractHarness, path string) string {
	t.Helper()
	body := uploadSave(t, h, path, map[string]string{
		"rom_sha1": "dkr-rom",
		"slotName": "default",
		"system":   "n64",
	}, "Diddy Kong Racing (USA).eep", buildDKRFixturePayload())
	return mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
}
