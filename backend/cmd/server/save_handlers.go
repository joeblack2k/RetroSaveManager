package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (a *app) handleSaveLatest(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	helperCtx, ok := a.authorizeHelperSyncRequest(w, r, nil)
	if !ok {
		return
	}

	romSHA1 := strings.TrimSpace(r.URL.Query().Get("romSha1"))
	slotName := normalizedSlot(r.URL.Query().Get("slotName"))

	latest, ok := a.latestSaveRecord(romSHA1, slotName)
	if ok {
		if helperCtx.IsHelper && !systemAllowedForDevice(helperCtx.Device, saveRecordSystemSlug(latest)) {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": true,
				"exists":  false,
				"sha256":  nil,
				"version": nil,
				"id":      nil,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"exists":  true,
			"sha256":  latest.Summary.SHA256,
			"version": latest.Summary.Version,
			"id":      latest.Summary.ID,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"exists":  false,
		"sha256":  nil,
		"version": nil,
		"id":      nil,
	})
}

func (a *app) handleSaves(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: err.Error(), StatusCode: http.StatusBadRequest})
			return
		}
		formValue := func(key string) string {
			return r.FormValue(key)
		}
		helperCtx, authorized := a.authorizeHelperSyncRequest(w, r, formValue)
		if !authorized {
			return
		}
		identity := extractHelperIdentity(r, formValue)

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "File is required", StatusCode: http.StatusBadRequest})
			return
		}
		defer file.Close()

		payload, err := io.ReadAll(file)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: err.Error(), StatusCode: http.StatusBadRequest})
			return
		}

		filename := header.Filename
		gameInfo := fallbackGameFromFilename(filename)
		input := saveCreateInput{
			Filename:   filename,
			Payload:    payload,
			Game:       gameInfo,
			Format:     inferSaveFormat(filename),
			Metadata:   nil,
			ROMSHA1:    strings.TrimSpace(formValue("rom_sha1")),
			ROMMD5:     strings.TrimSpace(formValue("rom_md5")),
			SlotName:   strings.TrimSpace(formValue("slotName")),
			SystemSlug: safeMultipartSystemSlug(formValue("system"), gameInfo.System),
			GameSlug:   canonicalSegment(gameInfo.Name, "unknown-game"),
		}
		preview := a.normalizeSaveInputDetailed(input)
		if preview.Rejected || !isSupportedSystemSlug(preview.Input.SystemSlug) {
			writeJSON(w, http.StatusUnprocessableEntity, apiError{
				Error:      "Unprocessable Entity",
				Message:    errUnsupportedSaveFormat.Error(),
				StatusCode: http.StatusUnprocessableEntity,
			})
			return
		}
		if helperCtx.IsHelper && !systemAllowedForDevice(helperCtx.Device, preview.Input.SystemSlug) {
			writeJSON(w, http.StatusForbidden, apiError{
				Error:      "Forbidden",
				Message:    "this device is not allowed to sync saves for this console",
				StatusCode: http.StatusForbidden,
			})
			return
		}

		record, err := a.createSave(input)
		if err != nil {
			if errors.Is(err, errUnsupportedSaveFormat) {
				writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: errUnsupportedSaveFormat.Error(), StatusCode: http.StatusUnprocessableEntity})
				return
			}
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
			return
		}

		if !helperCtx.IsHelper && identity.isComplete() {
			a.upsertDevice(identity.DeviceType, identity.Fingerprint)
		}

		a.saveCreatedEvent(record)
		a.resolveConflictForSave(record)

		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"save": map[string]any{
				"id":     record.Summary.ID,
				"sha256": record.Summary.SHA256,
			},
		})
		return
	}

	var req saveBatchUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeText(w, http.StatusUnprocessableEntity, "Failed to deserialize the JSON body into the target type")
		return
	}

	results := make([]any, 0, len(req.Items))
	successCount := 0
	errorCount := 0

	for _, item := range req.Items {
		payload, err := decodeSaveBatchData(item.Data)
		if err != nil {
			errorCount++
			results = append(results, map[string]any{
				"filename": item.Filename,
				"success":  false,
				"error":    "invalid base64 data",
			})
			continue
		}

		gameInfo := buildBatchGame(item.Game, item.Filename)
		record, err := a.createSave(saveCreateInput{
			Filename:   item.Filename,
			Payload:    payload,
			Game:       gameInfo,
			Format:     inferSaveFormat(item.Filename),
			Metadata:   nil,
			SystemSlug: safeMultipartSystemSlug("", gameInfo.System),
			GameSlug:   canonicalSegment(gameInfo.Name, "unknown-game"),
		})
		if err != nil {
			if errors.Is(err, errUnsupportedSaveFormat) {
				errorCount++
				results = append(results, map[string]any{
					"filename": item.Filename,
					"success":  false,
					"error":    errUnsupportedSaveFormat.Error(),
				})
				continue
			}
			errorCount++
			results = append(results, map[string]any{
				"filename": item.Filename,
				"success":  false,
				"error":    err.Error(),
			})
			continue
		}
		successCount++
		results = append(results, map[string]any{
			"filename": item.Filename,
			"success":  true,
			"save": map[string]any{
				"id":      record.Summary.ID,
				"sha256":  record.Summary.SHA256,
				"version": record.Summary.Version,
			},
		})
		a.saveCreatedEvent(record)
		a.resolveConflictForSave(record)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":      results,
		"successCount": successCount,
		"errorCount":   errorCount,
	})
}

