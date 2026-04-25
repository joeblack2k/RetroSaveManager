package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	ootSaveFileCount = 3

	ootOffsetDeaths                = 0x0022
	ootOffsetHealthCapacity        = 0x002E
	ootOffsetHealth                = 0x0030
	ootOffsetMagicLevel            = 0x0032
	ootOffsetMagic                 = 0x0033
	ootOffsetRupees                = 0x0034
	ootOffsetIsMagicAcquired       = 0x003A
	ootOffsetIsDoubleMagicAcquired = 0x003C
	ootOffsetIsDoubleDefense       = 0x003D
	ootOffsetInventoryEquipment    = 0x009C
	ootOffsetQuestItems            = 0x00A4
	ootOffsetGoldSkulltulaTokens   = 0x00D0

	ootHealthUnitsPerHeart = 0x10
	ootMagicNormalMeter    = 0x30
	ootMagicDoubleMeter    = 0x60
)

type ootSRAMCheatEditor struct{}

func init() {
	registerCheatEditor(ootSRAMCheatEditor{})
}

type ootParsedSlot struct {
	Present bool
	Block   []byte
}

type ootParsedSRAM struct {
	Payload     []byte
	WordSwapped bool
	Slots       [ootSaveFileCount]ootParsedSlot
}

func (ootSRAMCheatEditor) ID() string {
	return "oot-sram"
}

func (ootSRAMCheatEditor) Read(pack cheatPack, payload []byte) (saveCheatEditorState, error) {
	parsed, err := parseOOTSRAM(payload)
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
		state.SlotValues[ootSlotID(slotIndex)] = ootBuildFieldValues(pack, slot.Block)
	}
	if len(state.SlotValues) == 0 {
		return saveCheatEditorState{}, errors.New("ocarina of time SRAM does not contain a valid save slot")
	}
	return state, nil
}

func (ootSRAMCheatEditor) Apply(pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error) {
	parsed, err := parseOOTSRAM(payload)
	if err != nil {
		return nil, nil, err
	}
	slotIndex, err := ootSlotIndex(slotID)
	if err != nil {
		return nil, nil, err
	}
	slot := parsed.Slots[slotIndex]
	if !slot.Present {
		return nil, nil, fmt.Errorf("save slot %q is not present in this SRAM", slotID)
	}

	block := append([]byte(nil), slot.Block...)
	fieldMap := ootFieldIndex(pack)
	changed := map[string]any{}
	for fieldID, value := range updates {
		field, ok := fieldMap[fieldID]
		if !ok {
			return nil, nil, fmt.Errorf("unknown field %q", fieldID)
		}
		if err := ootApplyField(block, field, value); err != nil {
			return nil, nil, fmt.Errorf("apply %s: %w", fieldID, err)
		}
		changed[fieldID] = value
	}
	ootSetSaveBlockChecksum(block)

	updated := append([]byte(nil), parsed.Payload...)
	primaryOffset := ootSlotOffset(slotIndex, false)
	backupOffset := ootSlotOffset(slotIndex, true)
	copy(updated[primaryOffset:primaryOffset+ootSaveSize], block)
	copy(updated[backupOffset:backupOffset+ootSaveSize], block)
	if parsed.WordSwapped {
		updated = n64Swap32Words(updated)
	}
	return updated, changed, nil
}

func parseOOTSRAM(payload []byte) (*ootParsedSRAM, error) {
	normalized, wordSwapped, ok := ootNormalizePayload(payload)
	if !ok {
		return nil, fmt.Errorf("expected %d-byte Ocarina of Time SRAM with Zelda header", ootSramSize)
	}
	parsed := &ootParsedSRAM{
		Payload:     normalized,
		WordSwapped: wordSwapped,
	}
	validCount := 0
	for slotIndex := 0; slotIndex < ootSaveFileCount; slotIndex++ {
		primary := normalized[ootSlotOffset(slotIndex, false) : ootSlotOffset(slotIndex, false)+ootSaveSize]
		backup := normalized[ootSlotOffset(slotIndex, true) : ootSlotOffset(slotIndex, true)+ootSaveSize]
		switch {
		case ootVerifySaveBlock(primary):
			parsed.Slots[slotIndex] = ootParsedSlot{Present: true, Block: append([]byte(nil), primary...)}
			validCount++
		case ootVerifySaveBlock(backup):
			parsed.Slots[slotIndex] = ootParsedSlot{Present: true, Block: append([]byte(nil), backup...)}
			validCount++
		}
	}
	if validCount == 0 {
		return nil, errors.New("ocarina of time SRAM does not contain a valid checksummed save slot")
	}
	return parsed, nil
}

