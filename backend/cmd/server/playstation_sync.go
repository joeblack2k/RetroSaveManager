package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed testdata/ps2_memory_card_fixture.ps2.gz
var ps2ProjectionTemplateGzip []byte

const (
	playStationStateDirname = "_rsm/playstation"
	playStationStateFile    = "state.json"

	ps2ModeDirectory       = 0x8427
	ps2ModeRootParent      = 0xA426
	ps2ModeFileRegular     = 0x8417
	ps1DirectoryStateFree  = 0xA0
	ps1DirectoryStateFirst = 0x51
	ps1DirectoryStateMid   = 0x52
	ps1DirectoryStateLast  = 0x53
)

type playStationStore struct {
	mu        sync.Mutex
	root      string
	statePath string
	state     playStationState
}

type playStationState struct {
	Imports         map[string]psImportArtifact  `json:"imports,omitempty"`
	LogicalSaves    map[string]psLogicalSave     `json:"logicalSaves,omitempty"`
	ProjectionLines map[string]psProjectionLine  `json:"projectionLines,omitempty"`
	Projections     map[string]psProjection      `json:"projections,omitempty"`
	Tombstones      map[string]psTombstone       `json:"tombstones,omitempty"`
	DeviceLines     map[string]psDeviceLineState `json:"deviceLines,omitempty"`
}

type psImportArtifact struct {
	ID                 string            `json:"id"`
	SystemSlug         string            `json:"systemSlug"`
	RuntimeProfile     string            `json:"runtimeProfile"`
	CardSlot           string            `json:"cardSlot"`
	ProjectionLineKey  string            `json:"projectionLineKey"`
	SyncLineKey        string            `json:"syncLineKey"`
	Fingerprint        string            `json:"fingerprint,omitempty"`
	Filename           string            `json:"filename"`
	ArtifactKind       saveArtifactKind  `json:"artifactKind"`
	SHA256             string            `json:"sha256"`
	PayloadPath        string            `json:"payloadPath"`
	CreatedAt          time.Time         `json:"createdAt"`
	BaselineProjection string            `json:"baselineProjectionId,omitempty"`
	Manifest           []psManifestEntry `json:"manifest,omitempty"`
}

type psManifestEntry struct {
	LogicalKey        string `json:"logicalKey"`
	RevisionID        string `json:"revisionId"`
	ScopeKey          string `json:"scopeKey"`
	DisplayTitle      string `json:"displayTitle"`
	RegionCode        string `json:"regionCode,omitempty"`
	ProductCode       string `json:"productCode,omitempty"`
	Portable          bool   `json:"portable"`
	ProjectionLineKey string `json:"projectionLineKey,omitempty"`
	SyncLineKey       string `json:"syncLineKey,omitempty"`
}

type psLogicalSave struct {
	Key               string                  `json:"key"`
	SystemSlug        string                  `json:"systemSlug"`
	SyncLineKey       string                  `json:"syncLineKey,omitempty"`
	ProjectionLineKey string                  `json:"projectionLineKey,omitempty"`
	DisplayTitle      string                  `json:"displayTitle"`
	NormalizedTitle   string                  `json:"normalizedTitle"`
	RegionCode        string                  `json:"regionCode,omitempty"`
	ProductCode       string                  `json:"productCode,omitempty"`
	Portable          bool                    `json:"portable"`
	Revisions         []psLogicalSaveRevision `json:"revisions,omitempty"`
	LatestRevisionID  string                  `json:"latestRevisionId"`
}

type psLogicalSaveRevision struct {
	ID          string              `json:"id"`
	ImportID    string              `json:"importId"`
	CreatedAt   time.Time           `json:"createdAt"`
	SHA256      string              `json:"sha256"`
	MemoryEntry memoryCardEntry     `json:"memoryEntry"`
	PS1         *ps1LogicalRevision `json:"ps1,omitempty"`
	PS2         *ps2LogicalRevision `json:"ps2,omitempty"`
}

type ps1LogicalRevision struct {
	DirEntries [][]byte `json:"dirEntries,omitempty"`
	Blocks     [][]byte `json:"blocks,omitempty"`
}

type ps2LogicalRevision struct {
	DirectoryName string             `json:"directoryName,omitempty"`
	Nodes         []ps2LogicalFSNode `json:"nodes,omitempty"`
}

type ps2LogicalFSNode struct {
	Path      string `json:"path"`
	Mode      uint16 `json:"mode"`
	Directory bool   `json:"directory,omitempty"`
	Data      []byte `json:"data,omitempty"`
}

type psProjectionLine struct {
	Key                string    `json:"key"`
	SystemSlug         string    `json:"systemSlug"`
	RuntimeProfile     string    `json:"runtimeProfile"`
	CardSlot           string    `json:"cardSlot"`
	SyncLineKey        string    `json:"syncLineKey"`
	LatestProjectionID string    `json:"latestProjectionId,omitempty"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type psProjection struct {
	ID                string             `json:"id"`
	ProjectionLineKey string             `json:"projectionLineKey"`
	SystemSlug        string             `json:"systemSlug"`
	RuntimeProfile    string             `json:"runtimeProfile"`
	CardSlot          string             `json:"cardSlot"`
	Filename          string             `json:"filename"`
	SHA256            string             `json:"sha256"`
	PayloadPath       string             `json:"payloadPath"`
	CreatedAt         time.Time          `json:"createdAt"`
	SaveRecordID      string             `json:"saveRecordId,omitempty"`
	SourceImportID    string             `json:"sourceImportId,omitempty"`
	Portable          bool               `json:"portable"`
	Manifest          []psManifestEntry  `json:"manifest,omitempty"`
	MemoryCard        *memoryCardDetails `json:"memoryCard,omitempty"`
}

type psTombstone struct {
	ID           string    `json:"id"`
	LogicalKey   string    `json:"logicalKey"`
	ScopeKey     string    `json:"scopeKey"`
	Reason       string    `json:"reason,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	SourceImport string    `json:"sourceImportId,omitempty"`
}

type psDeviceLineState struct {
	Key                      string    `json:"key"`
	ProjectionLineKey        string    `json:"projectionLineKey"`
	Fingerprint              string    `json:"fingerprint"`
	LastDownloadedProjection string    `json:"lastDownloadedProjectionId,omitempty"`
	LastImportedArtifact     string    `json:"lastImportedArtifactId,omitempty"`
	UpdatedAt                time.Time `json:"updatedAt"`
}

type psImportRequest struct {
	Payload        []byte
	Filename       string
	ArtifactKind   saveArtifactKind
	RuntimeProfile string
	SystemSlug     string
	CardSlot       string
	Fingerprint    string
	CreatedAt      time.Time
	HelperDevice   string
}

type psBuiltProjection struct {
	ProjectionID      string
	ProjectionLineKey string
	SystemSlug        string
	RuntimeProfile    string
	CardSlot          string
	Filename          string
	Payload           []byte
	MemoryCard        *memoryCardDetails
	SourceImportID    string
	Portable          bool
}

type psImportResult struct {
	PrimaryProjectionLineKey string
	Built                    []psBuiltProjection
	Conflict                 *psImportConflict
}

type psImportConflict struct {
	ConflictKey     string
	CloudProjection string
	LocalSHA256     string
	CloudSHA256     string
}

type psExtractedEntry struct {
	LogicalKey        string
	SystemSlug        string
	SyncLineKey       string
	ProjectionLineKey string
	ScopeKey          string
	Portable          bool
	DisplayTitle      string
	NormalizedTitle   string
	RegionCode        string
	ProductCode       string
	MemoryEntry       memoryCardEntry
	PS1               *ps1LogicalRevision
	PS2               *ps2LogicalRevision
	SHA256            string
}

var (
	ps2ProjectionTemplateOnce sync.Once
	ps2ProjectionTemplateData []byte
	ps2ProjectionTemplateErr  error
)

func newPlayStationStore(saveRoot string) (*playStationStore, error) {
	root, err := safeJoinUnderRoot(saveRoot, playStationStateDirname)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create playstation root: %w", err)
	}
	store := &playStationStore{
		root:      root,
		statePath: filepath.Join(root, playStationStateFile),
		state:     emptyPlayStationState(),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func emptyPlayStationState() playStationState {
	return playStationState{
		Imports:         map[string]psImportArtifact{},
		LogicalSaves:    map[string]psLogicalSave{},
		ProjectionLines: map[string]psProjectionLine{},
		Projections:     map[string]psProjection{},
		Tombstones:      map[string]psTombstone{},
		DeviceLines:     map[string]psDeviceLineState{},
	}
}

func (s *playStationStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.state = emptyPlayStationState()
			return nil
		}
		return fmt.Errorf("read playstation state: %w", err)
	}
	state := emptyPlayStationState()
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode playstation state: %w", err)
	}
	s.state = state
	return nil
}

