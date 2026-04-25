package main

import (
	"encoding/json"
	"time"
)

type cheatCapability struct {
	Supported      bool   `json:"supported" yaml:"supported"`
	AvailableCount int    `json:"availableCount,omitempty" yaml:"availableCount,omitempty"`
	EditorID       string `json:"editorId,omitempty" yaml:"editorId,omitempty"`
	AdapterID      string `json:"adapterId,omitempty" yaml:"adapterId,omitempty"`
	PackID         string `json:"packId,omitempty" yaml:"packId,omitempty"`
}

type cheatOption struct {
	ID    string `json:"id" yaml:"id"`
	Label string `json:"label" yaml:"label"`
}

type cheatBitOption struct {
	ID    string `json:"id" yaml:"id"`
	Bit   int    `json:"bit" yaml:"bit"`
	Label string `json:"label" yaml:"label"`
}

type cheatOperation struct {
	Kind   string `json:"kind" yaml:"kind"`
	Flag   string `json:"flag,omitempty" yaml:"flag,omitempty"`
	Course string `json:"course,omitempty" yaml:"course,omitempty"`
	Mode   string `json:"mode,omitempty" yaml:"mode,omitempty"`
	Cup    string `json:"cup,omitempty" yaml:"cup,omitempty"`
	Field  string `json:"field,omitempty" yaml:"field,omitempty"`
}

type cheatField struct {
	ID          string           `json:"id" yaml:"id"`
	Ref         string           `json:"ref,omitempty" yaml:"ref,omitempty"`
	Label       string           `json:"label" yaml:"label"`
	Description string           `json:"description,omitempty" yaml:"description,omitempty"`
	Type        string           `json:"type" yaml:"type"`
	Min         *int             `json:"min,omitempty" yaml:"min,omitempty"`
	Max         *int             `json:"max,omitempty" yaml:"max,omitempty"`
	Step        *int             `json:"step,omitempty" yaml:"step,omitempty"`
	Options     []cheatOption    `json:"options,omitempty" yaml:"options,omitempty"`
	Bits        []cheatBitOption `json:"bits,omitempty" yaml:"bits,omitempty"`
	BitLabels   []string         `json:"-" yaml:"bitLabels,omitempty"`
	Op          cheatOperation   `json:"op" yaml:"op"`
}

type cheatSection struct {
	ID     string       `json:"id" yaml:"id"`
	Title  string       `json:"title" yaml:"title"`
	Fields []cheatField `json:"fields" yaml:"fields"`
}

type cheatPreset struct {
	ID          string         `json:"id" yaml:"id"`
	Label       string         `json:"label" yaml:"label"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Updates     map[string]any `json:"updates,omitempty" yaml:"updates,omitempty"`
}

type cheatSelector struct {
	ID      string        `json:"id" yaml:"id"`
	Label   string        `json:"label" yaml:"label"`
	Type    string        `json:"type" yaml:"type"`
	Options []cheatOption `json:"options,omitempty" yaml:"options,omitempty"`
}

type cheatPackMatch struct {
	TitleAliases []string `json:"titleAliases,omitempty" yaml:"titleAliases,omitempty"`
}

type cheatPackPayload struct {
	ExactSizes []int    `json:"exactSizes,omitempty" yaml:"exactSizes,omitempty"`
	Formats    []string `json:"formats,omitempty" yaml:"formats,omitempty"`
}

type cheatPack struct {
	PackID        string           `json:"packId,omitempty" yaml:"packId,omitempty"`
	SchemaVersion int              `json:"schemaVersion,omitempty" yaml:"schemaVersion,omitempty"`
	AdapterID     string           `json:"adapterId,omitempty" yaml:"adapterId,omitempty"`
	GameID        string           `json:"gameId" yaml:"gameId"`
	SystemSlug    string           `json:"systemSlug" yaml:"systemSlug"`
	EditorID      string           `json:"editorId" yaml:"editorId"`
	Title         string           `json:"title" yaml:"title"`
	Match         cheatPackMatch   `json:"match" yaml:"match"`
	Payload       cheatPackPayload `json:"payload" yaml:"payload"`
	Selector      *cheatSelector   `json:"selector,omitempty" yaml:"selector,omitempty"`
	Sections      []cheatSection   `json:"sections,omitempty" yaml:"sections,omitempty"`
	Presets       []cheatPreset    `json:"presets,omitempty" yaml:"presets,omitempty"`
}

type saveCheatsGetResponse struct {
	Success      bool                 `json:"success"`
	SaveID       string               `json:"saveId"`
	DisplayTitle string               `json:"displayTitle"`
	Cheats       saveCheatEditorState `json:"cheats"`
}

type saveCheatEditorState struct {
	Supported      bool                      `json:"supported"`
	GameID         string                    `json:"gameId,omitempty"`
	SystemSlug     string                    `json:"systemSlug,omitempty"`
	EditorID       string                    `json:"editorId,omitempty"`
	AdapterID      string                    `json:"adapterId,omitempty"`
	PackID         string                    `json:"packId,omitempty"`
	Title          string                    `json:"title,omitempty"`
	AvailableCount int                       `json:"availableCount,omitempty"`
	Selector       *cheatSelector            `json:"selector,omitempty"`
	Sections       []cheatSection            `json:"sections,omitempty"`
	Presets        []cheatPreset             `json:"presets,omitempty"`
	Values         map[string]any            `json:"values,omitempty"`
	SlotValues     map[string]map[string]any `json:"slotValues,omitempty"`
}

type saveCheatApplyRequest struct {
	SaveID       string                     `json:"saveId"`
	LogicalKey   string                     `json:"logicalKey,omitempty"`
	PSLogicalKey string                     `json:"psLogicalKey,omitempty"`
	SaturnEntry  string                     `json:"saturnEntry,omitempty"`
	EditorID     string                     `json:"editorId"`
	AdapterID    string                     `json:"adapterId,omitempty"`
	SlotID       string                     `json:"slotId,omitempty"`
	Updates      map[string]json.RawMessage `json:"updates,omitempty"`
	PresetIDs    []string                   `json:"presetIds,omitempty"`
}

type cheatPackManifest struct {
	PackID         string     `json:"packId"`
	AdapterID      string     `json:"adapterId"`
	Source         string     `json:"source"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	PublishedBy    string     `json:"publishedBy,omitempty"`
	Notes          string     `json:"notes,omitempty"`
	SourcePath     string     `json:"sourcePath,omitempty"`
	SourceRevision string     `json:"sourceRevision,omitempty"`
	SourceSHA256   string     `json:"sourceSha256,omitempty"`
	LastSyncedAt   *time.Time `json:"lastSyncedAt,omitempty"`
}

