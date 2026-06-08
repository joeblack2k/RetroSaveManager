package main

import (
	"net/http"
	"os"
	"strings"
)

func authMode() string {
	mode := strings.TrimSpace(strings.ToLower(os.Getenv("AUTH_MODE")))
	if mode == "" {
		return "disabled"
	}
	return mode
}

// trustRemoteUserHeader reports whether the Remote-User header should be
// trusted as proof of authentication when AUTH_MODE=enabled.
//
// Default is FALSE — a deployment that exposes RSM directly to the WAN
// without a stripping reverse proxy in front would otherwise be trivially
// spoofable by any client that sets the header. Operators who DO have a
// trusted forwardAuth proxy (Authelia, oauth2-proxy, Authentik, etc.) in
// front and want SSO to work must opt in explicitly by setting
// TRUST_REMOTE_USER_HEADER=true (or "yes" / "1").
func trustRemoteUserHeader() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("TRUST_REMOTE_USER_HEADER")))
	switch v {
	case "true", "yes", "1":
		return true
	default:
		return false
	}
}

func baseURLForRequest(r *http.Request) string {
	if env := strings.TrimSpace(os.Getenv("BASE_URL")); env != "" {
		return strings.TrimRight(env, "/")
	}
	if host := strings.TrimSpace(os.Getenv("PUBLIC_HOST")); host != "" {
		if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
			return strings.TrimRight(host, "/")
		}
		scheme := "http"
		if strings.EqualFold(strings.TrimSpace(os.Getenv("TLS_ENABLED")), "true") {
			scheme = "https"
		}
		return scheme + "://" + strings.TrimRight(host, "/")
	}
	if r == nil {
		return "http://localhost:3001"
	}
	scheme := "http"
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:3001"
	}
	return scheme + "://" + host
}
