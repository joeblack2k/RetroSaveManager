package main

import (
	"sort"
	"strings"
)

type saveRecordIndex struct {
	byID                map[string]saveRecord
	latestByROMSlotKey  map[string]saveRecord
	latestReadableByKey map[string]saveRecord
	latestByTrack       map[string]saveRecord
}

// buildSaveRecordIndex snapshots the read models that must stay in lock-step
// with app.saves. Callers rebuild it immediately after any save mutation so
// latest/download/delete paths do not drift back to repeated linear scans.
func buildSaveRecordIndex(records []saveRecord) saveRecordIndex {
	index := saveRecordIndex{
		byID:                make(map[string]saveRecord, len(records)),
		latestByROMSlotKey:  make(map[string]saveRecord),
		latestReadableByKey: make(map[string]saveRecord),
		latestByTrack:       make(map[string]saveRecord),
	}

	for _, record := range records {
		if id := strings.TrimSpace(record.Summary.ID); id != "" {
			index.byID[id] = record
		}
		if key := romSlotIndexKey(record.ROMSHA1, record.SlotName); key != "" {
			if existing, ok := index.latestByROMSlotKey[key]; !ok || saveRecordSortsAfter(record, existing) {
				index.latestByROMSlotKey[key] = record
			}
			if saveRecordPayloadExists(record) {
				if existing, ok := index.latestReadableByKey[key]; !ok || saveRecordSortsAfter(record, existing) {
					index.latestReadableByKey[key] = record
				}
			}
		}
		if key := strings.TrimSpace(canonicalDuplicateTrackKeyForRecord(record)); key != "" && saveRecordPayloadExists(record) {
			if existing, ok := index.latestByTrack[key]; !ok || saveRecordSortsAfter(record, existing) {
				index.latestByTrack[key] = record
			}
		}
	}

	return index
}

func (index saveRecordIndex) recordByID(saveID string) (saveRecord, bool) {
	if index.byID == nil {
		return saveRecord{}, false
	}
	record, ok := index.byID[strings.TrimSpace(saveID)]
	return record, ok
}

func (index saveRecordIndex) latestByROMSlot(romSHA1, slotName string) (saveRecord, bool) {
	if index.latestByROMSlotKey == nil {
		return saveRecord{}, false
	}
	record, ok := index.latestByROMSlotKey[romSlotIndexKey(romSHA1, slotName)]
	return record, ok
}

func (index saveRecordIndex) latestReadableByROMSlot(romSHA1, slotName string) (saveRecord, bool) {
	if index.latestReadableByKey == nil {
		return saveRecord{}, false
	}
	record, ok := index.latestReadableByKey[romSlotIndexKey(romSHA1, slotName)]
	return record, ok
}

func (index saveRecordIndex) latestReadableByTrackContext(filename, systemSlug, displayTitle, regionCode string) (saveRecord, bool) {
	if index.latestByTrack == nil {
		return saveRecord{}, false
	}
	input := saveCreateInput{
		Filename:     strings.TrimSpace(filename),
		SystemSlug:   strings.TrimSpace(systemSlug),
		DisplayTitle: strings.TrimSpace(displayTitle),
		RegionCode:   strings.TrimSpace(regionCode),
	}
	if input.Filename == "" || input.SystemSlug == "" {
		return saveRecord{}, false
	}
	key := canonicalDuplicateTrackKeyForInput(input, input.Filename)
	if strings.TrimSpace(key) == "" {
		return saveRecord{}, false
	}
	record, ok := index.latestByTrack[key]
	return record, ok
}

func (index saveRecordIndex) recordsByIDs(ids []string) []saveRecord {
	selected := make([]saveRecord, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		clean := strings.TrimSpace(id)
		if clean == "" {
			continue
		}
		if _, exists := seen[clean]; exists {
			continue
		}
		seen[clean] = struct{}{}
		if record, ok := index.recordByID(clean); ok {
			selected = append(selected, record)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Summary.CreatedAt.Equal(selected[j].Summary.CreatedAt) {
			return selected[i].Summary.ID > selected[j].Summary.ID
		}
		return selected[i].Summary.CreatedAt.After(selected[j].Summary.CreatedAt)
	})
	return selected
}

func romSlotIndexKey(romSHA1, slotName string) string {
	rom := strings.TrimSpace(romSHA1)
	if rom == "" {
		return ""
	}
	return rom + "::" + normalizedSlot(slotName)
}
