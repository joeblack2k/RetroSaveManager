package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	sm64EEPROMSize      = 0x200
	sm64SaveFileSize    = 56
	sm64FileStride      = sm64SaveFileSize * 2
	sm64CourseCount     = 25
	sm64MainCourseCount = 15
	sm64SaveMagic       = 0x4441
)

const (
	sm64FlagFileExists            uint32 = 1 << 0
	sm64FlagHaveWingCap           uint32 = 1 << 1
	sm64FlagHaveMetalCap          uint32 = 1 << 2
	sm64FlagHaveVanishCap         uint32 = 1 << 3
	sm64FlagHaveKey1              uint32 = 1 << 4
	sm64FlagHaveKey2              uint32 = 1 << 5
	sm64FlagUnlockedBasementDoor  uint32 = 1 << 6
	sm64FlagUnlockedUpstairsDoor  uint32 = 1 << 7
	sm64FlagDDDMovedBack          uint32 = 1 << 8
	sm64FlagMoatDrained           uint32 = 1 << 9
	sm64FlagUnlockedPSSDoor       uint32 = 1 << 10
	sm64FlagUnlockedWFDoor        uint32 = 1 << 11
	sm64FlagUnlockedCCMDoor       uint32 = 1 << 12
	sm64FlagUnlockedJRBDoor       uint32 = 1 << 13
	sm64FlagUnlockedBITDWDoor     uint32 = 1 << 14
	sm64FlagUnlockedBITFSDoor     uint32 = 1 << 15
	sm64FlagUnlocked50StarDoor    uint32 = 1 << 20
)

type sm64EEPROMCheatEditor struct{}

type sm64ParsedFile struct {
	Flags            uint32
	CourseStars      [sm64CourseCount]byte
	CourseCoinScores [sm64MainCourseCount]byte
}

type sm64ParsedEEPROM struct {
	Payload []byte
	Files   [4]sm64ParsedFile
}

func (sm64EEPROMCheatEditor) ID() string {
	return "sm64-eeprom"
}

func (sm64EEPROMCheatEditor) Read(pack cheatPack, payload []byte) (saveCheatEditorState, error) {
	parsed, err := parseSM64EEPROM(payload)
	if err != nil {
		return saveCheatEditorState{}, err
	}
	state := saveCheatEditorState{
		Values:     map[string]any{},
		SlotValues: map[string]map[string]any{},
	}
	for slotIndex := range parsed.Files {
		slotID := sm64SlotID(slotIndex)
		state.SlotValues[slotID] = sm64BuildFieldValues(pack, parsed.Files[slotIndex])
	}
	return state, nil
}

func (sm64EEPROMCheatEditor) Apply(pack cheatPack, payload []byte, slotID string, updates map[string]any) ([]byte, map[string]any, error) {
	parsed, err := parseSM64EEPROM(payload)
	if err != nil {
		return nil, nil, err
	}
	slotIndex, err := sm64SlotIndex(slotID)
	if err != nil {
		return nil, nil, err
	}
	file := parsed.Files[slotIndex]
	fieldMap := sm64FieldIndex(pack)
	changed := map[string]any{}
	for fieldID, value := range updates {
		field, ok := fieldMap[fieldID]
		if !ok {
			return nil, nil, fmt.Errorf("unknown field %q", fieldID)
		}
		if err := sm64ApplyField(&file, field, value); err != nil {
			return nil, nil, fmt.Errorf("apply %s: %w", fieldID, err)
		}
		changed[fieldID] = value
	}
	file.Flags |= sm64FlagFileExists
	parsed.Files[slotIndex] = file
	updated := make([]byte, len(parsed.Payload))
	copy(updated, parsed.Payload)
	block := sm64BuildSaveFileBlock(file)
	primaryOffset := slotIndex * sm64FileStride
	backupOffset := primaryOffset + sm64SaveFileSize
	copy(updated[primaryOffset:primaryOffset+sm64SaveFileSize], block)
	copy(updated[backupOffset:backupOffset+sm64SaveFileSize], block)
	return updated, changed, nil
}

func parseSM64EEPROM(payload []byte) (*sm64ParsedEEPROM, error) {
	if len(payload) != sm64EEPROMSize {
		return nil, fmt.Errorf("expected %d-byte SM64 EEPROM, got %d", sm64EEPROMSize, len(payload))
	}
	parsed := &sm64ParsedEEPROM{Payload: append([]byte(nil), payload...)}
	for fileIndex := 0; fileIndex < 4; fileIndex++ {
		primaryOffset := fileIndex * sm64FileStride
		backupOffset := primaryOffset + sm64SaveFileSize
		primary := payload[primaryOffset : primaryOffset+sm64SaveFileSize]
		backup := payload[backupOffset : backupOffset+sm64SaveFileSize]
		var source []byte
		switch {
		case sm64VerifySaveFileBlock(primary):
			source = primary
		case sm64VerifySaveFileBlock(backup):
			source = backup
		default:
			return nil, fmt.Errorf("save file %s does not contain a valid mirrored block", sm64SlotID(fileIndex))
		}
		parsed.Files[fileIndex] = sm64DecodeSaveFile(source)
	}
	return parsed, nil
}

