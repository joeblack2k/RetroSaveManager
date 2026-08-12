package main

import (
	"net/http"
	"testing"
)

func TestNewHTTPServerUsesSafeConnectionDefaults(t *testing.T) {
	t.Setenv("PORT", "8080")
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	server := newHTTPServer(handler)

	if server.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("Handler must be configured")
	}
	if server.ReadHeaderTimeout != defaultReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %s, want %s", server.ReadHeaderTimeout, defaultReadHeaderTimeout)
	}
	if server.IdleTimeout != defaultIdleTimeout {
		t.Fatalf("IdleTimeout = %s, want %s", server.IdleTimeout, defaultIdleTimeout)
	}
	if server.MaxHeaderBytes != defaultMaxHeaderBytes {
		t.Fatalf("MaxHeaderBytes = %d, want %d", server.MaxHeaderBytes, defaultMaxHeaderBytes)
	}
	if server.ReadTimeout != 0 {
		t.Fatalf("ReadTimeout = %s, want zero so slow legitimate save uploads are not cut off", server.ReadTimeout)
	}
	if server.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %s, want zero so large legitimate downloads are not cut off", server.WriteTimeout)
	}
}

func TestNewHTTPServerDefaultsToPort80(t *testing.T) {
	t.Setenv("PORT", "")

	server := newHTTPServer(http.NotFoundHandler())

	if server.Addr != ":80" {
		t.Fatalf("Addr = %q, want :80", server.Addr)
	}
}
