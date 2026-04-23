package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

func (a *app) handleSaveCheats(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	saveID := strings.TrimSpace(r.URL.Query().Get("saveId"))
	if saveID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "saveId is required", StatusCode: http.StatusBadRequest})
		return
	}
	record, ok := a.findSaveRecordByID(saveID)
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Save not found", StatusCode: http.StatusNotFound})
		return
	}
	service := a.cheatService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Cheat service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	state, err := service.get(record)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: err.Error(), StatusCode: http.StatusUnprocessableEntity})
		return
	}
	summary := canonicalSummaryForRecord(record)
	writeJSON(w, http.StatusOK, saveCheatsGetResponse{
		Success:      true,
		SaveID:       record.Summary.ID,
		DisplayTitle: firstNonEmpty(summary.DisplayTitle, summary.Game.DisplayTitle, summary.Game.Name, "Unknown Game"),
		Cheats:       state,
	})
}

func (a *app) handleSaveCheatsApply(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	var req saveCheatApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "invalid JSON body", StatusCode: http.StatusBadRequest})
		return
	}
	req.SaveID = strings.TrimSpace(req.SaveID)
	req.EditorID = strings.TrimSpace(req.EditorID)
	if req.SaveID == "" || req.EditorID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "saveId and editorId are required", StatusCode: http.StatusBadRequest})
		return
	}
	record, ok := a.findSaveRecordByID(req.SaveID)
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Save not found", StatusCode: http.StatusNotFound})
		return
	}
	service := a.cheatService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Cheat service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	payload, changed, err := service.apply(record, req)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: err.Error(), StatusCode: http.StatusUnprocessableEntity})
		return
	}
	metadata := mergeCheatMetadata(record, req, changed)
	newRecord, err := a.createSave(saveCreateInput{
		Filename:            record.Summary.Filename,
		Payload:             payload,
		Game:                record.Summary.Game,
		Format:              record.Summary.Format,
		Metadata:            metadata,
		ROMSHA1:             record.ROMSHA1,
		ROMMD5:              record.ROMMD5,
		SlotName:            record.SlotName,
		SystemSlug:          record.SystemSlug,
		GameSlug:            record.GameSlug,
		SystemPath:          record.SystemPath,
		GamePath:            record.GamePath,
		TrustedHelperSystem: metadataHasTrustedSystemEvidence(record.Summary.Metadata),
		DisplayTitle:        record.Summary.DisplayTitle,
		RegionCode:          record.Summary.RegionCode,
		RegionFlag:          record.Summary.RegionFlag,
		LanguageCodes:       record.Summary.LanguageCodes,
		CoverArtURL:         record.Summary.CoverArtURL,
		MemoryCard:          record.Summary.MemoryCard,
		Dreamcast:           record.Summary.Dreamcast,
		Saturn:              record.Summary.Saturn,
		Inspection:          record.Summary.Inspection,
		CreatedAt:           time.Now().UTC(),
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnsupportedSaveFormat) {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, apiError{Error: http.StatusText(status), Message: err.Error(), StatusCode: status})
		return
	}
	a.saveCreatedEvent(newRecord)
	a.resolveConflictForSave(newRecord)
	a.appendSyncLog(syncLogInput{
		DeviceName: "Web UI",
		Action:     "cheat_apply",
		Game:       syncLogGameLabelFromRecord(newRecord),
		SystemSlug: saveRecordSystemSlug(newRecord),
		SaveID:     newRecord.Summary.ID,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"sourceSaveId": record.Summary.ID,
		"save":         newRecord.Summary,
	})
}

func mergeCheatMetadata(source saveRecord, req saveCheatApplyRequest, changed map[string]any) any {
	cheatAudit := map[string]any{
		"action":        "cheat-apply",
		"sourceSaveId":  source.Summary.ID,
		"sourceVersion": source.Summary.Version,
		"sourceSHA256":  source.Summary.SHA256,
		"editorId":      req.EditorID,
		"slotId":        strings.TrimSpace(req.SlotID),
		"presetIds":     req.PresetIDs,
		"changed":       changed,
		"appliedAt":     time.Now().UTC().Format(time.RFC3339Nano),
	}
	if source.Summary.Metadata == nil {
		return map[string]any{"cheats": cheatAudit}
	}
	if existing, ok := source.Summary.Metadata.(map[string]any); ok {
		merged := make(map[string]any, len(existing)+1)
		for key, value := range existing {
			merged[key] = value
		}
		merged["cheats"] = cheatAudit
		return merged
	}
	return map[string]any{
		"cheats":         cheatAudit,
		"sourceMetadata": source.Summary.Metadata,
	}
}
