package main

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOOTCheatEditorReadAndApply(t *testing.T) {
	pack := mustLoadOOTCheatPack(t)
	payload := buildOOTCheatFixturePayload()
	editor := ootSRAMCheatEditor{}

	state, err := editor.Read(pack, payload)
	if err != nil {
		t.Fatalf("read cheats: %v", err)
	}
	slotA := state.SlotValues["A"]
	if slotA == nil {
		t.Fatalf("expected slot A values")
	}
	if rupees, ok := slotA["rupees"].(int); !ok || rupees != 123 {
		t.Fatalf("expected 123 rupees before apply, got %#v", slotA["rupees"])
	}
	if magicLevel, ok := slotA["magicLevel"].(string); !ok || magicLevel != "normal" {
		t.Fatalf("expected normal magic before apply, got %#v", slotA["magicLevel"])
	}

	updated, changed, err := editor.Apply(pack, payload, "A", map[string]any{
		"rupees":          999,
		"maxHearts":       20,
		"currentHearts":   20,
		"magicLevel":      "double",
		"currentMagic":    96,
		"doubleDefense":   true,
		"swords":          []string{"kokiriSword", "masterSword", "biggoronSword"},
		"medallions":      []string{"forestMedallion", "fireMedallion", "waterMedallion"},
		"spiritualStones": []string{"kokiriEmerald", "goronRuby", "zoraSapphire"},
	})
	if err != nil {
		t.Fatalf("apply cheats: %v", err)
	}
	if _, ok := changed["rupees"]; !ok {
		t.Fatalf("expected changed map to include rupees")
	}

	normalized, wordSwapped, ok := ootNormalizePayload(updated)
	if !ok {
		t.Fatalf("expected updated payload to remain recognizable OOT SRAM")
	}
	if !wordSwapped {
		t.Fatalf("expected updated payload to preserve word-swapped byte order")
	}
	primary := normalized[ootSlotOffset(0, false) : ootSlotOffset(0, false)+ootSaveSize]
	backup := normalized[ootSlotOffset(0, true) : ootSlotOffset(0, true)+ootSaveSize]
	if !ootVerifySaveBlock(primary) || !ootVerifySaveBlock(backup) {
		t.Fatalf("expected primary and backup slots to pass checksum validation")
	}
	if string(primary[ootNewfOffset:ootNewfOffset+len(ootNewfMagic)]) != string(ootNewfMagic) {
		t.Fatalf("expected save magic to be preserved")
	}
	if got := binary.BigEndian.Uint16(primary[ootOffsetRupees : ootOffsetRupees+2]); got != 999 {
		t.Fatalf("expected 999 rupees, got %d", got)
	}
	if got := binary.BigEndian.Uint16(primary[ootOffsetHealthCapacity : ootOffsetHealthCapacity+2]); got != 20*ootHealthUnitsPerHeart {
		t.Fatalf("expected 20 max hearts, got raw health capacity 0x%04x", got)
	}
	if primary[ootOffsetMagicLevel] != 2 || primary[ootOffsetMagic] != ootMagicDoubleMeter {
		t.Fatalf("expected full double magic, got level=%d magic=%d", primary[ootOffsetMagicLevel], primary[ootOffsetMagic])
	}
	if got := binary.BigEndian.Uint16(primary[ootOffsetInventoryEquipment : ootOffsetInventoryEquipment+2]); got&0x0007 != 0x0007 {
		t.Fatalf("expected sword equipment bits 0x7, got 0x%04x", got)
	}
	if got := binary.BigEndian.Uint32(primary[ootOffsetQuestItems : ootOffsetQuestItems+4]); got&0x001C0007 != 0x001C0007 {
		t.Fatalf("expected selected quest bits, got 0x%08x", got)
	}
}

