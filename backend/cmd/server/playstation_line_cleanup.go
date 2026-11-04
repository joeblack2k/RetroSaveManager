package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A PlayStation save belongs to exactly one projection line -- the card the
// game is read from. Before a line move counted as a change, a save could be
// served by every line it had ever touched: each kept a projection holding it,
// and a device restoring one of those lines wrote the save back onto that
// game's card.
//
// Once the sync path stops creating them, the lines already spread across a
// live server stay until something retires them, and retiring a line by hand
// means editing four maps and a directory tree in step. This does that in one
// pass, under the store's lock, and reports what it would touch before it
// touches anything.

// psLineRetirement is everything one cleanup run would remove or re-point.
type psLineRetirement struct {
	Lines        []string // projection line keys
	Projections  []string // projection ids, and their directories
	SaveRecords  []string // save record ids owned only by those projections
	DeviceRefs   []string // "<deviceKey>: <projectionID>" cleared
	ImportRefs   []string // "<importID>: <projectionID>" cleared
	KeptLines    []string
	KeptProjects []string
}

// lineStillOwnsASave reports whether the save a projection carries still
// belongs to the line the projection sits on. Unlike the sync path's check it
// compares the line as well as the scope: a save whose scope names another
// line is one this card no longer holds, however recently it was built.
func lineStillOwnsASave(systemSlug string, logicals map[string]psLogicalSave, projection psProjection) bool {
	for _, entry := range projection.Manifest {
		logical, ok := logicals[entry.LogicalKey]
		if !ok || logical.SystemSlug != systemSlug {
			continue
		}
		if projectionLineForLogical(logical) != projection.ProjectionLineKey {
			continue
		}
		if psScopeKey(logical.Portable, logical.SyncLineKey, logical.ProjectionLineKey) == entry.ScopeKey {
			return true
		}
	}
	return false
}

// planPlayStationLineRetirement decides what to retire. A line is kept when it
// still owns a save: its latest projection serves at least one save whose
// scope still names that line. Everything else has nothing left to serve and
// is retired whole.
func (s *playStationStore) planPlayStationLineRetirement(systemSlug string, records map[string]saveRecord) psLineRetirement {
	plan := psLineRetirement{}
	for key, line := range s.state.ProjectionLines {
		if line.SystemSlug != systemSlug {
			continue
		}
		id := strings.TrimSpace(line.LatestProjectionID)
		projection, ok := s.state.Projections[id]
		if !ok {
			plan.KeptLines = append(plan.KeptLines, key)
			continue
		}
		if lineStillOwnsASave(systemSlug, s.state.LogicalSaves, projection) {
			plan.KeptLines = append(plan.KeptLines, key)
			plan.KeptProjects = append(plan.KeptProjects, projection.ID)
			continue
		}
		plan.Lines = append(plan.Lines, key)
	}

	retiring := make(map[string]struct{}, len(plan.Lines))
	for _, key := range plan.Lines {
		retiring[key] = struct{}{}
	}
	// Every projection on a retiring line goes, not only its latest: a device
	// may hold any of them as its baseline.
	for id, projection := range s.state.Projections {
		if _, ok := retiring[projection.ProjectionLineKey]; ok {
			plan.Projections = append(plan.Projections, id)
		}
	}
	sort.Strings(plan.Lines)
	sort.Strings(plan.Projections)
	sort.Strings(plan.KeptLines)
	sort.Strings(plan.KeptProjects)

	// A save record is only removable when no surviving projection owns it.
	stillOwned := map[string]struct{}{}
	for id, projection := range s.state.Projections {
		if _, gone := retiring[projection.ProjectionLineKey]; gone {
			continue
		}
		if recordID := strings.TrimSpace(projection.SaveRecordID); recordID != "" {
			stillOwned[recordID] = struct{}{}
		}
		_ = id
	}
	removed := make(map[string]struct{}, len(plan.Projections))
	for _, id := range plan.Projections {
		removed[id] = struct{}{}
	}
	for _, id := range plan.Projections {
		recordID := strings.TrimSpace(s.state.Projections[id].SaveRecordID)
		if recordID == "" {
			continue
		}
		if _, kept := stillOwned[recordID]; kept {
			continue
		}
		if _, ok := records[recordID]; !ok {
			continue
		}
		plan.SaveRecords = append(plan.SaveRecords, recordID)
	}
	sort.Strings(plan.SaveRecords)

	for deviceKey, dev := range s.state.DeviceLines {
		if id := strings.TrimSpace(dev.LastDownloadedProjection); id != "" {
			if _, ok := removed[id]; ok {
				plan.DeviceRefs = append(plan.DeviceRefs, deviceKey+": "+id)
			}
		}
	}
	for importID, artifact := range s.state.Imports {
		if id := strings.TrimSpace(artifact.BaselineProjection); id != "" {
			if _, ok := removed[id]; ok {
				plan.ImportRefs = append(plan.ImportRefs, importID+": "+id)
			}
		}
	}
	sort.Strings(plan.DeviceRefs)
	sort.Strings(plan.ImportRefs)
	return plan
}

