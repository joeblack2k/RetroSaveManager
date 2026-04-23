package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	n64TrustLevelMediaOnly        = "media-only"
	n64TrustLevelGameValidated    = "game-validated"
	n64TrustLevelSemanticVerified = "semantic-validated"
)

type n64ValidationContext struct {
	Extension string
	MediaType string
	Filename  string
	ROMSHA1   string
}

type n64GameValidationResult struct {
	GameID            string
	GameTitle         string
	TrustLevel        string
	ParserLevel       string
	Evidence          []string
	Warnings          []string
	SlotCount         int
	ActiveSlotIndexes []int
	ChecksumValid     *bool
	SemanticFields    map[string]any
}

type n64GameValidator interface {
	ID() string
	Validate(ctx n64ValidationContext, payload []byte) (*n64GameValidationResult, bool)
}

var n64GameValidators = []n64GameValidator{
	ootN64Validator{},
	yoshisStoryN64Validator{},
}

type ootN64Validator struct{}

const (
	ootSramSize        = 0x8000
	ootHeaderSize      = 12
	ootSlotStartOffset = 0x20
	ootSlotStride      = 0x1450
	ootSaveSize        = 0x1354
	ootChecksumOffset  = 0x1352
	ootNewfOffset      = 0x001C
	ootSlotCount       = 3
)

var (
	ootHeaderMagic = []byte{0x00, 0x00, 0x00, 0x98, 0x09, 0x10, 0x21, 0x5A, 0x45, 0x4C, 0x44, 0x41}
	ootNewfMagic   = []byte("ZELDAZ")
)

func (ootN64Validator) ID() string {
	return "n64-oot-sram"
}

func (ootN64Validator) Validate(ctx n64ValidationContext, payload []byte) (*n64GameValidationResult, bool) {
	if ctx.MediaType != "sram" || len(payload) != ootSramSize {
		return nil, false
	}

	normalized, wordSwapped, ok := ootNormalizePayload(payload)
	if !ok {
		return nil, false
	}

	occupied := make([]int, 0, ootSlotCount)
	validPrimary := make([]int, 0, ootSlotCount)
	validBackup := make([]int, 0, ootSlotCount)
	for slot := 0; slot < ootSlotCount; slot++ {
		primaryOffset := ootSlotStartOffset + slot*ootSlotStride
		backupOffset := ootSlotStartOffset + (slot+ootSlotCount)*ootSlotStride
		primary := normalized[primaryOffset : primaryOffset+ootSaveSize]
		backup := normalized[backupOffset : backupOffset+ootSaveSize]

		if ootSlotOccupied(primary) || ootSlotOccupied(backup) {
			occupied = append(occupied, slot+1)
		}
		if ootVerifySaveBlock(primary) {
			validPrimary = append(validPrimary, slot+1)
		}
		if ootVerifySaveBlock(backup) {
			validBackup = append(validBackup, slot+1)
		}
	}

	evidence := []string{"validated Ocarina of Time SRAM header"}
	if wordSwapped {
		evidence = append(evidence, "word-swapped container normalized")
	} else {
		evidence = append(evidence, "native byte order container")
	}
	if len(validPrimary) > 0 {
		evidence = append(evidence, fmt.Sprintf("validPrimarySlots=%v", validPrimary))
	}
	if len(validBackup) > 0 {
		evidence = append(evidence, fmt.Sprintf("validBackupSlots=%v", validBackup))
	}

	warnings := make([]string, 0, 2)
	trustLevel := n64TrustLevelGameValidated
	parserLevel := saveParserLevelStructural
	var checksumValid *bool
	if len(validPrimary) > 0 || len(validBackup) > 0 {
		valid := true
		checksumValid = &valid
		parserLevel = saveParserLevelSemantic
		trustLevel = n64TrustLevelSemanticVerified
	} else {
		warnings = append(warnings, "OOT SRAM header matched, but no populated save slot passed checksum validation")
	}
	if len(occupied) == 0 {
		warnings = append(warnings, "OOT SRAM contains no occupied ZELDAZ save slots")
	}

	return &n64GameValidationResult{
		GameID:            "n64/ocarina-of-time",
		GameTitle:         "The Legend of Zelda: Ocarina of Time",
		TrustLevel:        trustLevel,
		ParserLevel:       parserLevel,
		Evidence:          evidence,
		Warnings:          warnings,
		SlotCount:         len(occupied),
		ActiveSlotIndexes: occupied,
		ChecksumValid:     checksumValid,
		SemanticFields: map[string]any{
			"variant":           "oot",
			"wordSwapped":       wordSwapped,
			"occupiedSlots":     occupied,
			"validPrimarySlots": validPrimary,
			"validBackupSlots":  validBackup,
		},
	}, true
}

