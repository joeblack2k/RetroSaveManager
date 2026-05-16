package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	sohFileVersion      = 1
	sohVanillaFileType  = 0
	sohBaseVersion      = 4
	sohStatsVersion     = 1
	sohPortContentType  = "application/json"
	sohDefaultBuildText = "RetroSaveManager conversion"

	ootOffsetEntranceIndex       = 0x0000
	ootOffsetLinkAge             = 0x0004
	ootOffsetCutsceneIndex       = 0x0008
	ootOffsetDayTime             = 0x000C
	ootOffsetNightFlag           = 0x0010
	ootOffsetTotalDays           = 0x0014
	ootOffsetBGSDayCount         = 0x0018
	ootOffsetPlayerName          = 0x0024
	ootOffsetSwordHealth         = 0x0036
	ootOffsetNaviTimer           = 0x0038
	ootOffsetBGSFlag             = 0x003E
	ootOffsetOcarinaGameRoundNum = 0x003F
	ootOffsetChildEquips         = 0x0040
	ootOffsetAdultEquips         = 0x004A
	ootOffsetUnk54               = 0x0054
	ootOffsetSavedSceneNum       = 0x0066
	ootOffsetEquips              = 0x0068
	ootOffsetInventory           = 0x0074
	ootOffsetSceneFlags          = 0x00D4
	ootOffsetFaroresWind         = 0x0E64
	ootOffsetGSFlags             = 0x0E9C
	ootOffsetHighScores          = 0x0EB8
	ootOffsetEventChkInf         = 0x0ED4
	ootOffsetItemGetInf          = 0x0EF0
	ootOffsetInfTable            = 0x0EF8
	ootOffsetWorldMapAreaData    = 0x0F38
	ootOffsetScarecrowLongSet    = 0x0F40
	ootOffsetScarecrowLongSong   = 0x0F41
	ootOffsetScarecrowSpawnSet   = 0x12C5
	ootOffsetScarecrowSpawnSong  = 0x12C6
	ootOffsetHorseData           = 0x1348

	ootItemEquipsButtonCount = 4
	ootItemEquipsCSlotCount  = 3
	sohItemEquipsButtonCount = 8
	sohItemEquipsCSlotCount  = 7
	ootItemEquipsSize        = 0x0A
	ootInventorySize         = 0x5E
	ootSceneFlagsCount       = 124
	ootSceneFlagsSize        = 0x1C
	ootFaroresWindSize       = 0x28
	ootOcarinaNoteSize       = 0x08
	ootScarecrowLongCount    = 108
	ootScarecrowSpawnCount   = 16
	ootHorseDataSize         = 0x0A
)

func shipOfHarkinianToOOTSRAM(payload []byte, slotHint string) ([]byte, error) {
	base, err := sohBaseSectionData(payload)
	if err != nil {
		return nil, err
	}
	slotIndex := sohSlotIndexFromHint(slotHint)
	block := make([]byte, ootSaveSize)
	copy(block[ootNewfOffset:ootNewfOffset+len(ootNewfMagic)], ootNewfMagic)
	sohWriteBaseDataToOOTBlock(block, base)
	ootSetSaveBlockChecksum(block)

	out := make([]byte, ootSramSize)
	copy(out[:len(ootHeaderMagic)], ootHeaderMagic)
	copy(out[ootSlotOffset(slotIndex, false):ootSlotOffset(slotIndex, false)+ootSaveSize], block)
	copy(out[ootSlotOffset(slotIndex, true):ootSlotOffset(slotIndex, true)+ootSaveSize], block)
	return out, nil
}

