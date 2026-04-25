package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	alttpSRAMSize          = 0x2000
	alttpSlotCount         = 3
	alttpSlotSize          = 0x500
	alttpMirrorBaseOffset  = 0x0f00
	alttpItemBaseOffset    = 0x0340
	alttpChecksumOffset    = 0x04fe
	alttpChecksumTarget    = 0x5a5a
	alttpUSMarkerOffset    = 0x03e5
	alttpJPMarkerOffset    = 0x03e1
	alttpMarkerLow         = 0xaa
	alttpMarkerHigh        = 0x55
	alttpAbilityFlagsField = 0x39
)

type alttpSRAMCheatEditor struct{}

func init() {
	registerCheatEditor(alttpSRAMCheatEditor{})
}

type alttpParsedSRAM struct {
	Payload []byte
	Slots   [alttpSlotCount]*alttpSlot
}

type alttpSlot struct {
	Block      []byte
	Region     string
	SourceBase int
}

type alttpCounterSpec struct {
	Offset         int
	Size           int
	Max            int
	MirrorOffset   int
	ClearOnApply   int
	ConvertFromRaw func(int) int
	ConvertToRaw   func(int) int
}

type alttpEnumSpec struct {
	Offset int
	Values map[string]byte
}

type alttpBoolSpec struct {
	Offset      int
	TrueValue   byte
	AbilityMask byte
}

type alttpBitmaskSpec struct {
	Offset int
}

var alttpCounterSpecs = map[string]alttpCounterSpec{
	"rupees":          {Offset: 0x22, Size: 2, Max: 999, MirrorOffset: 0x20},
	"bombs":           {Offset: 0x03, Size: 1, Max: 50},
	"arrows":          {Offset: 0x37, Size: 1, Max: 70},
	"bombUpgrades":    {Offset: 0x30, Size: 1, Max: 7},
	"arrowUpgrades":   {Offset: 0x31, Size: 1, Max: 7},
	"magicPower":      {Offset: 0x2e, Size: 1, Max: 128, ClearOnApply: 0x33},
	"heartPieces":     {Offset: 0x2b, Size: 1, Max: 3},
	"heartContainers": {Offset: 0x2c, Size: 1, Max: 20, MirrorOffset: 0x2d, ConvertFromRaw: alttpHealthToHearts, ConvertToRaw: alttpHeartsToHealth},
}

var alttpEnumSpecs = map[string]alttpEnumSpec{
	"bow": {
		Offset: 0x00,
		Values: map[string]byte{
			"none":               0,
			"bow":                1,
			"bowAndArrows":       2,
			"silverBow":          3,
			"bowAndSilverArrows": 4,
		},
	},
	"boomerang": {
		Offset: 0x01,
		Values: map[string]byte{
			"none": 0,
			"blue": 1,
			"red":  2,
		},
	},
	"mushroomPowder": {
		Offset: 0x04,
		Values: map[string]byte{
			"none":     0,
			"mushroom": 1,
			"powder":   2,
		},
	},
	"shovelFlute": {
		Offset: 0x0c,
		Values: map[string]byte{
			"none":         0,
			"shovel":       1,
			"flute":        2,
			"fluteAndBird": 3,
		},
	},
	"gloves": {
		Offset: 0x14,
		Values: map[string]byte{
			"none":        0,
			"powerGlove":  1,
			"titansMitts": 2,
		},
	},
	"sword": {
		Offset: 0x19,
		Values: map[string]byte{
			"none":     0,
			"fighter":  1,
			"master":   2,
			"tempered": 3,
			"golden":   4,
		},
	},
	"shield": {
		Offset: 0x1a,
		Values: map[string]byte{
			"none":    0,
			"fighter": 1,
			"red":     2,
			"mirror":  3,
		},
	},
	"armor": {
		Offset: 0x1b,
		Values: map[string]byte{
			"green": 0,
			"blue":  1,
			"red":   2,
		},
	},
	"magicUpgrade": {
		Offset: 0x3b,
		Values: map[string]byte{
			"normal":  0,
			"half":    1,
			"quarter": 2,
		},
	},
	"bottle1": alttpBottleEnumSpec(0x1c),
	"bottle2": alttpBottleEnumSpec(0x1d),
	"bottle3": alttpBottleEnumSpec(0x1e),
	"bottle4": alttpBottleEnumSpec(0x1f),
}

