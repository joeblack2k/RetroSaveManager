package main

import (
	"encoding/binary"
	"net/http"
	"testing"
)

func TestParseDreamcastVMSPayloadExtractsHeaderIconAndEyecatch(t *testing.T) {
	payload := buildDreamcastPackageFixture()
	details := parseDreamcastContainer("sonic.vms", payload)
	if details == nil {
		t.Fatal("expected Dreamcast package details")
	}
	if details.Container != "vms" {
		t.Fatalf("unexpected container: %q", details.Container)
	}
	if details.SaveEntries != 1 {
		t.Fatalf("expected one save entry, got %d", details.SaveEntries)
	}
	if details.SampleTitle != "SONIC ADV2" {
		t.Fatalf("unexpected sample title: %q", details.SampleTitle)
	}
	if details.SampleAppID != "FLYCAST" {
		t.Fatalf("unexpected sample app id: %q", details.SampleAppID)
	}
	if details.SampleIconDataURL == "" {
		t.Fatal("expected sample icon data URL")
	}
	if details.SampleEyecatchDataURL == "" {
		t.Fatal("expected sample eyecatch data URL")
	}
	if len(details.Entries) != 1 {
		t.Fatalf("expected one parsed entry, got %d", len(details.Entries))
	}
	entry := details.Entries[0]
	if entry.IconCount != 1 {
		t.Fatalf("expected one icon frame, got %d", entry.IconCount)
	}
	if entry.EyecatchTypeLabel != "palette-16" {
		t.Fatalf("unexpected eyecatch label: %q", entry.EyecatchTypeLabel)
	}
	if entry.IconDataURL == "" || entry.EyecatchDataURL == "" {
		t.Fatalf("expected icon + eyecatch previews, got %#v", entry)
	}
}

func TestParseDreamcastVMUImageExtractsEntryMetadata(t *testing.T) {
	payload := buildDreamcastVMUWithSingleSave()
	details := parseDreamcastContainer("Sonic Adventure 2.A1.bin", payload)
	if details == nil {
		t.Fatal("expected Dreamcast VMU details")
	}
	if details.Container != "vmu-bin" {
		t.Fatalf("unexpected container: %q", details.Container)
	}
	if details.SaveEntries != 1 {
		t.Fatalf("expected one save entry, got %d", details.SaveEntries)
	}
	if len(details.Entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(details.Entries))
	}
	entry := details.Entries[0]
	if entry.DirectoryName != "SONICADV2" {
		t.Fatalf("unexpected directory name: %q", entry.DirectoryName)
	}
	if entry.FileType != "data" {
		t.Fatalf("unexpected file type: %q", entry.FileType)
	}
	if entry.ShortDescription != "SONIC ADV2" {
		t.Fatalf("unexpected short description: %q", entry.ShortDescription)
	}
	if entry.IconDataURL == "" {
		t.Fatal("expected entry icon preview")
	}
}

func TestDetectSaveSystemRecognizesDreamcastVMUImage(t *testing.T) {
	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename: "Sonic Adventure 2.A1.bin",
		Payload:  buildDreamcastVMUWithSingleSave(),
	})
	if detected.Slug != "dreamcast" {
		t.Fatalf("expected dreamcast slug, got %q", detected.Slug)
	}
	if detected.System == nil || detected.System.Slug != "dreamcast" {
		t.Fatalf("expected dreamcast system details, got %#v", detected.System)
	}
	if !detected.Evidence.Payload {
		t.Fatalf("expected payload evidence, got %#v", detected.Evidence)
	}
}

func TestDetectSaveSystemRejectsDreamcastNVMEMBlob(t *testing.T) {
	payload := make([]byte, dreamcastVMUSize)
	copy(payload, dreamcastNVMEMMagic)
	detected := detectSaveSystem(saveSystemDetectionInput{
		Filename: "dc_nvmem.bin",
		Payload:  payload,
	})
	if detected.Slug != "unknown-system" || detected.System != nil {
		t.Fatalf("expected NVMEM blob to be rejected, got slug=%q system=%#v", detected.Slug, detected.System)
	}
}

