package main

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestALTTPCheatEditorReadAndApply(t *testing.T) {
	pack := mustLoadALTTPCheatPack(t)
	payload := buildALTTPFixturePayload()
	editor := alttpSRAMCheatEditor{}

	state, err := editor.Read(pack, payload)
	if err != nil {
		t.Fatalf("read cheats: %v", err)
	}
	slot2 := state.SlotValues["2"]
	if slot2 == nil {
		t.Fatalf("expected slot 2 values")
	}
	if rupees, ok := slot2["rupees"].(int); !ok || rupees != 100 {
		t.Fatalf("expected 100 rupees, got %#v", slot2["rupees"])
	}
	if sword, ok := slot2["sword"].(string); !ok || sword != "master" {
		t.Fatalf("expected Master Sword, got %#v", slot2["sword"])
	}
	if hookshot, ok := slot2["hookshot"].(bool); !ok || !hookshot {
		t.Fatalf("expected hookshot=true, got %#v", slot2["hookshot"])
	}
	if pendants := sortedStringArray(t, slot2["pendants"]); !reflect.DeepEqual(pendants, []string{"redPendant"}) {
		t.Fatalf("expected red pendant, got %#v", pendants)
	}

	updated, changed, err := editor.Apply(pack, payload, "2", map[string]any{
		"rupees":          999,
		"heartContainers": 20,
		"magicUpgrade":    "quarter",
		"pegasusBoots":    true,
		"bottle1":         "bluePotion",
		"pendants":        []string{"greenPendant", "bluePendant", "redPendant"},
		"crystals":        []string{"palaceOfDarkness", "swampPalace", "skullWoods", "thievesTown", "icePalace", "miseryMire", "turtleRock"},
	})
	if err != nil {
		t.Fatalf("apply cheats: %v", err)
	}
	if _, ok := changed["pegasusBoots"]; !ok {
		t.Fatalf("expected changed map to include pegasusBoots")
	}
	parsed, err := parseALTTPSRAM(updated)
	if err != nil {
		t.Fatalf("parse updated payload: %v", err)
	}
	block := parsed.Slots[1].Block
	if !bytesEqual(updated[alttpSlotOffset(1):alttpSlotOffset(1)+alttpSlotSize], updated[alttpMirrorSlotOffset(1):alttpMirrorSlotOffset(1)+alttpSlotSize]) {
		t.Fatalf("expected primary and mirror slot 2 to match after apply")
	}
	if _, ok := alttpVerifyBlock(block); !ok {
		t.Fatalf("expected rebuilt ALTTP block checksum to be valid")
	}
	if got := binary.LittleEndian.Uint16(block[alttpItemBaseOffset+0x22 : alttpItemBaseOffset+0x24]); got != 999 {
		t.Fatalf("expected actual rupees 999, got %d", got)
	}
	if got := binary.LittleEndian.Uint16(block[alttpItemBaseOffset+0x20 : alttpItemBaseOffset+0x22]); got != 999 {
		t.Fatalf("expected goal rupees 999, got %d", got)
	}
	if block[alttpItemBaseOffset+0x2c] != 160 || block[alttpItemBaseOffset+0x2d] != 160 {
		t.Fatalf("expected full health 160/160, got %d/%d", block[alttpItemBaseOffset+0x2d], block[alttpItemBaseOffset+0x2c])
	}
	if block[alttpItemBaseOffset+0x3b] != 2 {
		t.Fatalf("expected quarter magic upgrade, got %d", block[alttpItemBaseOffset+0x3b])
	}
	if block[alttpItemBaseOffset+0x15] != 1 || block[alttpItemBaseOffset+0x39]&0x04 == 0 {
		t.Fatalf("expected Pegasus Boots item and ability flag")
	}
	if block[alttpItemBaseOffset+0x0f] != 1 || block[alttpItemBaseOffset+0x1c] != 5 {
		t.Fatalf("expected selected blue potion bottle 1, selected=%d bottle1=%d", block[alttpItemBaseOffset+0x0f], block[alttpItemBaseOffset+0x1c])
	}
	if block[alttpItemBaseOffset+0x34] != 0x07 {
		t.Fatalf("expected all pendants mask 0x07, got 0x%02x", block[alttpItemBaseOffset+0x34])
	}
	if block[alttpItemBaseOffset+0x3a] != 0x7f {
		t.Fatalf("expected all crystals mask 0x7f, got 0x%02x", block[alttpItemBaseOffset+0x3a])
	}
}

func TestSaveCheatEndpointsExposeALTTPPack(t *testing.T) {
	h := newContractHarness(t)
	saveID := seedALTTPSave(t, h, "/saves")

	root := h.request(http.MethodGet, "/save/cheats?saveId="+saveID, nil)
	assertStatus(t, root, http.StatusOK)
	assertJSONContentType(t, root)

	body := decodeJSONMap(t, root.Body)
	cheats := mustObject(t, body["cheats"], "cheats")
	if !mustBool(t, cheats["supported"], "cheats.supported") {
		t.Fatalf("expected supported cheats payload")
	}
	if mustString(t, cheats["editorId"], "cheats.editorId") != "alttp-sram" {
		t.Fatalf("expected alttp-sram editor: %s", root.Body.String())
	}
	selector := mustObject(t, cheats["selector"], "cheats.selector")
	options := mustArray(t, selector["options"], "cheats.selector.options")
	if len(options) != 3 {
		t.Fatalf("expected three ALTTP save slot options: %s", root.Body.String())
	}
	presets := mustArray(t, cheats["presets"], "cheats.presets")
	foundFullKit := false
	for _, preset := range presets {
		item := mustObject(t, preset, "preset")
		if mustString(t, item["id"], "preset.id") == "fullAdventureKit" {
			foundFullKit = true
			break
		}
	}
	if !foundFullKit {
		t.Fatalf("expected fullAdventureKit preset in response: %s", root.Body.String())
	}
}

