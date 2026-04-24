package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultCheatLibraryRepo = "joeblack2k/RetroSaveManager"
	defaultCheatLibraryRef  = "main"
	defaultCheatLibraryPath = "cheats/packs"
)

type githubTreeResponse struct {
	Tree []githubTreeItem `json:"tree"`
}

type githubTreeItem struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

func (s *cheatService) libraryStatus() (cheatLibraryStatus, error) {
	return s.runtimeStore.readLibraryStatus(cheatLibraryConfigFromEnv())
}

func (s *cheatService) syncLibrary(ctx context.Context) (cheatLibraryStatus, error) {
	config := cheatLibraryConfigFromEnv()
	now := time.Now().UTC()
	status := cheatLibraryStatus{
		Config:       config,
		LastSyncedAt: &now,
		Imported:     []cheatLibraryImportedPack{},
		Errors:       []cheatLibrarySyncError{},
	}
	files, err := fetchCheatLibraryTree(ctx, config)
	if err != nil {
		status.Errors = append(status.Errors, cheatLibrarySyncError{Path: config.Path, Message: err.Error()})
		status.ErrorCount = len(status.Errors)
		_ = s.runtimeStore.writeLibraryStatus(status)
		return status, nil
	}
	existing, err := s.listManagedPacks()
	if err != nil {
		return cheatLibraryStatus{}, err
	}
	existingByID := map[string]cheatManagedPack{}
	for _, item := range existing {
		packID := canonicalCheatPackID(item.Manifest.PackID)
		if packID == "" {
			continue
		}
		existingByID[packID] = item
	}
	for _, sourcePath := range files {
		data, fetchErr := fetchCheatLibraryRaw(ctx, config, sourcePath)
		if fetchErr != nil {
			status.Errors = append(status.Errors, cheatLibrarySyncError{Path: sourcePath, Message: fetchErr.Error()})
			continue
		}
		sourceHash := sha256Hex(data)
		pack, decodeErr := decodeCheatPackData(data, true)
		if decodeErr != nil {
			status.Errors = append(status.Errors, cheatLibrarySyncError{Path: sourcePath, Message: decodeErr.Error()})
			continue
		}
		pack.PackID = canonicalCheatPackID(firstNonEmpty(pack.PackID, pack.GameID, pack.Title))
		prepared, validateErr := s.validateLiveCheatPack(pack)
		if validateErr != nil {
			status.Errors = append(status.Errors, cheatLibrarySyncError{Path: sourcePath, Message: validateErr.Error()})
			continue
		}
		packID := canonicalCheatPackID(firstNonEmpty(prepared.PackID, prepared.GameID, prepared.Title))
		manifest := cheatPackManifest{
			PackID:         packID,
			AdapterID:      prepared.AdapterID,
			Source:         cheatPackSourceGithub,
			Status:         cheatPackStatusActive,
			PublishedBy:    "GitHub library",
			SourcePath:     sourcePath,
			SourceRevision: config.Ref,
			SourceSHA256:   sourceHash,
			LastSyncedAt:   &now,
		}
		if existingPack, ok := existingByID[packID]; ok {
			manifest.Status = firstNonEmpty(existingPack.Manifest.Status, cheatPackStatusActive)
			manifest.CreatedAt = existingPack.Manifest.CreatedAt
			manifest.PublishedBy = firstNonEmpty(existingPack.Manifest.PublishedBy, manifest.PublishedBy)
			manifest.Notes = existingPack.Manifest.Notes
		}
		managed, writeErr := s.runtimeStore.writePack(prepared, manifest)
		if writeErr != nil {
			status.Errors = append(status.Errors, cheatLibrarySyncError{Path: sourcePath, Message: writeErr.Error()})
			continue
		}
		status.Imported = append(status.Imported, cheatLibraryImportedPack{
			Path:         sourcePath,
			PackID:       managed.Manifest.PackID,
			Title:        managed.Pack.Title,
			SystemSlug:   managed.Pack.SystemSlug,
			SourceSHA256: sourceHash,
			Status:       managed.Manifest.Status,
		})
	}
	status.ImportedCount = len(status.Imported)
	status.ErrorCount = len(status.Errors)
	sort.SliceStable(status.Imported, func(i, j int) bool {
		return status.Imported[i].Path < status.Imported[j].Path
	})
	sort.SliceStable(status.Errors, func(i, j int) bool {
		return status.Errors[i].Path < status.Errors[j].Path
	})
	if err := s.runtimeStore.writeLibraryStatus(status); err != nil {
		return cheatLibraryStatus{}, err
	}
	return status, nil
}

