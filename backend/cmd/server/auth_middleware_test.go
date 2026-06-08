package main

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// authMiddlewareHarness builds a contract harness, optionally toggling
// AUTH_MODE=enabled. Because t.Setenv resets the value at test end, each
// subtest can opt in or out of enforcement independently.
func authMiddlewareHarness(t *testing.T, enabled bool) *contractHarness {
	t.Helper()
	h := newContractHarness(t)
	if enabled {
		t.Setenv("AUTH_MODE", "enabled")
	}
	return h
}

// mintAppPassword creates a real app-password record on the harness app and
// returns the compact (XXX-XXX-formatted) plain-text key. The HTTP layer is
// not used so the call works regardless of AUTH_MODE.
func mintAppPassword(t *testing.T, h *contractHarness) string {
	t.Helper()
	h.app.mu.Lock()
	_, key := h.app.createAppPasswordLocked("middleware-test", time.Now().UTC())
	h.app.mu.Unlock()
	if _, _, ok := normalizeAppPasswordInput(key); !ok {
		t.Fatalf("created app password has unexpected format: %q", key)
	}
	return formatAppPasswordCompact(key)
}

func rawRequest(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-CSRF-Protection", "1")
	return req
}

// --- AUTH_MODE=disabled (baseline — must keep existing behavior) ---

func TestAuthMiddleware_Disabled_AllowsApiSavesWithoutAuth(t *testing.T) {
	h := authMiddlewareHarness(t, false)
	rr := h.do(rawRequest(http.MethodGet, "/api/saves"))
	assertStatus(t, rr, http.StatusOK)
}

// --- AUTH_MODE=enabled — enforcement ---

func TestAuthMiddleware_Enabled_BlocksApiSavesWithoutAuth(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	rr := h.do(rawRequest(http.MethodGet, "/api/saves"))
	assertStatus(t, rr, http.StatusUnauthorized)
	assertJSONContentType(t, rr)
}

