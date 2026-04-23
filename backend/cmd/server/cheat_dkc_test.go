package main

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestDKCCheatEditorReadAndApply(t *testing.T) {
	pack := mustLoadDKCCheatPack(t)
	payload := buildDKCFixturePayload()
	editor := dkcSRAMCheatEditor{}

	state, err := editor.Read(pack, payload)
	if err != nil {
		t.Fatalf("read cheats: %v", err)
	}
	slot2 := state.SlotValues["2"]
	if slot2 == nil {
		t.Fatalf("expected slot 2 values")
	}
	if jungle, ok := slot2["jungleHijinxs"].(string); !ok || jungle != "donkey" {
		t.Fatalf("expected Jungle Hijinxs to be donkey, got %#v", slot2["jungleHijinxs"])
	}
	if necky, ok := slot2["neckysNuts"].(string); !ok || necky != "none" {
		t.Fatalf("expected Necky's Nuts to be none, got %#v", slot2["neckysNuts"])
	}

	updated, changed, err := editor.Apply(pack, payload, "2", map[string]any{
		"jungleHijinxs":    "diddy",
		"neckysNuts":       "donkey",
		"gangPlankGalleon": "donkey",
	})
	if err != nil {
		t.Fatalf("apply cheats: %v", err)
	}
	if _, ok := changed["neckysNuts"]; !ok {
		t.Fatalf("expected changed map to include neckysNuts")
	}
	parsed, err := parseDKCSRAM(updated)
	if err != nil {
		t.Fatalf("parse updated payload: %v", err)
	}
	block := parsed.Slots[1]
	if block == nil {
		t.Fatalf("expected updated slot 2 block")
	}
	if progressionID, _ := dkcProgressionID(block[dkcProgressionSpecs["jungleHijinxs"].Offset]); progressionID != "diddy" {
		t.Fatalf("expected Jungle Hijinxs to become diddy, got %q", progressionID)
	}
	if progressionID, _ := dkcProgressionID(block[dkcProgressionSpecs["neckysNuts"].Offset]); progressionID != "donkey" {
		t.Fatalf("expected Necky's Nuts to become donkey, got %q", progressionID)
	}
	if block[dkcProgressionSpecs["neckysNuts"].MirrorOffset] != 0x01 {
		t.Fatalf("expected Necky's Nuts mirror byte to become 0x01, got 0x%02x", block[dkcProgressionSpecs["neckysNuts"].MirrorOffset])
	}
	if block[0] <= 3 {
		t.Fatalf("expected completion rate to increase, got %d", block[0])
	}
	if !dkcVerifyBlock(block) {
		t.Fatalf("expected rebuilt DKC block checksum to be valid")
	}
}

func TestSaveCheatEndpointsExposeDKCPack(t *testing.T) {
	h := newContractHarness(t)
	saveID := seedDKCSave(t, h, "/saves")

	root := h.request(http.MethodGet, "/save/cheats?saveId="+saveID, nil)
	v1 := h.request(http.MethodGet, "/v1/save/cheats?saveId="+saveID, nil)
	assertStatus(t, root, http.StatusOK)
	assertStatus(t, v1, http.StatusOK)
	assertJSONContentType(t, root)
	assertJSONContentType(t, v1)
	assertEqualJSONValue(t, normalizeForGolden(decodeJSONMap(t, root.Body)), normalizeForGolden(decodeJSONMap(t, v1.Body)), "dkc save cheats")

	body := decodeJSONMap(t, root.Body)
	cheats := mustObject(t, body["cheats"], "cheats")
	if !mustBool(t, cheats["supported"], "cheats.supported") {
		t.Fatalf("expected supported cheats payload")
	}
	if mustString(t, cheats["editorId"], "cheats.editorId") != "dkc-sram" {
		t.Fatalf("expected dkc-sram editor: %s", root.Body.String())
	}
	selector := mustObject(t, cheats["selector"], "cheats.selector")
	options := mustArray(t, selector["options"], "cheats.selector.options")
	if len(options) != 3 {
		t.Fatalf("expected three DKC save slot options: %s", root.Body.String())
	}
	presets := mustArray(t, cheats["presets"], "cheats.presets")
	foundUnlockAllWorlds := false
	for _, preset := range presets {
		item := mustObject(t, preset, "preset")
		if mustString(t, item["id"], "preset.id") == "unlockAllWorlds" {
			foundUnlockAllWorlds = true
			break
		}
	}
	if !foundUnlockAllWorlds {
		t.Fatalf("expected unlockAllWorlds preset in response: %s", root.Body.String())
	}
}

