package main

import (
	"net/http"
	"testing"
)

func TestN64CheatInventoryByObservedTitle(t *testing.T) {
	cases := []struct {
		name          string
		fileName      string
		payload       []byte
		wantSupported bool
		wantEditorID  string
	}{
		{
			name:          "super-mario-64-valid-parser-backed-payload",
			fileName:      "Super Mario 64 (USA).eep",
			payload:       buildSM64FixturePayload(),
			wantSupported: true,
			wantEditorID:  "sm64-eeprom",
		},
		{
			name:          "super-mario-64-valid-canonical-eeprom-payload",
			fileName:      "Super Mario 64 (USA).eep",
			payload:       normalizeN64EEPROM(buildSM64FixturePayload()),
			wantSupported: true,
			wantEditorID:  "sm64-eeprom",
		},
		{
			name:          "super-mario-64-invalid-payload-does-not-fake-support",
			fileName:      "Super Mario 64 (USA).eep",
			payload:       buildTestN64Payload("eep", "sm64-generic"),
			wantSupported: false,
		},
		{
			name:          "mario-kart-64-valid-parser-backed-payload",
			fileName:      "Mario Kart 64 (USA).eep",
			payload:       buildMK64FixturePayload(),
			wantSupported: true,
			wantEditorID:  "mk64-eeprom",
		},
		{
			name:          "mario-kart-64-valid-canonical-eeprom-payload",
			fileName:      "Mario Kart 64 (USA).eep",
			payload:       normalizeN64EEPROM(buildMK64FixturePayload()),
			wantSupported: true,
			wantEditorID:  "mk64-eeprom",
		},
		{
			name:          "mario-kart-64-invalid-payload-does-not-fake-support",
			fileName:      "Mario Kart 64 (USA).eep",
			payload:       buildTestN64Payload("eep", "mk64-generic"),
			wantSupported: false,
		},
		{
			name:          "star-fox-64-has-no-curated-pack",
			fileName:      "Star Fox 64 (USA).eep",
			payload:       buildTestN64Payload("eep", "star-fox-64"),
			wantSupported: false,
		},
		{
			name:          "diddy-kong-racing-valid-canonical-eeprom-payload",
			fileName:      "Diddy Kong Racing (USA).eep",
			payload:       normalizeN64EEPROM(buildDKRFixturePayload()),
			wantSupported: true,
			wantEditorID:  "dkr-eeprom",
		},
		{
			name:          "wave-race-64-has-no-curated-pack",
			fileName:      "Wave Race 64 (Japan).eep",
			payload:       buildTestN64Payload("eep", "wave-race-64"),
			wantSupported: false,
		},
		{
			name:          "f-zero-x-has-no-curated-pack",
			fileName:      "F-Zero X (USA).eep",
			payload:       buildTestN64Payload("eep", "f-zero-x"),
			wantSupported: false,
		},
		{
			name:          "yoshis-story-has-no-curated-pack",
			fileName:      "Yoshi's Story (USA) (En,Ja).eep",
			payload:       buildTestN64Payload("eep", "yoshis-story"),
			wantSupported: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newContractHarness(t)
			body := uploadSave(t, h, "/saves", map[string]string{
				"rom_sha1": "n64-cheat-inventory-" + tc.name,
				"slotName": "default",
				"system":   "n64",
			}, tc.fileName, tc.payload)
			saveID := mustString(t, mustObject(t, body["save"], "save")["id"], "save.id")

			rr := h.request(http.MethodGet, "/save/cheats?saveId="+saveID, nil)
			assertStatus(t, rr, http.StatusOK)
			assertJSONContentType(t, rr)

			payload := decodeJSONMap(t, rr.Body)
			cheats := mustObject(t, payload["cheats"], "cheats")
			if got := mustBool(t, cheats["supported"], "cheats.supported"); got != tc.wantSupported {
				t.Fatalf("supported mismatch for %s: got %v want %v body=%s", tc.fileName, got, tc.wantSupported, rr.Body.String())
			}

			editorID, _ := cheats["editorId"].(string)
			if editorID != tc.wantEditorID {
				t.Fatalf("editorId mismatch for %s: got %q want %q body=%s", tc.fileName, editorID, tc.wantEditorID, rr.Body.String())
			}

			if tc.wantSupported {
				if mustNumber(t, cheats["availableCount"], "cheats.availableCount") < 1 {
					t.Fatalf("expected available cheat fields for %s body=%s", tc.fileName, rr.Body.String())
				}
				return
			}

			if _, ok := cheats["availableCount"]; ok {
				t.Fatalf("did not expect availableCount for unsupported save %s body=%s", tc.fileName, rr.Body.String())
			}
		})
	}
}
