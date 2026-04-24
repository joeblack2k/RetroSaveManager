package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func registerAgentRoutes(r chi.Router, app *app, prefix string) {
	r.Route(prefix, func(r chi.Router) {
		r.Get("/", app.handleAgentAPIIndex)
		mountAgentRoutes(r, app)
	})
}

func mountAgentRoutes(r chi.Router, app *app) {
	r.Get("/overview", app.handleAgentOverview)
	r.Get("/sync/status", app.handleAgentSyncStatus)
	r.Get("/systems", app.handleAgentSystems)

	r.Get("/devices", app.handleDevicesList)
	r.Get("/devices/{id}", app.handleDevicesGet)
	r.Patch("/devices/{id}", app.handleDevicesPatch)
	r.Delete("/devices/{id}", app.handleDevicesDelete)
	r.Get("/logs", app.handleSyncLogs)

	r.Get("/save/latest", app.handleSaveLatest)
	r.Get("/saves", app.handleAgentSavesList)
	r.Post("/saves", app.handleSaves)
	r.Delete("/saves", app.handleDeleteManySaves)
	r.Post("/saves/rescan", app.handleAgentSaveRescan)
	r.Get("/saves/download-many", app.handleDownloadManySaves)
	r.Get("/saves/{id}", app.handleAgentSaveGet)
	r.Delete("/saves/{id}", app.handleAgentSaveDelete)
	r.Post("/saves/{id}/rollback", app.handleAgentSaveRollback)
	r.Get("/saves/{id}/download", app.handleAgentSaveDownload)

	r.Get("/roms", app.handleAgentRomsList)
	r.Get("/roms/lookup", app.handleRomLookup)
	r.Post("/roms/lookup", app.handleRomLookup)
	r.Post("/roms/lookup/batch", app.handleGamesLookup)
	r.Get("/roms/{hash}", app.handleAgentRomGet)

	r.Get("/conflicts", app.handleConflictsList)
	r.Get("/conflicts/count", app.handleConflictsCount)
	r.Get("/conflicts/check", app.handleConflictsCheck)
	r.Post("/conflicts/check", app.handleConflictsCheck)
	r.Post("/conflicts/report", app.handleConflictsReport)
	r.Get("/conflicts/{id}", app.handleConflictsGet)
	r.Post("/conflicts/{id}/resolve", app.handleConflictsResolve)

	r.Get("/helpers/auto-enroll", app.handleAuthAppPasswordsAutoStatus)
	r.Post("/helpers/auto-enroll", app.handleAuthAppPasswordsAutoEnable)

	r.Get("/cheats/packs", app.handleCheatPacksList)
	r.Post("/cheats/packs", app.handleCheatPackCreate)
	r.Get("/cheats/packs/{id}", app.handleCheatPackGet)
	r.Delete("/cheats/packs/{id}", app.handleCheatPackDelete)
	r.Post("/cheats/packs/{id}/disable", app.handleCheatPackDisable)
	r.Post("/cheats/packs/{id}/enable", app.handleCheatPackEnable)
	r.Get("/cheats/adapters", app.handleCheatAdaptersList)
	r.Get("/cheats/adapters/{id}", app.handleCheatAdapterGet)

	r.Get("/events", app.handleEvents)
}

func (a *app) handleAgentAPIIndex(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	basePath := agentAPIBasePath(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"api": map[string]any{
			"name":          "RetroSaveManager Agent API",
			"version":       "v1",
			"authMode":      "disabled",
			"basePath":      basePath,
			"basePathAlias": agentAPIAlternateBasePath(basePath),
			"docsFile":      "api.md",
			"compatBases":   []string{"/", "/v1"},
		},
		"endpoints": map[string]any{
			"overview":        basePath + "/overview",
			"syncStatus":      basePath + "/sync/status",
			"systems":         basePath + "/systems",
			"devices":         basePath + "/devices",
			"logs":            basePath + "/logs",
			"saves":           basePath + "/saves",
			"saveLatest":      basePath + "/save/latest",
			"roms":            basePath + "/roms",
			"romLookup":       basePath + "/roms/lookup",
			"conflicts":       basePath + "/conflicts",
			"autoEnroll":      basePath + "/helpers/auto-enroll",
			"cheatPacks":      basePath + "/cheats/packs",
			"cheatAdapters":   basePath + "/cheats/adapters",
			"events":          basePath + "/events",
			"bulkDownload":    basePath + "/saves/download-many",
			"saveRescan":      basePath + "/saves/rescan",
			"compatRootBase":  "/",
			"compatAliasBase": "/v1",
		},
	})
}

