package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func unsupportedSaveRejectReason(err error) string {
	if err == nil || !errors.Is(err, errUnsupportedSaveFormat) {
		return ""
	}
	full := strings.TrimSpace(err.Error())
	if full == "" {
		return ""
	}
	base := errUnsupportedSaveFormat.Error()
	if full == base {
		return ""
	}
	prefix := base + ":"
	if strings.HasPrefix(full, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(full, prefix))
	}
	return full
}

func (a *app) handleSaveLatest(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	helperCtx, ok := a.authorizeHelperSyncRequest(w, r, nil)
	if !ok {
		return
	}

	romSHA1 := strings.TrimSpace(r.URL.Query().Get("romSha1"))
	slotName := normalizedSlot(r.URL.Query().Get("slotName"))
	saturnEntry := strings.TrimSpace(r.URL.Query().Get("saturnEntry"))
	runtimeProfile := requestedRuntimeProfile(r.URL.Query(), "")

	if helperCtx.IsHelper {
		if runtimeProfile != "" && (strings.HasPrefix(runtimeProfile, "psx/") || strings.HasPrefix(runtimeProfile, "ps2/")) {
			cardSlot, ok := deriveExplicitMemoryCardName(slotName, slotName)
			if !ok {
				writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "slotName must be an explicit Memory Card 1/2 for PlayStation helper latest checks", StatusCode: http.StatusBadRequest})
				return
			}
			if store := a.playStationSyncStore(); store != nil {
				if saveID, _, exists := store.latestProjectionSaveRecord(runtimeProfile, cardSlot); exists {
					latest, found := a.findSaveRecordByID(saveID)
					if found {
						writeJSON(w, http.StatusOK, map[string]any{
							"success": true,
							"exists":  true,
							"sha256":  latest.Summary.SHA256,
							"version": latest.Summary.Version,
							"id":      latest.Summary.ID,
						})
						return
					}
				}
			}
		} else if runtimeProfile == "" {
			if _, _, projectionCheck := helperProjectionIdentity(helperCtx.Device.DeviceType, slotName); projectionCheck {
				writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "runtimeProfile is required for PlayStation helper latest checks", StatusCode: http.StatusBadRequest})
				return
			}
		}
	}

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
		shaValue := latest.Summary.SHA256
		recordSystem := saveRecordSystemSlug(latest)
		runtimeProfile = requestedRuntimeProfile(r.URL.Query(), recordSystem)
		if requiresRuntimeProfileForHelper(recordSystem, helperCtx.IsHelper) && runtimeProfile == "" {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "runtimeProfile is required for projection-capable helper latest checks", StatusCode: http.StatusBadRequest})
			return
		}
		if runtimeProfile != "" {
			payload, err := os.ReadFile(latest.payloadPath)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
				return
			}
			_, _, projected, err := projectPayloadForRuntime(a, latest, payload, runtimeProfile, saturnEntry)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: err.Error(), StatusCode: http.StatusBadRequest})
				return
			}
			sum := sha256.Sum256(projected)
			shaValue = hex.EncodeToString(sum[:])
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"exists":  true,
			"sha256":  shaValue,
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
		deviceName := syncLogDeviceNameFromHelperContext(helperCtx, identity)

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
		declaredSystem := safeMultipartSystemSlug(formValue("system"), gameInfo.System)
		runtimeProfile := requestedRuntimeProfileFromForm(formValue, declaredSystem)
		input := saveCreateInput{
			Filename:            filename,
			Payload:             payload,
			Game:                gameInfo,
			Format:              inferSaveFormat(filename),
			Metadata:            nil,
			ROMSHA1:             strings.TrimSpace(formValue("rom_sha1")),
			ROMMD5:              strings.TrimSpace(formValue("rom_md5")),
			SlotName:            strings.TrimSpace(formValue("slotName")),
			SystemSlug:          declaredSystem,
			GameSlug:            canonicalSegment(gameInfo.Name, "unknown-game"),
			TrustedHelperSystem: helperCtx.IsHelper && strings.TrimSpace(formValue("system")) != "",
		}
		if helperCtx.IsHelper && isProjectionCapableSystem(declaredSystem) && declaredSystem != "psx" && declaredSystem != "ps2" {
			if runtimeProfile == "" {
				writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "runtimeProfile is required for projection-capable helper uploads", StatusCode: http.StatusBadRequest})
				return
			}
			input, err = normalizeProjectionUpload(input, runtimeProfile)
			if err != nil {
				writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: err.Error(), StatusCode: http.StatusUnprocessableEntity})
				return
			}
		} else if runtimeProfile != "" && !isProjectionCapableSystem(declaredSystem) {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "runtimeProfile is only valid for projection-capable saves", StatusCode: http.StatusBadRequest})
			return
		}
		preview := a.normalizeSaveInputDetailed(input)
		if preview.Rejected || !isSupportedSystemSlug(preview.Input.SystemSlug) {
			rejectReason := strings.TrimSpace(preview.RejectReason)
			logMessage := errUnsupportedSaveFormat.Error()
			if rejectReason != "" {
				logMessage = logMessage + ": " + rejectReason
			}
			a.appendSyncLog(syncLogInput{
				DeviceName:   deviceName,
				Action:       "upload",
				Game:         syncLogGameLabelFromFilename(filename),
				ErrorMessage: logMessage,
				SystemSlug:   preview.Input.SystemSlug,
			})
			writeJSON(w, http.StatusUnprocessableEntity, apiError{
				Error:      "Unprocessable Entity",
				Message:    errUnsupportedSaveFormat.Error(),
				Reason:     rejectReason,
				StatusCode: http.StatusUnprocessableEntity,
			})
			return
		}
		if helperCtx.IsHelper && !systemAllowedForDevice(helperCtx.Device, preview.Input.SystemSlug) {
			a.appendSyncLog(syncLogInput{
				DeviceName:   deviceName,
				Action:       "upload",
				Game:         syncLogGameLabelFromFilename(filename),
				ErrorMessage: "this device is not allowed to sync saves for this console",
				SystemSlug:   preview.Input.SystemSlug,
			})
			writeJSON(w, http.StatusForbidden, apiError{
				Error:      "Forbidden",
				Message:    "this device is not allowed to sync saves for this console",
				StatusCode: http.StatusForbidden,
			})
			return
		}

		artifactKind := classifyPlayStationArtifact(preview.Input.Game.System, input.Format, input.Filename, input.Payload)
		if artifactKind == saveArtifactPS1MemoryCard || artifactKind == saveArtifactPS2MemoryCard {
			deviceType := firstNonEmpty(helperCtx.Device.DeviceType, identity.DeviceType)
			if helperCtx.IsHelper && runtimeProfile == "" {
				writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "runtimeProfile is required for PlayStation helper uploads", StatusCode: http.StatusBadRequest})
				return
			}
			record, conflict, err := a.createPlayStationProjectionSave(input, preview, deviceType, runtimeProfile, identity.Fingerprint)
			if err != nil {
				a.appendSyncLog(syncLogInput{
					DeviceName:   deviceName,
					Action:       "upload",
					Game:         syncLogGameLabelFromFilename(filename),
					ErrorMessage: err.Error(),
					SystemSlug:   preview.Input.SystemSlug,
				})
				writeJSON(w, http.StatusUnprocessableEntity, apiError{
					Error:      "Unprocessable Entity",
					Message:    err.Error(),
					StatusCode: http.StatusUnprocessableEntity,
				})
				return
			}
			if conflict != nil {
				a.reportConflict(conflict.ConflictKey, record.Summary.CardSlot, conflict.LocalSHA256, conflict.CloudSHA256, helperCtx.Device.DisplayName, record.Summary.Filename, record.Summary.FileSize)
			}
			if !helperCtx.IsHelper && identity.isComplete() {
				a.upsertDevice(identity.DeviceType, identity.Fingerprint)
			}
			a.appendSyncLog(syncLogInput{
				DeviceName: deviceName,
				Action:     "upload",
				Game:       syncLogGameLabelFromRecord(record),
				SystemSlug: saveRecordSystemSlug(record),
				SaveID:     record.Summary.ID,
			})
			writeJSON(w, http.StatusOK, map[string]any{
				"success": true,
				"save": map[string]any{
					"id":     record.Summary.ID,
					"sha256": record.Summary.SHA256,
				},
			})
			return
		}

		record, err := a.createSave(input)
		if err != nil {
			a.appendSyncLog(syncLogInput{
				DeviceName:   deviceName,
				Action:       "upload",
				Game:         syncLogGameLabelFromFilename(filename),
				ErrorMessage: err.Error(),
				SystemSlug:   preview.Input.SystemSlug,
			})
			if errors.Is(err, errUnsupportedSaveFormat) {
				writeJSON(w, http.StatusUnprocessableEntity, apiError{
					Error:      "Unprocessable Entity",
					Message:    errUnsupportedSaveFormat.Error(),
					Reason:     unsupportedSaveRejectReason(err),
					StatusCode: http.StatusUnprocessableEntity,
				})
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
		a.appendSyncLog(syncLogInput{
			DeviceName: deviceName,
			Action:     "upload",
			Game:       syncLogGameLabelFromRecord(record),
			SystemSlug: saveRecordSystemSlug(record),
			SaveID:     record.Summary.ID,
		})

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
			a.appendSyncLog(syncLogInput{
				DeviceName:   "Web UI",
				Action:       "upload",
				Game:         syncLogGameLabelFromFilename(item.Filename),
				ErrorMessage: "invalid base64 data",
			})
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
			a.appendSyncLog(syncLogInput{
				DeviceName:   "Web UI",
				Action:       "upload",
				Game:         syncLogGameLabelFromFilename(item.Filename),
				ErrorMessage: err.Error(),
				SystemSlug:   safeMultipartSystemSlug("", gameInfo.System),
			})
			if errors.Is(err, errUnsupportedSaveFormat) {
				rejectReason := unsupportedSaveRejectReason(err)
				result := map[string]any{
					"filename": item.Filename,
					"success":  false,
					"error":    errUnsupportedSaveFormat.Error(),
				}
				if rejectReason != "" {
					result["reason"] = rejectReason
				}
				errorCount++
				results = append(results, result)
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
		a.appendSyncLog(syncLogInput{
			DeviceName: "Web UI",
			Action:     "upload",
			Game:       syncLogGameLabelFromRecord(record),
			SystemSlug: saveRecordSystemSlug(record),
			SaveID:     record.Summary.ID,
		})
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
	psLogicalKey := strings.TrimSpace(r.URL.Query().Get("psLogicalKey"))
	systemSlug := canonicalOptionalSegment(r.URL.Query().Get("systemSlug"))
	displayTitle := strings.TrimSpace(r.URL.Query().Get("displayTitle"))
	sourceRecord := saveRecord{}
	hasSourceRecord := false

	if psLogicalKey != "" {
		if saveID == "" {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "saveId is required for PlayStation logical save history", StatusCode: http.StatusBadRequest})
			return
		}
		history, err := a.playStationLogicalHistoryForSaveRecord(saveID, psLogicalKey)
		if err != nil {
			writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: err.Error(), StatusCode: http.StatusNotFound})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success":      true,
			"game":         history.Game,
			"displayTitle": history.DisplayTitle,
			"systemSlug":   history.SystemSlug,
			"summary":      history.Summary,
			"versions":     history.Versions,
		})
		return
	}

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
	return summaryWithDownloadProfiles(summary)
}

