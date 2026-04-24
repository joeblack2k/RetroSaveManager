package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

const maxExpandedUploadBytes = 64 << 20

type expandedMultipartSaveUpload struct {
	Filename   string
	Payload    []byte
	Game       game
	SystemSlug string
	Metadata   any
	SourcePath string
}

func expandMultipartSaveUpload(filename string, payload []byte, formValue func(string) string) ([]expandedMultipartSaveUpload, bool, error) {
	if !strings.EqualFold(filepath.Ext(strings.TrimSpace(filename)), ".zip") {
		return nil, false, nil
	}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, true, fmt.Errorf("invalid zip upload: %w", err)
	}
	items := make([]expandedMultipartSaveUpload, 0, len(reader.File))
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		sourcePath := cleanZipEntryPath(entry.Name)
		entryName := safeFilename(filepath.Base(sourcePath))
		if entryName == "" || isIgnoredArchiveEntry(sourcePath, entryName) {
			continue
		}
		if entry.UncompressedSize64 > maxExpandedUploadBytes {
			return nil, true, fmt.Errorf("zip entry %s is too large", sourcePath)
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, true, fmt.Errorf("open zip entry %s: %w", sourcePath, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(rc, maxExpandedUploadBytes+1))
		_ = rc.Close()
		if readErr != nil {
			return nil, true, fmt.Errorf("read zip entry %s: %w", sourcePath, readErr)
		}
		if len(data) == 0 || len(data) > maxExpandedUploadBytes {
			continue
		}
		metadata, titleCode := wiiUploadMetadata(nil, sourcePath, formValue)
		systemSlug := safeMultipartSystemSlug(formValue("system"), nil)
		gameInfo := fallbackGameFromFilename(entryName)
		if titleCode != "" && looksLikeWiiDataBinPath(sourcePath) {
			systemSlug = "wii"
			gameInfo = wiiGameFromTitleCode(titleCode)
		}
		items = append(items, expandedMultipartSaveUpload{
			Filename:   entryName,
			Payload:    data,
			Game:       gameInfo,
			SystemSlug: systemSlug,
			Metadata:   metadata,
			SourcePath: sourcePath,
		})
	}
	if len(items) == 0 {
		return nil, true, fmt.Errorf("zip upload did not contain any supported save files")
	}
	return items, true, nil
}

func wiiUploadMetadata(existing any, sourcePath string, formValue func(string) string) (any, string) {
	titleCode := ""
	if formValue != nil {
		for _, key := range []string{"wiiTitleId", "wiiTitleID", "wiiTitleCode", "titleCode", "gameCode"} {
			if code := normalizeWiiTitleCode(formValue(key)); code != "" {
				titleCode = code
				break
			}
		}
	}
	if titleCode == "" {
		titleCode = wiiTitleCodeFromPath(sourcePath)
	}
	if titleCode == "" && !looksLikeWiiDataBinPath(sourcePath) {
		return existing, ""
	}
	wiiMeta := map[string]any{
		"sourcePath": strings.TrimSpace(strings.ReplaceAll(sourcePath, "\\", "/")),
	}
	if titleCode != "" {
		wiiMeta["titleCode"] = titleCode
		wiiMeta["gameCode"] = titleCode
	}
	return mergeRSMMetadata(existing, "wii", wiiMeta), titleCode
}

func cleanZipEntryPath(path string) string {
	clean := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	clean = strings.TrimLeft(clean, "/")
	parts := make([]string, 0, len(strings.Split(clean, "/")))
	for _, part := range strings.Split(clean, "/") {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "/")
}

func isIgnoredArchiveEntry(sourcePath, entryName string) bool {
	lowerPath := strings.ToLower(strings.TrimSpace(sourcePath))
	lowerName := strings.ToLower(strings.TrimSpace(entryName))
	if strings.HasPrefix(lowerPath, "__macosx/") || lowerName == ".ds_store" {
		return true
	}
	if noisy, _ := isLikelyNoiseFilename(entryName); noisy {
		return true
	}
	return false
}

