package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (s *gameModuleService) libraryStatus() (gameModuleLibraryStatus, error) {
	return s.readLibraryStatus(gameModuleLibraryConfigFromEnv())
}

func (s *gameModuleService) readLibraryStatus(config gameModuleLibraryConfig) (gameModuleLibraryStatus, error) {
	status := gameModuleLibraryStatus{Config: config, Imported: []gameModuleSyncImported{}, Errors: []gameModuleSyncError{}}
	if s == nil {
		return status, nil
	}
	data, err := os.ReadFile(s.status)
	if os.IsNotExist(err) {
		return status, nil
	}
	if err != nil {
		return status, err
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return gameModuleLibraryStatus{}, err
	}
	status.Config = config
	status.ImportedCount = len(status.Imported)
	status.ErrorCount = len(status.Errors)
	if status.Imported == nil {
		status.Imported = []gameModuleSyncImported{}
	}
	if status.Errors == nil {
		status.Errors = []gameModuleSyncError{}
	}
	return status, nil
}

func (s *gameModuleService) writeLibraryStatus(status gameModuleLibraryStatus) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.status, data, 0o644)
}

// syncLibrary imports .rsmodule.zip bundles from the configured public GitHub path.
// Invalid modules are reported but never activated.
func (s *gameModuleService) syncLibrary(ctx context.Context) (gameModuleLibraryStatus, error) {
	config := gameModuleLibraryConfigFromEnv()
	now := time.Now().UTC()
	status := gameModuleLibraryStatus{Config: config, LastSyncedAt: &now, Imported: []gameModuleSyncImported{}, Errors: []gameModuleSyncError{}}
	files, err := fetchGameModuleLibraryTree(ctx, config)
	if err != nil {
		status.Errors = append(status.Errors, gameModuleSyncError{Path: config.Path, Message: err.Error()})
		status.ErrorCount = len(status.Errors)
		_ = s.writeLibraryStatus(status)
		return status, nil
	}
	for _, file := range files {
		sourcePath := file.Path
		data, err := fetchGameModuleLibraryRaw(ctx, config, sourcePath, file.SHA)
		if err != nil {
			status.Errors = append(status.Errors, gameModuleSyncError{Path: sourcePath, Message: err.Error()})
			continue
		}
		sourceHash := sha256Hex(data)
		record, err := s.importZip(ctx, data, gameModuleSourceInfo{Source: gameModuleSourceGithub, SourcePath: sourcePath, SourceRevision: config.Ref, SourceSHA256: sourceHash, LastSyncedAt: &now})
		if err != nil {
			status.Errors = append(status.Errors, gameModuleSyncError{Path: sourcePath, Message: err.Error()})
			continue
		}
		status.Imported = append(status.Imported, gameModuleSyncImported{Path: sourcePath, ModuleID: record.Manifest.ModuleID, Title: record.Manifest.Title, SystemSlug: record.Manifest.SystemSlug, Status: record.Status, SHA256: sourceHash})
	}
	status.ImportedCount = len(status.Imported)
	status.ErrorCount = len(status.Errors)
	sort.SliceStable(status.Imported, func(i, j int) bool { return status.Imported[i].Path < status.Imported[j].Path })
	sort.SliceStable(status.Errors, func(i, j int) bool { return status.Errors[i].Path < status.Errors[j].Path })
	if err := s.writeLibraryStatus(status); err != nil {
		return gameModuleLibraryStatus{}, err
	}
	return status, nil
}

// gameModuleLibraryConfigFromEnv keeps module distribution configurable without
// requiring a new Docker image for community module updates.
func gameModuleLibraryConfigFromEnv() gameModuleLibraryConfig {
	return gameModuleLibraryConfig{
		Repo: firstNonEmpty(strings.TrimSpace(os.Getenv("MODULE_LIBRARY_REPO")), defaultModuleLibraryRepo),
		Ref:  firstNonEmpty(strings.TrimSpace(os.Getenv("MODULE_LIBRARY_REF")), defaultModuleLibraryRef),
		Path: cleanCheatLibraryPath(firstNonEmpty(strings.TrimSpace(os.Getenv("MODULE_LIBRARY_PATH")), defaultModuleLibraryPath)),
	}
}

func fetchGameModuleLibraryTree(ctx context.Context, config gameModuleLibraryConfig) ([]githubLibraryFile, error) {
	var tree githubTreeResponse
	if err := fetchCheatLibraryJSON(ctx, gameModuleTreeURL(config), &tree); err != nil {
		return nil, err
	}
	prefix := strings.Trim(config.Path, "/")
	if prefix != "" {
		prefix += "/"
	}
	files := make([]githubLibraryFile, 0, len(tree.Tree))
	for _, item := range tree.Tree {
		if strings.TrimSpace(item.Type) != "blob" {
			continue
		}
		itemPath := strings.Trim(item.Path, "/")
		if prefix != "" && !strings.HasPrefix(itemPath, prefix) {
			continue
		}
		if strings.EqualFold(filepath.Ext(itemPath), ".zip") && strings.HasSuffix(strings.ToLower(itemPath), ".rsmodule.zip") {
			files = append(files, githubLibraryFile{Path: itemPath, SHA: strings.TrimSpace(item.SHA)})
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func fetchGameModuleLibraryRaw(ctx context.Context, config gameModuleLibraryConfig, sourcePath, cacheKey string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gameModuleRawURL(config, sourcePath, cacheKey), nil)
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
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGameModuleZipBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("empty module zip")
	}
	if len(data) > maxGameModuleZipBytes {
		return nil, fmt.Errorf("module zip exceeds %d bytes", maxGameModuleZipBytes)
	}
	return data, nil
}

func gameModuleTreeURL(config gameModuleLibraryConfig) string {
	apiBase := strings.TrimRight(firstNonEmpty(strings.TrimSpace(os.Getenv("MODULE_LIBRARY_API_BASE")), "https://api.github.com/repos"), "/")
	return apiBase + "/" + urlPathEscapeSegments(config.Repo) + "/git/trees/" + url.PathEscape(config.Ref) + "?recursive=1"
}

func gameModuleRawURL(config gameModuleLibraryConfig, sourcePath, cacheKey string) string {
	rawBase := strings.TrimRight(firstNonEmpty(strings.TrimSpace(os.Getenv("MODULE_LIBRARY_RAW_BASE")), "https://raw.githubusercontent.com"), "/")
	return withRawCacheBuster(rawBase+"/"+urlPathEscapeSegments(config.Repo)+"/"+url.PathEscape(config.Ref)+"/"+urlPathEscapeSegments(sourcePath), cacheKey)
}
