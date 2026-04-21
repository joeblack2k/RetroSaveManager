package main

import "time"

type device struct {
	ID           int       `json:"id"`
	DeviceType   string    `json:"deviceType"`
	Fingerprint  string    `json:"fingerprint"`
	Alias        *string   `json:"alias"`
	DisplayName  string    `json:"displayName"`
	LastSyncedAt time.Time `json:"lastSyncedAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

type system struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug,omitempty"`
}

type game struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Boxart      *string `json:"boxart"`
	BoxartThumb *string `json:"boxartThumb"`
	HasParser   bool    `json:"hasParser"`
	System      *system `json:"system"`
}

type saveSummary struct {
	ID        string      `json:"id"`
	Game      game        `json:"game"`
	Filename  string      `json:"filename"`
	FileSize  int         `json:"fileSize"`
	Format    string      `json:"format"`
	Version   int         `json:"version"`
	SHA256    string      `json:"sha256"`
	CreatedAt time.Time   `json:"createdAt"`
	Metadata  interface{} `json:"metadata"`
}