func (s *playStationStore) persistLocked() error {
	payload, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode playstation state: %w", err)
	}
	if err := writeFileAtomic(s.statePath, payload, 0o644); err != nil {
		return fmt.Errorf("write playstation state: %w", err)
	}
	return nil
}

func supportedPlayStationRuntimeProfile(deviceType string, artifact saveArtifactKind) (profile string, systemSlug string, err error) {
	switch canonicalPlayStationDeviceType(deviceType) {
	case "mister":
		if artifact != saveArtifactPS1MemoryCard {
			return "", "", fmt.Errorf("MiSTer is only supported for PlayStation memory cards")
		}
		return "psx/mister", "psx", nil
	case "retroarch":
		if artifact != saveArtifactPS1MemoryCard {
			return "", "", fmt.Errorf("RetroArch is only supported for PlayStation memory cards")
		}
		return "psx/retroarch", "psx", nil
	case "pcsx2":
		if artifact != saveArtifactPS2MemoryCard {
			return "", "", fmt.Errorf("PCSX2 is only supported for PlayStation 2 memory cards")
		}
		return "ps2/pcsx2", "ps2", nil
	default:
		return "", "", fmt.Errorf("unsupported PlayStation runtime %q", strings.TrimSpace(deviceType))
	}
}

func canonicalPlayStationDeviceType(deviceType string) string {
	clean := strings.ToLower(strings.TrimSpace(deviceType))
	switch {
	case clean == "mister" || clean == "mister-fpga" || strings.HasPrefix(clean, "mister-"):
		return "mister"
	case clean == "retroarch" || strings.HasPrefix(clean, "retroarch-"):
		return "retroarch"
	case clean == "pcsx2" || strings.HasPrefix(clean, "pcsx2-"):
		return "pcsx2"
	default:
		return clean
	}
}

func supportedProjectionProfiles(systemSlug string) []string {
	switch canonicalSegment(systemSlug, "") {
	case "psx":
		return []string{"psx/mister", "psx/retroarch"}
	case "ps2":
		return []string{"ps2/pcsx2"}
	default:
		return nil
	}
}

func deriveExplicitMemoryCardName(slotName, filename string) (string, bool) {
	candidates := []string{slotName, filename}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if match := mcdNumberPattern.FindStringSubmatch(candidate); len(match) >= 2 {
			index, err := strconv.Atoi(match[1])
			if err == nil && index > 0 {
				return "Memory Card " + strconv.Itoa(index), true
			}
		}
		if match := cardNumberPattern.FindStringSubmatch(candidate); len(match) >= 3 {
			index, err := strconv.Atoi(match[2])
			if err == nil && index > 0 {
				return "Memory Card " + strconv.Itoa(index), true
			}
		}
	}
	return "", false
}

func projectionLineKey(runtimeProfile, cardSlot string) string {
	return strings.ToLower(strings.TrimSpace(runtimeProfile)) + "::" + canonicalSegment(cardSlot, "memory-card-1")
}

func projectionConflictKey(runtimeProfile, cardSlot string) string {
	return "ps-projection::" + projectionLineKey(runtimeProfile, cardSlot)
}

func syncLineKey(systemSlug, cardSlot string) string {
	return canonicalSegment(systemSlug, "unknown-system") + "::slot::" + canonicalSegment(cardSlot, "memory-card-1")
}

func deviceLineKey(runtimeProfile, cardSlot, fingerprint string) string {
	return projectionLineKey(runtimeProfile, cardSlot) + "::" + strings.ToLower(strings.TrimSpace(fingerprint))
}

func psScopeKey(portable bool, syncKey, projectionKey string) string {
	if portable {
		return syncKey
	}
	return projectionKey
}

func psTombstoneKey(scopeKey, logicalKey string) string {
	return strings.ToLower(strings.TrimSpace(scopeKey)) + "::" + strings.ToLower(strings.TrimSpace(logicalKey))
}

func isPlayStationRuntimeDevice(deviceType string) bool {
	switch canonicalPlayStationDeviceType(deviceType) {
	case "mister", "retroarch", "pcsx2":
		return true
	default:
		return false
	}
}

func (s *playStationStore) importMemoryCard(req psImportRequest) (psImportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := &s.state
	now := req.CreatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	lineKey := projectionLineKey(req.RuntimeProfile, req.CardSlot)
	scopeKey := syncLineKey(req.SystemSlug, req.CardSlot)
	deviceKey := deviceLineKey(req.RuntimeProfile, req.CardSlot, req.Fingerprint)

	entries, details, err := s.extractLogicalEntries(req.SystemSlug, req.RuntimeProfile, req.CardSlot, req.Filename, req.Payload)
	if err != nil {
		return psImportResult{}, err
	}
	manifest := make([]psManifestEntry, 0, len(entries))
	for _, entry := range entries {
		manifest = append(manifest, psManifestEntry{
			LogicalKey:        entry.LogicalKey,
			RevisionID:        "",
			ScopeKey:          entry.ScopeKey,
			DisplayTitle:      entry.DisplayTitle,
			RegionCode:        entry.RegionCode,
			ProductCode:       entry.ProductCode,
			Portable:          entry.Portable,
			ProjectionLineKey: entry.ProjectionLineKey,
			SyncLineKey:       entry.SyncLineKey,
		})
	}

	payloadPath, shaHex, err := s.writeImportArtifactPayload(req.Payload)
	if err != nil {
		return psImportResult{}, err
	}
	importID := "ps-import-" + now.Format("20060102150405.000000000") + "-" + shaHex[:12]
	artifact := psImportArtifact{
		ID:                importID,
		SystemSlug:        req.SystemSlug,
		RuntimeProfile:    req.RuntimeProfile,
		CardSlot:          req.CardSlot,
		ProjectionLineKey: lineKey,
		SyncLineKey:       scopeKey,
		Fingerprint:       strings.TrimSpace(req.Fingerprint),
		Filename:          safeFilename(req.Filename),
		ArtifactKind:      req.ArtifactKind,
		SHA256:            shaHex,
		PayloadPath:       payloadPath,
		CreatedAt:         now,
		Manifest:          manifest,
	}

	line := state.ProjectionLines[lineKey]
	line.Key = lineKey
	line.SystemSlug = req.SystemSlug
	line.RuntimeProfile = req.RuntimeProfile
	line.CardSlot = req.CardSlot
	line.SyncLineKey = scopeKey
	line.UpdatedAt = now
	state.ProjectionLines[lineKey] = line

	dev := state.DeviceLines[deviceKey]
	dev.Key = deviceKey
	dev.ProjectionLineKey = lineKey
	dev.Fingerprint = strings.TrimSpace(req.Fingerprint)
	dev.UpdatedAt = now

	var conflict *psImportConflict
	if dev.LastDownloadedProjection != "" {
		artifact.BaselineProjection = dev.LastDownloadedProjection
		if baseline, ok := state.Projections[dev.LastDownloadedProjection]; ok {
			baselineKeys := manifestKeysByScope(baseline.Manifest, lineKey, scopeKey)
			incomingKeys := manifestKeysByScope(manifest, lineKey, scopeKey)
			missing := missingManifestKeys(baselineKeys, incomingKeys)
			if len(missing) > 0 {
				if line.LatestProjectionID != "" && line.LatestProjectionID == dev.LastDownloadedProjection {
					for _, logicalKey := range missing {
						tombstone := psTombstone{
							ID:           deterministicConflictID(psTombstoneKey(scopeKey, logicalKey)),
							LogicalKey:   logicalKey,
							ScopeKey:     scopeKey,
							Reason:       "device upload removed logical save",
							CreatedAt:    now,
							SourceImport: importID,
						}
						state.Tombstones[psTombstoneKey(scopeKey, logicalKey)] = tombstone
					}
				} else {
					conflict = &psImportConflict{
						ConflictKey:     projectionConflictKey(req.RuntimeProfile, req.CardSlot),
						CloudProjection: dev.LastDownloadedProjection,
						LocalSHA256:     shaHex,
						CloudSHA256:     baseline.SHA256,
					}
				}
			}
		}
	}

	for i, entry := range entries {
		logical := state.LogicalSaves[entry.LogicalKey]
		logical.Key = entry.LogicalKey
		logical.SystemSlug = entry.SystemSlug
		logical.SyncLineKey = entry.SyncLineKey
		logical.ProjectionLineKey = entry.ProjectionLineKey
		logical.DisplayTitle = entry.DisplayTitle
		logical.NormalizedTitle = entry.NormalizedTitle
		logical.RegionCode = entry.RegionCode
		logical.ProductCode = entry.ProductCode
		logical.Portable = entry.Portable
		if latest, ok := logical.latestRevision(); ok && latest.SHA256 == entry.SHA256 {
			manifest[i].RevisionID = latest.ID
			delete(state.Tombstones, psTombstoneKey(entry.ScopeKey, entry.LogicalKey))
			state.LogicalSaves[entry.LogicalKey] = logical
			continue
		}
		revisionID := "ps-rev-" + now.Format("20060102150405.000000000") + "-" + hash12(entry.SHA256+entry.LogicalKey)
		revision := psLogicalSaveRevision{
			ID:          revisionID,
			ImportID:    importID,
			CreatedAt:   now,
			SHA256:      entry.SHA256,
			MemoryEntry: entry.MemoryEntry,
			PS1:         entry.PS1,
			PS2:         entry.PS2,
		}
		logical.Revisions = append(logical.Revisions, revision)
		logical.LatestRevisionID = revisionID
		state.LogicalSaves[entry.LogicalKey] = logical
		manifest[i].RevisionID = revisionID
		delete(state.Tombstones, psTombstoneKey(entry.ScopeKey, entry.LogicalKey))
	}

	artifact.Manifest = manifest
	state.Imports[importID] = artifact
	dev.LastImportedArtifact = importID
	state.DeviceLines[deviceKey] = dev

	line.LatestProjectionID = line.LatestProjectionID
	state.ProjectionLines[lineKey] = line

	built, err := s.rebuildProjectionLinesLocked(req.SystemSlug, req.CardSlot, req.RuntimeProfile, importID, details.Name)
	if err != nil {
		return psImportResult{}, err
	}
	if len(built) == 0 {
		return psImportResult{}, fmt.Errorf("no PlayStation projections were generated")
	}
	if err := s.persistLocked(); err != nil {
		return psImportResult{}, err
	}
	return psImportResult{PrimaryProjectionLineKey: lineKey, Built: built, Conflict: conflict}, nil
}

