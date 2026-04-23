package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type playStationLogicalContext struct {
	Projection psProjection
	Logical    psLogicalSave
}

type playStationLogicalHistory struct {
	Game         game
	DisplayTitle string
	SystemSlug   string
	Summary      map[string]any
	Versions     []saveSummary
}

func (a *app) playStationTemplateInputFromSummary(summary saveSummary) saveCreateInput {
	return saveCreateInput{
		Metadata:      summary.Metadata,
		RegionCode:    summary.RegionCode,
		RegionFlag:    summary.RegionFlag,
		LanguageCodes: summary.LanguageCodes,
		CoverArtURL:   summary.CoverArtURL,
		CreatedAt:     time.Now().UTC(),
	}
}

func (a *app) materializePlayStationProjections(template saveCreateInput, built []psBuiltProjection) (map[string]saveRecord, error) {
	store := a.playStationSyncStore()
	if store == nil {
		return nil, fmt.Errorf("playstation store is not initialized")
	}
	recordsByLine := make(map[string]saveRecord, len(built))
	createdAt := template.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	for _, candidate := range built {
		portability := candidate.Portable
		sys := supportedSystemFromSlug(candidate.SystemSlug)
		record, err := a.createSave(saveCreateInput{
			Filename:   candidate.Filename,
			Payload:    candidate.Payload,
			Game:       game{Name: candidate.CardSlot, DisplayTitle: candidate.CardSlot, System: sys},
			Format:     inferSaveFormat(candidate.Filename),
			Metadata:   mergePlayStationMetadata(template.Metadata, candidate),
			ROMSHA1:    projectionConflictKey(candidate.RuntimeProfile, candidate.CardSlot),
			SlotName:   candidate.CardSlot,
			SystemSlug: candidate.SystemSlug,
			GameSlug:   canonicalSegment(candidate.CardSlot, "memory-card"),
			SystemPath: firstNonEmpty(func() string {
				if sys != nil {
					return sys.Name
				}
				return ""
			}(), candidate.SystemSlug),
			GamePath:       candidate.CardSlot,
			DisplayTitle:   candidate.CardSlot,
			RegionCode:     template.RegionCode,
			RegionFlag:     template.RegionFlag,
			LanguageCodes:  template.LanguageCodes,
			CoverArtURL:    template.CoverArtURL,
			MemoryCard:     candidate.MemoryCard,
			RuntimeProfile: candidate.RuntimeProfile,
			CardSlot:       candidate.CardSlot,
			ProjectionID:   candidate.ProjectionID,
			SourceImportID: candidate.SourceImportID,
			Portable:       &portability,
			CreatedAt:      createdAt,
		})
		if err != nil {
			return nil, err
		}
		playStationSummaryFields(&record.Summary, candidate)
		if err := persistSaveRecordMetadata(record); err == nil {
			a.replaceSaveRecord(record)
		}
		if err := store.attachProjectionSaveRecord(candidate.ProjectionID, record.Summary.ID); err != nil {
			return nil, err
		}
		a.saveCreatedEvent(record)
		a.resolveConflictForSave(record)
		recordsByLine[candidate.ProjectionLineKey] = record
	}
	return recordsByLine, nil
}

func (a *app) playStationLogicalHistoryForSaveRecord(saveRecordID, logicalKey string) (playStationLogicalHistory, error) {
	store := a.playStationSyncStore()
	if store == nil {
		return playStationLogicalHistory{}, fmt.Errorf("playstation store is not initialized")
	}
	ctx, err := store.logicalContextForSaveRecord(saveRecordID, logicalKey)
	if err != nil {
		return playStationLogicalHistory{}, err
	}
	return buildPlayStationLogicalHistory(ctx), nil
}

func (a *app) downloadPlayStationLogicalSave(saveRecordID, logicalKey, revisionID string) (string, string, []byte, error) {
	store := a.playStationSyncStore()
	if store == nil {
		return "", "", nil, fmt.Errorf("playstation store is not initialized")
	}
	ctx, err := store.logicalContextForSaveRecord(saveRecordID, logicalKey)
	if err != nil {
		return "", "", nil, err
	}
	return buildPlayStationLogicalDownload(ctx, revisionID)
}

