package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const (
	sf64EEPROMSize        = 0x200
	sf64SaveBlockSize     = 0x100
	sf64SaveDataSize      = 0xFE
	sf64ChecksumOffset    = 0xFE
	sf64PlanetSlotCount   = 16
	sf64SoundModeOffset   = 0x14
	sf64MusicVolumeOffset = 0x15
	sf64VoiceVolumeOffset = 0x16
	sf64SFXVolumeOffset   = 0x17
	sf64SaveSlotVenom1    = 14
	sf64SaveSlotVenom2    = 15
)

// PlanetData bitfields are stored from the high bits down on N64; the exposed flags live in the low five bits.
const (
	sf64PlanetFlagExpertMedal byte = 0x10
	sf64PlanetFlagExpertClear byte = 0x08
	sf64PlanetFlagPlayed      byte = 0x04
	sf64PlanetFlagNormalMedal byte = 0x02
	sf64PlanetFlagNormalClear byte = 0x01
)

type sf64EEPROMCheatEditor struct{}

func init() {
	registerCheatEditor(sf64EEPROMCheatEditor{})
}

type sf64ParsedEEPROM struct {
	Payload []byte
	Save    []byte
}

func (sf64EEPROMCheatEditor) ID() string {
	return "sf64-eeprom"
}

func (sf64EEPROMCheatEditor) Read(pack cheatPack, payload []byte) (saveCheatEditorState, error) {
	window, err := n64SmallEEPROMWindow(payload, "Star Fox 64")
	if err != nil {
		return saveCheatEditorState{}, err
	}
	parsed, err := parseSF64EEPROM(window)
	if err != nil {
		return saveCheatEditorState{}, err
	}
	return saveCheatEditorState{Values: sf64BuildFieldValues(pack, parsed.Save)}, nil
}

func (sf64EEPROMCheatEditor) Apply(pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error) {
	if strings.TrimSpace(slotID) != "" {
		return nil, nil, fmt.Errorf("save slot selection is not supported for %q", strings.TrimSpace(slotID))
	}
	window, err := n64SmallEEPROMWindow(payload, "Star Fox 64")
	if err != nil {
		return nil, nil, err
	}
	parsed, err := parseSF64EEPROM(window)
	if err != nil {
		return nil, nil, err
	}
	save := append([]byte(nil), parsed.Save...)
	fieldMap := sf64FieldIndex(pack)
	changed := map[string]any{}
	for fieldID, value := range updates {
		field, ok := fieldMap[fieldID]
		if !ok {
			return nil, nil, fmt.Errorf("unknown field %q", fieldID)
		}
		if err := sf64ApplyField(save, field, value); err != nil {
			return nil, nil, fmt.Errorf("apply %s: %w", fieldID, err)
		}
		changed[fieldID] = value
	}
	sf64WriteChecksum(save)
	updated := append([]byte(nil), parsed.Payload...)
	copy(updated[:sf64SaveBlockSize], save)
	copy(updated[sf64SaveBlockSize:sf64SaveBlockSize*2], save)
	patched, err := n64PatchSmallEEPROMWindow(payload, updated, "Star Fox 64")
	if err != nil {
		return nil, nil, err
	}
	return patched, changed, nil
}

func parseSF64EEPROM(payload []byte) (*sf64ParsedEEPROM, error) {
	if len(payload) != sf64EEPROMSize {
		return nil, fmt.Errorf("expected %d-byte Star Fox 64 EEPROM, got %d", sf64EEPROMSize, len(payload))
	}
	primary := payload[:sf64SaveBlockSize]
	backup := payload[sf64SaveBlockSize : sf64SaveBlockSize*2]
	var source []byte
	switch {
	case sf64VerifySaveBlock(primary):
		source = primary
	case sf64VerifySaveBlock(backup):
		source = backup
	default:
		return nil, errors.New("star fox 64 EEPROM does not contain a valid primary or backup save block")
	}
	if err := sf64ValidateSaveData(source); err != nil {
		return nil, err
	}
	return &sf64ParsedEEPROM{
		Payload: append([]byte(nil), payload...),
		Save:    append([]byte(nil), source...),
	}, nil
}

