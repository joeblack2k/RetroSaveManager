package main

import "time"

type device struct {
	ID                       int                    `json:"id"`
	DeviceType               string                 `json:"deviceType"`
	Fingerprint              string                 `json:"fingerprint"`
	Alias                    *string                `json:"alias"`
	DisplayName              string                 `json:"displayName"`
	Hostname                 string                 `json:"hostname,omitempty"`
	HelperName               string                 `json:"helperName,omitempty"`
	HelperVersion            string                 `json:"helperVersion,omitempty"`
	Platform                 string                 `json:"platform,omitempty"`
	SyncPaths                []string               `json:"syncPaths,omitempty"`
	ReportedSystemSlugs      []string               `json:"reportedSystemSlugs,omitempty"`
	ConfigRevision           string                 `json:"configRevision,omitempty"`
	ConfigReportedAt         *time.Time             `json:"configReportedAt,omitempty"`
	ConfigGlobal             *deviceConfigGlobal    `json:"configGlobal,omitempty"`
	ConfigSources            []deviceConfigSource   `json:"configSources,omitempty"`
	ConfigCapabilities       map[string]any         `json:"configCapabilities,omitempty"`
	Service                  *deviceServiceState    `json:"service,omitempty"`
	Sensors                  *deviceSensorState     `json:"sensors,omitempty"`
	EffectivePolicy          *deviceEffectivePolicy `json:"effectivePolicy,omitempty"`
	LastSeenIP               string                 `json:"lastSeenIp,omitempty"`
	LastSeenUserAgent        string                 `json:"lastSeenUserAgent,omitempty"`
	LastSeenAt               time.Time              `json:"lastSeenAt"`
	SyncAll                  bool                   `json:"syncAll"`
	AllowedSystemSlugs       []string               `json:"allowedSystemSlugs,omitempty"`
	BoundAppPasswordID       *string                `json:"boundAppPasswordId,omitempty"`
	BoundAppPasswordName     string                 `json:"boundAppPasswordName,omitempty"`
	BoundAppPasswordLastFour string                 `json:"boundAppPasswordLastFour,omitempty"`
	LastSyncedAt             time.Time              `json:"lastSyncedAt"`
	CreatedAt                time.Time              `json:"createdAt"`
}

type deviceConfigGlobal struct {
	URL                   string `json:"url,omitempty"`
	Port                  int    `json:"port,omitempty"`
	BaseURL               string `json:"baseUrl,omitempty"`
	Email                 string `json:"email,omitempty"`
	AppPasswordConfigured bool   `json:"appPasswordConfigured"`
	Root                  string `json:"root,omitempty"`
	StateDir              string `json:"stateDir,omitempty"`
	Watch                 bool   `json:"watch"`
	WatchInterval         int    `json:"watchInterval,omitempty"`
	ForceUpload           bool   `json:"forceUpload"`
	DryRun                bool   `json:"dryRun"`
	RoutePrefix           string `json:"routePrefix,omitempty"`
}

type deviceConfigSource struct {
	ID                      string   `json:"id"`
	Label                   string   `json:"label,omitempty"`
	Kind                    string   `json:"kind,omitempty"`
	Profile                 string   `json:"profile,omitempty"`
	SavePath                string   `json:"savePath,omitempty"`
	SavePaths               []string `json:"savePaths,omitempty"`
	ROMPath                 string   `json:"romPath,omitempty"`
	ROMPaths                []string `json:"romPaths,omitempty"`
	Recursive               bool     `json:"recursive"`
	Systems                 []string `json:"systems,omitempty"`
	UnsupportedSystemSlugs  []string `json:"unsupportedSystemSlugs,omitempty"`
	CreateMissingSystemDirs bool     `json:"createMissingSystemDirs"`
	Managed                 bool     `json:"managed"`
	Origin                  string   `json:"origin,omitempty"`
}

