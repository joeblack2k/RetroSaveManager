package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	gameModuleSchemaVersion = 1
	gameModuleABIVersion    = "rsm-wasm-json-v1"

	gameModuleStatusActive   = "active"
	gameModuleStatusDisabled = "disabled"
	gameModuleStatusDeleted  = "deleted"
	gameModuleStatusError    = "error"

	gameModuleSourceGithub   = "github"
	gameModuleSourceUploaded = "uploaded"

	defaultModuleLibraryRepo = "joeblack2k/RetroSaveManager"
	defaultModuleLibraryRef  = "main"
	defaultModuleLibraryPath = "modules"

	maxGameModuleZipBytes    = 8 << 20
	maxGameModuleFileBytes   = 4 << 20
	maxGameModuleWASMBytes   = 2 << 20
	maxGameModuleOutputBytes = 2 << 20
	maxGameModuleFiles       = 128
	maxGameModuleMemoryPages = 128
)

// gameModuleService owns the runtime module store under SAVE_ROOT/_rsm/modules.
// It stays in package main so modules can plug into save, cheat, and inspection
// flows without exporting a large internal API surface.
type gameModuleService struct {
	saveRoot string
	root     string
	modsRoot string
	status   string
}

// gameModuleManifest is the trusted manifest format inside an .rsmodule.zip.
// Optional src/* files are review material only; the backend runs only the
// declared WASM and declarative YAML packs.
type gameModuleManifest struct {
	ModuleID      string                  `json:"moduleId" yaml:"moduleId"`
	SchemaVersion int                     `json:"schemaVersion" yaml:"schemaVersion"`
	Version       string                  `json:"version" yaml:"version"`
	SystemSlug    string                  `json:"systemSlug" yaml:"systemSlug"`
	GameID        string                  `json:"gameId" yaml:"gameId"`
	Title         string                  `json:"title" yaml:"title"`
	ParserID      string                  `json:"parserId" yaml:"parserId"`
	WASMFile      string                  `json:"wasmFile" yaml:"wasmFile"`
	ABIVersion    string                  `json:"abiVersion" yaml:"abiVersion"`
	CheatPacks    []gameModuleCheatPack   `json:"cheatPacks,omitempty" yaml:"cheatPacks,omitempty"`
	Payload       gameModulePayloadPolicy `json:"payload" yaml:"payload"`
	TitleAliases  []string                `json:"titleAliases,omitempty" yaml:"titleAliases,omitempty"`
	ROMHashes     []string                `json:"romHashes,omitempty" yaml:"romHashes,omitempty"`
}

type gameModuleCheatPack struct {
	Path string `json:"path" yaml:"path"`
}

type gameModulePayloadPolicy struct {
	ExactSizes []int    `json:"exactSizes,omitempty" yaml:"exactSizes,omitempty"`
	Formats    []string `json:"formats,omitempty" yaml:"formats,omitempty"`
}

type gameModuleRecord struct {
	Manifest       gameModuleManifest `json:"manifest"`
	Status         string             `json:"status"`
	Source         string             `json:"source"`
	SourcePath     string             `json:"sourcePath,omitempty"`
	SourceRevision string             `json:"sourceRevision,omitempty"`
	SourceSHA256   string             `json:"sourceSha256,omitempty"`
	ImportedAt     time.Time          `json:"importedAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
	LastSyncedAt   *time.Time         `json:"lastSyncedAt,omitempty"`
	Errors         []string           `json:"errors,omitempty"`
	CheatPackIDs   []string           `json:"cheatPackIds,omitempty"`
}

type gameModuleLibraryConfig struct {
	Repo string `json:"repo"`
	Ref  string `json:"ref"`
	Path string `json:"path"`
}

type gameModuleSyncImported struct {
	Path       string `json:"path"`
	ModuleID   string `json:"moduleId,omitempty"`
	Title      string `json:"title,omitempty"`
	SystemSlug string `json:"systemSlug,omitempty"`
	Status     string `json:"status,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
}

type gameModuleSyncError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type gameModuleLibraryStatus struct {
	Config        gameModuleLibraryConfig  `json:"config"`
	LastSyncedAt  *time.Time               `json:"lastSyncedAt,omitempty"`
	ImportedCount int                      `json:"importedCount"`
	ErrorCount    int                      `json:"errorCount"`
	Imported      []gameModuleSyncImported `json:"imported"`
	Errors        []gameModuleSyncError    `json:"errors"`
}

type gameModuleListResponse struct {
	Success bool                    `json:"success"`
	Modules []gameModuleRecord      `json:"modules"`
	Library gameModuleLibraryStatus `json:"library"`
}

type gameModuleResponse struct {
	Success bool             `json:"success"`
	Module  gameModuleRecord `json:"module"`
}

type gameModuleSyncResponse struct {
	Success bool                    `json:"success"`
	Library gameModuleLibraryStatus `json:"library"`
}

type gameModuleRescanResponse struct {
	Success bool             `json:"success"`
	Result  saveRescanResult `json:"result"`
}

type gameModuleSourceInfo struct {
	Source         string
	SourcePath     string
	SourceRevision string
	SourceSHA256   string
	LastSyncedAt   *time.Time
}

