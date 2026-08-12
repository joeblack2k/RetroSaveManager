package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const conditionalWritesHeader = "X-RSM-Conditional-Writes"

type saveWritePreconditionResult struct {
	Allowed bool
	Status  int
	Reason  string
	Latest  *saveRecord
}

func saveRecordRevision(record saveRecord) string {
	return strings.TrimSpace(record.Summary.ID)
}

func saveRecordETag(record saveRecord) string {
	revision := saveRecordRevision(record)
	if revision == "" {
		return ""
	}
	return `"` + revision + `"`
}

func normalizeRevisionToken(raw string) string {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "W/") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "W/"))
	}
	value = strings.Trim(value, `"`)
	return strings.TrimSpace(value)
}

func requestedBaseRevision(r *http.Request, formValue func(string) string) (string, error) {
	fromForm := ""
	if formValue != nil {
		fromForm = normalizeRevisionToken(firstNonEmpty(formValue("baseRevision"), formValue("base_revision")))
	}
	fromHeader := ""
	if r != nil {
		fromHeader = normalizeRevisionToken(r.Header.Get("If-Match"))
	}
	if fromForm != "" && fromHeader != "" && fromForm != fromHeader {
		return "", fmt.Errorf("baseRevision and If-Match must identify the same revision")
	}
	return firstNonEmpty(fromHeader, fromForm), nil
}

func evaluateSaveWritePrecondition(latest *saveRecord, baseRevision string, createOnly, strict bool) saveWritePreconditionResult {
	baseRevision = normalizeRevisionToken(baseRevision)
	if latest == nil {
		if baseRevision != "" {
			return saveWritePreconditionResult{Status: http.StatusPreconditionFailed, Reason: "base revision does not exist"}
		}
		return saveWritePreconditionResult{Allowed: true}
	}

	latestCopy := *latest
	if createOnly {
		return saveWritePreconditionResult{Status: http.StatusPreconditionFailed, Reason: "logical save already exists", Latest: &latestCopy}
	}
	if baseRevision == "" {
		if strict {
			return saveWritePreconditionResult{Status: http.StatusPreconditionRequired, Reason: "baseRevision or If-Match is required for this helper", Latest: &latestCopy}
		}
		return saveWritePreconditionResult{Allowed: true, Latest: &latestCopy}
	}
	if baseRevision != saveRecordRevision(latestCopy) {
		return saveWritePreconditionResult{Status: http.StatusPreconditionFailed, Reason: "cloud revision changed since the client read it", Latest: &latestCopy}
	}
	return saveWritePreconditionResult{Allowed: true, Latest: &latestCopy}
}

func helperSupportsConditionalWrites(ctx helperAuthContext) bool {
	if !ctx.IsHelper {
		return false
	}
	return capabilityBool(ctx.Device.ConfigCapabilities, "conditionalWrites", "conditional_writes", "ifMatch", "if_match")
}

func capabilityBool(raw map[string]any, keys ...string) bool {
	if len(raw) == 0 {
		return false
	}
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			if enabled, ok := value.(bool); ok && enabled {
				return true
			}
		}
	}
	for _, nestedKey := range []string{"sync", "saves", "protocol", "features"} {
		if nested, ok := raw[nestedKey].(map[string]any); ok && capabilityBool(nested, keys...) {
			return true
		}
	}
	return false
}

func multipartRuntimeProfile(formValue func(string) string) string {
	if formValue == nil {
		return ""
	}
	return firstNonEmpty(
		formValue("runtimeProfile"),
		formValue("runtime_profile"),
		formValue("n64Profile"),
		formValue("n64_profile"),
	)
}

