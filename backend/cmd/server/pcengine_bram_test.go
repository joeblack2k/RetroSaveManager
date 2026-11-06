package main

import (
	"encoding/binary"
	"testing"
)

// pcEngineBRAM builds a 2048-byte PC Engine backup RAM image: the "HUBM"
// header, the end-of-BRAM pointer (0x8800) and the next-free pointer. A
// freshly formatted BRAM holds no saves, so its next-free pointer is 0x8010,
// the first byte after the 16-byte header.
func pcEngineBRAM(nextFree uint16) []byte {
	payload := make([]byte, 2048)
	copy(payload, "HUBM")
	binary.LittleEndian.PutUint16(payload[4:6], 0x8800)
	binary.LittleEndian.PutUint16(payload[6:8], nextFree)
	if nextFree > 0x8010 {
		entry := payload[0x10:]
		binary.LittleEndian.PutUint16(entry[0:2], nextFree-0x8010)
		copy(entry[6:], "BALLISTIX8")
		for i := 0x20; i < int(nextFree-0x8000); i++ {
			payload[i] = byte(i)
		}
	}
	return payload
}

func pcEngineInput(systemSlug string, payload []byte) saveCreateInput {
	return saveCreateInput{
		Filename:            "Ballistix (USA).srm",
		Payload:             payload,
		Game:                game{Name: "Ballistix (USA)"},
		Format:              "sram",
		ROMSHA1:             "pc-engine-rom-sha1",
		SlotName:            "default",
		SystemSlug:          systemSlug,
		TrustedHelperSystem: true,
	}
}

// The SGM helpers declare PC Engine saves as "tgfx16".
func TestNormalizeSaveInputAcceptsTGFX16AsPCEngine(t *testing.T) {
	if got := supportedSystemSlugFromLabel("tgfx16"); got != "pc-engine" {
		t.Fatalf("expected tgfx16 to name pc-engine, got %q", got)
	}
	a := &app{}
	result := a.normalizeSaveInputDetailed(pcEngineInput("tgfx16", pcEngineBRAM(0x80c9)))
	if result.Rejected {
		t.Fatalf("expected a PC Engine BRAM declared as tgfx16 to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.SystemSlug != "pc-engine" {
		t.Fatalf("expected system slug pc-engine, got %q", result.Input.SystemSlug)
	}
}

// A formatted BRAM with no saves in it is every game's same 2048 bytes; it is
// blank, like an all-0x00 payload, and is not stored.
func TestNormalizeSaveInputRejectsFormattedEmptyPCEngineBRAM(t *testing.T) {
	a := &app{}
	for _, slug := range []string{"pc-engine", "tgfx16"} {
		result := a.normalizeSaveInputDetailed(pcEngineInput(slug, pcEngineBRAM(0x8010)))
		if !result.Rejected {
			t.Fatalf("%s: expected a formatted, empty PC Engine BRAM to be rejected", slug)
		}
		if result.RejectReason != "pc engine / turbografx-16 raw save payload is blank (formatted backup RAM with no saves)" {
			t.Fatalf("%s: unexpected reject reason: %q", slug, result.RejectReason)
		}
	}
}

func TestIsEmptyPCEngineBRAM(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		empty   bool
	}{
		{"formatted, no saves", pcEngineBRAM(0x8010), true},
		{"one save", pcEngineBRAM(0x80c9), false},
		{"no HUBM header", buildNonBlankPayload(2048, 0x5a), false},
		{"too short", []byte("HUBM"), false},
	}
	for _, tc := range cases {
		if got := isEmptyPCEngineBRAM(tc.payload); got != tc.empty {
			t.Errorf("%s: isEmptyPCEngineBRAM = %v, want %v", tc.name, got, tc.empty)
		}
	}
}
