package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAgentAPIIndexAndAlias(t *testing.T) {
	h := newContractHarness(t)

	for _, path := range []string{"/api", "/api/v1"} {
		rr := h.request(http.MethodGet, path, nil)
		assertStatus(t, rr, http.StatusOK)
		assertJSONContentType(t, rr)

		body := decodeJSONMap(t, rr.Body)
		if !mustBool(t, body["success"], "success") {
			t.Fatalf("expected success body=%s", rr.Body.String())
		}

		api := mustObject(t, body["api"], "api")
		if mustString(t, api["name"], "api.name") != "RetroSaveManager Agent API" {
			t.Fatalf("unexpected api name: %#v", api["name"])
		}
		if mustString(t, api["basePath"], "api.basePath") != path {
			t.Fatalf("unexpected basePath for %s: %#v", path, api["basePath"])
		}
	}
}

func TestAgentAPIOverviewAndSystems(t *testing.T) {
	h := newContractHarness(t)

	uploadSave(t, h, "/api/saves", map[string]string{
		"rom_sha1": "agent-overview-rom",
		"slotName": "default",
		"system":   "n64",
	}, "Mario Kart 64 (USA).eep", []byte("agent-overview-save"))

	overview := h.request(http.MethodGet, "/api/overview", nil)
	assertStatus(t, overview, http.StatusOK)
	assertJSONContentType(t, overview)

	body := decodeJSONMap(t, overview.Body)
	if !mustBool(t, body["success"], "success") {
		t.Fatalf("expected success body=%s", overview.Body.String())
	}
	root := mustObject(t, body["overview"], "overview")
	if mustString(t, root["authMode"], "overview.authMode") != "disabled" {
		t.Fatalf("unexpected authMode: %#v", root["authMode"])
	}

	stats := mustObject(t, root["stats"], "overview.stats")
	if mustNumber(t, stats["saveTracks"], "overview.stats.saveTracks") < 1 {
		t.Fatalf("expected at least one save track body=%s", overview.Body.String())
	}

	systems := mustObject(t, body["overview"], "overview")
	if systems["systems"] == nil {
		t.Fatalf("expected systems summary body=%s", overview.Body.String())
	}

	list := h.request(http.MethodGet, "/api/systems", nil)
	assertStatus(t, list, http.StatusOK)
	assertJSONContentType(t, list)

	listBody := decodeJSONMap(t, list.Body)
	items := mustArray(t, listBody["systems"], "systems")
	if len(items) == 0 {
		t.Fatalf("expected non-empty systems list")
	}
}

func TestAgentAPISaveAndRomFlow(t *testing.T) {
	h := newContractHarness(t)

	upload := uploadSave(t, h, "/api/saves", map[string]string{
		"rom_sha1": "agent-rom-sha1",
		"rom_md5":  "agent-rom-md5",
		"slotName": "default",
		"system":   "n64",
	}, "Star Fox 64 (USA).eep", []byte("agent-save-v1"))
	save := mustObject(t, upload["save"], "save")
	saveID := mustString(t, save["id"], "save.id")

	list := h.request(http.MethodGet, "/api/saves?systemSlug=n64&q=Star%20Fox", nil)
	assertStatus(t, list, http.StatusOK)
	assertJSONContentType(t, list)

	listBody := decodeJSONMap(t, list.Body)
	saves := mustArray(t, listBody["saves"], "saves")
	if len(saves) == 0 {
		t.Fatalf("expected at least one save body=%s", list.Body.String())
	}

	first := mustObject(t, saves[0], "saves[0]")
	saveEnvelope := mustObject(t, first["save"], "saves[0].save")
	if mustString(t, saveEnvelope["id"], "saves[0].save.id") != saveID {
		t.Fatalf("expected save id %s body=%s", saveID, list.Body.String())
	}
	actions := mustObject(t, first["actions"], "saves[0].actions")
	if !strings.HasPrefix(mustString(t, actions["detail"], "saves[0].actions.detail"), "/api/saves/") {
		t.Fatalf("expected detail action path body=%s", list.Body.String())
	}

	detail := h.request(http.MethodGet, "/api/saves/"+url.PathEscape(saveID), nil)
	assertStatus(t, detail, http.StatusOK)
	assertJSONContentType(t, detail)
	detailBody := decodeJSONMap(t, detail.Body)
	versions := mustArray(t, detailBody["versions"], "versions")
	if len(versions) == 0 {
		t.Fatalf("expected version history body=%s", detail.Body.String())
	}

	roms := h.request(http.MethodGet, "/api/roms?systemSlug=n64", nil)
	assertStatus(t, roms, http.StatusOK)
	assertJSONContentType(t, roms)
	romBody := decodeJSONMap(t, roms.Body)
	romItems := mustArray(t, romBody["roms"], "roms")
	if len(romItems) == 0 {
		t.Fatalf("expected non-empty rom list body=%s", roms.Body.String())
	}

	found := false
	for _, item := range romItems {
		rom := mustObject(t, item, "roms[]")
		if mustString(t, rom["romSha1"], "roms[].romSha1") == "agent-rom-sha1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected uploaded rom in rom list body=%s", roms.Body.String())
	}
}

func TestAgentAPIRescan(t *testing.T) {
	h := newContractHarness(t)

	uploadSave(t, h, "/api/saves", map[string]string{
		"rom_sha1": "agent-rescan-rom",
		"slotName": "default",
		"system":   "n64",
	}, "Wave Race 64 (USA).eep", []byte("agent-rescan-save"))

	rr := h.json(http.MethodPost, "/api/saves/rescan", strings.NewReader(`{"dryRun":false,"pruneUnsupported":true}`))
	assertStatus(t, rr, http.StatusOK)
	assertJSONContentType(t, rr)

	body := decodeJSONMap(t, rr.Body)
	if !mustBool(t, body["success"], "success") {
		t.Fatalf("expected success body=%s", rr.Body.String())
	}
	result := mustObject(t, body["result"], "result")
	if mustNumber(t, result["scanned"], "result.scanned") < 1 {
		t.Fatalf("expected scanned count body=%s", rr.Body.String())
	}
}

func TestAgentAPILogs(t *testing.T) {
	h := newContractHarness(t)

	uploadSave(t, h, "/api/saves", map[string]string{
		"rom_sha1": "agent-log-rom",
		"slotName": "default",
		"system":   "n64",
	}, "F-Zero X (USA).eep", []byte("agent-log-save"))

	rr := h.request(http.MethodGet, "/api/logs?hours=72&page=1&limit=50", nil)
	assertStatus(t, rr, http.StatusOK)
	assertJSONContentType(t, rr)

	body := decodeJSONMap(t, rr.Body)
	if !mustBool(t, body["success"], "success") {
		t.Fatalf("expected success body=%s", rr.Body.String())
	}
	logs := mustArray(t, body["logs"], "logs")
	if len(logs) == 0 {
		t.Fatalf("expected at least one log entry body=%s", rr.Body.String())
	}

	first := mustObject(t, logs[0], "logs[0]")
	if mustString(t, first["action"], "logs[0].action") == "" {
		t.Fatalf("expected action in first log body=%s", rr.Body.String())
	}
	if mustString(t, first["game"], "logs[0].game") == "" {
		t.Fatalf("expected game in first log body=%s", rr.Body.String())
	}
}
