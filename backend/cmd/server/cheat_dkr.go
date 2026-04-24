package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

const (
	dkrEEPROMSize = 0x200
	dkrSlotCount  = 3
	dkrSlotSize   = 40
)

type dkrEEPROMCheatEditor struct{}

type dkrParsedSlot struct {
	Present        bool
	CourseStatus   [34]byte
	TajFlags       byte
	Trophies       [5]byte
	Bosses         uint16
	Balloons       [6]byte
	TTAmulet       byte
	WizpigAmulet   byte
	FlagsWorld     [6]uint16
	Keys           byte
	CutsceneFlags  uint32
	Name           uint16
	Padding        byte
	StoredChecksum uint16
}

type dkrParsedEEPROM struct {
	Payload []byte
	Slots   [dkrSlotCount]dkrParsedSlot
}

var dkrCourseFieldIndex = map[string]int{
	"bluey1":           0,
	"fossilCanyon":     1,
	"pirateLagoon":     2,
	"ancientLake":      3,
	"walrusCove":       4,
	"hotTopVolcano":    5,
	"whaleBay":         6,
	"snowballValley":   7,
	"crescentIsland":   8,
	"fireMountain":     9,
	"everfrostPeak":    10,
	"spaceportAlpha":   11,
	"spacedustAlley":   12,
	"greenwoodVillage": 13,
	"boulderCanyon":    14,
	"windmillPlains":   15,
	"smokeyCastle":     16,
	"darkwaterBeach":   17,
	"iciclePyramid":    18,
	"frostyVillage":    19,
	"jungleFalls":      20,
	"treasureCaves":    21,
	"hauntedWoods":     22,
	"darkmoonCaverns":  23,
	"starCity":         24,
	"wizpig1":          25,
	"tricky1":          26,
	"bubbler1":         27,
	"smokey1":          28,
	"tricky2":          29,
	"bluey2":           30,
	"bubbler2":         31,
	"smokey2":          32,
	"wizpig2":          33,
}

var dkrBalloonFieldIndex = map[string]int{
	"balloonsTotal":          0,
	"balloonsDinoDomain":     1,
	"balloonsSherbetIsland":  2,
	"balloonsSnowflakeMount": 3,
	"balloonsDragonForest":   4,
	"balloonsFutureFunLand":  5,
}

var dkrTrophyFieldIndex = map[string]int{
	"dinoDomainTrophy":     0,
	"sherbetIslandTrophy":  1,
	"snowflakeMountTrophy": 2,
	"dragonForestTrophy":   3,
	"futureFunLandTrophy":  4,
}

var dkrAmuletFieldKind = map[string]string{
	"ttAmulet":     "tt",
	"wizpigAmulet": "wizpig",
}

func (dkrEEPROMCheatEditor) ID() string {
	return "dkr-eeprom"
}

func (dkrEEPROMCheatEditor) Read(pack cheatPack, payload []byte) (saveCheatEditorState, error) {
	window, err := n64SmallEEPROMWindow(payload, "Diddy Kong Racing")
	if err != nil {
		return saveCheatEditorState{}, err
	}
	parsed, err := parseDKREEPROM(window)
	if err != nil {
		return saveCheatEditorState{}, err
	}
	state := saveCheatEditorState{
		SlotValues: map[string]map[string]any{},
	}
	for slotIndex, slot := range parsed.Slots {
		if !slot.Present {
			continue
		}
		state.SlotValues[dkrSlotID(slotIndex)] = dkrBuildFieldValues(pack, slot)
	}
	if len(state.SlotValues) == 0 {
		return saveCheatEditorState{}, errors.New("diddy kong racing EEPROM does not contain a valid save slot")
	}
	return state, nil
}

func (dkrEEPROMCheatEditor) Apply(pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error) {
	window, err := n64SmallEEPROMWindow(payload, "Diddy Kong Racing")
	if err != nil {
		return nil, nil, err
	}
	parsed, err := parseDKREEPROM(window)
	if err != nil {
		return nil, nil, err
	}
	slotIndex, err := dkrSlotIndex(slotID)
	if err != nil {
		return nil, nil, err
	}
	slot := parsed.Slots[slotIndex]
	if !slot.Present {
		return nil, nil, fmt.Errorf("save slot %q is not present in this EEPROM", slotID)
	}
	fieldMap := dkrFieldIndex(pack)
	changed := map[string]any{}
	for fieldID, value := range updates {
		field, ok := fieldMap[fieldID]
		if !ok {
			return nil, nil, fmt.Errorf("unknown field %q", fieldID)
		}
		if err := dkrApplyField(&slot, field, value); err != nil {
			return nil, nil, fmt.Errorf("apply %s: %w", fieldID, err)
		}
		changed[fieldID] = value
	}
	parsed.Slots[slotIndex] = slot
	updated := append([]byte(nil), parsed.Payload...)
	copy(updated[dkrSlotOffset(slotIndex):dkrSlotOffset(slotIndex)+dkrSlotSize], dkrEncodeSlot(slot))
	patched, err := n64PatchSmallEEPROMWindow(payload, updated, "Diddy Kong Racing")
	if err != nil {
		return nil, nil, err
	}
	return patched, changed, nil
}

