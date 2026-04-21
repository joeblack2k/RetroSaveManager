package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type sseEvent struct {
	Type string
	Data any
}

type conflictRecord struct {
	ConflictID     string
	ROMSHA1        string
	SlotName       string
	LocalSHA256    string
	CloudSHA256    string
	CloudVersion   *int
	CloudSaveID    *string
	Status         string
	CreatedAt      time.Time
	DeviceName     string
	DeviceFilename string
	DeviceFileSize int
}

type authStateSnapshot struct {
	GameCount        int
	FileCount        int
	StorageUsedBytes int
	DeviceCount      int
}

type app struct {
	mu                    sync.Mutex
	nextDeviceID          int
	nextAppPasswordID     int
	nextLibraryID         int
	nextSuggestionID      int
	devices               map[int]device
	trustedDevices        map[string]trustedDevice
	appPasswords          map[string]appPassword
	catalog               map[string]catalogGame
	library               map[string]libraryGame
	roadmapItems          map[string]roadmapItem
	roadmapSuggestions    map[string]roadmapSuggestion
	saves                 []saveSummary
	saveStore             *saveStore
	saveRecords           []saveRecord
	conflicts             map[string]conflictRecord
	nextEventSubscriberID int
	eventSubscribers      map[int]chan sseEvent
}