func sm64VerifySaveFileBlock(block []byte) bool {
	if len(block) != sm64SaveFileSize {
		return false
	}
	magic := binary.BigEndian.Uint16(block[len(block)-4 : len(block)-2])
	if magic != sm64SaveMagic {
		return false
	}
	checksum := binary.BigEndian.Uint16(block[len(block)-2:])
	return checksum == sm64BlockChecksum(block)
}

func sm64BlockChecksum(block []byte) uint16 {
	var sum uint32
	for i := 0; i < len(block)-2; i++ {
		sum += uint32(block[i])
	}
	return uint16(sum & 0xffff)
}

func sm64DecodeSaveFile(block []byte) sm64ParsedFile {
	var file sm64ParsedFile
	file.Flags = binary.BigEndian.Uint32(block[8:12])
	copy(file.CourseStars[:], block[12:12+sm64CourseCount])
	copy(file.CourseCoinScores[:], block[12+sm64CourseCount:12+sm64CourseCount+sm64MainCourseCount])
	return file
}

func sm64BuildSaveFileBlock(file sm64ParsedFile) []byte {
	block := make([]byte, sm64SaveFileSize)
	binary.BigEndian.PutUint32(block[8:12], file.Flags)
	copy(block[12:12+sm64CourseCount], file.CourseStars[:])
	copy(block[12+sm64CourseCount:12+sm64CourseCount+sm64MainCourseCount], file.CourseCoinScores[:])
	binary.BigEndian.PutUint16(block[len(block)-4:len(block)-2], sm64SaveMagic)
	binary.BigEndian.PutUint16(block[len(block)-2:], sm64BlockChecksum(block))
	return block
}

func sm64BuildFieldValues(pack cheatPack, file sm64ParsedFile) map[string]any {
	values := map[string]any{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			switch field.Op.Kind {
			case "flag":
				mask, ok := sm64FlagMask(field.Op.Flag)
				if !ok {
					continue
				}
				values[field.ID] = file.Flags&mask != 0
			case "secretStars":
				values[field.ID] = sm64BitsToSelection(field.Bits, byte((file.Flags>>24)&0x7f))
			case "courseStars":
				courseIndex, ok := sm64CourseIndex(field.Op.Course)
				if !ok {
					continue
				}
				values[field.ID] = sm64BitsToSelection(field.Bits, file.CourseStars[courseIndex]&0x7f)
			case "courseCannon":
				courseIndex, ok := sm64CourseIndex(field.Op.Course)
				if !ok {
					continue
				}
				values[field.ID] = file.CourseStars[courseIndex]&0x80 != 0
			case "courseCoinScore":
				courseIndex, ok := sm64CourseIndex(field.Op.Course)
				if !ok || courseIndex >= len(file.CourseCoinScores) {
					continue
				}
				values[field.ID] = int(file.CourseCoinScores[courseIndex])
			}
		}
	}
	return values
}

func sm64FieldIndex(pack cheatPack) map[string]cheatField {
	fields := map[string]cheatField{}
	for _, section := range pack.Sections {
		for _, field := range section.Fields {
			fields[field.ID] = field
		}
	}
	return fields
}

