package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const quarantineDirName = "_rsm/quarantine"

type saveUploadPreviewItem struct {
	Filename         string            `json:"filename"`
	SourcePath       string            `json:"sourcePath,omitempty"`
	Accepted         bool              `json:"accepted"`
	DisplayTitle     string            `json:"displayTitle,omitempty"`
	SystemSlug       string            `json:"systemSlug,omitempty"`
	SystemName       string            `json:"systemName,omitempty"`
	RegionCode       string            `json:"regionCode,omitempty"`
	RegionFlag       string            `json:"regionFlag,omitempty"`
	Format           string            `json:"format,omitempty"`
	MediaType        string            `json:"mediaType,omitempty"`
	RuntimeProfile   string            `json:"runtimeProfile,omitempty"`
	ROMSHA1          string            `json:"romSha1,omitempty"`
	ROMMD5           string            `json:"romMd5,omitempty"`
	SizeBytes        int               `json:"sizeBytes"`
	SHA256           string            `json:"sha256"`
	ParserLevel      string            `json:"parserLevel,omitempty"`
	TrustLevel       string            `json:"trustLevel,omitempty"`
	Evidence         []string          `json:"evidence,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
	Reason           string            `json:"reason,omitempty"`
	Inspection       *saveInspection   `json:"inspection,omitempty"`
	DownloadProfiles []downloadProfile `json:"downloadProfiles,omitempty"`
}

type saveUploadPreviewResponse struct {
	Success       bool                    `json:"success"`
	AcceptedCount int                     `json:"acceptedCount"`
	RejectedCount int                     `json:"rejectedCount"`
	Items         []saveUploadPreviewItem `json:"items"`
}

type quarantineRecord struct {
	ID             string    `json:"id"`
	Filename       string    `json:"filename"`
	SourcePath     string    `json:"sourcePath,omitempty"`
	PayloadFile    string    `json:"payloadFile"`
	SizeBytes      int       `json:"sizeBytes"`
	SHA256         string    `json:"sha256"`
	Reason         string    `json:"reason"`
	SystemSlug     string    `json:"systemSlug,omitempty"`
	Format         string    `json:"format,omitempty"`
	MediaType      string    `json:"mediaType,omitempty"`
	RuntimeProfile string    `json:"runtimeProfile,omitempty"`
	ROMSHA1        string    `json:"romSha1,omitempty"`
	ROMMD5         string    `json:"romMd5,omitempty"`
	DisplayTitle   string    `json:"displayTitle,omitempty"`
	ParserLevel    string    `json:"parserLevel,omitempty"`
	TrustLevel     string    `json:"trustLevel,omitempty"`
	UploadedAt     time.Time `json:"uploadedAt"`
	UploadSource   string    `json:"uploadSource,omitempty"`
}

type validationStatus struct {
	GeneratedAt     time.Time                  `json:"generatedAt"`
	Counts          map[string]int             `json:"counts"`
	Systems         map[string]int             `json:"systems"`
	QuarantineCount int                        `json:"quarantineCount"`
	Quarantine      []quarantineRecord         `json:"quarantine"`
	CoverageSummary validationCoverageSummary  `json:"coverageSummary"`
	Coverage        []validationCoverageRecord `json:"coverage"`
	LastRescan      *saveRescanResult          `json:"lastRescan,omitempty"`
}

func (a *app) handleSavesPreview(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: err.Error(), StatusCode: http.StatusBadRequest})
		return
	}
	formValue := func(key string) string {
		return r.FormValue(key)
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

	items, err := a.previewMultipartUpload(header.Filename, payload, formValue)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: err.Error(), StatusCode: http.StatusUnprocessableEntity})
		return
	}
	response := saveUploadPreviewResponse{Success: true, Items: items}
	for _, item := range items {
		if item.Accepted {
			response.AcceptedCount++
		} else {
			response.RejectedCount++
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) handleValidationStatus(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	status, err := a.buildValidationStatus(nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "validation": status})
}

func (a *app) handleValidationRescan(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	status, err := a.buildValidationStatus(&result)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	a.appendSyncLog(syncLogInput{DeviceName: "API", Action: "validation_rescan", Game: "Save store"})
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "options": options, "result": result, "validation": status})
}

func (a *app) previewMultipartUpload(filename string, payload []byte, formValue func(string) string) ([]saveUploadPreviewItem, error) {
	if expanded, isArchive, err := expandMultipartSaveUpload(filename, payload, formValue); isArchive {
		if err != nil {
			return nil, err
		}
		items := make([]saveUploadPreviewItem, 0, len(expanded))
		for _, upload := range expanded {
			items = append(items, a.previewOneUpload(upload.Filename, upload.SourcePath, upload.Payload, upload.Game, upload.SystemSlug, upload.Metadata, formValue, false))
		}
		return items, nil
	}

	gameInfo := fallbackGameFromFilename(filename)
	metadata, wiiTitleCode := wiiUploadMetadata(nil, filename, formValue)
	if wiiTitleCode != "" && strings.EqualFold(safeFilename(filename), "data.bin") {
		gameInfo = wiiGameFromTitleCode(wiiTitleCode)
	}
	declaredSystem := safeMultipartSystemSlug(formValue("system"), gameInfo.System)
	if declaredSystem == "unknown-system" && wiiTitleCode != "" && strings.EqualFold(safeFilename(filename), "data.bin") {
		declaredSystem = "wii"
	}
	return []saveUploadPreviewItem{
		a.previewOneUpload(filename, "", payload, gameInfo, declaredSystem, metadata, formValue, false),
	}, nil
}

func (a *app) previewOneUpload(filename, sourcePath string, payload []byte, gameInfo game, declaredSystem string, metadata any, formValue func(string) string, trustedHelper bool) saveUploadPreviewItem {
	runtimeProfile := requestedRuntimeProfileFromForm(formValue, declaredSystem)
	input := saveCreateInput{
		Filename:            filename,
		Payload:             payload,
		Game:                gameInfo,
		Format:              inferSaveFormat(filename),
		Metadata:            metadata,
		ROMSHA1:             strings.TrimSpace(formValue("rom_sha1")),
		ROMMD5:              strings.TrimSpace(formValue("rom_md5")),
		SlotName:            firstNonEmpty(strings.TrimSpace(formValue("slotName")), sourcePath),
		SystemSlug:          declaredSystem,
		GameSlug:            canonicalSegment(gameInfo.Name, "unknown-game"),
		TrustedHelperSystem: trustedHelper || strings.TrimSpace(formValue("system")) != "",
	}
	if runtimeProfile != "" && isProjectionCapableSystem(declaredSystem) {
		if normalized, err := normalizeProjectionUpload(input, runtimeProfile); err == nil {
			input = normalized
		} else {
			return rejectedPreviewItem(filename, sourcePath, payload, declaredSystem, inferSaveFormat(filename), runtimeProfile, err.Error())
		}
	}

	preview := a.normalizeSaveInputDetailed(input)
	item := previewItemFromNormalized(filename, sourcePath, payload, runtimeProfile, preview)
	if preview.Rejected || !isSupportedSystemSlug(preview.Input.SystemSlug) {
		item.Accepted = false
		item.Reason = firstNonEmpty(strings.TrimSpace(preview.RejectReason), errUnsupportedSaveFormat.Error())
		return item
	}
	item.Accepted = true
	return item
}

func previewItemFromNormalized(filename, sourcePath string, payload []byte, runtimeProfile string, preview normalizedSaveInputResult) saveUploadPreviewItem {
	sum := sha256.Sum256(payload)
	input := preview.Input
	item := saveUploadPreviewItem{
		Filename:       safeFilename(filename),
		SourcePath:     strings.TrimSpace(strings.ReplaceAll(sourcePath, "\\", "/")),
		Accepted:       !preview.Rejected,
		DisplayTitle:   strings.TrimSpace(input.DisplayTitle),
		SystemSlug:     strings.TrimSpace(input.SystemSlug),
		RegionCode:     strings.TrimSpace(input.RegionCode),
		RegionFlag:     strings.TrimSpace(input.RegionFlag),
		Format:         strings.TrimSpace(input.Format),
		MediaType:      strings.TrimSpace(input.MediaType),
		RuntimeProfile: firstNonEmpty(strings.TrimSpace(input.RuntimeProfile), runtimeProfile),
		ROMSHA1:        strings.TrimSpace(input.ROMSHA1),
		ROMMD5:         strings.TrimSpace(input.ROMMD5),
		SizeBytes:      len(payload),
		SHA256:         hex.EncodeToString(sum[:]),
		Reason:         strings.TrimSpace(preview.RejectReason),
		Inspection:     input.Inspection,
	}
	if input.Game.System != nil {
		item.SystemName = input.Game.System.Name
	}
	if item.DisplayTitle == "" {
		item.DisplayTitle = strings.TrimSpace(input.Game.DisplayTitle)
	}
	if item.DisplayTitle == "" {
		item.DisplayTitle = strings.TrimSuffix(item.Filename, filepath.Ext(item.Filename))
	}
	if input.Inspection != nil {
		item.ParserLevel = strings.TrimSpace(input.Inspection.ParserLevel)
		item.TrustLevel = strings.TrimSpace(input.Inspection.TrustLevel)
		item.Evidence = append([]string(nil), input.Inspection.Evidence...)
		item.Warnings = append([]string(nil), input.Inspection.Warnings...)
	}
	if item.ParserLevel == "" {
		item.ParserLevel = saveParserLevelNone
	}
	if item.TrustLevel == "" {
		item.TrustLevel = validationTrustLevel(item.ParserLevel)
	}
	item.DownloadProfiles = downloadProfilesForSummary(saveSummary{
		SystemSlug:        item.SystemSlug,
		RuntimeProfile:    item.RuntimeProfile,
		MediaType:         item.MediaType,
		Filename:          item.Filename,
		DisplayTitle:      item.DisplayTitle,
		ProjectionCapable: input.ProjectionCapable,
		DownloadProfiles:  nil,
	})
	return item
}

func rejectedPreviewItem(filename, sourcePath string, payload []byte, systemSlug, format, runtimeProfile, reason string) saveUploadPreviewItem {
	sum := sha256.Sum256(payload)
	return saveUploadPreviewItem{
		Filename:       safeFilename(filename),
		SourcePath:     strings.TrimSpace(strings.ReplaceAll(sourcePath, "\\", "/")),
		Accepted:       false,
		SystemSlug:     canonicalSegment(systemSlug, "unknown-system"),
		Format:         strings.TrimSpace(format),
		RuntimeProfile: strings.TrimSpace(runtimeProfile),
		ROMSHA1:        "",
		ROMMD5:         "",
		SizeBytes:      len(payload),
		SHA256:         hex.EncodeToString(sum[:]),
		ParserLevel:    saveParserLevelNone,
		TrustLevel:     validationTrustLevel(saveParserLevelNone),
		Reason:         strings.TrimSpace(reason),
	}
}

func (a *app) quarantineRejectedUpload(filename, sourcePath string, payload []byte, item saveUploadPreviewItem, uploadSource string) {
	store := a.currentSaveStore()
	if store == nil || len(payload) == 0 {
		return
	}
	now := time.Now().UTC()
	sum := sha256.Sum256(payload)
	shaHex := hex.EncodeToString(sum[:])
	id := fmt.Sprintf("%s-%s", now.Format("20060102T150405.000000000Z"), shaHex[:12])
	dir, err := safeJoinUnderRoot(store.root, quarantineDirName, canonicalSegment(id, "rejected"))
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	safeName := safeFilename(filename)
	ext := filepath.Ext(safeName)
	if ext == "" {
		ext = ".bin"
	}
	payloadFile := "payload" + ext
	record := quarantineRecord{
		ID:             filepath.Base(dir),
		Filename:       safeName,
		SourcePath:     strings.TrimSpace(strings.ReplaceAll(sourcePath, "\\", "/")),
		PayloadFile:    payloadFile,
		SizeBytes:      len(payload),
		SHA256:         shaHex,
		Reason:         firstNonEmpty(strings.TrimSpace(item.Reason), errUnsupportedSaveFormat.Error()),
		SystemSlug:     strings.TrimSpace(item.SystemSlug),
		Format:         strings.TrimSpace(item.Format),
		MediaType:      strings.TrimSpace(item.MediaType),
		RuntimeProfile: strings.TrimSpace(item.RuntimeProfile),
		ROMSHA1:        strings.TrimSpace(item.ROMSHA1),
		ROMMD5:         strings.TrimSpace(item.ROMMD5),
		DisplayTitle:   strings.TrimSpace(item.DisplayTitle),
		ParserLevel:    firstNonEmpty(strings.TrimSpace(item.ParserLevel), saveParserLevelNone),
		TrustLevel:     firstNonEmpty(strings.TrimSpace(item.TrustLevel), validationTrustLevel(item.ParserLevel)),
		UploadedAt:     now,
		UploadSource:   strings.TrimSpace(uploadSource),
	}
	if err := writeFileAtomic(filepath.Join(dir, payloadFile), payload, 0o644); err != nil {
		return
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return
	}
	_ = writeFileAtomic(filepath.Join(dir, "metadata.json"), data, 0o644)
}

func (a *app) listQuarantineRecords() ([]quarantineRecord, error) {
	store := a.currentSaveStore()
	if store == nil {
		return nil, nil
	}
	root, err := safeJoinUnderRoot(store.root, quarantineDirName)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	records := make([]quarantineRecord, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name(), "metadata.json"))
		if err != nil {
			continue
		}
		var record quarantineRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		if strings.TrimSpace(record.ID) == "" {
			record.ID = entry.Name()
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].UploadedAt.After(records[j].UploadedAt)
	})
	return records, nil
}

func (a *app) buildValidationStatus(lastRescan *saveRescanResult) (validationStatus, error) {
	records := a.snapshotSaveRecords()
	counts := map[string]int{
		"mediaVerified":     0,
		"romVerified":       0,
		"structureVerified": 0,
		"semanticVerified":  0,
		"unknown":           0,
	}
	systems := map[string]int{}
	coverage := make([]validationCoverageRecord, 0, len(records))
	coverageSummary := validationCoverageSummary{Total: len(records)}
	for _, record := range records {
		slug := canonicalSegment(saveRecordSystemSlug(record), "unknown-system")
		systems[slug]++
		level := saveParserLevelNone
		trust := ""
		parserID := ""
		gameplayFactCount := 0
		if record.Summary.Inspection != nil {
			level = strings.TrimSpace(record.Summary.Inspection.ParserLevel)
			trust = strings.TrimSpace(record.Summary.Inspection.TrustLevel)
			parserID = strings.TrimSpace(record.Summary.Inspection.ParserID)
			gameplayFactCount = validationGameplayFactCount(record.Summary.Inspection.SemanticFields)
		}
		switch validationBucket(level, trust) {
		case "semantic":
			counts["semanticVerified"]++
			coverageSummary.Semantic++
		case "structure":
			counts["structureVerified"]++
		case "rom":
			counts["romVerified"]++
		case "media":
			counts["mediaVerified"]++
		default:
			counts["unknown"]++
		}
		cheatsSupported := record.Summary.Cheats != nil && record.Summary.Cheats.Supported && record.Summary.Cheats.AvailableCount > 0
		if cheatsSupported {
			coverageSummary.Cheats++
		}
		if gameplayFactCount > 0 {
			coverageSummary.GameplayFacts++
		} else {
			coverageSummary.Missing++
		}
		systemName := ""
		if record.Summary.Game.System != nil {
			systemName = record.Summary.Game.System.Name
		}
		cheatCount := 0
		if record.Summary.Cheats != nil {
			cheatCount = record.Summary.Cheats.AvailableCount
		}
		coverage = append(coverage, validationCoverageRecord{
			SaveID:            record.Summary.ID,
			DisplayTitle:      firstNonEmpty(record.Summary.DisplayTitle, record.Summary.Game.DisplayTitle, record.Summary.Game.Name, record.Summary.Filename),
			SystemSlug:        slug,
			SystemName:        systemName,
			ParserLevel:       level,
			ParserID:          parserID,
			TrustLevel:        trust,
			GameplayFactCount: gameplayFactCount,
			HasGameplayFacts:  gameplayFactCount > 0,
			CheatsSupported:   cheatsSupported,
			CheatCount:        cheatCount,
			UpdatedAt:         record.Summary.CreatedAt,
		})
	}
	sort.Slice(coverage, func(i, j int) bool {
		leftMissing := !coverage[i].HasGameplayFacts && !coverage[i].CheatsSupported
		rightMissing := !coverage[j].HasGameplayFacts && !coverage[j].CheatsSupported
		if leftMissing != rightMissing {
			return leftMissing
		}
		if coverage[i].SystemSlug != coverage[j].SystemSlug {
			return coverage[i].SystemSlug < coverage[j].SystemSlug
		}
		return strings.ToLower(coverage[i].DisplayTitle) < strings.ToLower(coverage[j].DisplayTitle)
	})
	quarantine, err := a.listQuarantineRecords()
	if err != nil {
		return validationStatus{}, err
	}
	return validationStatus{
		GeneratedAt:     time.Now().UTC(),
		Counts:          counts,
		Systems:         systems,
		QuarantineCount: len(quarantine),
		Quarantine:      quarantine,
		CoverageSummary: coverageSummary,
		Coverage:        coverage,
		LastRescan:      lastRescan,
	}, nil
}

func validationBucket(parserLevel, trustLevel string) string {
	level := strings.TrimSpace(strings.ToLower(parserLevel))
	trust := strings.TrimSpace(strings.ToLower(trustLevel))
	switch {
	case level == saveParserLevelSemantic || strings.Contains(trust, "semantic"):
		return "semantic"
	case level == saveParserLevelStructural || level == saveParserLevelContainer || strings.Contains(trust, "structure"):
		return "structure"
	case strings.Contains(trust, "rom"):
		return "rom"
	case level != "":
		return "media"
	default:
		return "unknown"
	}
}

func validationTrustLevel(parserLevel string) string {
	switch strings.TrimSpace(parserLevel) {
	case saveParserLevelSemantic:
		return "semantic-verified"
	case saveParserLevelStructural, saveParserLevelContainer:
		return "structure-verified"
	case saveParserLevelNone:
		return "media-verified"
	default:
		return "media-verified"
	}
}

func (a *app) currentSaveStore() *saveStore {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.saveStore
}