func newApp() *app {
	now := time.Now().UTC()
	catalog := map[string]catalogGame{
		"cat-1": {
			ID:          "cat-1",
			Name:        "Wario Land II",
			Description: "Classic Game Boy platform adventure.",
			System:      system{ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy"},
			Boxart:      nil,
			BoxartThumb: nil,
			DownloadURL: "/catalog/cat-1/download",
		},
		"cat-2": {
			ID:          "cat-2",
			Name:        "Chrono Trigger",
			Description: "Timeless RPG for SNES.",
			System:      system{ID: 26, Name: "Nintendo Super Nintendo Entertainment System", Slug: "snes"},
			Boxart:      nil,
			BoxartThumb: nil,
			DownloadURL: "/catalog/cat-2/download",
		},
	}

	a := &app{
		nextDeviceID:      2,
		nextAppPasswordID: 2,
		nextLibraryID:     2,
		nextSuggestionID:  2,
		devices: map[int]device{
			1: {
				ID:           1,
				DeviceType:   "internal",
				Fingerprint:  "seed0001",
				Alias:        nil,
				DisplayName:  "internal seed0001",
				LastSyncedAt: now,
				CreatedAt:    now,
			},
		},
		trustedDevices: map[string]trustedDevice{
			"trusted-1": {
				ID:        "trusted-1",
				Name:      "internal seed0001",
				CreatedAt: now,
			},
		},
		appPasswords: map[string]appPassword{
			"app-password-1": {
				ID:        "app-password-1",
				Name:      "default",
				LastFour:  "4e2a",
				CreatedAt: now,
			},
		},
		catalog: catalog,
		library: map[string]libraryGame{
			"lib-1": {
				ID:      "lib-1",
				Catalog: catalog["cat-1"],
				AddedAt: now,
			},
		},
		roadmapItems: map[string]roadmapItem{
			"roadmap-1": {
				ID:          "roadmap-1",
				Title:       "Improved save merge tooling",
				Description: "Add richer conflict previews and guided merge options.",
				Votes:       5,
				CreatedAt:   now.Add(-72 * time.Hour),
			},
			"roadmap-2": {
				ID:          "roadmap-2",
				Title:       "Per-device sync schedules",
				Description: "Allow different sync cadences per source/device.",
				Votes:       3,
				CreatedAt:   now.Add(-24 * time.Hour),
			},
		},
		roadmapSuggestions: map[string]roadmapSuggestion{},
		saves:              []saveSummary{},
		saveRecords:        []saveRecord{},
		conflicts:          map[string]conflictRecord{},
		eventSubscribers:   map[int]chan sseEvent{},
	}
	return a
}

func (a *app) initSaveStore() error {
	store, err := newSaveStoreFromEnv()
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.saveStore = store
	a.mu.Unlock()

	isEmpty, err := store.isEmpty()
	if err != nil {
		return err
	}
	if isEmpty {
		if err := a.bootstrapSeedSave(); err != nil {
			return err
		}
	}
	return a.reloadSavesFromDisk()
}

func (a *app) bootstrapSeedSave() error {
	gbc := system{ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy"}
	_, err := a.createSave(saveCreateInput{
		Filename:   "Wario Land II.srm",
		Payload:    []byte("seed-save-content"),
		Game:       game{ID: 281, Name: "Wario Land II", Boxart: nil, BoxartThumb: nil, HasParser: false, System: &gbc},
		Format:     "sram",
		Metadata:   nil,
		ROMSHA1:    "",
		ROMMD5:     "",
		SlotName:   "default",
		SystemSlug: gbc.Slug,
		GameSlug:   "wario-land-ii",
		CreatedAt:  time.Unix(1700000000, 0).UTC(),
	})
	return err
}

func (a *app) createSave(input saveCreateInput) (saveRecord, error) {
	a.mu.Lock()
	store := a.saveStore
	a.mu.Unlock()
	if store == nil {
		return saveRecord{}, fmt.Errorf("save store is not initialized")
	}

	record, err := store.create(input)
	if err != nil {
		return saveRecord{}, err
	}

	a.mu.Lock()
	a.saveRecords = append([]saveRecord{record}, a.saveRecords...)
	a.saves = append([]saveSummary{record.Summary}, a.saves...)
	a.mu.Unlock()
	return record, nil
}

func (a *app) reloadSavesFromDisk() error {
	a.mu.Lock()
	store := a.saveStore
	a.mu.Unlock()
	if store == nil {
		return fmt.Errorf("save store is not initialized")
	}

	records, err := store.load()
	if err != nil {
		return err
	}

	summaries := make([]saveSummary, 0, len(records))
	for _, record := range records {
		summaries = append(summaries, record.Summary)
	}

	a.mu.Lock()
	a.saveRecords = records
	a.saves = summaries
	a.mu.Unlock()
	return nil
}

func (a *app) upsertDevice(deviceType, fingerprint string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now().UTC()
	for id, d := range a.devices {
		if d.DeviceType == deviceType && d.Fingerprint == fingerprint {
			d.LastSyncedAt = now
			a.devices[id] = d
			return
		}
	}

	id := a.nextDeviceID
	a.nextDeviceID++
	a.devices[id] = device{
		ID:           id,
		DeviceType:   deviceType,
		Fingerprint:  fingerprint,
		Alias:        nil,
		DisplayName:  fmt.Sprintf("%s %s", deviceType, fingerprint),
		LastSyncedAt: now,
		CreatedAt:    now,
	}
}

func conflictKey(romSHA1, slotName string) string {
	return strings.ToLower(strings.TrimSpace(romSHA1)) + "::" + normalizedSlot(slotName)
}

func (a *app) latestSaveRecord(romSHA1, slotName string) (saveRecord, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return latestSaveRecordLocked(a.saveRecords, romSHA1, slotName)
}

func latestSaveRecordLocked(records []saveRecord, romSHA1, slotName string) (saveRecord, bool) {
	targetROM := strings.TrimSpace(romSHA1)
	targetSlot := normalizedSlot(slotName)
	var latest saveRecord
	found := false
	for _, record := range records {
		if targetROM == "" || record.ROMSHA1 != targetROM || normalizedSlot(record.SlotName) != targetSlot {
			continue
		}
		if !found || record.Summary.Version > latest.Summary.Version || (record.Summary.Version == latest.Summary.Version && record.Summary.CreatedAt.After(latest.Summary.CreatedAt)) {
			latest = record
			found = true
		}
	}
	return latest, found
}

func (a *app) saveCreatedEvent(record saveRecord) {
	a.publishEvent("save_created", map[string]any{
		"id":       record.Summary.ID,
		"sha256":   record.Summary.SHA256,
		"version":  record.Summary.Version,
		"romSha1":  record.ROMSHA1,
		"slotName": normalizedSlot(record.SlotName),
	})
}

func (a *app) reportConflict(romSHA1, slotName, localSHA256, cloudSHA256, deviceName, deviceFilename string, deviceFileSize int) conflictRecord {
	key := conflictKey(romSHA1, slotName)
	now := time.Now().UTC()
	cleanFilename := safeFilename(deviceFilename)
	if strings.TrimSpace(cleanFilename) == "" {
		cleanFilename = "unknown.save"
	}

	a.mu.Lock()
	latest, hasLatest := latestSaveRecordLocked(a.saveRecords, romSHA1, slotName)
	record := conflictRecord{
		ConflictID:     deterministicConflictID(key),
		ROMSHA1:        strings.TrimSpace(romSHA1),
		SlotName:       normalizedSlot(slotName),
		LocalSHA256:    strings.TrimSpace(localSHA256),
		CloudSHA256:    strings.TrimSpace(cloudSHA256),
		Status:         "open",
		CreatedAt:      now,
		DeviceName:     strings.TrimSpace(deviceName),
		DeviceFilename: cleanFilename,
		DeviceFileSize: deviceFileSize,
	}
	if hasLatest {
		version := latest.Summary.Version
		saveID := latest.Summary.ID
		record.CloudVersion = &version
		record.CloudSaveID = &saveID
		if record.CloudSHA256 == "" {
			record.CloudSHA256 = latest.Summary.SHA256
		}
	}
	a.conflicts[key] = record
	a.mu.Unlock()

	a.publishEvent("conflict_created", map[string]any{
		"conflictId":   record.ConflictID,
		"romSha1":      record.ROMSHA1,
		"slotName":     record.SlotName,
		"cloudSha256":  record.CloudSHA256,
		"cloudVersion": record.CloudVersion,
		"cloudSaveId":  record.CloudSaveID,
		"status":       record.Status,
	})
	return record
}

func (a *app) getConflict(romSHA1, slotName string) (conflictRecord, bool) {
	key := conflictKey(romSHA1, slotName)
	a.mu.Lock()
	defer a.mu.Unlock()
	record, ok := a.conflicts[key]
	return record, ok
}

func (a *app) getConflictByID(id string) (conflictRecord, bool) {
	targetID := strings.TrimSpace(id)
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, record := range a.conflicts {
		if record.ConflictID == targetID {
			return record, true
		}
	}
	return conflictRecord{}, false
}

func (a *app) listConflicts() []conflictRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	conflicts := make([]conflictRecord, 0, len(a.conflicts))
	for _, record := range a.conflicts {
		conflicts = append(conflicts, record)
	}
	return conflicts
}

func (a *app) resolveConflictByID(id string) (conflictRecord, bool) {
	targetID := strings.TrimSpace(id)
	a.mu.Lock()
	var (
		resolved conflictRecord
		ok       bool
	)
	for key, record := range a.conflicts {
		if record.ConflictID != targetID {
			continue
		}
		resolved = record
		delete(a.conflicts, key)
		ok = true
		break
	}
	a.mu.Unlock()
	if !ok {
		return conflictRecord{}, false
	}

	a.publishEvent("conflict_resolved", map[string]any{
		"conflictId":   resolved.ConflictID,
		"romSha1":      resolved.ROMSHA1,
		"slotName":     resolved.SlotName,
		"cloudSha256":  resolved.CloudSHA256,
		"cloudVersion": resolved.CloudVersion,
		"cloudSaveId":  resolved.CloudSaveID,
		"status":       "resolved",
	})
	return resolved, true
}

func (a *app) resolveConflictForSave(record saveRecord) {
	if strings.TrimSpace(record.ROMSHA1) == "" {
		return
	}

	key := conflictKey(record.ROMSHA1, record.SlotName)
	a.mu.Lock()
	conflict, ok := a.conflicts[key]
	if ok {
		delete(a.conflicts, key)
	}
	a.mu.Unlock()
	if !ok {
		return
	}

	version := record.Summary.Version
	saveID := record.Summary.ID
	a.publishEvent("conflict_resolved", map[string]any{
		"conflictId":   conflict.ConflictID,
		"romSha1":      conflict.ROMSHA1,
		"slotName":     conflict.SlotName,
		"cloudSha256":  record.Summary.SHA256,
		"cloudVersion": version,
		"cloudSaveId":  saveID,
		"status":       "resolved",
	})
}

func (a *app) activeConflictCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.conflicts)
}