func (a *app) rollbackPlayStationLogicalSave(sourceRecord saveRecord, logicalKey, revisionID string) (saveRecord, error) {
	store := a.playStationSyncStore()
	if store == nil {
		return saveRecord{}, fmt.Errorf("playstation store is not initialized")
	}
	runtimeProfile, cardSlot, projectionID, ok := playStationProjectionInfoFromRecord(sourceRecord)
	if !ok {
		return saveRecord{}, fmt.Errorf("save is not a playstation projection")
	}
	store.mu.Lock()
	projection, ok := store.state.Projections[strings.TrimSpace(projectionID)]
	if !ok {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("playstation projection not found")
	}
	if !projectionManifestContainsLogicalKey(projection.Manifest, logicalKey) {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("playstation logical save not found")
	}
	logical, ok := store.state.LogicalSaves[strings.TrimSpace(logicalKey)]
	if !ok {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("playstation logical save not found")
	}
	revision, ok := logical.revisionByID(revisionID)
	if !ok {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("playstation logical revision not found")
	}
	now := time.Now().UTC()
	revision.ID = "ps-logical-revision-" + now.Format("20060102150405.000000000") + "-" + hash12(logical.Key+"::"+revisionID+"::"+now.Format(time.RFC3339Nano))
	revision.ImportID = "rollback:" + sourceRecord.Summary.ID
	revision.CreatedAt = now
	logical.Revisions = append(logical.Revisions, revision)
	logical.LatestRevisionID = revision.ID
	store.state.LogicalSaves[logical.Key] = logical
	built, err := store.rebuildProjectionLinesLocked(projection.SystemSlug, projection.CardSlot, runtimeProfile, "rollback:"+logical.Key, cardSlot)
	if err != nil {
		store.mu.Unlock()
		return saveRecord{}, err
	}
	if err := store.persistLocked(); err != nil {
		store.mu.Unlock()
		return saveRecord{}, err
	}
	store.mu.Unlock()
	template := a.playStationTemplateInputFromSummary(sourceRecord.Summary)
	template.Metadata = mergeRollbackMetadata(sourceRecord)
	template.CreatedAt = now
	recordsByLine, err := a.materializePlayStationProjections(template, built)
	if err != nil {
		return saveRecord{}, err
	}
	record, ok := recordsByLine[projection.ProjectionLineKey]
	if !ok {
		return saveRecord{}, fmt.Errorf("rebuilt playstation projection missing current runtime line")
	}
	return record, nil
}

func (a *app) deletePlayStationLogicalSave(sourceRecord saveRecord, logicalKey string) (int, error) {
	store := a.playStationSyncStore()
	if store == nil {
		return 0, fmt.Errorf("playstation store is not initialized")
	}
	runtimeProfile, cardSlot, projectionID, ok := playStationProjectionInfoFromRecord(sourceRecord)
	if !ok {
		return 0, fmt.Errorf("save is not a playstation projection")
	}
	store.mu.Lock()
	projection, ok := store.state.Projections[strings.TrimSpace(projectionID)]
	if !ok {
		store.mu.Unlock()
		return 0, fmt.Errorf("playstation projection not found")
	}
	if !projectionManifestContainsLogicalKey(projection.Manifest, logicalKey) {
		store.mu.Unlock()
		return 0, fmt.Errorf("playstation logical save not found")
	}
	logical, ok := store.state.LogicalSaves[strings.TrimSpace(logicalKey)]
	if !ok {
		store.mu.Unlock()
		return 0, fmt.Errorf("playstation logical save not found")
	}
	now := time.Now().UTC()
	scopeKey := logicalScopeKey(logical)
	store.state.Tombstones[psTombstoneKey(scopeKey, logical.Key)] = psTombstone{
		ID:         deterministicConflictID(psTombstoneKey(scopeKey, logical.Key)),
		LogicalKey: logical.Key,
		ScopeKey:   scopeKey,
		Reason:     "logical save deleted from API",
		CreatedAt:  now,
	}
	built, err := store.rebuildProjectionLinesLocked(projection.SystemSlug, projection.CardSlot, runtimeProfile, "delete:"+logical.Key, cardSlot)
	if err != nil {
		store.mu.Unlock()
		return 0, err
	}
	if err := store.persistLocked(); err != nil {
		store.mu.Unlock()
		return 0, err
	}
	store.mu.Unlock()
	template := a.playStationTemplateInputFromSummary(sourceRecord.Summary)
	template.Metadata = map[string]any{"deleted": true, "logicalKey": logical.Key}
	template.CreatedAt = now
	if _, err := a.materializePlayStationProjections(template, built); err != nil {
		return 0, err
	}
	return len(a.snapshotSaveRecords()), nil
}

