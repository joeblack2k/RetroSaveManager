package main

import "time"

type trustedDevice struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type appPassword struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	LastFour  string     `json:"lastFour"`
	CreatedAt time.Time  `json:"createdAt"`
	LastUsed  *time.Time `json:"lastUsedAt,omitempty"`
}

type catalogGame struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	System      system  `json:"system"`
	Boxart      *string `json:"boxart"`
	BoxartThumb *string `json:"boxartThumb"`
	DownloadURL string  `json:"downloadUrl"`
}

type libraryGame struct {
	ID      string      `json:"id"`
	Catalog catalogGame `json:"catalog"`
	AddedAt time.Time   `json:"addedAt"`
}

type roadmapItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Votes       int       `json:"votes"`
	CreatedAt   time.Time `json:"createdAt"`
}

type roadmapSuggestion struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	UserID      string    `json:"userId"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}
