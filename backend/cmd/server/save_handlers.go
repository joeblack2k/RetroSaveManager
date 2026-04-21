package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

func (a *app) handleSaveLatest(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	romSHA1 := strings.TrimSpace(r.URL.Query().Get("romSha1"))
	slotName := normalizedSlot(r.URL.Query().Get("slotName"))

	latest, ok := a.latestSaveRecord(romSHA1, slotName)
	if ok {
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
			ROMSHA1:    strings.TrimSpace(r.FormValue("rom_sha1")),
			ROMMD5:     strings.TrimSpace(r.FormValue("rom_md5")),
			SlotName:   strings.TrimSpace(r.FormValue("slotName")),
			SystemSlug: safeMultipartSystemSlug(r.FormValue("system"), gameInfo.System),
			GameSlug:   canonicalSegment(gameInfo.Name, "unknown-game"),
		}

		record, err := a.createSave(input)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
			return
		}

		deviceType := strings.TrimSpace(r.FormValue("device_type"))
		fingerprint := strings.TrimSpace(r.FormValue("fingerprint"))
		if deviceType != "" && fingerprint != "" {
			a.upsertDevice(deviceType, fingerprint)
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

	a.mu.Lock()
	defer a.mu.Unlock()

	filtered := make([]saveSummary, 0, len(a.saveRecords))
	for _, record := range a.saveRecords {
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
		filtered = append(filtered, record.Summary)
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
		"saves":   filtered[offset:end],
		"total":   len(filtered),
		"limit":   limit,
		"offset":  offset,
	})
}

func (a *app) handleSaveSystems(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	writeJSON(w, http.StatusOK, []system{
		{ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy"},
		{ID: 26, Name: "Nintendo Super Nintendo Entertainment System", Slug: "snes"},
		{ID: 33, Name: "Sega Genesis/Mega Drive", Slug: "genesis"},
	})
}

func (a *app) handleSaveByGame(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	gameID := parseIntOrDefault(r.URL.Query().Get("gameId"), 281)
	a.mu.Lock()
	defer a.mu.Unlock()

	var versions []saveSummary
	for _, s := range a.saves {
		if s.Game.ID == gameID {
			versions = append(versions, s)
		}
	}

	if len(versions) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "game": nil, "versions": []any{}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"game":     versions[0].Game,
		"versions": versions,
	})
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
		return canonicalSegment(raw, "unknown-system")
	}
	if sys != nil {
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
