package main

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (a *app) handleModulesList(w http.ResponseWriter, r *http.Request) {
	service := a.moduleService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Module service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	modules, err := service.listModules()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	library, err := service.libraryStatus()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	writeJSON(w, http.StatusOK, gameModuleListResponse{Success: true, Modules: modules, Library: library})
}

func (a *app) handleModulesSync(w http.ResponseWriter, r *http.Request) {
	service := a.moduleService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Module service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	status, err := service.syncLibrary(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	_ = a.reloadSavesFromDisk()
	writeJSON(w, http.StatusOK, gameModuleSyncResponse{Success: true, Library: status})
}

func (a *app) handleModulesUpload(w http.ResponseWriter, r *http.Request) {
	service := a.moduleService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Module service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxGameModuleZipBytes+1024*1024)
	if err := r.ParseMultipartForm(maxGameModuleZipBytes + 1024); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "multipart module upload is required", StatusCode: http.StatusBadRequest})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "file is required", StatusCode: http.StatusBadRequest})
		return
	}
	defer file.Close()
	filename := safeFilename(header.Filename)
	if !strings.HasSuffix(strings.ToLower(filename), ".rsmodule.zip") {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "module upload must be a .rsmodule.zip file", StatusCode: http.StatusBadRequest})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxGameModuleZipBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: err.Error(), StatusCode: http.StatusBadRequest})
		return
	}
	if len(data) > maxGameModuleZipBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: "Payload Too Large", Message: "module zip is too large", StatusCode: http.StatusRequestEntityTooLarge})
		return
	}
	record, err := service.importZip(r.Context(), data, gameModuleSourceInfo{Source: gameModuleSourceUploaded, SourcePath: filename, SourceSHA256: sha256Hex(data)})
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: err.Error(), StatusCode: http.StatusUnprocessableEntity})
		return
	}
	_ = a.reloadSavesFromDisk()
	writeJSON(w, http.StatusCreated, gameModuleResponse{Success: true, Module: record})
}

func (a *app) handleModuleEnable(w http.ResponseWriter, r *http.Request) {
	a.handleModuleStatus(w, r, gameModuleStatusActive)
}

func (a *app) handleModuleDisable(w http.ResponseWriter, r *http.Request) {
	a.handleModuleStatus(w, r, gameModuleStatusDisabled)
}

func (a *app) handleModuleDelete(w http.ResponseWriter, r *http.Request) {
	a.handleModuleStatus(w, r, gameModuleStatusDeleted)
}

func (a *app) handleModuleStatus(w http.ResponseWriter, r *http.Request, status string) {
	service := a.moduleService()
	if service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "Service Unavailable", Message: "Module service is not initialized", StatusCode: http.StatusServiceUnavailable})
		return
	}
	record, err := service.setStatus(chi.URLParam(r, "id"), status)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: err.Error(), StatusCode: http.StatusNotFound})
		return
	}
	_ = a.reloadSavesFromDisk()
	writeJSON(w, http.StatusOK, gameModuleResponse{Success: true, Module: record})
}

func (a *app) handleModulesRescan(w http.ResponseWriter, r *http.Request) {
	result, err := a.rescanSaves(saveRescanOptions{DryRun: false, PruneUnsupported: false})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: err.Error(), StatusCode: http.StatusInternalServerError})
		return
	}
	writeJSON(w, http.StatusOK, gameModuleRescanResponse{Success: true, Result: result})
}
