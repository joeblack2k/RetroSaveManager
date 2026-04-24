package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type duplicateCleanupResult struct {
	DuplicateGroups          int
	DuplicateVersionsRemoved int
	Updated                  int
}

type projectionCleanupResult struct {
	DuplicateGroups          int
	DuplicateVersionsRemoved int
	SaveRecordIDsToDelete    map[string]struct{}
}

func (a *app) cleanupDuplicateSaveHistory(dryRun bool) (duplicateCleanupResult, error) {
	result := duplicateCleanupResult{}
	records := a.snapshotSaveRecords()
	recordsByID := make(map[string]saveRecord, len(records))
	for _, record := range records {
		recordsByID[strings.TrimSpace(record.Summary.ID)] = record
	}

	removeSaveRecordIDs := map[string]struct{}{}

	psResult, err := a.cleanupPlayStationDuplicateHistory(dryRun)
	if err != nil {
		return result, err
	}
	result.DuplicateGroups += psResult.DuplicateGroups
	result.DuplicateVersionsRemoved += psResult.DuplicateVersionsRemoved
	for id := range psResult.SaveRecordIDsToDelete {
		removeSaveRecordIDs[id] = struct{}{}
	}

	n64Result, err := a.cleanupN64ControllerPakDuplicateHistory(dryRun)
	if err != nil {
		return result, err
	}
	result.DuplicateGroups += n64Result.DuplicateGroups
	result.DuplicateVersionsRemoved += n64Result.DuplicateVersionsRemoved
	for id := range n64Result.SaveRecordIDsToDelete {
		removeSaveRecordIDs[id] = struct{}{}
	}

	genericGroups, genericRemoved := planGenericSaveDuplicateCleanup(records, removeSaveRecordIDs)
	result.DuplicateGroups += genericGroups
	result.DuplicateVersionsRemoved += genericRemoved

	if dryRun {
		return result, nil
	}

	for id := range removeSaveRecordIDs {
		record, ok := recordsByID[id]
		if !ok {
			continue
		}
		if err := os.RemoveAll(record.dirPath); err != nil {
			return result, fmt.Errorf("remove duplicate save %s: %w", id, err)
		}
		if a.saveStore != nil {
			cleanupEmptyParents(filepath.Dir(record.dirPath), a.saveStore.root)
		}
	}

	updated, err := a.reindexSaveVersionsAfterCleanup(removeSaveRecordIDs)
	if err != nil {
		return result, err
	}
	result.Updated += updated

	if err := a.reloadSavesFromDisk(); err != nil {
		return result, err
	}
	return result, nil
}

func planGenericSaveDuplicateCleanup(records []saveRecord, existingRemovals map[string]struct{}) (int, int) {
	groups := map[string][]saveRecord{}
	latestByKey := map[string]saveRecord{}
	for _, record := range records {
		id := strings.TrimSpace(record.Summary.ID)
		if _, skipped := existingRemovals[id]; skipped {
			continue
		}
		if saveRecordIsRollback(record) || !saveRecordPayloadExists(record) {
			continue
		}
		if _, _, _, ok := playStationProjectionInfoFromRecord(record); ok {
			continue
		}
		if _, _, _, ok := n64ControllerPakProjectionInfoFromRecord(record); ok {
			continue
		}
		key := strings.TrimSpace(canonicalVersionKeyForRecord(record))
		if key == "" {
			continue
		}
		latest, ok := latestByKey[key]
		if !ok || saveRecordSortsAfter(record, latest) {
			latestByKey[key] = record
		}
		groupKey := key + "::" + strings.TrimSpace(record.Summary.SHA256)
		groups[groupKey] = append(groups[groupKey], record)
	}

	duplicateGroups := 0
	duplicateRemoved := 0
	for _, group := range groups {
		if len(group) <= 1 {
			continue
		}
		duplicateGroups++
		latest := latestByKey[strings.TrimSpace(canonicalVersionKeyForRecord(group[0]))]
		survivorID := selectSaveRecordDuplicateSurvivor(group, latest)
		for _, record := range group {
			if strings.TrimSpace(record.Summary.ID) == survivorID {
				continue
			}
			existingRemovals[strings.TrimSpace(record.Summary.ID)] = struct{}{}
			duplicateRemoved++
		}
	}
	return duplicateGroups, duplicateRemoved
}

