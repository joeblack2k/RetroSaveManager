package main

import (
	"net/http"
	"strings"
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

// stripRoutePrefix removes one of the well-known router prefixes
// (/api/v1, /api, /v1) from p, returning the canonical sub-path. The
// longest prefix is tried first so that /api/v1/... is not misread as
// /api/... with a stray /v1 segment.
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

// isPublicAuthPath reports whether p is one of the bootstrap endpoints that
// must remain reachable without authentication. The check is prefix-aware so
// that, e.g., /api/v1/auth/login matches the canonical /auth/login entry.
//
// /healthz is handled as a special case: only the exact "/healthz" path is
// allowlisted. /api/healthz, /v1/healthz, etc. are NOT — they have no real
// route and matching them would invite the static-frontend NotFound fallback.
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
// behavior — and the existing test suite — is preserved verbatim.
//
// When AUTH_MODE=enabled the request is allowed through if ANY of the
// following are true:
//
//  1. The path is on the bootstrap allowlist (isPublicAuthPath).
//  2. The request carries a valid helper app-password (Bearer token,
//     X-RSM-App-Password header, or app_password form field), validated
//     against the existing app-password store.
//  3. The request carries a non-empty Remote-User header set by an upstream
//     forwardAuth reverse proxy (e.g. Authelia, oauth2-proxy). The mere
//     presence is trusted — the proxy is responsible for the verification.
//  4. The request carries a non-empty `session` cookie. This matches the
//     existing /auth/login handler which sets such a cookie without a
//     server-side store. TODO: replace with a real session store backed by
//     security_state.
//
// Otherwise the request is rejected with 401 Unauthorized + an apiError
// JSON body.
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
		// Helper protocol: only endpoints that actually call
		// authorizeHelperSyncRequest may defer auth to the downstream handler.
		// Checking identity headers alone would let callers spoof those headers
		// and bypass AUTH_MODE on unrelated API/static routes.
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

// isHelperProtocolRequest reports whether r is one of the helper-facing routes
// whose handler performs authorizeHelperSyncRequest. Identity headers alone are
// not sufficient; non-helper routes must still require normal auth.
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

// hasHelperIdentity reports whether r carries both helper-identity headers
// (X-RSM-Device-Type + X-RSM-Fingerprint).
func hasHelperIdentity(r *http.Request) bool {
	dt := strings.TrimSpace(r.Header.Get("X-RSM-Device-Type"))
	fp := strings.TrimSpace(r.Header.Get("X-RSM-Fingerprint"))
	return dt != "" && fp != ""
}

// isAuthenticatedRequest returns true if r presents any acceptable credential
// when AUTH_MODE=enabled. See requireAuth for the accepted forms.
func (a *app) isAuthenticatedRequest(r *http.Request) bool {
	// 1. Reverse-proxy forwardAuth (Authelia, oauth2-proxy, etc).
	// Only honored when the operator has explicitly opted in via
	// TRUST_REMOTE_USER_HEADER — otherwise a WAN-exposed RSM would trust
	// any client-supplied Remote-User header, which is trivially spoofable.
	if trustRemoteUserHeader() && strings.TrimSpace(r.Header.Get("Remote-User")) != "" {
		return true
	}

	// 2. Session cookie set by /auth/login. The current login handler does
	// not persist a server-side session, so any non-empty value is treated
	// as authenticated — this matches existing behavior. See TODO above.
	if cookie, err := r.Cookie("session"); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return true
	}

	// 3. Helper app-password (Bearer / X-RSM-App-Password / form field).
	// extractHelperAppPassword returns the raw caller-supplied value.
	// We normalize and look it up against the app-password store.
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