func TestAuthMiddleware_Enabled_AllowsValidBearerAppPassword(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	key := mintAppPassword(t, h)
	req := rawRequest(http.MethodGet, "/api/saves")
	req.Header.Set("Authorization", "Bearer "+key)
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

func TestAuthMiddleware_Enabled_AllowsValidXRSMAppPasswordHeader(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	key := mintAppPassword(t, h)
	req := rawRequest(http.MethodGet, "/api/saves")
	req.Header.Set("X-RSM-App-Password", key)
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

func TestAuthMiddleware_Enabled_AllowsRemoteUserHeader(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	t.Setenv("TRUST_REMOTE_USER_HEADER", "true")
	req := rawRequest(http.MethodGet, "/api/saves")
	req.Header.Set("Remote-User", "alice")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

func TestAuthMiddleware_Enabled_AllowsNonEmptySessionCookie(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	req := rawRequest(http.MethodGet, "/api/saves")
	req.AddCookie(&http.Cookie{Name: "session", Value: "anything"})
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

// --- AUTH_MODE=enabled — allowlist (bootstrap endpoints) ---

func TestAuthMiddleware_Enabled_AllowsHealthzWithoutAuth(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	rr := h.do(rawRequest(http.MethodGet, "/healthz"))
	assertStatus(t, rr, http.StatusOK)
}

func TestAuthMiddleware_Enabled_AllowsRuntimeConfigWithoutAuth(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	rr := h.do(rawRequest(http.MethodGet, "/runtime-config"))
	assertStatus(t, rr, http.StatusOK)
}

func TestAuthMiddleware_Enabled_AllowsAuthLoginWithoutAuth(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Protection", "1")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

// --- AUTH_MODE=enabled — bad credentials still rejected ---

func TestAuthMiddleware_Enabled_RejectsGarbageBearer(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	req := rawRequest(http.MethodGet, "/api/saves")
	// Format-valid (six A-Z0-9 chars) but not bound to any real password.
	req.Header.Set("Authorization", "Bearer ZZZZZZ")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
}

// --- AUTH_MODE=enabled — helper-identity defer-to-handler behavior ---
//
// These tests cover the bug where the middleware would 401 helper requests
// that have identity headers but no app-password, breaking the auto-enroll
// bootstrap flow. The middleware MUST defer to authorizeHelperSyncRequest
// (in helper_auth.go), which enforces the actual policy.

// helperMultipartSaveRequest builds a POST /saves multipart request that
// looks like a helper upload — identity carried via headers, not form fields,
// so the middleware's hasHelperIdentity check is exercised. Optionally sets
// an Authorization Bearer with the supplied app-password.
func helperMultipartSaveRequest(t *testing.T, deviceType, fingerprint, appPassword string) *http.Request {
	t.Helper()

	fields := map[string]string{
		"rom_sha1":       "middleware-helper-rom",
		"slotName":       "default",
		"system":         "snes",
		"runtimeProfile": "snes/snes9x",
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write multipart field %s: %v", key, err)
		}
	}
	part, err := writer.CreateFormFile("file", "Final Fantasy VI.srm")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	payload := normalizeTestUploadPayload(fields, "Final Fantasy VI.srm", []byte("helper-middleware-payload"))
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("write multipart payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/saves", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Protection", "1")
	req.Header.Set("X-RSM-Device-Type", deviceType)
	req.Header.Set("X-RSM-Fingerprint", fingerprint)
	if appPassword != "" {
		req.Header.Set("Authorization", "Bearer "+appPassword)
	}
	return req
}

func TestAuthMiddleware_Enabled_HelperHeadersNoKey_AutoEnrollOpen_Allows(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	// Open the auto-enroll window via the normal handler path.
	h.app.mu.Lock()
	h.app.enableAutoAppPasswordWindowLocked(15 * time.Minute)
	h.app.mu.Unlock()

	req := helperMultipartSaveRequest(t, "linux-x86", "deck-middleware-open", "")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
	if got := rr.Header().Get("X-RSM-Auto-App-Password"); got == "" {
		t.Fatalf("expected auto-provisioned app password header on bootstrap; headers=%v", rr.Header())
	}
}

func TestAuthMiddleware_Enabled_HelperHeadersNoKey_AutoEnrollClosed_Rejects(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	// Default state: auto-enroll window is NOT open.

	req := helperMultipartSaveRequest(t, "linux-x86", "deck-middleware-closed", "")
	rr := h.do(req)
	// The middleware passes through; the downstream handler enforces the
	// no-key + closed-window policy with 401.
	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestAuthMiddleware_Enabled_HelperHeadersValidKey_Allows(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	key := mintAppPassword(t, h)

	req := helperMultipartSaveRequest(t, "linux-x86", "deck-middleware-key", key)
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

// Regression test for the GET /api/saves + valid Bearer path. The empirical
// repro that surfaced the bootstrap bug had an empty Bearer (enrollment had
// failed), so this confirms the happy path independently.
func TestAuthMiddleware_Enabled_GetApiSavesWithValidBearer_Allows(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	key := mintAppPassword(t, h)

	req := rawRequest(http.MethodGet, "/api/saves")
	req.Header.Set("Authorization", "Bearer "+key)
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

// --- AUTH_MODE=enabled — path-prefix variants of allowlisted endpoints ---
//
// The compat router mounts the public-auth endpoints under "" and /v1. The
// agent router mounts a separate /runtime-config under /api and /api/v1, but
// does NOT re-expose /auth/login. The middleware's stripRoutePrefix must let
// each variant that actually has a backing route through without auth.

func TestAuthMiddleware_Enabled_AllowsRuntimeConfigPrefixVariants(t *testing.T) {
	for _, path := range []string{
		"/runtime-config",
		"/v1/runtime-config",
		"/api/runtime-config",
		"/api/v1/runtime-config",
	} {
		t.Run(path, func(t *testing.T) {
			h := authMiddlewareHarness(t, true)
			rr := h.do(rawRequest(http.MethodGet, path))
			assertStatus(t, rr, http.StatusOK)
		})
	}
}

func TestAuthMiddleware_Enabled_AllowsAuthLoginPrefixVariants(t *testing.T) {
	// Compat-router mounts only — /api(/v1)/auth/login has no backing route
	// and is intentionally not part of the agent surface.
	for _, path := range []string{
		"/auth/login",
		"/v1/auth/login",
	} {
		t.Run(path, func(t *testing.T) {
			h := authMiddlewareHarness(t, true)
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Protection", "1")
			rr := h.do(req)
			assertStatus(t, rr, http.StatusOK)
		})
	}
}

// /healthz is registered at the router ROOT only — NOT under any compat mount.
// Only the exact path is allowlisted; /api/healthz / /v1/healthz / etc. must
// be treated as non-bootstrap traffic and rejected with 401.
func TestAuthMiddleware_Enabled_HealthzAllowlistIsRootOnly(t *testing.T) {
	h := authMiddlewareHarness(t, true)

	rrRoot := h.do(rawRequest(http.MethodGet, "/healthz"))
	assertStatus(t, rrRoot, http.StatusOK)

	for _, path := range []string{
		"/api/healthz",
		"/v1/healthz",
		"/api/v1/healthz",
	} {
		t.Run(path, func(t *testing.T) {
			h := authMiddlewareHarness(t, true)
			rr := h.do(rawRequest(http.MethodGet, path))
			assertStatus(t, rr, http.StatusUnauthorized)
		})
	}
}

// --- AUTH_MODE=enabled — spoof-prevention edge cases ---
//
// Empty / whitespace-only Remote-User and session cookies must NOT be
// accepted as proof of authentication, regardless of how they got there
// (a misconfigured proxy that always sets Remote-User="" is the most
// realistic source).

func TestAuthMiddleware_Enabled_RejectsEmptyRemoteUserHeader(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	t.Setenv("TRUST_REMOTE_USER_HEADER", "true")
	req := rawRequest(http.MethodGet, "/api/saves")
	req.Header.Set("Remote-User", "")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestAuthMiddleware_Enabled_RejectsWhitespaceRemoteUserHeader(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	t.Setenv("TRUST_REMOTE_USER_HEADER", "true")
	req := rawRequest(http.MethodGet, "/api/saves")
	req.Header.Set("Remote-User", "    ")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestAuthMiddleware_Enabled_RejectsEmptySessionCookie(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	req := rawRequest(http.MethodGet, "/api/saves")
	req.AddCookie(&http.Cookie{Name: "session", Value: ""})
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestAuthMiddleware_Enabled_RejectsWhitespaceSessionCookie(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	req := rawRequest(http.MethodGet, "/api/saves")
	req.AddCookie(&http.Cookie{Name: "session", Value: "    "})
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
}

// --- AUTH_MODE=enabled — SSE /events endpoint ---
//
// /events serves text/event-stream and leaks live state if exposed.
// It must require auth in every prefix-mount variant.

func TestAuthMiddleware_Enabled_BlocksEventsWithoutAuth(t *testing.T) {
	for _, path := range []string{
		"/events",
		"/api/events",
	} {
		t.Run(path, func(t *testing.T) {
			h := authMiddlewareHarness(t, true)
			rr := h.do(rawRequest(http.MethodGet, path))
			assertStatus(t, rr, http.StatusUnauthorized)
		})
	}
}

// With a valid Bearer the SSE handler should at least send response headers
// with the SSE content-type before we tear the connection down. We use the
// existing ssePrelude helper which cancels right after seeing the initial
// ": connected\n\n" frame, so the test doesn't hang on the long-lived stream.
func TestAuthMiddleware_Enabled_AllowsEventsWithValidBearer(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	key := mintAppPassword(t, h)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	req.Header.Set("X-CSRF-Protection", "1")
	req.Header.Set("Authorization", "Bearer "+key)

	rr := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.handler.ServeHTTP(rr, req)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if strings.HasPrefix(rr.Body.String(), ": connected\n\n") {
			cancel()
			<-done
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatalf("timed out waiting for SSE prelude; status=%d headers=%v body=%q",
				rr.Code, rr.Header(), rr.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	if rr.Code != http.StatusOK {
		t.Fatalf("expected SSE 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected Content-Type to contain text/event-stream, got %q", ct)
	}
}

// --- AUTH_MODE=enabled — static frontend behavior ---
//
// The chi NotFound handler serves the SPA shell for any GET that isn't a
// reserved API path. With AUTH_MODE=enabled the auth middleware fires BEFORE
// NotFound, so unauthenticated SPA loads receive 401. This is intentional:
// when AUTH_MODE=enabled the operator is expected to either expose the UI
// behind a forwardAuth proxy (Authelia) or accept that the SPA is gated.
//
// These tests pin the documented behavior so it can't silently regress.

// staticFrontendHarness installs a minimal dist/index.html + dist/assets/app.js
// so the static-frontend handler is wired up. Returns the harness.
func staticFrontendHarness(t *testing.T, enabled bool) *contractHarness {
	t.Helper()
	distDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(filepath.Join(distDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "index.html"),
		[]byte("<!doctype html><html><body>RSM</body></html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "assets", "app.js"),
		[]byte("console.log('rsm');"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	t.Setenv("FRONTEND_DIST_DIR", distDir)
	return authMiddlewareHarness(t, enabled)
}

func TestAuthMiddleware_Enabled_BlocksStaticRootWithoutAuth(t *testing.T) {
	h := staticFrontendHarness(t, true)
	rr := h.do(rawRequest(http.MethodGet, "/"))
	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestAuthMiddleware_Enabled_BlocksStaticAssetWithoutAuth(t *testing.T) {
	h := staticFrontendHarness(t, true)
	rr := h.do(rawRequest(http.MethodGet, "/assets/app.js"))
	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestAuthMiddleware_Enabled_AllowsStaticRootWithRemoteUser(t *testing.T) {
	h := staticFrontendHarness(t, true)
	t.Setenv("TRUST_REMOTE_USER_HEADER", "true")
	req := rawRequest(http.MethodGet, "/")
	req.Header.Set("Remote-User", "alice")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

func TestAuthMiddleware_Disabled_AllowsStaticRoot(t *testing.T) {
	h := staticFrontendHarness(t, false)
	rr := h.do(rawRequest(http.MethodGet, "/"))
	assertStatus(t, rr, http.StatusOK)
}

// --- AUTH_MODE=enabled — TRUST_REMOTE_USER_HEADER opt-in ---
//
// Remote-User is only honored when the operator explicitly opts in. Default
// off is the safe-by-default posture: a WAN-exposed RSM with no stripping
// reverse proxy would otherwise trust any client-supplied header.

func TestAuthMiddleware_Enabled_RemoteUserIgnoredByDefault(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	// TRUST_REMOTE_USER_HEADER intentionally unset.
	req := rawRequest(http.MethodGet, "/api/saves")
	req.Header.Set("Remote-User", "alice")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestAuthMiddleware_Enabled_RemoteUserHonoredWhenTrusted(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	t.Setenv("TRUST_REMOTE_USER_HEADER", "true")
	req := rawRequest(http.MethodGet, "/api/saves")
	req.Header.Set("Remote-User", "alice")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

func TestAuthMiddleware_Enabled_RemoteUserIgnoredWhenExplicitlyFalse(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	t.Setenv("TRUST_REMOTE_USER_HEADER", "false")
	req := rawRequest(http.MethodGet, "/api/saves")
	req.Header.Set("Remote-User", "alice")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
}

// TRUST_REMOTE_USER_HEADER=true + empty/whitespace Remote-User must still
// reject — the trim check is the last line of defense if a misconfigured
// proxy strips the user but leaves the header in place.
func TestAuthMiddleware_Enabled_RemoteUserTrustedButEmptyRejects(t *testing.T) {
	h := authMiddlewareHarness(t, true)
	t.Setenv("TRUST_REMOTE_USER_HEADER", "true")
	req := rawRequest(http.MethodGet, "/api/saves")
	req.Header.Set("Remote-User", "   ")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
}
