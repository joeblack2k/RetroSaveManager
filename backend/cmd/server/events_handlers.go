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
	defer a.detachEventSubscriber(subscriberID)

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

// detachEventSubscriber removes a disconnected SSE subscriber without closing
// its channel. publishEvent snapshots subscriber channels before sending; a
// concurrent close after that snapshot could otherwise panic with
// "send on closed channel". The request context is the stream's lifecycle
// signal, so closing this internal channel is unnecessary.
func (a *app) detachEventSubscriber(id int) {
	a.mu.Lock()
	delete(a.eventSubscribers, id)
	a.mu.Unlock()
}