func parseDKREEPROM(payload []byte) (*dkrParsedEEPROM, error) {
	if len(payload) != dkrEEPROMSize {
		return nil, fmt.Errorf("expected %d-byte Diddy Kong Racing EEPROM, got %d", dkrEEPROMSize, len(payload))
	}
	parsed := &dkrParsedEEPROM{Payload: append([]byte(nil), payload...)}
	validCount := 0
	for slotIndex := 0; slotIndex < dkrSlotCount; slotIndex++ {
		block := payload[dkrSlotOffset(slotIndex) : dkrSlotOffset(slotIndex)+dkrSlotSize]
		if bytes.Equal(block, bytes.Repeat([]byte{0xff}, len(block))) {
			continue
		}
		slot, err := dkrDecodeSlot(block)
		if err != nil {
			continue
		}
		slot.Present = true
		parsed.Slots[slotIndex] = slot
		validCount++
	}
	if validCount == 0 {
		return nil, errors.New("diddy kong racing EEPROM does not contain a valid save slot")
	}
	return parsed, nil
}

func dkrDecodeSlot(block []byte) (dkrParsedSlot, error) {
	if len(block) != dkrSlotSize {
		return dkrParsedSlot{}, fmt.Errorf("expected %d-byte Diddy Kong Racing slot, got %d", dkrSlotSize, len(block))
	}
	cursor := dkrBitCursor{data: block}
	slot := dkrParsedSlot{}
	slot.StoredChecksum = uint16(dkrReadBits(&cursor, 16))
	for i := range slot.CourseStatus {
		slot.CourseStatus[i] = byte(dkrReadBits(&cursor, 2))
	}
	slot.TajFlags = byte(dkrReadBits(&cursor, 6))
	for i := range slot.Trophies {
		slot.Trophies[i] = byte(dkrReadBits(&cursor, 2))
	}
	slot.Bosses = uint16(dkrReadBits(&cursor, 12))
	for i := range slot.Balloons {
		slot.Balloons[i] = byte(dkrReadBits(&cursor, 7))
	}
	slot.TTAmulet = byte(dkrReadBits(&cursor, 3))
	slot.WizpigAmulet = byte(dkrReadBits(&cursor, 3))
	for i := range slot.FlagsWorld {
		slot.FlagsWorld[i] = uint16(dkrReadBits(&cursor, 16))
	}
	slot.Keys = byte(dkrReadBits(&cursor, 8))
	slot.CutsceneFlags = dkrReadBits(&cursor, 32)
	slot.Name = uint16(dkrReadBits(&cursor, 16))
	slot.Padding = byte(dkrReadBits(&cursor, 8))
	if slot.StoredChecksum != dkrSlotChecksum(block) {
		return dkrParsedSlot{}, errors.New("invalid Diddy Kong Racing slot checksum")
	}
	return slot, nil
}

func dkrEncodeSlot(slot dkrParsedSlot) []byte {
	out := make([]byte, dkrSlotSize)
	cursor := dkrBitCursor{data: out}
	dkrWriteBits(&cursor, 16, 0)
	for _, status := range slot.CourseStatus {
		dkrWriteBits(&cursor, 2, uint32(status&0x3))
	}
	dkrWriteBits(&cursor, 6, uint32(slot.TajFlags&0x3f))
	for _, trophy := range slot.Trophies {
		dkrWriteBits(&cursor, 2, uint32(trophy&0x3))
	}
	dkrWriteBits(&cursor, 12, uint32(slot.Bosses&0x0fff))
	for _, balloons := range slot.Balloons {
		dkrWriteBits(&cursor, 7, uint32(balloons&0x7f))
	}
	dkrWriteBits(&cursor, 3, uint32(slot.TTAmulet&0x7))
	dkrWriteBits(&cursor, 3, uint32(slot.WizpigAmulet&0x7))
	for _, flags := range slot.FlagsWorld {
		dkrWriteBits(&cursor, 16, uint32(flags))
	}
	dkrWriteBits(&cursor, 8, uint32(slot.Keys))
	dkrWriteBits(&cursor, 32, slot.CutsceneFlags)
	dkrWriteBits(&cursor, 16, uint32(slot.Name))
	dkrWriteBits(&cursor, 8, uint32(slot.Padding))
	dkrSetSlotChecksum(out)
	return out
}