type deviceServiceState struct {
	Mode                string     `json:"mode,omitempty"`
	Status              string     `json:"status,omitempty"`
	Loop                string     `json:"loop,omitempty"`
	ControlChannel      string     `json:"controlChannel,omitempty"`
	HeartbeatInterval   int        `json:"heartbeatInterval,omitempty"`
	ReconcileInterval   int        `json:"reconcileInterval,omitempty"`
	PID                 int        `json:"pid,omitempty"`
	StartedAt           *time.Time `json:"startedAt,omitempty"`
	UptimeSeconds       int64      `json:"uptimeSeconds,omitempty"`
	BinaryPath          string     `json:"binaryPath,omitempty"`
	LastSyncStartedAt   *time.Time `json:"lastSyncStartedAt,omitempty"`
	LastSyncFinishedAt  *time.Time `json:"lastSyncFinishedAt,omitempty"`
	LastSyncOk          *bool      `json:"lastSyncOk,omitempty"`
	LastError           string     `json:"lastError,omitempty"`
	LastEvent           string     `json:"lastEvent,omitempty"`
	SyncCycles          int        `json:"syncCycles,omitempty"`
	LastHeartbeatAt     *time.Time `json:"lastHeartbeatAt,omitempty"`
	Online              bool       `json:"online"`
	Freshness           string     `json:"freshness,omitempty"`
	StaleAfterSeconds   int        `json:"staleAfterSeconds,omitempty"`
	OfflineAfterSeconds int        `json:"offlineAfterSeconds,omitempty"`
	OfflineAt           *time.Time `json:"offlineAt,omitempty"`
}

type deviceSensorState struct {
	Online            bool                 `json:"online"`
	Authenticated     bool                 `json:"authenticated"`
	ConfigHash        string               `json:"configHash,omitempty"`
	ConfigReadable    bool                 `json:"configReadable"`
	ConfigError       string               `json:"configError,omitempty"`
	SourceCount       int                  `json:"sourceCount,omitempty"`
	SavePathCount     int                  `json:"savePathCount,omitempty"`
	ROMPathCount      int                  `json:"romPathCount,omitempty"`
	ConfiguredSystems []string             `json:"configuredSystems,omitempty"`
	SupportedSystems  []string             `json:"supportedSystems,omitempty"`
	SyncLockPresent   bool                 `json:"syncLockPresent"`
	LastSync          *deviceLastSyncStats `json:"lastSync,omitempty"`
}

type deviceLastSyncStats struct {
	Scanned    int `json:"scanned"`
	Uploaded   int `json:"uploaded"`
	Downloaded int `json:"downloaded"`
	InSync     int `json:"inSync"`
	Conflicts  int `json:"conflicts"`
	Skipped    int `json:"skipped"`
	Errors     int `json:"errors"`
}

type devicePolicyBlock struct {
	System      string `json:"system"`
	Reason      string `json:"reason"`
	SourceID    string `json:"sourceId,omitempty"`
	SourceLabel string `json:"sourceLabel,omitempty"`
}

type deviceSourceEffectivePolicy struct {
	SourceID           string              `json:"sourceId,omitempty"`
	SourceLabel        string              `json:"sourceLabel,omitempty"`
	Kind               string              `json:"kind,omitempty"`
	Profile            string              `json:"profile,omitempty"`
	AllowedSystemSlugs []string            `json:"allowedSystemSlugs,omitempty"`
	Blocked            []devicePolicyBlock `json:"blocked,omitempty"`
}

type deviceEffectivePolicy struct {
	Mode               string                        `json:"mode"`
	AllowedSystemSlugs []string                      `json:"allowedSystemSlugs,omitempty"`
	Blocked            []devicePolicyBlock           `json:"blocked,omitempty"`
	Sources            []deviceSourceEffectivePolicy `json:"sources,omitempty"`
}

type system struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
}

type game struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	DisplayTitle  string   `json:"displayTitle,omitempty"`
	RegionCode    string   `json:"regionCode,omitempty"`
	RegionFlag    string   `json:"regionFlag,omitempty"`
	LanguageCodes []string `json:"languageCodes,omitempty"`
	CoverArtURL   string   `json:"coverArtUrl,omitempty"`
	Boxart        *string  `json:"boxart"`
	BoxartThumb   *string  `json:"boxartThumb"`
	HasParser     bool     `json:"hasParser"`
	System        *system  `json:"system"`
}

type memoryCardEntry struct {
	LogicalKey      string `json:"logicalKey,omitempty"`
	Title           string `json:"title"`
	Slot            int    `json:"slot"`
	Blocks          int    `json:"blocks"`
	ProductCode     string `json:"productCode,omitempty"`
	RegionCode      string `json:"regionCode,omitempty"`
	DirectoryName   string `json:"directoryName,omitempty"`
	IconDataURL     string `json:"iconDataUrl,omitempty"`
	SizeBytes       int    `json:"sizeBytes,omitempty"`
	SaveCount       int    `json:"saveCount,omitempty"`
	LatestVersion   int    `json:"latestVersion,omitempty"`
	LatestSizeBytes int    `json:"latestSizeBytes,omitempty"`
	TotalSizeBytes  int    `json:"totalSizeBytes,omitempty"`
	LatestCreatedAt string `json:"latestCreatedAt,omitempty"`
	Portable        *bool  `json:"portable,omitempty"`
}

