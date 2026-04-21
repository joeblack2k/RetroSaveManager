package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (a *app) handleAuthSignup(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Signup successful",
		"user":    defaultUser(),
	})
}

func (a *app) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Logged out",
	})
}

func (a *app) handleAuthMessage(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "ok",
	})
}

func (a *app) handleAuth2FAVerify(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"user":    a.currentRequestPrincipalWithQuota(r),
	})
}

func (a *app) handleAuth2FASetup(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"secret":     "INTERNAL-TOTP-SECRET",
		"otpauthUrl": "otpauth://totp/RetroSaveManager:internal@localhost?secret=INTERNAL-TOTP-SECRET&issuer=RetroSaveManager",
	})
}

func (a *app) handleAuth2FAVerifySetup(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"backupCodes": []string{
			"CODE-1001",
			"CODE-1002",
			"CODE-1003",
			"CODE-1004",
			"CODE-1005",
		},
	})
}

func (a *app) handleAuth2FAStatus(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	a.mu.Lock()
	trustedCount := len(a.trustedDevices)
	a.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":             false,
		"trustedDevicesCount": trustedCount,
		"hasBackupCodes":      true,
	})
}

func (a *app) handleAuth2FADisable(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (a *app) handleAuth2FABackupCodesRegenerate(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"backupCodes": []string{
			"CODE-2001",
			"CODE-2002",
			"CODE-2003",
			"CODE-2004",
			"CODE-2005",
		},
	})
}

func (a *app) handleAuth2FATrustedDevicesList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	a.mu.Lock()
	devices := make([]trustedDevice, 0, len(a.trustedDevices))
	for _, td := range a.trustedDevices {
		devices = append(devices, td)
	}
	a.mu.Unlock()

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].CreatedAt.After(devices[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "devices": devices})
}

func (a *app) handleAuth2FATrustedDevicesDelete(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "id is required", StatusCode: http.StatusBadRequest})
		return
	}

	a.mu.Lock()
	_, ok := a.trustedDevices[id]
	if ok {
		delete(a.trustedDevices, id)
	}
	a.mu.Unlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Trusted device not found", StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (a *app) handleAuthAppPasswordsList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	a.mu.Lock()
	items := make([]appPassword, 0, len(a.appPasswords))
	for _, item := range a.appPasswords {
		items = append(items, item)
	}
	a.mu.Unlock()

	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "appPasswords": items})
}

func (a *app) handleAuthAppPasswordsCreate(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	var payload struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = "app-password"
	}

	now := time.Now().UTC()

	a.mu.Lock()
	id := a.nextAppPasswordID
	a.nextAppPasswordID++
	record := appPassword{
		ID:        "app-password-" + strconv.Itoa(id),
		Name:      name,
		LastFour:  randomHex(2),
		CreatedAt: now,
	}
	a.appPasswords[record.ID] = record
	a.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"appPassword": map[string]any{
			"id":        record.ID,
			"name":      record.Name,
			"lastFour":  record.LastFour,
			"createdAt": record.CreatedAt,
		},
		"plainTextToken": "rsm-" + randomHex(16),
	})
}

func (a *app) handleAuthAppPasswordsDelete(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "id is required", StatusCode: http.StatusBadRequest})
		return
	}

	a.mu.Lock()
	_, ok := a.appPasswords[id]
	if ok {
		delete(a.appPasswords, id)
	}
	a.mu.Unlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "App password not found", StatusCode: http.StatusNotFound})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (a *app) handleReferral(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	baseURL := baseURLForRequest(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"code":    "INTERNAL",
		"url":     baseURL + "/signup?ref=INTERNAL",
		"stats": map[string]any{
			"referrals": 0,
			"credits":   0,
		},
	})
}

func (a *app) handleDevSignup(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Developer signup is disabled in internal mode",
	})
}
