package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSM64CheatEditorReadAndApply(t *testing.T) {
	pack := mustLoadSM64CheatPack(t)
	payload := buildSM64FixturePayload()
	editor := sm64EEPROMCheatEditor{}

	state, err := editor.Read(pack, payload)
	if err != nil {
		t.Fatalf("read cheats: %v", err)
	}
	slotA := state.SlotValues["A"]
	if slotA == nil {
		t.Fatalf("expected slot A values")
	}
	if wingCap, ok := slotA["haveWingCap"].(bool); !ok || wingCap {
		t.Fatalf("expected wing cap to be false before apply: %#v", slotA["haveWingCap"])
	}
	if bobScore, ok := slotA["bob100Coin"].(int); !ok || bobScore != 55 {
		t.Fatalf("expected bob 100-coin score 55, got %#v", slotA["bob100Coin"])
	}

	updated, changed, err := editor.Apply(pack, payload, "A", map[string]any{
		"haveWingCap": true,
		"bob100Coin": 120,
		"bobStars":   []string{"bit1", "bit2", "bit3"},
	})
	if err != nil {
		t.Fatalf("apply cheats: %v", err)
	}
	if _, ok := changed["haveWingCap"]; !ok {
		t.Fatalf("expected changed map to include haveWingCap")
	}
	parsed, err := parseSM64EEPROM(updated)
	if err != nil {
		t.Fatalf("parse updated payload: %v", err)
	}
	if parsed.Files[0].Flags&sm64FlagHaveWingCap == 0 {
		t.Fatalf("expected wing cap flag to be set after apply")
	}
	if parsed.Files[0].CourseCoinScores[0] != 120 {
		t.Fatalf("expected bob 100-coin score to become 120, got %d", parsed.Files[0].CourseCoinScores[0])
	}
	if parsed.Files[0].CourseStars[0]&0x7f != 0x07 {
		t.Fatalf("expected bob star bits 0x07, got 0x%02x", parsed.Files[0].CourseStars[0]&0x7f)
	}
}

