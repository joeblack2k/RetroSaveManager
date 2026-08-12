package main

import (
	"net/http"
	"strings"
	"time"
)

// publicAuthPaths lists the canonical (un-prefixed) endpoint paths that must
// remain reachable without authentication even when AUTH_MODE=enabled. These
// are the bootstrap endpoints — without them a fresh client could never log
// in, fetch runtime config, or complete a device-pairing flow.
//
// Paths are matched after stripping any of the well-known route prefixes
// (/api/v1, /api, /v1) via stripRoutePrefix, so each entry needs to be listed
// only once.
//
// NOTE: /healthz is intentionally NOT in this map. It is registered at the
// router root only (NOT under any compat-mount prefix), so allowing
// /api/healthz / /v1/healthz / etc. through the allowlist would let those
// paths skip auth even though they have no real route — leaking 404s and,
// worse, falling through to the static-frontend handler. /healthz is matched
// as an exact path in isPublicAuthPath instead.
var publicAuthPaths = map[string]struct{}{
	"/runtime-config":           {},
	"/auth/login":               {},
	"/auth/signup":              {},
	"/auth/token":               {},
	"/auth/token/app-password":  {},
	"/auth/resend-verification": {},
	"/auth/verify-email":        {},
	"/auth/forgot-password":     {},
	"/auth/reset-password":      {},
	"/auth/device":              {},
	"/auth/device/token":        {},
	"/auth/device/verify":       {},
	"/auth/device/confirm":      {},
	"/auth/2fa/verify":          {},
	"/auth/2fa/setup/totp":      {},
	"/auth/2fa/verify-setup":    {},
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
