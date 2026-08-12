package main

import (
	"net/http"
	"strings"
	"time"
)

// publicAuthPaths contains only bootstrap endpoints that genuinely need to be
// reachable before a web/admin session exists. Placeholder account-management,
// 2FA and verification endpoints are deliberately not public in self-hosted
// authenticated mode.
var publicAuthPaths = map[string]struct{}{
	"/runtime-config":          {},
	"/auth/login":              {},
	"/auth/token/app-password": {},
	"/auth/device":             {},
	"/auth/device/token":       {},
}

func stripRoutePrefix(p string) string {
	for _, prefix := range []string{"/api/v1", "/api", "/v1"} {
		if p == prefix {
			return "/"
		}
		if strings.HasPrefix(p, prefix+"/") {
			return p[len(prefix):]
		}
	}
	return p
}

func isPublicAuthPath(p string) bool {
	if p == "/healthz" {
		return true
	}
	canonical := stripRoutePrefix(p)
	if _, ok := publicAuthPaths[canonical]; ok {
		return true
	}
	return false
}

// requireAuth enforces AUTH_MODE=enabled on all non-public endpoints. When
// AUTH_MODE is disabled (the default) the middleware is a no-op so existing
// trusted-LAN behavior remains available explicitly.
func (a *app) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authMode() != "enabled" {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicAuthPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		// Helper protocol routes perform their own app-password and device
		// authorization in authorizeHelperSyncRequest.
		if isHelperProtocolRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		if a.isAuthenticatedRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, apiError{
			Error:      "Unauthorized",
			Message:    "authentication required",
			StatusCode: http.StatusUnauthorized,
		})
	})
}

func isHelperProtocolRequest(r *http.Request) bool {
	if !hasHelperIdentity(r) {
		return false
	}
	p := stripRoutePrefix(r.URL.Path)
	switch r.Method {
	case http.MethodGet:
		if p == "/save/latest" ||
			p == "/saves/download" ||
			p == "/saves/download_many" ||
			p == "/saves/download-many" {
			return true
		}
		return strings.HasPrefix(p, "/saves/") && strings.HasSuffix(p, "/download")
	case http.MethodPost:
		return p == "/saves" ||
			p == "/devices/config/report" ||
			p == "/helpers/config/sync" ||
			p == "/helpers/heartbeat"
	default:
		return false
	}
}

func hasHelperIdentity(r *http.Request) bool {
	dt := strings.TrimSpace(r.Header.Get("X-RSM-Device-Type"))
	fp := strings.TrimSpace(r.Header.Get("X-RSM-Fingerprint"))
	return dt != "" && fp != ""
}

// isAuthenticatedRequest returns true only for an explicitly trusted proxy
// identity, a server-side session created by a successful login, or a valid
// helper app-password. Merely supplying an arbitrary non-empty session cookie
// is never sufficient.
func (a *app) isAuthenticatedRequest(r *http.Request) bool {
	if trustRemoteUserHeader() && strings.TrimSpace(r.Header.Get("Remote-User")) != "" {
		return true
	}

	if cookie, err := r.Cookie("session"); err == nil && validateWebSession(cookie.Value, time.Now().UTC()) {
		return true
	}

	raw := extractHelperAppPassword(r, nil)
	if strings.TrimSpace(raw) != "" {
		if _, compact, ok := normalizeAppPasswordInput(raw); ok {
			a.mu.Lock()
			_, found := a.findAppPasswordByCompactLocked(compact)
			a.mu.Unlock()
			if found {
				return true
			}
		}
	}

	return false
}