func TestNormalizeSaveInputAcceptsDreamcastVMUAndSetsParserMetadata(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename: "Sonic Adventure 2.A1.bin",
		Payload:  buildDreamcastVMUWithSingleSave(),
		Game:     game{Name: "Sonic Adventure 2"},
		SlotName: "A1",
	})
	if result.Rejected {
		t.Fatalf("expected Dreamcast VMU to be accepted, got reject=%q", result.RejectReason)
	}
	if result.Input.SystemSlug != "dreamcast" {
		t.Fatalf("expected dreamcast system slug, got %q", result.Input.SystemSlug)
	}
	if result.Input.Dreamcast == nil {
		t.Fatal("expected Dreamcast metadata on normalized input")
	}
	if result.Input.Dreamcast.SlotName != "A1" {
		t.Fatalf("expected Dreamcast slot A1, got %q", result.Input.Dreamcast.SlotName)
	}
	if result.Input.CoverArtURL == "" {
		t.Fatal("expected Dreamcast icon cover art URL")
	}
	if !result.Input.Game.HasParser {
		t.Fatal("expected parser flag to be set")
	}
	if result.Input.Inspection == nil {
		t.Fatal("expected Dreamcast inspection metadata")
	}
	if result.Input.Inspection.ParserLevel != saveParserLevelStructural {
		t.Fatalf("unexpected parser level: %q", result.Input.Inspection.ParserLevel)
	}
	if result.Input.Inspection.ParserID != "dreamcast-vmu" {
		t.Fatalf("unexpected parser id: %q", result.Input.Inspection.ParserID)
	}
	if result.Input.DisplayTitle != "SONIC ADV2" {
		t.Fatalf("expected parser title, got %q", result.Input.DisplayTitle)
	}
	meta, ok := result.Input.Metadata.(map[string]any)
	if !ok {
		t.Fatalf("expected metadata map, got %#v", result.Input.Metadata)
	}
	rsm := mustObject(t, meta["rsm"], "metadata.rsm")
	if _, ok := rsm["dreamcast"]; !ok {
		t.Fatalf("expected dreamcast metadata under rsm, got %v", rsm)
	}
}

func TestNormalizeSaveInputRejectsEmptyDreamcastVMU(t *testing.T) {
	a := &app{}
	result := a.normalizeSaveInputDetailed(saveCreateInput{
		Filename: "Sonic Adventure 2.A1.bin",
		Payload:  buildDreamcastEmptyVMU(),
		Game:     game{Name: "Sonic Adventure 2"},
		SlotName: "A1",
	})
	if !result.Rejected {
		t.Fatal("expected empty Dreamcast VMU to be rejected")
	}
	if result.RejectReason != "dreamcast container has no active save entries" {
		t.Fatalf("unexpected reject reason: %q", result.RejectReason)
	}
}

func TestContractSavesMultipartAcceptsDreamcastVMUImage(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "dreamcast-helper")

	uploadSave(t, h, "/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "dc-line:dreamcast:mister:a1",
		"slotName":     "A1",
		"system":       "dreamcast",
		"device_type":  "mister",
		"fingerprint":  "dreamcast-device",
	}, "Sonic Adventure 2.A1.bin", buildDreamcastVMUWithSingleSave())

	list := h.request(http.MethodGet, "/saves?limit=10&offset=0", nil)
	assertStatus(t, list, http.StatusOK)
	body := decodeJSONMap(t, list.Body)
	items := mustArray(t, body["saves"], "saves")
	if len(items) == 0 {
		t.Fatal("expected uploaded Dreamcast save in list")
	}
	first := mustObject(t, items[0], "items[0]")
	if mustString(t, first["systemSlug"], "items[0].systemSlug") != "dreamcast" {
		t.Fatalf("unexpected system slug: %s", prettyJSON(first))
	}
	if mustString(t, first["coverArtUrl"], "items[0].coverArtUrl") == "" {
		t.Fatalf("expected Dreamcast cover art in list item: %s", prettyJSON(first))
	}
	dreamcast := mustObject(t, first["dreamcast"], "items[0].dreamcast")
	if mustString(t, dreamcast["container"], "items[0].dreamcast.container") != "vmu-bin" {
		t.Fatalf("unexpected Dreamcast container payload: %s", prettyJSON(first))
	}
	inspection := mustObject(t, first["inspection"], "items[0].inspection")
	if mustString(t, inspection["parserLevel"], "items[0].inspection.parserLevel") != saveParserLevelStructural {
		t.Fatalf("unexpected Dreamcast inspection payload: %s", prettyJSON(first))
	}
}

