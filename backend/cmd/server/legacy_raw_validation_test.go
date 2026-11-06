package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"os"
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

func TestRawSystemsAcceptTheirMeasuredSaves(t *testing.T) {
	// Each row is a file measured on a real device; the extension and size are
	// what the emulator actually wrote, not what the format allows in theory.
	cases := []struct {
		slug string
		ext  string
		size int
		file string
	}{
		{slug: "ngp", ext: "flash", size: 536, file: "Baseball Stars - Pocket Sports Series (Japan, Europe) (En,Ja).flash"},
		{slug: "virtualboy", ext: "srm", size: 65536, file: "3-D Tetris (USA).srm"},
		{slug: "neogeo-cd", ext: "srm", size: 8192, file: "Choujin Gakuen - Gowcaizer ~ Voltage Fighter - Gowcaizer (World) (En,Ja).srm"},
	}

	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			if supportedSystemFromSlug(tc.slug) == nil {
				t.Fatalf("system %q is not in the supported catalog", tc.slug)
			}
			profile, ok := legacyRawSaveProfiles[tc.slug]
			if !ok {
				t.Fatalf("no raw save validation profile for %q", tc.slug)
			}
			if _, ok := profile.AllowedExts[tc.ext]; !ok {
				t.Fatalf("%s does not accept extension %q (only the measured save %q proves the system reachable)", tc.slug, tc.ext, tc.file)
			}
			if _, ok := profile.AllowedSizes[tc.size]; !ok {
				t.Fatalf("%s does not accept size %d, the measured size of %q", tc.slug, tc.size, tc.file)
			}
		})
	}
}

// A blank card from each system #56 added is refused at upload. Neo Geo CD's NeoCD core keeps an
// unformatted backup RAM as 8 KiB of zeros until a game formats it, so that is the blank shape.
func TestNormalizeSaveInputRejectsBlankSavesOfTheAddedSystems(t *testing.T) {
	cases := []struct {
		slug, file, reason string
		size               int
	}{
		{"neogeo-cd", "Ryuuko no Ken ~ Art of Fighting (World) (En,Ja,Es).srm", "neo geo cd raw save payload is blank (all 0x00)", 8192},
		{"ngp", "Metal Slug - 2nd Mission (World).srm", "neo geo pocket raw save payload is blank (all 0x00)", 8192},
		{"virtualboy", "Wario Land (Japan, USA).srm", "virtual boy raw save payload is blank (all 0x00)", 32768},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			result := (&app{}).normalizeSaveInputDetailed(saveCreateInput{
				Filename:            tc.file,
				Payload:             make([]byte, tc.size),
				Game:                game{Name: tc.file},
				Format:              "sram",
				ROMSHA1:             tc.slug + "-rom-sha1",
				SlotName:            "default",
				SystemSlug:          tc.slug,
				TrustedHelperSystem: true,
			})
			if !result.Rejected || result.RejectReason != tc.reason {
				t.Fatalf("expected a blank %s save refused with %q, got rejected=%v %q", tc.slug, tc.reason, result.Rejected, result.RejectReason)
			}
		})
	}
}

// The card Art of Fighting formats and writes on NeoCD (a Neo Geo CD memory card header, its block table
// and the game's file) still passes once the system is validated.
func TestNormalizeSaveInputAcceptsAFormattedNeoGeoCDCard(t *testing.T) {
	gz, err := os.ReadFile("testdata/neogeocd-aof-formatted.srm.gz")
	if err != nil {
		t.Fatal(err)
	}
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatal(err)
	}
	card, err := io.ReadAll(r)
	if err != nil || len(card) != 8192 {
		t.Fatalf("card: %d bytes, %v", len(card), err)
	}
	result := (&app{}).normalizeSaveInputDetailed(saveCreateInput{
		Filename:            "Ryuuko no Ken ~ Art of Fighting (World) (En,Ja,Es).srm",
		Payload:             card,
		Game:                game{Name: "Ryuuko no Ken ~ Art of Fighting"},
		Format:              "sram",
		ROMSHA1:             "4154d5fd1aa88d10417d969c461cb842d11d0102",
		SlotName:            "default",
		SystemSlug:          "neogeo-cd",
		TrustedHelperSystem: true,
	})
	if result.Rejected {
		t.Fatalf("expected the formatted Art of Fighting card accepted, got %q", result.RejectReason)
	}
}