func buildPlayStationLogicalHistory(ctx playStationLogicalContext) playStationLogicalHistory {
	latest, _ := ctx.Logical.latestRevision()
	sys := supportedSystemFromSlug(ctx.Logical.SystemSlug)
	regionCode := normalizeRegionCode(ctx.Logical.RegionCode)
	if regionCode == regionUnknown {
		regionCode = normalizeRegionCode(latest.MemoryEntry.RegionCode)
	}
	coverArtURL := strings.TrimSpace(latest.MemoryEntry.IconDataURL)
	gameInfo := game{
		ID:           deterministicGameID("ps-logical:" + ctx.Logical.Key),
		Name:         ctx.Logical.DisplayTitle,
		DisplayTitle: ctx.Logical.DisplayTitle,
		RegionCode:   regionCode,
		RegionFlag:   regionFlagFromCode(regionCode),
		HasParser:    true,
		System:       sys,
		CoverArtURL:  coverArtURL,
	}
	if coverArtURL != "" {
		thumb := coverArtURL
		box := coverArtURL
		gameInfo.BoxartThumb = &thumb
		gameInfo.Boxart = &box
	}
	versions := make([]saveSummary, 0, len(ctx.Logical.Revisions))
	for i := len(ctx.Logical.Revisions) - 1; i >= 0; i-- {
		revision := ctx.Logical.Revisions[i]
		portable := ctx.Logical.Portable
		entry := revision.MemoryEntry
		entry.LogicalKey = ctx.Logical.Key
		entry.SaveCount = len(ctx.Logical.Revisions)
		entry.LatestVersion = len(ctx.Logical.Revisions)
		entry.LatestSizeBytes = psLogicalSaveLatestSize(ctx.Logical)
		entry.TotalSizeBytes = psLogicalSaveTotalSize(ctx.Logical)
		entry.LatestCreatedAt = latest.CreatedAt.Format(time.RFC3339Nano)
		entry.Portable = &portable
		memoryCard := &memoryCardDetails{Name: ctx.Projection.CardSlot, Entries: []memoryCardEntry{entry}}
		downloadName := playStationLogicalDownloadFilename(ctx.Projection, ctx.Logical)
		versions = append(versions, summaryWithDownloadProfiles(saveSummary{
			ID:              revision.ID,
			Game:            gameInfo,
			DisplayTitle:    ctx.Logical.DisplayTitle,
			SystemSlug:      ctx.Logical.SystemSlug,
			RegionCode:      regionCode,
			RegionFlag:      regionFlagFromCode(regionCode),
			CoverArtURL:     coverArtURL,
			SaveCount:       len(ctx.Logical.Revisions),
			LatestSizeBytes: psLogicalRevisionSize(revision),
			TotalSizeBytes:  psLogicalSaveTotalSize(ctx.Logical),
			LatestVersion:   len(ctx.Logical.Revisions),
			MemoryCard:      memoryCard,
			RuntimeProfile:  ctx.Projection.RuntimeProfile,
			CardSlot:        ctx.Projection.CardSlot,
			ProjectionID:    ctx.Projection.ID,
			SourceImportID:  revision.ImportID,
			Portable:        &portable,
			Filename:        downloadName,
			FileSize:        psLogicalRevisionSize(revision),
			Format:          inferSaveFormat(downloadName),
			Version:         i + 1,
			SHA256:          revision.SHA256,
			CreatedAt:       revision.CreatedAt,
			Metadata: map[string]any{
				"playstationLogical": map[string]any{
					"logicalKey":     ctx.Logical.Key,
					"revisionId":     revision.ID,
					"productCode":    ctx.Logical.ProductCode,
					"runtimeProfile": ctx.Projection.RuntimeProfile,
					"cardSlot":       ctx.Projection.CardSlot,
					"portable":       portable,
				},
			},
		}))
	}
	historySummary := map[string]any{
		"displayTitle":    ctx.Logical.DisplayTitle,
		"system":          sys,
		"regionCode":      regionCode,
		"regionFlag":      regionFlagFromCode(regionCode),
		"languageCodes":   []string{},
		"saveCount":       len(ctx.Logical.Revisions),
		"totalSizeBytes":  psLogicalSaveTotalSize(ctx.Logical),
		"latestVersion":   len(ctx.Logical.Revisions),
		"latestCreatedAt": latest.CreatedAt,
		"runtimeProfile":  ctx.Projection.RuntimeProfile,
		"cardSlot":        ctx.Projection.CardSlot,
	}
	return playStationLogicalHistory{
		Game:         gameInfo,
		DisplayTitle: ctx.Logical.DisplayTitle,
		SystemSlug:   ctx.Logical.SystemSlug,
		Summary:      historySummary,
		Versions:     versions,
	}
}