func TestSaveCheatEndpointsExposeSchemaAndLocalOverride(t *testing.T) {
	h := newContractHarness(t)
	saveID := seedSM64Save(t, h, "/saves")
	record, ok := h.app.findSaveRecordByID(saveID)
	if !ok {
		t.Fatalf("expected uploaded save record")
	}
	overridePath, err := safeJoinUnderRoot(h.app.saveStore.root, record.SystemPath, record.GamePath, "_rsm", "cheats.local.yaml")
	if err != nil {
		t.Fatalf("override path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(overridePath), 0o755); err != nil {
		t.Fatalf("mkdir override dir: %v", err)
	}
	override := `gameId: n64/super-mario-64
systemSlug: n64
editorId: sm64-eeprom
presets:
  - id: localBoost
    label: Local Boost
    updates:
      haveMetalCap: true
`
	if err := os.WriteFile(overridePath, []byte(override), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	root := h.request(http.MethodGet, "/save/cheats?saveId="+saveID, nil)
	v1 := h.request(http.MethodGet, "/v1/save/cheats?saveId="+saveID, nil)
	assertStatus(t, root, http.StatusOK)
	assertStatus(t, v1, http.StatusOK)
	assertJSONContentType(t, root)
	assertJSONContentType(t, v1)
	assertEqualJSONValue(t, normalizeForGolden(decodeJSONMap(t, root.Body)), normalizeForGolden(decodeJSONMap(t, v1.Body)), "save cheats")

	body := decodeJSONMap(t, root.Body)
	cheats := mustObject(t, body["cheats"], "cheats")
	if !mustBool(t, cheats["supported"], "cheats.supported") {
		t.Fatalf("expected supported cheats payload")
	}
	presets := mustArray(t, cheats["presets"], "cheats.presets")
	foundLocal := false
	for _, preset := range presets {
		item := mustObject(t, preset, "preset")
		if mustString(t, item["id"], "preset.id") == "localBoost" {
			foundLocal = true
			break
		}
	}
	if !foundLocal {
		t.Fatalf("expected local override preset in response: %s", root.Body.String())
	}
}

func TestSaveCheatApplyCreatesNewCurrentVersionOnRootAndV1(t *testing.T) {
	for _, prefix := range []string{"", "/v1"} {
		t.Run(firstNonEmpty(prefix, "root"), func(t *testing.T) {
			h := newContractHarness(t)
			saveID := seedSM64Save(t, h, prefix+"/saves")
			applyReq := fmt.Sprintf(`{"saveId":%q,"editorId":"sm64-eeprom","slotId":"A","updates":{"haveWingCap":true,"bob100Coin":99}}`, saveID)
			rr := h.json(http.MethodPost, prefix+"/save/cheats/apply", strings.NewReader(applyReq))
			assertStatus(t, rr, http.StatusOK)
			assertJSONContentType(t, rr)
			body := decodeJSONMap(t, rr.Body)
			if !mustBool(t, body["success"], "success") {
				t.Fatalf("expected success=true")
			}
			if mustString(t, body["sourceSaveId"], "sourceSaveId") != saveID {
				t.Fatalf("unexpected sourceSaveId: %s", rr.Body.String())
			}
			save := mustObject(t, body["save"], "save")
			newID := mustString(t, save["id"], "save.id")
			if mustNumber(t, save["version"], "save.version") != 2 {
				t.Fatalf("expected new version 2, got %s", rr.Body.String())
			}

			latest := h.request(http.MethodGet, prefix+"/save/latest?romSha1=sm64-rom&slotName=default", nil)
			assertStatus(t, latest, http.StatusOK)
			latestBody := decodeJSONMap(t, latest.Body)
			if mustString(t, latestBody["id"], "id") != newID {
				t.Fatalf("expected cheat-applied save to become latest: %s", latest.Body.String())
			}

			list := h.request(http.MethodGet, prefix+"/saves?limit=20&offset=0", nil)
			assertStatus(t, list, http.StatusOK)
			listBody := decodeJSONMap(t, list.Body)
			saves := mustArray(t, listBody["saves"], "saves")
			foundCheatCap := false
			for _, entry := range saves {
				item := mustObject(t, entry, "save")
				if mustString(t, item["id"], "save.id") != newID {
					continue
				}
				cheatCap := mustObject(t, item["cheats"], "save.cheats")
				if mustBool(t, cheatCap["supported"], "save.cheats.supported") {
					foundCheatCap = true
				}
			}
			if !foundCheatCap {
				t.Fatalf("expected list summary to expose cheat capability: %s", list.Body.String())
			}
		})
	}
}

func mustLoadSM64CheatPack(t *testing.T) cheatPack {
	t.Helper()
	root, err := findCuratedCheatPackRoot()
	if err != nil {
		t.Fatalf("find curated cheat pack root: %v", err)
	}
	pack, err := loadCheatPackFile(filepath.Join(root, "n64", "super-mario-64.yaml"))
	if err != nil {
		t.Fatalf("load sm64 cheat pack: %v", err)
	}
	return pack
}

func buildSM64FixturePayload() []byte {
	payload := make([]byte, sm64EEPROMSize)
	files := [4]sm64ParsedFile{
		{
			Flags: sm64FlagFileExists | sm64FlagHaveKey1,
		},
		{
			Flags: sm64FlagFileExists | sm64FlagHaveMetalCap,
		},
		{
			Flags: sm64FlagFileExists,
		},
		{
			Flags: sm64FlagFileExists,
		},
	}
	files[0].CourseStars[0] = 0x03
	files[0].CourseCoinScores[0] = 55
	files[1].CourseStars[1] = 0x81
	files[1].CourseCoinScores[1] = 88
	for index, file := range files {
		block := sm64BuildSaveFileBlock(file)
		offset := index * sm64FileStride
		copy(payload[offset:offset+sm64SaveFileSize], block)
		copy(payload[offset+sm64SaveFileSize:offset+sm64FileStride], block)
	}
	return payload
}

func seedSM64Save(t *testing.T, h *contractHarness, path string) string {
	t.Helper()
	body := uploadSave(t, h, path, map[string]string{
		"rom_sha1": "sm64-rom",
		"slotName": "default",
		"system":   "n64",
	}, "Super Mario 64 (USA).eep", buildSM64FixturePayload())
	return mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")
}
