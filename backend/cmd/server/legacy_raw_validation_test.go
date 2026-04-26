package main

import (
	"net/http"
	"testing"
)

func TestNormalizeSaveInputAcceptsTrustedLegacyRawSaves(t *testing.T) {
	cases := []struct {
		systemSlug string
		profile    string
		filename   string
		size       int
		kind       string
	}{
		{
			systemSlug: "pc-engine",
			profile:    "pc-engine/mister",
			filename:   "Ys Book I & II.brm",
			size:       2048,
			kind:       "PC Engine / TurboGrafx-16 backup RAM",
		},
		{
			systemSlug: "atari-lynx",
			profile:    "atari-lynx/handy",
			filename:   "Todd's Adventures in Slime World.eeprom",
			size:       512,
			kind:       "Atari Lynx cartridge EEPROM",
		},
		{
			systemSlug: "wonderswan",
			profile:    "wonderswan/mednafen",
			filename:   "Final Fantasy.sav",
			size:       8192,
			kind:       "WonderSwan cartridge backup memory",
		},
		{
			systemSlug: "sg-1000",
			profile:    "sg-1000/gearsystem",
			filename:   "The Castle.sav",
			size:       8192,
			kind:       "SG-1000 cartridge backup memory",
		},
		{
			systemSlug: "colecovision",
			profile:    "colecovision/gearcoleco",
			filename:   "Boulder Dash.sav",
			size:       2048,
			kind:       "ColecoVision cartridge NVRAM/EEPROM",
		},
		{
			systemSlug: "atari-jaguar",
			profile:    "atari-jaguar/bigpemu",
			filename:   "Tempest 2000.eeprom",
			size:       2048,
			kind:       "Atari Jaguar cartridge EEPROM/NVRAM",
		},
		{
			systemSlug: "3do",
			profile:    "3do/opera",
			filename:   "Road Rash.srm",
			size:       32768,
			kind:       "3DO NVRAM",
		},
	}

	for _, tc := range cases {
		t.Run(tc.systemSlug, func(t *testing.T) {
			a := &app{}
			result := a.normalizeSaveInputDetailed(saveCreateInput{
				Filename:              tc.filename,
				Payload:               buildNonBlankPayload(tc.size, 0x5a),
				Game:                  game{Name: tc.filename},
				Format:                "sram",
				ROMSHA1:               tc.systemSlug + "-rom-sha1",
				RuntimeProfile:        tc.profile,
				SlotName:              "default",
				SourceArtifactProfile: tc.profile,
				SystemSlug:            tc.systemSlug,
				TrustedHelperSystem:   true,
			})
			if result.Rejected {
				t.Fatalf("expected %s raw save to be accepted, got reject=%q", tc.systemSlug, result.RejectReason)
			}
			if result.Input.SystemSlug != tc.systemSlug {
				t.Fatalf("expected system slug %q, got %q", tc.systemSlug, result.Input.SystemSlug)
			}
			if result.Input.Inspection == nil {
				t.Fatal("expected inspection metadata")
			}
			if result.Input.Inspection.ValidatedSystem != tc.systemSlug {
				t.Fatalf("unexpected validated system: %+v", result.Input.Inspection)
			}
			if got := result.Input.Inspection.SemanticFields["rawSaveKind"]; got != tc.kind {
				t.Fatalf("expected raw save kind %q, got %+v", tc.kind, result.Input.Inspection.SemanticFields)
			}
		})
	}
}

func TestNormalizeSaveInputRejectsLegacyRawSavesWithoutROMSHA1(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Ys Book I & II.brm",
		Payload:             buildNonBlankPayload(2048, 0x22),
		Game:                game{Name: "Ys Book I & II"},
		Format:              "sram",
		SlotName:            "default",
		SystemSlug:          "pc-engine",
		TrustedHelperSystem: true,
	})
	if !result.Rejected {
		t.Fatal("expected PC Engine raw save without rom_sha1 to be rejected")
	}
	if result.RejectReason != "pc engine / turbografx-16 raw saves require rom_sha1" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestNormalizeSaveInputRejectsBlankLegacyRawSaves(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Tempest 2000.eeprom",
		Payload:             make([]byte, 2048),
		Game:                game{Name: "Tempest 2000"},
		Format:              "eeprom",
		ROMSHA1:             "jaguar-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "atari-jaguar",
		TrustedHelperSystem: true,
	})
	if !result.Rejected {
		t.Fatal("expected blank Atari Jaguar raw save to be rejected")
	}
	if result.RejectReason != "atari jaguar raw save payload is blank (all 0x00)" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestLegacyRuntimeProfilesExposeDownloadTargets(t *testing.T) {
	cases := []struct {
		systemSlug      string
		expectedProfile string
		expectedExt     string
	}{
		{systemSlug: "sega-cd", expectedProfile: "sega-cd/genesis-plus-gx", expectedExt: ".brm"},
		{systemSlug: "sega-32x", expectedProfile: "sega-32x/picodrive", expectedExt: ".srm"},
		{systemSlug: "pc-engine", expectedProfile: "pc-engine/mister", expectedExt: ".sav"},
		{systemSlug: "atari-lynx", expectedProfile: "atari-lynx/handy", expectedExt: ".eeprom"},
		{systemSlug: "wonderswan", expectedProfile: "wonderswan/mednafen", expectedExt: ".sav"},
		{systemSlug: "sg-1000", expectedProfile: "sg-1000/gearsystem", expectedExt: ".sav"},
		{systemSlug: "colecovision", expectedProfile: "colecovision/gearcoleco", expectedExt: ".sav"},
		{systemSlug: "atari-jaguar", expectedProfile: "atari-jaguar/bigpemu", expectedExt: ".eeprom"},
		{systemSlug: "3do", expectedProfile: "3do/opera", expectedExt: ".srm"},
	}

	for _, tc := range cases {
		t.Run(tc.systemSlug, func(t *testing.T) {
			if !isProjectionCapableSystem(tc.systemSlug) {
				t.Fatalf("expected %s to be projection-capable", tc.systemSlug)
			}
			profiles := downloadProfilesForSummary(saveSummary{
				DisplayTitle: "Test Save",
				Filename:     "test.sav",
				SystemSlug:   tc.systemSlug,
			})
			for _, profile := range profiles {
				if profile.ID == tc.expectedProfile {
					if profile.TargetExtension != tc.expectedExt {
						t.Fatalf("expected %s extension %q, got %q", tc.expectedProfile, tc.expectedExt, profile.TargetExtension)
					}
					return
				}
			}
			t.Fatalf("expected profile %q in %+v", tc.expectedProfile, profiles)
		})
	}
}

func TestContractSavesMultipartRequiresRuntimeProfileForLegacyProjectionSystem(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "pc-engine-helper")

	rr := h.multipart("/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "pc-engine-rom-sha1",
		"slotName":     "default",
		"system":       "pc-engine",
		"device_type":  "mister",
		"fingerprint":  "pc-engine-device",
	}, "file", "Ys Book I & II.brm", buildNonBlankPayload(2048, 0x2a))
	assertStatus(t, rr, http.StatusBadRequest)
	assertJSONContentType(t, rr)
}