func (a *app) handleAgentOverview(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	now := time.Now().UTC()
	staleAfter := agentStaleAfter(r)
	devices := a.agentPublicDevices()
	saves := a.aggregatedSaveSummaries("", "", 0)
	roms := a.agentROMEntries(false)
	systems := a.agentSystemItems(saves, devices)
	autoEnroll := a.agentAutoEnrollStatus()
	deviceStatusCounts := map[string]int{"online": 0, "stale": 0}
	latestDeviceSeenAt := time.Time{}
	latestDeviceSyncedAt := time.Time{}
	for _, d := range devices {
		status := agentDeviceStatusLabel(d, staleAfter, now)
		deviceStatusCounts[status]++
		if d.LastSeenAt.After(latestDeviceSeenAt) {
			latestDeviceSeenAt = d.LastSeenAt
		}
		if d.LastSyncedAt.After(latestDeviceSyncedAt) {
			latestDeviceSyncedAt = d.LastSyncedAt
		}
	}
	latestSaveAt := time.Time{}
	totalTrackBytes := 0
	totalVersionCount := 0
	for _, summary := range saves {
		totalTrackBytes += agentSummaryTotalBytes(summary)
		totalVersionCount += maxInt(summary.SaveCount, 1)
		if summary.CreatedAt.After(latestSaveAt) {
			latestSaveAt = summary.CreatedAt
		}
	}
	snapshot := a.authSnapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"generatedAt": now,
		"overview": map[string]any{
			"authMode":   "disabled",
			"autoEnroll": autoEnroll,
			"stats": map[string]any{
				"games":            snapshot.GameCount,
				"saveFiles":        snapshot.FileCount,
				"saveTracks":       len(saves),
				"saveVersions":     totalVersionCount,
				"storageUsedBytes": snapshot.StorageUsedBytes,
				"trackBytes":       totalTrackBytes,
				"devices":          len(devices),
				"systems":          len(systems),
				"roms":             len(roms),
				"conflicts":        a.activeConflictCount(),
			},
			"latest": map[string]any{
				"saveCreatedAt":  zeroTimeToNil(latestSaveAt),
				"deviceSeenAt":   zeroTimeToNil(latestDeviceSeenAt),
				"deviceSyncedAt": zeroTimeToNil(latestDeviceSyncedAt),
			},
			"devices": map[string]any{
				"online":            deviceStatusCounts["online"],
				"stale":             deviceStatusCounts["stale"],
				"staleAfterMinutes": int(staleAfter.Minutes()),
			},
			"systems": systems,
		},
	})
}

func (a *app) handleAgentSyncStatus(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	now := time.Now().UTC()
	staleAfter := agentStaleAfter(r)
	devices := a.agentPublicDevices()
	deviceItems := make([]map[string]any, 0, len(devices))
	for _, d := range devices {
		deviceItems = append(deviceItems, agentDeviceEnvelope(d, staleAfter, now))
	}
	sort.Slice(deviceItems, func(i, j int) bool {
		left := mustTimeFromEnvelope(deviceItems[i], "lastSeenAt")
		right := mustTimeFromEnvelope(deviceItems[j], "lastSeenAt")
		if left.Equal(right) {
			return mustIntFromEnvelope(deviceItems[i], "id") > mustIntFromEnvelope(deviceItems[j], "id")
		}
		return left.After(right)
	})
	saves := a.aggregatedSaveSummaries("", "", 0)
	recentLimit := parseIntOrDefault(r.URL.Query().Get("recentLimit"), 10)
	if recentLimit <= 0 {
		recentLimit = 10
	}
	if recentLimit > len(saves) {
		recentLimit = len(saves)
	}
	recent := make([]map[string]any, 0, recentLimit)
	basePath := agentAPIBasePath(r)
	for _, summary := range saves[:recentLimit] {
		recent = append(recent, map[string]any{
			"save":    summary,
			"actions": agentSaveActionSet(basePath, summary),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success":           true,
		"generatedAt":       now,
		"staleAfterMinutes": int(staleAfter.Minutes()),
		"autoEnroll":        a.agentAutoEnrollStatus(),
		"conflicts": map[string]any{
			"count": a.activeConflictCount(),
		},
		"devices":     deviceItems,
		"recentSaves": recent,
	})
}

func (a *app) handleAgentSystems(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	items := a.agentSystemItems(a.aggregatedSaveSummaries("", "", 0), a.agentPublicDevices())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"systems": items,
		"total":   len(items),
	})
}