func manifestKeysByScope(entries []psManifestEntry, projectionLineKey, syncLineKey string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, entry := range entries {
		if entry.ScopeKey != projectionLineKey && entry.ScopeKey != syncLineKey {
			continue
		}
		out[entry.LogicalKey] = struct{}{}
	}
	return out
}

func missingManifestKeys(cloud, local map[string]struct{}) []string {
	missing := make([]string, 0)
	for key := range cloud {
		if _, ok := local[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

func (s *playStationStore) writeImportArtifactPayload(payload []byte) (string, string, error) {
	sha := sha256.Sum256(payload)
	shaHex := hex.EncodeToString(sha[:])
	id := "import-" + time.Now().UTC().Format("20060102150405.000000000") + "-" + shaHex[:12]
	dir := filepath.Join(s.root, "imports", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("create import artifact dir: %w", err)
	}
	path := filepath.Join(dir, "card.bin")
	if err := writeFileAtomic(path, payload, 0o644); err != nil {
		return "", "", fmt.Errorf("write import artifact payload: %w", err)
	}
	return path, shaHex, nil
}

func (s *playStationStore) extractLogicalEntries(systemSlug, runtimeProfile, cardSlot, filename string, payload []byte) ([]psExtractedEntry, *memoryCardDetails, error) {
	switch canonicalSegment(systemSlug, "") {
	case "psx":
		return extractPS1LogicalEntries(runtimeProfile, cardSlot, filename, payload)
	case "ps2":
		return extractPS2LogicalEntries(runtimeProfile, cardSlot, filename, payload)
	default:
		return nil, nil, fmt.Errorf("unsupported PlayStation system %q", systemSlug)
	}
}

func extractPS1LogicalEntries(runtimeProfile, cardSlot, filename string, payload []byte) ([]psExtractedEntry, *memoryCardDetails, error) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	imageData := normalizedPS1MemoryCardImage(payload, ext)
	if len(imageData) != ps1MemoryCardTotalSize {
		return nil, nil, fmt.Errorf("invalid PlayStation memory card payload")
	}
	details := parsePS1MemoryCard(payload, filename, cardSlot)
	if details == nil {
		details = &memoryCardDetails{Name: cardSlot}
	}
	entryBySlot := map[int]memoryCardEntry{}
	for _, entry := range details.Entries {
		entryBySlot[entry.Slot] = entry
	}

	out := make([]psExtractedEntry, 0, len(details.Entries))
	for dirIndex := 1; dirIndex <= psDirectoryEntries; dirIndex++ {
		offset := dirIndex * psDirectoryEntrySize
		if offset+psDirectoryEntrySize > len(imageData) {
			break
		}
		dirEntry := imageData[offset : offset+psDirectoryEntrySize]
		if !isPS1DirectoryStartEntry(dirEntry[0]) {
			continue
		}
		chain := ps1DirectoryChainIndices(imageData, dirIndex)
		if len(chain) == 0 {
			continue
		}
		blocks := make([][]byte, 0, len(chain))
		dirEntries := make([][]byte, 0, len(chain))
		for _, slot := range chain {
			blockOffset := slot * psMemoryCardBlockSize
			if blockOffset+psMemoryCardBlockSize > len(imageData) {
				continue
			}
			blocks = append(blocks, append([]byte(nil), imageData[blockOffset:blockOffset+psMemoryCardBlockSize]...))
			dirOffset := slot * psDirectoryEntrySize
			if dirOffset+psDirectoryEntrySize <= len(imageData) {
				dirEntries = append(dirEntries, append([]byte(nil), imageData[dirOffset:dirOffset+psDirectoryEntrySize]...))
			}
		}
		if len(blocks) == 0 {
			continue
		}
		productCode := strings.TrimSpace(extractPrintableASCII(dirEntry[0x0a:0x16]))
		title := parsePS1MemoryCardEntryTitle(blocks[0], productCode)
		displayTitle := canonicalDisplayTitle(title)
		regionCode := normalizeRegionCode(regionFromProductCode(productCode))
		if regionCode == regionUnknown {
			regionCode = normalizeRegionCode(detectRegionCode(title))
		}
		portable := strings.TrimSpace(productCode) != ""
		projectionKey := projectionLineKey(runtimeProfile, cardSlot)
		syncKey := syncLineKey("psx", cardSlot)
		logicalKey := portablePSLogicalKey("psx", productCode, displayTitle, regionCode)
		if !portable {
			logicalKey = nonPortablePSLogicalKey("psx", projectionKey, dirIndex, displayTitle)
		}
		memoryEntry := entryBySlot[dirIndex]
		memoryEntry.LogicalKey = logicalKey
		memoryEntry.Title = displayTitle
		memoryEntry.ProductCode = productCode
		memoryEntry.RegionCode = regionCode
		if memoryEntry.Slot == 0 {
			memoryEntry.Slot = dirIndex
		}
		if memoryEntry.Blocks == 0 {
			memoryEntry.Blocks = len(blocks)
		}
		entry := psExtractedEntry{
			LogicalKey:        logicalKey,
			SystemSlug:        "psx",
			SyncLineKey:       syncKey,
			ProjectionLineKey: projectionKey,
			ScopeKey:          psScopeKey(portable, syncKey, projectionKey),
			Portable:          portable,
			DisplayTitle:      displayTitle,
			NormalizedTitle:   canonicalTrackTitleKey(displayTitle),
			RegionCode:        regionCode,
			ProductCode:       productCode,
			MemoryEntry:       memoryEntry,
			PS1: &ps1LogicalRevision{
				DirEntries: dirEntries,
				Blocks:     blocks,
			},
		}
		entry.SHA256 = hashPS1LogicalEntry(entry)
		out = append(out, entry)
	}
	return out, details, nil
}

func portablePSLogicalKey(systemSlug, productCode, displayTitle, regionCode string) string {
	return canonicalSegment(systemSlug, "unknown-system") + "::" + strings.ToUpper(strings.TrimSpace(productCode)) + "::" + canonicalTrackTitleKey(displayTitle) + "::" + normalizeRegionCode(regionCode)
}

func nonPortablePSLogicalKey(systemSlug, projectionLineKey string, ordinal int, displayTitle string) string {
	return canonicalSegment(systemSlug, "unknown-system") + "::nonportable::" + canonicalSegment(projectionLineKey, "line") + "::" + strconv.Itoa(ordinal) + "::" + canonicalTrackTitleKey(displayTitle)
}

func ps1DirectoryChainIndices(payload []byte, start int) []int {
	visited := map[int]struct{}{}
	chain := make([]int, 0, 4)
	current := start
	for current >= 1 && current <= psDirectoryEntries {
		if _, exists := visited[current]; exists {
			break
		}
		visited[current] = struct{}{}
		chain = append(chain, current)
		offset := current * psDirectoryEntrySize
		if offset+10 > len(payload) {
			break
		}
		next := int(binary.LittleEndian.Uint16(payload[offset+8 : offset+10]))
		if next == 0 || next == 0xFFFF {
			break
		}
		current = next
	}
	return chain
}

func hashPS1LogicalEntry(entry psExtractedEntry) string {
	h := sha256.New()
	io.WriteString(h, entry.ProductCode)
	io.WriteString(h, entry.DisplayTitle)
	io.WriteString(h, entry.RegionCode)
	if entry.PS1 != nil {
		for _, dir := range entry.PS1.DirEntries {
			_, _ = h.Write(dir)
		}
		for _, block := range entry.PS1.Blocks {
			_, _ = h.Write(block)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func extractPS2LogicalEntries(runtimeProfile, cardSlot, filename string, payload []byte) ([]psExtractedEntry, *memoryCardDetails, error) {
	reader, err := newPS2MemoryCardReader(payload)
	if err != nil {
		return nil, nil, err
	}
	rootEntries, err := reader.readRootDirectoryEntries()
	if err != nil {
		return nil, nil, err
	}
	details := parsePS2MemoryCard(payload, cardSlot)
	if details == nil {
		details = &memoryCardDetails{Name: cardSlot}
	}
	detailByDir := map[string]memoryCardEntry{}
	for _, entry := range details.Entries {
		detailByDir[entry.DirectoryName] = entry
	}
	projectionKey := projectionLineKey(runtimeProfile, cardSlot)
	syncKey := syncLineKey("ps2", cardSlot)
	out := make([]psExtractedEntry, 0, len(details.Entries))
	for _, dirEntry := range rootEntries {
		if !ps2ModeIsDir(dirEntry.Mode) || dirEntry.Name == "." || dirEntry.Name == ".." || strings.TrimSpace(dirEntry.Name) == "" {
			continue
		}
		children, err := reader.readDirectoryEntries(dirEntry.FirstCluster, dirEntry.Length)
		if err != nil {
			continue
		}
		iconSysBytes, ok := reader.readDirectoryFile(children, "icon.sys")
		if !ok || len(iconSysBytes) < 964 {
			continue
		}
		iconSys, err := parsePS2IconSys(iconSysBytes[:964])
		if err != nil {
			continue
		}
		title := combinePS2Title(iconSys.TitleLineOne, iconSys.TitleLineTwo)
		if title == "" {
			title = dirEntry.Name
		}
		if isPS2SystemConfigurationEntry(dirEntry.Name, title) {
			continue
		}
		nodes, err := extractPS2DirectoryNodes(reader, children, "")
		if err != nil {
			return nil, nil, err
		}
		productCode := derivePS2DirectoryProductCode(dirEntry.Name)
		regionCode := normalizeRegionCode(regionFromProductCode(productCode))
		if regionCode == regionUnknown {
			regionCode = normalizeRegionCode(detectRegionCode(title))
		}
		displayTitle := canonicalDisplayTitle(title)
		portable := strings.TrimSpace(productCode) != ""
		logicalKey := portablePSLogicalKey("ps2", productCode, displayTitle, regionCode)
		if !portable {
			logicalKey = nonPortablePSLogicalKey("ps2", projectionKey, len(out)+1, displayTitle)
		}
		memoryEntry := detailByDir[dirEntry.Name]
		memoryEntry.LogicalKey = logicalKey
		memoryEntry.Title = displayTitle
		memoryEntry.DirectoryName = dirEntry.Name
		memoryEntry.ProductCode = productCode
		memoryEntry.RegionCode = regionCode
		entry := psExtractedEntry{
			LogicalKey:        logicalKey,
			SystemSlug:        "ps2",
			SyncLineKey:       syncKey,
			ProjectionLineKey: projectionKey,
			ScopeKey:          psScopeKey(portable, syncKey, projectionKey),
			Portable:          portable,
			DisplayTitle:      displayTitle,
			NormalizedTitle:   canonicalTrackTitleKey(displayTitle),
			RegionCode:        regionCode,
			ProductCode:       productCode,
			MemoryEntry:       memoryEntry,
			PS2: &ps2LogicalRevision{
				DirectoryName: dirEntry.Name,
				Nodes:         nodes,
			},
		}
		entry.SHA256 = hashPS2LogicalEntry(entry)
		out = append(out, entry)
	}
	return out, details, nil
}

func extractPS2DirectoryNodes(reader *ps2MemoryCardReader, entries []ps2DirectoryEntry, prefix string) ([]ps2LogicalFSNode, error) {
	nodes := make([]ps2LogicalFSNode, 0, len(entries))
	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." || strings.TrimSpace(entry.Name) == "" {
			continue
		}
		path := entry.Name
		if prefix != "" {
			path = prefix + "/" + entry.Name
		}
		switch {
		case ps2ModeIsDir(entry.Mode):
			children, err := reader.readDirectoryEntries(entry.FirstCluster, entry.Length)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, ps2LogicalFSNode{Path: path, Mode: entry.Mode, Directory: true})
			nested, err := extractPS2DirectoryNodes(reader, children, path)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, nested...)
		case ps2ModeIsFile(entry.Mode):
			data, err := reader.readFile(entry.FirstCluster, int(entry.Length))
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, ps2LogicalFSNode{Path: path, Mode: entry.Mode, Data: data})
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Directory == nodes[j].Directory {
			return nodes[i].Path < nodes[j].Path
		}
		return nodes[i].Directory
	})
	return nodes, nil
}

