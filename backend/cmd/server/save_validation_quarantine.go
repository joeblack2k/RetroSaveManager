package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (a *app) handleValidationQuarantineDelete(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	record, dir, err := a.quarantineRecordByID(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: err.Error(), StatusCode: http.StatusNotFound})
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	a.appendSyncLog(syncLogInput{DeviceName: "API", Action: "quarantine_delete", Game: firstNonEmpty(record.DisplayTitle, record.Filename), SystemSlug: record.SystemSlug})
	status, err := a.buildValidationStatus(nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "deleted": record.ID, "validation": status})
}

func (a *app) handleValidationQuarantineRetry(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	record, dir, err := a.quarantineRecordByID(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: err.Error(), StatusCode: http.StatusNotFound})
		return
	}
	payload, err := os.ReadFile(filepath.Join(dir, record.PayloadFile))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	result, statusCode, err := a.retryQuarantinedUpload(record, payload)
	if err != nil {
		preview, _ := result["preview"].(saveUploadPreviewItem)
		if preview.Reason != "" {
			record.Reason = preview.Reason
			_ = a.writeQuarantineRecord(dir, record)
		}
		writeJSON(w, statusCode, map[string]any{
			"success": false,
			"error":   "retry_failed",
			"message": err.Error(),
			"preview": preview,
		})
		return
	}
	if result["imported"] == true || result["duplicate"] == true {
		_ = os.RemoveAll(dir)
	}
	status, err := a.buildValidationStatus(nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	result["success"] = true
	result["validation"] = status
	writeJSON(w, http.StatusOK, result)
}

