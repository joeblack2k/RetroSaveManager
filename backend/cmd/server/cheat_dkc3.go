package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	dkc3SRAMSize          = 0x800
	dkc3SlotCount         = 3
	dkc3SlotBaseOffset    = 0x62
	dkc3SlotStride        = 0x28a
	dkc3SlotBlockSize     = 0x28a
	dkc3HeaderSize        = 0x06
	dkc3AltDataOffset     = 0x148
	dkc3ChecksumSumOffset = 0x00
	dkc3ChecksumXorOffset = 0x02
	dkc3MarkerOffset      = 0x04
	dkc3ChecksumStart     = 0x06
	dkc3ChecksumEnd       = 0x28a
)

type dkc3SRAMCheatEditor struct{}

func init() {
	registerCheatEditor(dkc3SRAMCheatEditor{})
}

type dkc3ParsedSRAM struct {
	Payload []byte
	Slots   [dkc3SlotCount]*dkc3Slot
}

type dkc3Slot struct {
	Block      []byte
	DataOffset int
}

type dkc3CounterSpec struct {
	Offset int
	Max    int
}

var dkc3CounterSpecs = map[string]dkc3CounterSpec{
	"bearCoins":   {Offset: 0x13, Max: 99},
	"bonusCoins":  {Offset: 0x15, Max: 85},
	"bananaBirds": {Offset: 0x17, Max: 15},
	"dkCoins":     {Offset: 0x19, Max: 41},
}

func (dkc3SRAMCheatEditor) ID() string {
	return "dkc3-sram"
}

func (dkc3SRAMCheatEditor) Read(pack cheatPack, payload []byte) (saveCheatEditorState, error) {
	parsed, err := parseDKC3SRAM(payload)
	if err != nil {
		return saveCheatEditorState{}, err
	}
	state := saveCheatEditorState{
		SlotValues: map[string]map[string]any{},
	}
	for slotIndex, slot := range parsed.Slots {
		if slot == nil {
			continue
		}
		state.SlotValues[dkc3SlotID(slotIndex)] = dkc3BuildFieldValues(pack, slot)
	}
	if len(state.SlotValues) == 0 {
		return saveCheatEditorState{}, errors.New("donkey kong country 3 SRAM does not contain a valid save slot")
	}
	return state, nil
}

func (dkc3SRAMCheatEditor) Apply(pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error) {
	parsed, err := parseDKC3SRAM(payload)
	if err != nil {
		return nil, nil, err
	}
	slotIndex, err := dkc3SlotIndex(slotID)
	if err != nil {
		return nil, nil, err
	}
	slot := parsed.Slots[slotIndex]
	if slot == nil {
		return nil, nil, fmt.Errorf("save slot %q is not present in this SRAM", slotID)
	}
	fieldMap := dkc3FieldIndex(pack)
	changed := map[string]any{}
	for fieldID, value := range updates {
		field, ok := fieldMap[fieldID]
		if !ok {
			return nil, nil, fmt.Errorf("unknown field %q", fieldID)
		}
		if err := dkc3ApplyField(slot, field, value); err != nil {
			return nil, nil, fmt.Errorf("apply %s: %w", fieldID, err)
		}
		changed[fieldID] = value
	}
	dkc3WriteChecksum(slot.Block)
	updated := append([]byte(nil), parsed.Payload...)
	start := dkc3SlotOffset(slotIndex)
	copy(updated[start:start+dkc3SlotBlockSize], slot.Block)
	return updated, changed, nil
}

func parseDKC3SRAM(payload []byte) (*dkc3ParsedSRAM, error) {
	if len(payload) != dkc3SRAMSize {
		return nil, fmt.Errorf("expected %d-byte Donkey Kong Country 3 SRAM, got %d", dkc3SRAMSize, len(payload))
	}
	parsed := &dkc3ParsedSRAM{Payload: append([]byte(nil), payload...)}
	validCount := 0
	for slotIndex := 0; slotIndex < dkc3SlotCount; slotIndex++ {
		start := dkc3SlotOffset(slotIndex)
		block := append([]byte(nil), payload[start:start+dkc3SlotBlockSize]...)
		dataOffset, ok := dkc3VerifyBlock(slotIndex, block)
		if !ok {
			continue
		}
		parsed.Slots[slotIndex] = &dkc3Slot{Block: block, DataOffset: dataOffset}
		validCount++
	}
	if validCount == 0 {
		return nil, errors.New("donkey kong country 3 SRAM does not contain a valid checksummed save slot")
	}
	return parsed, nil
}

