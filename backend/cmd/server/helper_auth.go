package main

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

type helperIdentity struct {
	DeviceType  string
	Fingerprint string
}

type helperMetadata struct {
	Hostname            string
	HelperName          string
	HelperVersion       string
	Platform            string
	SyncPaths           []string
	ReportedSystemSlugs []string
	LastSeenIP          string
	LastSeenUserAgent   string
}

type helperAuthContext struct {
	IsHelper        bool
	Device          device
	AppPassword     appPassword
	GeneratedAppKey string
}

func (h helperIdentity) hasAnyMarker() bool {
	return strings.TrimSpace(h.DeviceType) != "" || strings.TrimSpace(h.Fingerprint) != ""
}

func (h helperIdentity) isComplete() bool {
	return strings.TrimSpace(h.DeviceType) != "" && strings.TrimSpace(h.Fingerprint) != ""
}

func extractHelperIdentity(r *http.Request, formValue func(string) string) helperIdentity {
	deviceType := firstNonEmpty(
		helperFieldValue(r, formValue, []string{"device_type", "deviceType"}, []string{"X-RSM-Device-Type"}),
	)
	fingerprint := firstNonEmpty(
		helperFieldValue(r, formValue, []string{"fingerprint"}, []string{"X-RSM-Fingerprint"}),
	)
	return helperIdentity{DeviceType: deviceType, Fingerprint: fingerprint}
}

func extractHelperMetadata(r *http.Request, formValue func(string) string) helperMetadata {
	return helperMetadata{
		Hostname: helperFieldValue(r, formValue,
			[]string{"hostname", "host_name", "helper_hostname", "helperHostname"},
			[]string{"X-RSM-Hostname"},
		),
		HelperName: helperFieldValue(r, formValue,
			[]string{"helper_name", "helperName", "client_name", "clientName"},
			[]string{"X-RSM-Helper-Name"},
		),
		HelperVersion: helperFieldValue(r, formValue,
			[]string{"helper_version", "helperVersion", "client_version", "clientVersion"},
			[]string{"X-RSM-Helper-Version"},
		),
		Platform: helperFieldValue(r, formValue,
			[]string{"platform", "os", "os_name", "osName"},
			[]string{"X-RSM-Platform"},
		),
		SyncPaths: normalizeHelperPaths(helperListFieldValues(r, formValue,
			[]string{"sync_paths", "syncPaths", "sync_path", "syncPath", "save_root", "saveRoot", "save_roots", "saveRoots"},
			[]string{"X-RSM-Sync-Paths"},
		)),
		ReportedSystemSlugs: normalizeAllowedSystemSlugs(helperListFieldValues(r, formValue,
			[]string{"systems", "system_slugs", "systemSlugs", "supported_systems", "supportedSystems"},
			[]string{"X-RSM-Systems"},
		)),
		LastSeenIP:        requestClientIP(r),
		LastSeenUserAgent: strings.TrimSpace(r.Header.Get("User-Agent")),
	}
}