func sf64VerifySaveBlock(block []byte) bool {
	if len(block) != sf64SaveBlockSize {
		return false
	}
	stored := binary.BigEndian.Uint16(block[sf64ChecksumOffset : sf64ChecksumOffset+2])
	return stored == sf64Checksum(block[:sf64SaveDataSize])
}

func sf64Checksum(data []byte) uint16 {
	var checksum uint16
	limit := len(data)
	if limit > sf64SaveDataSize {
		limit = sf64SaveDataSize
	}
	for i := 0; i < limit; i++ {
		checksum ^= uint16(data[i])
		checksum <<= 1
		checksum = (checksum & 0xFE) | ((checksum >> 8) & 1)
	}
	return (checksum & 0xFF) | 0x9500
}

func sf64WriteChecksum(block []byte) {
	if len(block) < sf64SaveBlockSize {
		return
	}
	binary.BigEndian.PutUint16(block[sf64ChecksumOffset:sf64ChecksumOffset+2], sf64Checksum(block[:sf64SaveDataSize]))
}

func sf64ValidateSaveData(block []byte) error {
	if len(block) != sf64SaveBlockSize {
		return fmt.Errorf("expected %d-byte Star Fox 64 save block, got %d", sf64SaveBlockSize, len(block))
	}
	if _, ok := sf64SoundModeID(block[sf64SoundModeOffset]); !ok {
		return fmt.Errorf("unsupported Star Fox 64 sound mode %d", block[sf64SoundModeOffset])
	}
	for _, offset := range []int{sf64MusicVolumeOffset, sf64VoiceVolumeOffset, sf64SFXVolumeOffset} {
		if block[offset] > 99 {
			return fmt.Errorf("unsupported Star Fox 64 volume value %d at 0x%02x", block[offset], offset)
		}
	}
	return nil
}

func sf64BuildFieldValues(pack cheatPack, save []byte) map[string]any {
	values := map[string]any{}
	if len(save) < sf64SaveBlockSize {
		return values
	}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			switch field.Op.Kind {
			case "sf64PlanetFlag":
				mask, ok := sf64PlanetFlagMask(field.Op.Flag)
				if !ok {
					continue
				}
				values[field.ID] = sf64PlanetFlagSelection(save, field.Bits, mask)
			case "sf64PlanetFlagBoolean":
				mask, maskOK := sf64PlanetFlagMask(field.Op.Flag)
				slotIndex, slotOK := sf64PlanetSlotIndex(field.Op.Field)
				if !maskOK || !slotOK {
					continue
				}
				values[field.ID] = save[slotIndex]&mask != 0
			case "sf64SoundMode":
				if modeID, ok := sf64SoundModeID(save[sf64SoundModeOffset]); ok {
					values[field.ID] = modeID
				}
			case "sf64Volume":
				offset, ok := sf64VolumeOffset(field.Op.Field)
				if !ok {
					continue
				}
				values[field.ID] = int(save[offset])
			}
		}
	}
	return values
}

func sf64FieldIndex(pack cheatPack) map[string]cheatField {
	fields := map[string]cheatField{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			fields[field.ID] = field
		}
	}
	return fields
}

