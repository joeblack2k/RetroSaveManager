package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"
)

type playStationLogicalCheatTarget struct {
	Context  playStationLogicalContext
	Revision psLogicalSaveRevision
	Summary  saveSummary
	Payload  []byte
}

type ps2ArchiveEntry struct {
	file *zip.File
	name string
	dir  bool
}

func (a *app) playStationLogicalCheatTarget(sourceRecord saveRecord, logicalKey string) (playStationLogicalCheatTarget, error) {
	store := a.playStationSyncStore()
	if store == nil {
		return playStationLogicalCheatTarget{}, fmt.Errorf("playstation store is not initialized")
	}
	ctx, err := store.logicalContextForSaveRecord(sourceRecord.Summary.ID, logicalKey)
	if err != nil {
		return playStationLogicalCheatTarget{}, err
	}
	revision, ok := ctx.Logical.latestRevision()
	if !ok {
		return playStationLogicalCheatTarget{}, fmt.Errorf("playstation logical save has no revisions")
	}
	summary := buildPlayStationLogicalListSummary(ctx)
	summary = a.enrichPlayStationLogicalSummary(sourceRecord, ctx, summary, revision)
	payload, err := buildPlayStationLogicalPayload(ctx, revision)
	if err != nil {
		return playStationLogicalCheatTarget{}, err
	}
	return playStationLogicalCheatTarget{Context: ctx, Revision: revision, Summary: summary, Payload: payload}, nil
}

func (a *app) enrichPlayStationLogicalHistory(sourceRecord saveRecord, history playStationLogicalHistory) playStationLogicalHistory {
	store := a.playStationSyncStore()
	if store == nil {
		return history
	}
	for i := range history.Versions {
		logicalKey := strings.TrimSpace(history.Versions[i].LogicalKey)
		if logicalKey == "" {
			logicalKey = logicalKeyFromSummaryMetadata(history.Versions[i])
		}
		if logicalKey == "" {
			continue
		}
		ctx, err := store.logicalContextForSaveRecord(sourceRecord.Summary.ID, logicalKey)
		if err != nil {
			continue
		}
		revision, ok := resolvePlayStationLogicalRevision(ctx.Logical, history.Versions[i].ID)
		if !ok {
			continue
		}
		history.Versions[i] = a.enrichPlayStationLogicalSummary(sourceRecord, ctx, history.Versions[i], revision)
	}
	return history
}

func (a *app) enrichPlayStationLogicalSummary(sourceRecord saveRecord, ctx playStationLogicalContext, summary saveSummary, revision psLogicalSaveRevision) saveSummary {
	payload, err := buildPlayStationLogicalPayload(ctx, revision)
	if err != nil {
		return summary
	}
	baseInspection := playStationLogicalBaseInspection(ctx, revision, len(payload))
	summary.Inspection = baseInspection
	if modules := a.moduleService(); modules != nil {
		if inspection, ok := modules.inspectSave(saveCreateInput{
			Filename:     summary.Filename,
			Payload:      payload,
			Format:       summary.Format,
			SystemSlug:   summary.SystemSlug,
			DisplayTitle: firstNonEmpty(summary.DisplayTitle, summary.Game.DisplayTitle, summary.Game.Name),
			Game:         summary.Game,
			SlotName:     summary.CardSlot,
			Metadata:     summary.Metadata,
		}, baseInspection); ok {
			summary.Inspection = inspection
		}
	}
	if cheats := a.cheatService(); cheats != nil {
		summary.Cheats = cheats.capabilityForPayload(sourceRecord, summary, payload, true)
	}
	return summary
}

func buildPlayStationLogicalPayload(ctx playStationLogicalContext, revision psLogicalSaveRevision) ([]byte, error) {
	_, _, payload, err := buildPlayStationLogicalDownload(playStationLogicalContext{
		Projection: ctx.Projection,
		Logical:    clonePlayStationLogicalWithRevision(ctx.Logical, revision),
	}, revision.ID)
	return payload, err
}

