package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A shared Flycast VMU card (RetroArch keeps every game's saves in bios/dc/vmu_save_<slot>.bin when
// per-game cards are off) is a whole 128 KiB VMU image, which parseDreamcastVMUImage labels
// container "vmu-bin".

func readDreamcastVMUFixture(t *testing.T, name string) []byte {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return payload
}

func TestDreamcastVMUCardIsCompatibleWithFlycastAndMiSTerProfiles(t *testing.T) {
	payload := readDreamcastVMUFixture(t, "dreamcast_vmu_soulcalibur_A1.bin")
	details := parseDreamcastContainer("vmu_save_A1.bin", payload)
	if details == nil || details.Container != "vmu-bin" || details.SaveEntries <= 0 {
		t.Fatalf("Soulcalibur card: want a vmu-bin container with saves, got %+v", details)
	}
	summary := saveSummary{Filename: "vmu_save_A1.bin", Dreamcast: details}
	for _, profile := range []string{"dreamcast/retroarch-flycast", "dreamcast/flycast", "dreamcast/redream", "dreamcast/mister"} {
		if !dreamcastRuntimeProfileCompatible(summary, profile) {
			t.Errorf("%s: a VMU card (container vmu-bin) is refused", profile)
		}
		if _, _, out, err := projectDreamcastPayload(summary, payload, profile); err != nil {
			t.Errorf("%s: projecting the VMU card: %v", profile, err)
		} else if string(out) != string(payload) {
			t.Errorf("%s: projection changed the card's bytes", profile)
		}
	}
}

func TestBlankDreamcastVMUCardIsStillRefused(t *testing.T) {
	payload := readDreamcastVMUFixture(t, "dreamcast_vmu_blank.bin")
	result := validateDreamcastSave(saveCreateInput{Filename: "vmu_save_B1.bin", Payload: payload}, normalizedSaveMetadata{})
	if !result.Rejected || !strings.Contains(result.RejectReason, "no active save entries") {
		t.Fatalf("blank VMU card: want refused with no active save entries, got rejected=%v %q", result.Rejected, result.RejectReason)
	}
}
