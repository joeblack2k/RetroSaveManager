package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAgentAPICheatAdaptersAndBuiltinPacks(t *testing.T) {
	h := newContractHarness(t)

	adapters := h.request(http.MethodGet, "/api/cheats/adapters", nil)
	assertStatus(t, adapters, http.StatusOK)
	assertJSONContentType(t, adapters)

	adapterBody := decodeJSONMap(t, adapters.Body)
	if !mustBool(t, adapterBody["success"], "success") {
		t.Fatalf("expected success body=%s", adapters.Body.String())
	}
	adapterItems := mustArray(t, adapterBody["adapters"], "adapters")
	if len(adapterItems) == 0 {
		t.Fatalf("expected adapters body=%s", adapters.Body.String())
	}
	foundSM64 := false
	for _, item := range adapterItems {
		adapter := mustObject(t, item, "adapter")
		if mustString(t, adapter["id"], "adapter.id") == "sm64-eeprom" {
			foundSM64 = true
			break
		}
	}
	if !foundSM64 {
		t.Fatalf("expected sm64-eeprom in adapter catalog body=%s", adapters.Body.String())
	}

	packs := h.request(http.MethodGet, "/api/cheats/packs", nil)
	assertStatus(t, packs, http.StatusOK)
	assertJSONContentType(t, packs)

	packBody := decodeJSONMap(t, packs.Body)
	if !mustBool(t, packBody["success"], "success") {
		t.Fatalf("expected success body=%s", packs.Body.String())
	}
	packItems := mustArray(t, packBody["packs"], "packs")
	if len(packItems) == 0 {
		t.Fatalf("expected builtin packs body=%s", packs.Body.String())
	}
	foundBuiltin := false
	for _, item := range packItems {
		pack := mustObject(t, item, "pack")
		manifest := mustObject(t, pack["manifest"], "pack.manifest")
		if mustString(t, manifest["packId"], "pack.manifest.packId") != "n64--super-mario-64" {
			continue
		}
		foundBuiltin = true
		if !mustBool(t, pack["builtin"], "pack.builtin") {
			t.Fatalf("expected builtin pack body=%s", packs.Body.String())
		}
	}
	if !foundBuiltin {
		t.Fatalf("expected n64--super-mario-64 builtin pack body=%s", packs.Body.String())
	}
}

