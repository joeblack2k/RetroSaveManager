package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	genesisSonicParserID = "genesis-sonic-sram"

	sonicS3RegionOffset  = 0x0b4
	sonicS3BackupOffset  = 0x0fa
	sonicS3RegionSize    = 0x034
	sonicS3SlotSize      = 0x008
	sonicS3SlotCount     = 6
	sonicS3IntegrityWord = 0x4244

	sonicSKRegionOffset  = 0x140
	sonicSKBackupOffset  = 0x196
	sonicSKRegionSize    = 0x054
	sonicSKSlotSize      = 0x00a
	sonicSKSlotCount     = 8
	sonicSKIntegrityWord = 0x4244
)

type sonicParsedSave struct {
	Variant       string
	Title         string
	GameID        string
	PrimaryValid  bool
	BackupValid   bool
	ActiveSlots   []sonicSaveSlot
	ChecksumValid bool
}

type sonicSaveSlot struct {
	Index         int
	Character     string
	LevelCode     int
	Zone          string
	Act           string
	Stage         string
	Status        string
	Lives         *int
	Continues     *int
	ChaosEmeralds int
	SuperEmeralds int
}

func validateGenesisSonicSave(input saveCreateInput, base *saveInspection) (*saveInspection, bool) {
	if base == nil {
		return nil, false
	}
	parsed, ok := parseGenesisSonicSave(input.Payload)
	if !ok || len(parsed.ActiveSlots) == 0 {
		return nil, false
	}

	primary := parsed.ActiveSlots[0]
	activeSlots := make([]int, 0, len(parsed.ActiveSlots))
	slotSummaries := make([]string, 0, len(parsed.ActiveSlots))
	for _, slot := range parsed.ActiveSlots {
		activeSlots = append(activeSlots, slot.Index)
		summary := fmt.Sprintf("Slot %d: %s", slot.Index, slot.Stage)
		if slot.Character != "" {
			summary += " as " + slot.Character
		}
		slotSummaries = append(slotSummaries, summary)
	}

	extra := map[string]any{
		"family":           "sonic",
		"variant":          parsed.Variant,
		"activeSlots":      activeSlots,
		"slotSummaries":    slotSummaries,
		"currentSlot":      fmt.Sprintf("Slot %d", primary.Index),
		"character":        primary.Character,
		"level":            primary.Stage,
		"stage":            primary.Stage,
		"zone":             primary.Zone,
		"act":              primary.Act,
		"progress":         primary.Status,
		"chaosEmeralds":    primary.ChaosEmeralds,
		"superEmeralds":    primary.SuperEmeralds,
		"primaryCopyValid": parsed.PrimaryValid,
		"backupCopyValid":  parsed.BackupValid,
	}
	if primary.Lives != nil {
		extra["lives"] = *primary.Lives
	} else {
		extra["livesNote"] = "Not stored in Sonic 3 standalone SRAM"
	}
	if primary.Continues != nil {
		extra["continues"] = *primary.Continues
	}

	inspection := cloneSaveInspection(base)
	inspection.ParserLevel = saveParserLevelSemantic
	inspection.ParserID = genesisSonicParserID
	inspection.ValidatedSystem = "genesis"
	inspection.ValidatedGameID = parsed.GameID
	inspection.ValidatedGameTitle = parsed.Title
	inspection.TrustLevel = n64TrustLevelSemanticVerified
	inspection.Evidence = append(cloneEvidence(base.Evidence), "validated Sonic SRAM integrity/checksum", "decoded Sonic save slot progress")
	if parsed.PrimaryValid {
		inspection.Evidence = append(inspection.Evidence, "primary Sonic save copy valid")
	}
	if parsed.BackupValid {
		inspection.Evidence = append(inspection.Evidence, "backup Sonic save copy valid")
	}
	inspection.Warnings = filterSegaRawGenericWarnings(base.Warnings)
	if parsed.Variant == "sonic-3" {
		inspection.Warnings = append(inspection.Warnings, "Sonic 3 standalone does not persist lives in SRAM")
	}
	inspection.PayloadSizeBytes = len(input.Payload)
	inspection.SlotCount = len(activeSlots)
	inspection.ActiveSlotIndexes = activeSlots
	inspection.ChecksumValid = boolPtr(parsed.ChecksumValid)
	inspection.SemanticFields = mergeSemanticFields(base.SemanticFields, extra)
	return inspection, true
}