func selectSaveRecordDuplicateSurvivor(group []saveRecord, latest saveRecord) string {
	latestID := strings.TrimSpace(latest.Summary.ID)
	for _, record := range group {
		if strings.TrimSpace(record.Summary.ID) == latestID {
			return latestID
		}
	}
	oldest := group[0]
	for _, candidate := range group[1:] {
		if saveRecordSortsBefore(candidate, oldest) {
			oldest = candidate
		}
	}
	return strings.TrimSpace(oldest.Summary.ID)
}

func saveRecordSortsBefore(left, right saveRecord) bool {
	if !left.Summary.CreatedAt.Equal(right.Summary.CreatedAt) {
		return left.Summary.CreatedAt.Before(right.Summary.CreatedAt)
	}
	return strings.TrimSpace(left.Summary.ID) < strings.TrimSpace(right.Summary.ID)
}

func (a *app) reindexSaveVersionsAfterCleanup(removeIDs map[string]struct{}) (int, error) {
	records := a.snapshotSaveRecords()
	groups := map[string][]saveRecord{}
	for _, record := range records {
		if _, removed := removeIDs[strings.TrimSpace(record.Summary.ID)]; removed {
			continue
		}
		key := strings.TrimSpace(canonicalVersionKeyForRecord(record))
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], record)
	}

	updated := 0
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			if !group[i].Summary.CreatedAt.Equal(group[j].Summary.CreatedAt) {
				return group[i].Summary.CreatedAt.Before(group[j].Summary.CreatedAt)
			}
			return strings.TrimSpace(group[i].Summary.ID) < strings.TrimSpace(group[j].Summary.ID)
		})
		for idx := range group {
			wantVersion := idx + 1
			if group[idx].Summary.Version == wantVersion && group[idx].Summary.LatestVersion == wantVersion {
				continue
			}
			group[idx].Summary.Version = wantVersion
			group[idx].Summary.LatestVersion = wantVersion
			if err := persistSaveRecordMetadata(group[idx]); err != nil {
				return updated, err
			}
			updated++
		}
	}
	return updated, nil
}