func (a *app) latestRecordForWritePrecondition(helperCtx helperAuthContext, formValue func(string) string) (saveRecord, bool, error) {
	romSHA1 := ""
	slotName := "default"
	if formValue != nil {
		romSHA1 = strings.TrimSpace(firstNonEmpty(formValue("rom_sha1"), formValue("romSha1")))
		slotName = normalizedSlot(formValue("slotName"))
	}
	runtimeProfile := multipartRuntimeProfile(formValue)

	if helperCtx.IsHelper && runtimeProfile != "" && (strings.HasPrefix(runtimeProfile, "psx/") || strings.HasPrefix(runtimeProfile, "ps2/")) {
		cardSlot, ok := deriveExplicitMemoryCardName(slotName, slotName)
		if !ok {
			return saveRecord{}, false, fmt.Errorf("slotName must be an explicit Memory Card 1/2 for PlayStation conditional writes")
		}
		if a.playStationSyncStore() != nil {
			return a.latestPlayStationProjectionRecord(runtimeProfile, cardSlot, saveCreateInput{CreatedAt: time.Now().UTC()})
		}
	}
	if helperCtx.IsHelper && runtimeProfile != "" && strings.HasPrefix(runtimeProfile, "n64/") {
		if store := a.n64ControllerPakStore(); store != nil {
			if saveID, exists := store.latestProjectionSaveRecord(n64ControllerPakSyncLineKey(romSHA1, slotName), runtimeProfile); exists {
				record, found := a.findSaveRecordByID(saveID)
				if found && saveRecordPayloadExists(record) {
					return record, true, nil
				}
			}
		}
	}
	if record, found := a.latestReadableSaveRecord(romSHA1, slotName); found {
		return record, true, nil
	}
	return saveRecord{}, false, nil
}

func (a *app) enforceSaveWritePrecondition(w http.ResponseWriter, r *http.Request, helperCtx helperAuthContext, formValue func(string) string, filename string) bool {
	w.Header().Set(conditionalWritesHeader, "supported")

	baseRevision, err := requestedBaseRevision(r, formValue)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: err.Error(), StatusCode: http.StatusBadRequest})
		return false
	}
	createOnly := r != nil && strings.TrimSpace(r.Header.Get("If-None-Match")) == "*"
	strict := helperSupportsConditionalWrites(helperCtx)

	if strings.EqualFold(filepath.Ext(strings.TrimSpace(filename)), ".zip") && strict && baseRevision == "" && !createOnly {
		writeJSON(w, http.StatusPreconditionRequired, apiError{Error: "Precondition Required", Message: "conditional archive uploads require an explicit create-only or base revision contract", StatusCode: http.StatusPreconditionRequired})
		return false
	}

	latest, found, err := a.latestRecordForWritePrecondition(helperCtx, formValue)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: err.Error(), StatusCode: http.StatusBadRequest})
		return false
	}
	var latestPtr *saveRecord
	if found {
		latestCopy := latest
		latestPtr = &latestCopy
	}
	result := evaluateSaveWritePrecondition(latestPtr, baseRevision, createOnly, strict)
	if result.Allowed {
		return true
	}
	if result.Latest != nil {
		w.Header().Set("ETag", saveRecordETag(*result.Latest))
	}
	body := map[string]any{
		"success": false,
		"reason":  result.Reason,
	}
	if result.Latest != nil {
		body["latest"] = map[string]any{
			"id":       result.Latest.Summary.ID,
			"revision": saveRecordRevision(*result.Latest),
			"sha256":   result.Latest.Summary.SHA256,
			"version":  result.Latest.Summary.Version,
			"format":   saveRecordFormat(*result.Latest),
		}
	}
	writeJSON(w, result.Status, body)
	return false
}

func writeLatestSaveRecord(w http.ResponseWriter, latest saveRecord, shaValue string) {
	w.Header().Set(conditionalWritesHeader, "supported")
	w.Header().Set("ETag", saveRecordETag(latest))
	writeJSON(w, http.StatusOK, map[string]any{
		"success":           true,
		"exists":            true,
		"sha256":            shaValue,
		"version":           latest.Summary.Version,
		"id":                latest.Summary.ID,
		"revision":          saveRecordRevision(latest),
		"format":            saveRecordFormat(latest),
		"conditionalWrites": true,
	})
}
