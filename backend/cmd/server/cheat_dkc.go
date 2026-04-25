package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"strconv"
	"strings"
)

const (
	dkcSRAMSize             = 0x800
	dkcSlotCount            = 3
	dkcSlotStride           = 0x2a8
	dkcPrimaryBlockSize     = 0x154
	dkcMagicOffset          = 0x0c
	dkcChecksumOffset       = 0x08
	dkcChecksumControlStart = 0x0c
	dkcChecksumControlEnd   = 0x11e
	dkcValidMagic           = 0x52455241
)

type dkcSRAMCheatEditor struct{}

func init() {
	registerCheatEditor(dkcSRAMCheatEditor{})
}

type dkcParsedSRAM struct {
	Payload []byte
	Slots   [dkcSlotCount][]byte
}

type dkcProgressionSpec struct {
	Offset       int
	MirrorOffset int
}

var dkcProgressionSpecs = map[string]dkcProgressionSpec{
	"jungleHijinxs":       {Offset: 0x2e},
	"ropeyRampage":        {Offset: 0x24},
	"reptileRumble":       {Offset: 0x19},
	"coralCapers":         {Offset: 0xd7},
	"barrelCannonCanyon":  {Offset: 0x2f},
	"veryGnawtysLair":     {Offset: 0xf8, MirrorOffset: 0x104},
	"winkysWalkway":       {Offset: 0xf1},
	"mineCartCarnage":     {Offset: 0x46},
	"bouncyBonanza":       {Offset: 0x1f},
	"stopAndGoStation":    {Offset: 0x49},
	"millstoneMayhem":     {Offset: 0x5a},
	"neckysNuts":          {Offset: 0xf9, MirrorOffset: 0x105},
	"vultureCulture":      {Offset: 0xbd},
	"treeTopTown":         {Offset: 0xbc},
	"forestFrenzy":        {Offset: 0xe8},
	"templeTempest":       {Offset: 0x5b},
	"orangUtanGang":       {Offset: 0x25},
	"clamCity":            {Offset: 0xf6},
	"bumbleBRumble":       {Offset: 0xfd, MirrorOffset: 0x101},
	"snowBarrelBlast":     {Offset: 0x3c},
	"slipslideRide":       {Offset: 0x85},
	"iceAgeAlley":         {Offset: 0xbf},
	"croctopusChase":      {Offset: 0x56},
	"torchlightTrouble":   {Offset: 0x2c},
	"ropeBridgeRumble":    {Offset: 0xe6},
	"reallyGnawtyRampage": {Offset: 0xfa, MirrorOffset: 0x100},
	"oilDrumAlley":        {Offset: 0x58},
	"trickTrackTrek":      {Offset: 0x47},
	"elevatorAntics":      {Offset: 0x30},
	"poisonPond":          {Offset: 0x3a},
	"mineCartMadness":     {Offset: 0x3f},
	"blackoutBasement":    {Offset: 0x59},
	"bossDumbDrum":        {Offset: 0xfb, MirrorOffset: 0xff},
	"tankedUpTrouble":     {Offset: 0x48},
	"manicMincers":        {Offset: 0x2a},
	"mistyMine":           {Offset: 0x22},
	"loopyLights":         {Offset: 0x4e},
	"platformPerils":      {Offset: 0x43},
	"neckysRevenge":       {Offset: 0xfc, MirrorOffset: 0xfe},
	"gangPlankGalleon":    {Offset: 0x76, MirrorOffset: 0x80},
}

func (dkcSRAMCheatEditor) ID() string {
	return "dkc-sram"
}

func (dkcSRAMCheatEditor) Read(pack cheatPack, payload []byte) (saveCheatEditorState, error) {
	parsed, err := parseDKCSRAM(payload)
	if err != nil {
		return saveCheatEditorState{}, err
	}
	state := saveCheatEditorState{
		SlotValues: map[string]map[string]any{},
	}
	for slotIndex, block := range parsed.Slots {
		if block == nil {
			continue
		}
		state.SlotValues[dkcSlotID(slotIndex)] = dkcBuildFieldValues(pack, block)
	}
	if len(state.SlotValues) == 0 {
		return saveCheatEditorState{}, errors.New("donkey kong country SRAM does not contain a valid save slot")
	}
	return state, nil
}

func (dkcSRAMCheatEditor) Apply(pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error) {
	parsed, err := parseDKCSRAM(payload)
	if err != nil {
		return nil, nil, err
	}
	slotIndex, err := dkcSlotIndex(slotID)
	if err != nil {
		return nil, nil, err
	}
	block := parsed.Slots[slotIndex]
	if block == nil {
		return nil, nil, fmt.Errorf("save slot %q is not present in this SRAM", slotID)
	}
	fieldMap := dkcFieldIndex(pack)
	changed := map[string]any{}
	progressionChanged := false
	for fieldID, value := range updates {
		field, ok := fieldMap[fieldID]
		if !ok {
			return nil, nil, fmt.Errorf("unknown field %q", fieldID)
		}
		changedField, err := dkcApplyField(block, field, value)
		if err != nil {
			return nil, nil, fmt.Errorf("apply %s: %w", fieldID, err)
		}
		progressionChanged = progressionChanged || changedField
		changed[fieldID] = value
	}
	if progressionChanged {
		dkcRecomputeCompletion(block)
	}
	dkcWriteChecksum(block)
	updated := append([]byte(nil), parsed.Payload...)
	start := dkcSlotOffset(slotIndex)
	copy(updated[start:start+dkcPrimaryBlockSize], block)
	return updated, changed, nil
}