func buildPlayStationLogicalListSummary(ctx playStationLogicalContext) saveSummary {
	latest, _ := ctx.Logical.latestRevision()
	sys := supportedSystemFromSlug(ctx.Logical.SystemSlug)
	regionCode := normalizeRegionCode(ctx.Logical.RegionCode)
	if regionCode == regionUnknown {
		regionCode = normalizeRegionCode(latest.MemoryEntry.RegionCode)
	}
	coverArtURL := strings.TrimSpace(latest.MemoryEntry.IconDataURL)
	gameInfo := game{
		ID:           deterministicGameID("ps-logical:" + ctx.Logical.Key),
		Name:         ctx.Logical.DisplayTitle,
		DisplayTitle: ctx.Logical.DisplayTitle,
		RegionCode:   regionCode,
		RegionFlag:   regionFlagFromCode(regionCode),
		HasParser:    true,
		System:       sys,
		CoverArtURL:  coverArtURL,
	}
	if coverArtURL != "" {
		thumb := coverArtURL
		box := coverArtURL
		gameInfo.BoxartThumb = &thumb
		gameInfo.Boxart = &box
	}
	portable := ctx.Logical.Portable
	filename := playStationLogicalDownloadFilename(ctx.Projection, ctx.Logical)
	return summaryWithDownloadProfiles(saveSummary{
		ID:              ctx.Projection.SaveRecordID,
		Game:            gameInfo,
		DisplayTitle:    ctx.Logical.DisplayTitle,
		LogicalKey:      ctx.Logical.Key,
		SystemSlug:      ctx.Logical.SystemSlug,
		RegionCode:      regionCode,
		RegionFlag:      regionFlagFromCode(regionCode),
		CoverArtURL:     coverArtURL,
		SaveCount:       len(ctx.Logical.Revisions),
		LatestSizeBytes: psLogicalSaveLatestSize(ctx.Logical),
		TotalSizeBytes:  psLogicalSaveTotalSize(ctx.Logical),
		LatestVersion:   len(ctx.Logical.Revisions),
		RuntimeProfile:  ctx.Projection.RuntimeProfile,
		CardSlot:        ctx.Projection.CardSlot,
		ProjectionID:    ctx.Projection.ID,
		SourceImportID:  latest.ImportID,
		Portable:        &portable,
		Filename:        filename,
		FileSize:        psLogicalSaveLatestSize(ctx.Logical),
		Format:          inferSaveFormat(filename),
		Version:         len(ctx.Logical.Revisions),
		SHA256:          latest.SHA256,
		CreatedAt:       latest.CreatedAt,
		Metadata: map[string]any{
			"playstationLogical": map[string]any{
				"logicalKey":     ctx.Logical.Key,
				"productCode":    ctx.Logical.ProductCode,
				"runtimeProfile": ctx.Projection.RuntimeProfile,
				"cardSlot":       ctx.Projection.CardSlot,
				"portable":       portable,
			},
		},
	})
}