func sf64ApplyField(save []byte, field cheatField, value any) error {
	if len(save) < sf64SaveBlockSize {
		return fmt.Errorf("expected %d-byte Star Fox 64 save block, got %d", sf64SaveBlockSize, len(save))
	}
	switch field.Op.Kind {
	case "sf64PlanetFlag":
		selection, err := normalizeCheatStringArray(value)
		if err != nil {
			return err
		}
		mask, ok := sf64PlanetFlagMask(field.Op.Flag)
		if !ok {
			return fmt.Errorf("unsupported planet flag %q", field.Op.Flag)
		}
		slotByID := map[string]int{}
		for _, bit := range field.Bits {
			if bit.Bit < 0 || bit.Bit >= sf64PlanetSlotCount {
				return fmt.Errorf("planet slot bit %d is out of range", bit.Bit)
			}
			slotByID[bit.ID] = bit.Bit
			save[bit.Bit] &^= mask
		}
		for _, id := range selection {
			slotIndex, ok := slotByID[id]
			if !ok {
				return fmt.Errorf("unknown planet flag bit %q", id)
			}
			save[slotIndex] |= mask
		}
		return nil
	case "sf64PlanetFlagBoolean":
		boolValue, ok := value.(bool)
		if !ok {
			return errors.New("expected boolean")
		}
		mask, ok := sf64PlanetFlagMask(field.Op.Flag)
		if !ok {
			return fmt.Errorf("unsupported planet flag %q", field.Op.Flag)
		}
		slotIndex, ok := sf64PlanetSlotIndex(field.Op.Field)
		if !ok {
			return fmt.Errorf("unknown planet slot %q", field.Op.Field)
		}
		if boolValue {
			save[slotIndex] |= mask
		} else {
			save[slotIndex] &^= mask
		}
		return nil
	case "sf64SoundMode":
		raw, ok := value.(string)
		if !ok {
			return errors.New("expected enum string")
		}
		mode, ok := sf64SoundModeValue(raw)
		if !ok {
			return fmt.Errorf("unsupported sound mode %q", raw)
		}
		save[sf64SoundModeOffset] = mode
		return nil
	case "sf64Volume":
		intValue, ok := value.(int)
		if !ok {
			return errors.New("expected integer")
		}
		offset, ok := sf64VolumeOffset(field.Op.Field)
		if !ok {
			return fmt.Errorf("unknown volume field %q", field.Op.Field)
		}
		if intValue < 0 || intValue > 99 {
			return errors.New("volume must be between 0 and 99")
		}
		save[offset] = byte(intValue)
		return nil
	default:
		return fmt.Errorf("unsupported op kind %q", field.Op.Kind)
	}
}

func sf64PlanetFlagSelection(save []byte, bits []cheatBitOption, mask byte) []string {
	selection := make([]string, 0, len(bits))
	for _, bit := range bits {
		if bit.Bit < 0 || bit.Bit >= sf64PlanetSlotCount {
			continue
		}
		if save[bit.Bit]&mask != 0 {
			selection = append(selection, bit.ID)
		}
	}
	return selection
}

func sf64PlanetFlagMask(flag string) (byte, bool) {
	switch strings.TrimSpace(flag) {
	case "played":
		return sf64PlanetFlagPlayed, true
	case "normalClear":
		return sf64PlanetFlagNormalClear, true
	case "normalMedal":
		return sf64PlanetFlagNormalMedal, true
	case "expertClear":
		return sf64PlanetFlagExpertClear, true
	case "expertMedal":
		return sf64PlanetFlagExpertMedal, true
	default:
		return 0, false
	}
}

func sf64PlanetSlotIndex(field string) (int, bool) {
	switch strings.TrimSpace(field) {
	case "meteo":
		return 0, true
	case "area6":
		return 1, true
	case "bolse":
		return 2, true
	case "sectorZ":
		return 3, true
	case "sectorX":
		return 4, true
	case "sectorY":
		return 5, true
	case "katina":
		return 6, true
	case "macbeth":
		return 7, true
	case "zoness":
		return 8, true
	case "corneria":
		return 9, true
	case "titania":
		return 10, true
	case "aquas":
		return 11, true
	case "fortuna":
		return 12, true
	case "solar":
		return 13, true
	case "venom1":
		return sf64SaveSlotVenom1, true
	case "venom2":
		return sf64SaveSlotVenom2, true
	default:
		return 0, false
	}
}

func sf64SoundModeID(mode byte) (string, bool) {
	switch mode {
	case 0:
		return "stereo", true
	case 1:
		return "mono", true
	case 2:
		return "headphones", true
	default:
		return "", false
	}
}

func sf64SoundModeValue(id string) (byte, bool) {
	switch strings.TrimSpace(strings.ToLower(id)) {
	case "stereo":
		return 0, true
	case "mono":
		return 1, true
	case "headphones":
		return 2, true
	default:
		return 0, false
	}
}

func sf64VolumeOffset(field string) (int, bool) {
	switch strings.TrimSpace(field) {
	case "musicVolume":
		return sf64MusicVolumeOffset, true
	case "voiceVolume":
		return sf64VoiceVolumeOffset, true
	case "sfxVolume":
		return sf64SFXVolumeOffset, true
	default:
		return 0, false
	}
}
