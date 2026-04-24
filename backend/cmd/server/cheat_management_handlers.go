package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (a *app) handleCheatPacksList(w http.ResponseWriter, r *http.Request) {
	principal := requestPrincipal(r)
	_ = principal
	service := a.cheatService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Cheat service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	packs, err := service.listManagedPacks()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	writeJSON(w, http.StatusOK, cheatManagedPackListResponse{Success: true, Packs: packs})
}

func (a *app) handleCheatPackGet(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	service := a.cheatService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Cheat service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	item, err := service.getManagedPack(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: err.Error(), StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, cheatManagedPackResponse{Success: true, Pack: item})
}

func (a *app) handleCheatPackCreate(w http.ResponseWriter, r *http.Request) {
	principal := requestPrincipal(r)
	service := a.cheatService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Cheat service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	var req cheatPackCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "invalid JSON body", StatusCode: http.StatusBadRequest})
		return
	}
	req.YAML = strings.TrimSpace(req.YAML)
	if req.YAML == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "yaml is required", StatusCode: http.StatusBadRequest})
		return
	}
	item, err := service.createManagedPack(req, principal)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: err.Error(), StatusCode: http.StatusUnprocessableEntity})
		return
	}
	writeJSON(w, http.StatusCreated, cheatManagedPackResponse{Success: true, Pack: item})
}

func (a *app) handleCheatPackDelete(w http.ResponseWriter, r *http.Request) {
	principal := requestPrincipal(r)
	service := a.cheatService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Cheat service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	item, err := service.deleteManagedPack(chi.URLParam(r, "id"), principal)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: err.Error(), StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, cheatManagedPackResponse{Success: true, Pack: item})
}

func (a *app) handleCheatPackDisable(w http.ResponseWriter, r *http.Request) {
	principal := requestPrincipal(r)
	service := a.cheatService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Cheat service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	item, err := service.disableManagedPack(chi.URLParam(r, "id"), principal)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: err.Error(), StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, cheatManagedPackResponse{Success: true, Pack: item})
}

func (a *app) handleCheatPackEnable(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	service := a.cheatService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Cheat service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	item, err := service.enableManagedPack(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: err.Error(), StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, cheatManagedPackResponse{Success: true, Pack: item})
}

func (a *app) handleCheatAdaptersList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	service := a.cheatService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Cheat service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	writeJSON(w, http.StatusOK, cheatAdapterListResponse{Success: true, Adapters: service.listAdapterDescriptors()})
}

func (a *app) handleCheatAdapterGet(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	service := a.cheatService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Cheat service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	item, err := service.adapterDescriptor(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: err.Error(), StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, cheatAdapterResponse{Success: true, Adapter: item})
}