func hashPS2LogicalEntry(entry psExtractedEntry) string {
	h := sha256.New()
	io.WriteString(h, entry.ProductCode)
	io.WriteString(h, entry.DisplayTitle)
	io.WriteString(h, entry.RegionCode)
	if entry.PS2 != nil {
		io.WriteString(h, entry.PS2.DirectoryName)
		for _, node := range entry.PS2.Nodes {
			io.WriteString(h, node.Path)
			buf := make([]byte, 2)
			binary.LittleEndian.PutUint16(buf, node.Mode)
			_, _ = h.Write(buf)
			if node.Directory {
				_, _ = h.Write([]byte{1})
				continue
			}
			_, _ = h.Write(node.Data)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *playStationStore) rebuildProjectionLinesLocked(systemSlug, cardSlot, originProfile, sourceImportID, fallbackCardName string) ([]psBuiltProjection, error) {
	profiles := supportedProjectionProfiles(systemSlug)
	built := make([]psBuiltProjection, 0, len(profiles))
	for _, profile := range profiles {
		lineKey := projectionLineKey(profile, cardSlot)
		line := s.state.ProjectionLines[lineKey]
		line.Key = lineKey
		line.SystemSlug = systemSlug
		line.RuntimeProfile = profile
		line.CardSlot = cardSlot
		line.SyncLineKey = syncLineKey(systemSlug, cardSlot)
		line.UpdatedAt = time.Now().UTC()
		s.state.ProjectionLines[lineKey] = line

		active := s.activeLogicalSavesForLineLocked(systemSlug, line.SyncLineKey, lineKey)
		filename := s.projectionFilenameForLineLocked(profile, systemSlug, cardSlot, originProfile)
		payload, cardDetails, portable, err := buildProjectionPayload(systemSlug, profile, cardSlot, filename, active)
		if err != nil {
			return nil, err
		}
		sha := sha256.Sum256(payload)
		shaHex := hex.EncodeToString(sha[:])
		projectionID := "ps-projection-" + time.Now().UTC().Format("20060102150405.000000000") + "-" + shaHex[:12]
		payloadPath := filepath.Join(s.root, "projections", projectionID, "card.bin")
		if err := os.MkdirAll(filepath.Dir(payloadPath), 0o755); err != nil {
			return nil, fmt.Errorf("create projection dir: %w", err)
		}
		if err := writeFileAtomic(payloadPath, payload, 0o644); err != nil {
			return nil, fmt.Errorf("write projection payload: %w", err)
		}
		manifest := make([]psManifestEntry, 0, len(active))
		for _, logical := range active {
			manifest = append(manifest, psManifestEntry{
				LogicalKey:        logical.Key,
				RevisionID:        logical.LatestRevisionID,
				ScopeKey:          psScopeKey(logical.Portable, logical.SyncLineKey, logical.ProjectionLineKey),
				DisplayTitle:      logical.DisplayTitle,
				RegionCode:        logical.RegionCode,
				ProductCode:       logical.ProductCode,
				Portable:          logical.Portable,
				ProjectionLineKey: logical.ProjectionLineKey,
				SyncLineKey:       logical.SyncLineKey,
			})
		}
		projection := psProjection{
			ID:                projectionID,
			ProjectionLineKey: lineKey,
			SystemSlug:        systemSlug,
			RuntimeProfile:    profile,
			CardSlot:          cardSlot,
			Filename:          filename,
			SHA256:            shaHex,
			PayloadPath:       payloadPath,
			CreatedAt:         time.Now().UTC(),
			SourceImportID:    sourceImportID,
			Portable:          portable,
			Manifest:          manifest,
			MemoryCard:        cardDetails,
		}
		s.state.Projections[projectionID] = projection
		line.LatestProjectionID = projectionID
		s.state.ProjectionLines[lineKey] = line
		built = append(built, psBuiltProjection{
			ProjectionID:      projectionID,
			ProjectionLineKey: lineKey,
			SystemSlug:        systemSlug,
			RuntimeProfile:    profile,
			CardSlot:          cardSlot,
			Filename:          filename,
			Payload:           payload,
			MemoryCard:        cardDetails,
			SourceImportID:    sourceImportID,
			Portable:          portable,
		})
	}
	if len(built) == 0 && fallbackCardName != "" {
		_ = fallbackCardName
	}
	return built, nil
}

func (s *playStationStore) projectionFilenameForLineLocked(runtimeProfile, systemSlug, cardSlot, originProfile string) string {
	lineKey := projectionLineKey(runtimeProfile, cardSlot)
	line := s.state.ProjectionLines[lineKey]
	if line.LatestProjectionID != "" {
		if projection, ok := s.state.Projections[line.LatestProjectionID]; ok && strings.TrimSpace(projection.Filename) != "" {
			return projection.Filename
		}
	}
	for _, importArtifact := range s.state.Imports {
		if importArtifact.RuntimeProfile == runtimeProfile && importArtifact.CardSlot == cardSlot && strings.TrimSpace(importArtifact.Filename) != "" {
			return safeFilename(importArtifact.Filename)
		}
	}
	switch canonicalSegment(systemSlug, "") {
	case "psx":
		return strings.ReplaceAll(strings.ToLower(cardSlot), " ", "_") + ".mcr"
	case "ps2":
		return strings.ReplaceAll(strings.ToLower(cardSlot), " ", "_") + ".ps2"
	default:
		return strings.ReplaceAll(strings.ToLower(cardSlot), " ", "_") + ".bin"
	}
}

func (s *playStationStore) activeLogicalSavesForLineLocked(systemSlug, syncKey, projectionLineKey string) []psLogicalSave {
	active := make([]psLogicalSave, 0)
	for _, logical := range s.state.LogicalSaves {
		if logical.SystemSlug != systemSlug {
			continue
		}
		scopeKey := logical.ProjectionLineKey
		if logical.Portable {
			scopeKey = logical.SyncLineKey
			if scopeKey != syncKey {
				continue
			}
		} else if scopeKey != projectionLineKey {
			continue
		}
		if _, tombstoned := s.state.Tombstones[psTombstoneKey(scopeKey, logical.Key)]; tombstoned {
			continue
		}
		active = append(active, logical)
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].ProductCode == active[j].ProductCode {
			return active[i].DisplayTitle < active[j].DisplayTitle
		}
		return active[i].ProductCode < active[j].ProductCode
	})
	return active
}

