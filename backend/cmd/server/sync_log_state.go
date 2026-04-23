package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	syncLogStateFileName  = "sync_logs.json"
	syncLogMaxEntries     = 5000
	syncLogRetentionHours = 24 * 14
)

type syncLogRecord struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	DeviceName   string    `json:"deviceName"`
	Action       string    `json:"action"`
	Game         string    `json:"game"`
	Error        bool      `json:"error"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	SystemSlug   string    `json:"systemSlug,omitempty"`
	SaveID       string    `json:"saveId,omitempty"`
	ConflictID   string    `json:"conflictId,omitempty"`
}

type syncLogStateFile struct {
	Logs      []syncLogRecord `json:"logs"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type syncLogInput struct {
	CreatedAt    time.Time
	DeviceName   string
	Action       string
	Game         string
	ErrorMessage string
	SystemSlug   string
	SaveID       string
	ConflictID   string
}

func syncLogStateFilePathFromEnv() string {
	return filepath.Join(stateRootDirFromEnv(), syncLogStateFileName)
}

func (a *app) loadSyncLogState() error {
	if a == nil || strings.TrimSpace(a.syncLogStateFile) == "" {
		return nil
	}

	data, err := os.ReadFile(a.syncLogStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read sync log state: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var file syncLogStateFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("decode sync log state: %w", err)
	}

	now := time.Now().UTC()
	logs := pruneSyncLogs(file.Logs, now)

	a.mu.Lock()
	a.syncLogs = logs
	a.mu.Unlock()
	return nil
}

func pruneSyncLogs(logs []syncLogRecord, now time.Time) []syncLogRecord {
	if len(logs) == 0 {
		return []syncLogRecord{}
	}

	cutoff := now.Add(-time.Duration(syncLogRetentionHours) * time.Hour)
	filtered := make([]syncLogRecord, 0, len(logs))
	for _, record := range logs {
		if record.CreatedAt.IsZero() {
			continue
		}
		if record.CreatedAt.Before(cutoff) {
			continue
		}
		filtered = append(filtered, record)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].ID > filtered[j].ID
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	if len(filtered) > syncLogMaxEntries {
		filtered = filtered[:syncLogMaxEntries]
	}
	return filtered
}

func (a *app) persistSyncLogStateLocked() error {
	if a == nil || strings.TrimSpace(a.syncLogStateFile) == "" {
		return nil
	}

	snapshot := syncLogStateFile{
		Logs:      append([]syncLogRecord(nil), a.syncLogs...),
		UpdatedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sync log state: %w", err)
	}
	if err := writeFileAtomic(a.syncLogStateFile, data, 0o644); err != nil {
		return fmt.Errorf("write sync log state: %w", err)
	}
	return nil
}

func (a *app) appendSyncLog(input syncLogInput) {
	if a == nil {
		return
	}

	createdAt := input.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	record := syncLogRecord{
		ID:           "sync-log-" + createdAt.Format("20060102150405.000000000") + "-" + hash12(strings.Join([]string{input.Action, input.DeviceName, input.Game, input.SaveID, input.ConflictID, createdAt.Format(time.RFC3339Nano)}, "::")),
		CreatedAt:    createdAt,
		DeviceName:   firstNonEmpty(input.DeviceName, "Web UI"),
		Action:       firstNonEmpty(input.Action, "unknown"),
		Game:         firstNonEmpty(input.Game, "Unknown"),
		Error:        strings.TrimSpace(input.ErrorMessage) != "",
		ErrorMessage: strings.TrimSpace(input.ErrorMessage),
		SystemSlug:   canonicalOptionalSegment(input.SystemSlug),
		SaveID:       strings.TrimSpace(input.SaveID),
		ConflictID:   strings.TrimSpace(input.ConflictID),
	}

	a.mu.Lock()
	a.syncLogs = append([]syncLogRecord{record}, a.syncLogs...)
	a.syncLogs = pruneSyncLogs(a.syncLogs, createdAt)
	_ = a.persistSyncLogStateLocked()
	a.mu.Unlock()

	a.publishEvent("sync_log", record)
}

func (a *app) snapshotSyncLogs() []syncLogRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	logs := make([]syncLogRecord, len(a.syncLogs))
	copy(logs, a.syncLogs)
	return logs
}

func syncLogDeviceNameFromHelperContext(helperCtx helperAuthContext, identity helperIdentity) string {
	if strings.TrimSpace(helperCtx.Device.DisplayName) != "" {
		return helperCtx.Device.DisplayName
	}
	if identity.isComplete() {
		return defaultDeviceDisplayName(identity.DeviceType, identity.Fingerprint)
	}
	return "Web UI"
}

func syncLogGameLabelFromRecord(record saveRecord) string {
	return firstNonEmpty(
		strings.TrimSpace(record.Summary.DisplayTitle),
		strings.TrimSpace(record.Summary.Game.DisplayTitle),
		strings.TrimSpace(record.Summary.Game.Name),
		strings.TrimSpace(record.Summary.Filename),
		"Unknown",
	)
}

func syncLogGameLabelFromSummary(summary saveSummary) string {
	return firstNonEmpty(
		strings.TrimSpace(summary.DisplayTitle),
		strings.TrimSpace(summary.Game.DisplayTitle),
		strings.TrimSpace(summary.Game.Name),
		strings.TrimSpace(summary.Filename),
		"Unknown",
	)
}

func syncLogGameLabelFromFilename(filename string) string {
	clean := strings.TrimSpace(filename)
	if clean == "" {
		return "Unknown"
	}
	return clean
}

func (a *app) handleSyncLogs(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	hours := parseIntOrDefault(r.URL.Query().Get("hours"), 72)
	if hours <= 0 {
		hours = 72
	}
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 50)
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	page := parseIntOrDefault(r.URL.Query().Get("page"), 1)
	if page <= 0 {
		page = 1
	}

	now := time.Now().UTC()
	cutoff := now.Add(-time.Duration(hours) * time.Hour)
	logs := a.snapshotSyncLogs()
	filtered := make([]syncLogRecord, 0, len(logs))
	for _, record := range logs {
		if record.CreatedAt.Before(cutoff) {
			continue
		}
		filtered = append(filtered, record)
	}

	total := len(filtered)
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"generatedAt": now,
		"hours":       hours,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"totalPages":  totalPages,
		"logs":        filtered[start:end],
	})
}