func buildPlayStationLogicalDownload(ctx playStationLogicalContext, revisionID string) (string, string, []byte, error) {
	revision, ok := resolvePlayStationLogicalRevision(ctx.Logical, revisionID)
	if !ok {
		return "", "", nil, fmt.Errorf("playstation logical revision not found")
	}
	filename := playStationLogicalDownloadFilename(ctx.Projection, ctx.Logical)
	switch canonicalSegment(ctx.Logical.SystemSlug, "") {
	case "psx":
		clone := clonePlayStationLogicalWithRevision(ctx.Logical, revision)
		payload, _, _, err := buildProjectionPayload(ctx.Logical.SystemSlug, ctx.Projection.RuntimeProfile, ctx.Projection.CardSlot, filename, []psLogicalSave{clone})
		if err != nil {
			return "", "", nil, err
		}
		return filename, "application/octet-stream", payload, nil
	case "ps2":
		payload, err := buildPS2LogicalSaveArchive(ctx.Logical, revision, filename)
		if err != nil {
			return "", "", nil, err
		}
		return filename, "application/zip", payload, nil
	default:
		return "", "", nil, fmt.Errorf("unsupported playstation logical system %q", ctx.Logical.SystemSlug)
	}
}

func buildPS2LogicalSaveArchive(logical psLogicalSave, revision psLogicalSaveRevision, filename string) ([]byte, error) {
	if revision.PS2 == nil {
		return nil, fmt.Errorf("playstation 2 logical save payload is missing")
	}
	root := strings.TrimSpace(revision.PS2.DirectoryName)
	if root == "" {
		root = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	root = safeFilename(root)
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	createdDirs := map[string]struct{}{}
	for _, node := range revision.PS2.Nodes {
		relPath := strings.Trim(strings.ReplaceAll(node.Path, "\\", "/"), "/")
		if relPath == "" {
			continue
		}
		entryName := filepath.ToSlash(filepath.Join(root, relPath))
		if node.Directory {
			dirName := strings.TrimSuffix(entryName, "/") + "/"
			if _, exists := createdDirs[dirName]; exists {
				continue
			}
			createdDirs[dirName] = struct{}{}
			if _, err := writer.Create(dirName); err != nil {
				_ = writer.Close()
				return nil, err
			}
			continue
		}
		fileWriter, err := writer.Create(entryName)
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err := fileWriter.Write(node.Data); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func projectionManifestContainsLogicalKey(manifest []psManifestEntry, logicalKey string) bool {
	target := strings.TrimSpace(logicalKey)
	for _, entry := range manifest {
		if strings.TrimSpace(entry.LogicalKey) == target {
			return true
		}
	}
	return false
}

func (s *playStationStore) currentProjectionForLogicalLocked(logical psLogicalSave) (psProjection, bool) {
	best := psProjection{}
	found := false
	for _, line := range s.state.ProjectionLines {
		if line.SystemSlug != logical.SystemSlug || strings.TrimSpace(line.LatestProjectionID) == "" {
			continue
		}
		if logical.Portable {
			if strings.TrimSpace(line.SyncLineKey) != strings.TrimSpace(logical.SyncLineKey) {
				continue
			}
		} else if strings.TrimSpace(line.Key) != strings.TrimSpace(logical.ProjectionLineKey) {
			continue
		}
		projection, ok := s.state.Projections[line.LatestProjectionID]
		if !ok || strings.TrimSpace(projection.SaveRecordID) == "" {
			continue
		}
		if !projectionManifestContainsLogicalKey(projection.Manifest, logical.Key) {
			continue
		}
		if !found || projection.CreatedAt.After(best.CreatedAt) || (projection.CreatedAt.Equal(best.CreatedAt) && projection.SaveRecordID > best.SaveRecordID) {
			best = projection
			found = true
		}
	}
	return best, found
}

func (s *playStationStore) listLogicalContexts() []playStationLogicalContext {
	s.mu.Lock()
	defer s.mu.Unlock()

	contexts := make([]playStationLogicalContext, 0, len(s.state.LogicalSaves))
	for _, logical := range s.state.LogicalSaves {
		if _, ok := logical.latestRevision(); !ok {
			continue
		}
		scopeKey := logicalScopeKey(logical)
		if _, tombstoned := s.state.Tombstones[psTombstoneKey(scopeKey, logical.Key)]; tombstoned {
			continue
		}
		projection, ok := s.currentProjectionForLogicalLocked(logical)
		if !ok {
			continue
		}
		contexts = append(contexts, playStationLogicalContext{Projection: projection, Logical: logical})
	}
	sort.Slice(contexts, func(i, j int) bool {
		left, leftOK := contexts[i].Logical.latestRevision()
		right, rightOK := contexts[j].Logical.latestRevision()
		if !leftOK || !rightOK {
			return contexts[i].Logical.Key < contexts[j].Logical.Key
		}
		if left.CreatedAt.Equal(right.CreatedAt) {
			return contexts[i].Logical.Key < contexts[j].Logical.Key
		}
		return left.CreatedAt.After(right.CreatedAt)
	})
	return contexts
}

func (a *app) playStationLogicalListSummaries(systemID int) []saveSummary {
	store := a.playStationSyncStore()
	if store == nil {
		return nil
	}
	contexts := store.listLogicalContexts()
	out := make([]saveSummary, 0, len(contexts))
	for _, ctx := range contexts {
		sys := supportedSystemFromSlug(ctx.Logical.SystemSlug)
		if systemID != 0 && (sys == nil || sys.ID != systemID) {
			continue
		}
		out = append(out, buildPlayStationLogicalListSummary(ctx))
	}
	return out
}

func (s *playStationStore) logicalContextForSaveRecord(saveRecordID, logicalKey string) (playStationLogicalContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	targetSaveID := strings.TrimSpace(saveRecordID)
	targetLogicalKey := strings.TrimSpace(logicalKey)
	if targetSaveID == "" || targetLogicalKey == "" {
		return playStationLogicalContext{}, fmt.Errorf("playstation logical save requires saveId and psLogicalKey")
	}
	var projection psProjection
	foundProjection := false
	for _, candidate := range s.state.Projections {
		if strings.TrimSpace(candidate.SaveRecordID) != targetSaveID {
			continue
		}
		projection = candidate
		foundProjection = true
		break
	}
	if !foundProjection {
		return playStationLogicalContext{}, fmt.Errorf("playstation projection not found")
	}
	if !projectionManifestContainsLogicalKey(projection.Manifest, targetLogicalKey) {
		return playStationLogicalContext{}, fmt.Errorf("playstation logical save not found in projection")
	}
	logical, ok := s.state.LogicalSaves[targetLogicalKey]
	if !ok {
		return playStationLogicalContext{}, fmt.Errorf("playstation logical save not found")
	}
	return playStationLogicalContext{Projection: projection, Logical: logical}, nil
}

func (logical psLogicalSave) revisionByID(id string) (psLogicalSaveRevision, bool) {
	target := strings.TrimSpace(id)
	for _, revision := range logical.Revisions {
		if strings.TrimSpace(revision.ID) == target {
			return revision, true
		}
	}
	return psLogicalSaveRevision{}, false
}

func logicalScopeKey(logical psLogicalSave) string {
	if logical.Portable {
		return logical.SyncLineKey
	}
	return logical.ProjectionLineKey
}

func resolvePlayStationLogicalRevision(logical psLogicalSave, revisionID string) (psLogicalSaveRevision, bool) {
	if strings.TrimSpace(revisionID) == "" {
		return logical.latestRevision()
	}
	return logical.revisionByID(revisionID)
}

func clonePlayStationLogicalWithRevision(logical psLogicalSave, revision psLogicalSaveRevision) psLogicalSave {
	clone := logical
	clone.Revisions = []psLogicalSaveRevision{revision}
	clone.LatestRevisionID = revision.ID
	return clone
}

func playStationLogicalDownloadFilename(projection psProjection, logical psLogicalSave) string {
	base := strings.TrimSpace(firstNonEmpty(logical.DisplayTitle, logical.ProductCode, projection.CardSlot, "playstation-save"))
	switch canonicalSegment(logical.SystemSlug, "") {
	case "psx":
		ext := strings.ToLower(filepath.Ext(strings.TrimSpace(projection.Filename)))
		switch ext {
		case ".mcr", ".mcd", ".mc", ".gme", ".vmp", ".psv":
		default:
			ext = ".mcr"
		}
		return safeFilename(base + ext)
	case "ps2":
		return safeFilename(base + ".zip")
	default:
		return safeFilename(base + ".bin")
	}
}