func dkrBuildFieldValues(pack cheatPack, slot dkrParsedSlot) map[string]any {
	values := map[string]any{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			switch field.Op.Kind {
			case "dkrBalloons":
				index, ok := dkrBalloonFieldIndex[field.Op.Field]
				if !ok {
					continue
				}
				values[field.ID] = int(slot.Balloons[index])
			case "dkrAmulet":
				switch dkrAmuletFieldKind[field.Op.Field] {
				case "tt":
					values[field.ID] = int(slot.TTAmulet)
				case "wizpig":
					values[field.ID] = int(slot.WizpigAmulet)
				}
			case "dkrCourseStatus":
				index, ok := dkrCourseFieldIndex[field.Op.Field]
				if !ok {
					continue
				}
				if statusID, ok := dkrCourseStatusID(slot.CourseStatus[index]); ok {
					values[field.ID] = statusID
				}
			case "dkrTrophy":
				index, ok := dkrTrophyFieldIndex[field.Op.Field]
				if !ok {
					continue
				}
				if trophyID, ok := dkrTrophyID(slot.Trophies[index]); ok {
					values[field.ID] = trophyID
				}
			case "dkrKeys":
				values[field.ID] = dkrBitsToSelection(field.Bits, uint32(slot.Keys))
			case "dkrBosses":
				values[field.ID] = dkrBitsToSelection(field.Bits, uint32(slot.Bosses))
			case "dkrTajFlags":
				values[field.ID] = dkrBitsToSelection(field.Bits, uint32(slot.TajFlags))
			}
		}
	}
	return values
}

func dkrFieldIndex(pack cheatPack) map[string]cheatField {
	fields := map[string]cheatField{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			fields[field.ID] = field
		}
	}
	return fields
}

func dkrApplyField(slot *dkrParsedSlot, field cheatField, value any) error {
	switch field.Op.Kind {
	case "dkrBalloons":
		intValue, ok := value.(int)
		if !ok {
			return errors.New("expected integer")
		}
		index, ok := dkrBalloonFieldIndex[field.Op.Field]
		if !ok {
			return fmt.Errorf("unknown balloon field %q", field.Op.Field)
		}
		slot.Balloons[index] = byte(intValue)
		return nil
	case "dkrAmulet":
		intValue, ok := value.(int)
		if !ok {
			return errors.New("expected integer")
		}
		switch dkrAmuletFieldKind[field.Op.Field] {
		case "tt":
			slot.TTAmulet = byte(intValue)
			return nil
		case "wizpig":
			slot.WizpigAmulet = byte(intValue)
			return nil
		default:
			return fmt.Errorf("unknown amulet field %q", field.Op.Field)
		}
	case "dkrCourseStatus":
		raw, ok := value.(string)
		if !ok {
			return errors.New("expected enum string")
		}
		index, ok := dkrCourseFieldIndex[field.Op.Field]
		if !ok {
			return fmt.Errorf("unknown course field %q", field.Op.Field)
		}
		status, ok := dkrCourseStatusValue(raw)
		if !ok {
			return fmt.Errorf("unsupported course status %q", raw)
		}
		slot.CourseStatus[index] = status
		return nil
	case "dkrTrophy":
		raw, ok := value.(string)
		if !ok {
			return errors.New("expected enum string")
		}
		index, ok := dkrTrophyFieldIndex[field.Op.Field]
		if !ok {
			return fmt.Errorf("unknown trophy field %q", field.Op.Field)
		}
		trophy, ok := dkrTrophyValue(raw)
		if !ok {
			return fmt.Errorf("unsupported trophy value %q", raw)
		}
		slot.Trophies[index] = trophy
		return nil
	case "dkrKeys":
		values, err := normalizeCheatStringArray(value)
		if err != nil {
			return err
		}
		slot.Keys = byte(dkrSelectionToBits(field.Bits, values))
		return nil
	case "dkrBosses":
		values, err := normalizeCheatStringArray(value)
		if err != nil {
			return err
		}
		slot.Bosses = uint16(dkrSelectionToBits(field.Bits, values))
		return nil
	case "dkrTajFlags":
		values, err := normalizeCheatStringArray(value)
		if err != nil {
			return err
		}
		slot.TajFlags = byte(dkrSelectionToBits(field.Bits, values))
		return nil
	default:
		return fmt.Errorf("unsupported op kind %q", field.Op.Kind)
	}
}