func ootSRAMToShipOfHarkinian(summary saveSummary, payload []byte) (string, []byte, error) {
	parsed, err := parseOOTSRAM(payload)
	if err != nil {
		return "", nil, err
	}
	slotIndex, ok := ootSlotIndexFromSummary(summary, parsed)
	if !ok {
		return "", nil, fmt.Errorf("Ocarina of Time SRAM does not contain a populated slot for Ship of Harkinian")
	}
	block := parsed.Slots[slotIndex].Block
	root := map[string]any{
		"version":  sohFileVersion,
		"fileType": sohVanillaFileType,
		"sections": map[string]any{
			"base": map[string]any{
				"version": sohBaseVersion,
				"data":    sohBaseDataFromOOTBlock(block),
			},
			"sohStats": map[string]any{
				"version": sohStatsVersion,
				"data":    sohStatsFromOOTBlock(block),
			},
		},
	}
	encoded, err := json.MarshalIndent(root, "", " ")
	if err != nil {
		return "", nil, fmt.Errorf("encode Ship of Harkinian save: %w", err)
	}
	encoded = append(encoded, '\n')
	return fmt.Sprintf("file%d.sav", slotIndex+1), encoded, nil
}

func sohBaseSectionData(payload []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("Ship of Harkinian save must be JSON: %w", err)
	}
	sections, ok := sohObject(root["sections"])
	if !ok {
		return nil, fmt.Errorf("Ship of Harkinian save is missing sections")
	}
	baseSection, ok := sohObject(sections["base"])
	if !ok {
		return nil, fmt.Errorf("Ship of Harkinian save is missing base section")
	}
	baseData, ok := sohObject(baseSection["data"])
	if !ok {
		return nil, fmt.Errorf("Ship of Harkinian base section is missing data")
	}
	return baseData, nil
}

func sohWriteBaseDataToOOTBlock(block []byte, data map[string]any) {
	sohWriteI32(block, ootOffsetEntranceIndex, data, "entranceIndex")
	sohWriteI32(block, ootOffsetLinkAge, data, "linkAge")
	sohWriteI32(block, ootOffsetCutsceneIndex, data, "cutsceneIndex")
	sohWriteU16(block, ootOffsetDayTime, data, "dayTime")
	sohWriteI32(block, ootOffsetNightFlag, data, "nightFlag")
	sohWriteI32(block, ootOffsetTotalDays, data, "totalDays")
	sohWriteI32(block, ootOffsetBGSDayCount, data, "bgsDayCount")
	sohWriteU16(block, ootOffsetDeaths, data, "deaths")
	sohWriteByteArray(block[ootOffsetPlayerName:ootOffsetPlayerName+8], data["playerName"], 0xDF)
	sohWriteI16(block, ootOffsetHealthCapacity, data, "healthCapacity")
	sohWriteI16(block, ootOffsetHealth, data, "health")
	sohWriteI8(block, ootOffsetMagicLevel, data, "magicLevel")
	sohWriteI8(block, ootOffsetMagic, data, "magic")
	sohWriteI16(block, ootOffsetRupees, data, "rupees")
	sohWriteU16(block, ootOffsetSwordHealth, data, "swordHealth")
	sohWriteU16(block, ootOffsetNaviTimer, data, "naviTimer")
	sohWriteBoolByte(block, ootOffsetIsMagicAcquired, data, "isMagicAcquired", "magicAcquired")
	sohWriteBoolByte(block, ootOffsetIsDoubleMagicAcquired, data, "isDoubleMagicAcquired", "doubleMagic")
	sohWriteBoolByte(block, ootOffsetIsDoubleDefense, data, "isDoubleDefenseAcquired", "doubleDefense")
	sohWriteU8(block, ootOffsetBGSFlag, data, "bgsFlag")
	sohWriteU8(block, ootOffsetOcarinaGameRoundNum, data, "ocarinaGameRoundNum")
	sohWriteItemEquips(block[ootOffsetChildEquips:ootOffsetChildEquips+ootItemEquipsSize], data["childEquips"])
	sohWriteItemEquips(block[ootOffsetAdultEquips:ootOffsetAdultEquips+ootItemEquipsSize], data["adultEquips"])
	sohWriteU32(block, ootOffsetUnk54, data, "unk_54")
	sohWriteI16(block, ootOffsetSavedSceneNum, data, "savedSceneNum")
	sohWriteItemEquips(block[ootOffsetEquips:ootOffsetEquips+ootItemEquipsSize], data["equips"])
	sohWriteInventory(block[ootOffsetInventory:ootOffsetInventory+ootInventorySize], data["inventory"])
	sohWriteSceneFlags(block[ootOffsetSceneFlags:ootOffsetSceneFlags+ootSceneFlagsCount*ootSceneFlagsSize], data["sceneFlags"])
	sohWriteFaroresWind(block[ootOffsetFaroresWind:ootOffsetFaroresWind+ootFaroresWindSize], data["fw"])
	sohWriteI32Array(block[ootOffsetGSFlags:ootOffsetGSFlags+6*4], data["gsFlags"], 6)
	sohWriteI32Array(block[ootOffsetHighScores:ootOffsetHighScores+7*4], data["highScores"], 7)
	sohWriteU16Array(block[ootOffsetEventChkInf:ootOffsetEventChkInf+14*2], data["eventChkInf"], 14)
	sohWriteU16Array(block[ootOffsetItemGetInf:ootOffsetItemGetInf+4*2], data["itemGetInf"], 4)
	sohWriteU16Array(block[ootOffsetInfTable:ootOffsetInfTable+30*2], data["infTable"], 30)
	sohWriteU32(block, ootOffsetWorldMapAreaData, data, "worldMapAreaData")
	sohWriteU8(block, ootOffsetScarecrowLongSet, data, "scarecrowLongSongSet")
	sohWriteOcarinaNotes(block[ootOffsetScarecrowLongSong:ootOffsetScarecrowLongSong+ootScarecrowLongCount*ootOcarinaNoteSize], firstNonNil(data["scarecrowLongSong"], data["scarecrowCustomSong"]), ootScarecrowLongCount)
	sohWriteU8(block, ootOffsetScarecrowSpawnSet, data, "scarecrowSpawnSongSet")
	sohWriteOcarinaNotes(block[ootOffsetScarecrowSpawnSong:ootOffsetScarecrowSpawnSong+ootScarecrowSpawnCount*ootOcarinaNoteSize], data["scarecrowSpawnSong"], ootScarecrowSpawnCount)
	sohWriteHorseData(block[ootOffsetHorseData:ootOffsetHorseData+ootHorseDataSize], data["horseData"])
}

