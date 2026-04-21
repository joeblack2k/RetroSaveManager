package main

import (
	"log"
	"net/http"
	"os"
)

func serve(handler http.Handler) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	addr := ":" + port
	log.Printf("1retro selfhost stub listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