func (a *app) handleListSaves(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntOrDefault(r.URL.Query().Get("offset"), 0)
	romSHA1 := strings.TrimSpace(r.URL.Query().Get("romSha1"))
	romMD5 := strings.TrimSpace(r.URL.Query().Get("romMd5"))
	systemID := parseIntOrDefault(r.URL.Query().Get("systemId"), 0)

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
	}
	aggregates := make(map[string]saveAggregate, len(filteredRecords))
	for _, record := range filteredRecords {
		if !isSupportedSystemSlug(saveRecordSystemSlug(record)) {
			continue
		}
		summary := canonicalSummaryForRecord(record)
		if !isSupportedSystemSlug(summary.SystemSlug) {
			continue
		}
		key := canonicalHistoryKeyForRecord(record)
		agg := aggregates[key]
		if agg.saveCount == 0 || summary.CreatedAt.After(agg.representative.CreatedAt) || (summary.CreatedAt.Equal(agg.representative.CreatedAt) && summary.ID > agg.representative.ID) {
			agg.representative = summary
		}
		agg.saveCount++
		agg.totalSizeBytes += record.Summary.FileSize
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
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].ID > filtered[j].ID
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"saves":   filtered[offset:end],
		"total":   len(filtered),
		"limit":   limit,
		"offset":  offset,
	})
}

func (a *app) handleSaveSystems(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	writeJSON(w, http.StatusOK, a.saveSystemsCatalog())
}