func ootNormalizePayload(payload []byte) ([]byte, bool, bool) {
	if len(payload) != ootSramSize {
		return nil, false, false
	}
	if bytes.Equal(payload[:ootHeaderSize], ootHeaderMagic) {
		return append([]byte(nil), payload...), false, true
	}
	swapped := n64Swap32Words(payload)
	if bytes.Equal(swapped[:ootHeaderSize], ootHeaderMagic) {
		return swapped, true, true
	}
	return nil, false, false
}

func ootSlotOccupied(block []byte) bool {
	return len(block) >= ootNewfOffset+len(ootNewfMagic) && bytes.Equal(block[ootNewfOffset:ootNewfOffset+len(ootNewfMagic)], ootNewfMagic)
}

func ootVerifySaveBlock(block []byte) bool {
	if len(block) != ootSaveSize {
		return false
	}
	if !ootSlotOccupied(block) {
		return false
	}
	expected := binary.BigEndian.Uint16(block[ootChecksumOffset : ootChecksumOffset+2])
	scratch := append([]byte(nil), block...)
	scratch[ootChecksumOffset] = 0
	scratch[ootChecksumOffset+1] = 0
	var checksum uint32
	for i := 0; i < len(scratch); i += 2 {
		checksum += uint32(binary.BigEndian.Uint16(scratch[i : i+2]))
	}
	return uint16(checksum&0xffff) == expected
}

type yoshisStoryN64Validator struct{}

const (
	yoshisStoryEEPROMSize  = 0x800
	yoshisStoryBufferSize  = 0x400
	yoshisStoryDataSize    = 0x3FA
	yoshisStoryMagicOffset = 0x3FC
)

const yoshisStoryMagic uint32 = 0x81317531

func (yoshisStoryN64Validator) ID() string {
	return "n64-yoshis-story-eeprom"
}

func (yoshisStoryN64Validator) Validate(ctx n64ValidationContext, payload []byte) (*n64GameValidationResult, bool) {
	if ctx.MediaType != "eeprom" || len(payload) != yoshisStoryEEPROMSize {
		return nil, false
	}

	validCopies := make([]int, 0, 2)
	nonZeroData := make([]int, 0, 2)
	var firstCopy []byte
	identicalCopies := true
	for copyIndex := 0; copyIndex < 2; copyIndex++ {
		offset := copyIndex * yoshisStoryBufferSize
		chunk := payload[offset : offset+yoshisStoryBufferSize]
		if binary.BigEndian.Uint32(chunk[yoshisStoryMagicOffset:yoshisStoryMagicOffset+4]) == yoshisStoryMagic {
			validCopies = append(validCopies, copyIndex+1)
		}
		nonZeroData = append(nonZeroData, countBytesNotEqual(chunk[:yoshisStoryDataSize], 0x00))
		if copyIndex == 0 {
			firstCopy = chunk
			continue
		}
		if !bytes.Equal(firstCopy, chunk) {
			identicalCopies = false
		}
	}

	if len(validCopies) == 0 {
		return nil, false
	}

	warnings := make([]string, 0, 2)
	if !identicalCopies {
		warnings = append(warnings, "Yoshi's Story EEPROM copies are not byte-identical")
	}
	if nonZeroData[0] == 0 && nonZeroData[1] == 0 {
		warnings = append(warnings, "Yoshi's Story EEPROM is structurally valid but contains no non-zero gameplay data")
	}
	warnings = append(warnings, "Yoshi's Story checksum algorithm is not validated yet; trust is based on mirrored container structure and game magic")

	evidence := []string{
		fmt.Sprintf("validCopies=%v", validCopies),
		fmt.Sprintf("copy1NonZeroData=%d", nonZeroData[0]),
		fmt.Sprintf("copy2NonZeroData=%d", nonZeroData[1]),
		"validated Yoshi's Story mirrored EEPROM magic",
	}
	if identicalCopies {
		evidence = append(evidence, "mirrored copies are identical")
	}

	return &n64GameValidationResult{
		GameID:            "n64/yoshis-story",
		GameTitle:         "Yoshi's Story",
		TrustLevel:        n64TrustLevelGameValidated,
		ParserLevel:       saveParserLevelStructural,
		Evidence:          evidence,
		Warnings:          warnings,
		SlotCount:         len(validCopies),
		ActiveSlotIndexes: validCopies,
		SemanticFields: map[string]any{
			"variant":          "yoshis-story",
			"validCopies":      validCopies,
			"copy1NonZeroData": nonZeroData[0],
			"copy2NonZeroData": nonZeroData[1],
			"identicalCopies":  identicalCopies,
		},
	}, true
}

func n64Swap32Words(payload []byte) []byte {
	swapped := append([]byte(nil), payload...)
	for i := 0; i+3 < len(swapped); i += 4 {
		swapped[i], swapped[i+1], swapped[i+2], swapped[i+3] = swapped[i+3], swapped[i+2], swapped[i+1], swapped[i]
	}
	return swapped
}