func helperFieldValue(r *http.Request, formValue func(string) string, formKeys []string, headerKeys []string) string {
	for _, key := range formKeys {
		if formValue != nil {
			if value := strings.TrimSpace(formValue(key)); value != "" {
				return value
			}
		}
		if r != nil {
			for _, value := range r.Form[key] {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					return trimmed
				}
			}
			for _, value := range r.URL.Query()[key] {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	if r != nil {
		for _, key := range headerKeys {
			for _, value := range r.Header.Values(key) {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func helperListFieldValues(r *http.Request, formValue func(string) string, formKeys []string, headerKeys []string) []string {
	out := make([]string, 0, 8)
	for _, key := range formKeys {
		if formValue != nil {
			out = append(out, splitHelperListField(formValue(key))...)
		}
		if r != nil {
			for _, value := range r.Form[key] {
				out = append(out, splitHelperListField(value)...)
			}
			for _, value := range r.URL.Query()[key] {
				out = append(out, splitHelperListField(value)...)
			}
		}
	}
	if r != nil {
		for _, key := range headerKeys {
			for _, value := range r.Header.Values(key) {
				out = append(out, splitHelperListField(value)...)
			}
		}
	}
	return out
}

func splitHelperListField(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		var values []string
		if err := json.Unmarshal([]byte(trimmed), &values); err == nil {
			out := make([]string, 0, len(values))
			for _, value := range values {
				if value = strings.TrimSpace(value); value != "" {
					out = append(out, value)
				}
			}
			return out
		}
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '|'
	})
	if len(parts) == 0 {
		return []string{trimmed}
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func requestClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			value = strings.TrimSpace(strings.Split(value, ",")[0])
		}
		if value != "" {
			return value
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func extractHelperAppPassword(r *http.Request, formValue func(string) string) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		token, hasBearer := parseBearerToken(authHeader)
		if hasBearer {
			if _, _, ok := normalizeAppPasswordInput(token); ok {
				return strings.TrimSpace(token)
			}
		}
	}

	xHeader := strings.TrimSpace(r.Header.Get("X-RSM-App-Password"))
	if xHeader != "" {
		return xHeader
	}

	if formValue != nil {
		if field := strings.TrimSpace(formValue("app_password")); field != "" {
			return field
		}
	}

	return ""
}

func parseBearerToken(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	parts := strings.Fields(raw)
	if len(parts) < 2 {
		return "", false
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return strings.TrimSpace(strings.Join(parts[1:], " ")), true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (a *app) authorizeHelperSyncRequest(w http.ResponseWriter, r *http.Request, formValue func(string) string) (helperAuthContext, bool) {
	identity := extractHelperIdentity(r, formValue)
	metadata := extractHelperMetadata(r, formValue)
	rawKey := extractHelperAppPassword(r, formValue)
	isHelperRequest := strings.TrimSpace(rawKey) != "" || identity.hasAnyMarker()
	if !isHelperRequest {
		return helperAuthContext{IsHelper: false}, true
	}

	if strings.TrimSpace(rawKey) == "" {
		ctx, status, msg := a.authenticateHelperWithoutKey(identity, metadata)
		if status != 0 {
			errorLabel := "Unauthorized"
			if status == http.StatusBadRequest {
				errorLabel = "Bad Request"
			} else if status == http.StatusForbidden {
				errorLabel = "Forbidden"
			}
			writeJSON(w, status, apiError{Error: errorLabel, Message: msg, StatusCode: status})
			return helperAuthContext{}, false
		}
		if strings.TrimSpace(ctx.GeneratedAppKey) != "" {
			w.Header().Set("X-RSM-Auto-App-Password", ctx.GeneratedAppKey)
		}
		ctx.IsHelper = true
		return ctx, true
	}

	_, compact, ok := normalizeAppPasswordInput(rawKey)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Error: "Unauthorized", Message: "invalid app password format", StatusCode: http.StatusUnauthorized})
		return helperAuthContext{}, false
	}

	ctx, status, msg := a.authenticateHelperKey(compact, identity, metadata)
	if status != 0 {
		errorLabel := "Forbidden"
		if status == http.StatusBadRequest {
			errorLabel = "Bad Request"
		} else if status == http.StatusUnauthorized {
			errorLabel = "Unauthorized"
		} else if status == http.StatusConflict {
			errorLabel = "Conflict"
		}
		writeJSON(w, status, apiError{Error: errorLabel, Message: msg, StatusCode: status})
		return helperAuthContext{}, false
	}
	ctx.IsHelper = true
	return ctx, true
}

func (a *app) authenticateHelperWithoutKey(identity helperIdentity, metadata helperMetadata) (helperAuthContext, int, string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !identity.isComplete() {
		return helperAuthContext{}, http.StatusBadRequest, "device_type and fingerprint are required"
	}
	if !a.autoAppPasswordWindowActiveLocked(time.Now().UTC()) {
		return helperAuthContext{}, http.StatusUnauthorized, "app password is required for helper sync requests"
	}

	boundDevice, foundDevice := a.findDeviceByIdentityLocked(identity.DeviceType, identity.Fingerprint)
	if !foundDevice {
		boundDevice = a.upsertDeviceLocked(identity.DeviceType, identity.Fingerprint)
	}
	now := time.Now().UTC()
	boundDevice = applyHelperMetadataToDevice(boundDevice, metadata, now)

	if existingKeyID, hasExistingKey := a.appPasswordIDForDeviceLocked(boundDevice.ID); hasExistingKey {
		record := a.appPasswords[existingKeyID]
		record.LastUsed = &now
		a.appPasswords[existingKeyID] = record
		boundDevice.LastSyncedAt = now
		a.saveDeviceLocked(boundDevice)
		_ = a.persistSecurityDeviceStateLocked()
		publicDevice := a.publicDeviceLocked(a.devices[boundDevice.ID])
		publicPassword := a.publicAppPasswordLocked(a.appPasswords[existingKeyID])
		return helperAuthContext{IsHelper: true, Device: publicDevice, AppPassword: publicPassword}, 0, ""
	}

	name := defaultDeviceDisplayName(identity.DeviceType, identity.Fingerprint)
	record, plainTextKey := a.createAppPasswordLocked(name, now)
	a.bindAppPasswordToDeviceLocked(record.ID, boundDevice)
	_ = a.persistSecurityDeviceStateLocked()

	publicDevice := a.publicDeviceLocked(a.devices[boundDevice.ID])
	publicPassword := a.publicAppPasswordLocked(a.appPasswords[record.ID])
	return helperAuthContext{
		IsHelper:        true,
		Device:          publicDevice,
		AppPassword:     publicPassword,
		GeneratedAppKey: plainTextKey,
	}, 0, ""
}

func (a *app) authenticateHelperKey(compactKey string, identity helperIdentity, metadata helperMetadata) (helperAuthContext, int, string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if identity.hasAnyMarker() && !identity.isComplete() {
		return helperAuthContext{}, http.StatusBadRequest, "device_type and fingerprint are required together"
	}

	record, ok := a.findAppPasswordByCompactLocked(compactKey)
	if !ok {
		return helperAuthContext{}, http.StatusForbidden, "invalid app password"
	}

	keyID := record.ID
	if strings.TrimSpace(keyID) == "" {
		return helperAuthContext{}, http.StatusForbidden, "invalid app password"
	}

	hasBoundIdentity := strings.TrimSpace(record.BoundDeviceType) != "" && strings.TrimSpace(record.BoundFingerprint) != ""
	if hasBoundIdentity {
		if identity.isComplete() && !deviceIdentityMatches(identity.DeviceType, identity.Fingerprint, record.BoundDeviceType, record.BoundFingerprint) {
			return helperAuthContext{}, http.StatusConflict, "app password is already bound to another device"
		}
		if !identity.isComplete() {
			identity = helperIdentity{
				DeviceType:  record.BoundDeviceType,
				Fingerprint: record.BoundFingerprint,
			}
		}
	} else if !identity.isComplete() {
		return helperAuthContext{}, http.StatusBadRequest, "device_type and fingerprint are required on first key use"
	}

	boundDevice, foundDevice := a.findDeviceByIdentityLocked(identity.DeviceType, identity.Fingerprint)
	if !foundDevice {
		boundDevice = a.upsertDeviceLocked(identity.DeviceType, identity.Fingerprint)
	}

	if otherKeyID, hasOtherKey := a.appPasswordIDForDeviceLocked(boundDevice.ID); hasOtherKey && otherKeyID != keyID {
		return helperAuthContext{}, http.StatusConflict, "device already has a different app password bound"
	}

	if record.BoundDeviceID != nil && *record.BoundDeviceID != boundDevice.ID {
		if _, exists := a.devices[*record.BoundDeviceID]; exists {
			return helperAuthContext{}, http.StatusConflict, "app password is already bound to another device"
		}
	}

	now := time.Now().UTC()
	boundDevice = applyHelperMetadataToDevice(boundDevice, metadata, now)
	record.BoundDeviceType = strings.TrimSpace(identity.DeviceType)
	record.BoundFingerprint = strings.TrimSpace(identity.Fingerprint)
	record.BoundDeviceID = &boundDevice.ID
	record.LastUsed = &now
	a.appPasswords[keyID] = record

	a.bindAppPasswordToDeviceLocked(keyID, boundDevice)

	_ = a.persistSecurityDeviceStateLocked()

	publicDevice := a.publicDeviceLocked(a.devices[boundDevice.ID])
	publicPassword := a.publicAppPasswordLocked(a.appPasswords[keyID])
	return helperAuthContext{IsHelper: true, Device: publicDevice, AppPassword: publicPassword}, 0, ""
}

func saveRecordSystemSlug(record saveRecord) string {
	systemSlug := canonicalSegment(record.SystemSlug, "unknown-system")
	if systemSlug != "unknown-system" {
		return systemSlug
	}
	if record.Summary.Game.System != nil {
		systemSlug = canonicalSegment(record.Summary.Game.System.Slug, "unknown-system")
		if systemSlug != "unknown-system" {
			return systemSlug
		}
		systemSlug = canonicalSegment(record.Summary.Game.System.Name, "unknown-system")
		if systemSlug != "unknown-system" {
			return systemSlug
		}
	}
	return "unknown-system"
}