type memoryCardDetails struct {
	Name    string            `json:"name"`
	Entries []memoryCardEntry `json:"entries,omitempty"`
}

type controllerPakEntry struct {
	LogicalKey     string `json:"logicalKey,omitempty"`
	GameCode       string `json:"gameCode,omitempty"`
	PublisherCode  string `json:"publisherCode,omitempty"`
	NoteName       string `json:"noteName,omitempty"`
	EntryIndex     int    `json:"entryIndex,omitempty"`
	PageCount      int    `json:"pageCount,omitempty"`
	BlockUsage     int    `json:"blockUsage,omitempty"`
	StructureValid bool   `json:"structureValid,omitempty"`
	ChecksumValid  *bool  `json:"checksumValid,omitempty"`
	SizeBytes      int    `json:"sizeBytes,omitempty"`
}

type saveInspection struct {
	ParserLevel        string         `json:"parserLevel,omitempty"`
	ParserID           string         `json:"parserId,omitempty"`
	ValidatedSystem    string         `json:"validatedSystem,omitempty"`
	ValidatedGameID    string         `json:"validatedGameId,omitempty"`
	ValidatedGameTitle string         `json:"validatedGameTitle,omitempty"`
	TrustLevel         string         `json:"trustLevel,omitempty"`
	Evidence           []string       `json:"evidence,omitempty"`
	Warnings           []string       `json:"warnings,omitempty"`
	PayloadSizeBytes   int            `json:"payloadSizeBytes,omitempty"`
	SlotCount          int            `json:"slotCount,omitempty"`
	ActiveSlotIndexes  []int          `json:"activeSlotIndexes,omitempty"`
	ChecksumValid      *bool          `json:"checksumValid,omitempty"`
	SemanticFields     map[string]any `json:"semanticFields,omitempty"`
}

type saveSummary struct {
	ID                    string              `json:"id"`
	Game                  game                `json:"game"`
	Cheats                *cheatCapability    `json:"cheats,omitempty"`
	DownloadProfiles      []downloadProfile   `json:"downloadProfiles,omitempty"`
	DisplayTitle          string              `json:"displayTitle,omitempty"`
	LogicalKey            string              `json:"logicalKey,omitempty"`
	SystemSlug            string              `json:"systemSlug,omitempty"`
	RegionCode            string              `json:"regionCode,omitempty"`
	RegionFlag            string              `json:"regionFlag,omitempty"`
	LanguageCodes         []string            `json:"languageCodes,omitempty"`
	CoverArtURL           string              `json:"coverArtUrl,omitempty"`
	MediaType             string              `json:"mediaType,omitempty"`
	ProjectionCapable     *bool               `json:"projectionCapable,omitempty"`
	SourceArtifactProfile string              `json:"sourceArtifactProfile,omitempty"`
	SaveCount             int                 `json:"saveCount,omitempty"`
	LatestSizeBytes       int                 `json:"latestSizeBytes,omitempty"`
	TotalSizeBytes        int                 `json:"totalSizeBytes,omitempty"`
	LatestVersion         int                 `json:"latestVersion,omitempty"`
	MemoryCard            *memoryCardDetails  `json:"memoryCard,omitempty"`
	ControllerPakEntry    *controllerPakEntry `json:"controllerPakEntry,omitempty"`
	Dreamcast             *dreamcastDetails   `json:"dreamcast,omitempty"`
	Saturn                *saturnDetails      `json:"saturn,omitempty"`
	Inspection            *saveInspection     `json:"inspection,omitempty"`
	RuntimeProfile        string              `json:"runtimeProfile,omitempty"`
	CardSlot              string              `json:"cardSlot,omitempty"`
	ProjectionID          string              `json:"projectionId,omitempty"`
	SourceImportID        string              `json:"sourceImportId,omitempty"`
	Portable              *bool               `json:"portable,omitempty"`
	Filename              string              `json:"filename"`
	FileSize              int                 `json:"fileSize"`
	Format                string              `json:"format"`
	Version               int                 `json:"version"`
	SHA256                string              `json:"sha256"`
	CreatedAt             time.Time           `json:"createdAt"`
	Metadata              interface{}         `json:"metadata"`
}
