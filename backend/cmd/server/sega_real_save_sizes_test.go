package main

import "testing"

// A cartridge's SRAM is the size the cart carries, not a power of two: Master
// System carts hold 8188-byte saves with a 4-byte header, and Genesis carts
// hold 2114 and 16128. The size desk rejected them as an unrecognised format,
// which is the wrong reading -- it is a recognised format at a length the table
// did not list.
func TestContractAcceptsRealSegaSramSizes(t *testing.T) {
	cases := []struct {
		system   string
		profile  string
		filename string
		size     int
	}{
		{"master-system", "sms/genesis-plus-gx", "Phantasy Star (USA, Europe) (Rev A).srm", 8188},
		{"genesis", "genesis/genesis-plus-gx", "Madden NFL '94 (USA, Europe).srm", 2114},
		{"genesis", "genesis/retroarch-genesis-plus-gx", "Shining in the Darkness (USA, Europe).srm", 16128},
		{"genesis", "genesis/genesis-plus-gx", "Shining in the Darkness (USA, Europe).srm", 16128},
	}
	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			h := newContractHarness(t)
			helperKey := createHelperAppPassword(t, h, "", tc.system+"-helper")

			body := uploadSave(t, h, "/saves", map[string]string{
				"app_password":   helperKey,
				"rom_sha1":       tc.system + "-rom-sha1",
				"slotName":       "default",
				"system":         tc.system,
				"device_type":    "retroarch",
				"fingerprint":    tc.system + "-device",
				"runtimeProfile": tc.profile,
			}, tc.filename, buildNonBlankPayload(tc.size, 0x05))

			save := mustObject(t, body["save"], "save")
			saveID := mustString(t, save["id"], "save.id")
			stored := false
			for _, record := range h.app.snapshotSaveRecords() {
				if record.Summary.ID != saveID {
					continue
				}
				stored = true
				if record.Summary.LatestSizeBytes != tc.size {
					t.Fatalf("stored %d bytes, want the %d the cart wrote", record.Summary.LatestSizeBytes, tc.size)
				}
			}
			if !stored {
				t.Fatalf("upload reported save %q but no record holds it: %s", saveID, prettyJSON(save))
			}
		})
	}
}

// Game Gear carts hold 26752-byte saves. Same rule as the other Sega systems:
// the cart's size is the save's size.
func TestContractAcceptsGameGearCartridgeSize(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "gamegear-helper")

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password":   helperKey,
		"rom_sha1":       "gamegear-rom-sha1",
		"slotName":       "default",
		"system":         "game-gear",
		"device_type":    "retroarch",
		"fingerprint":    "gamegear-device",
		"runtimeProfile": "gamegear/genesis-plus-gx",
	}, "Shining Force - The Sword of Hajya (USA).srm", buildNonBlankPayload(26752, 0x05))

	save := mustObject(t, body["save"], "save")
	if id := mustString(t, save["id"], "save.id"); id == "" {
		t.Fatalf("no save id: %s", prettyJSON(save))
	}
}

// RetroDECK writes the DeSmuME container as a .sav. The footer decides the
// format, not the extension: a raw backup fails the splitter's own checks and
// falls through, so trying it on any name is safe.
func TestContractAcceptsDeSmuMEDsvUnderASavName(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "nds-helper")

	raw := buildNonBlankPayload(8192, 0x22)
	wrapped, err := buildDeSmuMEDSV(raw)
	if err != nil {
		t.Fatalf("build DeSmuME DSV fixture: %v", err)
	}

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password":   helperKey,
		"rom_sha1":       "nds-rom-sha1",
		"slotName":       "default",
		"system":         "nds",
		"device_type":    "retroarch",
		"fingerprint":    "deck-nds",
		"runtimeProfile": "nds/desmume",
	}, "New Super Mario Bros. (USA, Australia).sav", wrapped)

	save := mustObject(t, body["save"], "save")
	if got := mustString(t, save["sha256"], "save.sha256"); got != payloadSHA256Hex(raw) {
		t.Fatalf("expected the raw backup under the footer, got %s", prettyJSON(save))
	}
}

// DeSmuME appends a 122-byte footer to the raw backup. The format is decided by
// the file, not by what the uploader declared, so a .dsv is split whether or not
// the runtime profile says DeSmuME -- otherwise the raw size check sees the
// footer and refuses a save the client can read perfectly well.
func TestContractAcceptsDeSmuMEDsvWithoutTheProfile(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "nds-helper")

	raw := buildNonBlankPayload(262144, 0x11)
	dsv, err := buildDeSmuMEDSV(raw)
	if err != nil {
		t.Fatalf("build DeSmuME DSV fixture: %v", err)
	}

	body := uploadSave(t, h, "/saves", map[string]string{
		"app_password":   helperKey,
		"rom_sha1":       "nds-rom-sha1",
		"slotName":       "default",
		"system":         "nds",
		"device_type":    "retroarch",
		"fingerprint":    "deck-nds",
		"runtimeProfile": "nds/melonds",
	}, "Mario Kart DS (USA) (En,Fr,De,Es,It) (Wii U Virtual Console).dsv", dsv)

	save := mustObject(t, body["save"], "save")
	if got := mustString(t, save["sha256"], "save.sha256"); got != payloadSHA256Hex(raw) {
		t.Fatalf("expected the stored payload to be the raw backup without the footer: %s", prettyJSON(save))
	}
}
