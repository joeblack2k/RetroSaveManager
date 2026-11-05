package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// A MAME machine's NVRAM components are named after the device they belong to
// -- eeprom, game, saveram -- and every machine uses the same names. A record
// stored before the uploader said which machine it came from is titled with
// that component name, so a title-based lookup cannot tell one machine's
// "eeprom" from another's. When the requested machine has no record of its
// own, answering with a same-named component from a different machine hands
// the helper another machine's save to write over its own.
func TestLatestSaveDoesNotAnswerWithAnotherMachinesComponent(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "arcade-windows")

	donor := make([]byte, 8192)
	for i := range donor {
		donor[i] = byte(i * 3)
	}
	uploadSave(t, h, "/saves", map[string]string{
		"app_password":   helperKey,
		"rom_sha1":       "rom-acedrvrw",
		"slotName":       "default",
		"system":         "arcade",
		"device_type":    "windows",
		"fingerprint":    "arcade-windows",
		"runtimeProfile": "arcade/mister",
	}, "eeprom.nv", donor)

	latest := helperGET(t, h, "/save/latest?romSha1=rom-akatana&slotName=default&device_type=windows&fingerprint=arcade-windows&runtimeProfile=arcade/mister&filename=eeprom&system=arcade&displayTitle=eeprom", helperKey)
	assertStatus(t, latest, http.StatusOK)
	var body struct {
		Exists *bool   `json:"exists"`
		ID     *string `json:"id"`
	}
	if err := json.Unmarshal(latest.Body.Bytes(), &body); err != nil {
		t.Fatalf("latest body: %s", latest.Body.String())
	}
	if body.ID != nil && *body.ID != "" {
		t.Fatalf("a machine with no record of its own was handed %q, which belongs to another machine: %s", *body.ID, latest.Body.String())
	}
	if body.Exists != nil && *body.Exists {
		t.Fatalf("a machine with no record of its own was told a save exists: %s", latest.Body.String())
	}
}

// The machine's components share its ROM sha and slot, so a lookup that stops
// there cannot distinguish them: whichever component was stored last answers
// for all of them, and a restore writes that one payload over every sibling.
func TestLatestSaveDistinguishesOneMachinesComponents(t *testing.T) {
	h := newContractHarness(t)
	_, helperKey := createHelperAppPasswordRecord(t, h, "", "arcade-windows")

	eeprom := make([]byte, 128)
	for i := range eeprom {
		eeprom[i] = byte(i)
	}
	saveram := make([]byte, 64)
	for i := range saveram {
		saveram[i] = byte(255 - i)
	}
	fields := func(name string) map[string]string {
		return map[string]string{
			"app_password":   helperKey,
			"rom_sha1":       "rom-samsho",
			"slotName":       "default",
			"system":         "arcade",
			"device_type":    "windows",
			"fingerprint":    "arcade-windows",
			"runtimeProfile": "arcade/mister",
		}
	}
	uploadSave(t, h, "/saves", fields("eeprom"), "eeprom.nv", eeprom)
	uploadSave(t, h, "/saves", fields("saveram"), "saveram.nv", saveram)

	lookup := func(component string) string {
		t.Helper()
		response := helperGET(t, h, "/save/latest?romSha1=rom-samsho&slotName=default&device_type=windows&fingerprint=arcade-windows&runtimeProfile=arcade/mister&system=arcade&filename="+component+".nv&displayTitle="+component, helperKey)
		assertStatus(t, response, http.StatusOK)
		var body struct {
			SHA256 *string `json:"sha256"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("latest body: %s", response.Body.String())
		}
		if body.SHA256 == nil {
			return ""
		}
		return *body.SHA256
	}

	eepromLookup := lookup("eeprom")
	saveramLookup := lookup("saveram")
	if eepromLookup == "" || saveramLookup == "" {
		t.Fatalf("a stored component was not found: eeprom=%q saveram=%q", eepromLookup, saveramLookup)
	}
	if eepromLookup == saveramLookup {
		t.Fatalf("two components of one machine resolved to the same payload %q", eepromLookup)
	}
}