var alttpBoolSpecs = map[string]alttpBoolSpec{
	"hookshot":     {Offset: 0x02, TrueValue: 1},
	"fireRod":      {Offset: 0x05, TrueValue: 1},
	"iceRod":       {Offset: 0x06, TrueValue: 1},
	"bombos":       {Offset: 0x07, TrueValue: 1},
	"ether":        {Offset: 0x08, TrueValue: 1},
	"quake":        {Offset: 0x09, TrueValue: 1},
	"lamp":         {Offset: 0x0a, TrueValue: 1},
	"hammer":       {Offset: 0x0b, TrueValue: 1},
	"bugNet":       {Offset: 0x0d, TrueValue: 1},
	"book":         {Offset: 0x0e, TrueValue: 1},
	"caneSomaria":  {Offset: 0x10, TrueValue: 1},
	"caneByrna":    {Offset: 0x11, TrueValue: 1},
	"magicCape":    {Offset: 0x12, TrueValue: 1},
	"magicMirror":  {Offset: 0x13, TrueValue: 2},
	"pegasusBoots": {Offset: 0x15, TrueValue: 1, AbilityMask: 0x04},
	"zoraFlippers": {Offset: 0x16, TrueValue: 1, AbilityMask: 0x02},
	"moonPearl":    {Offset: 0x17, TrueValue: 1},
}

var alttpBitmaskSpecs = map[string]alttpBitmaskSpec{
	"pendants": {Offset: 0x34},
	"crystals": {Offset: 0x3a},
}

func (alttpSRAMCheatEditor) ID() string {
	return "alttp-sram"
}

func (alttpSRAMCheatEditor) Read(pack cheatPack, payload []byte) (saveCheatEditorState, error) {
	parsed, err := parseALTTPSRAM(payload)
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
		state.SlotValues[alttpSlotID(slotIndex)] = alttpBuildFieldValues(pack, slot.Block)
	}
	if len(state.SlotValues) == 0 {
		return saveCheatEditorState{}, errors.New("a link to the past SRAM does not contain a valid save slot")
	}
	return state, nil
}

func (alttpSRAMCheatEditor) Apply(pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error) {
	parsed, err := parseALTTPSRAM(payload)
	if err != nil {
		return nil, nil, err
	}
	slotIndex, err := alttpSlotIndex(slotID)
	if err != nil {
		return nil, nil, err
	}
	slot := parsed.Slots[slotIndex]
	if slot == nil {
		return nil, nil, fmt.Errorf("save slot %q is not present in this SRAM", slotID)
	}
	fieldMap := alttpFieldIndex(pack)
	changed := map[string]any{}
	for fieldID, value := range updates {
		field, ok := fieldMap[fieldID]
		if !ok {
			return nil, nil, fmt.Errorf("unknown field %q", fieldID)
		}
		if err := alttpApplyField(slot.Block, field, value); err != nil {
			return nil, nil, fmt.Errorf("apply %s: %w", fieldID, err)
		}
		changed[fieldID] = value
	}
	alttpRepairSelectedBottle(slot.Block)
	alttpWriteChecksum(slot.Block)
	updated := append([]byte(nil), parsed.Payload...)
	primaryStart := alttpSlotOffset(slotIndex)
	mirrorStart := alttpMirrorSlotOffset(slotIndex)
	copy(updated[primaryStart:primaryStart+alttpSlotSize], slot.Block)
	copy(updated[mirrorStart:mirrorStart+alttpSlotSize], slot.Block)
	return updated, changed, nil
}

func parseALTTPSRAM(payload []byte) (*alttpParsedSRAM, error) {
	if len(payload) != alttpSRAMSize {
		return nil, fmt.Errorf("expected %d-byte A Link to the Past SRAM, got %d", alttpSRAMSize, len(payload))
	}
	parsed := &alttpParsedSRAM{Payload: append([]byte(nil), payload...)}
	validCount := 0
	for slotIndex := 0; slotIndex < alttpSlotCount; slotIndex++ {
		primaryStart := alttpSlotOffset(slotIndex)
		primary := append([]byte(nil), payload[primaryStart:primaryStart+alttpSlotSize]...)
		if region, ok := alttpVerifyBlock(primary); ok {
			parsed.Slots[slotIndex] = &alttpSlot{Block: primary, Region: region, SourceBase: primaryStart}
			validCount++
			continue
		}
		mirrorStart := alttpMirrorSlotOffset(slotIndex)
		mirror := append([]byte(nil), payload[mirrorStart:mirrorStart+alttpSlotSize]...)
		if region, ok := alttpVerifyBlock(mirror); ok {
			parsed.Slots[slotIndex] = &alttpSlot{Block: mirror, Region: region, SourceBase: mirrorStart}
			validCount++
		}
	}
	if validCount == 0 {
		return nil, errors.New("a link to the past SRAM does not contain a valid checksummed save slot")
	}
	return parsed, nil
}

func alttpVerifyBlock(block []byte) (string, bool) {
	if len(block) != alttpSlotSize {
		return "", false
	}
	region := alttpBlockRegion(block)
	if region == "" {
		return "", false
	}
	return region, alttpBlockChecksum(block) == alttpChecksumTarget
}