func (a *app) handleSaveByGame(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	gameID := parseIntOrDefault(r.URL.Query().Get("gameId"), 0)
	saveID := strings.TrimSpace(r.URL.Query().Get("saveId"))
	systemSlug := canonicalOptionalSegment(r.URL.Query().Get("systemSlug"))
	displayTitle := strings.TrimSpace(r.URL.Query().Get("displayTitle"))
	sourceRecord := saveRecord{}
	hasSourceRecord := false

	if saveID != "" {
		record, ok := a.findSaveRecordByID(saveID)
		if ok {
			sourceRecord = record
			hasSourceRecord = true
			if gameID == 0 {
				gameID = record.Summary.Game.ID
			}
			if systemSlug == "" {
				systemSlug = canonicalOptionalSegment(record.SystemSlug)
				if systemSlug == "" && record.Summary.Game.System != nil {
					systemSlug = canonicalOptionalSegment(record.Summary.Game.System.Slug)
				}
			}
			if displayTitle == "" {
				displayTitle = strings.TrimSpace(record.Summary.DisplayTitle)
			}
		}
	}
	if gameID == 0 && saveID == "" && systemSlug == "" && displayTitle == "" {
		for _, record := range a.snapshotSaveRecords() {
			if !isSupportedSystemSlug(saveRecordSystemSlug(record)) {
				continue
			}
			sourceRecord = record
			hasSourceRecord = true
			gameID = canonicalSummaryForRecord(record).Game.ID
			break
		}
	}

	records := a.snapshotSaveRecords()
	var versions []saveSummary
	for _, record := range records {
		if !isSupportedSystemSlug(saveRecordSystemSlug(record)) {
			continue
		}
		if hasSourceRecord && !sameSaveHistoryTrack(record, sourceRecord) {
			continue
		}
		s := canonicalSummaryForRecord(record)
		if gameID != 0 && s.Game.ID != gameID {
			continue
		}
		recordSystem := canonicalOptionalSegment(s.SystemSlug)
		if systemSlug != "" && recordSystem != systemSlug {
			continue
		}
		if displayTitle != "" && !strings.EqualFold(s.DisplayTitle, canonicalDisplayTitle(displayTitle)) {
			continue
		}
		versions = append(versions, s)
	}

	sort.Slice(versions, func(i, j int) bool {
		if versions[i].CreatedAt.Equal(versions[j].CreatedAt) {
			if versions[i].Version == versions[j].Version {
				return versions[i].ID > versions[j].ID
			}
			return versions[i].Version > versions[j].Version
		}
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})

	if len(versions) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "game": nil, "versions": []any{}})
		return
	}

	totalSizeBytes := 0
	regionCode := regionUnknown
	languageCodes := make([]string, 0, 4)
	for _, version := range versions {
		totalSizeBytes += version.FileSize
		if normalizeRegionCode(regionCode) == regionUnknown && normalizeRegionCode(version.RegionCode) != regionUnknown {
			regionCode = normalizeRegionCode(version.RegionCode)
		}
		if len(languageCodes) == 0 && len(version.LanguageCodes) > 0 {
			languageCodes = append(languageCodes, version.LanguageCodes...)
		}
	}
	latest := versions[0]
	displayTitleOut := strings.TrimSpace(latest.DisplayTitle)
	if displayTitleOut == "" || displayTitleOut == "Unknown Game" {
		displayTitleOut = strings.TrimSpace(latest.Game.DisplayTitle)
	}
	if displayTitleOut == "" || displayTitleOut == "Unknown Game" {
		displayTitleOut = strings.TrimSpace(latest.Game.Name)
	}
	if displayTitleOut == "" {
		displayTitleOut = "Unknown Game"
	}
	systemSlugOut := systemSlug
	if systemSlugOut == "" && latest.Game.System != nil {
		systemSlugOut = canonicalOptionalSegment(latest.Game.System.Slug)
		if systemSlugOut == "" {
			systemSlugOut = canonicalSegment(latest.Game.System.Name, "unknown-system")
		}
	}
	if normalizeRegionCode(regionCode) == regionUnknown {
		regionCode = normalizeRegionCode(latest.RegionCode)
	}
	if len(languageCodes) == 0 {
		languageCodes = normalizeLanguageCodes(latest.LanguageCodes)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"game":         latest.Game,
		"displayTitle": displayTitleOut,
		"systemSlug":   systemSlugOut,
		"summary": map[string]any{
			"displayTitle":    displayTitleOut,
			"system":          latest.Game.System,
			"regionCode":      normalizeRegionCode(regionCode),
			"regionFlag":      regionFlagFromCode(regionCode),
			"languageCodes":   languageCodes,
			"saveCount":       len(versions),
			"totalSizeBytes":  totalSizeBytes,
			"latestVersion":   latest.Version,
			"latestCreatedAt": latest.CreatedAt,
		},
		"versions": versions,
	})
}

func sameSaveHistoryTrack(candidate saveRecord, source saveRecord) bool {
	return canonicalHistoryKeyForRecord(candidate) == canonicalHistoryKeyForRecord(source)
}

