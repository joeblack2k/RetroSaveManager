package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (a *app) handleConflictsList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	records := a.listConflicts()
	items := buildConflictItems(records, a.snapshotSaveRecords())
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "conflicts": items})
}

func (a *app) handleConflictsGet(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	record, ok := a.getConflictByID(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Conflict not found", StatusCode: http.StatusNotFound})
		return
	}
	items := buildConflictItems([]conflictRecord{record}, a.snapshotSaveRecords())
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "conflict": items[0]})
}

func (a *app) handleConflictsResolve(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	id := chi.URLParam(r, "id")
	resolution := conflictResolutionFromRequest(r)
	_, ok := a.resolveConflictByID(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Conflict not found", StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"conflictId": strings.TrimSpace(id),
		"resolution": resolution,
	})
}

func (a *app) handleConflictsCheck(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	romSHA1, slotName := conflictQueryFromRequest(r)
	if identity := extractHelperIdentity(r, nil); identity.hasAnyMarker() {
		if runtimeProfile, cardSlot, ok := helperProjectionIdentity(identity.DeviceType, slotName); ok {
			romSHA1 = projectionConflictKey(runtimeProfile, cardSlot)
			slotName = cardSlot
		}
	}
	record, ok := a.getConflict(romSHA1, slotName)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"exists":       false,
			"conflictId":   nil,
			"status":       nil,
			"cloudSha256":  nil,
			"cloudVersion": nil,
			"cloudSaveId":  nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"exists":       true,
		"conflictId":   record.ConflictID,
		"status":       record.Status,
		"cloudSha256":  record.CloudSHA256,
		"cloudVersion": record.CloudVersion,
		"cloudSaveId":  record.CloudSaveID,
	})
}

func (a *app) handleConflictsReport(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "Invalid `boundary` for `multipart/form-data` request", StatusCode: http.StatusBadRequest})
		return
	}
	if err := requireFile(r.MultipartForm, "file"); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "file is required", StatusCode: http.StatusBadRequest})
		return
	}

	requiredFields := []string{"romSha1", "slotName", "localSha256", "cloudSha256"}
	for _, field := range requiredFields {
		if strings.TrimSpace(r.FormValue(field)) == "" {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: field + " is required", StatusCode: http.StatusBadRequest})
			return
		}
	}

	deviceFilename := "unknown.save"
	deviceFileSize := 0
	if files := r.MultipartForm.File["file"]; len(files) > 0 {
		deviceFilename = safeFilename(files[0].Filename)
		deviceFileSize = int(files[0].Size)
	}

	romSHA1 := strings.TrimSpace(r.FormValue("romSha1"))
	slotName := normalizedSlot(r.FormValue("slotName"))
	if identity := extractHelperIdentity(r, func(key string) string { return r.FormValue(key) }); identity.hasAnyMarker() {
		if runtimeProfile, cardSlot, ok := helperProjectionIdentity(identity.DeviceType, slotName); ok {
			romSHA1 = projectionConflictKey(runtimeProfile, cardSlot)
			slotName = cardSlot
		}
	}
	record := a.reportConflict(
		romSHA1,
		slotName,
		r.FormValue("localSha256"),
		r.FormValue("cloudSha256"),
		conflictDeviceNameFromRequest(r),
		deviceFilename,
		deviceFileSize,
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"created":    true,
		"conflictId": record.ConflictID,
	})
}