func playStationLogicalBaseInspection(ctx playStationLogicalContext, revision psLogicalSaveRevision, payloadSize int) *saveInspection {
	level := saveParserLevelStructural
	parserID := "playstation-logical-save"
	if canonicalSegment(ctx.Logical.SystemSlug, "") == "ps2" {
		parserID = "ps2-logical-save"
	}
	return &saveInspection{
		ParserLevel:        level,
		ParserID:           parserID,
		ValidatedSystem:    ctx.Logical.SystemSlug,
		ValidatedGameID:    firstNonEmpty(ctx.Logical.ProductCode, ctx.Logical.Key),
		ValidatedGameTitle: ctx.Logical.DisplayTitle,
		TrustLevel:         "structure-verified",
		Evidence: []string{
			"playstation logical save extracted from projection",
			"logical revision=" + revision.ID,
		},
		PayloadSizeBytes: payloadSize,
	}
}

func (a *app) promotePlayStationLogicalCheatPayload(sourceRecord saveRecord, logicalKey string, patchedPayload []byte, metadata any) (saveRecord, error) {
	runtimeProfile, cardSlot, projectionID, ok := playStationProjectionInfoFromRecord(sourceRecord)
	if !ok {
		return saveRecord{}, fmt.Errorf("save is not a playstation projection")
	}
	store := a.playStationSyncStore()
	if store == nil {
		return saveRecord{}, fmt.Errorf("playstation store is not initialized")
	}
	now := time.Now().UTC()

	store.mu.Lock()
	projection, ok := store.state.Projections[strings.TrimSpace(projectionID)]
	if !ok {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("playstation projection not found")
	}
	if !projectionManifestContainsLogicalKey(projection.Manifest, logicalKey) {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("playstation logical save not found in projection")
	}
	logical, ok := store.state.LogicalSaves[strings.TrimSpace(logicalKey)]
	if !ok {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("playstation logical save not found")
	}
	if canonicalSegment(logical.SystemSlug, "") != "ps2" {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("logical cheat apply currently supports PS2 logical saves")
	}
	latest, ok := logical.latestRevision()
	if !ok || latest.PS2 == nil {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("playstation logical save has no PS2 payload")
	}
	patchedPS2, err := parsePS2LogicalSaveArchive(*latest.PS2, patchedPayload)
	if err != nil {
		store.mu.Unlock()
		return saveRecord{}, err
	}
	memoryEntry := latest.MemoryEntry
	memoryEntry.SizeBytes = ps2LogicalRevisionDataSize(patchedPS2)
	sha := hashPS2LogicalRevision(logical, memoryEntry, patchedPS2)
	revisionID := "ps-rev-" + now.Format("20060102150405.000000000") + "-" + hash12("cheat:"+logical.Key+":"+sha+":"+now.Format(time.RFC3339Nano))
	revision := psLogicalSaveRevision{
		ID:          revisionID,
		ImportID:    "cheat:" + sourceRecord.Summary.ID,
		CreatedAt:   now,
		SHA256:      sha,
		MemoryEntry: memoryEntry,
		PS2:         patchedPS2,
	}
	logical.Revisions = append(logical.Revisions, revision)
	logical.LatestRevisionID = revision.ID
	store.state.LogicalSaves[logical.Key] = logical
	built, err := store.rebuildProjectionLinesLocked(projection.SystemSlug, projection.CardSlot, runtimeProfile, "cheat:"+logical.Key, cardSlot)
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
	template.Metadata = metadata
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

func parsePS2LogicalSaveArchive(previous ps2LogicalRevision, payload []byte) (*ps2LogicalRevision, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, fmt.Errorf("patched PS2 logical payload must be a zip archive: %w", err)
	}
	if len(reader.File) == 0 {
		return nil, fmt.Errorf("patched PS2 logical archive is empty")
	}

	entries := make([]ps2ArchiveEntry, 0, len(reader.File))
	for _, file := range reader.File {
		name, err := cleanPS2ArchivePath(file.Name)
		if err != nil {
			return nil, err
		}
		if name == "" {
			continue
		}
		entries = append(entries, ps2ArchiveEntry{file: file, name: name, dir: file.FileInfo().IsDir()})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("patched PS2 logical archive has no files")
	}

	stripRoot := commonPS2ArchiveRoot(entries)
	previousModes := map[string]uint16{}
	for _, node := range previous.Nodes {
		clean, err := cleanPS2ArchivePath(node.Path)
		if err == nil && clean != "" {
			previousModes[clean] = node.Mode
		}
	}

	nodesByPath := map[string]ps2LogicalFSNode{}
	fileCount := 0
	for _, entry := range entries {
		rel := trimPS2ArchiveRoot(entry.name, stripRoot)
		if rel == "" {
			continue
		}
		addPS2ImplicitDirs(nodesByPath, rel, previousModes)
		if entry.dir {
			nodesByPath[rel] = ps2LogicalFSNode{Path: rel, Mode: firstNonZeroUint16(previousModes[rel], ps2ModeDirectory), Directory: true}
			continue
		}
		if entry.file.UncompressedSize64 > 8<<20 {
			return nil, fmt.Errorf("patched PS2 logical file %q is too large", rel)
		}
		data, err := readZipFile(entry.file)
		if err != nil {
			return nil, err
		}
		mode := firstNonZeroUint16(previousModes[rel], ps2ModeFileRegular)
		nodesByPath[rel] = ps2LogicalFSNode{Path: rel, Mode: mode, Data: data}
		fileCount++
	}
	if fileCount == 0 {
		return nil, fmt.Errorf("patched PS2 logical archive has no files")
	}

	nodes := make([]ps2LogicalFSNode, 0, len(nodesByPath))
	for _, node := range nodesByPath {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Directory == nodes[j].Directory {
			return nodes[i].Path < nodes[j].Path
		}
		return nodes[i].Directory
	})
	return &ps2LogicalRevision{DirectoryName: previous.DirectoryName, Nodes: nodes}, nil
}

