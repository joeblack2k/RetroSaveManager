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