func canonicalOptionalSegment(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return canonicalSegment(value, "")
}

func canonicalSummaryForRecord(record saveRecord) saveSummary {
	summary := record.Summary
	track := canonicalTrackFromRecord(record)
	summary.SystemSlug = canonicalSegment(track.SystemSlug, "unknown-system")
	summary.DisplayTitle = track.DisplayTitle
	summary.RegionCode = canonicalRegion(summary.RegionCode, track.RegionCode)
	summary.RegionFlag = regionFlagFromCode(summary.RegionCode)
	summary.LanguageCodes = normalizeLanguageCodes(summary.LanguageCodes)
	if len(summary.LanguageCodes) == 0 {
		summary.LanguageCodes = normalizeLanguageCodes(summary.Game.LanguageCodes)
	}
	summary.Game.ID = canonicalGameIDForTrack(track)
	summary.Game.Name = track.DisplayTitle
	summary.Game.DisplayTitle = track.DisplayTitle
	summary.Game.RegionCode = summary.RegionCode
	summary.Game.RegionFlag = summary.RegionFlag
	summary.Game.LanguageCodes = summary.LanguageCodes
	summary.Game.System = track.System
	if !track.IsMemoryCard {
		summary.MemoryCard = nil
	}
	if strings.TrimSpace(summary.CoverArtURL) == "" {
		summary.CoverArtURL = strings.TrimSpace(summary.Game.CoverArtURL)
	}
	if strings.TrimSpace(summary.CoverArtURL) == "" && summary.Game.BoxartThumb != nil {
		summary.CoverArtURL = strings.TrimSpace(*summary.Game.BoxartThumb)
	}
	if strings.TrimSpace(summary.CoverArtURL) == "" && summary.Game.Boxart != nil {
		summary.CoverArtURL = strings.TrimSpace(*summary.Game.Boxart)
	}
	return summary
}