func (a *app) handleSaveRollback(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	var req struct {
		SaveID       string `json:"saveId"`
		PSLogicalKey string `json:"psLogicalKey"`
		RevisionID   string `json:"revisionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "invalid JSON body", StatusCode: http.StatusBadRequest})
		return
	}
	targetID := strings.TrimSpace(req.SaveID)
	psLogicalKey := strings.TrimSpace(req.PSLogicalKey)
	revisionID := strings.TrimSpace(req.RevisionID)
	if targetID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "saveId is required", StatusCode: http.StatusBadRequest})
		return
	}

	sourceRecord, ok := a.findSaveRecordByID(targetID)
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Save not found", StatusCode: http.StatusNotFound})
		return
	}
	if psLogicalKey != "" {
		if revisionID == "" {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "revisionId is required for PlayStation logical rollback", StatusCode: http.StatusBadRequest})
			return
		}
		if _, _, _, isPSProjection := playStationProjectionInfoFromRecord(sourceRecord); !isPSProjection {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "psLogicalKey is only valid for PlayStation saves", StatusCode: http.StatusBadRequest})
			return
		}
		record, err := a.rollbackPlayStationLogicalSave(sourceRecord, psLogicalKey, revisionID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
			return
		}
		a.appendSyncLog(syncLogInput{
			DeviceName: "Web UI",
			Action:     "rollback",
			Game:       syncLogGameLabelFromRecord(record),
			SystemSlug: saveRecordSystemSlug(record),
			SaveID:     record.Summary.ID,
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"success":      true,
			"sourceSaveId": sourceRecord.Summary.ID,
			"save":         record.Summary,
		})
		return
	}
	if runtimeProfile, cardSlot, _, isPSProjection := playStationProjectionInfoFromRecord(sourceRecord); isPSProjection {
		payload, err := os.ReadFile(sourceRecord.payloadPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
			return
		}
		record, err := a.rollbackPlayStationProjection(sourceRecord, runtimeProfile, cardSlot, payload)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
			return
		}
		a.saveCreatedEvent(record)
		a.resolveConflictForSave(record)
		a.appendSyncLog(syncLogInput{
			DeviceName: "Web UI",
			Action:     "rollback",
			Game:       syncLogGameLabelFromRecord(record),
			SystemSlug: saveRecordSystemSlug(record),
			SaveID:     record.Summary.ID,
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"success":      true,
			"sourceSaveId": sourceRecord.Summary.ID,
			"save":         record.Summary,
		})
		return
	}
	payload, err := os.ReadFile(sourceRecord.payloadPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}

	rollbackMeta := mergeRollbackMetadata(sourceRecord)
	newRecord, err := a.createSave(saveCreateInput{
		Filename:              sourceRecord.Summary.Filename,
		Payload:               payload,
		Game:                  sourceRecord.Summary.Game,
		Format:                sourceRecord.Summary.Format,
		Metadata:              rollbackMeta,
		ROMSHA1:               sourceRecord.ROMSHA1,
		ROMMD5:                sourceRecord.ROMMD5,
		SlotName:              sourceRecord.SlotName,
		SystemSlug:            sourceRecord.SystemSlug,
		GameSlug:              sourceRecord.GameSlug,
		SystemPath:            sourceRecord.SystemPath,
		GamePath:              sourceRecord.GamePath,
		TrustedHelperSystem:   metadataHasTrustedSystemEvidence(sourceRecord.Summary.Metadata),
		DisplayTitle:          sourceRecord.Summary.DisplayTitle,
		RegionCode:            sourceRecord.Summary.RegionCode,
		RegionFlag:            sourceRecord.Summary.RegionFlag,
		LanguageCodes:         sourceRecord.Summary.LanguageCodes,
		CoverArtURL:           sourceRecord.Summary.CoverArtURL,
		MemoryCard:            sourceRecord.Summary.MemoryCard,
		Dreamcast:             sourceRecord.Summary.Dreamcast,
		Saturn:                sourceRecord.Summary.Saturn,
		Inspection:            sourceRecord.Summary.Inspection,
		MediaType:             sourceRecord.Summary.MediaType,
		ProjectionCapable:     sourceRecord.Summary.ProjectionCapable,
		SourceArtifactProfile: sourceRecord.Summary.SourceArtifactProfile,
		RuntimeProfile:        sourceRecord.Summary.RuntimeProfile,
		CardSlot:              sourceRecord.Summary.CardSlot,
		ProjectionID:          sourceRecord.Summary.ProjectionID,
		SourceImportID:        sourceRecord.Summary.SourceImportID,
		Portable:              sourceRecord.Summary.Portable,
		CreatedAt:             time.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, errUnsupportedSaveFormat) {
			writeJSON(w, http.StatusUnprocessableEntity, apiError{
				Error:      "Unprocessable Entity",
				Message:    errUnsupportedSaveFormat.Error(),
				Reason:     unsupportedSaveRejectReason(err),
				StatusCode: http.StatusUnprocessableEntity,
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}

	a.saveCreatedEvent(newRecord)
	a.resolveConflictForSave(newRecord)
	a.appendSyncLog(syncLogInput{
		DeviceName: "Web UI",
		Action:     "rollback",
		Game:       syncLogGameLabelFromRecord(newRecord),
		SystemSlug: saveRecordSystemSlug(newRecord),
		SaveID:     newRecord.Summary.ID,
	})

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
	psLogicalKey := strings.TrimSpace(r.URL.Query().Get("psLogicalKey"))
	if saveID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "id is required", StatusCode: http.StatusBadRequest})
		return
	}
	record, ok := a.findSaveRecordByID(saveID)
	if ok {
		if psLogicalKey != "" {
			remainingVersions, err := a.deletePlayStationLogicalSave(record, psLogicalKey)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
				return
			}
			a.appendSyncLog(syncLogInput{
				DeviceName: "Web UI",
				Action:     "delete",
				Game:       syncLogGameLabelFromRecord(record),
				SystemSlug: saveRecordSystemSlug(record),
				SaveID:     record.Summary.ID,
			})
			writeJSON(w, http.StatusOK, map[string]any{"success": true, "remainingVersions": remainingVersions})
			return
		}
		if runtimeProfile, cardSlot, _, isPSProjection := playStationProjectionInfoFromRecord(record); isPSProjection {
			remainingVersions, err := a.deletePlayStationProjection(runtimeProfile, cardSlot)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
				return
			}
			a.appendSyncLog(syncLogInput{
				DeviceName: "Web UI",
				Action:     "delete",
				Game:       syncLogGameLabelFromRecord(record),
				SystemSlug: saveRecordSystemSlug(record),
				SaveID:     record.Summary.ID,
			})
			writeJSON(w, http.StatusOK, map[string]any{"success": true, "remainingVersions": remainingVersions})
			return
		}
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
	if ok {
		a.appendSyncLog(syncLogInput{
			DeviceName: "Web UI",
			Action:     "delete",
			Game:       syncLogGameLabelFromRecord(record),
			SystemSlug: saveRecordSystemSlug(record),
			SaveID:     record.Summary.ID,
		})
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

	deletedRecords := a.saveRecordsByIDs(ids)
	remainingVersions, _, err := a.deleteSaveRecordsByIDs(ids)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	for _, record := range deletedRecords {
		a.appendSyncLog(syncLogInput{
			DeviceName: "Web UI",
			Action:     "delete",
			Game:       syncLogGameLabelFromRecord(record),
			SystemSlug: saveRecordSystemSlug(record),
			SaveID:     record.Summary.ID,
		})
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
	psLogicalKey := strings.TrimSpace(r.URL.Query().Get("psLogicalKey"))
	revisionID := strings.TrimSpace(r.URL.Query().Get("revisionId"))
	saturnEntry := strings.TrimSpace(r.URL.Query().Get("saturnEntry"))
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
	if psLogicalKey != "" {
		filename, contentType, payload, err := a.downloadPlayStationLogicalSave(record.Summary.ID, psLogicalKey, revisionID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: err.Error(), StatusCode: http.StatusNotFound})
			return
		}
		a.appendSyncLog(syncLogInput{
			DeviceName: firstNonEmpty(helperCtx.Device.DisplayName, "Web UI"),
			Action:     "download",
			Game:       syncLogGameLabelFromRecord(record),
			SystemSlug: saveRecordSystemSlug(record),
			SaveID:     record.Summary.ID,
		})
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		_, _ = w.Write(payload)
		return
	}
	runtimeProfile := requestedRuntimeProfile(r.URL.Query(), saveRecordSystemSlug(record))
	if helperCtx.IsHelper {
		if requiresRuntimeProfileForHelper(saveRecordSystemSlug(record), true) && runtimeProfile == "" {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "runtimeProfile is required for projection-capable helper downloads", StatusCode: http.StatusBadRequest})
			return
		}
		if runtimeProfile, cardSlot, _, isPSProjection := playStationProjectionInfoFromRecord(record); isPSProjection {
			if store := a.playStationSyncStore(); store != nil {
				store.markProjectionDownloaded(runtimeProfile, cardSlot, helperCtx.Device.Fingerprint)
			}
		}
	}

	payload, err := os.ReadFile(record.payloadPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	filename := record.Summary.Filename
	contentType := "application/octet-stream"
	if runtimeProfile != "" {
		filename, contentType, payload, err = projectPayloadForRuntime(a, record, payload, runtimeProfile, saturnEntry)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: err.Error(), StatusCode: http.StatusBadRequest})
			return
		}
	}
	a.appendSyncLog(syncLogInput{
		DeviceName: firstNonEmpty(helperCtx.Device.DisplayName, "Web UI"),
		Action:     "download",
		Game:       syncLogGameLabelFromRecord(record),
		SystemSlug: saveRecordSystemSlug(record),
		SaveID:     record.Summary.ID,
	})
	w.Header().Set("Content-Type", "application/octet-stream")
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
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
	a.appendSyncLog(syncLogInput{
		DeviceName: firstNonEmpty(helperCtx.Device.DisplayName, "Web UI"),
		Action:     "download_many",
		Game:       fmt.Sprintf("%d saves", len(records)),
	})

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

func (a *app) playStationSyncStore() *playStationStore {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.playStationStore
}

func (a *app) createPlayStationProjectionSave(input saveCreateInput, preview normalizedSaveInputResult, deviceType, requestedProfile, fingerprint string) (saveRecord, *psImportConflict, error) {
	store := a.playStationSyncStore()
	if store == nil {
		return saveRecord{}, nil, fmt.Errorf("playstation store is not initialized")
	}
	createdAt := input.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	artifactKind := classifyPlayStationArtifact(preview.Input.Game.System, input.Format, input.Filename, input.Payload)
	runtimeProfile, systemSlug, err := resolvePlayStationRuntimeProfile(deviceType, requestedProfile, artifactKind)
	if err != nil {
		return saveRecord{}, nil, err
	}
	cardSlot, ok := deriveExplicitMemoryCardName(input.SlotName, input.Filename)
	if !ok {
		return saveRecord{}, nil, fmt.Errorf("PlayStation sync requires an explicit Memory Card 1/2 slot")
	}
	result, err := store.importMemoryCard(psImportRequest{
		Payload:        input.Payload,
		Filename:       input.Filename,
		ArtifactKind:   artifactKind,
		RuntimeProfile: runtimeProfile,
		SystemSlug:     systemSlug,
		CardSlot:       cardSlot,
		Fingerprint:    strings.TrimSpace(fingerprint),
		CreatedAt:      createdAt,
		HelperDevice:   strings.TrimSpace(deviceType),
	})
	if err != nil {
		return saveRecord{}, nil, err
	}
	recordsByLine, err := a.materializePlayStationProjections(saveCreateInput{
		Metadata:      input.Metadata,
		RegionCode:    preview.Input.RegionCode,
		RegionFlag:    preview.Input.RegionFlag,
		LanguageCodes: preview.Input.LanguageCodes,
		CoverArtURL:   preview.Input.CoverArtURL,
		CreatedAt:     createdAt,
	}, result.Built)
	if err != nil {
		return saveRecord{}, nil, err
	}
	primary, ok := recordsByLine[result.PrimaryProjectionLineKey]
	if !ok {
		return saveRecord{}, nil, fmt.Errorf("primary PlayStation projection was not created")
	}
	return primary, result.Conflict, nil
}

func (a *app) replaceSaveRecord(updated saveRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.saveRecords {
		if a.saveRecords[i].Summary.ID == updated.Summary.ID {
			a.saveRecords[i] = updated
			break
		}
	}
	for i := range a.saves {
		if a.saves[i].ID == updated.Summary.ID {
			a.saves[i] = updated.Summary
			break
		}
	}
}

func (a *app) rollbackPlayStationProjection(sourceRecord saveRecord, runtimeProfile, cardSlot string, payload []byte) (saveRecord, error) {
	store := a.playStationSyncStore()
	if store == nil {
		return saveRecord{}, fmt.Errorf("playstation store is not initialized")
	}
	store.markProjectionDownloaded(runtimeProfile, cardSlot, "rollback:"+sourceRecord.Summary.ID)
	systemSlug := "psx"
	if strings.HasPrefix(runtimeProfile, "ps2/") {
		systemSlug = "ps2"
	}
	returnRecord, _, err := a.createPlayStationProjectionSave(saveCreateInput{
		Filename: sourceRecord.Summary.Filename,
		Payload:  payload,
		Metadata: mergeRollbackMetadata(sourceRecord),
		SlotName: cardSlot,
		Game:     sourceRecord.Summary.Game,
		Format:   sourceRecord.Summary.Format,
	}, normalizedSaveInputResult{
		Input: saveCreateInput{SlotName: cardSlot},
		Detection: saveSystemDetectionResult{
			Slug:   systemSlug,
			System: supportedSystemFromSlug(systemSlug),
		},
	}, runtimeDeviceTypeFromProfile(runtimeProfile), runtimeProfile, "rollback:"+sourceRecord.Summary.ID)
	return returnRecord, err
}

func (a *app) deletePlayStationProjection(runtimeProfile, cardSlot string) (int, error) {
	store := a.playStationSyncStore()
	if store == nil {
		return 0, fmt.Errorf("playstation store is not initialized")
	}
	projection, ok := store.projectionForRuntime(runtimeProfile, cardSlot)
	if !ok {
		return 0, fmt.Errorf("PlayStation projection not found")
	}
	store.mu.Lock()
	line, ok := store.state.ProjectionLines[projection.ProjectionLineKey]
	if !ok {
		store.mu.Unlock()
		return 0, fmt.Errorf("PlayStation projection line not found")
	}
	active := store.activeLogicalSavesForLineLocked(line.SystemSlug, line.SyncLineKey, line.Key)
	now := time.Now().UTC()
	for _, logical := range active {
		scopeKey := logical.ProjectionLineKey
		if logical.Portable {
			scopeKey = logical.SyncLineKey
		}
		store.state.Tombstones[psTombstoneKey(scopeKey, logical.Key)] = psTombstone{
			ID:         deterministicConflictID(psTombstoneKey(scopeKey, logical.Key)),
			LogicalKey: logical.Key,
			ScopeKey:   scopeKey,
			Reason:     "projection deleted from API",
			CreatedAt:  now,
		}
	}
	built, err := store.rebuildProjectionLinesLocked(line.SystemSlug, line.CardSlot, runtimeProfile, "delete:"+projection.ID, line.CardSlot)
	if err != nil {
		store.mu.Unlock()
		return 0, err
	}
	if err := store.persistLocked(); err != nil {
		store.mu.Unlock()
		return 0, err
	}
	store.mu.Unlock()
	template := a.playStationTemplateInputFromSummary(saveSummary{Metadata: map[string]any{"deleted": true}})
	template.Metadata = map[string]any{"deleted": true}
	template.CreatedAt = time.Now().UTC()
	if _, err := a.materializePlayStationProjections(template, built); err != nil {
		return 0, err
	}
	return len(a.snapshotSaveRecords()), nil
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
