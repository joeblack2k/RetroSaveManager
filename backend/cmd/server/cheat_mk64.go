package main

import (
	"errors"
	"fmt"
	"strings"
)

const (
	mk64EEPROMSize   = 0x200
	mk64MainOffset   = 0x180
	mk64BackupOffset = 0x1F8
	mk64StuffSize    = 0x8
)

type mk64EEPROMCheatEditor struct{}

type mk64ParsedStuff struct {
	GrandPrixPoints     [4]byte
	SoundMode           byte
	UnknownChecksumByte byte
}

type mk64ParsedEEPROM struct {
	Payload []byte
	Active  mk64ParsedStuff
}

func (mk64EEPROMCheatEditor) ID() string {
	return "mk64-eeprom"
}

func (mk64EEPROMCheatEditor) Read(pack cheatPack, payload []byte) (saveCheatEditorState, error) {
	parsed, err := parseMK64EEPROM(payload)
	if err != nil {
		return saveCheatEditorState{}, err
	}
	return saveCheatEditorState{Values: mk64BuildFieldValues(pack, parsed.Active)}, nil
}

func (mk64EEPROMCheatEditor) Apply(pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error) {
	if strings.TrimSpace(slotID) != "" {
		return nil, nil, fmt.Errorf("save slot selection is not supported for %q", strings.TrimSpace(slotID))
	}
	parsed, err := parseMK64EEPROM(payload)
	if err != nil {
		return nil, nil, err
	}
	state := parsed.Active
	fieldMap := mk64FieldIndex(pack)
	changed := map[string]any{}
	for fieldID, value := range updates {
		field, ok := fieldMap[fieldID]
		if !ok {
			return nil, nil, fmt.Errorf("unknown field %q", fieldID)
		}
		if err := mk64ApplyField(&state, field, value); err != nil {
			return nil, nil, fmt.Errorf("apply %s: %w", fieldID, err)
		}
		changed[fieldID] = value
	}
	updated := append([]byte(nil), parsed.Payload...)
	stuff := mk64BuildStuff(state)
	copy(updated[mk64MainOffset:mk64MainOffset+mk64StuffSize], stuff)
	copy(updated[mk64BackupOffset:mk64BackupOffset+mk64StuffSize], stuff)
	return updated, changed, nil
}

func parseMK64EEPROM(payload []byte) (*mk64ParsedEEPROM, error) {
	if len(payload) != mk64EEPROMSize {
		return nil, fmt.Errorf("expected %d-byte Mario Kart 64 EEPROM, got %d", mk64EEPROMSize, len(payload))
	}
	main := payload[mk64MainOffset : mk64MainOffset+mk64StuffSize]
	backup := payload[mk64BackupOffset : mk64BackupOffset+mk64StuffSize]
	switch {
	case mk64VerifyStuff(main):
		parsed, err := mk64DecodeStuff(main)
		if err != nil {
			return nil, err
		}
		return &mk64ParsedEEPROM{Payload: append([]byte(nil), payload...), Active: parsed}, nil
	case mk64VerifyStuff(backup):
		parsed, err := mk64DecodeStuff(backup)
		if err != nil {
			return nil, err
		}
		return &mk64ParsedEEPROM{Payload: append([]byte(nil), payload...), Active: parsed}, nil
	default:
		return nil, errors.New("mario kart 64 save does not contain a valid main or backup save-info block")
	}
}

func mk64DecodeStuff(block []byte) (mk64ParsedStuff, error) {
	if len(block) != mk64StuffSize {
		return mk64ParsedStuff{}, fmt.Errorf("expected %d-byte Mario Kart 64 save-info block, got %d", mk64StuffSize, len(block))
	}
	state := mk64ParsedStuff{SoundMode: block[4], UnknownChecksumByte: block[5]}
	copy(state.GrandPrixPoints[:], block[:4])
	if _, ok := mk64SoundModeID(state.SoundMode); !ok {
		return mk64ParsedStuff{}, fmt.Errorf("unsupported Mario Kart 64 sound mode %d", state.SoundMode)
	}
	return state, nil
}

func mk64VerifyStuff(block []byte) bool {
	if len(block) != mk64StuffSize {
		return false
	}
	checksum1 := mk64Checksum1(block[:5])
	checksum2 := mk64Checksum2(checksum1)
	return block[6] == checksum1 && block[7] == checksum2
}

func mk64Checksum1(saveInfo []byte) byte {
	var sum int
	for idx, value := range saveInfo {
		sum += (int(value)+1)*(idx+1) + idx
	}
	return byte(sum % 0x100)
}

func mk64Checksum2(checksum1 byte) byte {
	return byte((int(checksum1) + 90) % 0x100)
}