func TestContractSavesMultipartRejectsEmptyDreamcastVMUImage(t *testing.T) {
	h := newContractHarness(t)
	helperKey := createHelperAppPassword(t, h, "", "dreamcast-helper")

	rr := h.multipart("/saves", map[string]string{
		"app_password": helperKey,
		"rom_sha1":     "dc-line:dreamcast:mister:a1",
		"slotName":     "A1",
		"system":       "dreamcast",
		"device_type":  "mister",
		"fingerprint":  "dreamcast-device",
	}, "file", "Sonic Adventure 2.A1.bin", buildDreamcastEmptyVMU())
	assertStatus(t, rr, http.StatusUnprocessableEntity)
	assertJSONContentType(t, rr)
}

func buildDreamcastPackageFixture() []byte {
	payloadData := []byte("dreamcast-save-payload")
	iconPalette := []uint16{
		0x0000,
		0xFF00,
		0xF0F0,
		0xF00F,
	}
	eyecatchPalette := []uint16{
		0x0000,
		0xFFFF,
		0xFF00,
		0xF0F0,
	}
	iconBitmap := make([]byte, dreamcastIconFrameBytes)
	for y := 0; y < dreamcastIconHeight; y++ {
		for x := 0; x < dreamcastIconWidth; x += 2 {
			left := byte(1)
			right := byte(2)
			if (x/2+y)%2 == 0 {
				left, right = 2, 3
			}
			offset := (y*dreamcastIconWidth + x) / 2
			iconBitmap[offset] = (left << 4) | right
		}
	}
	eyecatchBitmap := make([]byte, (dreamcastEyecatchWidth*dreamcastEyecatchHeight)/2)
	for y := 0; y < dreamcastEyecatchHeight; y++ {
		for x := 0; x < dreamcastEyecatchWidth; x += 2 {
			left := byte(1)
			right := byte(2)
			if (x/2+y)%3 == 0 {
				left, right = 3, 1
			}
			offset := (y*dreamcastEyecatchWidth + x) / 2
			eyecatchBitmap[offset] = (left << 4) | right
		}
	}
	eyecatchData := make([]byte, 32+len(eyecatchBitmap))
	for idx, value := range eyecatchPalette {
		binary.LittleEndian.PutUint16(eyecatchData[idx*2:idx*2+2], value)
	}
	copy(eyecatchData[32:], eyecatchBitmap)

	total := dreamcastHeaderSize + dreamcastIconFrameBytes + len(eyecatchData) + len(payloadData)
	out := make([]byte, total)
	writeDreamcastTextField(out[0x00:0x10], "SONIC ADV2", ' ')
	writeDreamcastTextField(out[0x10:0x30], "Sonic Adventure 2", ' ')
	writeDreamcastTextField(out[0x30:0x40], "FLYCAST", 0)
	binary.LittleEndian.PutUint16(out[0x40:0x42], 1)
	binary.LittleEndian.PutUint16(out[0x42:0x44], 8)
	binary.LittleEndian.PutUint16(out[0x44:0x46], 3)
	binary.LittleEndian.PutUint32(out[0x48:0x4C], uint32(len(payloadData)))
	for idx, value := range iconPalette {
		binary.LittleEndian.PutUint16(out[0x60+idx*2:0x60+idx*2+2], value)
	}
	copy(out[dreamcastHeaderSize:dreamcastHeaderSize+dreamcastIconFrameBytes], iconBitmap)
	copy(out[dreamcastHeaderSize+dreamcastIconFrameBytes:], eyecatchData)
	copy(out[dreamcastHeaderSize+dreamcastIconFrameBytes+len(eyecatchData):], payloadData)
	crc := dreamcastCRCForPayload(out, 0)
	binary.LittleEndian.PutUint16(out[0x46:0x48], crc)
	return out
}