func cheatLibraryConfigFromEnv() cheatLibraryConfig {
	return cheatLibraryConfig{
		Repo: firstNonEmpty(strings.TrimSpace(os.Getenv("CHEAT_LIBRARY_REPO")), defaultCheatLibraryRepo),
		Ref:  firstNonEmpty(strings.TrimSpace(os.Getenv("CHEAT_LIBRARY_REF")), defaultCheatLibraryRef),
		Path: cleanCheatLibraryPath(firstNonEmpty(strings.TrimSpace(os.Getenv("CHEAT_LIBRARY_PATH")), defaultCheatLibraryPath)),
	}
}

func cleanCheatLibraryPath(raw string) string {
	cleaned := strings.Trim(path.Clean("/"+strings.TrimSpace(raw)), "/")
	if cleaned == "." {
		return defaultCheatLibraryPath
	}
	return cleaned
}

func fetchCheatLibraryTree(ctx context.Context, config cheatLibraryConfig) ([]string, error) {
	var tree githubTreeResponse
	if err := fetchCheatLibraryJSON(ctx, cheatLibraryTreeURL(config), &tree); err != nil {
		return nil, err
	}
	prefix := strings.Trim(config.Path, "/")
	if prefix != "" {
		prefix += "/"
	}
	files := make([]string, 0, len(tree.Tree))
	for _, item := range tree.Tree {
		if strings.TrimSpace(item.Type) != "blob" {
			continue
		}
		itemPath := strings.Trim(item.Path, "/")
		if prefix != "" && !strings.HasPrefix(itemPath, prefix) {
			continue
		}
		ext := strings.ToLower(filepath.Ext(itemPath))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		files = append(files, itemPath)
	}
	sort.Strings(files)
	return files, nil
}

func fetchCheatLibraryRaw(ctx context.Context, config cheatLibraryConfig, sourcePath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cheatLibraryRawURL(config, sourcePath), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "RetroSaveManager")
	resp, err := cheatLibraryHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub raw returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty cheat pack")
	}
	return data, nil
}

func fetchCheatLibraryJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "RetroSaveManager")
	resp, err := cheatLibraryHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(target); err != nil {
		return err
	}
	return nil
}

func cheatLibraryHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func cheatLibraryTreeURL(config cheatLibraryConfig) string {
	apiBase := strings.TrimRight(firstNonEmpty(strings.TrimSpace(os.Getenv("CHEAT_LIBRARY_API_BASE")), "https://api.github.com/repos"), "/")
	return apiBase + "/" + urlPathEscapeSegments(config.Repo) + "/git/trees/" + url.PathEscape(config.Ref) + "?recursive=1"
}

func cheatLibraryRawURL(config cheatLibraryConfig, sourcePath string) string {
	rawBase := strings.TrimRight(firstNonEmpty(strings.TrimSpace(os.Getenv("CHEAT_LIBRARY_RAW_BASE")), "https://raw.githubusercontent.com"), "/")
	return rawBase + "/" + urlPathEscapeSegments(config.Repo) + "/" + url.PathEscape(config.Ref) + "/" + urlPathEscapeSegments(sourcePath)
}

func urlPathEscapeSegments(value string) string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, url.PathEscape(part))
	}
	return strings.Join(out, "/")
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