func parseGenesisSonicSave(payload []byte) (sonicParsedSave, bool) {
	if parsed, ok := parseSonicSKSave(payload); ok {
		return parsed, true
	}
	if parsed, ok := parseSonic3Save(payload); ok {
		return parsed, true
	}
	return sonicParsedSave{}, false
}

func parseSonic3Save(payload []byte) (sonicParsedSave, bool) {
	primary, primaryValid := readSonicRegion(payload, sonicS3RegionOffset, sonicS3RegionSize, sonicS3IntegrityWord)
	backup, backupValid := readSonicRegion(payload, sonicS3BackupOffset, sonicS3RegionSize, sonicS3IntegrityWord)
	if !primaryValid && !backupValid {
		return sonicParsedSave{}, false
	}
	region := primary
	if !primaryValid {
		region = backup
	}
	slots := parseSonic3Slots(region[:sonicS3SlotSize*sonicS3SlotCount])
	if len(slots) == 0 {
		return sonicParsedSave{}, false
	}
	return sonicParsedSave{
		Variant:       "sonic-3",
		Title:         "Sonic The Hedgehog 3",
		GameID:        "genesis/sonic-the-hedgehog-3",
		PrimaryValid:  primaryValid,
		BackupValid:   backupValid,
		ActiveSlots:   slots,
		ChecksumValid: primaryValid || backupValid,
	}, true
}

func parseSonicSKSave(payload []byte) (sonicParsedSave, bool) {
	primary, primaryValid := readSonicRegion(payload, sonicSKRegionOffset, sonicSKRegionSize, sonicSKIntegrityWord)
	backup, backupValid := readSonicRegion(payload, sonicSKBackupOffset, sonicSKRegionSize, sonicSKIntegrityWord)
	if !primaryValid && !backupValid {
		return sonicParsedSave{}, false
	}
	region := primary
	if !primaryValid {
		region = backup
	}
	slots := parseSonicSKSlots(region[:sonicSKSlotSize*sonicSKSlotCount])
	if len(slots) == 0 {
		return sonicParsedSave{}, false
	}
	return sonicParsedSave{
		Variant:       "sonic-3-and-knuckles",
		Title:         "Sonic 3 & Knuckles",
		GameID:        "genesis/sonic-3-and-knuckles",
		PrimaryValid:  primaryValid,
		BackupValid:   backupValid,
		ActiveSlots:   slots,
		ChecksumValid: primaryValid || backupValid,
	}, true
}

func readSonicRegion(payload []byte, offset, size int, integrity uint16) ([]byte, bool) {
	if offset < 0 || size <= 4 || offset+size > len(payload) {
		return nil, false
	}
	region := payload[offset : offset+size]
	if allBytesEqual(region, 0x00) || allBytesEqual(region, 0xff) {
		return nil, false
	}
	if binary.BigEndian.Uint16(region[size-4:size-2]) != integrity {
		return nil, false
	}
	if sonicSRAMChecksum(region[:size-2]) != binary.BigEndian.Uint16(region[size-2:size]) {
		return nil, false
	}
	return append([]byte(nil), region...), true
}

func parseSonic3Slots(data []byte) []sonicSaveSlot {
	slots := make([]sonicSaveSlot, 0, sonicS3SlotCount)
	for idx := 0; idx < sonicS3SlotCount; idx++ {
		slot := data[idx*sonicS3SlotSize : (idx+1)*sonicS3SlotSize]
		if slot[0]&0x80 != 0 {
			continue
		}
		character, ok := sonic3CharacterName(slot[2])
		if !ok {
			continue
		}
		level := int(slot[3])
		zone, act, stage, ok := sonic3StageName(level)
		if !ok {
			continue
		}
		emeralds := countHighBits(slot[6])
		if slot[5] <= 7 {
			emeralds = int(slot[5])
		}
		slots = append(slots, sonicSaveSlot{
			Index:         idx + 1,
			Character:     character,
			LevelCode:     level,
			Zone:          zone,
			Act:           act,
			Stage:         stage,
			Status:        sonicCompletionStatus(slot[0]),
			ChaosEmeralds: emeralds,
		})
	}
	return slots
}