func ootBuildFieldValues(pack cheatPack, block []byte) map[string]any {
	values := map[string]any{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			switch field.Op.Kind {
			case "ootNumber":
				if value, ok := ootReadNumber(block, field.Op.Field); ok {
					values[field.ID] = value
				}
			case "ootBoolean":
				if value, ok := ootReadBoolean(block, field.Op.Field); ok {
					values[field.ID] = value
				}
			case "ootMagicLevel":
				values[field.ID] = ootMagicLevelID(block[ootOffsetMagicLevel])
			case "ootQuestItems":
				values[field.ID] = ootBitsToSelection(field.Bits, binary.BigEndian.Uint32(block[ootOffsetQuestItems:ootOffsetQuestItems+4]))
			case "ootEquipment":
				equipment := binary.BigEndian.Uint16(block[ootOffsetInventoryEquipment : ootOffsetInventoryEquipment+2])
				if shift, ok := ootEquipmentShift(field.Op.Field); ok {
					values[field.ID] = ootBitsToSelection(field.Bits, uint32((equipment>>shift)&0xF))
				}
			}
		}
	}
	return values
}

func ootFieldIndex(pack cheatPack) map[string]cheatField {
	fields := map[string]cheatField{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			fields[field.ID] = field
		}
	}
	return fields
}

func ootApplyField(block []byte, field cheatField, value any) error {
	switch field.Op.Kind {
	case "ootNumber":
		intValue, ok := value.(int)
		if !ok {
			return errors.New("expected integer")
		}
		return ootWriteNumber(block, field.Op.Field, intValue)
	case "ootBoolean":
		boolValue, ok := value.(bool)
		if !ok {
			return errors.New("expected boolean")
		}
		return ootWriteBoolean(block, field.Op.Field, boolValue)
	case "ootMagicLevel":
		raw, ok := value.(string)
		if !ok {
			return errors.New("expected enum string")
		}
		return ootWriteMagicLevel(block, raw)
	case "ootQuestItems":
		values, err := normalizeCheatStringArray(value)
		if err != nil {
			return err
		}
		current := binary.BigEndian.Uint32(block[ootOffsetQuestItems : ootOffsetQuestItems+4])
		mask := ootFieldBitMask(field.Bits)
		next := (current &^ mask) | (ootSelectionToBits(field.Bits, values) & mask)
		binary.BigEndian.PutUint32(block[ootOffsetQuestItems:ootOffsetQuestItems+4], next)
		return nil
	case "ootEquipment":
		values, err := normalizeCheatStringArray(value)
		if err != nil {
			return err
		}
		shift, ok := ootEquipmentShift(field.Op.Field)
		if !ok {
			return fmt.Errorf("unknown equipment field %q", field.Op.Field)
		}
		equipment := binary.BigEndian.Uint16(block[ootOffsetInventoryEquipment : ootOffsetInventoryEquipment+2])
		group := uint16(ootSelectionToBits(field.Bits, values) & 0xF)
		equipment = (equipment &^ (0xF << shift)) | (group << shift)
		binary.BigEndian.PutUint16(block[ootOffsetInventoryEquipment:ootOffsetInventoryEquipment+2], equipment)
		return nil
	default:
		return fmt.Errorf("unsupported op kind %q", field.Op.Kind)
	}
}

func ootReadNumber(block []byte, field string) (int, bool) {
	switch strings.TrimSpace(field) {
	case "deaths":
		return int(binary.BigEndian.Uint16(block[ootOffsetDeaths : ootOffsetDeaths+2])), true
	case "maxHearts":
		return int(binary.BigEndian.Uint16(block[ootOffsetHealthCapacity:ootOffsetHealthCapacity+2])) / ootHealthUnitsPerHeart, true
	case "currentHearts":
		return int(binary.BigEndian.Uint16(block[ootOffsetHealth:ootOffsetHealth+2])) / ootHealthUnitsPerHeart, true
	case "currentMagic":
		return int(block[ootOffsetMagic]), true
	case "rupees":
		return int(binary.BigEndian.Uint16(block[ootOffsetRupees : ootOffsetRupees+2])), true
	case "goldSkulltulaTokens":
		return int(binary.BigEndian.Uint16(block[ootOffsetGoldSkulltulaTokens : ootOffsetGoldSkulltulaTokens+2])), true
	default:
		return 0, false
	}
}

