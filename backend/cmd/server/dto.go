package main

import "encoding/json"

type loginRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DeviceType  string `json:"deviceType"`
	Fingerprint string `json:"fingerprint"`
}

type tokenResponse struct {
	Success       bool   `json:"success"`
	Token         string `json:"token"`
	ExpiresInDays int    `json:"expiresInDays"`
}

type apiError struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

type lookupQuery struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type gamesLookupRequest struct {
	Items []lookupQuery `json:"items"`
}

type saveBatchItem struct {
	Filename string          `json:"filename"`
	Game     json.RawMessage `json:"game"`
	Data     string          `json:"data"`
}

type saveBatchUploadRequest struct {
	Items []saveBatchItem `json:"items"`
}

type devicePatchRequest struct {
	Alias              *string   `json:"alias"`
	SyncAll            *bool     `json:"syncAll"`
	AllowedSystemSlugs *[]string `json:"allowedSystemSlugs"`
}
