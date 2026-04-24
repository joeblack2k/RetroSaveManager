package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/clktmr/n64/drivers/controller/pakfs"
)

const (
	n64ControllerPakStateDirname = "_rsm/n64-controller-pak"
	n64ControllerPakStateFile    = "state.json"
)

type n64ControllerPakStore struct {
	mu        sync.Mutex
	root      string
	statePath string
	state     n64ControllerPakState
}

type n64ControllerPakState struct {
	Imports         map[string]n64ControllerPakImportArtifact `json:"imports,omitempty"`
	LogicalSaves    map[string]n64ControllerPakLogicalSave    `json:"logicalSaves,omitempty"`
	ProjectionLines map[string]n64ControllerPakProjectionLine `json:"projectionLines,omitempty"`
	Projections     map[string]n64ControllerPakProjection     `json:"projections,omitempty"`
	Tombstones      map[string]n64ControllerPakTombstone      `json:"tombstones,omitempty"`
}

type n64ControllerPakImportArtifact struct {
	ID                string                          `json:"id"`
	SyncLineKey       string                          `json:"syncLineKey"`
	RuntimeProfile    string                          `json:"runtimeProfile"`
	Filename          string                          `json:"filename"`
	SHA256            string                          `json:"sha256"`
	PayloadPath       string                          `json:"payloadPath"`
	CreatedAt         time.Time                       `json:"createdAt"`
	ProjectionLineKey string                          `json:"projectionLineKey"`
	Manifest          []n64ControllerPakManifestEntry `json:"manifest,omitempty"`
}

type n64ControllerPakManifestEntry struct {
	LogicalKey    string `json:"logicalKey"`
	RevisionID    string `json:"revisionId"`
	GameCode      string `json:"gameCode,omitempty"`
	PublisherCode string `json:"publisherCode,omitempty"`
	NoteName      string `json:"noteName,omitempty"`
}

type n64ControllerPakLogicalSave struct {
	Key              string                            `json:"key"`
	SyncLineKey      string                            `json:"syncLineKey"`
	GameCode         string                            `json:"gameCode,omitempty"`
	PublisherCode    string                            `json:"publisherCode,omitempty"`
	NoteName         string                            `json:"noteName,omitempty"`
	CatalogGameID    string                            `json:"catalogGameId,omitempty"`
	CatalogTitle     string                            `json:"catalogTitle,omitempty"`
	CatalogSlug      string                            `json:"catalogSlug,omitempty"`
	RegionCode       string                            `json:"regionCode,omitempty"`
	Revisions        []n64ControllerPakLogicalRevision `json:"revisions,omitempty"`
	LatestRevisionID string                            `json:"latestRevisionId,omitempty"`
}

type n64ControllerPakLogicalRevision struct {
	ID          string             `json:"id"`
	ImportID    string             `json:"importId"`
	CreatedAt   time.Time          `json:"createdAt"`
	SHA256      string             `json:"sha256"`
	PayloadPath string             `json:"payloadPath"`
	Entry       controllerPakEntry `json:"entry"`
}