type cheatPackTombstone struct {
	PackID      string    `json:"packId"`
	Status      string    `json:"status"`
	DeletedAt   time.Time `json:"deletedAt"`
	DeletedBy   string    `json:"deletedBy,omitempty"`
	Source      string    `json:"source,omitempty"`
	Description string    `json:"description,omitempty"`
}

type cheatManagedPack struct {
	Pack           cheatPack         `json:"pack"`
	Manifest       cheatPackManifest `json:"manifest"`
	Builtin        bool              `json:"builtin"`
	SupportsSaveUI bool              `json:"supportsSaveUi"`
}

type cheatManagedPackListResponse struct {
	Success bool               `json:"success"`
	Packs   []cheatManagedPack `json:"packs"`
}

type cheatManagedPackResponse struct {
	Success bool             `json:"success"`
	Pack    cheatManagedPack `json:"pack"`
}

type cheatPackCreateRequest struct {
	YAML        string `json:"yaml"`
	Source      string `json:"source,omitempty"`
	PublishedBy string `json:"publishedBy,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

type cheatLibraryConfig struct {
	Repo string `json:"repo"`
	Ref  string `json:"ref"`
	Path string `json:"path"`
}

type cheatLibraryImportedPack struct {
	Path         string `json:"path"`
	PackID       string `json:"packId,omitempty"`
	Title        string `json:"title,omitempty"`
	SystemSlug   string `json:"systemSlug,omitempty"`
	SourceSHA256 string `json:"sourceSha256,omitempty"`
	Status       string `json:"status,omitempty"`
}

type cheatLibrarySyncError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type cheatLibraryStatus struct {
	Config        cheatLibraryConfig         `json:"config"`
	LastSyncedAt  *time.Time                 `json:"lastSyncedAt,omitempty"`
	ImportedCount int                        `json:"importedCount"`
	ErrorCount    int                        `json:"errorCount"`
	Imported      []cheatLibraryImportedPack `json:"imported"`
	Errors        []cheatLibrarySyncError    `json:"errors"`
}

type cheatLibraryResponse struct {
	Success bool               `json:"success"`
	Library cheatLibraryStatus `json:"library"`
}

type cheatAdapterDescriptor struct {
	ID                     string   `json:"id"`
	Kind                   string   `json:"kind"`
	Family                 string   `json:"family"`
	SystemSlug             string   `json:"systemSlug"`
	RequiredParserID       string   `json:"requiredParserId,omitempty"`
	MinimumParserLevel     string   `json:"minimumParserLevel,omitempty"`
	SupportsRuntimeProfile bool     `json:"supportsRuntimeProfiles"`
	SupportsLogicalSaves   bool     `json:"supportsLogicalSaves"`
	SupportsLiveUpload     bool     `json:"supportsLiveUpload"`
	MatchKeys              []string `json:"matchKeys,omitempty"`
}

type cheatAdapterListResponse struct {
	Success  bool                     `json:"success"`
	Adapters []cheatAdapterDescriptor `json:"adapters"`
}

type cheatAdapterResponse struct {
	Success bool                   `json:"success"`
	Adapter cheatAdapterDescriptor `json:"adapter"`
}