func (a *app) cleanupPlayStationDuplicateHistory(dryRun bool) (projectionCleanupResult, error) {
	store := a.playStationSyncStore()
	if store == nil {
		return projectionCleanupResult{SaveRecordIDsToDelete: map[string]struct{}{}}, nil
	}

	result := projectionCleanupResult{SaveRecordIDsToDelete: map[string]struct{}{}}
	store.mu.Lock()
	defer store.mu.Unlock()

	revisionRemap := map[string]string{}
	for key, logical := range store.state.LogicalSaves {
		groups := map[string][]psLogicalSaveRevision{}
		for _, revision := range logical.Revisions {
			if psLogicalRevisionIsRollback(revision) {
				continue
			}
			groups[strings.TrimSpace(revision.SHA256)] = append(groups[strings.TrimSpace(revision.SHA256)], revision)
		}
		if len(groups) == 0 {
			continue
		}
		removedIDs := map[string]struct{}{}
		for _, group := range groups {
			if len(group) <= 1 {
				continue
			}
			result.DuplicateGroups++
			survivorID := selectPSRevisionDuplicateSurvivor(group, strings.TrimSpace(logical.LatestRevisionID))
			for _, revision := range group {
				if strings.TrimSpace(revision.ID) == survivorID {
					continue
				}
				removedIDs[strings.TrimSpace(revision.ID)] = struct{}{}
				revisionRemap[strings.TrimSpace(revision.ID)] = survivorID
				result.DuplicateVersionsRemoved++
			}
		}
		if len(removedIDs) == 0 {
			continue
		}
		filtered := make([]psLogicalSaveRevision, 0, len(logical.Revisions)-len(removedIDs))
		for _, revision := range logical.Revisions {
			if _, removed := removedIDs[strings.TrimSpace(revision.ID)]; removed {
				continue
			}
			filtered = append(filtered, revision)
		}
		logical.Revisions = filtered
		if remapped, removed := revisionRemap[strings.TrimSpace(logical.LatestRevisionID)]; removed {
			logical.LatestRevisionID = remapped
		}
		if strings.TrimSpace(logical.LatestRevisionID) == "" {
			if latest, ok := logical.latestRevision(); ok {
				logical.LatestRevisionID = latest.ID
			}
		}
		store.state.LogicalSaves[key] = logical
	}
	if len(revisionRemap) > 0 {
		remapPlayStationManifestRevisions(store.state.Imports, revisionRemap)
		remapPlayStationProjectionManifestRevisions(store.state.Projections, revisionRemap)
	}

	projectionRemap := map[string]string{}
	latestProjectionByLine := map[string]string{}
	for lineKey, line := range store.state.ProjectionLines {
		latestProjectionByLine[lineKey] = strings.TrimSpace(line.LatestProjectionID)
	}
	projectionGroups := map[string][]psProjection{}
	for _, projection := range store.state.Projections {
		if strings.HasPrefix(strings.TrimSpace(projection.SourceImportID), "rollback:") {
			continue
		}
		groupKey := strings.TrimSpace(projection.ProjectionLineKey) + "::" + strings.TrimSpace(projection.SHA256)
		projectionGroups[groupKey] = append(projectionGroups[groupKey], projection)
	}
	projectionDirsToDelete := make([]string, 0)
	for _, group := range projectionGroups {
		if len(group) <= 1 {
			continue
		}
		result.DuplicateGroups++
		latestProjectionID := latestProjectionByLine[strings.TrimSpace(group[0].ProjectionLineKey)]
		survivorID := selectPSProjectionDuplicateSurvivor(group, latestProjectionID)
		for _, projection := range group {
			if strings.TrimSpace(projection.ID) == survivorID {
				continue
			}
			projectionRemap[strings.TrimSpace(projection.ID)] = survivorID
			result.DuplicateVersionsRemoved++
			if saveID := strings.TrimSpace(projection.SaveRecordID); saveID != "" {
				result.SaveRecordIDsToDelete[saveID] = struct{}{}
			}
			if dir := filepath.Dir(strings.TrimSpace(projection.PayloadPath)); dir != "." && dir != "" {
				projectionDirsToDelete = append(projectionDirsToDelete, dir)
			}
			delete(store.state.Projections, strings.TrimSpace(projection.ID))
		}
	}
	if len(projectionRemap) > 0 {
		for key, line := range store.state.ProjectionLines {
			if remapped, ok := projectionRemap[strings.TrimSpace(line.LatestProjectionID)]; ok {
				line.LatestProjectionID = remapped
				store.state.ProjectionLines[key] = line
			}
		}
		for key, device := range store.state.DeviceLines {
			if remapped, ok := projectionRemap[strings.TrimSpace(device.LastDownloadedProjection)]; ok {
				device.LastDownloadedProjection = remapped
				store.state.DeviceLines[key] = device
			}
		}
		for key, artifact := range store.state.Imports {
			if remapped, ok := projectionRemap[strings.TrimSpace(artifact.BaselineProjection)]; ok {
				artifact.BaselineProjection = remapped
				store.state.Imports[key] = artifact
			}
		}
	}

	if dryRun {
		return result, nil
	}
	for _, dir := range projectionDirsToDelete {
		if err := os.RemoveAll(dir); err != nil {
			return result, fmt.Errorf("remove duplicate playstation projection artifact %s: %w", dir, err)
		}
	}
	if result.DuplicateGroups > 0 || result.DuplicateVersionsRemoved > 0 {
		if err := store.persistLocked(); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (a *app) cleanupN64ControllerPakDuplicateHistory(dryRun bool) (projectionCleanupResult, error) {
	store := a.n64ControllerPakStore()
	if store == nil {
		return projectionCleanupResult{SaveRecordIDsToDelete: map[string]struct{}{}}, nil
	}

	result := projectionCleanupResult{SaveRecordIDsToDelete: map[string]struct{}{}}
	store.mu.Lock()
	defer store.mu.Unlock()

	revisionRemap := map[string]string{}
	revisionDirsToDelete := make([]string, 0)
	for key, logical := range store.state.LogicalSaves {
		groups := map[string][]n64ControllerPakLogicalRevision{}
		for _, revision := range logical.Revisions {
			if n64ControllerPakRevisionIsRollback(revision) {
				continue
			}
			groups[strings.TrimSpace(revision.SHA256)] = append(groups[strings.TrimSpace(revision.SHA256)], revision)
		}
		if len(groups) == 0 {
			continue
		}
		removedIDs := map[string]struct{}{}
		for _, group := range groups {
			if len(group) <= 1 {
				continue
			}
			result.DuplicateGroups++
			survivorID := selectN64RevisionDuplicateSurvivor(group, strings.TrimSpace(logical.LatestRevisionID))
			for _, revision := range group {
				if strings.TrimSpace(revision.ID) == survivorID {
					continue
				}
				removedIDs[strings.TrimSpace(revision.ID)] = struct{}{}
				revisionRemap[strings.TrimSpace(revision.ID)] = survivorID
				result.DuplicateVersionsRemoved++
				if dir := filepath.Dir(strings.TrimSpace(revision.PayloadPath)); dir != "." && dir != "" {
					revisionDirsToDelete = append(revisionDirsToDelete, dir)
				}
			}
		}
		if len(removedIDs) == 0 {
			continue
		}
		filtered := make([]n64ControllerPakLogicalRevision, 0, len(logical.Revisions)-len(removedIDs))
		for _, revision := range logical.Revisions {
			if _, removed := removedIDs[strings.TrimSpace(revision.ID)]; removed {
				continue
			}
			filtered = append(filtered, revision)
		}
		logical.Revisions = filtered
		if remapped, removed := revisionRemap[strings.TrimSpace(logical.LatestRevisionID)]; removed {
			logical.LatestRevisionID = remapped
		}
		if strings.TrimSpace(logical.LatestRevisionID) == "" {
			if latest, ok := logical.latestRevision(); ok {
				logical.LatestRevisionID = latest.ID
			}
		}
		store.state.LogicalSaves[key] = logical
	}
	if len(revisionRemap) > 0 {
		remapN64ManifestRevisions(store.state.Imports, revisionRemap)
		remapN64ProjectionManifestRevisions(store.state.Projections, revisionRemap)
	}

	projectionRemap := map[string]string{}
	projectionDirsToDelete := make([]string, 0)
	latestProjectionByLine := map[string]string{}
	for lineKey, line := range store.state.ProjectionLines {
		latestProjectionByLine[lineKey] = strings.TrimSpace(line.LatestProjectionID)
	}
	projectionGroups := map[string][]n64ControllerPakProjection{}
	for _, projection := range store.state.Projections {
		if strings.HasPrefix(strings.TrimSpace(projection.SourceImportID), "rollback:") {
			continue
		}
		groupKey := strings.TrimSpace(projection.ProjectionLineKey) + "::" + strings.TrimSpace(projection.SHA256)
		projectionGroups[groupKey] = append(projectionGroups[groupKey], projection)
	}
	for _, group := range projectionGroups {
		if len(group) <= 1 {
			continue
		}
		result.DuplicateGroups++
		latestProjectionID := latestProjectionByLine[strings.TrimSpace(group[0].ProjectionLineKey)]
		survivorID := selectN64ProjectionDuplicateSurvivor(group, latestProjectionID)
		for _, projection := range group {
			if strings.TrimSpace(projection.ID) == survivorID {
				continue
			}
			projectionRemap[strings.TrimSpace(projection.ID)] = survivorID
			result.DuplicateVersionsRemoved++
			if saveID := strings.TrimSpace(projection.SaveRecordID); saveID != "" {
				result.SaveRecordIDsToDelete[saveID] = struct{}{}
			}
			if dir := filepath.Dir(strings.TrimSpace(projection.PayloadPath)); dir != "." && dir != "" {
				projectionDirsToDelete = append(projectionDirsToDelete, dir)
			}
			delete(store.state.Projections, strings.TrimSpace(projection.ID))
		}
	}
	if len(projectionRemap) > 0 {
		for key, line := range store.state.ProjectionLines {
			if remapped, ok := projectionRemap[strings.TrimSpace(line.LatestProjectionID)]; ok {
				line.LatestProjectionID = remapped
				store.state.ProjectionLines[key] = line
			}
		}
	}

	if dryRun {
		return result, nil
	}
	for _, dir := range revisionDirsToDelete {
		if err := os.RemoveAll(dir); err != nil {
			return result, fmt.Errorf("remove duplicate n64 controller pak revision artifact %s: %w", dir, err)
		}
	}
	for _, dir := range projectionDirsToDelete {
		if err := os.RemoveAll(dir); err != nil {
			return result, fmt.Errorf("remove duplicate n64 controller pak projection artifact %s: %w", dir, err)
		}
	}
	if result.DuplicateGroups > 0 || result.DuplicateVersionsRemoved > 0 {
		if err := store.persistLocked(); err != nil {
			return result, err
		}
	}
	return result, nil
}

func remapPlayStationManifestRevisions(imports map[string]psImportArtifact, remap map[string]string) {
	for key, artifact := range imports {
		changed := false
		for i := range artifact.Manifest {
			if replacement, ok := remap[strings.TrimSpace(artifact.Manifest[i].RevisionID)]; ok {
				artifact.Manifest[i].RevisionID = replacement
				changed = true
			}
		}
		if changed {
			imports[key] = artifact
		}
	}
}

func remapPlayStationProjectionManifestRevisions(projections map[string]psProjection, remap map[string]string) {
	for key, projection := range projections {
		changed := false
		for i := range projection.Manifest {
			if replacement, ok := remap[strings.TrimSpace(projection.Manifest[i].RevisionID)]; ok {
				projection.Manifest[i].RevisionID = replacement
				changed = true
			}
		}
		if changed {
			projections[key] = projection
		}
	}
}

func remapN64ManifestRevisions(imports map[string]n64ControllerPakImportArtifact, remap map[string]string) {
	for key, artifact := range imports {
		changed := false
		for i := range artifact.Manifest {
			if replacement, ok := remap[strings.TrimSpace(artifact.Manifest[i].RevisionID)]; ok {
				artifact.Manifest[i].RevisionID = replacement
				changed = true
			}
		}
		if changed {
			imports[key] = artifact
		}
	}
}

func remapN64ProjectionManifestRevisions(projections map[string]n64ControllerPakProjection, remap map[string]string) {
	for key, projection := range projections {
		changed := false
		for i := range projection.Manifest {
			if replacement, ok := remap[strings.TrimSpace(projection.Manifest[i].RevisionID)]; ok {
				projection.Manifest[i].RevisionID = replacement
				changed = true
			}
		}
		if changed {
			projections[key] = projection
		}
	}
}

func selectPSRevisionDuplicateSurvivor(group []psLogicalSaveRevision, latestID string) string {
	for _, revision := range group {
		if strings.TrimSpace(revision.ID) == latestID {
			return latestID
		}
	}
	oldest := group[0]
	for _, candidate := range group[1:] {
		if timeSortBefore(candidate.CreatedAt, oldest.CreatedAt, candidate.ID, oldest.ID) {
			oldest = candidate
		}
	}
	return strings.TrimSpace(oldest.ID)
}

func selectN64RevisionDuplicateSurvivor(group []n64ControllerPakLogicalRevision, latestID string) string {
	for _, revision := range group {
		if strings.TrimSpace(revision.ID) == latestID {
			return latestID
		}
	}
	oldest := group[0]
	for _, candidate := range group[1:] {
		if timeSortBefore(candidate.CreatedAt, oldest.CreatedAt, candidate.ID, oldest.ID) {
			oldest = candidate
		}
	}
	return strings.TrimSpace(oldest.ID)
}

func selectPSProjectionDuplicateSurvivor(group []psProjection, latestID string) string {
	for _, projection := range group {
		if strings.TrimSpace(projection.ID) == latestID {
			return latestID
		}
	}
	oldest := group[0]
	for _, candidate := range group[1:] {
		if timeSortBefore(candidate.CreatedAt, oldest.CreatedAt, candidate.ID, oldest.ID) {
			oldest = candidate
		}
	}
	return strings.TrimSpace(oldest.ID)
}

func selectN64ProjectionDuplicateSurvivor(group []n64ControllerPakProjection, latestID string) string {
	for _, projection := range group {
		if strings.TrimSpace(projection.ID) == latestID {
			return latestID
		}
	}
	oldest := group[0]
	for _, candidate := range group[1:] {
		if timeSortBefore(candidate.CreatedAt, oldest.CreatedAt, candidate.ID, oldest.ID) {
			oldest = candidate
		}
	}
	return strings.TrimSpace(oldest.ID)
}

func timeSortBefore(leftTime, rightTime time.Time, leftID, rightID string) bool {
	if !leftTime.Equal(rightTime) {
		return leftTime.Before(rightTime)
	}
	return strings.TrimSpace(leftID) < strings.TrimSpace(rightID)
}