func dkc3VerifyBlock(slotIndex int, block []byte) (int, bool) {
	if len(block) != dkc3SlotBlockSize {
		return 0, false
	}
	marker := binary.LittleEndian.Uint16(block[dkc3MarkerOffset : dkc3MarkerOffset+2])
	expectedMarker := uint16(slotIndex<<8) | 0x52
	if marker&0xfffe != expectedMarker {
		return 0, false
	}
	sum, xor := dkc3BlockChecksum(block)
	if binary.LittleEndian.Uint16(block[dkc3ChecksumSumOffset:dkc3ChecksumSumOffset+2]) != sum {
		return 0, false
	}
	if binary.LittleEndian.Uint16(block[dkc3ChecksumXorOffset:dkc3ChecksumXorOffset+2]) != xor {
		return 0, false
	}
	dataOffset := dkc3HeaderSize
	if slotIndex == 2 && marker&0x0001 != 0 {
		dataOffset = dkc3AltDataOffset
	}
	if dataOffset+0xdd >= len(block) {
		return 0, false
	}
	return dataOffset, true
}

func dkc3BlockChecksum(block []byte) (uint16, uint16) {
	var sum uint16
	var xor uint16
	for offset := dkc3ChecksumStart; offset < dkc3ChecksumEnd; offset += 2 {
		word := binary.LittleEndian.Uint16(block[offset : offset+2])
		sum += word
		xor ^= word
	}
	if sum == 0 {
		sum = 1
	}
	return sum, xor
}

func dkc3WriteChecksum(block []byte) {
	sum, xor := dkc3BlockChecksum(block)
	binary.LittleEndian.PutUint16(block[dkc3ChecksumSumOffset:dkc3ChecksumSumOffset+2], sum)
	binary.LittleEndian.PutUint16(block[dkc3ChecksumXorOffset:dkc3ChecksumXorOffset+2], xor)
}

func dkc3BuildFieldValues(pack cheatPack, slot *dkc3Slot) map[string]any {
	values := map[string]any{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			switch field.Op.Kind {
			case "dkc3Counter":
				spec, ok := dkc3CounterSpecs[field.Op.Field]
				if !ok {
					continue
				}
				values[field.ID] = int(binary.LittleEndian.Uint16(slot.Block[slot.DataOffset+spec.Offset : slot.DataOffset+spec.Offset+2]))
			}
		}
	}
	return values
}

func dkc3FieldIndex(pack cheatPack) map[string]cheatField {
	fields := map[string]cheatField{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			fields[field.ID] = field
		}
	}
	return fields
}

func dkc3ApplyField(slot *dkc3Slot, field cheatField, value any) error {
	switch field.Op.Kind {
	case "dkc3Counter":
		raw, ok := value.(int)
		if !ok {
			return errors.New("expected integer")
		}
		spec, ok := dkc3CounterSpecs[field.Op.Field]
		if !ok {
			return fmt.Errorf("unknown DKC3 counter field %q", field.Op.Field)
		}
		if raw < 0 || raw > spec.Max {
			return fmt.Errorf("counter must be between 0 and %d", spec.Max)
		}
		binary.LittleEndian.PutUint16(slot.Block[slot.DataOffset+spec.Offset:slot.DataOffset+spec.Offset+2], uint16(raw))
		return nil
	default:
		return fmt.Errorf("unsupported op kind %q", field.Op.Kind)
	}
}

func dkc3SlotOffset(index int) int {
	return dkc3SlotBaseOffset + index*dkc3SlotStride
}

func dkc3SlotID(index int) string {
	return strconv.Itoa(index + 1)
}

func dkc3SlotIndex(slotID string) (int, error) {
	normalized := strings.TrimSpace(slotID)
	index, err := strconv.Atoi(normalized)
	if err != nil || index < 1 || index > dkc3SlotCount {
		return 0, fmt.Errorf("unsupported save slot %q", slotID)
	}
	return index - 1, nil
}
