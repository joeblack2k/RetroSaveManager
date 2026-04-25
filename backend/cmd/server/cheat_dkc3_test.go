package main

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestDKC3CheatEditorReadAndApply(t *testing.T) {
	pack := mustLoadDKC3CheatPack(t)
	payload := buildDKC3FixturePayloadWithCounters(1, map[string]int{
		"bearCoins":   12,
		"bonusCoins":  4,
		"bananaBirds": 2,
		"dkCoins":     3,
	})
	editor := dkc3SRAMCheatEditor{}

	state, err := editor.Read(pack, payload)
	if err != nil {
		t.Fatalf("read cheats: %v", err)
	}
	slot2 := state.SlotValues["2"]
	if slot2 == nil {
		t.Fatalf("expected slot 2 values")
	}
	if got := mustIntCheatValue(t, slot2["bearCoins"], "bearCoins"); got != 12 {
		t.Fatalf("expected 12 bear coins, got %d", got)
	}
	if got := mustIntCheatValue(t, slot2["dkCoins"], "dkCoins"); got != 3 {
		t.Fatalf("expected 3 DK coins, got %d", got)
	}

	updated, changed, err := editor.Apply(pack, payload, "2", map[string]any{
		"bearCoins":   99,
		"bonusCoins":  85,
		"bananaBirds": 15,
		"dkCoins":     41,
	})
	if err != nil {
		t.Fatalf("apply cheats: %v", err)
	}
	if _, ok := changed["bonusCoins"]; !ok {
		t.Fatalf("expected changed map to include bonusCoins")
	}
	parsed, err := parseDKC3SRAM(updated)
	if err != nil {
		t.Fatalf("parse updated payload: %v", err)
	}
	slot := parsed.Slots[1]
	if slot == nil {
		t.Fatalf("expected updated slot 2 block")
	}
	for fieldID, want := range map[string]int{
		"bearCoins":   99,
		"bonusCoins":  85,
		"bananaBirds": 15,
		"dkCoins":     41,
	} {
		spec := dkc3CounterSpecs[fieldID]
		got := int(binary.LittleEndian.Uint16(slot.Block[slot.DataOffset+spec.Offset : slot.DataOffset+spec.Offset+2]))
		if got != want {
			t.Fatalf("expected %s=%d, got %d", fieldID, want, got)
		}
	}
	if _, ok := dkc3VerifyBlock(1, slot.Block); !ok {
		t.Fatalf("expected rebuilt DKC3 slot checksum to be valid")
	}
}

func TestSaveCheatEndpointsExposeDKC3Pack(t *testing.T) {
	h := newContractHarness(t)
	saveID := seedDKC3Save(t, h, "/saves")

	root := h.request(http.MethodGet, "/save/cheats?saveId="+saveID, nil)
	v1 := h.request(http.MethodGet, "/v1/save/cheats?saveId="+saveID, nil)
	assertStatus(t, root, http.StatusOK)
	assertStatus(t, v1, http.StatusOK)
	assertJSONContentType(t, root)
	assertJSONContentType(t, v1)
	assertEqualJSONValue(t, normalizeForGolden(decodeJSONMap(t, root.Body)), normalizeForGolden(decodeJSONMap(t, v1.Body)), "dkc3 save cheats")

	body := decodeJSONMap(t, root.Body)
	cheats := mustObject(t, body["cheats"], "cheats")
	if !mustBool(t, cheats["supported"], "cheats.supported") {
		t.Fatalf("expected supported cheats payload")
	}
	if mustString(t, cheats["editorId"], "cheats.editorId") != "dkc3-sram" {
		t.Fatalf("expected dkc3-sram editor: %s", root.Body.String())
	}
	selector := mustObject(t, cheats["selector"], "cheats.selector")
	options := mustArray(t, selector["options"], "cheats.selector.options")
	if len(options) != 3 {
		t.Fatalf("expected three DKC3 save slot options: %s", root.Body.String())
	}
	presets := mustArray(t, cheats["presets"], "cheats.presets")
	foundMaxCollectibles := false
	for _, preset := range presets {
		item := mustObject(t, preset, "preset")
		if mustString(t, item["id"], "preset.id") == "maxCollectibles" {
			foundMaxCollectibles = true
			break
		}
	}
	if !foundMaxCollectibles {
		t.Fatalf("expected maxCollectibles preset in response: %s", root.Body.String())
	}
}

