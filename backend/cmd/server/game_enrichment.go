package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type enrichmentEntry struct {
	CoverArtURL string    `json:"coverArtUrl,omitempty"`
	RegionCode  string    `json:"regionCode,omitempty"`
	Source      string    `json:"source,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type enrichmentCacheFile struct {
	UpdatedAt time.Time                  `json:"updatedAt"`
	Entries   map[string]enrichmentEntry `json:"entries"`
}

type gameEnricher struct {
	mu               sync.Mutex
	cache            map[string]enrichmentEntry
	stateFile        string
	httpClient       *http.Client
	igdbClientID     string
	igdbClientSecret string
	igdbToken        string
	igdbTokenExpiry  time.Time
	rawgAPIKey       string
}

func newGameEnricherFromEnv() *gameEnricher {
	stateRoot := stateRootDirFromEnv()

	e := &gameEnricher{
		cache:     map[string]enrichmentEntry{},
		stateFile: filepath.Join(stateRoot, "game_enrichment_cache.json"),
		httpClient: &http.Client{
			Timeout: 6 * time.Second,
		},
		igdbClientID:     strings.TrimSpace(os.Getenv("IGDB_CLIENT_ID")),
		igdbClientSecret: strings.TrimSpace(os.Getenv("IGDB_CLIENT_SECRET")),
		rawgAPIKey:       strings.TrimSpace(os.Getenv("RAWG_API_KEY")),
	}
	e.loadCache()
	return e
}

func (e *gameEnricher) enrich(title, systemName, currentRegion string) enrichmentEntry {
	displayTitle := strings.TrimSpace(title)
	if displayTitle == "" {
		return enrichmentEntry{}
	}

	key := enrichmentCacheKey(displayTitle, systemName)

	e.mu.Lock()
	if cached, ok := e.cache[key]; ok {
		e.mu.Unlock()
		if cached.RegionCode == "" {
			cached.RegionCode = normalizeRegionCode(currentRegion)
		}
		return cached
	}
	e.mu.Unlock()

	enriched := enrichmentEntry{
		RegionCode: normalizeRegionCode(currentRegion),
		UpdatedAt:  time.Now().UTC(),
	}

	if igdbHit, ok := e.lookupIGDB(displayTitle); ok {
		enriched.CoverArtURL = igdbHit.CoverArtURL
		if normalizeRegionCode(enriched.RegionCode) == regionUnknown && normalizeRegionCode(igdbHit.RegionCode) != regionUnknown {
			enriched.RegionCode = normalizeRegionCode(igdbHit.RegionCode)
		}
		enriched.Source = "igdb"
	} else if rawgHit, ok := e.lookupRAWG(displayTitle); ok {
		enriched.CoverArtURL = rawgHit.CoverArtURL
		if normalizeRegionCode(enriched.RegionCode) == regionUnknown && normalizeRegionCode(rawgHit.RegionCode) != regionUnknown {
			enriched.RegionCode = normalizeRegionCode(rawgHit.RegionCode)
		}
		enriched.Source = "rawg"
	}

	e.mu.Lock()
	e.cache[key] = enriched
	e.saveCacheLocked()
	e.mu.Unlock()
	return enriched
}

func enrichmentCacheKey(title, systemName string) string {
	return strings.ToLower(strings.TrimSpace(title)) + "|" + strings.ToLower(strings.TrimSpace(systemName))
}

func (e *gameEnricher) loadCache() {
	data, err := os.ReadFile(e.stateFile)
	if err != nil || len(data) == 0 {
		return
	}
	var cache enrichmentCacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		return
	}
	if cache.Entries == nil {
		return
	}
	e.mu.Lock()
	for key, value := range cache.Entries {
		e.cache[key] = value
	}
	e.mu.Unlock()
}

func (e *gameEnricher) saveCacheLocked() {
	payload := enrichmentCacheFile{
		UpdatedAt: time.Now().UTC(),
		Entries:   e.cache,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	_ = writeFileAtomic(e.stateFile, data, 0o644)
}

func (e *gameEnricher) lookupIGDB(title string) (enrichmentEntry, bool) {
	if e.igdbClientID == "" || e.igdbClientSecret == "" {
		return enrichmentEntry{}, false
	}

	token, ok := e.getIGDBToken()
	if !ok {
		return enrichmentEntry{}, false
	}

	escapedTitle := strings.ReplaceAll(title, `"`, `\"`)
	query := fmt.Sprintf(`search "%s"; fields name,cover.url; limit 5;`, escapedTitle)
	req, err := http.NewRequest(http.MethodPost, "https://api.igdb.com/v4/games", bytes.NewBufferString(query))
	if err != nil {
		return enrichmentEntry{}, false
	}
	req.Header.Set("Client-ID", e.igdbClientID)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "text/plain")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return enrichmentEntry{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return enrichmentEntry{}, false
	}

	var payload []struct {
		Name  string `json:"name"`
		Cover *struct {
			URL string `json:"url"`
		} `json:"cover"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return enrichmentEntry{}, false
	}
	if len(payload) == 0 {
		return enrichmentEntry{}, false
	}

	cover := ""
	for _, game := range payload {
		if game.Cover == nil || strings.TrimSpace(game.Cover.URL) == "" {
			continue
		}
		cover = normalizeIGDBCoverURL(game.Cover.URL)
		if cover != "" {
			break
		}
	}
	if cover == "" {
		return enrichmentEntry{}, false
	}
	return enrichmentEntry{
		CoverArtURL: cover,
		UpdatedAt:   time.Now().UTC(),
	}, true
}

func (e *gameEnricher) getIGDBToken() (string, bool) {
	e.mu.Lock()
	if e.igdbToken != "" && time.Now().UTC().Before(e.igdbTokenExpiry) {
		token := e.igdbToken
		e.mu.Unlock()
		return token, true
	}
	e.mu.Unlock()

	values := url.Values{}
	values.Set("client_id", e.igdbClientID)
	values.Set("client_secret", e.igdbClientSecret)
	values.Set("grant_type", "client_credentials")

	req, err := http.NewRequest(http.MethodPost, "https://id.twitch.tv/oauth2/token?"+values.Encode(), nil)
	if err != nil {
		return "", false
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", false
	}
	if strings.TrimSpace(body.AccessToken) == "" {
		return "", false
	}

	expiry := time.Now().UTC().Add(time.Duration(body.ExpiresIn) * time.Second)
	if body.ExpiresIn > 120 {
		expiry = expiry.Add(-60 * time.Second)
	}

	e.mu.Lock()
	e.igdbToken = body.AccessToken
	e.igdbTokenExpiry = expiry
	e.mu.Unlock()
	return body.AccessToken, true
}

func normalizeIGDBCoverURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "//") {
		value = "https:" + value
	}
	value = strings.ReplaceAll(value, "/t_thumb/", "/t_cover_big/")
	return value
}

func (e *gameEnricher) lookupRAWG(title string) (enrichmentEntry, bool) {
	if e.rawgAPIKey == "" {
		return enrichmentEntry{}, false
	}

	values := url.Values{}
	values.Set("key", e.rawgAPIKey)
	values.Set("search", title)
	values.Set("search_precise", "true")
	values.Set("page_size", "5")
	requestURL := "https://api.rawg.io/api/games?" + values.Encode()
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return enrichmentEntry{}, false
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return enrichmentEntry{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return enrichmentEntry{}, false
	}

	var payload struct {
		Results []struct {
			Name            string `json:"name"`
			BackgroundImage string `json:"background_image"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return enrichmentEntry{}, false
	}
	if len(payload.Results) == 0 {
		return enrichmentEntry{}, false
	}

	for _, result := range payload.Results {
		cover := strings.TrimSpace(result.BackgroundImage)
		if cover == "" {
			continue
		}
		return enrichmentEntry{
			CoverArtURL: cover,
			RegionCode:  detectRegionCode(result.Name),
			UpdatedAt:   time.Now().UTC(),
		}, true
	}
	return enrichmentEntry{}, false
}