func (a *app) handleAgentSavesList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntOrDefault(r.URL.Query().Get("offset"), 0)
	gameID := parseIntOrDefault(r.URL.Query().Get("gameId"), 0)
	romSHA1 := strings.TrimSpace(r.URL.Query().Get("romSha1"))
	romMD5 := strings.TrimSpace(r.URL.Query().Get("romMd5"))
	systemSlug := canonicalOptionalSegment(r.URL.Query().Get("systemSlug"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	items := a.aggregatedSaveSummaries(romSHA1, romMD5, 0)
	filtered := make([]saveSummary, 0, len(items))
	for _, summary := range items {
		if gameID > 0 && summary.Game.ID != gameID {
			continue
		}
		if systemSlug != "" && canonicalOptionalSegment(summary.SystemSlug) != systemSlug {
			continue
		}
		if query != "" && !agentSaveMatchesQuery(summary, query) {
			continue
		}
		filtered = append(filtered, summary)
	}

	summaryStats := map[string]any{
		"tracks":          len(filtered),
		"saveVersions":    0,
		"totalSizeBytes":  0,
		"latestCreatedAt": nil,
	}
	latestCreatedAt := time.Time{}
	for _, item := range filtered {
		summaryStats["saveVersions"] = summaryStats["saveVersions"].(int) + maxInt(item.SaveCount, 1)
		summaryStats["totalSizeBytes"] = summaryStats["totalSizeBytes"].(int) + agentSummaryTotalBytes(item)
		if item.CreatedAt.After(latestCreatedAt) {
			latestCreatedAt = item.CreatedAt
		}
	}
	if !latestCreatedAt.IsZero() {
		summaryStats["latestCreatedAt"] = latestCreatedAt
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	basePath := agentAPIBasePath(r)
	results := make([]map[string]any, 0, end-offset)
	for _, item := range filtered[offset:end] {
		results = append(results, map[string]any{
			"save":    item,
			"actions": agentSaveActionSet(basePath, item),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"filters": map[string]any{
			"romSha1":    emptyStringToNil(romSHA1),
			"romMd5":     emptyStringToNil(romMD5),
			"gameId":     zeroIntToNil(gameID),
			"systemSlug": emptyStringToNil(systemSlug),
			"q":          emptyStringToNil(query),
		},
		"summary": summaryStats,
		"saves":   results,
		"total":   len(filtered),
		"limit":   limit,
		"offset":  offset,
	})
}

func (a *app) handleAgentSaveGet(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	clone := cloneRequestWithQuery(r, map[string]string{"saveId": chi.URLParam(r, "id")})
	a.handleSaveByGame(w, clone)
}

func (a *app) handleAgentSaveDelete(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	clone := cloneRequestWithQuery(r, map[string]string{"id": chi.URLParam(r, "id")})
	a.handleDeleteSave(w, clone)
}

func (a *app) handleAgentSaveDownload(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	clone := cloneRequestWithQuery(r, map[string]string{"id": chi.URLParam(r, "id")})
	a.handleDownloadSave(w, clone)
}

func (a *app) handleAgentSaveRollback(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	var req struct {
		PSLogicalKey string `json:"psLogicalKey"`
		RevisionID   string `json:"revisionId"`
	}
	_ = decodeJSONBody(r, &req)
	payload := map[string]any{
		"saveId": chi.URLParam(r, "id"),
	}
	if value := firstNonEmpty(req.PSLogicalKey, r.URL.Query().Get("psLogicalKey")); strings.TrimSpace(value) != "" {
		payload["psLogicalKey"] = strings.TrimSpace(value)
	}
	if value := firstNonEmpty(req.RevisionID, r.URL.Query().Get("revisionId")); strings.TrimSpace(value) != "" {
		payload["revisionId"] = strings.TrimSpace(value)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	clone := r.Clone(r.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.Header = clone.Header.Clone()
	clone.Header.Set("Content-Type", "application/json")
	a.handleSaveRollback(w, clone)
}

func (a *app) handleAgentSaveRescan(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	var req struct {
		DryRun           *bool `json:"dryRun"`
		PruneUnsupported *bool `json:"pruneUnsupported"`
	}
	_ = decodeJSONBody(r, &req)
	options := saveRescanOptions{DryRun: false, PruneUnsupported: true}
	if req.DryRun != nil {
		options.DryRun = *req.DryRun
	}
	if req.PruneUnsupported != nil {
		options.PruneUnsupported = *req.PruneUnsupported
	}
	result, err := a.rescanSaves(options)
	if err != nil {
		a.appendSyncLog(syncLogInput{
			DeviceName:   "API",
			Action:       "rescan",
			Game:         "Save store",
			ErrorMessage: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	a.appendSyncLog(syncLogInput{
		DeviceName: "API",
		Action:     "rescan",
		Game:       "Save store",
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"options": options,
		"result":  result,
	})
}

func (a *app) handleAgentRomsList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	includeMissing := parseBoolLike(r.URL.Query().Get("includeMissing"))
	systemSlug := canonicalOptionalSegment(r.URL.Query().Get("systemSlug"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntOrDefault(r.URL.Query().Get("offset"), 0)
	roms := a.agentROMEntries(includeMissing)
	filtered := make([]map[string]any, 0, len(roms))
	for _, item := range roms {
		if systemSlug != "" && canonicalOptionalSegment(mustStringValue(item["systemSlug"])) != systemSlug {
			continue
		}
		if query != "" && !agentROMMatchesQuery(item, query) {
			continue
		}
		filtered = append(filtered, item)
	}
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"filters": map[string]any{
			"includeMissing": includeMissing,
			"systemSlug":     emptyStringToNil(systemSlug),
			"q":              emptyStringToNil(query),
		},
		"roms":   filtered[offset:end],
		"total":  len(filtered),
		"limit":  limit,
		"offset": offset,
	})
}

func (a *app) handleAgentRomGet(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	target := strings.TrimSpace(chi.URLParam(r, "hash"))
	if target == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "hash is required", StatusCode: http.StatusBadRequest})
		return
	}
	for _, item := range a.agentROMEntries(true) {
		if strings.EqualFold(target, mustStringValue(item["key"])) || strings.EqualFold(target, mustStringValue(item["romSha1"])) || strings.EqualFold(target, mustStringValue(item["romMd5"])) {
			writeJSON(w, http.StatusOK, map[string]any{"success": true, "rom": item})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "ROM not found", StatusCode: http.StatusNotFound})
}

func (a *app) aggregatedSaveSummaries(romSHA1, romMD5 string, systemID int) []saveSummary {
	records := a.snapshotSaveRecords()
	filteredRecords := make([]saveRecord, 0, len(records))
	for _, record := range records {
		if romSHA1 != "" && record.ROMSHA1 != romSHA1 {
			continue
		}
		if romMD5 != "" && record.ROMMD5 != romMD5 {
			continue
		}
		if systemID != 0 {
			if record.Summary.Game.System == nil || record.Summary.Game.System.ID != systemID {
				continue
			}
		}
		filteredRecords = append(filteredRecords, record)
	}

	type saveAggregate struct {
		representative saveSummary
		saveCount      int
		totalSizeBytes int
		projectionLine bool
	}
	aggregates := make(map[string]saveAggregate, len(filteredRecords))
	for _, record := range filteredRecords {
		recordSystemSlug := canonicalOptionalSegment(saveRecordSystemSlug(record))
		if recordSystemSlug == "psx" || recordSystemSlug == "ps2" {
			continue
		}
		if !isSupportedSystemSlug(saveRecordSystemSlug(record)) {
			continue
		}
		summary := canonicalSummaryForRecord(record)
		if !isSupportedSystemSlug(summary.SystemSlug) {
			continue
		}
		key := canonicalListKeyForRecord(record)
		agg := aggregates[key]
		if _, _, _, ok := playStationProjectionInfoFromRecord(record); ok {
			agg.projectionLine = true
		}
		if agg.saveCount == 0 || summary.CreatedAt.After(agg.representative.CreatedAt) || (summary.CreatedAt.Equal(agg.representative.CreatedAt) && summary.ID > agg.representative.ID) {
			agg.representative = summary
		}
		if agg.projectionLine {
			if agg.saveCount < 1 {
				agg.saveCount = 1
			}
			if record.Summary.FileSize > agg.totalSizeBytes {
				agg.totalSizeBytes = record.Summary.FileSize
			}
		} else {
			agg.saveCount++
			agg.totalSizeBytes += record.Summary.FileSize
		}
		aggregates[key] = agg
	}

	filtered := make([]saveSummary, 0, len(aggregates))
	for _, agg := range aggregates {
		summary := agg.representative
		summary.SaveCount = agg.saveCount
		summary.TotalSizeBytes = agg.totalSizeBytes
		summary.LatestSizeBytes = summary.FileSize
		summary.LatestVersion = summary.Version
		filtered = append(filtered, summary)
	}
	if romSHA1 == "" && romMD5 == "" {
		filtered = append(filtered, a.playStationLogicalListSummaries(systemID)...)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].ID > filtered[j].ID
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	return filtered
}

func (a *app) agentPublicDevices() []device {
	a.mu.Lock()
	defer a.mu.Unlock()
	items := make([]device, 0, len(a.devices))
	for _, d := range a.devices {
		items = append(items, a.publicDeviceLocked(d))
	}
	return items
}

func (a *app) agentAutoEnrollStatus() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	active := a.autoAppPasswordWindowActiveLocked(time.Now().UTC())
	return map[string]any{
		"active":       active,
		"enabledUntil": copyTimePtr(a.autoAppPasswordEnabledUntil),
	}
}

func (a *app) agentSystemItems(summaries []saveSummary, devices []device) []map[string]any {
	type systemAgg struct {
		system          system
		saveTracks      int
		saveVersions    int
		totalSizeBytes  int
		gameIDs         map[int]struct{}
		allowedDevices  map[int]struct{}
		reportedDevices map[int]struct{}
		latestCreatedAt time.Time
	}
	aggs := map[string]*systemAgg{}
	for _, item := range a.saveSystemsCatalog() {
		normalized := normalizeSystemCatalogEntry(item)
		aggs[normalized.Slug] = &systemAgg{
			system:          normalized,
			gameIDs:         map[int]struct{}{},
			allowedDevices:  map[int]struct{}{},
			reportedDevices: map[int]struct{}{},
		}
	}
	for _, summary := range summaries {
		sys := agentSystemFromSummary(summary)
		agg, ok := aggs[sys.Slug]
		if !ok {
			agg = &systemAgg{system: sys, gameIDs: map[int]struct{}{}, allowedDevices: map[int]struct{}{}, reportedDevices: map[int]struct{}{}}
			aggs[sys.Slug] = agg
		}
		agg.saveTracks++
		agg.saveVersions += maxInt(summary.SaveCount, 1)
		agg.totalSizeBytes += agentSummaryTotalBytes(summary)
		agg.gameIDs[summary.Game.ID] = struct{}{}
		if summary.CreatedAt.After(agg.latestCreatedAt) {
			agg.latestCreatedAt = summary.CreatedAt
		}
	}
	for _, d := range devices {
		if d.SyncAll {
			for slug, agg := range aggs {
				if slug == "" || slug == "unknown-system" {
					continue
				}
				agg.allowedDevices[d.ID] = struct{}{}
			}
		} else {
			for _, slug := range normalizeAllowedSystemSlugs(d.AllowedSystemSlugs) {
				if agg, ok := aggs[slug]; ok {
					agg.allowedDevices[d.ID] = struct{}{}
				}
			}
		}
		for _, slug := range normalizeAllowedSystemSlugs(d.ReportedSystemSlugs) {
			if agg, ok := aggs[slug]; ok {
				agg.reportedDevices[d.ID] = struct{}{}
			}
		}
	}
	items := make([]map[string]any, 0, len(aggs))
	for _, agg := range aggs {
		items = append(items, map[string]any{
			"system":          agg.system,
			"saveTracks":      agg.saveTracks,
			"saveVersions":    agg.saveVersions,
			"gameCount":       len(agg.gameIDs),
			"totalSizeBytes":  agg.totalSizeBytes,
			"allowedDevices":  len(agg.allowedDevices),
			"reportedDevices": len(agg.reportedDevices),
			"latestCreatedAt": zeroTimeToNil(agg.latestCreatedAt),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		left := mustSystemFromEnvelope(items[i])
		right := mustSystemFromEnvelope(items[j])
		if left.Manufacturer == right.Manufacturer {
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		}
		if left.Manufacturer == "Other" {
			return false
		}
		if right.Manufacturer == "Other" {
			return true
		}
		return strings.ToLower(left.Manufacturer) < strings.ToLower(right.Manufacturer)
	})
	return items
}

func (a *app) agentROMEntries(includeMissing bool) []map[string]any {
	records := a.snapshotSaveRecords()
	type romAgg struct {
		Key             string
		RomSHA1         string
		RomMD5          string
		SystemSlug      string
		SystemName      string
		GameID          int
		GameName        string
		DisplayTitle    string
		RegionCode      string
		SaveCount       int
		LatestSaveID    string
		LatestVersion   int
		LatestCreatedAt time.Time
		LatestSizeBytes int
		TotalSizeBytes  int
		SlotNames       map[string]struct{}
		SaveIDs         []string
	}
	groups := map[string]*romAgg{}
	for _, record := range records {
		systemSlug := canonicalOptionalSegment(saveRecordSystemSlug(record))
		if !isSupportedSystemSlug(systemSlug) {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(record.ROMSHA1), "ps-projection::") {
			continue
		}
		key := firstNonEmpty(
			func() string {
				if strings.TrimSpace(record.ROMSHA1) != "" {
					return "sha1:" + strings.ToLower(strings.TrimSpace(record.ROMSHA1))
				}
				return ""
			}(),
			func() string {
				if strings.TrimSpace(record.ROMMD5) != "" {
					return "md5:" + strings.ToLower(strings.TrimSpace(record.ROMMD5))
				}
				return ""
			}(),
		)
		if key == "" {
			if !includeMissing {
				continue
			}
			key = "missing:" + record.Summary.ID
		}
		displayTitle := strings.TrimSpace(firstNonEmpty(record.Summary.DisplayTitle, record.Summary.Game.DisplayTitle, record.Summary.Game.Name))
		if displayTitle == "" {
			displayTitle = "Unknown game"
		}
		agg, ok := groups[key]
		if !ok {
			agg = &romAgg{
				Key:          key,
				RomSHA1:      strings.TrimSpace(record.ROMSHA1),
				RomMD5:       strings.TrimSpace(record.ROMMD5),
				SystemSlug:   systemSlug,
				SystemName:   agentSystemDisplayName(agentSystemFromSummary(record.Summary)),
				GameID:       record.Summary.Game.ID,
				GameName:     strings.TrimSpace(record.Summary.Game.Name),
				DisplayTitle: displayTitle,
				RegionCode:   record.Summary.RegionCode,
				SlotNames:    map[string]struct{}{},
				SaveIDs:      []string{},
			}
			groups[key] = agg
		}
		agg.SaveCount++
		agg.TotalSizeBytes += record.Summary.FileSize
		agg.LatestSizeBytes = maxInt(agg.LatestSizeBytes, record.Summary.FileSize)
		if record.Summary.CreatedAt.After(agg.LatestCreatedAt) || (record.Summary.CreatedAt.Equal(agg.LatestCreatedAt) && record.Summary.ID > agg.LatestSaveID) {
			agg.LatestCreatedAt = record.Summary.CreatedAt
			agg.LatestSaveID = record.Summary.ID
			agg.LatestVersion = record.Summary.Version
		}
		if slot := normalizedSlot(record.SlotName); slot != "" {
			agg.SlotNames[slot] = struct{}{}
		}
		agg.SaveIDs = append(agg.SaveIDs, record.Summary.ID)
	}
	items := make([]map[string]any, 0, len(groups))
	for _, agg := range groups {
		slotNames := make([]string, 0, len(agg.SlotNames))
		for slot := range agg.SlotNames {
			slotNames = append(slotNames, slot)
		}
		sort.Strings(slotNames)
		sort.Strings(agg.SaveIDs)
		items = append(items, map[string]any{
			"key":             agg.Key,
			"romSha1":         emptyStringToNil(agg.RomSHA1),
			"romMd5":          emptyStringToNil(agg.RomMD5),
			"systemSlug":      agg.SystemSlug,
			"systemName":      agg.SystemName,
			"gameId":          agg.GameID,
			"gameName":        agg.GameName,
			"displayTitle":    agg.DisplayTitle,
			"regionCode":      agg.RegionCode,
			"saveCount":       agg.SaveCount,
			"latestSaveId":    agg.LatestSaveID,
			"latestVersion":   agg.LatestVersion,
			"latestCreatedAt": zeroTimeToNil(agg.LatestCreatedAt),
			"latestSizeBytes": agg.LatestSizeBytes,
			"totalSizeBytes":  agg.TotalSizeBytes,
			"slotNames":       slotNames,
			"saveIds":         agg.SaveIDs,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		left := mustTimeFromAny(items[i]["latestCreatedAt"])
		right := mustTimeFromAny(items[j]["latestCreatedAt"])
		if left.Equal(right) {
			return mustStringValue(items[i]["key"]) < mustStringValue(items[j]["key"])
		}
		return left.After(right)
	})
	return items
}

func cloneRequestWithQuery(r *http.Request, updates map[string]string) *http.Request {
	clone := r.Clone(r.Context())
	query := clone.URL.Query()
	for key, value := range updates {
		if strings.TrimSpace(value) == "" {
			continue
		}
		query.Set(key, value)
	}
	clone.URL.RawQuery = query.Encode()
	return clone
}

func agentAPIBasePath(r *http.Request) string {
	clean := path.Clean("/" + strings.TrimSpace(r.URL.Path))
	if clean == "/api/v1" || strings.HasPrefix(clean, "/api/v1/") {
		return "/api/v1"
	}
	return "/api"
}

func agentAPIAlternateBasePath(basePath string) string {
	if basePath == "/api/v1" {
		return "/api"
	}
	return "/api/v1"
}

func agentSaveActionSet(basePath string, summary saveSummary) map[string]any {
	values := url.Values{}
	if strings.TrimSpace(summary.LogicalKey) != "" {
		values.Set("psLogicalKey", summary.LogicalKey)
	}
	query := values.Encode()
	detailURL := fmt.Sprintf("%s/saves/%s", basePath, url.PathEscape(summary.ID))
	downloadURL := fmt.Sprintf("%s/saves/%s/download", basePath, url.PathEscape(summary.ID))
	deleteURL := fmt.Sprintf("%s/saves/%s", basePath, url.PathEscape(summary.ID))
	rollbackURL := fmt.Sprintf("%s/saves/%s/rollback", basePath, url.PathEscape(summary.ID))
	if query != "" {
		detailURL += "?" + query
		downloadURL += "?" + query
		deleteURL += "?" + query
		rollbackURL += "?" + query
	}
	return map[string]any{
		"detail":   detailURL,
		"download": downloadURL,
		"delete":   deleteURL,
		"rollback": rollbackURL,
	}
}

func agentSaveMatchesQuery(summary saveSummary, query string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return true
	}
	candidates := []string{
		summary.DisplayTitle,
		summary.Game.DisplayTitle,
		summary.Game.Name,
		summary.Filename,
		summary.SystemSlug,
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(strings.TrimSpace(candidate)), needle) {
			return true
		}
	}
	return false
}

func agentROMMatchesQuery(item map[string]any, query string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return true
	}
	for _, key := range []string{"key", "romSha1", "romMd5", "systemSlug", "systemName", "gameName", "displayTitle"} {
		if strings.Contains(strings.ToLower(mustStringValue(item[key])), needle) {
			return true
		}
	}
	return false
}

func agentDeviceEnvelope(d device, staleAfter time.Duration, now time.Time) map[string]any {
	lastSeenAgeSeconds := 0
	if !d.LastSeenAt.IsZero() {
		lastSeenAgeSeconds = int(now.Sub(d.LastSeenAt).Seconds())
	}
	lastSyncedAgeSeconds := 0
	if !d.LastSyncedAt.IsZero() {
		lastSyncedAgeSeconds = int(now.Sub(d.LastSyncedAt).Seconds())
	}
	return map[string]any{
		"id":                   d.ID,
		"status":               agentDeviceStatusLabel(d, staleAfter, now),
		"lastSeenAgeSeconds":   maxInt(lastSeenAgeSeconds, 0),
		"lastSyncedAgeSeconds": maxInt(lastSyncedAgeSeconds, 0),
		"appPasswordBound":     d.BoundAppPasswordID != nil && strings.TrimSpace(*d.BoundAppPasswordID) != "",
		"allowedSystemCount":   len(normalizeAllowedSystemSlugs(d.AllowedSystemSlugs)),
		"reportedSystemCount":  len(normalizeAllowedSystemSlugs(d.ReportedSystemSlugs)),
		"syncPathCount":        len(normalizeHelperPaths(d.SyncPaths)),
		"lastSeenAt":           zeroTimeToNil(d.LastSeenAt),
		"lastSyncedAt":         zeroTimeToNil(d.LastSyncedAt),
		"device":               d,
	}
}

func agentDeviceStatusLabel(d device, staleAfter time.Duration, now time.Time) string {
	seenAt := d.LastSeenAt
	if seenAt.IsZero() {
		seenAt = d.LastSyncedAt
	}
	if seenAt.IsZero() {
		return "stale"
	}
	if now.Sub(seenAt) <= staleAfter {
		return "online"
	}
	return "stale"
}

func agentSystemFromSummary(summary saveSummary) system {
	if summary.Game.System != nil {
		return normalizeSystemCatalogEntry(*summary.Game.System)
	}
	if known := supportedSystemFromSlug(summary.SystemSlug); known != nil {
		return *known
	}
	return normalizeSystemCatalogEntry(system{Slug: summary.SystemSlug, Name: toDisplayWords(summary.SystemSlug)})
}

func agentSystemDisplayName(sys system) string {
	if strings.TrimSpace(sys.Name) != "" {
		return sys.Name
	}
	return toDisplayWords(sys.Slug)
}

func agentSummaryTotalBytes(summary saveSummary) int {
	if summary.TotalSizeBytes > 0 {
		return summary.TotalSizeBytes
	}
	if summary.LatestSizeBytes > 0 {
		return summary.LatestSizeBytes
	}
	return summary.FileSize
}

func agentStaleAfter(r *http.Request) time.Duration {
	minutes := parseIntOrDefault(r.URL.Query().Get("staleAfterMinutes"), 30)
	if minutes <= 0 {
		minutes = 30
	}
	return time.Duration(minutes) * time.Minute
}

func parseBoolLike(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func emptyStringToNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func zeroIntToNil(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func zeroTimeToNil(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func mustStringValue(value any) string {
	s, _ := value.(string)
	return strings.TrimSpace(s)
}

func mustTimeFromAny(value any) time.Time {
	t, _ := value.(time.Time)
	return t
}

func mustTimeFromEnvelope(item map[string]any, field string) time.Time {
	return mustTimeFromAny(item[field])
}

func mustIntFromEnvelope(item map[string]any, field string) int {
	v, _ := item[field].(int)
	return v
}

func mustSystemFromEnvelope(item map[string]any) system {
	value, _ := item["system"].(system)
	return value
}