func TestSaveCheatApplyDKC3CreatesNewCurrentVersionOnRootAndV1(t *testing.T) {
	for _, prefix := range []string{"", "/v1"} {
		t.Run(firstNonEmpty(prefix, "root"), func(t *testing.T) {
			h := newContractHarness(t)
			saveID := seedDKC3Save(t, h, prefix+"/saves")
			applyReq := fmt.Sprintf(`{"saveId":%q,"editorId":"dkc3-sram","slotId":"2","presetIds":["maxCollectibles"],"updates":{"bearCoins":42}}`, saveID)
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

			latest := h.request(http.MethodGet, prefix+"/save/latest?romSha1=dkc3-rom&slotName=default", nil)
			assertStatus(t, latest, http.StatusOK)
			latestBody := decodeJSONMap(t, latest.Body)
			if mustString(t, latestBody["id"], "id") != newID {
				t.Fatalf("expected cheat-applied save to become latest: %s", latest.Body.String())
			}

			cheatState := h.request(http.MethodGet, prefix+"/save/cheats?saveId="+newID, nil)
			assertStatus(t, cheatState, http.StatusOK)
			cheatBody := decodeJSONMap(t, cheatState.Body)
			slotValues := mustObject(t, mustObject(t, cheatBody["cheats"], "cheats")["slotValues"], "cheats.slotValues")
			slot2 := mustObject(t, slotValues["2"], "slotValues.2")
			for fieldID, want := range map[string]float64{
				"bearCoins":   42,
				"bonusCoins":  85,
				"bananaBirds": 15,
				"dkCoins":     41,
			} {
				if mustNumber(t, slot2[fieldID], fieldID) != want {
					t.Fatalf("expected %s=%v after preset/apply: %s", fieldID, want, cheatState.Body.String())
				}
			}
		})
	}
}

func mustLoadDKC3CheatPack(t *testing.T) cheatPack {
	t.Helper()
	root, err := findCuratedCheatPackRoot()
	if err != nil {
		t.Fatalf("find curated cheat pack root: %v", err)
	}
	pack, err := loadCheatPackFile(filepath.Join(root, "snes", "donkey-kong-country-3.yaml"))
	if err != nil {
		t.Fatalf("load dkc3 cheat pack: %v", err)
	}
	return pack
}

func buildDKC3FixturePayloadWithCounters(slotIndex int, counters map[string]int) []byte {
	payload := make([]byte, dkc3SRAMSize)
	for _, signature := range snesDKC3Signatures {
		copy(payload[signature.Offset:], []byte(signature.Value))
	}
	block := buildDKC3FixtureBlock(slotIndex, counters)
	copy(payload[dkc3SlotOffset(slotIndex):dkc3SlotOffset(slotIndex)+dkc3SlotBlockSize], block)
	return payload
}

func buildDKC3FixtureBlock(slotIndex int, counters map[string]int) []byte {
	block := make([]byte, dkc3SlotBlockSize)
	block[dkc3MarkerOffset] = 0x52
	block[dkc3MarkerOffset+1] = byte(slotIndex)
	copy(block[dkc3HeaderSize:], []byte("DIXI"))
	block[dkc3HeaderSize+4] = 0xc5
	block[dkc3HeaderSize+0x0e] = 0x17
	for fieldID, value := range counters {
		spec := dkc3CounterSpecs[fieldID]
		binary.LittleEndian.PutUint16(block[dkc3HeaderSize+spec.Offset:dkc3HeaderSize+spec.Offset+2], uint16(value))
	}
	dkc3WriteChecksum(block)
	return block
}

func seedDKC3Save(t *testing.T, h *contractHarness, path string) string {
	t.Helper()
	body := uploadSave(t, h, path, map[string]string{
		"rom_sha1": "dkc3-rom",
		"slotName": "default",
		"system":   "snes",
	}, "Donkey Kong Country 3 - Dixie Kong's Double Trouble! (USA).sa1", buildDKC3FixturePayloadWithCounters(1, map[string]int{
		"bearCoins":   12,
		"bonusCoins":  4,
		"bananaBirds": 2,
		"dkCoins":     3,
	}))
	return mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
}

func mustIntCheatValue(t *testing.T, value any, field string) int {
	t.Helper()
	intValue, ok := value.(int)
	if !ok {
		t.Fatalf("expected %s to be int, got %#v", field, value)
	}
	return intValue
}