func sm64ApplyField(file *sm64ParsedFile, field cheatField, value any) error {
	switch field.Op.Kind {
	case "flag":
		mask, ok := sm64FlagMask(field.Op.Flag)
		if !ok {
			return fmt.Errorf("unsupported flag %q", field.Op.Flag)
		}
		flagValue, ok := value.(bool)
		if !ok {
			return errors.New("expected boolean")
		}
		if flagValue {
			file.Flags |= mask
		} else {
			file.Flags &^= mask
		}
		return nil
	case "secretStars":
		selection, ok := value.([]string)
		if !ok {
			return errors.New("expected bit selection")
		}
		mask := sm64SelectionToBits(field.Bits, selection)
		file.Flags &^= uint32(0x7f) << 24
		file.Flags |= uint32(mask) << 24
		return nil
	case "courseStars":
		courseIndex, ok := sm64CourseIndex(field.Op.Course)
		if !ok {
			return fmt.Errorf("unknown course %q", field.Op.Course)
		}
		selection, ok := value.([]string)
		if !ok {
			return errors.New("expected bit selection")
		}
		mask := sm64SelectionToBits(field.Bits, selection) & 0x7f
		file.CourseStars[courseIndex] = (file.CourseStars[courseIndex] & 0x80) | mask
		return nil
	case "courseCannon":
		courseIndex, ok := sm64CourseIndex(field.Op.Course)
		if !ok {
			return fmt.Errorf("unknown course %q", field.Op.Course)
		}
		flagValue, ok := value.(bool)
		if !ok {
			return errors.New("expected boolean")
		}
		if flagValue {
			file.CourseStars[courseIndex] |= 0x80
		} else {
			file.CourseStars[courseIndex] &^= 0x80
		}
		return nil
	case "courseCoinScore":
		courseIndex, ok := sm64CourseIndex(field.Op.Course)
		if !ok || courseIndex >= len(file.CourseCoinScores) {
			return fmt.Errorf("unknown course %q", field.Op.Course)
		}
		intValue, ok := value.(int)
		if !ok {
			return errors.New("expected integer")
		}
		file.CourseCoinScores[courseIndex] = byte(intValue)
		return nil
	default:
		return fmt.Errorf("unsupported op kind %q", field.Op.Kind)
	}
}

func sm64BitsToSelection(bits []cheatBitOption, value byte) []string {
	selected := make([]string, 0, len(bits))
	for _, bit := range bits {
		if value&(1<<bit.Bit) != 0 {
			selected = append(selected, bit.ID)
		}
	}
	sort.Strings(selected)
	return selected
}

func sm64SelectionToBits(bits []cheatBitOption, selected []string) byte {
	selectedSet := map[string]struct{}{}
	for _, item := range selected {
		selectedSet[strings.TrimSpace(item)] = struct{}{}
	}
	var out byte
	for _, bit := range bits {
		if _, ok := selectedSet[bit.ID]; ok {
			out |= 1 << bit.Bit
		}
	}
	return out
}

func sm64SlotID(index int) string {
	return string(rune('A' + index))
}

func sm64SlotIndex(slotID string) (int, error) {
	normalized := strings.ToUpper(strings.TrimSpace(slotID))
	switch normalized {
	case "A":
		return 0, nil
	case "B":
		return 1, nil
	case "C":
		return 2, nil
	case "D":
		return 3, nil
	default:
		return 0, fmt.Errorf("unsupported save slot %q", slotID)
	}
}

func sm64FlagMask(name string) (uint32, bool) {
	switch strings.TrimSpace(name) {
	case "haveWingCap":
		return sm64FlagHaveWingCap, true
	case "haveMetalCap":
		return sm64FlagHaveMetalCap, true
	case "haveVanishCap":
		return sm64FlagHaveVanishCap, true
	case "haveKey1":
		return sm64FlagHaveKey1, true
	case "haveKey2":
		return sm64FlagHaveKey2, true
	case "unlockedBasementDoor":
		return sm64FlagUnlockedBasementDoor, true
	case "unlockedUpstairsDoor":
		return sm64FlagUnlockedUpstairsDoor, true
	case "dddMovedBack":
		return sm64FlagDDDMovedBack, true
	case "moatDrained":
		return sm64FlagMoatDrained, true
	case "unlockedPssDoor":
		return sm64FlagUnlockedPSSDoor, true
	case "unlockedWfDoor":
		return sm64FlagUnlockedWFDoor, true
	case "unlockedCcmDoor":
		return sm64FlagUnlockedCCMDoor, true
	case "unlockedJrbDoor":
		return sm64FlagUnlockedJRBDoor, true
	case "unlockedBitdwDoor":
		return sm64FlagUnlockedBITDWDoor, true
	case "unlockedBitfsDoor":
		return sm64FlagUnlockedBITFSDoor, true
	case "unlocked50StarDoor":
		return sm64FlagUnlocked50StarDoor, true
	default:
		return 0, false
	}
}

func sm64CourseIndex(course string) (int, bool) {
	switch strings.TrimSpace(course) {
	case "bob":
		return 0, true
	case "wf":
		return 1, true
	case "jrb":
		return 2, true
	case "ccm":
		return 3, true
	case "bbh":
		return 4, true
	case "hmc":
		return 5, true
	case "lll":
		return 6, true
	case "ssl":
		return 7, true
	case "ddd":
		return 8, true
	case "sl":
		return 9, true
	case "wdw":
		return 10, true
	case "ttm":
		return 11, true
	case "thi":
		return 12, true
	case "ttc":
		return 13, true
	case "rr":
		return 14, true
	default:
		return 0, false
	}
}
