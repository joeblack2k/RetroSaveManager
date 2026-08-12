package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func configureTestAdmin(t *testing.T) {
	t.Helper()
	t.Setenv("AUTH_MODE", "enabled")
	t.Setenv("RSM_ADMIN_EMAIL", "admin@example.invalid")
	t.Setenv("RSM_ADMIN_PASSWORD", "correct-horse-battery-staple")
	t.Setenv("TRUST_REMOTE_USER_HEADER", "false")
	t.Setenv("AUTH_COOKIE_SECURE", "false")
}

func TestValidateAuthConfigurationFailsClosed(t *testing.T) {
	t.Setenv("AUTH_MODE", "enabled")
	t.Setenv("RSM_ADMIN_EMAIL", "")
	t.Setenv("RSM_ADMIN_PASSWORD", "")
	t.Setenv("TRUST_REMOTE_USER_HEADER", "false")
	if err := validateAuthConfiguration(); err == nil {
		t.Fatal("expected AUTH_MODE=enabled without credentials to fail closed")
	}

	t.Setenv("TRUST_REMOTE_USER_HEADER", "true")
	if err := validateAuthConfiguration(); err != nil {
		t.Fatalf("expected explicit trusted-proxy auth mode to be accepted: %v", err)
	}
}

func TestValidateAuthConfigurationRejectsShortPassword(t *testing.T) {
	t.Setenv("AUTH_MODE", "enabled")
	t.Setenv("RSM_ADMIN_EMAIL", "admin@example.invalid")
	t.Setenv("RSM_ADMIN_PASSWORD", "short")
	if err := validateAuthConfiguration(); err == nil {
		t.Fatal("expected short admin password to be rejected")
	}
}

func TestAuthenticateConfiguredAdminRequiresBothCredentials(t *testing.T) {
	configureTestAdmin(t)
	if !authenticateConfiguredAdmin("ADMIN@example.invalid", "correct-horse-battery-staple") {
		t.Fatal("expected configured admin credentials to authenticate")
	}
	if authenticateConfiguredAdmin("admin@example.invalid", "wrong-password") {
		t.Fatal("expected wrong password to be rejected")
	}
	if authenticateConfiguredAdmin("other@example.invalid", "correct-horse-battery-staple") {
		t.Fatal("expected wrong email to be rejected")
	}
}

func TestWebSessionPersistsOnlyTokenHashAndExpires(t *testing.T) {
	t.Setenv("STATE_ROOT", t.TempDir())
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	raw, expiresAt, err := createWebSession("admin@example.invalid", now)
	if err != nil {
		t.Fatalf("create web session: %v", err)
	}
	if raw == "" || len(raw) != 64 {
		t.Fatalf("unexpected raw session token length: %d", len(raw))
	}
	if !expiresAt.Equal(now.Add(webSessionLifetime)) {
		t.Fatalf("unexpected expiry: %s", expiresAt)
	}

	data, err := os.ReadFile(filepath.Join(os.Getenv("STATE_ROOT"), webSessionStateFileName))
	if err != nil {
		t.Fatalf("read persisted web session: %v", err)
	}
	if strings.Contains(string(data), raw) {
		t.Fatal("raw session token must never be persisted")
	}
	var state webSessionStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode persisted web session: %v", err)
	}
	if len(state.Sessions) != 1 || state.Sessions[0].TokenHash != webSessionTokenHash(raw) {
		t.Fatalf("unexpected persisted session state: %+v", state)
	}
	if !validateWebSession(raw, now.Add(time.Hour)) {
		t.Fatal("expected live session to validate")
	}
	if validateWebSession(raw, expiresAt) {
		t.Fatal("expected session to be invalid at its expiry instant")
	}
}

func TestAuthLoginRejectsInvalidCredentialsWithoutCookie(t *testing.T) {
	h := newContractHarness(t)
	configureTestAdmin(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"admin@example.invalid","password":"wrong-password"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
	if cookies := rr.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("invalid login must not set cookies: %+v", cookies)
	}
}

func TestAuthLoginCreatesServerSideSessionForProtectedRoutes(t *testing.T) {
	h := newContractHarness(t)
	configureTestAdmin(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"admin@example.invalid","password":"correct-horse-battery-staple"}`))
	req.Header.Set("Content-Type", "application/json")
	login := h.do(req)
	assertStatus(t, login, http.StatusOK)

	var sessionCookie *http.Cookie
	for _, cookie := range login.Result().Cookies() {
		if cookie.Name == "session" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("expected successful login to set an opaque session cookie")
	}

	protected := rawRequest(http.MethodGet, "/api/saves")
	protected.AddCookie(sessionCookie)
	protectedResponse := h.do(protected)
	assertStatus(t, protectedResponse, http.StatusOK)
}

func TestAuthMiddlewareRejectsArbitrarySessionCookie(t *testing.T) {
	h := newContractHarness(t)
	configureTestAdmin(t)

	req := rawRequest(http.MethodGet, "/api/saves")
	req.AddCookie(&http.Cookie{Name: "session", Value: "anything"})
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestAuthLogoutRevokesServerSideSession(t *testing.T) {
	h := newContractHarness(t)
	configureTestAdmin(t)

	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"admin@example.invalid","password":"correct-horse-battery-staple"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	login := h.do(loginReq)
	assertStatus(t, login, http.StatusOK)
	cookies := login.Result().Cookies()
	var rawSession string
	for _, cookie := range cookies {
		if cookie.Name == "session" {
			rawSession = cookie.Value
			break
		}
	}
	if rawSession == "" {
		t.Fatal("missing session cookie")
	}
	if !validateWebSession(rawSession, time.Now().UTC()) {
		t.Fatal("new session should be valid before logout")
	}

	logoutReq := rawRequest(http.MethodPost, "/auth/logout")
	logoutReq.AddCookie(&http.Cookie{Name: "session", Value: rawSession})
	logout := h.do(logoutReq)
	assertStatus(t, logout, http.StatusOK)
	if validateWebSession(rawSession, time.Now().UTC()) {
		t.Fatal("logged-out session must be revoked server-side")
	}

	replay := rawRequest(http.MethodGet, "/api/saves")
	replay.AddCookie(&http.Cookie{Name: "session", Value: rawSession})
	replayResponse := h.do(replay)
	assertStatus(t, replayResponse, http.StatusUnauthorized)
}

func TestCorruptWebSessionStateFailsClosed(t *testing.T) {
	h := newContractHarness(t)
	configureTestAdmin(t)
	if err := os.WriteFile(webSessionStateFilePathFromEnv(), []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt session state: %v", err)
	}

	req := rawRequest(http.MethodGet, "/api/saves")
	req.AddCookie(&http.Cookie{Name: "session", Value: "attacker-controlled"})
	rr := h.do(req)
	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestSessionCookieSecureFollowsHTTPSBaseURL(t *testing.T) {
	t.Setenv("AUTH_COOKIE_SECURE", "")
	t.Setenv("BASE_URL", "https://saves.example.invalid")
	req := httptest.NewRequest(http.MethodGet, "http://internal/", nil)
	if !sessionCookieSecure(req) {
		t.Fatal("expected HTTPS public base URL to produce Secure session cookies")
	}
}
