package main

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

func TestAuthModeDefaultsToDisabled(t *testing.T) {
	t.Setenv("AUTH_MODE", "")
	if got := authMode(); got != "disabled" {
		t.Fatalf("authMode default mismatch: got %q want %q", got, "disabled")
	}
}

func TestAuthModeNormalizesCaseAndWhitespace(t *testing.T) {
	t.Setenv("AUTH_MODE", "  DiSaBlEd  ")
	if got := authMode(); got != "disabled" {
		t.Fatalf("authMode normalization mismatch: got %q want %q", got, "disabled")
	}
}

func TestBaseURLForRequestPrefersBaseURL(t *testing.T) {
	t.Setenv("BASE_URL", "https://retro.internal/base/")
	t.Setenv("PUBLIC_HOST", "retro.local")
	t.Setenv("TLS_ENABLED", "true")

	req := httptest.NewRequest("GET", "http://example.local/auth/device", nil)
	if got := baseURLForRequest(req); got != "https://retro.internal/base" {
		t.Fatalf("baseURLForRequest mismatch: got %q", got)
	}
}

func TestBaseURLForRequestBuildsFromPublicHostAndTLSFlag(t *testing.T) {
	t.Setenv("BASE_URL", "")
	t.Setenv("PUBLIC_HOST", "retro.lan")
	t.Setenv("TLS_ENABLED", "true")

	req := httptest.NewRequest("GET", "http://example.local/auth/device", nil)
	if got := baseURLForRequest(req); got != "https://retro.lan" {
		t.Fatalf("baseURLForRequest mismatch: got %q", got)
	}
}

func TestBaseURLForRequestUsesPublicHostSchemeWhenProvided(t *testing.T) {
	t.Setenv("BASE_URL", "")
	t.Setenv("PUBLIC_HOST", "http://retro.internal:8080/")
	t.Setenv("TLS_ENABLED", "true")

	req := httptest.NewRequest("GET", "http://example.local/auth/device", nil)
	if got := baseURLForRequest(req); got != "http://retro.internal:8080" {
		t.Fatalf("baseURLForRequest mismatch: got %q", got)
	}
}

func TestBaseURLForRequestFallsBackToForwardedProtoAndHost(t *testing.T) {
	t.Setenv("BASE_URL", "")
	t.Setenv("PUBLIC_HOST", "")
	t.Setenv("TLS_ENABLED", "false")

	req := httptest.NewRequest("GET", "http://example.local/auth/device", nil)
	req.Host = "rsm.local:8081"
	req.Header.Set("X-Forwarded-Proto", "https")

	if got := baseURLForRequest(req); got != "https://rsm.local:8081" {
		t.Fatalf("baseURLForRequest mismatch: got %q", got)
	}
}

func TestBaseURLForRequestFallsBackToTLSState(t *testing.T) {
	t.Setenv("BASE_URL", "")
	t.Setenv("PUBLIC_HOST", "")
	t.Setenv("TLS_ENABLED", "false")

	req := httptest.NewRequest("GET", "http://example.local/auth/device", nil)
	req.Host = "rsm.local"
	req.TLS = &tls.ConnectionState{}

	if got := baseURLForRequest(req); got != "https://rsm.local" {
		t.Fatalf("baseURLForRequest mismatch: got %q", got)
	}
}

func TestBaseURLForRequestFallbackWithoutRequest(t *testing.T) {
	t.Setenv("BASE_URL", "")
	t.Setenv("PUBLIC_HOST", "")
	t.Setenv("TLS_ENABLED", "false")

	if got := baseURLForRequest(nil); got != "http://localhost:3001" {
		t.Fatalf("baseURLForRequest mismatch: got %q", got)
	}
}
