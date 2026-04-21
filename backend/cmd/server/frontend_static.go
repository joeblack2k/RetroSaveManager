package main

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const defaultFrontendDistDir = "./frontend/dist"

type frontendStaticHandler struct {
	root      string
	indexPath string
}

func newFrontendStaticHandler() *frontendStaticHandler {
	distDir := strings.TrimSpace(os.Getenv("FRONTEND_DIST_DIR"))
	if distDir == "" {
		distDir = defaultFrontendDistDir
	}

	absRoot, err := filepath.Abs(distDir)
	if err != nil {
		return nil
	}
	indexPath := filepath.Join(absRoot, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return nil
	}

	return &frontendStaticHandler{
		root:      absRoot,
		indexPath: indexPath,
	}
}

func (h *frontendStaticHandler) serve(w http.ResponseWriter, r *http.Request) bool {
	if h == nil {
		return false
	}

	requestPath := strings.TrimSpace(r.URL.Path)
	if requestPath == "" {
		requestPath = "/"
	}
	if strings.ContainsRune(requestPath, '\x00') {
		return false
	}

	cleanPath := filepath.Clean("/" + requestPath)
	if cleanPath == "/" {
		http.ServeFile(w, r, h.indexPath)
		return true
	}

	relative := strings.TrimPrefix(cleanPath, "/")
	target := filepath.Join(h.root, filepath.FromSlash(relative))
	target = filepath.Clean(target)
	if !isSubpath(h.root, target) {
		return false
	}

	info, err := os.Stat(target)
	if err == nil && !info.IsDir() {
		http.ServeFile(w, r, target)
		return true
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false
	}

	// Missing asset path should return 404 instead of SPA shell.
	if hasFileExtension(relative) {
		return false
	}

	http.ServeFile(w, r, h.indexPath)
	return true
}

func hasFileExtension(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(base, ".")
}

func isSubpath(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

func isReservedAPIPath(path string) bool {
	clean := filepath.Clean("/" + strings.TrimSpace(path))
	if clean == "/healthz" {
		return true
	}

	prefixes := []string{
		"/v1",
		"/auth",
		"/save",
		"/saves",
		"/rom",
		"/conflicts",
		"/devices",
		"/events",
		"/games",
		"/catalog",
		"/roadmap/items",
		"/roadmap/suggestions",
		"/parser",
		"/referral",
		"/dev",
		"/stripe",
	}
	for _, prefix := range prefixes {
		if hasRoutePrefix(clean, prefix) {
			return true
		}
	}
	return false
}

func hasRoutePrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}