func sohBaseDataFromOOTBlock(block []byte) map[string]any {
	return map[string]any{
		"entranceIndex":           int(readI32(block, ootOffsetEntranceIndex)),
		"linkAge":                 int(readI32(block, ootOffsetLinkAge)),
		"cutsceneIndex":           int(readI32(block, ootOffsetCutsceneIndex)),
		"dayTime":                 int(binary.BigEndian.Uint16(block[ootOffsetDayTime : ootOffsetDayTime+2])),
		"nightFlag":               int(readI32(block, ootOffsetNightFlag)),
		"totalDays":               int(readI32(block, ootOffsetTotalDays)),
		"bgsDayCount":             int(readI32(block, ootOffsetBGSDayCount)),
		"deaths":                  int(binary.BigEndian.Uint16(block[ootOffsetDeaths : ootOffsetDeaths+2])),
		"playerName":              readByteArray(block[ootOffsetPlayerName:ootOffsetPlayerName+8], 8),
		"healthCapacity":          int(readI16(block, ootOffsetHealthCapacity)),
		"health":                  int(readI16(block, ootOffsetHealth)),
		"magicLevel":              int(int8(block[ootOffsetMagicLevel])),
		"magic":                   int(int8(block[ootOffsetMagic])),
		"rupees":                  int(readI16(block, ootOffsetRupees)),
		"swordHealth":             int(binary.BigEndian.Uint16(block[ootOffsetSwordHealth : ootOffsetSwordHealth+2])),
		"naviTimer":               int(binary.BigEndian.Uint16(block[ootOffsetNaviTimer : ootOffsetNaviTimer+2])),
		"isMagicAcquired":         block[ootOffsetIsMagicAcquired] != 0,
		"isDoubleMagicAcquired":   block[ootOffsetIsDoubleMagicAcquired] != 0,
		"isDoubleDefenseAcquired": block[ootOffsetIsDoubleDefense] != 0,
		"bgsFlag":                 int(block[ootOffsetBGSFlag]),
		"ocarinaGameRoundNum":     int(block[ootOffsetOcarinaGameRoundNum]),
		"childEquips":             sohItemEquipsFromOOT(block[ootOffsetChildEquips : ootOffsetChildEquips+ootItemEquipsSize]),
		"adultEquips":             sohItemEquipsFromOOT(block[ootOffsetAdultEquips : ootOffsetAdultEquips+ootItemEquipsSize]),
		"unk_54":                  int(binary.BigEndian.Uint32(block[ootOffsetUnk54 : ootOffsetUnk54+4])),
		"savedSceneNum":           int(readI16(block, ootOffsetSavedSceneNum)),
		"equips":                  sohItemEquipsFromOOT(block[ootOffsetEquips : ootOffsetEquips+ootItemEquipsSize]),
		"inventory":               sohInventoryFromOOT(block[ootOffsetInventory : ootOffsetInventory+ootInventorySize]),
		"sceneFlags":              sohSceneFlagsFromOOT(block[ootOffsetSceneFlags : ootOffsetSceneFlags+ootSceneFlagsCount*ootSceneFlagsSize]),
		"fw":                      sohFaroresWindFromOOT(block[ootOffsetFaroresWind : ootOffsetFaroresWind+ootFaroresWindSize]),
		"gsFlags":                 readI32Array(block[ootOffsetGSFlags:ootOffsetGSFlags+6*4], 6),
		"highScores":              readI32Array(block[ootOffsetHighScores:ootOffsetHighScores+7*4], 7),
		"eventChkInf":             readU16Array(block[ootOffsetEventChkInf:ootOffsetEventChkInf+14*2], 14),
		"itemGetInf":              readU16Array(block[ootOffsetItemGetInf:ootOffsetItemGetInf+4*2], 4),
		"infTable":                readU16Array(block[ootOffsetInfTable:ootOffsetInfTable+30*2], 30),
		"worldMapAreaData":        int(binary.BigEndian.Uint32(block[ootOffsetWorldMapAreaData : ootOffsetWorldMapAreaData+4])),
		"scarecrowLongSongSet":    int(block[ootOffsetScarecrowLongSet]),
		"scarecrowLongSong":       sohOcarinaNotesFromOOT(block[ootOffsetScarecrowLongSong:ootOffsetScarecrowLongSong+ootScarecrowLongCount*ootOcarinaNoteSize], ootScarecrowLongCount),
		"scarecrowSpawnSongSet":   int(block[ootOffsetScarecrowSpawnSet]),
		"scarecrowSpawnSong":      sohOcarinaNotesFromOOT(block[ootOffsetScarecrowSpawnSong:ootOffsetScarecrowSpawnSong+ootScarecrowSpawnCount*ootOcarinaNoteSize], ootScarecrowSpawnCount),
		"horseData":               sohHorseDataFromOOT(block[ootOffsetHorseData : ootOffsetHorseData+ootHorseDataSize]),
		"randomizerInf":           []int{},
		"isMasterQuest":           false,
		"backupFW":                sohFaroresWindZero(),
		"dogParams":               0,
		"filenameLanguage":        0,
		"maskMemory":              0,
	}
}

