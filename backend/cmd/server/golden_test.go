package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestGoldenAuthMe(t *testing.T) {
	h := newContractHarness(t)
	rr := h.request(http.MethodGet, "/auth/me", nil)
	assertStatus(t, rr, http.StatusOK)
	assertGoldenJSONResponse(t, rr, "auth_me.json")
}

func TestGoldenSaveLatestMissing(t *testing.T) {
	h := newContractHarness(t)
	rr := h.request(http.MethodGet, "/save/latest?romSha1=missing-rom&slotName=default", nil)
	assertStatus(t, rr, http.StatusOK)
	assertGoldenJSONResponse(t, rr, "save_latest_missing.json")
}

func TestGoldenSaveLatestSuccess(t *testing.T) {
	h := newContractHarness(t)
	uploadSave(t, h, "/saves", map[string]string{"rom_sha1": "golden-rom", "slotName": "default", "system": "n64"}, "slot1.eep", []byte("golden-save"))
	rr := h.request(http.MethodGet, "/save/latest?romSha1=golden-rom&slotName=default", nil)
	assertStatus(t, rr, http.StatusOK)
	assertGoldenJSONResponse(t, rr, "save_latest_success.json")
}

func TestGoldenSavesListEnvelope(t *testing.T) {
	h := newContractHarness(t)
	rr := h.request(http.MethodGet, "/v1/saves?limit=2&offset=0", nil)
	assertStatus(t, rr, http.StatusOK)
	assertGoldenJSONResponse(t, rr, "saves_list.json")
}

func TestGoldenDevicesList(t *testing.T) {
	h := newContractHarness(t)
	rr := h.request(http.MethodGet, "/devices", nil)
	assertStatus(t, rr, http.StatusOK)
	assertGoldenJSONResponse(t, rr, "devices_list.json")
}

func TestGoldenConflictCheckMissing(t *testing.T) {
	h := newContractHarness(t)
	rr := h.request(http.MethodGet, "/conflicts/check?romSha1=missing-rom&slotName=default", nil)
	assertStatus(t, rr, http.StatusOK)
	assertGoldenJSONResponse(t, rr, "conflicts_check_missing.json")
}

func TestGoldenRomLookup(t *testing.T) {
	h := newContractHarness(t)
	rr := h.request(http.MethodGet, "/rom/lookup?filenameStem=Wario%20Land%20II", nil)
	assertStatus(t, rr, http.StatusOK)
	assertGoldenJSONResponse(t, rr, "rom_lookup.json")
}

func TestGoldenEventsPrelude(t *testing.T) {
	h := newContractHarness(t)
	rr := h.ssePrelude("/events")
	assertStatus(t, rr, http.StatusOK)
	got := rr.Header().Get("Content-Type") + "\n" + strings.SplitN(rr.Body.String(), "\n\n", 2)[0] + "\n\n"
	assertGoldenText(t, got, "events_connected.txt")
}