func mk64BuildStuff(state mk64ParsedStuff) []byte {
	block := make([]byte, mk64StuffSize)
	copy(block[:4], state.GrandPrixPoints[:])
	block[4] = state.SoundMode
	block[5] = state.UnknownChecksumByte
	block[6] = mk64Checksum1(block[:5])
	block[7] = mk64Checksum2(block[6])
	return block
}

func mk64BuildFieldValues(pack cheatPack, state mk64ParsedStuff) map[string]any {
	values := map[string]any{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			switch field.Op.Kind {
			case "soundMode":
				if modeID, ok := mk64SoundModeID(state.SoundMode); ok {
					values[field.ID] = modeID
				}
			case "gpCupPoints":
				modeIndex, ok := mk64ModeIndex(field.Op.Mode)
				if !ok {
					continue
				}
				cupIndex, ok := mk64CupIndex(field.Op.Cup)
				if !ok {
					continue
				}
				if pointID, ok := mk64CupPointID(mk64CupPoints(state.GrandPrixPoints[modeIndex], cupIndex)); ok {
					values[field.ID] = pointID
				}
			}
		}
	}
	return values
}

func mk64FieldIndex(pack cheatPack) map[string]cheatField {
	fields := map[string]cheatField{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			fields[field.ID] = field
		}
	}
	return fields
}

func mk64ApplyField(state *mk64ParsedStuff, field cheatField, value any) error {
	switch field.Op.Kind {
	case "soundMode":
		raw, ok := value.(string)
		if !ok {
			return errors.New("expected enum string")
		}
		mode, ok := mk64SoundModeValue(raw)
		if !ok {
			return fmt.Errorf("unsupported sound mode %q", raw)
		}
		state.SoundMode = mode
		return nil
	case "gpCupPoints":
		raw, ok := value.(string)
		if !ok {
			return errors.New("expected enum string")
		}
		points, ok := mk64CupPointValue(raw)
		if !ok {
			return fmt.Errorf("unsupported cup value %q", raw)
		}
		modeIndex, ok := mk64ModeIndex(field.Op.Mode)
		if !ok {
			return fmt.Errorf("unknown cc mode %q", field.Op.Mode)
		}
		cupIndex, ok := mk64CupIndex(field.Op.Cup)
		if !ok {
			return fmt.Errorf("unknown cup %q", field.Op.Cup)
		}
		state.GrandPrixPoints[modeIndex] = mk64SetCupPoints(state.GrandPrixPoints[modeIndex], cupIndex, points)
		return nil
	default:
		return fmt.Errorf("unsupported op kind %q", field.Op.Kind)
	}
}

func mk64ModeIndex(mode string) (int, bool) {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "50cc":
		return 0, true
	case "100cc":
		return 1, true
	case "150cc":
		return 2, true
	case "extra":
		return 3, true
	default:
		return 0, false
	}
}

func mk64CupIndex(cup string) (int, bool) {
	switch strings.TrimSpace(strings.ToLower(cup)) {
	case "mushroom":
		return 0, true
	case "flower":
		return 1, true
	case "star":
		return 2, true
	case "special":
		return 3, true
	default:
		return 0, false
	}
}

func mk64CupPoints(ccPoints byte, cupIndex int) byte {
	shift := cupIndex * 2
	return (ccPoints >> shift) & 0x3
}

func mk64SetCupPoints(ccPoints byte, cupIndex int, points byte) byte {
	shift := cupIndex * 2
	mask := byte(0x3 << shift)
	return (ccPoints &^ mask) | ((points & 0x3) << shift)
}

func mk64CupPointID(points byte) (string, bool) {
	switch points {
	case 0:
		return "none", true
	case 1:
		return "bronze", true
	case 2:
		return "silver", true
	case 3:
		return "gold", true
	default:
		return "", false
	}
}

func mk64CupPointValue(id string) (byte, bool) {
	switch strings.TrimSpace(strings.ToLower(id)) {
	case "none":
		return 0, true
	case "bronze":
		return 1, true
	case "silver":
		return 2, true
	case "gold":
		return 3, true
	default:
		return 0, false
	}
}

func mk64SoundModeID(mode byte) (string, bool) {
	switch mode {
	case 0:
		return "stereo", true
	case 1:
		return "headphones", true
	case 2:
		return "unused", true
	case 3:
		return "mono", true
	default:
		return "", false
	}
}

func mk64SoundModeValue(id string) (byte, bool) {
	switch strings.TrimSpace(strings.ToLower(id)) {
	case "stereo":
		return 0, true
	case "headphones":
		return 1, true
	case "unused":
		return 2, true
	case "mono":
		return 3, true
	default:
		return 0, false
	}
}