func sohStatsFromOOTBlock(block []byte) map[string]any {
	healthCapacity := int(readI16(block, ootOffsetHealthCapacity))
	heartContainers := 0
	if healthCapacity > 3*ootHealthUnitsPerHeart {
		heartContainers = (healthCapacity / ootHealthUnitsPerHeart) - 3
	}
	return map[string]any{
		"buildVersion":        sohDefaultBuildText,
		"buildVersionMajor":   0,
		"buildVersionMinor":   0,
		"buildVersionPatch":   0,
		"heartPieces":         0,
		"heartContainers":     heartContainers,
		"dungeonKeys":         readSignedByteArray(block[ootOffsetInventory+0x48:ootOffsetInventory+0x48+19], 19),
		"playTimer":           0,
		"pauseTimer":          0,
		"rtaTiming":           false,
		"firstInput":          0,
		"fileCreatedAt":       0,
		"timestamps":          []int{},
		"counts":              []int{},
		"scenesDiscovered":    []int{},
		"entrancesDiscovered": []int{},
	}
}

func sohSlotIndexFromHint(value string) int {
	clean := canonicalOptionalSegment(strings.TrimSuffix(filepath.Base(strings.TrimSpace(value)), filepath.Ext(value)))
	switch clean {
	case "file2", "slot2", "b", "2":
		return 1
	case "file3", "slot3", "c", "3":
		return 2
	default:
		return 0
	}
}