func buildDreamcastVMUWithSingleSave() []byte {
	bytes := buildDreamcastEmptyVMU()
	payload := buildDreamcastPackageFixture()
	fileBlocks := (len(payload) + dreamcastBlockSize - 1) / dreamcastBlockSize
	fatOffset := 254 * dreamcastBlockSize
	firstBlock := 10
	for index := 0; index < fileBlocks; index++ {
		current := firstBlock + index
		next := uint16(dreamcastFATEnd)
		if index < fileBlocks-1 {
			next = uint16(current + 1)
		}
		offset := fatOffset + current*2
		binary.LittleEndian.PutUint16(bytes[offset:offset+2], next)
		blockStart := current * dreamcastBlockSize
		copy(bytes[blockStart:blockStart+dreamcastBlockSize], payload[index*dreamcastBlockSize:minInt(len(payload), (index+1)*dreamcastBlockSize)])
	}
	dirOffset := 253 * dreamcastBlockSize
	bytes[dirOffset] = dreamcastFileTypeData
	binary.LittleEndian.PutUint16(bytes[dirOffset+0x02:dirOffset+0x04], uint16(firstBlock))
	copy(bytes[dirOffset+0x04:dirOffset+0x10], []byte("SONICADV2"))
	binary.LittleEndian.PutUint16(bytes[dirOffset+0x18:dirOffset+0x1A], uint16(fileBlocks))
	binary.LittleEndian.PutUint16(bytes[dirOffset+0x1A:dirOffset+0x1C], 0)
	return bytes
}

func buildDreamcastEmptyVMU() []byte {
	bytes := make([]byte, dreamcastVMUSize)
	rootOffset := dreamcastRootBlock * dreamcastBlockSize
	for i := 0; i < 16; i++ {
		bytes[rootOffset+i] = 0x55
	}
	binary.LittleEndian.PutUint16(bytes[rootOffset+0x46:rootOffset+0x48], 254)
	binary.LittleEndian.PutUint16(bytes[rootOffset+0x48:rootOffset+0x4A], 1)
	binary.LittleEndian.PutUint16(bytes[rootOffset+0x4A:rootOffset+0x4C], 253)
	binary.LittleEndian.PutUint16(bytes[rootOffset+0x4C:rootOffset+0x4E], 13)
	binary.LittleEndian.PutUint16(bytes[rootOffset+0x50:rootOffset+0x52], 200)

	fatOffset := 254 * dreamcastBlockSize
	for block := 0; block < dreamcastVMUBlockCount; block++ {
		binary.LittleEndian.PutUint16(bytes[fatOffset+block*2:fatOffset+block*2+2], dreamcastFATFree)
	}
	for block := 253; block >= 241; block-- {
		value := uint16(block - 1)
		if block == 241 {
			value = dreamcastFATEnd
		}
		binary.LittleEndian.PutUint16(bytes[fatOffset+block*2:fatOffset+block*2+2], value)
	}
	binary.LittleEndian.PutUint16(bytes[fatOffset+254*2:fatOffset+254*2+2], dreamcastFATEnd)
	binary.LittleEndian.PutUint16(bytes[fatOffset+255*2:fatOffset+255*2+2], dreamcastFATEnd)
	return bytes
}

func writeDreamcastTextField(dst []byte, value string, pad byte) {
	for i := range dst {
		dst[i] = pad
	}
	copy(dst, []byte(value))
}