func (a *app) quarantineRecordByID(rawID string) (quarantineRecord, string, error) {
	id := strings.TrimSpace(rawID)
	if id == "" || strings.ContainsAny(id, `/\`) || id == "." || id == ".." {
		return quarantineRecord{}, "", fmt.Errorf("invalid quarantine id")
	}
	store := a.currentSaveStore()
	if store == nil {
		return quarantineRecord{}, "", fmt.Errorf("save store is not ready")
	}
	dir, err := safeJoinUnderRoot(store.root, quarantineDirName, id)
	if err != nil {
		return quarantineRecord{}, "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return quarantineRecord{}, "", fmt.Errorf("quarantine item not found")
	}
	var record quarantineRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return quarantineRecord{}, "", fmt.Errorf("quarantine metadata is invalid")
	}
	if strings.TrimSpace(record.ID) == "" {
		record.ID = id
	}
	if strings.TrimSpace(record.PayloadFile) == "" {
		return quarantineRecord{}, "", fmt.Errorf("quarantine payload is missing")
	}
	return record, dir, nil
}

func (a *app) writeQuarantineRecord(dir string, record quarantineRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, "metadata.json"), data, 0o644)
}

func (a *app) retryQuarantinedUpload(record quarantineRecord, payload []byte) (map[string]any, int, error) {
	gameInfo := fallbackGameFromFilename(firstNonEmpty(record.DisplayTitle, record.Filename))
	if displayTitle := strings.TrimSpace(record.DisplayTitle); displayTitle != "" {
		gameInfo.ID = deterministicGameID(displayTitle)
		gameInfo.Name = displayTitle
		gameInfo.DisplayTitle = displayTitle
	}
	if sys := supportedSystemFromSlug(record.SystemSlug); sys != nil {
		gameInfo.System = sys
	}
	input := saveCreateInput{
		Filename:              record.Filename,
		Payload:               payload,
		Game:                  gameInfo,
		Format:                firstNonEmpty(record.Format, inferSaveFormat(record.Filename)),
		ROMSHA1:               strings.TrimSpace(record.ROMSHA1),
		ROMMD5:                strings.TrimSpace(record.ROMMD5),
		SlotName:              firstNonEmpty(record.SourcePath, record.Filename),
		SystemSlug:            canonicalSegment(record.SystemSlug, "unknown-system"),
		GameSlug:              canonicalSegment(gameInfo.Name, "unknown-game"),
		TrustedHelperSystem:   isSupportedSystemSlug(record.SystemSlug),
		RuntimeProfile:        strings.TrimSpace(record.RuntimeProfile),
		SourceArtifactProfile: strings.TrimSpace(record.RuntimeProfile),
	}
	if input.RuntimeProfile != "" && isProjectionCapableSystem(input.SystemSlug) {
		normalized, err := normalizeProjectionUpload(input, input.RuntimeProfile)
		if err != nil {
			item := rejectedPreviewItem(record.Filename, record.SourcePath, payload, input.SystemSlug, input.Format, input.RuntimeProfile, err.Error())
			return map[string]any{"preview": item}, http.StatusUnprocessableEntity, err
		}
		input = normalized
	}

	preview := a.normalizeSaveInputDetailed(input)
	item := previewItemFromNormalized(record.Filename, record.SourcePath, payload, input.RuntimeProfile, preview)
	if preview.Rejected || !isSupportedSystemSlug(preview.Input.SystemSlug) {
		item.Accepted = false
		item.Reason = firstNonEmpty(strings.TrimSpace(preview.RejectReason), errUnsupportedSaveFormat.Error())
		return map[string]any{"preview": item}, http.StatusUnprocessableEntity, fmt.Errorf("%s: %s", errUnsupportedSaveFormat.Error(), item.Reason)
	}

	artifactKind := classifyPlayStationArtifact(preview.Input.Game.System, preview.Input.Format, preview.Input.Filename, preview.Input.Payload)
	if artifactKind == saveArtifactPS1MemoryCard || artifactKind == saveArtifactPS2MemoryCard {
		if strings.TrimSpace(input.RuntimeProfile) == "" {
			return map[string]any{"preview": item}, http.StatusUnprocessableEntity, fmt.Errorf("runtimeProfile is required to retry PlayStation projection imports")
		}
		result, err := a.createPlayStationProjectionSaveDetailed(preview.Input, preview, runtimeDeviceTypeFromProfile(input.RuntimeProfile), input.RuntimeProfile, "quarantine:"+record.ID)
		if err != nil {
			return map[string]any{"preview": item}, http.StatusUnprocessableEntity, err
		}
		if result.Disposition == uploadDuplicateStaleHistorical {
			return quarantinedImportResult(record, result.Record, result.Disposition), http.StatusConflict, fmt.Errorf("newest cloud save already differs from this quarantined historical duplicate")
		}
		return quarantinedImportResult(record, result.Record, result.Disposition), http.StatusOK, nil
	}

	if preview.Input.SystemSlug == "n64" && strings.TrimSpace(preview.Input.MediaType) == "controller-pak" {
		result, err := a.createN64ControllerPakProjectionSaveDetailed(preview.Input, preview)
		if err != nil {
			return map[string]any{"preview": item}, http.StatusUnprocessableEntity, err
		}
		if result.Disposition == uploadDuplicateStaleHistorical {
			return quarantinedImportResult(record, result.Record, result.Disposition), http.StatusConflict, fmt.Errorf("newest cloud save already differs from this quarantined historical duplicate")
		}
		return quarantinedImportResult(record, result.Record, result.Disposition), http.StatusOK, nil
	}

	if duplicate := checkUploadDuplicate(a.snapshotSaveRecords(), preview.Input); duplicate.Found {
		if duplicate.Disposition == uploadDuplicateIgnoredLatest {
			a.appendSyncLog(syncLogInput{DeviceName: "API", Action: "quarantine_duplicate_ignored", Game: syncLogGameLabelFromRecord(duplicate.Latest), SystemSlug: saveRecordSystemSlug(duplicate.Latest), SaveID: duplicate.Latest.Summary.ID})
			return duplicateUploadResponse(duplicate.Latest, duplicate.Disposition), http.StatusOK, nil
		}
		return map[string]any{"preview": item, "latest": duplicate.Latest.Summary, "reason": "stale_historical_duplicate"}, http.StatusConflict, fmt.Errorf("newest cloud save already differs from this quarantined historical duplicate")
	}

	newRecord, err := a.createSave(preview.Input)
	if err != nil {
		return map[string]any{"preview": item}, http.StatusUnprocessableEntity, err
	}
	a.saveCreatedEvent(newRecord)
	a.resolveConflictForSave(newRecord)
	a.appendSyncLog(syncLogInput{DeviceName: "API", Action: "quarantine_retry_import", Game: syncLogGameLabelFromRecord(newRecord), SystemSlug: saveRecordSystemSlug(newRecord), SaveID: newRecord.Summary.ID})
	return map[string]any{
		"imported": true,
		"save":     newRecord.Summary,
		"message":  "Quarantined save imported and removed from quarantine.",
	}, http.StatusOK, nil
}

func quarantinedImportResult(record quarantineRecord, imported saveRecord, disposition uploadDuplicateDisposition) map[string]any {
	result := map[string]any{
		"save":    imported.Summary,
		"message": "Quarantined save imported and removed from quarantine.",
	}
	if disposition == uploadDuplicateIgnoredLatest {
		result["duplicate"] = true
		result["duplicateDisposition"] = string(disposition)
		result["message"] = "Quarantined payload already matches the current cloud save and was removed."
		return result
	}
	if disposition == uploadDuplicateStaleHistorical {
		result["reason"] = "stale_historical_duplicate"
		result["source"] = record.ID
		return result
	}
	result["imported"] = true
	return result
}
