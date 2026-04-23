package main

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeSaveInputRejectsRuntimeProfilePlaceholderTitlesAcrossProjectionSystems(t *testing.T) {
	t.Helper()

	gbaPayload := make([]byte, 32768)
	copy(gbaPayload[:16], []byte("SRAM_V113"))

	cases := []struct {
		name       string
		filename   string
		systemSlug string
		profile    string
		romSHA1    string
		payload    []byte
	}{
		{
			name:       "n64 retroarch placeholder",
			filename:   "profile-retroarch.eep",
			systemSlug: "n64",
			profile:    n64ProfileRetroArch,
			payload:    buildTestN64Payload("eep", "placeholder"),
		},
		{
			name:       "snes snes9x placeholder",
			filename:   "profile-snes9x.srm",
			systemSlug: "snes",
			profile:    "snes/snes9x",
			romSHA1:    "snes-placeholder-rom",
			payload:    buildNonBlankPayload(2048, 0x13),
		},
		{
			name:       "gba mgba placeholder",
			filename:   "profile-mgba.sav",
			systemSlug: "gba",
			profile:    "gba/mgba",
			romSHA1:    "gba-placeholder-rom",
			payload:    gbaPayload,
		},
		{
			name:       "nes fceux placeholder",
			filename:   "profile-fceux.sav",
			systemSlug: "nes",
			profile:    "nes/fceux",
			payload:    buildNonBlankPayload(8192, 0x21),
		},
		{
			name:       "genesis plus gx placeholder",
			filename:   "profile-genesis-plus-gx.srm",
			systemSlug: "genesis",
			profile:    "genesis/genesis-plus-gx",
			romSHA1:    "genesis-placeholder-rom",
			payload:    buildNonBlankPayload(8192, 0x31),
		},
		{
			name:       "master system meka placeholder",
			filename:   "profile-meka.sav",
			systemSlug: "master-system",
			profile:    "sms/meka",
			romSHA1:    "sms-placeholder-rom",
			payload:    buildNonBlankPayload(8192, 0x41),
		},
		{
			name:       "game gear gearsystem placeholder",
			filename:   "profile-gearsystem.sav",
			systemSlug: "game-gear",
			profile:    "gamegear/gearsystem",
			romSHA1:    "gg-placeholder-rom",
			payload:    buildNonBlankPayload(8192, 0x51),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			a := &app{}
			result := a.normalizeSaveInputDetailed(saveCreateInput{
				Filename:              tc.filename,
				Payload:               tc.payload,
				Game:                  game{Name: strings.TrimSuffix(tc.filename, filepath.Ext(tc.filename))},
				SystemSlug:            tc.systemSlug,
				TrustedHelperSystem:   true,
				ROMSHA1:               tc.romSHA1,
				RuntimeProfile:        tc.profile,
				SourceArtifactProfile: tc.profile,
			})
			if !result.Rejected {
				t.Fatalf("expected placeholder upload to be rejected for %s", tc.systemSlug)
			}
			if !strings.Contains(result.RejectReason, "runtime profile placeholder") {
				t.Fatalf("unexpected reject reason: %q", result.RejectReason)
			}
		})
	}
}

func TestContractSavesMultipartRejectsNESRuntimeProfilePlaceholderUpload(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "nes-placeholder-helper")

	rr := h.multipart("/saves", map[string]string{
		"app_password":   helperKey,
		"slotName":       "default",
		"system":         "nes",
		"device_type":    "retroarch",
		"fingerprint":    "nes-placeholder-device",
		"runtimeProfile": "nes/fceux",
	}, "file", "profile-fceux.sav", buildNonBlankPayload(8192, 0x29))
	assertStatus(t, rr, http.StatusUnprocessableEntity)
	assertJSONContentType(t, rr)

	body := decodeJSONMap(t, rr.Body)
	reason := strings.ToLower(mustString(t, body["reason"], "reason"))
	if !strings.Contains(reason, "runtime profile placeholder") {
		t.Fatalf("unexpected placeholder rejection response: %s", prettyJSON(body))
	}
}