func TestSaveCheatApplyALTTPUpdatesCurrentVersion(t *testing.T) {
	h := newContractHarness(t)
	saveID := seedALTTPSave(t, h, "/saves")
	applyReq := fmt.Sprintf(`{"saveId":%q,"editorId":"alttp-sram","slotId":"2","presetIds":["maxResources"],"updates":{"sword":"golden"}}`, saveID)
	rr := h.json(http.MethodPost, "/save/cheats/apply", strings.NewReader(applyReq))
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

	latest := h.request(http.MethodGet, "/save/latest?romSha1=alttp-rom&slotName=default", nil)
	assertStatus(t, latest, http.StatusOK)
	latestBody := decodeJSONMap(t, latest.Body)
	if mustString(t, latestBody["id"], "id") != newID {
		t.Fatalf("expected cheat-applied save to become latest: %s", latest.Body.String())
	}

	cheatState := h.request(http.MethodGet, "/save/cheats?saveId="+newID, nil)
	assertStatus(t, cheatState, http.StatusOK)
	cheatBody := decodeJSONMap(t, cheatState.Body)
	slotValues := mustObject(t, mustObject(t, cheatBody["cheats"], "cheats")["slotValues"], "cheats.slotValues")
	slot2 := mustObject(t, slotValues["2"], "slotValues.2")
	if mustString(t, slot2["sword"], "slot2.sword") != "golden" {
		t.Fatalf("expected golden sword after apply: %s", cheatState.Body.String())
	}
	if mustNumber(t, slot2["rupees"], "slot2.rupees") != 999 {
		t.Fatalf("expected max rupees after preset: %s", cheatState.Body.String())
	}
}

func mustLoadALTTPCheatPack(t *testing.T) cheatPack {
	t.Helper()
	root, err := findCuratedCheatPackRoot()
	if err != nil {
		t.Fatalf("find curated cheat pack root: %v", err)
	}
	pack, err := loadCheatPackFile(filepath.Join(root, "snes", "the-legend-of-zelda-a-link-to-the-past.yaml"))
	if err != nil {
		t.Fatalf("load alttp cheat pack: %v", err)
	}
	return pack
}

func buildALTTPFixturePayload() []byte {
	payload := make([]byte, alttpSRAMSize)
	block := buildALTTPFixtureBlock()
	copy(payload[alttpSlotOffset(1):alttpSlotOffset(1)+alttpSlotSize], block)
	copy(payload[alttpMirrorSlotOffset(1):alttpMirrorSlotOffset(1)+alttpSlotSize], block)
	return payload
}

func buildALTTPFixtureBlock() []byte {
	block := make([]byte, alttpSlotSize)
	block[alttpUSMarkerOffset] = alttpMarkerLow
	block[alttpUSMarkerOffset+1] = alttpMarkerHigh
	block[alttpItemBaseOffset+0x02] = 1
	block[alttpItemBaseOffset+0x19] = 2
	block[alttpItemBaseOffset+0x2b] = 1
	block[alttpItemBaseOffset+0x2c] = 0x18
	block[alttpItemBaseOffset+0x2d] = 0x18
	block[alttpItemBaseOffset+0x2e] = 64
	block[alttpItemBaseOffset+0x34] = 0x01
	block[alttpItemBaseOffset+0x3a] = 0x02
	binary.LittleEndian.PutUint16(block[alttpItemBaseOffset+0x20:alttpItemBaseOffset+0x22], 100)
	binary.LittleEndian.PutUint16(block[alttpItemBaseOffset+0x22:alttpItemBaseOffset+0x24], 100)
	alttpWriteChecksum(block)
	return block
}

func seedALTTPSave(t *testing.T, h *contractHarness, path string) string {
	t.Helper()
	helperKey := createHelperAppPassword(t, h, "", "alttp-helper")
	body := uploadSave(t, h, path, map[string]string{
		"app_password":   helperKey,
		"device_type":    "desktop",
		"fingerprint":    "alttp-helper",
		"rom_sha1":       "alttp-rom",
		"runtimeProfile": "snes/snes9x",
		"slotName":       "default",
		"system":         "snes",
	}, "The Legend of Zelda - A Link to the Past (USA).srm", buildALTTPFixturePayload())
	return mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
}

func sortedStringArray(t *testing.T, value any) []string {
	t.Helper()
	items, err := normalizeCheatStringArray(value)
	if err != nil {
		t.Fatalf("expected string array, got %#v", value)
	}
	sort.Strings(items)
	return items
}

func bytesEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