func TestSaveCheatApplyDKCCreatesNewCurrentVersionOnRootAndV1(t *testing.T) {
	for _, prefix := range []string{"", "/v1"} {
		t.Run(firstNonEmpty(prefix, "root"), func(t *testing.T) {
			h := newContractHarness(t)
			saveID := seedDKCSave(t, h, prefix+"/saves")
			applyReq := fmt.Sprintf(`{"saveId":%q,"editorId":"dkc-sram","slotId":"2","presetIds":["unlockAllWorlds"],"updates":{"gangPlankGalleon":"donkey"}}`, saveID)
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

			latest := h.request(http.MethodGet, prefix+"/save/latest?romSha1=dkc-rom&slotName=default", nil)
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
			for _, fieldID := range []string{"veryGnawtysLair", "neckysNuts", "reallyGnawtyRampage", "bossDumbDrum", "neckysRevenge", "bumbleBRumble", "gangPlankGalleon"} {
				if mustString(t, slot2[fieldID], fieldID) != "donkey" {
					t.Fatalf("expected %s to be donkey after preset/apply: %s", fieldID, cheatState.Body.String())
				}
			}
		})
	}
}

func mustLoadDKCCheatPack(t *testing.T) cheatPack {
	t.Helper()
	root, err := findCuratedCheatPackRoot()
	if err != nil {
		t.Fatalf("find curated cheat pack root: %v", err)
	}
	pack, err := loadCheatPackFile(filepath.Join(root, "snes", "donkey-kong-country.yaml"))
	if err != nil {
		t.Fatalf("load dkc cheat pack: %v", err)
	}
	return pack
}

func buildDKCFixturePayload() []byte {
	payload := make([]byte, dkcSRAMSize)
	block := buildDKCFixtureBlock(map[string]string{
		"jungleHijinxs":   "donkey",
		"ropeyRampage":    "donkey",
		"veryGnawtysLair": "donkey",
	})
	copy(payload[dkcSlotOffset(1):dkcSlotOffset(1)+dkcPrimaryBlockSize], block)
	return payload
}

func buildDKCFixtureBlock(progressions map[string]string) []byte {
	block := make([]byte, dkcPrimaryBlockSize)
	binary.LittleEndian.PutUint32(block[dkcMagicOffset:dkcMagicOffset+4], dkcValidMagic)
	block[0x10] = 0x00
	block[0x15] = 0x16
	for fieldID, progressionID := range progressions {
		spec := dkcProgressionSpecs[fieldID]
		value, ok := dkcProgressionValue(progressionID)
		if !ok {
			panic("unsupported DKC fixture progression")
		}
		block[spec.Offset] = value
		if spec.MirrorOffset > 0 {
			block[spec.MirrorOffset] = value
		}
	}
	dkcRecomputeCompletion(block)
	dkcWriteChecksum(block)
	return block
}

func seedDKCSave(t *testing.T, h *contractHarness, path string) string {
	t.Helper()
	body := uploadSave(t, h, path, map[string]string{
		"rom_sha1": "dkc-rom",
		"slotName": "default",
		"system":   "snes",
	}, "Donkey Kong Country (USA).sa1", buildDKCFixturePayload())
	return mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
}
