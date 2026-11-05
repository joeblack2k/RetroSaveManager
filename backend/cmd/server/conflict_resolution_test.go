package main

import (
	"net/http"
	"strings"
	"testing"
)

// A "keep local" or "keep server" decision is one-shot: it is tied to the pair of
// hashes it was made for, it is acted on once, and a decision whose pair no
// longer matches is stale and raises the conflict again instead.

const (
	resolveLocal = "1111111111111111111111111111111111111111111111111111111111111111"
	resolveCloud = "2222222222222222222222222222222222222222222222222222222222222222"
)

func reportResolveConflict(t *testing.T, h *contractHarness, local, cloud string) string {
	t.Helper()
	resp := h.multipart("/conflicts/report", map[string]string{
		"romSha1":     "resolve-rom",
		"slotName":    "default",
		"localSha256": local,
		"cloudSha256": cloud,
	}, "file", "Resolve (USA).srm", []byte("local-bytes"))
	assertStatus(t, resp, http.StatusOK)
	id, _ := decodeJSONMap(t, resp.Body)["conflictId"].(string)
	if id == "" {
		t.Fatalf("report returned no conflictId")
	}
	return id
}

func checkResolveConflict(t *testing.T, h *contractHarness) map[string]any {
	t.Helper()
	resp := h.request(http.MethodGet, "/conflicts/check?romSha1=resolve-rom&slotName=default", nil)
	assertStatus(t, resp, http.StatusOK)
	return decodeJSONMap(t, resp.Body)
}

func resolveConflictAs(t *testing.T, h *contractHarness, id, resolution string) {
	t.Helper()
	resp := h.json(http.MethodPost, "/conflicts/"+id+"/resolve", strings.NewReader(`{"resolution":"`+resolution+`"}`))
	assertStatus(t, resp, http.StatusOK)
}

func TestConflictDecisionIsOfferedWithThePairItWasMadeFor(t *testing.T) {
	h := newContractHarness(t)
	id := reportResolveConflict(t, h, resolveLocal, resolveCloud)
	resolveConflictAs(t, h, id, "keep_local")

	body := checkResolveConflict(t, h)
	if body["exists"] != true {
		t.Fatalf("a decided conflict must stay until the helper acts on it: %s", prettyJSON(body))
	}
	if got, _ := body["resolution"].(string); got != "keep_local" {
		t.Fatalf("check must carry the decision, got %q: %s", got, prettyJSON(body))
	}
	if got, _ := body["localSha256"].(string); got != resolveLocal {
		t.Fatalf("check must carry the local sha256 the decision is for, got %q", got)
	}
	if got, _ := body["cloudSha256"].(string); got != resolveCloud {
		t.Fatalf("check must carry the cloud sha256 the decision is for, got %q", got)
	}
}

func TestAppliedConflictDecisionIsClearedOnce(t *testing.T) {
	h := newContractHarness(t)
	id := reportResolveConflict(t, h, resolveLocal, resolveCloud)
	resolveConflictAs(t, h, id, "keep_server")

	resp := h.json(http.MethodPost, "/conflicts/"+id+"/applied",
		strings.NewReader(`{"localSha256":"`+resolveLocal+`","cloudSha256":"`+resolveCloud+`"}`))
	assertStatus(t, resp, http.StatusOK)

	if body := checkResolveConflict(t, h); body["exists"] != false {
		t.Fatalf("an applied decision must clear the conflict: %s", prettyJSON(body))
	}
}

func TestStaleConflictDecisionIsNotActedOnAndReopens(t *testing.T) {
	h := newContractHarness(t)
	id := reportResolveConflict(t, h, resolveLocal, resolveCloud)
	resolveConflictAs(t, h, id, "keep_local")

	other := "3333333333333333333333333333333333333333333333333333333333333333"
	resp := h.json(http.MethodPost, "/conflicts/"+id+"/applied",
		strings.NewReader(`{"localSha256":"`+other+`","cloudSha256":"`+resolveCloud+`"}`))
	assertStatus(t, resp, http.StatusConflict)

	body := checkResolveConflict(t, h)
	if body["exists"] != true {
		t.Fatalf("a stale decision must leave the conflict open: %s", prettyJSON(body))
	}
	if got, _ := body["resolution"].(string); got != "" {
		t.Fatalf("a stale decision must be cleared, not kept as a standing preference, got %q", got)
	}
}

func TestReportForANewPairClearsAnEarlierDecision(t *testing.T) {
	h := newContractHarness(t)
	id := reportResolveConflict(t, h, resolveLocal, resolveCloud)
	resolveConflictAs(t, h, id, "keep_local")

	other := "4444444444444444444444444444444444444444444444444444444444444444"
	reportResolveConflict(t, h, other, resolveCloud)

	body := checkResolveConflict(t, h)
	if got, _ := body["resolution"].(string); got != "" {
		t.Fatalf("a decision made for another pair must not carry over, got %q", got)
	}
}

func TestDismissStillDeletesTheConflict(t *testing.T) {
	h := newContractHarness(t)
	id := reportResolveConflict(t, h, resolveLocal, resolveCloud)
	resolveConflictAs(t, h, id, "dismiss")
	if body := checkResolveConflict(t, h); body["exists"] != false {
		t.Fatalf("dismiss must keep deleting the conflict: %s", prettyJSON(body))
	}
}