func buildProjectionPayload(systemSlug, runtimeProfile, cardSlot, filename string, saves []psLogicalSave) ([]byte, *memoryCardDetails, bool, error) {
	switch canonicalSegment(systemSlug, "") {
	case "psx":
		return buildPS1Projection(runtimeProfile, cardSlot, filename, saves)
	case "ps2":
		return buildPS2Projection(runtimeProfile, cardSlot, filename, saves)
	default:
		return nil, nil, false, fmt.Errorf("unsupported projection system %q", systemSlug)
	}
}

func buildPS1Projection(runtimeProfile, cardSlot, filename string, saves []psLogicalSave) ([]byte, *memoryCardDetails, bool, error) {
	raw := blankPS1CardTemplate()
	entries := make([]memoryCardEntry, 0, len(saves))
	nextFreeSlot := 1
	allPortable := true
	for _, logical := range saves {
		rev, ok := logical.latestRevision()
		if !ok || rev.PS1 == nil || len(rev.PS1.Blocks) == 0 {
			continue
		}
		blockCount := len(rev.PS1.Blocks)
		if nextFreeSlot+blockCount-1 > psDirectoryEntries {
			break
		}
		for i := 0; i < blockCount; i++ {
			slot := nextFreeSlot + i
			blockOffset := slot * psMemoryCardBlockSize
			copy(raw[blockOffset:blockOffset+psMemoryCardBlockSize], rev.PS1.Blocks[i])
			var dir []byte
			if i < len(rev.PS1.DirEntries) {
				dir = append([]byte(nil), rev.PS1.DirEntries[i]...)
			} else {
				dir = make([]byte, psDirectoryEntrySize)
			}
			if len(dir) < psDirectoryEntrySize {
				padded := make([]byte, psDirectoryEntrySize)
				copy(padded, dir)
				dir = padded
			}
			switch {
			case blockCount == 1:
				dir[0] = ps1DirectoryStateFirst
			case i == 0:
				dir[0] = ps1DirectoryStateFirst
			case i == blockCount-1:
				dir[0] = ps1DirectoryStateLast
			default:
				dir[0] = ps1DirectoryStateMid
			}
			next := uint16(0xFFFF)
			if i < blockCount-1 {
				next = uint16(slot + 1)
			}
			binary.LittleEndian.PutUint16(dir[8:10], next)
			updatePS1DirectoryChecksum(dir)
			dirOffset := slot * psDirectoryEntrySize
			copy(raw[dirOffset:dirOffset+psDirectoryEntrySize], dir)
		}
		entries = append(entries, memoryCardEntry{
			LogicalKey:      logical.Key,
			Title:           rev.MemoryEntry.Title,
			Slot:            nextFreeSlot,
			Blocks:          blockCount,
			ProductCode:     rev.MemoryEntry.ProductCode,
			RegionCode:      rev.MemoryEntry.RegionCode,
			IconDataURL:     rev.MemoryEntry.IconDataURL,
			SizeBytes:       psLogicalSaveLatestSize(logical),
			SaveCount:       len(logical.Revisions),
			LatestVersion:   len(logical.Revisions),
			LatestSizeBytes: psLogicalSaveLatestSize(logical),
			TotalSizeBytes:  psLogicalSaveTotalSize(logical),
			LatestCreatedAt: rev.CreatedAt.Format(time.RFC3339Nano),
		})
		if !logical.Portable {
			allPortable = false
		}
		nextFreeSlot += blockCount
	}
	for slot := nextFreeSlot; slot <= psDirectoryEntries; slot++ {
		dirOffset := slot * psDirectoryEntrySize
		dir := make([]byte, psDirectoryEntrySize)
		dir[0] = ps1DirectoryStateFree
		updatePS1DirectoryChecksum(dir)
		copy(raw[dirOffset:dirOffset+psDirectoryEntrySize], dir)
		blockOffset := slot * psMemoryCardBlockSize
		for i := 0; i < psMemoryCardBlockSize; i++ {
			raw[blockOffset+i] = 0
		}
	}
	wrapped := wrapPS1ProjectionPayload(filename, raw)
	card := parsePS1MemoryCard(wrapped, filename, cardSlot)
	if card == nil {
		card = &memoryCardDetails{Name: cardSlot, Entries: entries}
	} else {
		card.Entries = mergeProjectionEntryStats(card.Entries, entries)
	}
	return wrapped, card, allPortable, nil
}

func blankPS1CardTemplate() []byte {
	payload := make([]byte, ps1MemoryCardTotalSize)
	copy(payload[:2], []byte("MC"))
	for slot := 1; slot <= psDirectoryEntries; slot++ {
		offset := slot * psDirectoryEntrySize
		payload[offset] = ps1DirectoryStateFree
		updatePS1DirectoryChecksum(payload[offset : offset+psDirectoryEntrySize])
	}
	return payload
}

func updatePS1DirectoryChecksum(entry []byte) {
	if len(entry) < psDirectoryEntrySize {
		return
	}
	checksum := byte(0)
	for i := 0; i < psDirectoryEntrySize-1; i++ {
		checksum ^= entry[i]
	}
	entry[psDirectoryEntrySize-1] = checksum
}

func wrapPS1ProjectionPayload(filename string, raw []byte) []byte {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	switch ext {
	case "gme":
		out := make([]byte, ps1DexDriveHeaderSize+len(raw))
		copy(out[:12], []byte("123-456-STD"))
		copy(out[ps1DexDriveHeaderSize:], raw)
		return out
	case "vmp":
		out := make([]byte, ps1PSPVMPHeaderSize+len(raw))
		copy(out[0x00:0x04], []byte("VMP\x00"))
		copy(out[ps1PSPVMPHeaderSize:], raw)
		return out
	default:
		return raw
	}
}