// reportPlayStationLineRetirement renders the plan as the dry-run list.
func reportPlayStationLineRetirement(plan psLineRetirement) string {
	var b strings.Builder
	fmt.Fprintf(&b, "keep %d line(s), %d projection(s)\n", len(plan.KeptLines), len(plan.KeptProjects))
	for _, key := range plan.KeptLines {
		fmt.Fprintf(&b, "  keep line %s\n", key)
	}
	fmt.Fprintf(&b, "retire %d line(s)\n", len(plan.Lines))
	for _, key := range plan.Lines {
		fmt.Fprintf(&b, "  retire line %s\n", key)
	}
	fmt.Fprintf(&b, "remove %d projection(s) and their directories\n", len(plan.Projections))
	for _, id := range plan.Projections {
		fmt.Fprintf(&b, "  remove projection %s\n", id)
	}
	fmt.Fprintf(&b, "remove %d save record(s) no surviving projection serves\n", len(plan.SaveRecords))
	for _, id := range plan.SaveRecords {
		fmt.Fprintf(&b, "  remove record %s\n", id)
	}
	fmt.Fprintf(&b, "clear %d device reference(s)\n", len(plan.DeviceRefs))
	for _, ref := range plan.DeviceRefs {
		fmt.Fprintf(&b, "  clear device %s\n", ref)
	}
	fmt.Fprintf(&b, "clear %d import reference(s)\n", len(plan.ImportRefs))
	for _, ref := range plan.ImportRefs {
		fmt.Fprintf(&b, "  clear import %s\n", ref)
	}
	return b.String()
}

// applyPlayStationLineRetirement carries out a plan: state first, then the
// payload directories, then the save records. A record whose directory is
// already gone is skipped rather than failed, so a re-run after a partial
// removal finishes instead of stopping.
func (a *app) applyPlayStationLineRetirement(store *playStationStore, plan psLineRetirement) (psLineRetirement, error) {
	removed := make(map[string]struct{}, len(plan.Projections))
	for _, id := range plan.Projections {
		removed[id] = struct{}{}
	}
	retiring := make(map[string]struct{}, len(plan.Lines))
	for _, key := range plan.Lines {
		retiring[key] = struct{}{}
	}

	for _, id := range plan.Projections {
		delete(store.state.Projections, id)
	}
	for _, key := range plan.Lines {
		delete(store.state.ProjectionLines, key)
	}
	for deviceKey, dev := range store.state.DeviceLines {
		if _, ok := removed[strings.TrimSpace(dev.LastDownloadedProjection)]; ok {
			dev.LastDownloadedProjection = ""
			store.state.DeviceLines[deviceKey] = dev
		}
	}
	for importID, artifact := range store.state.Imports {
		if _, ok := removed[strings.TrimSpace(artifact.BaselineProjection)]; ok {
			artifact.BaselineProjection = ""
			store.state.Imports[importID] = artifact
		}
	}
	if err := store.persistLocked(); err != nil {
		return plan, fmt.Errorf("persist the retired lines: %w", err)
	}

	records := make(map[string]saveRecord)
	for _, record := range a.snapshotSaveRecords() {
		records[strings.TrimSpace(record.Summary.ID)] = record
	}
	for _, id := range plan.SaveRecords {
		record, ok := records[id]
		if !ok {
			continue
		}
		if record.dirPath != "" {
			if err := os.RemoveAll(record.dirPath); err != nil {
				return plan, fmt.Errorf("remove save record %s: %w", id, err)
			}
			cleanupEmptyParents(filepath.Dir(record.dirPath), a.saveStore.root)
		}
	}
	for _, id := range plan.Projections {
		if err := os.RemoveAll(filepath.Join(store.root, "projections", id)); err != nil {
			return plan, fmt.Errorf("remove projection %s: %w", id, err)
		}
	}
	if len(plan.SaveRecords) > 0 {
		if err := a.reloadSavesFromDisk(); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

// cleanupPlayStationLineHistory is the one-shot entry point: report, then act
// only when the caller says so.
func (a *app) cleanupPlayStationLineHistory(systemSlug string, dryRun bool) (psLineRetirement, error) {
	store := a.playStationSyncStore()
	records := make(map[string]saveRecord)
	for _, record := range a.snapshotSaveRecords() {
		records[strings.TrimSpace(record.Summary.ID)] = record
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	plan := store.planPlayStationLineRetirement(systemSlug, records)
	if dryRun {
		return plan, nil
	}
	return a.applyPlayStationLineRetirement(store, plan)
}