func alttpBlockRegion(block []byte) string {
	if len(block) != alttpSlotSize {
		return ""
	}
	if block[alttpUSMarkerOffset] == alttpMarkerLow && block[alttpUSMarkerOffset+1] == alttpMarkerHigh {
		return "usa-eur"
	}
	if block[alttpJPMarkerOffset] == alttpMarkerLow && block[alttpJPMarkerOffset+1] == alttpMarkerHigh {
		return "jpn"
	}
	return ""
}

func alttpBlockChecksum(block []byte) uint16 {
	var checksum uint16
	for offset := 0; offset < alttpSlotSize; offset += 2 {
		checksum += binary.LittleEndian.Uint16(block[offset : offset+2])
	}
	return checksum
}

func alttpWriteChecksum(block []byte) {
	var checksum uint16
	for offset := 0; offset < alttpChecksumOffset; offset += 2 {
		checksum += binary.LittleEndian.Uint16(block[offset : offset+2])
	}
	binary.LittleEndian.PutUint16(block[alttpChecksumOffset:alttpChecksumOffset+2], alttpChecksumTarget-checksum)
}

func alttpBuildFieldValues(pack cheatPack, block []byte) map[string]any {
	values := map[string]any{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			switch field.Op.Kind {
			case "alttpCounter":
				if spec, ok := alttpCounterSpecs[field.Op.Field]; ok {
					values[field.ID] = alttpReadCounter(block, spec)
				}
			case "alttpEnum":
				if spec, ok := alttpEnumSpecs[field.Op.Field]; ok {
					if valueID, ok := alttpEnumID(spec, block[alttpItemBaseOffset+spec.Offset]); ok {
						values[field.ID] = valueID
					}
				}
			case "alttpBoolean":
				if spec, ok := alttpBoolSpecs[field.Op.Field]; ok {
					values[field.ID] = alttpReadBool(block, spec)
				}
			case "alttpBitmask":
				if spec, ok := alttpBitmaskSpecs[field.Op.Field]; ok {
					values[field.ID] = alttpReadBitmask(block[alttpItemBaseOffset+spec.Offset], field.Bits)
				}
			}
		}
	}
	return values
}

func alttpFieldIndex(pack cheatPack) map[string]cheatField {
	fields := map[string]cheatField{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			fields[field.ID] = field
		}
	}
	return fields
}

func alttpApplyField(block []byte, field cheatField, value any) error {
	switch field.Op.Kind {
	case "alttpCounter":
		raw, ok := value.(int)
		if !ok {
			return errors.New("expected integer")
		}
		spec, ok := alttpCounterSpecs[field.Op.Field]
		if !ok {
			return fmt.Errorf("unknown ALTTP counter field %q", field.Op.Field)
		}
		if raw < 0 || raw > spec.Max {
			return fmt.Errorf("counter must be between 0 and %d", spec.Max)
		}
		alttpWriteCounter(block, spec, raw)
		return nil
	case "alttpEnum":
		raw, ok := value.(string)
		if !ok {
			return errors.New("expected enum string")
		}
		spec, ok := alttpEnumSpecs[field.Op.Field]
		if !ok {
			return fmt.Errorf("unknown ALTTP enum field %q", field.Op.Field)
		}
		enumValue, ok := spec.Values[strings.TrimSpace(raw)]
		if !ok {
			return fmt.Errorf("unsupported enum value %q", raw)
		}
		block[alttpItemBaseOffset+spec.Offset] = enumValue
		return nil
	case "alttpBoolean":
		raw, ok := value.(bool)
		if !ok {
			return errors.New("expected boolean")
		}
		spec, ok := alttpBoolSpecs[field.Op.Field]
		if !ok {
			return fmt.Errorf("unknown ALTTP boolean field %q", field.Op.Field)
		}
		alttpWriteBool(block, spec, raw)
		return nil
	case "alttpBitmask":
		values, err := normalizeCheatStringArray(value)
		if err != nil {
			return err
		}
		spec, ok := alttpBitmaskSpecs[field.Op.Field]
		if !ok {
			return fmt.Errorf("unknown ALTTP bitmask field %q", field.Op.Field)
		}
		mask, err := alttpBuildBitmask(values, field.Bits)
		if err != nil {
			return err
		}
		block[alttpItemBaseOffset+spec.Offset] = mask
		return nil
	default:
		return fmt.Errorf("unsupported op kind %q", field.Op.Kind)
	}
}

