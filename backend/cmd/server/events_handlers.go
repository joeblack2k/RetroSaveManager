package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (a *app) handleEvents(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	subscriberID, events := a.subscribeEvents()
	defer a.unsubscribeEvents(subscriberID)

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			payload, err := json.Marshal(event.Data)
			if err != nil {
				payload = []byte(`{"success":false}`)
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload)
			flusher.Flush()
		case t := <-ticker.C:
			_, _ = fmt.Fprintf(w, ": keepalive %s\n\n", t.UTC().Format(time.RFC3339))
			flusher.Flush()
		}
	}
}
