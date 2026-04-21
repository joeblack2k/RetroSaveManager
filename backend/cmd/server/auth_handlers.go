package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (a *app) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	principal := requestPrincipal(r)

	var req loginRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.DeviceType != "" && req.Fingerprint != "" {
		a.upsertDevice(req.DeviceType, req.Fingerprint)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    randomHex(32),
		Path:     "/",
		HttpOnly: true,
		MaxAge:   7 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Login successful",
		"user":    principal,
	})
}

func (a *app) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	writeJSON(w, http.StatusOK, tokenResponse{
		Success:       true,
		Token:         randomHex(64),
		ExpiresInDays: 7,
	})
}

func (a *app) handleAuthTokenAppPassword(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	var payload struct {
		Name       string `json:"name"`
		DeviceType string `json:"device_type"`
		DeviceTypeAlt string `json:"deviceType"`
		Fingerprint string `json:"fingerprint"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	deviceType := firstNonEmpty(payload.DeviceType, payload.DeviceTypeAlt)
	fingerprint := strings.TrimSpace(payload.Fingerprint)
	if deviceType == "" && fingerprint == "" {
		writeJSON(w, http.StatusOK, tokenResponse{
			Success:       true,
			Token:         randomHex(64),
			ExpiresInDays: 7,
		})
		return
	}
	if deviceType == "" || fingerprint == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Error:      "Bad Request",
			Message:    "device_type and fingerprint are required together",
			StatusCode: http.StatusBadRequest,
		})
		return
	}

	now := time.Now().UTC()

	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.autoAppPasswordWindowActiveLocked(now) {
		writeJSON(w, http.StatusForbidden, apiError{
			Error:      "Forbidden",
			Message:    "auto app password enrollment is disabled",
			StatusCode: http.StatusForbidden,
		})
		return
	}

	deviceRecord := a.upsertDeviceLocked(deviceType, fingerprint)
	if existingKeyID, hasExistingKey := a.appPasswordIDForDeviceLocked(deviceRecord.ID); hasExistingKey {
		delete(a.appPasswords, existingKeyID)
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = defaultDeviceDisplayName(deviceType, fingerprint)
	}
	createdRecord, plainTextKey := a.createAppPasswordLocked(name, now)
	a.bindAppPasswordToDeviceLocked(createdRecord.ID, deviceRecord)
	publicRecord := a.publicAppPasswordLocked(a.appPasswords[createdRecord.ID])
	_ = a.persistSecurityDeviceStateLocked()

	writeJSON(w, http.StatusOK, map[string]any{
		"success":        true,
		"token":          plainTextKey,
		"plainTextKey":   plainTextKey,
		"expiresInDays":  7,
		"autoProvisioned": true,
		"appPassword": map[string]any{
			"id":                 publicRecord.ID,
			"name":               publicRecord.Name,
			"lastFour":           publicRecord.LastFour,
			"createdAt":          publicRecord.CreatedAt,
			"lastUsedAt":         publicRecord.LastUsed,
			"boundDeviceId":      publicRecord.BoundDeviceID,
			"syncAll":            publicRecord.SyncAll,
			"allowedSystemSlugs": publicRecord.AllowedSystemSlugs,
		},
	})
}

func (a *app) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Authenticated",
		"user":    a.currentRequestPrincipalWithQuota(r),
	})
}

func (a *app) handleAuthDevice(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	userCode := strings.ToUpper(randomHex(2))
	writeJSON(w, http.StatusOK, map[string]any{
		"deviceCode":       randomHex(32),
		"userCode":         userCode,
		"verificationUri":  baseURLForRequest(r) + "/device/" + userCode,
		"expiresInSeconds": 900,
	})
}

func (a *app) handleAuthDeviceToken(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload["deviceCode"] == nil {
		writeText(w, http.StatusUnprocessableEntity, "Failed to deserialize the JSON body into the target type: missing field `deviceCode`")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"success": false,
		"status":  "pending",
	})
}

func (a *app) handleAuthDeviceVerify(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{"expiresAt": time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339Nano)})
}

func (a *app) handleAuthDeviceConfirm(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