func (a *app) handleSaveRollback(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	var req struct {
		SaveID string `json:"saveId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "invalid JSON body", StatusCode: http.StatusBadRequest})
		return
	}
	targetID := strings.TrimSpace(req.SaveID)
	if targetID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "saveId is required", StatusCode: http.StatusBadRequest})
		return
	}

	sourceRecord, ok := a.findSaveRecordByID(targetID)
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Save not found", StatusCode: http.StatusNotFound})
		return
	}
	payload, err := os.ReadFile(sourceRecord.payloadPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}

	rollbackMeta := mergeRollbackMetadata(sourceRecord)
	newRecord, err := a.createSave(saveCreateInput{
		Filename:      sourceRecord.Summary.Filename,
		Payload:       payload,
		Game:          sourceRecord.Summary.Game,
		Format:        sourceRecord.Summary.Format,
		Metadata:      rollbackMeta,
		ROMSHA1:       sourceRecord.ROMSHA1,
		ROMMD5:        sourceRecord.ROMMD5,
		SlotName:      sourceRecord.SlotName,
		SystemSlug:    sourceRecord.SystemSlug,
		GameSlug:      sourceRecord.GameSlug,
		SystemPath:    sourceRecord.SystemPath,
		GamePath:      sourceRecord.GamePath,
		DisplayTitle:  sourceRecord.Summary.DisplayTitle,
		RegionCode:    sourceRecord.Summary.RegionCode,
		RegionFlag:    sourceRecord.Summary.RegionFlag,
		LanguageCodes: sourceRecord.Summary.LanguageCodes,
		CoverArtURL:   sourceRecord.Summary.CoverArtURL,
		MemoryCard:    sourceRecord.Summary.MemoryCard,
		CreatedAt:     time.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, errUnsupportedSaveFormat) {
			writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: errUnsupportedSaveFormat.Error(), StatusCode: http.StatusUnprocessableEntity})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}

	a.saveCreatedEvent(newRecord)
	a.resolveConflictForSave(newRecord)

	writeJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"sourceSaveId": sourceRecord.Summary.ID,
		"save":         newRecord.Summary,
	})
}

func mergeRollbackMetadata(source saveRecord) any {
	rollbackAudit := map[string]any{
		"action":        "rollback-promote-copy",
		"sourceSaveId":  source.Summary.ID,
		"sourceVersion": source.Summary.Version,
		"sourceSHA256":  source.Summary.SHA256,
		"rolledBackAt":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if source.Summary.Metadata == nil {
		return map[string]any{"rollback": rollbackAudit}
	}
	if existing, ok := source.Summary.Metadata.(map[string]any); ok {
		merged := make(map[string]any, len(existing)+1)
		for key, value := range existing {
			merged[key] = value
		}
		merged["rollback"] = rollbackAudit
		return merged
	}
	return map[string]any{
		"rollback":       rollbackAudit,
		"sourceMetadata": source.Summary.Metadata,
	}
}

func (a *app) handleDeleteSave(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	saveID := strings.TrimSpace(r.URL.Query().Get("id"))
	if saveID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "id is required", StatusCode: http.StatusBadRequest})
		return
	}

	remainingVersions, found, err := a.deleteSaveRecordsByIDs([]string{saveID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Save not found", StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "remainingVersions": remainingVersions})
}

func (a *app) handleDeleteManySaves(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	ids := splitCSV(r.URL.Query().Get("ids"))
	if len(ids) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "ids is required", StatusCode: http.StatusBadRequest})
		return
	}

	remainingVersions, _, err := a.deleteSaveRecordsByIDs(ids)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "remainingVersions": remainingVersions})
}

func (a *app) handleDeleteGameSaves(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	gameIDs := parseCSVInts(r.URL.Query().Get("gameIds"))
	if len(gameIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "gameIds is required", StatusCode: http.StatusBadRequest})
		return
	}

	remainingVersions, err := a.deleteSaveRecordsByGameIDs(gameIDs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "remainingVersions": remainingVersions})
}

func (a *app) handleDownloadSave(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	helperCtx, ok := a.authorizeHelperSyncRequest(w, r, nil)
	if !ok {
		return
	}

	saveID := strings.TrimSpace(r.URL.Query().Get("id"))
	if saveID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "id is required", StatusCode: http.StatusBadRequest})
		return
	}

	record, ok := a.findSaveRecordByID(saveID)
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Save not found", StatusCode: http.StatusNotFound})
		return
	}
	if helperCtx.IsHelper && !systemAllowedForDevice(helperCtx.Device, saveRecordSystemSlug(record)) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "Forbidden", Message: "this device is not allowed to download saves for this console", StatusCode: http.StatusForbidden})
		return
	}

	payload, err := os.ReadFile(record.payloadPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+record.Summary.Filename+`"`)
	_, _ = w.Write(payload)
}

func (a *app) handleDownloadManySaves(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	helperCtx, ok := a.authorizeHelperSyncRequest(w, r, nil)
	if !ok {
		return
	}

	ids := splitCSV(r.URL.Query().Get("ids"))
	if len(ids) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "ids is required", StatusCode: http.StatusBadRequest})
		return
	}

	records := a.saveRecordsByIDs(ids)
	if len(records) == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "No saves found", StatusCode: http.StatusNotFound})
		return
	}
	if helperCtx.IsHelper {
		filtered := make([]saveRecord, 0, len(records))
		for _, record := range records {
			if systemAllowedForDevice(helperCtx.Device, saveRecordSystemSlug(record)) {
				filtered = append(filtered, record)
			}
		}
		records = filtered
		if len(records) == 0 {
			writeJSON(w, http.StatusForbidden, apiError{Error: "Forbidden", Message: "this device is not allowed to download saves for these consoles", StatusCode: http.StatusForbidden})
			return
		}
	}

	archive, err := zipRecords(records)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="saves.zip"`)
	_, _ = w.Write(archive)
}

func safeMultipartSystemSlug(raw string, sys *system) string {
	if strings.TrimSpace(raw) != "" {
		if normalized := supportedSystemSlugFromLabel(raw); normalized != "" {
			return normalized
		}
		return canonicalSegment(raw, "unknown-system")
	}
	if sys != nil {
		if normalized := supportedSystemSlugFromLabel(firstNonEmpty(sys.Slug, sys.Name)); normalized != "" {
			return normalized
		}
		return canonicalSegment(sys.Slug, "unknown-system")
	}
	return "unknown-system"
}

func (a *app) snapshotSaveRecords() []saveRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	records := make([]saveRecord, len(a.saveRecords))
	copy(records, a.saveRecords)
	return records
}

func (a *app) findSaveRecordByID(saveID string) (saveRecord, bool) {
	targetID := strings.TrimSpace(saveID)
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, record := range a.saveRecords {
		if record.Summary.ID == targetID {
			return record, true
		}
	}
	return saveRecord{}, false
}

func (a *app) saveRecordsByIDs(ids []string) []saveRecord {
	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if clean := strings.TrimSpace(id); clean != "" {
			idSet[clean] = struct{}{}
		}
	}

	records := a.snapshotSaveRecords()
	selected := make([]saveRecord, 0, len(idSet))
	for _, record := range records {
		if _, ok := idSet[record.Summary.ID]; ok {
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

func (a *app) deleteSaveRecordsByIDs(ids []string) (int, bool, error) {
	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if clean := strings.TrimSpace(id); clean != "" {
			idSet[clean] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return 0, false, nil
	}

	records := a.snapshotSaveRecords()
	targets := make([]saveRecord, 0, len(idSet))
	affectedGames := map[int]struct{}{}
	for _, record := range records {
		if _, ok := idSet[record.Summary.ID]; !ok {
			continue
		}
		targets = append(targets, record)
		affectedGames[record.Summary.Game.ID] = struct{}{}
	}
	if len(targets) == 0 {
		return len(records), false, nil
	}

	for _, record := range targets {
		if err := os.RemoveAll(record.dirPath); err != nil {
			return 0, true, err
		}
	}
	if err := a.reloadSavesFromDisk(); err != nil {
		return 0, true, err
	}
	return a.remainingVersionsForGames(affectedGames), true, nil
}

func (a *app) deleteSaveRecordsByGameIDs(gameIDs []int) (int, error) {
	gameSet := make(map[int]struct{}, len(gameIDs))
	for _, gameID := range gameIDs {
		if gameID > 0 {
			gameSet[gameID] = struct{}{}
		}
	}
	if len(gameSet) == 0 {
		return 0, nil
	}

	records := a.snapshotSaveRecords()
	for _, record := range records {
		if _, ok := gameSet[record.Summary.Game.ID]; !ok {
			continue
		}
		if err := os.RemoveAll(record.dirPath); err != nil {
			return 0, err
		}
	}
	if err := a.reloadSavesFromDisk(); err != nil {
		return 0, err
	}
	return a.remainingVersionsForGames(gameSet), nil
}

func (a *app) remainingVersionsForGames(gameIDs map[int]struct{}) int {
	records := a.snapshotSaveRecords()
	if len(gameIDs) == 1 {
		for gameID := range gameIDs {
			count := 0
			for _, record := range records {
				if record.Summary.Game.ID == gameID {
					count++
				}
			}
			return count
		}
	}
	return len(records)
}

func zipRecords(records []saveRecord) ([]byte, error) {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	usedNames := map[string]int{}
	for _, record := range records {
		payload, err := os.ReadFile(record.payloadPath)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		name := zipEntryName(record, usedNames)
		writer, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := writer.Write(payload); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func zipEntryName(record saveRecord, usedNames map[string]int) string {
	baseName := safeFilename(record.Summary.Filename)
	if count := usedNames[baseName]; count == 0 {
		usedNames[baseName] = 1
		return baseName
	}
	usedNames[baseName]++
	return record.Summary.ID + "-" + baseName
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		clean := strings.TrimSpace(part)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func parseCSVInts(raw string) []int {
	parts := splitCSV(raw)
	out := make([]int, 0, len(parts))
	seen := map[int]struct{}{}
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