func ootWriteNumber(block []byte, field string, value int) error {
	switch strings.TrimSpace(field) {
	case "deaths":
		binary.BigEndian.PutUint16(block[ootOffsetDeaths:ootOffsetDeaths+2], uint16(value))
	case "maxHearts":
		binary.BigEndian.PutUint16(block[ootOffsetHealthCapacity:ootOffsetHealthCapacity+2], uint16(value*ootHealthUnitsPerHeart))
		current := int(binary.BigEndian.Uint16(block[ootOffsetHealth:ootOffsetHealth+2])) / ootHealthUnitsPerHeart
		if current > value {
			binary.BigEndian.PutUint16(block[ootOffsetHealth:ootOffsetHealth+2], uint16(value*ootHealthUnitsPerHeart))
		}
	case "currentHearts":
		binary.BigEndian.PutUint16(block[ootOffsetHealth:ootOffsetHealth+2], uint16(value*ootHealthUnitsPerHeart))
	case "currentMagic":
		block[ootOffsetMagic] = byte(value)
	case "rupees":
		binary.BigEndian.PutUint16(block[ootOffsetRupees:ootOffsetRupees+2], uint16(value))
	case "goldSkulltulaTokens":
		binary.BigEndian.PutUint16(block[ootOffsetGoldSkulltulaTokens:ootOffsetGoldSkulltulaTokens+2], uint16(value))
	default:
		return fmt.Errorf("unknown number field %q", field)
	}
	return nil
}

func ootReadBoolean(block []byte, field string) (bool, bool) {
	switch strings.TrimSpace(field) {
	case "doubleDefense":
		return block[ootOffsetIsDoubleDefense] != 0, true
	default:
		return false, false
	}
}

func ootWriteBoolean(block []byte, field string, value bool) error {
	var raw byte
	if value {
		raw = 1
	}
	switch strings.TrimSpace(field) {
	case "doubleDefense":
		block[ootOffsetIsDoubleDefense] = raw
	default:
		return fmt.Errorf("unknown boolean field %q", field)
	}
	return nil
}

func ootMagicLevelID(value byte) string {
	switch value {
	case 1:
		return "normal"
	case 2:
		return "double"
	default:
		return "none"
	}
}

func ootWriteMagicLevel(block []byte, raw string) error {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "none":
		block[ootOffsetMagicLevel] = 0
		block[ootOffsetIsMagicAcquired] = 0
		block[ootOffsetIsDoubleMagicAcquired] = 0
		block[ootOffsetMagic] = 0
	case "normal":
		block[ootOffsetMagicLevel] = 1
		block[ootOffsetIsMagicAcquired] = 1
		block[ootOffsetIsDoubleMagicAcquired] = 0
		if block[ootOffsetMagic] > ootMagicNormalMeter {
			block[ootOffsetMagic] = ootMagicNormalMeter
		}
	case "double":
		block[ootOffsetMagicLevel] = 2
		block[ootOffsetIsMagicAcquired] = 1
		block[ootOffsetIsDoubleMagicAcquired] = 1
		if block[ootOffsetMagic] > ootMagicDoubleMeter {
			block[ootOffsetMagic] = ootMagicDoubleMeter
		}
	default:
		return fmt.Errorf("unsupported magic level %q", raw)
	}
	return nil
}

func ootBitsToSelection(bits []cheatBitOption, value uint32) []string {
	selected := make([]string, 0, len(bits))
	for _, bit := range bits {
		if bit.Bit < 0 || bit.Bit > 31 {
			continue
		}
		if value&(1<<bit.Bit) != 0 {
			selected = append(selected, bit.ID)
		}
	}
	sort.Strings(selected)
	return selected
}

func ootSelectionToBits(bits []cheatBitOption, selected []string) uint32 {
	selectedSet := map[string]struct{}{}
	for _, item := range selected {
		selectedSet[strings.TrimSpace(item)] = struct{}{}
	}
	var out uint32
	for _, bit := range bits {
		if bit.Bit < 0 || bit.Bit > 31 {
			continue
		}
		if _, ok := selectedSet[bit.ID]; ok {
			out |= 1 << bit.Bit
		}
	}
	return out
}

func ootFieldBitMask(bits []cheatBitOption) uint32 {
	var out uint32
	for _, bit := range bits {
		if bit.Bit < 0 || bit.Bit > 31 {
			continue
		}
		out |= 1 << bit.Bit
	}
	return out
}

func ootEquipmentShift(field string) (uint, bool) {
	switch strings.TrimSpace(field) {
	case "swords":
		return 0, true
	case "shields":
		return 4, true
	case "tunics":
		return 8, true
	case "boots":
		return 12, true
	default:
		return 0, false
	}
}

func ootSlotOffset(slotIndex int, backup bool) int {
	if backup {
		slotIndex += ootSaveFileCount
	}
	return ootSlotStartOffset + slotIndex*ootSlotStride
}

func ootSlotID(slotIndex int) string {
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

func ootSlotIndex(slotID string) (int, error) {
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

func ootSetSaveBlockChecksum(block []byte) {
	block[ootChecksumOffset] = 0
	block[ootChecksumOffset+1] = 0
	var checksum uint32
	for i := 0; i < ootSaveSize; i += 2 {
		checksum += uint32(binary.BigEndian.Uint16(block[i : i+2]))
	}
	binary.BigEndian.PutUint16(block[ootChecksumOffset:ootChecksumOffset+2], uint16(checksum&0xffff))
}
