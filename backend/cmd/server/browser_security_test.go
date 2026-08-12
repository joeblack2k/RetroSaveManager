package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPublicAuthSurfaceIsMinimal(t *testing.T) {
	for _, path := range []string{
		"/auth/login",
		"/v1/auth/login",
		"/auth/token/app-password",
		"/auth/device",
		"/auth/device/token",
		"/runtime-config",
		"/healthz",
	} {
		if !isPublicAuthPath(path) {
			t.Fatalf("expected bootstrap path %q to be public", path)
		}
	}

	for _, path := range []string{
		"/auth/signup",
		"/auth/token",
		"/auth/forgot-password",
		"/auth/reset-password",
		"/auth/device/verify",
		"/auth/device/confirm",
		"/auth/2fa/setup/totp",
		"/auth/2fa/verify-setup",
	} {
		if isPublicAuthPath(path) {
			t.Fatalf("unexpected public auth path %q", path)
		}
	}
}

func TestBrowserSessionWriteRequiresCSRFHeader(t *testing.T) {
	h := newContractHarness(t)
	configureTestAdmin(t)

	token, _, err := createWebSession("admin@example.invalid", time.Now().UTC())
	if err != nil {
		t.Fatalf("create web session: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rr := h.do(req)
	assertStatus(t, rr, http.StatusForbidden)
}

func TestBrowserSessionWriteWithCSRFHeaderSucceeds(t *testing.T) {
	h := newContractHarness(t)
	configureTestAdmin(t)

	token, _, err := createWebSession("admin@example.invalid", time.Now().UTC())
	if err != nil {
		t.Fatalf("create web session: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/app-passwords/auto-enroll", strings.NewReader(`{"minutes":15}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfProtectionHeader, csrfProtectionExpected)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

func TestBrowserSessionSafeReadDoesNotRequireCSRFHeader(t *testing.T) {
	h := newContractHarness(t)
	configureTestAdmin(t)

	token, _, err := createWebSession("admin@example.invalid", time.Now().UTC())
	if err != nil {
		t.Fatalf("create web session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rr := h.do(req)
	assertStatus(t, rr, http.StatusOK)
}

func TestLoginThrottleBlocksAfterRepeatedFailures(t *testing.T) {
	resetLoginThrottleForTest()
	t.Cleanup(resetLoginThrottleForTest)
	now := time.Date(2026, 8, 12, 16, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "192.0.2.10:43210"

	for attempt := 1; attempt < loginFailureLimit; attempt++ {
		if retry := recordFailedLogin(req, "admin@example.invalid", now.Add(time.Duration(attempt)*time.Second)); retry != 0 {
			t.Fatalf("attempt %d blocked too early: %v", attempt, retry)
		}
	}
	retry := recordFailedLogin(req, "admin@example.invalid", now.Add(time.Duration(loginFailureLimit)*time.Second))
	if retry <= 0 || retry > loginBlockDuration {
		t.Fatalf("expected block duration after threshold, got %v", retry)
	}
	if got := loginRetryAfter(req, "admin@example.invalid", now.Add(10*time.Second)); got <= 0 {
		t.Fatal("expected subsequent login to remain rate limited")
	}
}

func TestLoginThrottleIsScopedByClientAndAccount(t *testing.T) {
	resetLoginThrottleForTest()
	t.Cleanup(resetLoginThrottleForTest)
	now := time.Date(2026, 8, 12, 16, 0, 0, 0, time.UTC)
	blocked := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	blocked.RemoteAddr = "192.0.2.10:43210"
	for attempt := 0; attempt < loginFailureLimit; attempt++ {
		recordFailedLogin(blocked, "admin@example.invalid", now.Add(time.Duration(attempt)*time.Second))
	}

	otherIP := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	otherIP.RemoteAddr = "192.0.2.11:43210"
	if retry := loginRetryAfter(otherIP, "admin@example.invalid", now.Add(10*time.Second)); retry != 0 {
		t.Fatalf("different client IP was unexpectedly blocked: %v", retry)
	}
	if retry := loginRetryAfter(blocked, "other@example.invalid", now.Add(10*time.Second)); retry != 0 {
		t.Fatalf("different account was unexpectedly blocked: %v", retry)
	}
}

func TestLoginRateLimitedResponseIncludesRetryAfter(t *testing.T) {
	rr := httptest.NewRecorder()
	writeLoginRateLimited(rr, 1500*time.Millisecond)
	assertStatus(t, rr, http.StatusTooManyRequests)
	seconds, err := strconv.Atoi(rr.Header().Get("Retry-After"))
	if err != nil || seconds != 2 {
		t.Fatalf("unexpected Retry-After header: %q", rr.Header().Get("Retry-After"))
	}
}

func TestProxyHeadersAreOptIn(t *testing.T) {
	t.Setenv("TRUST_PROXY_HEADERS", "false")
	if trustProxyHeaders() {
		t.Fatal("proxy headers must not be trusted by default")
	}
	t.Setenv("TRUST_PROXY_HEADERS", "true")
	if !trustProxyHeaders() {
		t.Fatal("explicit proxy-header trust should be honored")
	}
}