func conflictQueryFromRequest(r *http.Request) (string, string) {
	romSHA1 := strings.TrimSpace(r.URL.Query().Get("romSha1"))
	slotName := normalizedSlot(r.URL.Query().Get("slotName"))
	if romSHA1 != "" {
		return romSHA1, slotName
	}
	if r.Method != http.MethodPost {
		return "", slotName
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			ROMSHA1  string `json:"romSha1"`
			SlotName string `json:"slotName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			return strings.TrimSpace(req.ROMSHA1), normalizedSlot(req.SlotName)
		}
		return "", slotName
	}

	if err := r.ParseForm(); err == nil {
		return strings.TrimSpace(r.FormValue("romSha1")), normalizedSlot(r.FormValue("slotName"))
	}
	return "", slotName
}

func deterministicConflictID(key string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(key))))
	return "conf-" + hex.EncodeToString(sum[:])[:12]
}

func (a *app) handleConflictsCount(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	writeJSON(w, http.StatusOK, map[string]any{"count": a.activeConflictCount()})
}

func conflictResolutionFromRequest(r *http.Request) string {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			Resolution string `json:"resolution"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && strings.TrimSpace(req.Resolution) != "" {
			return strings.TrimSpace(req.Resolution)
		}
	}
	if err := r.ParseForm(); err == nil && strings.TrimSpace(r.FormValue("resolution")) != "" {
		return strings.TrimSpace(r.FormValue("resolution"))
	}
	return "dismiss"
}

func conflictDeviceNameFromRequest(r *http.Request) string {
	candidates := []string{
		r.FormValue("deviceName"),
		r.FormValue("displayName"),
		r.FormValue("alias"),
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}

	deviceType := strings.TrimSpace(r.FormValue("deviceType"))
	if deviceType == "" {
		deviceType = strings.TrimSpace(r.FormValue("device_type"))
	}
	fingerprint := strings.TrimSpace(r.FormValue("fingerprint"))
	if deviceType != "" && fingerprint != "" {
		return deviceType + " " + fingerprint
	}
	return ""
}

func buildConflictItems(records []conflictRecord, saveRecords []saveRecord) []map[string]any {
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].ConflictID > records[j].ConflictID
		}
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})

	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, buildConflictItem(record, saveRecords))
	}
	return items
}

func buildConflictItem(record conflictRecord, saveRecords []saveRecord) map[string]any {
	latest, hasLatest := latestSaveRecordLocked(saveRecords, record.ROMSHA1, record.SlotName)
	gameName := "Unknown Game"
	var boxartThumb *string
	cloudLatest := any(nil)
	if hasLatest {
		gameName = latest.Summary.Game.Name
		boxartThumb = latest.Summary.Game.BoxartThumb
		cloudLatestMap := map[string]any{
			"filename":  latest.Summary.Filename,
			"fileSize":  latest.Summary.FileSize,
			"version":   latest.Summary.Version,
			"createdAt": latest.Summary.CreatedAt,
		}
		if summary, ok := metadataSummary(latest.Summary.Metadata); ok {
			cloudLatestMap["metadata"] = map[string]any{"summary": summary}
		}
		cloudLatest = cloudLatestMap
	}

	deviceName := any(nil)
	if strings.TrimSpace(record.DeviceName) != "" {
		deviceName = record.DeviceName
	}

	return map[string]any{
		"id": record.ConflictID,
		"game": map[string]any{
			"name":        gameName,
			"boxartThumb": boxartThumb,
		},
		"deviceName":     deviceName,
		"deviceFilename": fallbackConflictFilename(record.DeviceFilename),
		"deviceFileSize": maxInt(record.DeviceFileSize, 0),
		"createdAt":      record.CreatedAt,
		"cloudLatest":    cloudLatest,
	}
}

func metadataSummary(metadata any) (string, bool) {
	if metadata == nil {
		return "", false
	}
	if text, ok := metadata.(string); ok && strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text), true
	}
	if metaMap, ok := metadata.(map[string]any); ok {
		if summary, ok := metaMap["summary"].(string); ok && strings.TrimSpace(summary) != "" {
			return strings.TrimSpace(summary), true
		}
	}
	return "", false
}

func fallbackConflictFilename(filename string) string {
	filename = safeFilename(filename)
	if strings.TrimSpace(filename) == "" {
		return "unknown.save"
	}
	return filename
}

func maxInt(value, floor int) int {
	if value < floor {
		return floor
	}
	return value
}