type gameModuleParseRequest struct {
	Payload      []byte         `json:"payload"`
	Filename     string         `json:"filename,omitempty"`
	Format       string         `json:"format,omitempty"`
	SystemSlug   string         `json:"systemSlug,omitempty"`
	DisplayTitle string         `json:"displayTitle,omitempty"`
	ROMSHA1      string         `json:"romSha1,omitempty"`
	ROMMD5       string         `json:"romMd5,omitempty"`
	SlotName     string         `json:"slotName,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type gameModuleParseResponse struct {
	Supported          bool           `json:"supported"`
	ParserLevel        string         `json:"parserLevel,omitempty"`
	ParserID           string         `json:"parserId,omitempty"`
	ValidatedSystem    string         `json:"validatedSystem,omitempty"`
	ValidatedGameID    string         `json:"validatedGameId,omitempty"`
	ValidatedGameTitle string         `json:"validatedGameTitle,omitempty"`
	TrustLevel         string         `json:"trustLevel,omitempty"`
	Evidence           []string       `json:"evidence,omitempty"`
	Warnings           []string       `json:"warnings,omitempty"`
	SlotCount          int            `json:"slotCount,omitempty"`
	ActiveSlotIndexes  []int          `json:"activeSlotIndexes,omitempty"`
	ChecksumValid      *bool          `json:"checksumValid,omitempty"`
	SemanticFields     map[string]any `json:"semanticFields,omitempty"`
}

type gameModuleCheatRequest struct {
	Payload     []byte          `json:"payload"`
	Pack        cheatPack       `json:"pack"`
	LogicalKey  string          `json:"logicalKey,omitempty"`
	SaturnEntry string          `json:"saturnEntry,omitempty"`
	SlotID      string          `json:"slotId,omitempty"`
	Updates     map[string]any  `json:"updates,omitempty"`
	Summary     saveSummary     `json:"summary"`
	Inspection  *saveInspection `json:"inspection,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
}

type gameModuleCheatApplyResponse struct {
	Payload        []byte         `json:"payload"`
	Changed        map[string]any `json:"changed,omitempty"`
	IntegritySteps []string       `json:"integritySteps,omitempty"`
}

// newGameModuleService creates the module store without moving or rewriting user saves.
func newGameModuleService(saveRoot string) (*gameModuleService, error) {
	root, err := safeJoinUnderRoot(saveRoot, "_rsm", "modules")
	if err != nil {
		return nil, err
	}
	modsRoot := filepath.Join(root, "installed")
	for _, dir := range []string{root, modsRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return &gameModuleService{
		saveRoot: saveRoot,
		root:     root,
		modsRoot: modsRoot,
		status:   filepath.Join(root, "library-status.json"),
	}, nil
}

func (s *gameModuleService) listModules() ([]gameModuleRecord, error) {
	if s == nil {
		return nil, nil
	}
	items := make([]gameModuleRecord, 0, 8)
	err := filepath.WalkDir(s.modsRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "module.json" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var record gameModuleRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return err
		}
		items = append(items, record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Manifest.SystemSlug == items[j].Manifest.SystemSlug {
			return items[i].Manifest.Title < items[j].Manifest.Title
		}
		return items[i].Manifest.SystemSlug < items[j].Manifest.SystemSlug
	})
	return items, nil
}

func (s *gameModuleService) moduleDir(moduleID string) string {
	return filepath.Join(s.modsRoot, canonicalGameModuleID(moduleID))
}

func (s *gameModuleService) moduleRecordPath(moduleID string) string {
	return filepath.Join(s.moduleDir(moduleID), "module.json")
}

func (s *gameModuleService) readModule(moduleID string) (gameModuleRecord, error) {
	data, err := os.ReadFile(s.moduleRecordPath(moduleID))
	if err != nil {
		return gameModuleRecord{}, fmt.Errorf("module %q not found", moduleID)
	}
	var record gameModuleRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return gameModuleRecord{}, err
	}
	return record, nil
}

// writeModuleRecord is the one persistence point for module metadata so imports
// and enable/disable state stay consistent.
func (s *gameModuleService) writeModuleRecord(record gameModuleRecord) error {
	moduleID := canonicalGameModuleID(record.Manifest.ModuleID)
	if moduleID == "" {
		return errors.New("moduleId is required")
	}
	record.Manifest.ModuleID = moduleID
	if record.ImportedAt.IsZero() {
		record.ImportedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.moduleDir(moduleID), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(s.moduleRecordPath(moduleID), data, 0o644)
}

// metadataMap converts typed Go metadata into a plain JSON object for the WASM ABI.
func metadataMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func canonicalGameModuleID(raw string) string {
	return canonicalSegment(raw, "")
}

func (s *gameModuleService) setStatus(moduleID, status string) (gameModuleRecord, error) {
	record, err := s.readModule(moduleID)
	if err != nil {
		return gameModuleRecord{}, err
	}
	record.Status = status
	if err := s.writeModuleRecord(record); err != nil {
		return gameModuleRecord{}, err
	}
	return record, nil
}

func (s *gameModuleService) setStatusByPackID(packID, status string) (gameModuleRecord, bool, error) {
	modules, err := s.listModules()
	if err != nil {
		return gameModuleRecord{}, false, err
	}
	target := canonicalCheatPackID(packID)
	for _, record := range modules {
		for _, id := range record.CheatPackIDs {
			if canonicalCheatPackID(id) == target {
				updated, err := s.setStatus(record.Manifest.ModuleID, status)
				return updated, true, err
			}
		}
	}
	return gameModuleRecord{}, false, nil
}
