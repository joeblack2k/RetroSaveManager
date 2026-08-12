package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultReadHeaderTimeout = 10 * time.Second
	defaultIdleTimeout       = 2 * time.Minute
	defaultMaxHeaderBytes    = 1 << 20
)

func newHTTPServer(handler http.Handler) *http.Server {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "80"
	}

	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		IdleTimeout:       defaultIdleTimeout,
		MaxHeaderBytes:    defaultMaxHeaderBytes,
	}
}

func serve(handler http.Handler) {
	server := newHTTPServer(handler)
	log.Printf("rsm selfhost service listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}