func TestSaveCheatApplyOOTPresetCreatesNewCurrentVersion(t *testing.T) {
	h := newContractHarness(t)
	saveID := seedOOTSave(t, h, "/saves")

	root := h.request(http.MethodGet, "/save/cheats?saveId="+saveID, nil)
	assertStatus(t, root, http.StatusOK)
	assertJSONContentType(t, root)
	cheats := mustObject(t, decodeJSONMap(t, root.Body)["cheats"], "cheats")
	if !mustBool(t, cheats["supported"], "cheats.supported") {
		t.Fatalf("expected OOT cheats to be supported: %s", root.Body.String())
	}
	if got := mustString(t, cheats["editorId"], "cheats.editorId"); got != "oot-sram" {
		t.Fatalf("expected oot-sram editor, got %q", got)
	}

	applyReq := fmt.Sprintf(`{"saveId":%q,"editorId":"oot-sram","slotId":"A","presetIds":["heroComplete"]}`, saveID)
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

	record, ok := h.app.findSaveRecordByID(newID)
	if !ok {
		t.Fatalf("expected new save record %s", newID)
	}
	payload, err := os.ReadFile(record.payloadPath)
	if err != nil {
		t.Fatalf("read updated payload: %v", err)
	}
	parsed, err := parseOOTSRAM(payload)
	if err != nil {
		t.Fatalf("parse updated OOT payload: %v", err)
	}
	slotA := parsed.Slots[0].Block
	if got := binary.BigEndian.Uint16(slotA[ootOffsetRupees : ootOffsetRupees+2]); got != 999 {
		t.Fatalf("expected heroComplete to set 999 rupees, got %d", got)
	}
	if got := binary.BigEndian.Uint16(slotA[ootOffsetGoldSkulltulaTokens : ootOffsetGoldSkulltulaTokens+2]); got != 100 {
		t.Fatalf("expected heroComplete to set 100 skulltula tokens, got %d", got)
	}
	if got := binary.BigEndian.Uint32(slotA[ootOffsetQuestItems : ootOffsetQuestItems+4]); got&0x00FFFFFF != 0x00FFFFFF {
		t.Fatalf("expected heroComplete to set quest item bits 0-23, got 0x%08x", got)
	}
}

func mustLoadOOTCheatPack(t *testing.T) cheatPack {
	t.Helper()
	root, err := findCuratedCheatPackRoot()
	if err != nil {
		t.Fatalf("find curated cheat pack root: %v", err)
	}
	pack, err := loadCheatPackFile(filepath.Join(root, "n64", "ocarina-of-time.yaml"))
	if err != nil {
		t.Fatalf("load oot cheat pack: %v", err)
	}
	return pack
}

func buildOOTCheatFixturePayload() []byte {
	payload := make([]byte, ootSramSize)
	copy(payload[:len(ootHeaderMagic)], ootHeaderMagic)
	slot := make([]byte, ootSaveSize)
	copy(slot[ootNewfOffset:ootNewfOffset+len(ootNewfMagic)], ootNewfMagic)
	binary.BigEndian.PutUint16(slot[ootOffsetDeaths:ootOffsetDeaths+2], 2)
	binary.BigEndian.PutUint16(slot[ootOffsetHealthCapacity:ootOffsetHealthCapacity+2], 3*ootHealthUnitsPerHeart)
	binary.BigEndian.PutUint16(slot[ootOffsetHealth:ootOffsetHealth+2], 2*ootHealthUnitsPerHeart)
	slot[ootOffsetMagicLevel] = 1
	slot[ootOffsetMagic] = ootMagicNormalMeter
	binary.BigEndian.PutUint16(slot[ootOffsetRupees:ootOffsetRupees+2], 123)
	slot[ootOffsetIsMagicAcquired] = 1
	binary.BigEndian.PutUint16(slot[ootOffsetInventoryEquipment:ootOffsetInventoryEquipment+2], 0x1111)
	binary.BigEndian.PutUint32(slot[ootOffsetQuestItems:ootOffsetQuestItems+4], 1<<18)
	binary.BigEndian.PutUint16(slot[ootOffsetGoldSkulltulaTokens:ootOffsetGoldSkulltulaTokens+2], 7)
	ootSetSaveBlockChecksum(slot)
	copy(payload[ootSlotOffset(0, false):ootSlotOffset(0, false)+ootSaveSize], slot)
	copy(payload[ootSlotOffset(0, true):ootSlotOffset(0, true)+ootSaveSize], slot)
	return n64Swap32Words(payload)
}

func seedOOTSave(t *testing.T, h *contractHarness, path string) string {
	t.Helper()
	body := uploadSave(t, h, path, map[string]string{
		"rom_sha1": "oot-rom",
		"slotName": "default",
		"system":   "n64",
	}, "Legend of Zelda, The - Ocarina of Time (USA).sra", buildOOTCheatFixturePayload())
	return mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
}
