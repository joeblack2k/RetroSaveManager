package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (a *app) handleDevicesList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	a.mu.Lock()
	defer a.mu.Unlock()

	devices := make([]device, 0, len(a.devices))
	for _, d := range a.devices {
		devices = append(devices, d)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].ID > devices[j].ID })

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "devices": devices})
}

func (a *app) handleDevicesPatch(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	id := parseIntOrDefault(chi.URLParam(r, "id"), 0)
	var req devicePatchRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	a.mu.Lock()
	defer a.mu.Unlock()

	d, ok := a.devices[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Device not found", StatusCode: http.StatusNotFound})
		return
	}

	d.Alias = req.Alias
	if req.Alias != nil && strings.TrimSpace(*req.Alias) != "" {
		d.DisplayName = *req.Alias
	}
	a.devices[id] = d

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "device": d})
}

func (a *app) handleDevicesDelete(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	id := parseIntOrDefault(chi.URLParam(r, "id"), 0)
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.devices[id]; !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Device not found", StatusCode: http.StatusNotFound})
		return
	}
	delete(a.devices, id)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
