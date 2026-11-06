package main

import (
	"encoding/hex"
	"testing"
)

// The first 256 bytes of a freshly formatted Opera (3DO) NVRAM, as Opera's
// libretro core writes it for any game before the game stores anything: the
// "opera formatted" volume label, then one free extent covering the card. The
// rest of the 32 KiB image is zero.
const operaFormattedHeader = "015a5a5a5a5a02006f7065726120666f726d617474656400000000000000000000000000000000006e7672616d000000000000000000000000000000000000000000000000000000ffffffff0000000100008000fffffffe0000000000000001000000000000008400000000000000000000000000000000000000000000000000000000855a02b6000000980000009800000014000000147aa565bd000000840000008400007f6800000014000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

func operaNVRAM(t *testing.T, edit func([]byte)) []byte {
	t.Helper()
	hdr, err := hex.DecodeString(operaFormattedHeader)
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 32768)
	copy(payload, hdr)
	if edit != nil {
		edit(payload)
	}
	return payload
}

// Bust-A-Move's first save: the free extent shrinks and a file entry "MC_PB"
// follows it (bytes taken from the image Opera wrote).
func withOneFile(p []byte) {
	b, _ := hex.DecodeString("be4f32a6000000f90000008400000061000000400000002000000000202020204d435f5042")
	copy(p[0x98:], b)
	p[0x3f00] = 0x5a
}

func TestIsEmptyOperaNVRAM(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func([]byte)
		empty bool
	}{
		{"formatted, no files", nil, true},
		{"one file stored", withOneFile, false},
		{"no opera label", func(p []byte) { copy(p[8:], "something else ") }, false},
		{"free extent short of the card", func(p []byte) { p[0xa7] = 0x60 }, false},
	} {
		if got := isEmptyOperaNVRAM(operaNVRAM(t, tc.edit)); got != tc.empty {
			t.Errorf("%s: isEmptyOperaNVRAM = %v, want %v", tc.name, got, tc.empty)
		}
	}
	if isEmptyOperaNVRAM([]byte("short")) {
		t.Error("a short payload is not an Opera NVRAM")
	}
}

func operaInput(payload []byte) saveCreateInput {
	return saveCreateInput{
		Filename:            "Battle Chess (USA).0.srm",
		Payload:             payload,
		Game:                game{Name: "Battle Chess (USA)"},
		Format:              "sram",
		ROMSHA1:             "3do-rom-sha1",
		SlotName:            "default",
		SystemSlug:          "3do",
		TrustedHelperSystem: true,
	}
}

// A formatted Opera NVRAM with nothing in it is every game's same image; it is
// blank and is not stored. One that holds a game's file is kept.
func TestNormalizeSaveInputRejectsFormattedEmptyOperaNVRAM(t *testing.T) {
	a := &app{}
	if r := a.normalizeSaveInputDetailed(operaInput(operaNVRAM(t, nil))); !r.Rejected {
		t.Fatal("expected an empty formatted Opera NVRAM to be rejected as blank")
	}
	if r := a.normalizeSaveInputDetailed(operaInput(operaNVRAM(t, withOneFile))); r.Rejected {
		t.Fatalf("expected an Opera NVRAM holding a file to be accepted, got reject=%q", r.RejectReason)
	}
}