func parseSonicSKSlots(data []byte) []sonicSaveSlot {
	slots := make([]sonicSaveSlot, 0, sonicSKSlotCount)
	for idx := 0; idx < sonicSKSlotCount; idx++ {
		slot := data[idx*sonicSKSlotSize : (idx+1)*sonicSKSlotSize]
		if slot[0]&0x80 != 0 {
			continue
		}
		character, ok := sonicSKCharacterName(slot[2] >> 4)
		if !ok {
			continue
		}
		level := int(slot[3])
		zone, act, stage, ok := sonicSKStageName(level)
		if !ok {
			continue
		}
		chaos, super := sonicSKEmeraldCounts(binary.BigEndian.Uint16(slot[6:8]))
		lives := int(slot[8])
		continues := int(slot[9])
		if lives > 99 || continues > 99 {
			continue
		}
		slots = append(slots, sonicSaveSlot{
			Index:         idx + 1,
			Character:     character,
			LevelCode:     level,
			Zone:          zone,
			Act:           act,
			Stage:         stage,
			Status:        sonicCompletionStatus(slot[0]),
			Lives:         intPtr(lives),
			Continues:     intPtr(continues),
			ChaosEmeralds: chaos,
			SuperEmeralds: super,
		})
	}
	return slots
}

func sonicSRAMChecksum(data []byte) uint16 {
	var checksum uint16
	for idx := 0; idx+1 < len(data); idx += 2 {
		checksum ^= binary.BigEndian.Uint16(data[idx : idx+2])
		carry := checksum&1 != 0
		checksum >>= 1
		if carry {
			checksum ^= 0x8810
		}
	}
	return checksum
}

func sonic3CharacterName(value byte) (string, bool) {
	switch value {
	case 0:
		return "Sonic & Tails", true
	case 1:
		return "Sonic", true
	case 2:
		return "Tails", true
	default:
		return "", false
	}
}

func sonicSKCharacterName(value byte) (string, bool) {
	switch value {
	case 0:
		return "Sonic & Tails", true
	case 1:
		return "Sonic", true
	case 2:
		return "Tails", true
	case 3:
		return "Knuckles", true
	default:
		return "", false
	}
}

func sonic3StageName(level int) (string, string, string, bool) {
	names := map[int]string{
		0: "Angel Island Zone",
		1: "Hydrocity Zone",
		2: "Marble Garden Zone",
		3: "Carnival Night Zone",
		5: "IceCap Zone",
		6: "Launch Base Zone",
	}
	if level == 7 {
		return "Game Complete", "", "Game Complete", true
	}
	zone, ok := names[level]
	if !ok {
		return "", "", "", false
	}
	return zone, "Act 1", zone + " Act 1", true
}

func sonicSKStageName(level int) (string, string, string, bool) {
	names := map[int]string{
		0:  "Angel Island Zone",
		1:  "Hydrocity Zone",
		2:  "Marble Garden Zone",
		3:  "Carnival Night Zone",
		4:  "IceCap Zone",
		5:  "Launch Base Zone",
		6:  "Mushroom Hill Zone",
		7:  "Flying Battery Zone",
		8:  "Sandopolis Zone",
		9:  "Lava Reef Zone",
		10: "Hidden Palace Zone",
		11: "Sky Sanctuary Zone",
		12: "Death Egg Zone",
		13: "The Doomsday Zone",
	}
	zone, ok := names[level]
	if !ok {
		return "", "", "", false
	}
	if level >= 10 {
		return zone, "Act 1", zone + " Act 1", true
	}
	return zone, "Act 1", zone + " Act 1", true
}

func sonicCompletionStatus(value byte) string {
	switch value & 0x03 {
	case 1:
		return "Completed"
	case 2:
		return "Completed with Chaos Emeralds"
	case 3:
		return "Completed with Super Emeralds"
	default:
		return "In progress"
	}
}

func sonicSKEmeraldCounts(value uint16) (int, int) {
	chaos := 0
	super := 0
	for idx := 0; idx < 7; idx++ {
		state := (value >> uint(14-(idx*2))) & 0x03
		if state != 0 {
			chaos++
		}
		if state == 3 {
			super++
		}
	}
	return chaos, super
}

func countHighBits(value byte) int {
	count := 0
	for idx := 0; idx < 8; idx++ {
		if value&(0x80>>idx) != 0 {
			count++
		}
	}
	return count
}

func intPtr(value int) *int {
	return &value
}

func filterSegaRawGenericWarnings(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.Contains(value, "No structural decoder is available yet for this Sega raw save") {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}
