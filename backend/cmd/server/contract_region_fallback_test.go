package main

import (
	"net/http"
	"net/url"
	"testing"
)

// A track-context fallback hit serves another ROM's bytes, so it must be a record
// whose region identifies the same release. The SMB Deluxe incident: a (J) save
// stored with region UNKNOWN was served to a USA ROM, written onto the USA file,
// and uploaded back as the USA track's newest version.
func TestContractSaveLatestTrackFallbackRefusesAnUnknownRegionRecord(t *testing.T) {
	h := newContractHarness(t)

	uploadSave(t, h, "/saves", map[string]string{
		"rom_sha1": "smbdx-japan-rom",
		"slotName": "default",
		"system":   "gameboy",
	}, "Super Mario Bros. Deluxe (J) (C).srm", buildNonBlankPayload(8192, 0x4a))

	for _, region := range []string{"", "US"} {
		query := url.Values{}
		query.Set("romSha1", "smbdx-usa-rom")
		query.Set("slotName", "default")
		query.Set("filename", "Super Mario Bros. Deluxe (USA, Europe) (Rev 1).sav")
		query.Set("system", "gameboy")
		if region != "" {
			query.Set("regionCode", region)
		}
		rr := h.request(http.MethodGet, "/save/latest?"+query.Encode(), nil)
		assertStatus(t, rr, http.StatusOK)
		body := decodeJSONMap(t, rr.Body)
		if mustBool(t, body["exists"], "exists") {
			t.Fatalf("regionCode=%q: a region-UNKNOWN record of another ROM was served by title: %s", region, prettyJSON(body))
		}
	}
}

// A fallback must not cross regions either: a record of a known region is served
// only to a request of that same region.
func TestContractSaveLatestTrackFallbackRefusesAnotherRegionsRecord(t *testing.T) {
	h := newContractHarness(t)

	uploadSave(t, h, "/saves", map[string]string{
		"rom_sha1": "game-japan-rom",
		"slotName": "default",
		"system":   "gameboy",
	}, "Some Game (Japan).srm", buildNonBlankPayload(8192, 0x51))

	query := url.Values{}
	query.Set("romSha1", "game-usa-rom")
	query.Set("slotName", "default")
	query.Set("filename", "Some Game (USA).sav")
	query.Set("system", "gameboy")
	rr := h.request(http.MethodGet, "/save/latest?"+query.Encode(), nil)
	assertStatus(t, rr, http.StatusOK)
	body := decodeJSONMap(t, rr.Body)
	if mustBool(t, body["exists"], "exists") {
		t.Fatalf("a JP record was served to a USA request: %s", prettyJSON(body))
	}
}

// A fallback must not serve a same-region save of a different ROM either. The
// Kirby's Adventure incident: the USA ROM's request matched no ROM sha1, the
// title fallback served the (USA) (Rev 1) record, and the MiSTer wrote it over
// its own USA save as an ordinary update.
func TestContractSaveLatestTrackFallbackRefusesAnotherROMsSameRegionRecord(t *testing.T) {
	h := newContractHarness(t)

	uploadSave(t, h, "/saves", map[string]string{
		"rom_sha1": "kirby-usa-rev1-rom",
		"slotName": "default",
		"system":   "nes",
	}, "Kirby's Adventure (USA) (Rev 1).srm", buildNonBlankPayload(8192, 0x4b))

	query := url.Values{}
	query.Set("romSha1", "kirby-usa-rom")
	query.Set("slotName", "default")
	query.Set("filename", "Kirby's Adventure (USA).sav")
	query.Set("system", "nes")
	rr := h.request(http.MethodGet, "/save/latest?"+query.Encode(), nil)
	assertStatus(t, rr, http.StatusOK)
	body := decodeJSONMap(t, rr.Body)
	if mustBool(t, body["exists"], "exists") {
		t.Fatalf("a USA (Rev 1) record of another ROM was served to the USA ROM by title: %s", prettyJSON(body))
	}
}

// A request that carries no ROM sha1 still matches by title: the guard refuses
// only when both sides name a ROM and the two differ.
func TestContractSaveLatestTrackFallbackStillServesARequestWithoutAROMSHA1(t *testing.T) {
	h := newContractHarness(t)

	uploadSave(t, h, "/saves", map[string]string{
		"rom_sha1": "kirby-usa-rom",
		"slotName": "default",
		"system":   "nes",
	}, "Kirby's Adventure (USA).srm", buildNonBlankPayload(8192, 0x4c))

	query := url.Values{}
	query.Set("slotName", "default")
	query.Set("filename", "Kirby's Adventure (USA).sav")
	query.Set("system", "nes")
	rr := h.request(http.MethodGet, "/save/latest?"+query.Encode(), nil)
	assertStatus(t, rr, http.StatusOK)
	body := decodeJSONMap(t, rr.Body)
	if !mustBool(t, body["exists"], "exists") {
		t.Fatalf("a title match for a request with no ROM sha1 was refused: %s", prettyJSON(body))
	}
}