func (a *app) authSnapshot() authStateSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	gameKeys := map[string]struct{}{}
	storageUsedBytes := 0
	for _, record := range a.saveRecords {
		storageUsedBytes += record.Summary.FileSize
		key := fmt.Sprintf("%d:%s", record.Summary.Game.ID, strings.TrimSpace(record.Summary.Game.Name))
		gameKeys[key] = struct{}{}
	}

	return authStateSnapshot{
		GameCount:        len(gameKeys),
		FileCount:        len(a.saveRecords),
		StorageUsedBytes: storageUsedBytes,
		DeviceCount:      len(a.devices),
	}
}

func (a *app) subscribeEvents() (int, <-chan sseEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := a.nextEventSubscriberID
	a.nextEventSubscriberID++
	ch := make(chan sseEvent, 16)
	a.eventSubscribers[id] = ch
	return id, ch
}

func (a *app) unsubscribeEvents(id int) {
	a.mu.Lock()
	ch, ok := a.eventSubscribers[id]
	if ok {
		delete(a.eventSubscribers, id)
	}
	a.mu.Unlock()
	if ok {
		close(ch)
	}
}

func (a *app) publishEvent(eventType string, data any) {
	a.mu.Lock()
	subscribers := make([]chan sseEvent, 0, len(a.eventSubscribers))
	for _, ch := range a.eventSubscribers {
		subscribers = append(subscribers, ch)
	}
	a.mu.Unlock()

	event := sseEvent{Type: eventType, Data: data}
	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