func buildPS2Projection(runtimeProfile, cardSlot, filename string, saves []psLogicalSave) ([]byte, *memoryCardDetails, bool, error) {
	template, err := loadPS2ProjectionTemplate()
	if err != nil {
		return nil, nil, false, err
	}
	payload := append([]byte(nil), template...)
	pageSize := int(binary.LittleEndian.Uint16(payload[40:42]))
	pagesPerCluster := int(binary.LittleEndian.Uint16(payload[42:44]))
	clusterSize := pageSize * pagesPerCluster
	allocOffset := int(binary.LittleEndian.Uint32(payload[52:56]))
	allocEnd := int(binary.LittleEndian.Uint32(payload[56:60]))
	entriesPerCluster := clusterSize / 4
	if clusterSize <= 0 || allocOffset <= 0 || allocEnd <= 0 || entriesPerCluster <= 0 {
		return nil, nil, false, fmt.Errorf("invalid PS2 projection template geometry")
	}

	root := &ps2FSDir{name: "", root: true}
	entries := make([]memoryCardEntry, 0, len(saves))
	allPortable := true
	for _, logical := range saves {
		rev, ok := logical.latestRevision()
		if !ok || rev.PS2 == nil {
			continue
		}
		if !logical.Portable {
			allPortable = false
		}
		dirName := strings.TrimSpace(rev.PS2.DirectoryName)
		if dirName == "" {
			dirName = sanitizePS2DirectoryName(firstNonEmpty(logical.ProductCode, logical.DisplayTitle))
		}
		root.addRevision(dirName, rev)
		entries = append(entries, memoryCardEntry{
			LogicalKey:      logical.Key,
			Title:           rev.MemoryEntry.Title,
			Slot:            rev.MemoryEntry.Slot,
			Blocks:          rev.MemoryEntry.Blocks,
			ProductCode:     rev.MemoryEntry.ProductCode,
			RegionCode:      rev.MemoryEntry.RegionCode,
			DirectoryName:   dirName,
			IconDataURL:     rev.MemoryEntry.IconDataURL,
			SizeBytes:       psLogicalSaveLatestSize(logical),
			SaveCount:       len(logical.Revisions),
			LatestVersion:   len(logical.Revisions),
			LatestSizeBytes: psLogicalSaveLatestSize(logical),
			TotalSizeBytes:  psLogicalSaveTotalSize(logical),
			LatestCreatedAt: rev.CreatedAt.Format(time.RFC3339Nano),
		})
	}

	fat := make([]uint32, allocEnd)
	allocator := &ps2ProjectionAllocator{clusterSize: clusterSize, nextCluster: 1, limit: allocEnd}
	if err := allocatePS2Tree(root, 0, allocator, fat); err != nil {
		return nil, nil, false, err
	}
	if err := writePS2Tree(payload, allocOffset, clusterSize, root, fat); err != nil {
		return nil, nil, false, err
	}
	if err := writePS2FAT(payload, clusterSize, entriesPerCluster, fat); err != nil {
		return nil, nil, false, err
	}
	card := parsePS2MemoryCard(payload, cardSlot)
	if card == nil {
		card = &memoryCardDetails{Name: cardSlot, Entries: entries}
	} else {
		card.Entries = mergeProjectionEntryStats(card.Entries, entries)
	}
	return payload, card, allPortable, nil
}

type ps2FSDir struct {
	name         string
	root         bool
	parentIndex  uint32
	entryIndex   uint32
	childrenDirs map[string]*ps2FSDir
	files        []*ps2FSFile
	firstCluster uint32
	clusterCount int
	chain        []int
	entryCount   uint32
	serialized   []byte
}

type ps2FSFile struct {
	name         string
	mode         uint16
	data         []byte
	firstCluster uint32
	clusterCount int
}

type ps2ProjectionAllocator struct {
	clusterSize int
	nextCluster int
	limit       int
}

func (d *ps2FSDir) ensureChild(name string) *ps2FSDir {
	if d.childrenDirs == nil {
		d.childrenDirs = map[string]*ps2FSDir{}
	}
	if child, ok := d.childrenDirs[name]; ok {
		return child
	}
	child := &ps2FSDir{name: name}
	d.childrenDirs[name] = child
	return child
}

func (d *ps2FSDir) addRevision(dirName string, rev psLogicalSaveRevision) {
	top := d.ensureChild(dirName)
	for _, node := range rev.PS2.Nodes {
		parts := splitPS2Path(node.Path)
		if len(parts) == 0 {
			continue
		}
		current := top
		for i, part := range parts {
			if i == len(parts)-1 {
				if node.Directory {
					current.ensureChild(part)
				} else {
					current.files = append(current.files, &ps2FSFile{name: part, mode: ps2ModeFileRegular, data: append([]byte(nil), node.Data...)})
				}
				continue
			}
			current = current.ensureChild(part)
		}
	}
}

func splitPS2Path(path string) []string {
	parts := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		out = append(out, part)
	}
	return out
}

func allocatePS2Tree(dir *ps2FSDir, parentIndex uint32, allocator *ps2ProjectionAllocator, fat []uint32) error {
	dir.parentIndex = parentIndex
	sortedDirs := dir.sortedDirs()
	sortedFiles := dir.sortedFiles()
	for i, child := range sortedDirs {
		child.entryIndex = uint32(i + 2)
		if err := allocatePS2Tree(child, child.entryIndex, allocator, fat); err != nil {
			return err
		}
	}
	for _, file := range sortedFiles {
		count := clustersForLength(len(file.data), allocator.clusterSize)
		first, err := allocator.reserve(count)
		if err != nil {
			return err
		}
		file.firstCluster = uint32(first)
		file.clusterCount = count
		setFATChain(fat, first, count)
	}
	entries := make([][]byte, 0, 2+len(sortedDirs)+len(sortedFiles))
	if dir.root {
		entries = append(entries, encodePS2DirectoryEntry(ps2ModeDirectory, uint32(2+len(sortedDirs)+len(sortedFiles)), 0, 0, "."))
		entries = append(entries, encodePS2DirectoryEntry(ps2ModeRootParent, 0, 0, 0, ".."))
	} else {
		entries = append(entries, encodePS2DirectoryEntry(ps2ModeDirectory, 0, 0, dir.parentIndex, "."))
		entries = append(entries, encodePS2DirectoryEntry(ps2ModeDirectory, 0, 0, 0, ".."))
	}
	for _, child := range sortedDirs {
		entries = append(entries, encodePS2DirectoryEntry(ps2ModeDirectory, child.entryCount, child.firstCluster, 0, child.name))
	}
	for _, file := range sortedFiles {
		entries = append(entries, encodePS2DirectoryEntry(ps2ModeFileRegular, uint32(len(file.data)), file.firstCluster, 0, file.name))
	}
	dir.serialized = bytes.Join(entries, nil)
	dir.entryCount = uint32(len(entries))
	count := clustersForLength(len(dir.serialized), allocator.clusterSize)
	if dir.root {
		dir.firstCluster = 0
		dir.clusterCount = count
		dir.chain = []int{0}
		if count > 1 {
			extraStart, err := allocator.reserve(count - 1)
			if err != nil {
				return err
			}
			for i := 0; i < count-1; i++ {
				dir.chain = append(dir.chain, extraStart+i)
			}
		}
		setFATChainForExplicitSequence(fat, dir.chain)
		return nil
	}
	first, err := allocator.reserve(count)
	if err != nil {
		return err
	}
	dir.firstCluster = uint32(first)
	dir.clusterCount = count
	setFATChain(fat, first, count)
	return nil
}