func dkrCourseStatusID(value byte) (string, bool) {
	switch value & 0x3 {
	case 0:
		return "none", true
	case 1:
		return "visited", true
	case 2:
		return "completed", true
	case 3:
		return "silver", true
	default:
		return "", false
	}
}

func dkrCourseStatusValue(raw string) (byte, bool) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "none":
		return 0, true
	case "visited":
		return 1, true
	case "completed":
		return 2, true
	case "silver":
		return 3, true
	default:
		return 0, false
	}
}

func dkrTrophyID(value byte) (string, bool) {
	switch value & 0x3 {
	case 0:
		return "none", true
	case 1:
		return "thirdPlace", true
	case 2:
		return "secondPlace", true
	case 3:
		return "firstPlace", true
	default:
		return "", false
	}
}

func dkrTrophyValue(raw string) (byte, bool) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "none":
		return 0, true
	case "thirdplace":
		return 1, true
	case "secondplace":
		return 2, true
	case "firstplace":
		return 3, true
	default:
		return 0, false
	}
}

func dkrBitsToSelection(bits []cheatBitOption, value uint32) []string {
	selected := make([]string, 0, len(bits))
	for _, bit := range bits {
		if bit.Bit < 0 || bit.Bit > 31 {
			continue
		}
		if value&(1<<bit.Bit) != 0 {
			selected = append(selected, bit.ID)
		}
	}
	return selected
}

func dkrSelectionToBits(bits []cheatBitOption, selected []string) uint32 {
	lookup := map[string]cheatBitOption{}
	for _, bit := range bits {
		lookup[bit.ID] = bit
	}
	var value uint32
	for _, id := range selected {
		bit, ok := lookup[id]
		if !ok || bit.Bit < 0 || bit.Bit > 31 {
			continue
		}
		value |= 1 << bit.Bit
	}
	return value
}

func dkrSlotOffset(slotIndex int) int {
	return slotIndex * dkrSlotSize
}

func dkrSlotID(slotIndex int) string {
	switch slotIndex {
	case 0:
		return "A"
	case 1:
		return "B"
	case 2:
		return "C"
	default:
		return ""
	}
}

func dkrSlotIndex(slotID string) (int, error) {
	switch strings.TrimSpace(strings.ToUpper(slotID)) {
	case "A":
		return 0, nil
	case "B":
		return 1, nil
	case "C":
		return 2, nil
	default:
		return 0, fmt.Errorf("unknown save slot %q", slotID)
	}
}

func dkrSlotChecksum(block []byte) uint16 {
	var checksum uint16 = 5
	for _, value := range block[2:] {
		checksum += uint16(value)
	}
	return checksum
}

func dkrSetSlotChecksum(block []byte) {
	checksum := dkrSlotChecksum(block)
	cursor := dkrBitCursor{data: block}
	dkrWriteBits(&cursor, 16, uint32(checksum))
}

type dkrBitCursor struct {
	data  []byte
	index int
	value byte
	mask  byte
}

func dkrReadBits(cursor *dkrBitCursor, bitCount int) uint32 {
	if bitCount <= 0 {
		return 0
	}
	var out uint32
	bit := uint32(1 << (bitCount - 1))
	for bitCount > 0 {
		if cursor.mask == 0 {
			cursor.value = cursor.data[cursor.index]
			cursor.index++
			cursor.mask = 0x80
		}
		if cursor.value&cursor.mask != 0 {
			out |= bit
		}
		bit >>= 1
		cursor.mask >>= 1
		bitCount--
	}
	return out
}

func dkrWriteBits(cursor *dkrBitCursor, bitCount int, value uint32) {
	if bitCount <= 0 {
		return
	}
	bit := uint32(1 << (bitCount - 1))
	for bitCount > 0 {
		if cursor.mask == 0 {
			if cursor.index > 0 {
				cursor.data[cursor.index-1] = cursor.value
			}
			cursor.value = 0
			cursor.mask = 0x80
			cursor.index++
		}
		if value&bit != 0 {
			cursor.value |= cursor.mask
		}
		bit >>= 1
		cursor.mask >>= 1
		bitCount--
	}
	if cursor.index > 0 {
		cursor.data[cursor.index-1] = cursor.value
	}
}