func parseDKCSRAM(payload []byte) (*dkcParsedSRAM, error) {
	if len(payload) != dkcSRAMSize {
		return nil, fmt.Errorf("expected %d-byte Donkey Kong Country SRAM, got %d", dkcSRAMSize, len(payload))
	}
	parsed := &dkcParsedSRAM{Payload: append([]byte(nil), payload...)}
	validCount := 0
	for slotIndex := 0; slotIndex < dkcSlotCount; slotIndex++ {
		start := dkcSlotOffset(slotIndex)
		block := append([]byte(nil), payload[start:start+dkcPrimaryBlockSize]...)
		if !dkcVerifyBlock(block) {
			continue
		}
		parsed.Slots[slotIndex] = block
		validCount++
	}
	if validCount == 0 {
		return nil, errors.New("donkey kong country SRAM does not contain a valid primary save slot")
	}
	return parsed, nil
}

func dkcVerifyBlock(block []byte) bool {
	if len(block) != dkcPrimaryBlockSize {
		return false
	}
	if binary.LittleEndian.Uint32(block[dkcMagicOffset:dkcMagicOffset+4]) != dkcValidMagic {
		return false
	}
	return binary.LittleEndian.Uint32(block[dkcChecksumOffset:dkcChecksumOffset+4]) == dkcBlockChecksum(block)
}

func dkcBlockChecksum(block []byte) uint32 {
	var checksumHigh uint32
	var checksumLow uint16
	for offset := dkcChecksumControlStart; offset < dkcChecksumControlEnd; offset += 2 {
		word := binary.LittleEndian.Uint16(block[offset : offset+2])
		checksumHigh += uint32(word)
		checksumLow ^= word
	}
	return (checksumHigh << 16) | uint32(checksumLow)
}

func dkcWriteChecksum(block []byte) {
	binary.LittleEndian.PutUint32(block[dkcChecksumOffset:dkcChecksumOffset+4], dkcBlockChecksum(block))
}

func dkcBuildFieldValues(pack cheatPack, block []byte) map[string]any {
	values := map[string]any{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			switch field.Op.Kind {
			case "dkcProgression":
				spec, ok := dkcProgressionSpecs[field.Op.Field]
				if !ok {
					continue
				}
				if progressionID, ok := dkcProgressionID(block[spec.Offset]); ok {
					values[field.ID] = progressionID
				}
			}
		}
	}
	return values
}

func dkcFieldIndex(pack cheatPack) map[string]cheatField {
	fields := map[string]cheatField{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			fields[field.ID] = field
		}
	}
	return fields
}

func dkcApplyField(block []byte, field cheatField, value any) (bool, error) {
	switch field.Op.Kind {
	case "dkcProgression":
		raw, ok := value.(string)
		if !ok {
			return false, errors.New("expected enum string")
		}
		spec, ok := dkcProgressionSpecs[field.Op.Field]
		if !ok {
			return false, fmt.Errorf("unknown progression field %q", field.Op.Field)
		}
		progression, ok := dkcProgressionValue(raw)
		if !ok {
			return false, fmt.Errorf("unsupported progression value %q", raw)
		}
		block[spec.Offset] = (block[spec.Offset] & 0x7e) | progression
		if spec.MirrorOffset > 0 {
			block[spec.MirrorOffset] = progression
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported op kind %q", field.Op.Kind)
	}
}

func dkcRecomputeCompletion(block []byte) {
	percent := 0
	for offset := 1; offset < 0xe2; offset++ {
		if offset == 0x6a {
			continue
		}
		percent += bits.OnesCount8(block[offset] & 0x7f)
	}
	block[0] = byte(percent)
}

func dkcProgressionID(value byte) (string, bool) {
	switch value & 0x81 {
	case 0x00:
		return "none", true
	case 0x01:
		return "donkey", true
	case 0x81:
		return "diddy", true
	default:
		return "", false
	}
}

func dkcProgressionValue(id string) (byte, bool) {
	switch strings.TrimSpace(strings.ToLower(id)) {
	case "none":
		return 0x00, true
	case "donkey":
		return 0x01, true
	case "diddy":
		return 0x81, true
	default:
		return 0x00, false
	}
}

func dkcSlotOffset(index int) int {
	return index * dkcSlotStride
}

func dkcSlotID(index int) string {
	return strconv.Itoa(index + 1)
}

func dkcSlotIndex(slotID string) (int, error) {
	normalized := strings.TrimSpace(slotID)
	index, err := strconv.Atoi(normalized)
	if err != nil || index < 1 || index > dkcSlotCount {
		return 0, fmt.Errorf("unsupported save slot %q", slotID)
	}
	return index - 1, nil
}