func cleanPS2ArchivePath(name string) (string, error) {
	clean := strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
	if clean == "" {
		return "", nil
	}
	if path.IsAbs(clean) {
		return "", fmt.Errorf("patched PS2 logical archive contains absolute path %q", name)
	}
	clean = path.Clean(clean)
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("patched PS2 logical archive contains unsafe path %q", name)
	}
	return clean, nil
}

func commonPS2ArchiveRoot(entries []ps2ArchiveEntry) string {
	root := ""
	for _, entry := range entries {
		parts := strings.Split(entry.name, "/")
		if len(parts) == 1 && entry.dir {
			continue
		}
		if len(parts) < 2 {
			return ""
		}
		if root == "" {
			root = parts[0]
			continue
		}
		if root != parts[0] {
			return ""
		}
	}
	return root
}

func trimPS2ArchiveRoot(name, root string) string {
	if root == "" {
		return name
	}
	if name == root {
		return ""
	}
	return strings.TrimPrefix(name, root+"/")
}

func addPS2ImplicitDirs(nodes map[string]ps2LogicalFSNode, rel string, previousModes map[string]uint16) {
	parts := strings.Split(rel, "/")
	if len(parts) <= 1 {
		return
	}
	current := ""
	for _, part := range parts[:len(parts)-1] {
		if current == "" {
			current = part
		} else {
			current += "/" + part
		}
		if _, ok := nodes[current]; !ok {
			nodes[current] = ps2LogicalFSNode{Path: current, Mode: firstNonZeroUint16(previousModes[current], ps2ModeDirectory), Directory: true}
		}
	}
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, int64((8<<20)+1)))
	if err != nil {
		return nil, err
	}
	if len(data) > 8<<20 {
		return nil, fmt.Errorf("patched PS2 logical file %q is too large", file.Name)
	}
	return data, nil
}

func firstNonZeroUint16(values ...uint16) uint16 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func ps2LogicalRevisionDataSize(revision *ps2LogicalRevision) int {
	if revision == nil {
		return 0
	}
	total := 0
	for _, node := range revision.Nodes {
		if !node.Directory {
			total += len(node.Data)
		}
	}
	return total
}

func hashPS2LogicalRevision(logical psLogicalSave, memoryEntry memoryCardEntry, revision *ps2LogicalRevision) string {
	return hashPS2LogicalEntry(psExtractedEntry{
		ProductCode:  logical.ProductCode,
		DisplayTitle: firstNonEmpty(logical.DisplayTitle, memoryEntry.Title),
		RegionCode:   logical.RegionCode,
		MemoryEntry:  memoryEntry,
		PS2:          revision,
	})
}

func logicalKeyFromSummaryMetadata(summary saveSummary) string {
	metadata, ok := summary.Metadata.(map[string]any)
	if !ok {
		return ""
	}
	logical, ok := metadata["playstationLogical"].(map[string]any)
	if !ok {
		return ""
	}
	value, _ := logical["logicalKey"].(string)
	return strings.TrimSpace(value)
}