func ootSlotIndexFromSummary(summary saveSummary, parsed *ootParsedSRAM) (int, bool) {
	preferred := sohSlotIndexFromHint(firstNonEmpty(summary.SlotID, summary.RelativePath, summary.Filename))
	if preferred >= 0 && preferred < len(parsed.Slots) && parsed.Slots[preferred].Present {
		return preferred, true
	}
	for index, slot := range parsed.Slots {
		if slot.Present {
			return index, true
		}
	}
	return 0, false
}

func sohObject(value any) (map[string]any, bool) {
	object, ok := value.(map[string]any)
	return object, ok
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func sohInt(value any) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		i, err := typed.Int64()
		if err == nil {
			return i, true
		}
		f, err := typed.Float64()
		if err == nil {
			return int64(f), true
		}
	case float64:
		return int64(typed), true
	case float32:
		return int64(typed), true
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	case uint:
		return int64(typed), true
	case uint64:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case bool:
		if typed {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func sohDataInt(data map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			return sohInt(value)
		}
	}
	return 0, false
}

func sohWriteI32(block []byte, offset int, data map[string]any, key string) {
	if value, ok := sohDataInt(data, key); ok {
		binary.BigEndian.PutUint32(block[offset:offset+4], uint32(int32(value)))
	}
}

func sohWriteU32(block []byte, offset int, data map[string]any, key string) {
	if value, ok := sohDataInt(data, key); ok {
		binary.BigEndian.PutUint32(block[offset:offset+4], uint32(value))
	}
}

func sohWriteI16(block []byte, offset int, data map[string]any, key string) {
	if value, ok := sohDataInt(data, key); ok {
		binary.BigEndian.PutUint16(block[offset:offset+2], uint16(int16(value)))
	}
}

func sohWriteU16(block []byte, offset int, data map[string]any, key string) {
	if value, ok := sohDataInt(data, key); ok {
		binary.BigEndian.PutUint16(block[offset:offset+2], uint16(value))
	}
}

func sohWriteI8(block []byte, offset int, data map[string]any, key string) {
	if value, ok := sohDataInt(data, key); ok {
		block[offset] = byte(int8(value))
	}
}

func sohWriteU8(block []byte, offset int, data map[string]any, key string) {
	if value, ok := sohDataInt(data, key); ok {
		block[offset] = byte(value)
	}
}

func sohWriteBoolByte(block []byte, offset int, data map[string]any, keys ...string) {
	if value, ok := sohDataInt(data, keys...); ok && value != 0 {
		block[offset] = 1
	}
}

func sohWriteByteArray(dst []byte, value any, fill byte) {
	for i := range dst {
		dst[i] = fill
	}
	array, ok := value.([]any)
	if !ok {
		return
	}
	for index := 0; index < len(dst) && index < len(array); index++ {
		if value, ok := sohInt(array[index]); ok {
			dst[index] = byte(value)
		}
	}
}