func (d *ps2FSDir) sortedDirs() []*ps2FSDir {
	out := make([]*ps2FSDir, 0, len(d.childrenDirs))
	for _, child := range d.childrenDirs {
		out = append(out, child)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func (d *ps2FSDir) sortedFiles() []*ps2FSFile {
	out := append([]*ps2FSFile(nil), d.files...)
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func clustersForLength(length, clusterSize int) int {
	if length <= 0 {
		return 1
	}
	count := length / clusterSize
	if length%clusterSize != 0 {
		count++
	}
	if count <= 0 {
		count = 1
	}
	return count
}

func (a *ps2ProjectionAllocator) reserve(count int) (int, error) {
	if count <= 0 {
		count = 1
	}
	first := a.nextCluster
	if first+count > a.limit {
		return 0, fmt.Errorf("PS2 projection exceeds available card space")
	}
	a.nextCluster += count
	return first, nil
}

func (a *ps2ProjectionAllocator) bump(count int) {
	if count <= 0 {
		return
	}
	a.nextCluster += count
}

func setFATChainForExplicitSequence(fat []uint32, chain []int) {
	if len(fat) == 0 || len(chain) == 0 {
		return
	}
	for i := 0; i < len(chain); i++ {
		index := chain[i]
		if index < 0 || index >= len(fat) {
			continue
		}
		if i == len(chain)-1 {
			fat[index] = ps2FATChainEnd
			continue
		}
		fat[index] = ps2FATAllocatedBit | uint32(chain[i+1])
	}
}

func setFATChain(fat []uint32, first, count int) {
	for i := 0; i < count; i++ {
		index := first + i
		if index < 0 || index >= len(fat) {
			continue
		}
		if i == count-1 {
			fat[index] = ps2FATChainEnd
			continue
		}
		fat[index] = ps2FATAllocatedBit | uint32(index+1)
	}
}

func writePS2Tree(payload []byte, allocOffset, clusterSize int, root *ps2FSDir, fat []uint32) error {
	zeroAllocatableArea(payload, allocOffset, clusterSize, len(fat))
	if err := writePS2Directory(payload, allocOffset, clusterSize, root); err != nil {
		return err
	}
	for _, child := range root.sortedDirs() {
		if err := writePS2DirectoryRecursive(payload, allocOffset, clusterSize, child); err != nil {
			return err
		}
	}
	return nil
}

func writePS2DirectoryRecursive(payload []byte, allocOffset, clusterSize int, dir *ps2FSDir) error {
	if err := writePS2Directory(payload, allocOffset, clusterSize, dir); err != nil {
		return err
	}
	for _, child := range dir.sortedDirs() {
		if err := writePS2DirectoryRecursive(payload, allocOffset, clusterSize, child); err != nil {
			return err
		}
	}
	for _, file := range dir.sortedFiles() {
		if err := writePS2Clusters(payload, allocOffset, clusterSize, int(file.firstCluster), file.clusterCount, file.data); err != nil {
			return err
		}
	}
	return nil
}

func writePS2Directory(payload []byte, allocOffset, clusterSize int, dir *ps2FSDir) error {
	if dir.root && len(dir.chain) > 0 {
		return writePS2ClusterChain(payload, allocOffset, clusterSize, dir.chain, dir.serialized)
	}
	return writePS2Clusters(payload, allocOffset, clusterSize, int(dir.firstCluster), dir.clusterCount, dir.serialized)
}

func writePS2ClusterChain(payload []byte, allocOffset, clusterSize int, chain []int, data []byte) error {
	for i, cluster := range chain {
		start := (allocOffset + cluster) * clusterSize
		end := start + clusterSize
		if start < 0 || end > len(payload) {
			return fmt.Errorf("PS2 projection cluster out of range")
		}
		chunkStart := i * clusterSize
		chunkEnd := chunkStart + clusterSize
		for j := start; j < end; j++ {
			payload[j] = 0
		}
		if chunkStart >= len(data) {
			continue
		}
		if chunkEnd > len(data) {
			chunkEnd = len(data)
		}
		copy(payload[start:start+(chunkEnd-chunkStart)], data[chunkStart:chunkEnd])
	}
	return nil
}

func writePS2Clusters(payload []byte, allocOffset, clusterSize, firstCluster, clusterCount int, data []byte) error {
	for i := 0; i < clusterCount; i++ {
		start := (allocOffset + firstCluster + i) * clusterSize
		end := start + clusterSize
		if start < 0 || end > len(payload) {
			return fmt.Errorf("PS2 projection cluster out of range")
		}
		chunkStart := i * clusterSize
		chunkEnd := chunkStart + clusterSize
		if chunkStart >= len(data) {
			for j := start; j < end; j++ {
				payload[j] = 0
			}
			continue
		}
		if chunkEnd > len(data) {
			chunkEnd = len(data)
		}
		copy(payload[start:end], make([]byte, clusterSize))
		copy(payload[start:start+(chunkEnd-chunkStart)], data[chunkStart:chunkEnd])
	}
	return nil
}

func zeroAllocatableArea(payload []byte, allocOffset, clusterSize, allocEnd int) {
	start := allocOffset * clusterSize
	end := (allocOffset + allocEnd) * clusterSize
	if start < 0 || start >= len(payload) {
		return
	}
	if end > len(payload) {
		end = len(payload)
	}
	for i := start; i < end; i++ {
		payload[i] = 0
	}
}

func writePS2FAT(payload []byte, clusterSize, entriesPerCluster int, fat []uint32) error {
	indirectCluster := binary.LittleEndian.Uint32(payload[80:84])
	if indirectCluster == 0 {
		return fmt.Errorf("PS2 template missing indirect FAT cluster")
	}
	indirectOffset := int(indirectCluster) * clusterSize
	if indirectOffset+clusterSize > len(payload) {
		return fmt.Errorf("PS2 template indirect FAT cluster out of range")
	}
	indirect := uint32SliceFromBytes(payload[indirectOffset : indirectOffset+clusterSize])
	fatClusterIndex := 0
	for i := 0; i < len(fat); i += entriesPerCluster {
		if fatClusterIndex >= len(indirect) {
			return fmt.Errorf("PS2 template FAT cluster table too small")
		}
		cluster := indirect[fatClusterIndex]
		if cluster == 0 || cluster == 0xFFFFFFFF {
			return fmt.Errorf("PS2 template FAT cluster %d missing", fatClusterIndex)
		}
		start := int(cluster) * clusterSize
		end := start + clusterSize
		if start < 0 || end > len(payload) {
			return fmt.Errorf("PS2 FAT cluster out of range")
		}
		count := entriesPerCluster
		if i+count > len(fat) {
			count = len(fat) - i
		}
		clusterBytes := make([]byte, clusterSize)
		for j := 0; j < count; j++ {
			binary.LittleEndian.PutUint32(clusterBytes[j*4:j*4+4], fat[i+j])
		}
		copy(payload[start:end], clusterBytes)
		fatClusterIndex++
	}
	return nil
}

func encodePS2DirectoryEntry(mode uint16, length, firstCluster, parentEntry uint32, name string) []byte {
	entry := make([]byte, ps2DirectoryEntSize)
	binary.LittleEndian.PutUint16(entry[0:2], mode)
	binary.LittleEndian.PutUint32(entry[4:8], length)
	binary.LittleEndian.PutUint32(entry[16:20], firstCluster)
	binary.LittleEndian.PutUint32(entry[20:24], parentEntry)
	copy(entry[64:], []byte(name))
	return entry
}

func sanitizePS2DirectoryName(raw string) string {
	clean := strings.ToUpper(strings.TrimSpace(raw))
	clean = strings.ReplaceAll(clean, " ", "_")
	clean = strings.ReplaceAll(clean, "/", "_")
	clean = strings.ReplaceAll(clean, "\\", "_")
	clean = strings.ReplaceAll(clean, ":", "_")
	if clean == "" {
		clean = "RSM_SAVE"
	}
	if len(clean) > 31 {
		clean = clean[:31]
	}
	return clean
}

func loadPS2ProjectionTemplate() ([]byte, error) {
	ps2ProjectionTemplateOnce.Do(func() {
		zr, err := gzip.NewReader(bytes.NewReader(ps2ProjectionTemplateGzip))
		if err != nil {
			ps2ProjectionTemplateErr = err
			return
		}
		defer zr.Close()
		ps2ProjectionTemplateData, ps2ProjectionTemplateErr = io.ReadAll(zr)
	})
	if ps2ProjectionTemplateErr != nil {
		return nil, ps2ProjectionTemplateErr
	}
	return append([]byte(nil), ps2ProjectionTemplateData...), nil
}

func (s *playStationStore) attachProjectionSaveRecord(projectionID, saveRecordID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	projection, ok := s.state.Projections[strings.TrimSpace(projectionID)]
	if !ok {
		return fmt.Errorf("projection %s not found", projectionID)
	}
	projection.SaveRecordID = strings.TrimSpace(saveRecordID)
	s.state.Projections[projection.ID] = projection
	return s.persistLocked()
}

func (s *playStationStore) latestProjectionSaveRecord(runtimeProfile, cardSlot string) (string, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	line, ok := s.state.ProjectionLines[projectionLineKey(runtimeProfile, cardSlot)]
	if !ok || strings.TrimSpace(line.LatestProjectionID) == "" {
		return "", "", false
	}
	projection, ok := s.state.Projections[line.LatestProjectionID]
	if !ok || strings.TrimSpace(projection.SaveRecordID) == "" {
		return "", "", false
	}
	return projection.SaveRecordID, projection.SHA256, true
}

func (s *playStationStore) markProjectionDownloaded(runtimeProfile, cardSlot, fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lineKey := projectionLineKey(runtimeProfile, cardSlot)
	line, ok := s.state.ProjectionLines[lineKey]
	if !ok || strings.TrimSpace(line.LatestProjectionID) == "" {
		return
	}
	deviceKey := deviceLineKey(runtimeProfile, cardSlot, fingerprint)
	deviceLine := s.state.DeviceLines[deviceKey]
	deviceLine.Key = deviceKey
	deviceLine.ProjectionLineKey = lineKey
	deviceLine.Fingerprint = strings.TrimSpace(fingerprint)
	deviceLine.LastDownloadedProjection = line.LatestProjectionID
	deviceLine.UpdatedAt = time.Now().UTC()
	s.state.DeviceLines[deviceKey] = deviceLine
	_ = s.persistLocked()
}

func (s *playStationStore) projectionBySaveRecordID(saveRecordID string) (psProjection, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, projection := range s.state.Projections {
		if projection.SaveRecordID == strings.TrimSpace(saveRecordID) {
			return projection, true
		}
	}
	return psProjection{}, false
}

func (s *playStationStore) projectionForRuntime(runtimeProfile, cardSlot string) (psProjection, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	line, ok := s.state.ProjectionLines[projectionLineKey(runtimeProfile, cardSlot)]
	if !ok || line.LatestProjectionID == "" {
		return psProjection{}, false
	}
	projection, ok := s.state.Projections[line.LatestProjectionID]
	return projection, ok
}

func (logical psLogicalSave) latestRevision() (psLogicalSaveRevision, bool) {
	for _, revision := range logical.Revisions {
		if revision.ID == logical.LatestRevisionID {
			return revision, true
		}
	}
	if len(logical.Revisions) == 0 {
		return psLogicalSaveRevision{}, false
	}
	return logical.Revisions[len(logical.Revisions)-1], true
}

func psLogicalSaveLatestSize(logical psLogicalSave) int {
	latest, ok := logical.latestRevision()
	if !ok {
		return 0
	}
	return psLogicalRevisionSize(latest)
}

func psLogicalSaveTotalSize(logical psLogicalSave) int {
	total := 0
	for _, revision := range logical.Revisions {
		total += psLogicalRevisionSize(revision)
	}
	return total
}

func psLogicalRevisionSize(revision psLogicalSaveRevision) int {
	if revision.MemoryEntry.SizeBytes > 0 {
		return revision.MemoryEntry.SizeBytes
	}
	if revision.PS1 != nil {
		size := 0
		for _, block := range revision.PS1.Blocks {
			size += len(block)
		}
		return size
	}
	if revision.PS2 != nil {
		size := 0
		for _, node := range revision.PS2.Nodes {
			if node.Directory {
				continue
			}
			size += len(node.Data)
		}
		return size
	}
	return 0
}

func mergeProjectionEntryStats(existing []memoryCardEntry, enriched []memoryCardEntry) []memoryCardEntry {
	if len(existing) == 0 {
		return enriched
	}
	indexByKey := make(map[string]memoryCardEntry, len(enriched))
	for _, entry := range enriched {
		indexByKey[memoryCardEntryMergeKey(entry)] = entry
	}
	merged := make([]memoryCardEntry, 0, len(existing))
	for _, entry := range existing {
		if enrichedEntry, ok := indexByKey[memoryCardEntryMergeKey(entry)]; ok {
			merged = append(merged, mergeMemoryCardEntry(entry, enrichedEntry))
			continue
		}
		merged = append(merged, entry)
	}
	return merged
}

func mergeMemoryCardEntry(existing, enriched memoryCardEntry) memoryCardEntry {
	merged := existing
	if strings.TrimSpace(merged.Title) == "" {
		merged.Title = enriched.Title
	}
	if strings.TrimSpace(merged.LogicalKey) == "" {
		merged.LogicalKey = enriched.LogicalKey
	}
	if merged.Slot == 0 {
		merged.Slot = enriched.Slot
	}
	if merged.Blocks == 0 {
		merged.Blocks = enriched.Blocks
	}
	if strings.TrimSpace(merged.ProductCode) == "" {
		merged.ProductCode = enriched.ProductCode
	}
	if strings.TrimSpace(merged.RegionCode) == "" {
		merged.RegionCode = enriched.RegionCode
	}
	if strings.TrimSpace(merged.DirectoryName) == "" {
		merged.DirectoryName = enriched.DirectoryName
	}
	if strings.TrimSpace(merged.IconDataURL) == "" {
		merged.IconDataURL = enriched.IconDataURL
	}
	if merged.SizeBytes == 0 {
		merged.SizeBytes = enriched.SizeBytes
	}
	merged.SaveCount = enriched.SaveCount
	merged.LatestVersion = enriched.LatestVersion
	merged.LatestSizeBytes = enriched.LatestSizeBytes
	merged.TotalSizeBytes = enriched.TotalSizeBytes
	merged.LatestCreatedAt = enriched.LatestCreatedAt
	if enriched.Portable != nil {
		merged.Portable = enriched.Portable
	}
	return merged
}

func memoryCardEntryMergeKey(entry memoryCardEntry) string {
	if directory := strings.TrimSpace(entry.DirectoryName); directory != "" {
		return "dir:" + strings.ToUpper(directory)
	}
	if productCode := strings.TrimSpace(entry.ProductCode); productCode != "" {
		return "product:" + strings.ToUpper(productCode) + "::" + canonicalTrackTitleKey(entry.Title)
	}
	if entry.Slot > 0 {
		return "slot:" + strconv.Itoa(entry.Slot) + "::" + canonicalTrackTitleKey(entry.Title)
	}
	return "title:" + canonicalTrackTitleKey(entry.Title)
}

func hash12(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:12]
}

func helperProjectionIdentity(deviceType, slotName string) (string, string, bool) {
	deviceType = canonicalPlayStationDeviceType(deviceType)
	if !isPlayStationRuntimeDevice(deviceType) {
		return "", "", false
	}
	cardSlot, ok := deriveExplicitMemoryCardName(slotName, slotName)
	if !ok {
		return "", "", false
	}
	profile, _, err := supportedPlayStationRuntimeProfile(deviceType, map[string]saveArtifactKind{
		"mister":    saveArtifactPS1MemoryCard,
		"retroarch": saveArtifactPS1MemoryCard,
		"pcsx2":     saveArtifactPS2MemoryCard,
	}[deviceType])
	if err != nil {
		return "", "", false
	}
	return profile, cardSlot, true
}

func runtimeDeviceTypeFromProfile(runtimeProfile string) string {
	switch strings.ToLower(strings.TrimSpace(runtimeProfile)) {
	case "psx/mister":
		return "mister"
	case "psx/retroarch":
		return "retroarch"
	case "ps2/pcsx2":
		return "pcsx2"
	default:
		return ""
	}
}

func mergePlayStationMetadata(existing any, projection psBuiltProjection) any {
	portable := projection.Portable
	playstationMeta := map[string]any{
		"runtimeProfile": projection.RuntimeProfile,
		"cardSlot":       projection.CardSlot,
		"projectionId":   projection.ProjectionID,
		"sourceImportId": projection.SourceImportID,
		"portable":       portable,
	}
	if existingMap, ok := existing.(map[string]any); ok {
		merged := make(map[string]any, len(existingMap)+1)
		for key, value := range existingMap {
			merged[key] = value
		}
		merged["playstation"] = playstationMeta
		return merged
	}
	return map[string]any{"playstation": playstationMeta}
}

func playStationSummaryFields(summary *saveSummary, projection psBuiltProjection) {
	if summary == nil {
		return
	}
	summary.RuntimeProfile = projection.RuntimeProfile
	summary.CardSlot = projection.CardSlot
	summary.ProjectionID = projection.ProjectionID
	summary.SourceImportID = projection.SourceImportID
	summary.MemoryCard = projection.MemoryCard
	portable := projection.Portable
	summary.Portable = &portable
	if summary.MemoryCard != nil {
		for i := range summary.MemoryCard.Entries {
			entryPortable := projection.Portable
			summary.MemoryCard.Entries[i].Portable = &entryPortable
		}
	}
}

func playStationProjectionInfoFromRecord(record saveRecord) (runtimeProfile, cardSlot, projectionID string, ok bool) {
	projectionID = strings.TrimSpace(record.Summary.ProjectionID)
	runtimeProfile = strings.TrimSpace(record.Summary.RuntimeProfile)
	cardSlot = strings.TrimSpace(record.Summary.CardSlot)
	if projectionID == "" || runtimeProfile == "" || cardSlot == "" {
		return "", "", "", false
	}
	return runtimeProfile, cardSlot, projectionID, true
}