func alttpReadCounter(block []byte, spec alttpCounterSpec) int {
	var value int
	if spec.Size == 2 {
		value = int(binary.LittleEndian.Uint16(block[alttpItemBaseOffset+spec.Offset : alttpItemBaseOffset+spec.Offset+2]))
	} else {
		value = int(block[alttpItemBaseOffset+spec.Offset])
	}
	if spec.ConvertFromRaw != nil {
		value = spec.ConvertFromRaw(value)
	}
	return value
}

func alttpWriteCounter(block []byte, spec alttpCounterSpec, value int) {
	raw := value
	if spec.ConvertToRaw != nil {
		raw = spec.ConvertToRaw(value)
	}
	writeOne := func(offset int) {
		if spec.Size == 2 {
			binary.LittleEndian.PutUint16(block[alttpItemBaseOffset+offset:alttpItemBaseOffset+offset+2], uint16(raw))
			return
		}
		block[alttpItemBaseOffset+offset] = byte(raw)
	}
	writeOne(spec.Offset)
	if spec.MirrorOffset > 0 {
		writeOne(spec.MirrorOffset)
	}
	if spec.ClearOnApply > 0 {
		block[alttpItemBaseOffset+spec.ClearOnApply] = 0
	}
}

func alttpReadBool(block []byte, spec alttpBoolSpec) bool {
	if block[alttpItemBaseOffset+spec.Offset] != spec.TrueValue {
		return false
	}
	if spec.AbilityMask == 0 {
		return true
	}
	return block[alttpItemBaseOffset+alttpAbilityFlagsField]&spec.AbilityMask != 0
}

func alttpWriteBool(block []byte, spec alttpBoolSpec, enabled bool) {
	if enabled {
		block[alttpItemBaseOffset+spec.Offset] = spec.TrueValue
		if spec.AbilityMask != 0 {
			block[alttpItemBaseOffset+alttpAbilityFlagsField] |= spec.AbilityMask
		}
		return
	}
	block[alttpItemBaseOffset+spec.Offset] = 0
	if spec.AbilityMask != 0 {
		block[alttpItemBaseOffset+alttpAbilityFlagsField] &^= spec.AbilityMask
	}
}

func alttpEnumID(spec alttpEnumSpec, value byte) (string, bool) {
	for id, candidate := range spec.Values {
		if candidate == value {
			return id, true
		}
	}
	return "", false
}

func alttpReadBitmask(mask byte, bits []cheatBitOption) []string {
	out := []string{}
	for _, option := range bits {
		if option.Bit < 0 || option.Bit > 7 {
			continue
		}
		if mask&(1<<option.Bit) != 0 {
			out = append(out, option.ID)
		}
	}
	return out
}

func alttpBuildBitmask(values []string, bits []cheatBitOption) (byte, error) {
	byID := map[string]cheatBitOption{}
	for _, option := range bits {
		byID[option.ID] = option
	}
	var mask byte
	for _, id := range values {
		option, ok := byID[strings.TrimSpace(id)]
		if !ok {
			return 0, fmt.Errorf("unknown bit %q", id)
		}
		if option.Bit < 0 || option.Bit > 7 {
			return 0, fmt.Errorf("invalid bit index %d for %q", option.Bit, id)
		}
		mask |= 1 << option.Bit
	}
	return mask, nil
}

func alttpRepairSelectedBottle(block []byte) {
	selected := byte(0)
	for index, offset := range []int{0x1c, 0x1d, 0x1e, 0x1f} {
		if block[alttpItemBaseOffset+offset] != 0 {
			selected = byte(index + 1)
			break
		}
	}
	block[alttpItemBaseOffset+0x0f] = selected
}

func alttpBottleEnumSpec(offset int) alttpEnumSpec {
	return alttpEnumSpec{
		Offset: offset,
		Values: map[string]byte{
			"none":        0,
			"mushroom":    1,
			"empty":       2,
			"redPotion":   3,
			"greenPotion": 4,
			"bluePotion":  5,
			"fairy":       6,
			"bee":         7,
			"goodBee":     8,
		},
	}
}

func alttpHealthToHearts(raw int) int {
	return raw / 8
}

func alttpHeartsToHealth(hearts int) int {
	return hearts * 8
}

func alttpSlotOffset(index int) int {
	return index * alttpSlotSize
}

func alttpMirrorSlotOffset(index int) int {
	return alttpMirrorBaseOffset + index*alttpSlotSize
}

func alttpSlotID(index int) string {
	return strconv.Itoa(index + 1)
}

func alttpSlotIndex(slotID string) (int, error) {
	normalized := strings.TrimSpace(slotID)
	index, err := strconv.Atoi(normalized)
	if err != nil || index < 1 || index > alttpSlotCount {
		return 0, fmt.Errorf("unsupported save slot %q", slotID)
	}
	return index - 1, nil
}