func sohWriteSignedByteArray(dst []byte, value any) {
	array, ok := value.([]any)
	if !ok {
		return
	}
	for index := 0; index < len(dst) && index < len(array); index++ {
		if value, ok := sohInt(array[index]); ok {
			dst[index] = byte(int8(value))
		}
	}
}

func sohWriteItemEquips(dst []byte, value any) {
	for index := 0; index < ootItemEquipsButtonCount+ootItemEquipsCSlotCount; index++ {
		dst[index] = 0xFF
	}
	object, ok := sohObject(value)
	if !ok {
		return
	}
	sohWriteByteArray(dst[:ootItemEquipsButtonCount], object["buttonItems"], 0xFF)
	sohWriteByteArray(dst[ootItemEquipsButtonCount:ootItemEquipsButtonCount+ootItemEquipsCSlotCount], object["cButtonSlots"], 0xFF)
	if equipment, ok := sohInt(object["equipment"]); ok {
		binary.BigEndian.PutUint16(dst[8:10], uint16(equipment))
	}
}

func sohWriteInventory(dst []byte, value any) {
	for index := range dst {
		dst[index] = 0
	}
	object, ok := sohObject(value)
	if !ok {
		return
	}
	sohWriteByteArray(dst[0x00:0x18], object["items"], 0xFF)
	sohWriteSignedByteArray(dst[0x18:0x28], object["ammo"])
	if equipment, ok := sohInt(object["equipment"]); ok {
		binary.BigEndian.PutUint16(dst[0x28:0x2A], uint16(equipment))
	}
	if upgrades, ok := sohInt(object["upgrades"]); ok {
		binary.BigEndian.PutUint32(dst[0x2C:0x30], uint32(upgrades))
	}
	if questItems, ok := sohInt(object["questItems"]); ok {
		binary.BigEndian.PutUint32(dst[0x30:0x34], uint32(questItems))
	}
	sohWriteByteArray(dst[0x34:0x48], object["dungeonItems"], 0)
	sohWriteSignedByteArray(dst[0x48:0x5B], object["dungeonKeys"])
	if defense, ok := sohInt(object["defenseHearts"]); ok {
		dst[0x5B] = byte(int8(defense))
	}
	if tokens, ok := sohInt(object["gsTokens"]); ok {
		binary.BigEndian.PutUint16(dst[0x5C:0x5E], uint16(int16(tokens)))
	}
}

func sohWriteSceneFlags(dst []byte, value any) {
	array, ok := value.([]any)
	if !ok {
		return
	}
	fields := []string{"chest", "swch", "clear", "collect", "unk", "rooms", "floors"}
	for index := 0; index < ootSceneFlagsCount && index < len(array); index++ {
		object, ok := sohObject(array[index])
		if !ok {
			continue
		}
		base := index * ootSceneFlagsSize
		for fieldIndex, field := range fields {
			if value, ok := sohInt(object[field]); ok {
				binary.BigEndian.PutUint32(dst[base+fieldIndex*4:base+fieldIndex*4+4], uint32(value))
			}
		}
	}
}

func sohWriteFaroresWind(dst []byte, value any) {
	object, ok := sohObject(value)
	if !ok {
		return
	}
	if pos, ok := sohObject(object["pos"]); ok {
		for index, field := range []string{"x", "y", "z"} {
			if value, ok := sohInt(pos[field]); ok {
				binary.BigEndian.PutUint32(dst[index*4:index*4+4], uint32(int32(value)))
			}
		}
	}
	for index, field := range []string{"yaw", "playerParams", "entranceIndex", "roomIndex", "set", "tempSwchFlags", "tempCollectFlags"} {
		if value, ok := sohInt(object[field]); ok {
			offset := 0x0C + index*4
			binary.BigEndian.PutUint32(dst[offset:offset+4], uint32(int32(value)))
		}
	}
}

func sohWriteI32Array(dst []byte, value any, count int) {
	array, ok := value.([]any)
	if !ok {
		return
	}
	for index := 0; index < count && index < len(array); index++ {
		if value, ok := sohInt(array[index]); ok {
			binary.BigEndian.PutUint32(dst[index*4:index*4+4], uint32(int32(value)))
		}
	}
}