func TestAgentAPICheatPackCreateValidationAndLifecycle(t *testing.T) {
	h := newContractHarness(t)

	unknownAdapter := postCheatPack(t, h, cheatPackCreateRequest{
		YAML: `
schemaVersion: 1
adapterId: no-such-adapter
gameId: n64/super-mario-64
systemSlug: n64
title: Invalid Adapter
sections:
  - id: abilities
    title: Abilities
    fields:
      - id: runtimeWingCap
        ref: haveWingCap
        label: Runtime Wing Cap
        type: boolean
`,
	})
	assertStatus(t, unknownAdapter, http.StatusUnprocessableEntity)
	unknownAdapterBody := decodeJSONMap(t, unknownAdapter.Body)
	if !strings.Contains(strings.ToLower(mustString(t, unknownAdapterBody["message"], "message")), "unknown adapterid") {
		t.Fatalf("unexpected adapter validation message: %s", unknownAdapter.Body.String())
	}

	unknownField := postCheatPack(t, h, cheatPackCreateRequest{
		YAML: `
schemaVersion: 1
adapterId: sm64-eeprom
gameId: n64/super-mario-64
systemSlug: n64
title: Invalid Field Ref
sections:
  - id: abilities
    title: Abilities
    fields:
      - id: runtimeWingCap
        ref: doesNotExist
        label: Runtime Wing Cap
        type: boolean
`,
	})
	assertStatus(t, unknownField, http.StatusUnprocessableEntity)
	unknownFieldBody := decodeJSONMap(t, unknownField.Body)
	if !strings.Contains(strings.ToLower(mustString(t, unknownFieldBody["message"], "message")), "unknown field ref") {
		t.Fatalf("unexpected field validation message: %s", unknownField.Body.String())
	}

	validYAML := `
packId: sm64-runtime-ui
schemaVersion: 1
adapterId: sm64-eeprom
gameId: n64/super-mario-64
systemSlug: n64
title: SM64 Runtime UI
match:
  titleAliases:
    - Super Mario 64
sections:
  - id: runtime-abilities
    title: Runtime Abilities
    fields:
      - id: runtimeWingCap
        ref: haveWingCap
        label: Runtime Wing Cap
        type: boolean
presets:
  - id: runtimePreset
    label: Runtime Preset
    updates:
      runtimeWingCap: true
`
	created := postCheatPack(t, h, cheatPackCreateRequest{
		YAML:        validYAML,
		Source:      cheatPackSourceWorker,
		PublishedBy: "codex-test",
		Notes:       "runtime lifecycle test",
	})
	assertStatus(t, created, http.StatusCreated)
	assertJSONContentType(t, created)

	createdBody := decodeJSONMap(t, created.Body)
	if !mustBool(t, createdBody["success"], "success") {
		t.Fatalf("expected success body=%s", created.Body.String())
	}
	createdPack := mustObject(t, createdBody["pack"], "pack")
	createdManifest := mustObject(t, createdPack["manifest"], "pack.manifest")
	if mustString(t, createdManifest["packId"], "pack.manifest.packId") != "sm64-runtime-ui" {
		t.Fatalf("unexpected created packId body=%s", created.Body.String())
	}
	if mustString(t, createdManifest["status"], "pack.manifest.status") != cheatPackStatusActive {
		t.Fatalf("expected active status body=%s", created.Body.String())
	}

	packDir, err := safeJoinUnderRoot(h.app.saveStore.root, "_rsm", "cheats", "packs", "sm64-runtime-ui")
	if err != nil {
		t.Fatalf("runtime pack dir: %v", err)
	}
	if _, err := os.Stat(packDir); err != nil {
		t.Fatalf("expected runtime pack directory %s: %v", packDir, err)
	}

	disabled := h.request(http.MethodPost, "/api/cheats/packs/sm64-runtime-ui/disable", nil)
	assertStatus(t, disabled, http.StatusOK)
	disabledBody := decodeJSONMap(t, disabled.Body)
	disabledPack := mustObject(t, disabledBody["pack"], "pack")
	disabledManifest := mustObject(t, disabledPack["manifest"], "pack.manifest")
	if mustString(t, disabledManifest["status"], "pack.manifest.status") != cheatPackStatusDisabled {
		t.Fatalf("expected disabled status body=%s", disabled.Body.String())
	}

	enabled := h.request(http.MethodPost, "/api/cheats/packs/sm64-runtime-ui/enable", nil)
	assertStatus(t, enabled, http.StatusOK)
	enabledBody := decodeJSONMap(t, enabled.Body)
	enabledPack := mustObject(t, enabledBody["pack"], "pack")
	enabledManifest := mustObject(t, enabledPack["manifest"], "pack.manifest")
	if mustString(t, enabledManifest["status"], "pack.manifest.status") != cheatPackStatusActive {
		t.Fatalf("expected active status body=%s", enabled.Body.String())
	}

	deleted := h.request(http.MethodDelete, "/api/cheats/packs/sm64-runtime-ui", nil)
	assertStatus(t, deleted, http.StatusOK)
	deletedBody := decodeJSONMap(t, deleted.Body)
	deletedPack := mustObject(t, deletedBody["pack"], "pack")
	deletedManifest := mustObject(t, deletedPack["manifest"], "pack.manifest")
	if mustString(t, deletedManifest["status"], "pack.manifest.status") != cheatPackStatusDeleted {
		t.Fatalf("expected deleted status body=%s", deleted.Body.String())
	}

	list := h.request(http.MethodGet, "/api/cheats/packs", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	packs := mustArray(t, listBody["packs"], "packs")
	foundDeleted := false
	for _, item := range packs {
		pack := mustObject(t, item, "pack")
		manifest := mustObject(t, pack["manifest"], "pack.manifest")
		if mustString(t, manifest["packId"], "pack.manifest.packId") != "sm64-runtime-ui" {
			continue
		}
		foundDeleted = true
		if mustString(t, manifest["status"], "pack.manifest.status") != cheatPackStatusDeleted {
			t.Fatalf("expected deleted pack in list body=%s", list.Body.String())
		}
		if mustBool(t, pack["supportsSaveUi"], "pack.supportsSaveUi") {
			t.Fatalf("deleted pack should not support save UI body=%s", list.Body.String())
		}
	}
	if !foundDeleted {
		t.Fatalf("expected deleted runtime pack in list body=%s", list.Body.String())
	}
}

func TestAgentAPIBuiltinCheatPackDeleteCreatesTombstone(t *testing.T) {
	h := newContractHarness(t)

	deleted := h.request(http.MethodDelete, "/api/cheats/packs/n64--super-mario-64", nil)
	assertStatus(t, deleted, http.StatusOK)
	assertJSONContentType(t, deleted)

	body := decodeJSONMap(t, deleted.Body)
	if !mustBool(t, body["success"], "success") {
		t.Fatalf("expected success body=%s", deleted.Body.String())
	}
	pack := mustObject(t, body["pack"], "pack")
	manifest := mustObject(t, pack["manifest"], "pack.manifest")
	if mustString(t, manifest["status"], "pack.manifest.status") != cheatPackStatusDeleted {
		t.Fatalf("expected deleted builtin tombstone body=%s", deleted.Body.String())
	}
	if !mustBool(t, pack["builtin"], "pack.builtin") {
		t.Fatalf("expected builtin flag body=%s", deleted.Body.String())
	}

	tombstonePath, err := safeJoinUnderRoot(h.app.saveStore.root, "_rsm", "cheats", "tombstones", "n64--super-mario-64.json")
	if err != nil {
		t.Fatalf("tombstone path: %v", err)
	}
	if _, err := os.Stat(tombstonePath); err != nil {
		t.Fatalf("expected tombstone file %s: %v", tombstonePath, err)
	}
}

func postCheatPack(t *testing.T, h *contractHarness, req cheatPackCreateRequest) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal cheat pack request: %v", err)
	}
	return h.json(http.MethodPost, "/api/cheats/packs", strings.NewReader(string(data)))
}
