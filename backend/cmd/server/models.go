package main

import "time"

type device struct {
	ID                       int       `json:"id"`
	DeviceType               string    `json:"deviceType"`
	Fingerprint              string    `json:"fingerprint"`
	Alias                    *string   `json:"alias"`
	DisplayName              string    `json:"displayName"`
	SyncAll                  bool      `json:"syncAll"`
	AllowedSystemSlugs       []string  `json:"allowedSystemSlugs,omitempty"`
	BoundAppPasswordID       *string   `json:"boundAppPasswordId,omitempty"`
	BoundAppPasswordName     string    `json:"boundAppPasswordName,omitempty"`
	BoundAppPasswordLastFour string    `json:"boundAppPasswordLastFour,omitempty"`
	LastSyncedAt             time.Time `json:"lastSyncedAt"`
	CreatedAt                time.Time `json:"createdAt"`
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

type saveSummary struct {
	ID              string             `json:"id"`
	Game            game               `json:"game"`
	DisplayTitle    string             `json:"displayTitle,omitempty"`
	SystemSlug      string             `json:"systemSlug,omitempty"`
	RegionCode      string             `json:"regionCode,omitempty"`
	RegionFlag      string             `json:"regionFlag,omitempty"`
	LanguageCodes   []string           `json:"languageCodes,omitempty"`
	CoverArtURL     string             `json:"coverArtUrl,omitempty"`
	SaveCount       int                `json:"saveCount,omitempty"`
	LatestSizeBytes int                `json:"latestSizeBytes,omitempty"`
	TotalSizeBytes  int                `json:"totalSizeBytes,omitempty"`
	LatestVersion   int                `json:"latestVersion,omitempty"`
	MemoryCard      *memoryCardDetails `json:"memoryCard,omitempty"`
	RuntimeProfile  string             `json:"runtimeProfile,omitempty"`
	CardSlot        string             `json:"cardSlot,omitempty"`
	ProjectionID    string             `json:"projectionId,omitempty"`
	SourceImportID  string             `json:"sourceImportId,omitempty"`
	Portable        *bool              `json:"portable,omitempty"`
	Filename        string             `json:"filename"`
	FileSize        int                `json:"fileSize"`
	Format          string             `json:"format"`
	Version         int                `json:"version"`
	SHA256          string             `json:"sha256"`
	CreatedAt       time.Time          `json:"createdAt"`
	Metadata        interface{}        `json:"metadata"`
}