func sohWriteU16Array(dst []byte, value any, count int) {
	array, ok := value.([]any)
	if !ok {
		return
	}
	for index := 0; index < count && index < len(array); index++ {
		if value, ok := sohInt(array[index]); ok {
			binary.BigEndian.PutUint16(dst[index*2:index*2+2], uint16(value))
		}
	}
}

func sohWriteOcarinaNotes(dst []byte, value any, count int) {
	array, ok := value.([]any)
	if !ok {
		return
	}
	for index := 0; index < count && index < len(array); index++ {
		base := index * ootOcarinaNoteSize
		if raw, ok := sohInt(array[index]); ok {
			dst[base] = byte(raw)
			continue
		}
		object, ok := sohObject(array[index])
		if !ok {
			continue
		}
		if value, ok := sohInt(object["noteIdx"]); ok {
			dst[base] = byte(value)
		}
		if value, ok := sohInt(object["unk_01"]); ok {
			dst[base+1] = byte(value)
		}
		if value, ok := sohInt(object["unk_02"]); ok {
			binary.BigEndian.PutUint16(dst[base+2:base+4], uint16(value))
		}
		if value, ok := sohInt(object["volume"]); ok {
			dst[base+4] = byte(value)
		}
		if value, ok := sohInt(object["vibrato"]); ok {
			dst[base+5] = byte(value)
		}
		if value, ok := sohInt(object["tone"]); ok {
			dst[base+6] = byte(int8(value))
		}
		if value, ok := sohInt(object["semitone"]); ok {
			dst[base+7] = byte(value)
		}
	}
}

func sohWriteHorseData(dst []byte, value any) {
	object, ok := sohObject(value)
	if !ok {
		return
	}
	if value, ok := sohInt(object["scene"]); ok {
		binary.BigEndian.PutUint16(dst[0:2], uint16(int16(value)))
	}
	if pos, ok := sohObject(object["pos"]); ok {
		for index, field := range []string{"x", "y", "z"} {
			if value, ok := sohInt(pos[field]); ok {
				offset := 2 + index*2
				binary.BigEndian.PutUint16(dst[offset:offset+2], uint16(int16(value)))
			}
		}
	}
	if value, ok := sohInt(object["angle"]); ok {
		binary.BigEndian.PutUint16(dst[8:10], uint16(int16(value)))
	}
}

func readI16(block []byte, offset int) int16 {
	return int16(binary.BigEndian.Uint16(block[offset : offset+2]))
}

func readI32(block []byte, offset int) int32 {
	return int32(binary.BigEndian.Uint32(block[offset : offset+4]))
}

func readByteArray(buf []byte, count int) []int {
	out := make([]int, count)
	for index := 0; index < count && index < len(buf); index++ {
		out[index] = int(buf[index])
	}
	return out
}

func readSignedByteArray(buf []byte, count int) []int {
	out := make([]int, count)
	for index := 0; index < count && index < len(buf); index++ {
		out[index] = int(int8(buf[index]))
	}
	return out
}

func readI32Array(buf []byte, count int) []int {
	out := make([]int, count)
	for index := 0; index < count; index++ {
		out[index] = int(int32(binary.BigEndian.Uint32(buf[index*4 : index*4+4])))
	}
	return out
}

func readU16Array(buf []byte, count int) []int {
	out := make([]int, count)
	for index := 0; index < count; index++ {
		out[index] = int(binary.BigEndian.Uint16(buf[index*2 : index*2+2]))
	}
	return out
}

