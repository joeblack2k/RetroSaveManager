package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

type playStationProjectionRebuildOptions struct {
	DryRun bool
}

type playStationProjectionRebuildResult struct {
	GroupsScanned      int      `json:"groupsScanned"`
	ProjectionRecords  int      `json:"projectionRecords"`
	ProjectionLineKeys []string `json:"projectionLineKeys,omitempty"`
}

type playStationProjectionGroup struct {
	Key        string
	SystemSlug string
	CardSlot   string
	Template   saveCreateInput
	Built      []psBuiltProjection
}

func runPlayStationProjectionRebuild(args []string) error {
	fs := flag.NewFlagSet("rebuild-playstation-projections", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "inspect projection lines without rewriting artifacts")
	if err := fs.Parse(args); err != nil {
		return err
	}

	a := newApp()
	if err := a.initSaveStore(); err != nil {
		return err
	}

	result, err := a.rebuildPlayStationProjections(playStationProjectionRebuildOptions{
		DryRun: *dryRun,
	})
	if err != nil {
		return err
	}

	log.Printf(
		"playstation projection rebuild complete: groups=%d projection_records=%d dry_run=%v",
		result.GroupsScanned,
		result.ProjectionRecords,
		*dryRun,
	)
	for _, key := range result.ProjectionLineKeys {
		log.Printf("reprojected playstation line group: %s", key)
	}
	return nil
}

func (a *app) rebuildPlayStationProjections(options playStationProjectionRebuildOptions) (playStationProjectionRebuildResult, error) {
	result := playStationProjectionRebuildResult{}
	store := a.playStationSyncStore()
	if store == nil {
		return result, fmt.Errorf("playstation store is not initialized")
	}

	templateByGroup := a.currentPlayStationProjectionTemplates()

	store.mu.Lock()

	groupsByKey := map[string]playStationProjectionGroup{}
	for _, line := range store.state.ProjectionLines {
		systemSlug := canonicalSegment(line.SystemSlug, "")
		cardSlot := strings.TrimSpace(line.CardSlot)
		if systemSlug == "" || cardSlot == "" {
			continue
		}
		groupKey := systemSlug + "::" + canonicalSegment(cardSlot, "card-slot")
		group := groupsByKey[groupKey]
		group.Key = groupKey
		group.SystemSlug = systemSlug
		group.CardSlot = cardSlot
		if template, ok := templateByGroup[groupKey]; ok {
			group.Template = template
		}
		groupsByKey[groupKey] = group
	}

	groupKeys := make([]string, 0, len(groupsByKey))
	for key := range groupsByKey {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	result.GroupsScanned = len(groupKeys)
	result.ProjectionLineKeys = append(result.ProjectionLineKeys, groupKeys...)

	if options.DryRun {
		store.mu.Unlock()
		return result, nil
	}

	groupPlans := make([]playStationProjectionGroup, 0, len(groupKeys))
	for _, key := range groupKeys {
		group := groupsByKey[key]
		built, err := store.rebuildProjectionLinesLocked(group.SystemSlug, group.CardSlot, "", "rebuild:"+group.Key, group.CardSlot)
		if err != nil {
			store.mu.Unlock()
			return result, err
		}
		group.Built = built
		groupPlans = append(groupPlans, group)
		result.ProjectionRecords += len(built)
	}
	if err := store.persistLocked(); err != nil {
		store.mu.Unlock()
		return result, err
	}
	store.mu.Unlock()

	now := time.Now().UTC()
	for _, group := range groupPlans {
		template := group.Template
		template.CreatedAt = now
		if _, err := a.materializePlayStationProjections(template, group.Built); err != nil {
			return result, err
		}
	}

	return result, nil
}

func (a *app) currentPlayStationProjectionTemplates() map[string]saveCreateInput {
	templates := map[string]saveCreateInput{}
	for _, record := range a.snapshotSaveRecords() {
		runtimeProfile, cardSlot, _, ok := playStationProjectionInfoFromRecord(record)
		if !ok {
			continue
		}
		groupKey := canonicalSegment(saveRecordSystemSlug(record), "") + "::" + canonicalSegment(cardSlot, "card-slot")
		current, exists := templates[groupKey]
		if !exists || record.Summary.CreatedAt.After(current.CreatedAt) {
			template := a.playStationTemplateInputFromSummary(record.Summary)
			template.CreatedAt = record.Summary.CreatedAt
			template.Metadata = record.Summary.Metadata
			if template.CardSlot == "" {
				template.CardSlot = cardSlot
			}
			if template.RuntimeProfile == "" {
				template.RuntimeProfile = runtimeProfile
			}
			templates[groupKey] = template
		}
	}
	return templates
}