type n64ControllerPakProjectionLine struct {
	Key                string    `json:"key"`
	SyncLineKey        string    `json:"syncLineKey"`
	RuntimeProfile     string    `json:"runtimeProfile"`
	LatestProjectionID string    `json:"latestProjectionId,omitempty"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type n64ControllerPakProjection struct {
	ID                string                          `json:"id"`
	ProjectionLineKey string                          `json:"projectionLineKey"`
	SyncLineKey       string                          `json:"syncLineKey"`
	RuntimeProfile    string                          `json:"runtimeProfile"`
	Filename          string                          `json:"filename"`
	SHA256            string                          `json:"sha256"`
	PayloadPath       string                          `json:"payloadPath"`
	CreatedAt         time.Time                       `json:"createdAt"`
	SaveRecordID      string                          `json:"saveRecordId,omitempty"`
	SourceImportID    string                          `json:"sourceImportId,omitempty"`
	Manifest          []n64ControllerPakManifestEntry `json:"manifest,omitempty"`
}

type n64ControllerPakTombstone struct {
	LogicalKey  string    `json:"logicalKey"`
	SyncLineKey string    `json:"syncLineKey"`
	CreatedAt   time.Time `json:"createdAt"`
	Reason      string    `json:"reason,omitempty"`
}

type n64ControllerPakImportRequest struct {
	CanonicalPayload []byte
	Filename         string
	RuntimeProfile   string
	ROMSHA1          string
	SlotName         string
	CreatedAt        time.Time
}

type n64ControllerPakBuiltProjection struct {
	ProjectionID      string
	ProjectionLineKey string
	SyncLineKey       string
	RuntimeProfile    string
	Filename          string
	Payload           []byte
	SourceImportID    string
}

type n64ControllerPakImportResult struct {
	PrimaryProjectionLineKey string
	Built                    []n64ControllerPakBuiltProjection
}

type n64ControllerPakLogicalContext struct {
	Projection n64ControllerPakProjection
	Logical    n64ControllerPakLogicalSave
}

type n64ControllerPakHistory struct {
	Game         game
	DisplayTitle string
	SystemSlug   string
	Summary      map[string]any
	Versions     []saveSummary
}

type n64ControllerPakExtractedEntry struct {
	LogicalKey   string
	SyncLineKey  string
	RevisionID   string
	SHA256       string
	Payload      []byte
	DisplayTitle string
	RegionCode   string
	Catalog      *n64ControllerPakCatalogEntry
	Entry        controllerPakEntry
}

type n64ControllerPakCatalogEntry struct {
	GameID     string
	Title      string
	Slug       string
	RegionCode string
}

type n64ControllerPakMemBuffer []byte

func (b n64ControllerPakMemBuffer) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fs.ErrInvalid
	}
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (b n64ControllerPakMemBuffer) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fs.ErrInvalid
	}
	if off >= int64(len(b)) {
		return 0, io.ErrShortWrite
	}
	n := copy(b[off:], p)
	if n < len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

var n64ControllerPakCatalog = map[string]n64ControllerPakCatalogEntry{
	"NKTE::01": {GameID: "n64/mario-kart-64", Title: "Mario Kart 64", Slug: "mario-kart-64", RegionCode: regionUS},
	"CZLE::01": {GameID: "n64/ocarina-of-time", Title: "The Legend of Zelda: Ocarina of Time", Slug: "the-legend-of-zelda-ocarina-of-time", RegionCode: regionUS},
	"NPOE::01": {GameID: "n64/paper-mario", Title: "Paper Mario", Slug: "paper-mario", RegionCode: regionUS},
	"NMVE::01": {GameID: "n64/majoras-mask", Title: "The Legend of Zelda: Majora's Mask", Slug: "the-legend-of-zelda-majoras-mask", RegionCode: regionUS},
}

func newN64ControllerPakStore(saveRoot string) (*n64ControllerPakStore, error) {
	root, err := safeJoinUnderRoot(saveRoot, n64ControllerPakStateDirname)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create n64 controller pak root: %w", err)
	}
	store := &n64ControllerPakStore{
		root:      root,
		statePath: filepath.Join(root, n64ControllerPakStateFile),
		state:     emptyN64ControllerPakState(),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func emptyN64ControllerPakState() n64ControllerPakState {
	return n64ControllerPakState{
		Imports:         map[string]n64ControllerPakImportArtifact{},
		LogicalSaves:    map[string]n64ControllerPakLogicalSave{},
		ProjectionLines: map[string]n64ControllerPakProjectionLine{},
		Projections:     map[string]n64ControllerPakProjection{},
		Tombstones:      map[string]n64ControllerPakTombstone{},
	}
}

func (s *n64ControllerPakStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.state = emptyN64ControllerPakState()
			return nil
		}
		return fmt.Errorf("read n64 controller pak state: %w", err)
	}
	state := emptyN64ControllerPakState()
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode n64 controller pak state: %w", err)
	}
	s.state = state
	return nil
}

func (s *n64ControllerPakStore) persistLocked() error {
	payload, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode n64 controller pak state: %w", err)
	}
	if err := writeFileAtomic(s.statePath, payload, 0o644); err != nil {
		return fmt.Errorf("write n64 controller pak state: %w", err)
	}
	return nil
}

func n64ControllerPakSyncLineKey(romSHA1, slotName string) string {
	return strings.ToLower(strings.TrimSpace(romSHA1)) + "::" + normalizedSlot(slotName)
}

func n64ControllerPakProjectionLineKey(syncLineKey, runtimeProfile string) string {
	return strings.ToLower(strings.TrimSpace(syncLineKey)) + "::" + strings.ToLower(strings.TrimSpace(runtimeProfile))
}

func n64ControllerPakTombstoneKey(syncLineKey, logicalKey string) string {
	return strings.ToLower(strings.TrimSpace(syncLineKey)) + "::" + strings.ToLower(strings.TrimSpace(logicalKey))
}

func n64ControllerPakProjectionConflictKey(syncLineKey, runtimeProfile string) string {
	return "n64-controller-pak-projection::" + n64ControllerPakProjectionLineKey(syncLineKey, runtimeProfile)
}

func n64CatalogKey(gameCode, publisherCode string) string {
	return strings.ToUpper(strings.TrimSpace(gameCode)) + "::" + strings.ToUpper(strings.TrimSpace(publisherCode))
}

func lookupN64ControllerPakCatalog(gameCode, publisherCode string) *n64ControllerPakCatalogEntry {
	if entry, ok := n64ControllerPakCatalog[n64CatalogKey(gameCode, publisherCode)]; ok {
		copy := entry
		return &copy
	}
	return nil
}

func n64ControllerPakRegionFromGameCode(gameCode string) string {
	gameCode = strings.ToUpper(strings.TrimSpace(gameCode))
	if len(gameCode) < 4 {
		return regionUnknown
	}
	switch gameCode[len(gameCode)-1] {
	case 'E', 'U':
		return regionUS
	case 'J':
		return regionJP
	case 'P', 'D', 'F', 'I', 'S', 'X', 'Y':
		return regionEU
	default:
		return regionUnknown
	}
}

func normalizeN64ControllerPakDisplayTitle(noteName string, catalog *n64ControllerPakCatalogEntry) string {
	if catalog != nil && strings.TrimSpace(catalog.Title) != "" {
		return strings.TrimSpace(catalog.Title)
	}
	name := strings.TrimSpace(noteName)
	if name == "" {
		return "Unknown Game"
	}
	return name
}

func portableN64ControllerPakLogicalKey(gameCode, publisherCode, noteName string) string {
	return "n64::controller-pak::" + strings.ToUpper(strings.TrimSpace(gameCode)) + "::" + strings.ToUpper(strings.TrimSpace(publisherCode)) + "::" + canonicalTrackTitleKey(noteName)
}

func newN64ControllerPakProjectionMetadata(syncLineKey, runtimeProfile, projectionID, sourceImportID string) map[string]any {
	return map[string]any{
		"rsm": map[string]any{
			"n64ControllerPakProjection": map[string]any{
				"syncLineKey":    syncLineKey,
				"runtimeProfile": runtimeProfile,
				"projectionId":   projectionID,
				"sourceImportId": sourceImportID,
			},
		},
	}
}

func n64ControllerPakProjectionInfoFromRecord(record saveRecord) (syncLineKey, runtimeProfile, projectionID string, ok bool) {
	if canonicalSegment(saveRecordSystemSlug(record), "") != "n64" || strings.TrimSpace(record.Summary.MediaType) != "controller-pak" {
		return "", "", "", false
	}
	metadata, ok := record.Summary.Metadata.(map[string]any)
	if !ok {
		return "", "", "", false
	}
	rsm, ok := metadata["rsm"].(map[string]any)
	if !ok {
		return "", "", "", false
	}
	projection, ok := rsm["n64ControllerPakProjection"].(map[string]any)
	if !ok {
		return "", "", "", false
	}
	syncLineKey, _ = projection["syncLineKey"].(string)
	runtimeProfile, _ = projection["runtimeProfile"].(string)
	projectionID, _ = projection["projectionId"].(string)
	if strings.TrimSpace(syncLineKey) == "" || strings.TrimSpace(runtimeProfile) == "" || strings.TrimSpace(projectionID) == "" {
		return "", "", "", false
	}
	return strings.TrimSpace(syncLineKey), strings.TrimSpace(runtimeProfile), strings.TrimSpace(projectionID), true
}

func (s *n64ControllerPakStore) writeArtifactPayload(kind string, payload []byte, suffix string) (string, error) {
	name := kind + "-" + time.Now().UTC().Format("20060102150405.000000000") + "-" + hash12(string(payload)) + suffix
	path, err := safeJoinUnderRoot(s.root, kind, name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := writeFileAtomic(path, payload, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *n64ControllerPakStore) extractLogicalEntries(syncLineKey string, payload []byte) ([]n64ControllerPakExtractedEntry, error) {
	mem := n64ControllerPakMemBuffer(append([]byte(nil), payload...))
	fsys, err := pakfs.Read(mem)
	if err != nil {
		return nil, fmt.Errorf("parse controller pak filesystem: %w", err)
	}
	root := fsys.ReadDirRoot()
	out := make([]n64ControllerPakExtractedEntry, 0, len(root))
	for idx, dirEntry := range root {
		opened, err := fsys.Open(dirEntry.Name())
		if err != nil {
			return nil, fmt.Errorf("open controller pak entry %q: %w", dirEntry.Name(), err)
		}
		file, ok := opened.(*pakfs.File)
		if !ok {
			return nil, fmt.Errorf("controller pak entry %q is not a file", dirEntry.Name())
		}
		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			return nil, fmt.Errorf("read controller pak entry %q: %w", dirEntry.Name(), err)
		}
		gameCodeRaw := file.GameCode()
		publisherCodeRaw := file.CompanyCode()
		gameCode := string(bytes.Trim(gameCodeRaw[:], "\x00 "))
		publisherCode := string(bytes.Trim(publisherCodeRaw[:], "\x00 "))
		noteName := strings.TrimSpace(file.Name())
		catalog := lookupN64ControllerPakCatalog(gameCode, publisherCode)
		regionCode := n64ControllerPakRegionFromGameCode(gameCode)
		if catalog != nil && normalizeRegionCode(catalog.RegionCode) != regionUnknown {
			regionCode = normalizeRegionCode(catalog.RegionCode)
		}
		entry := controllerPakEntry{
			LogicalKey:     portableN64ControllerPakLogicalKey(strings.ToUpper(strings.TrimSpace(gameCode)), strings.ToUpper(strings.TrimSpace(publisherCode)), noteName),
			GameCode:       strings.ToUpper(strings.TrimSpace(gameCode)),
			PublisherCode:  strings.ToUpper(strings.TrimSpace(publisherCode)),
			NoteName:       noteName,
			EntryIndex:     idx + 1,
			PageCount:      int((file.Size() + 255) / 256),
			BlockUsage:     int((file.Size() + 255) / 256),
			StructureValid: true,
			ChecksumValid:  boolPtr(true),
			SizeBytes:      len(data),
		}
		payloadSum := sha256.Sum256(data)
		revisionID := "n64-cpk-revision-" + time.Now().UTC().Format("20060102150405.000000000") + "-" + hash12(entry.GameCode+"::"+entry.PublisherCode+"::"+noteName+"::"+hex.EncodeToString(payloadSum[:]))
		out = append(out, n64ControllerPakExtractedEntry{
			LogicalKey:   portableN64ControllerPakLogicalKey(entry.GameCode, entry.PublisherCode, noteName),
			SyncLineKey:  syncLineKey,
			RevisionID:   revisionID,
			SHA256:       hex.EncodeToString(payloadSum[:]),
			Payload:      data,
			DisplayTitle: normalizeN64ControllerPakDisplayTitle(noteName, catalog),
			RegionCode:   regionCode,
			Catalog:      catalog,
			Entry:        entry,
		})
	}
	return out, nil
}

func countN64ControllerPakEntries(payload []byte) (int, error) {
	mem := n64ControllerPakMemBuffer(append([]byte(nil), payload...))
	fsys, err := pakfs.Read(mem)
	if err != nil {
		return 0, fmt.Errorf("parse controller pak filesystem: %w", err)
	}
	return len(fsys.ReadDirRoot()), nil
}

func (logical n64ControllerPakLogicalSave) latestRevision() (n64ControllerPakLogicalRevision, bool) {
	for _, revision := range logical.Revisions {
		if strings.TrimSpace(revision.ID) == strings.TrimSpace(logical.LatestRevisionID) {
			return revision, true
		}
	}
	if len(logical.Revisions) == 0 {
		return n64ControllerPakLogicalRevision{}, false
	}
	latest := logical.Revisions[0]
	for _, revision := range logical.Revisions[1:] {
		if revision.CreatedAt.After(latest.CreatedAt) {
			latest = revision
		}
	}
	return latest, true
}

func (logical n64ControllerPakLogicalSave) revisionByID(id string) (n64ControllerPakLogicalRevision, bool) {
	target := strings.TrimSpace(id)
	for _, revision := range logical.Revisions {
		if strings.TrimSpace(revision.ID) == target {
			return revision, true
		}
	}
	return n64ControllerPakLogicalRevision{}, false
}

func (s *n64ControllerPakStore) currentProjectionForLogicalLocked(logical n64ControllerPakLogicalSave) (n64ControllerPakProjection, bool) {
	var best n64ControllerPakProjection
	found := false
	for _, line := range s.state.ProjectionLines {
		if strings.TrimSpace(line.SyncLineKey) != strings.TrimSpace(logical.SyncLineKey) {
			continue
		}
		projection, ok := s.state.Projections[strings.TrimSpace(line.LatestProjectionID)]
		if !ok {
			continue
		}
		if projectionManifestContainsN64LogicalKey(projection.Manifest, logical.Key) {
			if !found || projection.CreatedAt.After(best.CreatedAt) || (projection.CreatedAt.Equal(best.CreatedAt) && projection.RuntimeProfile < best.RuntimeProfile) {
				best = projection
				found = true
			}
		}
	}
	return best, found
}

func projectionManifestContainsN64LogicalKey(manifest []n64ControllerPakManifestEntry, logicalKey string) bool {
	target := strings.TrimSpace(logicalKey)
	for _, entry := range manifest {
		if strings.TrimSpace(entry.LogicalKey) == target {
			return true
		}
	}
	return false
}

func (s *n64ControllerPakStore) listLogicalContexts() []n64ControllerPakLogicalContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	contexts := make([]n64ControllerPakLogicalContext, 0, len(s.state.LogicalSaves))
	for _, logical := range s.state.LogicalSaves {
		if _, ok := logical.latestRevision(); !ok {
			continue
		}
		if _, tombstoned := s.state.Tombstones[n64ControllerPakTombstoneKey(logical.SyncLineKey, logical.Key)]; tombstoned {
			continue
		}
		projection, ok := s.currentProjectionForLogicalLocked(logical)
		if !ok {
			continue
		}
		contexts = append(contexts, n64ControllerPakLogicalContext{Projection: projection, Logical: logical})
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

func buildN64ControllerPakLogicalListSummary(ctx n64ControllerPakLogicalContext) saveSummary {
	revision, _ := ctx.Logical.latestRevision()
	sys := supportedSystemFromSlug("n64")
	displayTitle := strings.TrimSpace(firstNonEmpty(ctx.Logical.CatalogTitle, revision.Entry.NoteName, ctx.Logical.NoteName, "Unknown Game"))
	filename := strings.TrimSpace(ctx.Projection.Filename)
	if filename == "" {
		filename = safeFilename(displayTitle + ".cpk")
	}
	summary := saveSummary{
		ID:                    ctx.Projection.SaveRecordID,
		Game:                  game{ID: deterministicGameID("n64-logical:" + ctx.Logical.Key), Name: displayTitle, DisplayTitle: displayTitle, RegionCode: ctx.Logical.RegionCode, RegionFlag: regionFlagFromCode(ctx.Logical.RegionCode), System: sys},
		DisplayTitle:          displayTitle,
		LogicalKey:            ctx.Logical.Key,
		SystemSlug:            "n64",
		RegionCode:            normalizeRegionCode(ctx.Logical.RegionCode),
		RegionFlag:            regionFlagFromCode(ctx.Logical.RegionCode),
		MediaType:             "controller-pak",
		ProjectionCapable:     boolPtr(true),
		SourceArtifactProfile: ctx.Projection.RuntimeProfile,
		RuntimeProfile:        ctx.Projection.RuntimeProfile,
		Filename:              filename,
		FileSize:              len(revision.PayloadPath),
		Format:                inferSaveFormat(filename),
		Version:               len(ctx.Logical.Revisions),
		SHA256:                revision.SHA256,
		CreatedAt:             revision.CreatedAt,
		Metadata: map[string]any{
			"rsm": map[string]any{
				"n64ControllerPakLogical": map[string]any{
					"syncLineKey": logicalSyncLineKey(ctx.Logical),
				},
			},
		},
		ControllerPakEntry: &revision.Entry,
		SaveCount:          len(ctx.Logical.Revisions),
		LatestSizeBytes:    revision.Entry.SizeBytes,
		TotalSizeBytes:     n64ControllerPakTotalRevisionBytes(ctx.Logical),
		LatestVersion:      len(ctx.Logical.Revisions),
	}
	if summary.FileSize == 0 {
		summary.FileSize = revision.Entry.SizeBytes
	}
	return summaryWithDownloadProfiles(summary)
}

func logicalSyncLineKey(logical n64ControllerPakLogicalSave) string { return logical.SyncLineKey }

func n64ControllerPakTotalRevisionBytes(logical n64ControllerPakLogicalSave) int {
	total := 0
	for _, revision := range logical.Revisions {
		total += revision.Entry.SizeBytes
	}
	return total
}

func buildN64ControllerPakHistory(ctx n64ControllerPakLogicalContext) n64ControllerPakHistory {
	revisions := append([]n64ControllerPakLogicalRevision(nil), ctx.Logical.Revisions...)
	sort.Slice(revisions, func(i, j int) bool {
		if revisions[i].CreatedAt.Equal(revisions[j].CreatedAt) {
			return revisions[i].ID > revisions[j].ID
		}
		return revisions[i].CreatedAt.After(revisions[j].CreatedAt)
	})
	displayTitle := strings.TrimSpace(firstNonEmpty(ctx.Logical.CatalogTitle, ctx.Logical.NoteName, "Unknown Game"))
	sys := supportedSystemFromSlug("n64")
	versions := make([]saveSummary, 0, len(revisions))
	totalBytes := 0
	for idx, revision := range revisions {
		totalBytes += revision.Entry.SizeBytes
		versions = append(versions, summaryWithDownloadProfiles(saveSummary{
			ID:                 revision.ID,
			Game:               game{ID: deterministicGameID("n64-logical:" + ctx.Logical.Key), Name: displayTitle, DisplayTitle: displayTitle, RegionCode: ctx.Logical.RegionCode, RegionFlag: regionFlagFromCode(ctx.Logical.RegionCode), System: sys},
			DisplayTitle:       displayTitle,
			LogicalKey:         ctx.Logical.Key,
			SystemSlug:         "n64",
			RegionCode:         normalizeRegionCode(ctx.Logical.RegionCode),
			RegionFlag:         regionFlagFromCode(ctx.Logical.RegionCode),
			MediaType:          "controller-pak",
			ProjectionCapable:  boolPtr(true),
			RuntimeProfile:     ctx.Projection.RuntimeProfile,
			Filename:           ctx.Projection.Filename,
			FileSize:           revision.Entry.SizeBytes,
			Format:             inferSaveFormat(ctx.Projection.Filename),
			Version:            len(revisions) - idx,
			SHA256:             revision.SHA256,
			CreatedAt:          revision.CreatedAt,
			ControllerPakEntry: &revision.Entry,
		}))
	}
	return n64ControllerPakHistory{
		Game:         game{ID: deterministicGameID("n64-logical:" + ctx.Logical.Key), Name: displayTitle, DisplayTitle: displayTitle, RegionCode: ctx.Logical.RegionCode, RegionFlag: regionFlagFromCode(ctx.Logical.RegionCode), System: sys},
		DisplayTitle: displayTitle,
		SystemSlug:   "n64",
		Summary: map[string]any{
			"displayTitle":    displayTitle,
			"system":          sys,
			"regionCode":      normalizeRegionCode(ctx.Logical.RegionCode),
			"regionFlag":      regionFlagFromCode(ctx.Logical.RegionCode),
			"saveCount":       len(versions),
			"totalSizeBytes":  totalBytes,
			"latestVersion":   len(versions),
			"latestCreatedAt": versions[0].CreatedAt,
		},
		Versions: versions,
	}
}

func (s *n64ControllerPakStore) logicalContextForSaveRecord(saveRecordID, logicalKey string) (n64ControllerPakLogicalContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	targetSaveID := strings.TrimSpace(saveRecordID)
	targetLogicalKey := strings.TrimSpace(logicalKey)
	if targetSaveID == "" || targetLogicalKey == "" {
		return n64ControllerPakLogicalContext{}, fmt.Errorf("n64 logical save requires saveId and logicalKey")
	}
	var projection n64ControllerPakProjection
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
		return n64ControllerPakLogicalContext{}, fmt.Errorf("n64 controller pak projection not found")
	}
	if !projectionManifestContainsN64LogicalKey(projection.Manifest, targetLogicalKey) {
		return n64ControllerPakLogicalContext{}, fmt.Errorf("n64 controller pak logical save not found in projection")
	}
	logical, ok := s.state.LogicalSaves[targetLogicalKey]
	if !ok {
		return n64ControllerPakLogicalContext{}, fmt.Errorf("n64 controller pak logical save not found")
	}
	return n64ControllerPakLogicalContext{Projection: projection, Logical: logical}, nil
}

func (s *n64ControllerPakStore) latestProjectionSaveRecord(syncLineKey, runtimeProfile string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	line, ok := s.state.ProjectionLines[n64ControllerPakProjectionLineKey(syncLineKey, runtimeProfile)]
	if !ok {
		return "", false
	}
	projection, ok := s.state.Projections[strings.TrimSpace(line.LatestProjectionID)]
	if !ok {
		return "", false
	}
	if strings.TrimSpace(projection.SaveRecordID) == "" {
		return "", false
	}
	return projection.SaveRecordID, true
}

func (s *n64ControllerPakStore) projectionBySaveRecordID(saveRecordID string) (n64ControllerPakProjection, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, projection := range s.state.Projections {
		if strings.TrimSpace(projection.SaveRecordID) == strings.TrimSpace(saveRecordID) {
			return projection, true
		}
	}
	return n64ControllerPakProjection{}, false
}

func (s *n64ControllerPakStore) attachProjectionSaveRecord(projectionID, saveRecordID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	projection, ok := s.state.Projections[strings.TrimSpace(projectionID)]
	if !ok {
		return fmt.Errorf("n64 controller pak projection not found")
	}
	projection.SaveRecordID = strings.TrimSpace(saveRecordID)
	s.state.Projections[projection.ID] = projection
	return s.persistLocked()
}

func buildCanonicalN64ControllerPak(entries []n64ControllerPakLogicalSave, revisions map[string]n64ControllerPakLogicalRevision) ([]byte, []n64ControllerPakManifestEntry, error) {
	buf := make(n64ControllerPakMemBuffer, n64RetroArchControllerPakSize)
	n64InitControllerPak(buf)
	fsys, err := pakfs.Read(buf)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	manifest := make([]n64ControllerPakManifestEntry, 0, len(entries))
	for _, logical := range entries {
		revision, ok := revisions[logical.Key]
		if !ok {
			continue
		}
		name := strings.TrimSpace(revision.Entry.NoteName)
		if name == "" {
			name = strings.TrimSpace(firstNonEmpty(logical.NoteName, logical.CatalogTitle, "SAVE"))
		}
		f, err := fsys.Create(name)
		if err != nil {
			return nil, nil, err
		}
		var gameCode [4]byte
		copy(gameCode[:], []byte(strings.ToUpper(strings.TrimSpace(logical.GameCode))))
		var publisher [2]byte
		copy(publisher[:], []byte(strings.ToUpper(strings.TrimSpace(logical.PublisherCode))))
		if err := f.SetGameCode(gameCode); err != nil {
			return nil, nil, err
		}
		if err := f.SetCompanyCode(publisher); err != nil {
			return nil, nil, err
		}
		payload, err := os.ReadFile(revision.PayloadPath)
		if err != nil {
			return nil, nil, err
		}
		if _, err := f.WriteAt(payload, 0); err != nil {
			return nil, nil, err
		}
		manifest = append(manifest, n64ControllerPakManifestEntry{
			LogicalKey:    logical.Key,
			RevisionID:    revision.ID,
			GameCode:      logical.GameCode,
			PublisherCode: logical.PublisherCode,
			NoteName:      revision.Entry.NoteName,
		})
	}
	return append([]byte(nil), buf...), manifest, nil
}

func n64ControllerPakProjectionFilename(syncLineKey, runtimeProfile string, imports map[string]n64ControllerPakImportArtifact) string {
	var latest *n64ControllerPakImportArtifact
	for _, candidate := range imports {
		if strings.TrimSpace(candidate.SyncLineKey) != strings.TrimSpace(syncLineKey) {
			continue
		}
		if latest == nil || candidate.CreatedAt.After(latest.CreatedAt) || (candidate.CreatedAt.Equal(latest.CreatedAt) && candidate.ID > latest.ID) {
			copy := candidate
			latest = &copy
		}
	}
	stem := "controller-pak"
	if latest != nil {
		stem = strings.TrimSpace(strings.TrimSuffix(latest.Filename, filepath.Ext(latest.Filename)))
	}
	if stem == "" {
		stem = "controller-pak"
	}
	profile := canonicalN64Profile(runtimeProfile)
	switch profile {
	case n64ProfileMister:
		return safeFilename(stem + ".cpk")
	case n64ProfileRetroArch:
		return safeFilename(stem + ".srm")
	default:
		return safeFilename(stem + ".mpk")
	}
}

func (s *n64ControllerPakStore) rebuildProjectionLinesLocked(syncLineKey, sourceImportID string, createdAt time.Time) ([]n64ControllerPakBuiltProjection, error) {
	definitions := []string{n64ProfileMister, n64ProfileRetroArch, n64ProfileProject64, n64ProfileMupenFamily, n64ProfileEverDrive}
	active := make([]n64ControllerPakLogicalSave, 0)
	revisions := make(map[string]n64ControllerPakLogicalRevision)
	for _, logical := range s.state.LogicalSaves {
		if strings.TrimSpace(logical.SyncLineKey) != strings.TrimSpace(syncLineKey) {
			continue
		}
		if _, tombstoned := s.state.Tombstones[n64ControllerPakTombstoneKey(syncLineKey, logical.Key)]; tombstoned {
			continue
		}
		revision, ok := logical.latestRevision()
		if !ok {
			continue
		}
		active = append(active, logical)
		revisions[logical.Key] = revision
	}
	canonical, manifest, err := buildCanonicalN64ControllerPak(active, revisions)
	if err != nil {
		return nil, err
	}
	built := make([]n64ControllerPakBuiltProjection, 0, len(definitions))
	for _, runtimeProfile := range definitions {
		filename := n64ControllerPakProjectionFilename(syncLineKey, runtimeProfile, s.state.Imports)
		summary := saveSummary{Filename: filename, MediaType: "controller-pak", FileSize: len(canonical), SystemSlug: "n64"}
		projectedFilename, _, projectedPayload, err := projectN64Payload(summary, canonical, runtimeProfile)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(projectedPayload)
		payloadPath, err := s.writeArtifactPayload("projections", projectedPayload, filepath.Ext(projectedFilename))
		if err != nil {
			return nil, err
		}
		projectionLineKey := n64ControllerPakProjectionLineKey(syncLineKey, runtimeProfile)
		projectionID := "n64-cpk-projection-" + createdAt.Format("20060102150405.000000000") + "-" + hash12(projectionLineKey+"::"+hex.EncodeToString(sum[:]))
		projection := n64ControllerPakProjection{
			ID:                projectionID,
			ProjectionLineKey: projectionLineKey,
			SyncLineKey:       syncLineKey,
			RuntimeProfile:    runtimeProfile,
			Filename:          projectedFilename,
			SHA256:            hex.EncodeToString(sum[:]),
			PayloadPath:       payloadPath,
			CreatedAt:         createdAt,
			SourceImportID:    sourceImportID,
			Manifest:          manifest,
		}
		s.state.Projections[projectionID] = projection
		s.state.ProjectionLines[projectionLineKey] = n64ControllerPakProjectionLine{
			Key:                projectionLineKey,
			SyncLineKey:        syncLineKey,
			RuntimeProfile:     runtimeProfile,
			LatestProjectionID: projectionID,
			UpdatedAt:          createdAt,
		}
		built = append(built, n64ControllerPakBuiltProjection{
			ProjectionID:      projectionID,
			ProjectionLineKey: projectionLineKey,
			SyncLineKey:       syncLineKey,
			RuntimeProfile:    runtimeProfile,
			Filename:          projectedFilename,
			Payload:           projectedPayload,
			SourceImportID:    sourceImportID,
		})
	}
	return built, nil
}

func (s *n64ControllerPakStore) importControllerPak(req n64ControllerPakImportRequest) (n64ControllerPakImportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	createdAt := req.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	syncLineKey := n64ControllerPakSyncLineKey(req.ROMSHA1, req.SlotName)
	projectionLineKey := n64ControllerPakProjectionLineKey(syncLineKey, req.RuntimeProfile)
	payloadPath, err := s.writeArtifactPayload("imports", req.CanonicalPayload, filepath.Ext(req.Filename))
	if err != nil {
		return n64ControllerPakImportResult{}, err
	}
	sum := sha256.Sum256(req.CanonicalPayload)
	entries, err := s.extractLogicalEntries(syncLineKey, req.CanonicalPayload)
	if err != nil {
		return n64ControllerPakImportResult{}, err
	}
	manifest := make([]n64ControllerPakManifestEntry, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	importID := "n64-cpk-import-" + createdAt.Format("20060102150405.000000000") + "-" + hash12(syncLineKey+"::"+hex.EncodeToString(sum[:]))
	for _, entry := range entries {
		logical := s.state.LogicalSaves[entry.LogicalKey]
		logical.Key = entry.LogicalKey
		logical.SyncLineKey = syncLineKey
		logical.GameCode = entry.Entry.GameCode
		logical.PublisherCode = entry.Entry.PublisherCode
		logical.NoteName = entry.Entry.NoteName
		logical.RegionCode = entry.RegionCode
		if entry.Catalog != nil {
			logical.CatalogGameID = entry.Catalog.GameID
			logical.CatalogTitle = entry.Catalog.Title
			logical.CatalogSlug = entry.Catalog.Slug
			if normalizeRegionCode(logical.RegionCode) == regionUnknown {
				logical.RegionCode = normalizeRegionCode(entry.Catalog.RegionCode)
			}
		}
		revisionPayloadPath, err := s.writeArtifactPayload("revisions", entry.Payload, ".bin")
		if err != nil {
			return n64ControllerPakImportResult{}, err
		}
		revision := n64ControllerPakLogicalRevision{ID: entry.RevisionID, ImportID: importID, CreatedAt: createdAt, SHA256: entry.SHA256, PayloadPath: revisionPayloadPath, Entry: entry.Entry}
		logical.Revisions = append(logical.Revisions, revision)
		logical.LatestRevisionID = revision.ID
		s.state.LogicalSaves[logical.Key] = logical
		delete(s.state.Tombstones, n64ControllerPakTombstoneKey(syncLineKey, logical.Key))
		seen[logical.Key] = struct{}{}
		manifest = append(manifest, n64ControllerPakManifestEntry{LogicalKey: logical.Key, RevisionID: revision.ID, GameCode: logical.GameCode, PublisherCode: logical.PublisherCode, NoteName: logical.NoteName})
	}
	for _, logical := range s.state.LogicalSaves {
		if strings.TrimSpace(logical.SyncLineKey) != syncLineKey {
			continue
		}
		if _, ok := seen[logical.Key]; ok {
			continue
		}
		s.state.Tombstones[n64ControllerPakTombstoneKey(syncLineKey, logical.Key)] = n64ControllerPakTombstone{LogicalKey: logical.Key, SyncLineKey: syncLineKey, CreatedAt: createdAt, Reason: "missing from latest controller pak import"}
	}
	s.state.Imports[importID] = n64ControllerPakImportArtifact{ID: importID, SyncLineKey: syncLineKey, RuntimeProfile: req.RuntimeProfile, Filename: req.Filename, SHA256: hex.EncodeToString(sum[:]), PayloadPath: payloadPath, CreatedAt: createdAt, ProjectionLineKey: projectionLineKey, Manifest: manifest}
	built, err := s.rebuildProjectionLinesLocked(syncLineKey, importID, createdAt)
	if err != nil {
		return n64ControllerPakImportResult{}, err
	}
	if err := s.persistLocked(); err != nil {
		return n64ControllerPakImportResult{}, err
	}
	return n64ControllerPakImportResult{PrimaryProjectionLineKey: projectionLineKey, Built: built}, nil
}

func (a *app) n64ControllerPakStore() *n64ControllerPakStore {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.n64ControllerPakStoreRef
}

func (a *app) n64ControllerPakListSummaries(systemID int, romSHA1 string) []saveSummary {
	store := a.n64ControllerPakStore()
	if store == nil {
		return nil
	}
	contexts := store.listLogicalContexts()
	out := make([]saveSummary, 0, len(contexts))
	for _, ctx := range contexts {
		sys := supportedSystemFromSlug("n64")
		if systemID != 0 && (sys == nil || sys.ID != systemID) {
			continue
		}
		if strings.TrimSpace(romSHA1) != "" && !strings.HasPrefix(strings.TrimSpace(ctx.Logical.SyncLineKey), strings.ToLower(strings.TrimSpace(romSHA1))+"::") {
			continue
		}
		out = append(out, buildN64ControllerPakLogicalListSummary(ctx))
	}
	return out
}

func (a *app) materializeN64ControllerPakProjections(template saveCreateInput, romSHA1, slotName string, built []n64ControllerPakBuiltProjection) (map[string]saveRecord, error) {
	store := a.n64ControllerPakStore()
	if store == nil {
		return nil, fmt.Errorf("n64 controller pak store is not initialized")
	}
	recordsByLine := make(map[string]saveRecord, len(built))
	createdAt := template.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	for _, candidate := range built {
		record, err := a.createSave(saveCreateInput{
			Filename:              candidate.Filename,
			Payload:               candidate.Payload,
			Game:                  game{Name: "Controller Pak", DisplayTitle: "Controller Pak", System: supportedSystemFromSlug("n64")},
			Format:                inferSaveFormat(candidate.Filename),
			Metadata:              newN64ControllerPakProjectionMetadata(candidate.SyncLineKey, candidate.RuntimeProfile, candidate.ProjectionID, candidate.SourceImportID),
			ROMSHA1:               n64ControllerPakProjectionConflictKey(candidate.SyncLineKey, candidate.RuntimeProfile),
			SlotName:              slotName,
			SystemSlug:            "n64",
			GameSlug:              "controller-pak",
			DisplayTitle:          "Controller Pak",
			MediaType:             "controller-pak",
			ProjectionCapable:     boolPtr(true),
			SourceArtifactProfile: candidate.RuntimeProfile,
			TrustedHelperSystem:   true,
			RuntimeProfile:        candidate.RuntimeProfile,
			ProjectionID:          candidate.ProjectionID,
			SourceImportID:        candidate.SourceImportID,
			CreatedAt:             createdAt,
		})
		if err != nil {
			return nil, fmt.Errorf("materialize n64 controller pak projection %s (%s): %w", candidate.RuntimeProfile, candidate.Filename, err)
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

func (a *app) createN64ControllerPakProjectionSave(input saveCreateInput, preview normalizedSaveInputResult) (saveRecord, error) {
	store := a.n64ControllerPakStore()
	if store == nil {
		return saveRecord{}, fmt.Errorf("n64 controller pak store is not initialized")
	}
	result, err := store.importControllerPak(n64ControllerPakImportRequest{CanonicalPayload: input.Payload, Filename: input.Filename, RuntimeProfile: input.RuntimeProfile, ROMSHA1: input.ROMSHA1, SlotName: input.SlotName, CreatedAt: input.CreatedAt})
	if err != nil {
		return saveRecord{}, err
	}
	recordsByLine, err := a.materializeN64ControllerPakProjections(saveCreateInput{CreatedAt: firstNonZeroTime(input.CreatedAt, preview.Input.CreatedAt)}, input.ROMSHA1, input.SlotName, result.Built)
	if err != nil {
		return saveRecord{}, err
	}
	primary, ok := recordsByLine[result.PrimaryProjectionLineKey]
	if !ok {
		return saveRecord{}, fmt.Errorf("primary n64 controller pak projection was not created")
	}
	return primary, nil
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func boolPtr(value bool) *bool { return &value }

func (a *app) n64ControllerPakHistoryForSaveRecord(saveRecordID, logicalKey string) (n64ControllerPakHistory, error) {
	store := a.n64ControllerPakStore()
	if store == nil {
		return n64ControllerPakHistory{}, fmt.Errorf("n64 controller pak store is not initialized")
	}
	ctx, err := store.logicalContextForSaveRecord(saveRecordID, logicalKey)
	if err != nil {
		return n64ControllerPakHistory{}, err
	}
	return buildN64ControllerPakHistory(ctx), nil
}

func (a *app) downloadN64ControllerPakLogicalSave(saveRecordID, logicalKey, revisionID, requestedProfile string) (string, string, []byte, error) {
	store := a.n64ControllerPakStore()
	if store == nil {
		return "", "", nil, fmt.Errorf("n64 controller pak store is not initialized")
	}
	ctx, err := store.logicalContextForSaveRecord(saveRecordID, logicalKey)
	if err != nil {
		return "", "", nil, err
	}
	if strings.TrimSpace(requestedProfile) == "" {
		requestedProfile = ctx.Projection.RuntimeProfile
	}
	revision, ok := ctx.Logical.latestRevision()
	if strings.TrimSpace(revisionID) != "" {
		revision, ok = ctx.Logical.revisionByID(revisionID)
	}
	if !ok {
		return "", "", nil, fmt.Errorf("n64 controller pak revision not found")
	}
	store.mu.Lock()
	active := make([]n64ControllerPakLogicalSave, 0)
	revisions := make(map[string]n64ControllerPakLogicalRevision)
	for _, logical := range store.state.LogicalSaves {
		if strings.TrimSpace(logical.SyncLineKey) != strings.TrimSpace(ctx.Logical.SyncLineKey) {
			continue
		}
		if _, tombstoned := store.state.Tombstones[n64ControllerPakTombstoneKey(logical.SyncLineKey, logical.Key)]; tombstoned && logical.Key != ctx.Logical.Key {
			continue
		}
		latest, ok := logical.latestRevision()
		if !ok {
			continue
		}
		if logical.Key == ctx.Logical.Key {
			latest = revision
		}
		active = append(active, logical)
		revisions[logical.Key] = latest
	}
	filename := n64ControllerPakProjectionFilename(ctx.Logical.SyncLineKey, requestedProfile, store.state.Imports)
	store.mu.Unlock()
	canonical, _, err := buildCanonicalN64ControllerPak(active, revisions)
	if err != nil {
		return "", "", nil, err
	}
	summary := saveSummary{Filename: filename, MediaType: "controller-pak", FileSize: len(canonical), SystemSlug: "n64"}
	return projectN64Payload(summary, canonical, requestedProfile)
}

func (a *app) rollbackN64ControllerPakLogicalSave(sourceRecord saveRecord, logicalKey, revisionID string) (saveRecord, error) {
	store := a.n64ControllerPakStore()
	if store == nil {
		return saveRecord{}, fmt.Errorf("n64 controller pak store is not initialized")
	}
	syncLineKey, _, _, ok := n64ControllerPakProjectionInfoFromRecord(sourceRecord)
	if !ok {
		return saveRecord{}, fmt.Errorf("save is not an n64 controller pak projection")
	}
	store.mu.Lock()
	logical, ok := store.state.LogicalSaves[strings.TrimSpace(logicalKey)]
	if !ok || strings.TrimSpace(logical.SyncLineKey) != strings.TrimSpace(syncLineKey) {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("n64 controller pak logical save not found")
	}
	revision, ok := logical.revisionByID(revisionID)
	if !ok {
		store.mu.Unlock()
		return saveRecord{}, fmt.Errorf("n64 controller pak revision not found")
	}
	now := time.Now().UTC()
	copyRevision := revision
	copyRevision.ID = "n64-cpk-revision-" + now.Format("20060102150405.000000000") + "-" + hash12(logical.Key+"::"+revisionID+"::rollback")
	copyRevision.ImportID = "rollback:" + sourceRecord.Summary.ID
	copyRevision.CreatedAt = now
	logical.Revisions = append(logical.Revisions, copyRevision)
	logical.LatestRevisionID = copyRevision.ID
	store.state.LogicalSaves[logical.Key] = logical
	delete(store.state.Tombstones, n64ControllerPakTombstoneKey(syncLineKey, logical.Key))
	built, err := store.rebuildProjectionLinesLocked(syncLineKey, copyRevision.ImportID, now)
	if err != nil {
		store.mu.Unlock()
		return saveRecord{}, err
	}
	if err := store.persistLocked(); err != nil {
		store.mu.Unlock()
		return saveRecord{}, err
	}
	store.mu.Unlock()
	recordsByLine, err := a.materializeN64ControllerPakProjections(saveCreateInput{CreatedAt: now}, strings.TrimSpace(""), sourceRecord.SlotName, built)
	if err != nil {
		return saveRecord{}, err
	}
	for _, record := range recordsByLine {
		if _, runtimeProfile, _, ok := n64ControllerPakProjectionInfoFromRecord(record); ok && runtimeProfile == sourceRecord.Summary.RuntimeProfile {
			return record, nil
		}
	}
	for _, record := range recordsByLine {
		return record, nil
	}
	return saveRecord{}, fmt.Errorf("rebuilt n64 controller pak projection missing runtime line")
}

func (a *app) deleteN64ControllerPakLogicalSave(sourceRecord saveRecord, logicalKey string) (int, error) {
	store := a.n64ControllerPakStore()
	if store == nil {
		return 0, fmt.Errorf("n64 controller pak store is not initialized")
	}
	syncLineKey, _, _, ok := n64ControllerPakProjectionInfoFromRecord(sourceRecord)
	if !ok {
		return 0, fmt.Errorf("save is not an n64 controller pak projection")
	}
	store.mu.Lock()
	logical, ok := store.state.LogicalSaves[strings.TrimSpace(logicalKey)]
	if !ok || strings.TrimSpace(logical.SyncLineKey) != strings.TrimSpace(syncLineKey) {
		store.mu.Unlock()
		return 0, fmt.Errorf("n64 controller pak logical save not found")
	}
	now := time.Now().UTC()
	store.state.Tombstones[n64ControllerPakTombstoneKey(syncLineKey, logical.Key)] = n64ControllerPakTombstone{LogicalKey: logical.Key, SyncLineKey: syncLineKey, CreatedAt: now, Reason: "logical save deleted from API"}
	built, err := store.rebuildProjectionLinesLocked(syncLineKey, "delete:"+logical.Key, now)
	if err != nil {
		store.mu.Unlock()
		return 0, err
	}
	if err := store.persistLocked(); err != nil {
		store.mu.Unlock()
		return 0, err
	}
	store.mu.Unlock()
	if _, err := a.materializeN64ControllerPakProjections(saveCreateInput{CreatedAt: now}, strings.TrimSpace(""), sourceRecord.SlotName, built); err != nil {
		return 0, err
	}
	return len(a.snapshotSaveRecords()), nil
}

func (a *app) projectN64ControllerPakProjectionPayload(record saveRecord, requestedProfile string) (string, string, []byte, error) {
	profile := canonicalRuntimeProfile("n64", requestedProfile)
	if profile == "" {
		return "", "", nil, fmt.Errorf("unsupported runtimeProfile %q", requestedProfile)
	}
	store := a.n64ControllerPakStore()
	if store == nil {
		return "", "", nil, fmt.Errorf("n64 controller pak store is not initialized")
	}
	syncLineKey, currentProfile, _, ok := n64ControllerPakProjectionInfoFromRecord(record)
	if !ok {
		return "", "", nil, fmt.Errorf("save is not an n64 controller pak projection")
	}
	targetRecord := record
	if profile != currentProfile {
		saveID, exists := store.latestProjectionSaveRecord(syncLineKey, profile)
		if !exists {
			return "", "", nil, fmt.Errorf("n64 controller pak projection %q is not available", profile)
		}
		resolved, found := a.findSaveRecordByID(saveID)
		if !found {
			return "", "", nil, fmt.Errorf("n64 controller pak projection save record not found")
		}
		targetRecord = resolved
	}
	payload, err := os.ReadFile(targetRecord.payloadPath)
	if err != nil {
		return "", "", nil, err
	}
	return targetRecord.Summary.Filename, "application/octet-stream", payload, nil
}