func sohItemEquipsFromOOT(buf []byte) map[string]any {
	buttons := make([]int, sohItemEquipsButtonCount)
	for index := range buttons {
		buttons[index] = 0xFF
	}
	copy(buttons, readByteArray(buf[:ootItemEquipsButtonCount], ootItemEquipsButtonCount))
	cSlots := make([]int, sohItemEquipsCSlotCount)
	for index := range cSlots {
		cSlots[index] = 0xFF
	}
	copy(cSlots, readByteArray(buf[ootItemEquipsButtonCount:ootItemEquipsButtonCount+ootItemEquipsCSlotCount], ootItemEquipsCSlotCount))
	return map[string]any{
		"buttonItems":  buttons,
		"cButtonSlots": cSlots,
		"equipment":    int(binary.BigEndian.Uint16(buf[8:10])),
	}
}

func sohInventoryFromOOT(buf []byte) map[string]any {
	return map[string]any{
		"items":         readByteArray(buf[0x00:0x18], 24),
		"ammo":          readSignedByteArray(buf[0x18:0x28], 16),
		"equipment":     int(binary.BigEndian.Uint16(buf[0x28:0x2A])),
		"upgrades":      int(binary.BigEndian.Uint32(buf[0x2C:0x30])),
		"questItems":    int(binary.BigEndian.Uint32(buf[0x30:0x34])),
		"dungeonItems":  readByteArray(buf[0x34:0x48], 20),
		"dungeonKeys":   readSignedByteArray(buf[0x48:0x5B], 19),
		"defenseHearts": int(int8(buf[0x5B])),
		"gsTokens":      int(int16(binary.BigEndian.Uint16(buf[0x5C:0x5E]))),
	}
}

func sohSceneFlagsFromOOT(buf []byte) []map[string]any {
	out := make([]map[string]any, ootSceneFlagsCount)
	fields := []string{"chest", "swch", "clear", "collect", "unk", "rooms", "floors"}
	for index := range out {
		item := map[string]any{}
		base := index * ootSceneFlagsSize
		for fieldIndex, field := range fields {
			item[field] = int(binary.BigEndian.Uint32(buf[base+fieldIndex*4 : base+fieldIndex*4+4]))
		}
		out[index] = item
	}
	return out
}

func sohFaroresWindFromOOT(buf []byte) map[string]any {
	return map[string]any{
		"pos": map[string]any{
			"x": int(readI32(buf, 0x00)),
			"y": int(readI32(buf, 0x04)),
			"z": int(readI32(buf, 0x08)),
		},
		"yaw":              int(readI32(buf, 0x0C)),
		"playerParams":     int(readI32(buf, 0x10)),
		"entranceIndex":    int(readI32(buf, 0x14)),
		"roomIndex":        int(readI32(buf, 0x18)),
		"set":              int(readI32(buf, 0x1C)),
		"tempSwchFlags":    int(readI32(buf, 0x20)),
		"tempCollectFlags": int(readI32(buf, 0x24)),
	}
}

func sohFaroresWindZero() map[string]any {
	return map[string]any{
		"pos":              map[string]any{"x": 0, "y": 0, "z": 0},
		"yaw":              0,
		"playerParams":     0,
		"entranceIndex":    0,
		"roomIndex":        0,
		"set":              0,
		"tempSwchFlags":    0,
		"tempCollectFlags": 0,
	}
}

func sohOcarinaNotesFromOOT(buf []byte, count int) []map[string]any {
	out := make([]map[string]any, count)
	for index := range out {
		base := index * ootOcarinaNoteSize
		out[index] = map[string]any{
			"noteIdx":  int(buf[base]),
			"unk_01":   int(buf[base+1]),
			"unk_02":   int(binary.BigEndian.Uint16(buf[base+2 : base+4])),
			"volume":   int(buf[base+4]),
			"vibrato":  int(buf[base+5]),
			"tone":     int(int8(buf[base+6])),
			"semitone": int(buf[base+7]),
		}
	}
	return out
}

func sohHorseDataFromOOT(buf []byte) map[string]any {
	return map[string]any{
		"scene": int(readI16(buf, 0)),
		"pos": map[string]any{
			"x": int(readI16(buf, 2)),
			"y": int(readI16(buf, 4)),
			"z": int(readI16(buf, 6)),
		},
		"angle": int(readI16(buf, 8)),
	}
}