func (a *app) handleExpandedMultipartSaveUploads(w http.ResponseWriter, helperCtx helperAuthContext, identity helperIdentity, deviceName string, uploads []expandedMultipartSaveUpload, formValue func(string) string) {
	results := make([]any, 0, len(uploads))
	successCount := 0
	errorCount := 0
	var firstRecord *saveRecord

	for _, upload := range uploads {
		input := saveCreateInput{
			Filename:            upload.Filename,
			Payload:             upload.Payload,
			Game:                upload.Game,
			Format:              inferSaveFormat(upload.Filename),
			Metadata:            upload.Metadata,
			ROMSHA1:             strings.TrimSpace(formValue("rom_sha1")),
			ROMMD5:              strings.TrimSpace(formValue("rom_md5")),
			SlotName:            firstNonEmpty(strings.TrimSpace(formValue("slotName")), upload.SourcePath),
			SystemSlug:          upload.SystemSlug,
			GameSlug:            canonicalSegment(upload.Game.Name, "unknown-game"),
			TrustedHelperSystem: helperCtx.IsHelper && strings.TrimSpace(formValue("system")) != "",
		}
		preview := a.normalizeSaveInputDetailed(input)
		if preview.Rejected || !isSupportedSystemSlug(preview.Input.SystemSlug) {
			rejectReason := strings.TrimSpace(preview.RejectReason)
			logMessage := errUnsupportedSaveFormat.Error()
			if rejectReason != "" {
				logMessage += ": " + rejectReason
			}
			a.appendSyncLog(syncLogInput{DeviceName: deviceName, Action: "upload", Game: syncLogGameLabelFromFilename(upload.Filename), ErrorMessage: logMessage, SystemSlug: preview.Input.SystemSlug})
			errorCount++
			results = append(results, map[string]any{"filename": upload.Filename, "sourcePath": upload.SourcePath, "success": false, "error": errUnsupportedSaveFormat.Error(), "reason": rejectReason})
			continue
		}
		if helperCtx.IsHelper && !systemAllowedForDevice(helperCtx.Device, preview.Input.SystemSlug) {
			a.appendSyncLog(syncLogInput{DeviceName: deviceName, Action: "upload", Game: syncLogGameLabelFromFilename(upload.Filename), ErrorMessage: "this device is not allowed to sync saves for this console", SystemSlug: preview.Input.SystemSlug})
			errorCount++
			results = append(results, map[string]any{"filename": upload.Filename, "sourcePath": upload.SourcePath, "success": false, "error": "this device is not allowed to sync saves for this console"})
			continue
		}
		if duplicate := checkUploadDuplicate(a.snapshotSaveRecords(), preview.Input); duplicate.Found {
			if duplicate.Disposition == uploadDuplicateIgnoredLatest {
				successCount++
				result := duplicateUploadResponse(duplicate.Latest, duplicate.Disposition)
				result["filename"] = upload.Filename
				result["sourcePath"] = upload.SourcePath
				results = append(results, result)
				a.appendSyncLog(syncLogInput{DeviceName: deviceName, Action: "upload_duplicate_ignored", Game: syncLogGameLabelFromRecord(duplicate.Latest), SystemSlug: saveRecordSystemSlug(duplicate.Latest), SaveID: duplicate.Latest.Summary.ID})
				continue
			}
			errorCount++
			result := staleHistoricalUploadResponse(duplicate.Latest, "stale_historical_duplicate")
			result["filename"] = upload.Filename
			result["sourcePath"] = upload.SourcePath
			results = append(results, result)
			a.appendSyncLog(syncLogInput{DeviceName: deviceName, Action: "upload_stale_rejected", Game: syncLogGameLabelFromRecord(duplicate.Latest), ErrorMessage: "newest cloud save already differs from uploaded historical duplicate", SystemSlug: saveRecordSystemSlug(duplicate.Latest), SaveID: duplicate.Latest.Summary.ID})
			continue
		}
		record, err := a.createSave(input)
		if err != nil {
			a.appendSyncLog(syncLogInput{DeviceName: deviceName, Action: "upload", Game: syncLogGameLabelFromFilename(upload.Filename), ErrorMessage: err.Error(), SystemSlug: preview.Input.SystemSlug})
			errorCount++
			result := map[string]any{"filename": upload.Filename, "sourcePath": upload.SourcePath, "success": false, "error": err.Error()}
			if errors.Is(err, errUnsupportedSaveFormat) {
				result["error"] = errUnsupportedSaveFormat.Error()
				if reason := unsupportedSaveRejectReason(err); reason != "" {
					result["reason"] = reason
				}
			}
			results = append(results, result)
			continue
		}
		if !helperCtx.IsHelper && identity.isComplete() {
			a.upsertDevice(identity.DeviceType, identity.Fingerprint)
		}
		successCount++
		recordCopy := record
		if firstRecord == nil {
			firstRecord = &recordCopy
		}
		results = append(results, map[string]any{"filename": upload.Filename, "sourcePath": upload.SourcePath, "success": true, "save": map[string]any{"id": record.Summary.ID, "sha256": record.Summary.SHA256, "version": record.Summary.Version}})
		a.saveCreatedEvent(record)
		a.resolveConflictForSave(record)
		a.appendSyncLog(syncLogInput{DeviceName: deviceName, Action: "upload", Game: syncLogGameLabelFromRecord(record), SystemSlug: saveRecordSystemSlug(record), SaveID: record.Summary.ID})
	}

	status := http.StatusOK
	if successCount == 0 && errorCount > 0 {
		status = http.StatusUnprocessableEntity
	}
	response := map[string]any{
		"success":      successCount > 0,
		"successCount": successCount,
		"errorCount":   errorCount,
		"results":      results,
	}
	if firstRecord != nil {
		response["save"] = map[string]any{"id": firstRecord.Summary.ID, "sha256": firstRecord.Summary.SHA256, "version": firstRecord.Summary.Version}
	}
	writeJSON(w, status, response)
}
