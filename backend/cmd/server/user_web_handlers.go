package main

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (a *app) handleParserWASM(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	// Small valid WASM preamble to keep parser bootstrap flows from failing.
	wasm := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	w.Header().Set("Content-Type", "application/wasm")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(wasm)
}

func (a *app) handleCatalogList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	a.mu.Lock()
	items := make([]catalogGame, 0, len(a.catalog))
	for _, item := range a.catalog {
		items = append(items, item)
	}
	a.mu.Unlock()

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"items":   items,
		"total":   len(items),
	})
}

func (a *app) handleCatalogGet(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "id is required", StatusCode: http.StatusBadRequest})
		return
	}

	a.mu.Lock()
	item, ok := a.catalog[id]
	a.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Catalog item not found", StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "item": item})
}

func (a *app) handleCatalogDownload(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "id is required", StatusCode: http.StatusBadRequest})
		return
	}

	a.mu.Lock()
	item, ok := a.catalog[id]
	a.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Catalog item not found", StatusCode: http.StatusNotFound})
		return
	}

	payload := []byte("RetroSaveManager catalog placeholder for " + item.Name + "\n")
	filename := safeFilename(item.Name) + ".txt"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = w.Write(payload)
}

func (a *app) handleGamesLibraryList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	a.mu.Lock()
	items := make([]libraryGame, 0, len(a.library))
	for _, item := range a.library {
		items = append(items, item)
	}
	a.mu.Unlock()

	sort.Slice(items, func(i, j int) bool { return items[i].AddedAt.After(items[j].AddedAt) })
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"games":   items,
		"total":   len(items),
	})
}

func (a *app) handleGamesLibraryCreate(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	var payload struct {
		CatalogID string `json:"catalogId"`
	}
	_ = decodeJSONBody(r, &payload)

	catalogID := strings.TrimSpace(payload.CatalogID)

	a.mu.Lock()
	defer a.mu.Unlock()

	var selected catalogGame
	if catalogID != "" {
		item, ok := a.catalog[catalogID]
		if !ok {
			writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Catalog item not found", StatusCode: http.StatusNotFound})
			return
		}
		selected = item
	} else {
		for _, item := range a.catalog {
			selected = item
			break
		}
	}

	id := "lib-" + strconv.Itoa(a.nextLibraryID)
	a.nextLibraryID++
	entry := libraryGame{
		ID:      id,
		Catalog: selected,
		AddedAt: time.Now().UTC(),
	}
	a.library[id] = entry
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "entry": entry})
}

func (a *app) handleGamesLibraryDelete(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "id is required", StatusCode: http.StatusBadRequest})
		return
	}

	a.mu.Lock()
	_, ok := a.library[id]
	if ok {
		delete(a.library, id)
	}
	a.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Library item not found", StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (a *app) handleRoadmapItemsList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	a.mu.Lock()
	items := make([]roadmapItem, 0, len(a.roadmapItems))
	for _, item := range a.roadmapItems {
		items = append(items, item)
	}
	a.mu.Unlock()

	sort.Slice(items, func(i, j int) bool {
		if items[i].Votes == items[j].Votes {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].Votes > items[j].Votes
	})
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "items": items})
}

func (a *app) handleRoadmapItemsVote(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "id is required", StatusCode: http.StatusBadRequest})
		return
	}

	a.mu.Lock()
	item, ok := a.roadmapItems[id]
	if ok {
		item.Votes++
		a.roadmapItems[id] = item
	}
	a.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Roadmap item not found", StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "item": item})
}

func (a *app) handleRoadmapSuggestionsCreate(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	var payload struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	_ = decodeJSONBody(r, &payload)

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "title is required", StatusCode: http.StatusBadRequest})
		return
	}

	a.mu.Lock()
	id := "suggestion-" + strconv.Itoa(a.nextSuggestionID)
	a.nextSuggestionID++
	item := roadmapSuggestion{
		ID:          id,
		Title:       title,
		Description: strings.TrimSpace(payload.Description),
		UserID:      internalPrincipalID(),
		Status:      "open",
		CreatedAt:   time.Now().UTC(),
	}
	a.roadmapSuggestions[id] = item
	a.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "suggestion": item})
}

func (a *app) handleRoadmapSuggestionsMine(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	a.mu.Lock()
	items := make([]roadmapSuggestion, 0, len(a.roadmapSuggestions))
	for _, item := range a.roadmapSuggestions {
		if item.UserID == internalPrincipalID() {
			items = append(items, item)
		}
	}
	a.mu.Unlock()

	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "items": items})
}
