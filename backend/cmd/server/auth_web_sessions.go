package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	webSessionStateFileName = "web_sessions.json"
	webSessionLifetime      = 7 * 24 * time.Hour
	webSessionMaxEntries    = 64
	minimumAdminPasswordLen = 12
)

type webSessionRecord struct {
	TokenHash string    `json:"tokenHash"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type webSessionStateFile struct {
	Sessions  []webSessionRecord `json:"sessions"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

var webSessionStateMu sync.Mutex

func configuredAdminCredentials() (email string, password string, ok bool) {
	email = strings.TrimSpace(strings.ToLower(os.Getenv("RSM_ADMIN_EMAIL")))
	password = os.Getenv("RSM_ADMIN_PASSWORD")
	return email, password, email != "" && password != ""
}

func validateAuthConfiguration() error {
	if authMode() != "enabled" {
		return nil
	}

	_, password, hasBuiltInAdmin := configuredAdminCredentials()
	if hasBuiltInAdmin {
		if len(password) < minimumAdminPasswordLen {
			return fmt.Errorf("RSM_ADMIN_PASSWORD must contain at least %d characters when AUTH_MODE=enabled", minimumAdminPasswordLen)
		}
		return nil
	}
	if trustRemoteUserHeader() {
		return nil
	}
	return errors.New("AUTH_MODE=enabled requires RSM_ADMIN_EMAIL and RSM_ADMIN_PASSWORD, or TRUST_REMOTE_USER_HEADER=true behind a trusted authenticating reverse proxy")
}

func authenticateConfiguredAdmin(email, password string) bool {
	configuredEmail, configuredPassword, ok := configuredAdminCredentials()
	if !ok {
		return false
	}

	providedEmailHash := sha256.Sum256([]byte(strings.TrimSpace(strings.ToLower(email))))
	configuredEmailHash := sha256.Sum256([]byte(configuredEmail))
	providedPasswordHash := sha256.Sum256([]byte(password))
	configuredPasswordHash := sha256.Sum256([]byte(configuredPassword))

	emailOK := subtle.ConstantTimeCompare(providedEmailHash[:], configuredEmailHash[:]) == 1
	passwordOK := subtle.ConstantTimeCompare(providedPasswordHash[:], configuredPasswordHash[:]) == 1
	return emailOK && passwordOK
}

func webSessionStateFilePathFromEnv() string {
	return filepath.Join(stateRootDirFromEnv(), webSessionStateFileName)
}

func secureRandomHex(byteCount int) (string, error) {
	if byteCount <= 0 {
		return "", errors.New("random byte count must be positive")
	}
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read cryptographic randomness: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func webSessionTokenHash(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func loadWebSessionStateLocked(now time.Time) (webSessionStateFile, bool, error) {
	path := webSessionStateFilePathFromEnv()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return webSessionStateFile{Sessions: []webSessionRecord{}}, false, nil
		}
		return webSessionStateFile{}, false, fmt.Errorf("read web session state: %w", err)
	}
	if len(data) == 0 {
		return webSessionStateFile{}, false, errors.New("web session state is empty")
	}

	var state webSessionStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return webSessionStateFile{}, false, fmt.Errorf("decode web session state: %w", err)
	}

	clean := make([]webSessionRecord, 0, len(state.Sessions))
	changed := false
	for _, session := range state.Sessions {
		if strings.TrimSpace(session.TokenHash) == "" || session.ExpiresAt.IsZero() || !now.UTC().Before(session.ExpiresAt.UTC()) {
			changed = true
			continue
		}
		clean = append(clean, session)
	}
	state.Sessions = clean
	return state, changed, nil
}

func persistWebSessionStateLocked(state webSessionStateFile, now time.Time) error {
	state.UpdatedAt = now.UTC()
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode web session state: %w", err)
	}
	if err := writeFileAtomic(webSessionStateFilePathFromEnv(), payload, 0o600); err != nil {
		return fmt.Errorf("write web session state: %w", err)
	}
	return nil
}

func createWebSession(email string, now time.Time) (string, time.Time, error) {
	rawToken, err := secureRandomHex(32)
	if err != nil {
		return "", time.Time{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	expiresAt := now.Add(webSessionLifetime)
	record := webSessionRecord{
		TokenHash: webSessionTokenHash(rawToken),
		Email:     strings.TrimSpace(strings.ToLower(email)),
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	webSessionStateMu.Lock()
	defer webSessionStateMu.Unlock()

	state, _, err := loadWebSessionStateLocked(now)
	if err != nil {
		return "", time.Time{}, err
	}
	state.Sessions = append(state.Sessions, record)
	sort.Slice(state.Sessions, func(i, j int) bool {
		return state.Sessions[i].CreatedAt.After(state.Sessions[j].CreatedAt)
	})
	if len(state.Sessions) > webSessionMaxEntries {
		state.Sessions = state.Sessions[:webSessionMaxEntries]
	}
	if err := persistWebSessionStateLocked(state, now); err != nil {
		return "", time.Time{}, err
	}
	return rawToken, expiresAt, nil
}

func validateWebSession(rawToken string, now time.Time) bool {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	targetHash := webSessionTokenHash(rawToken)

	webSessionStateMu.Lock()
	defer webSessionStateMu.Unlock()

	state, changed, err := loadWebSessionStateLocked(now)
	if err != nil {
		return false
	}
	valid := false
	for _, session := range state.Sessions {
		if subtle.ConstantTimeCompare([]byte(session.TokenHash), []byte(targetHash)) == 1 {
			valid = true
			break
		}
	}
	if changed {
		_ = persistWebSessionStateLocked(state, now)
	}
	return valid
}

func revokeWebSession(rawToken string, now time.Time) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	targetHash := webSessionTokenHash(rawToken)

	webSessionStateMu.Lock()
	defer webSessionStateMu.Unlock()

	state, changed, err := loadWebSessionStateLocked(now)
	if err != nil {
		return err
	}
	kept := state.Sessions[:0]
	for _, session := range state.Sessions {
		if subtle.ConstantTimeCompare([]byte(session.TokenHash), []byte(targetHash)) == 1 {
			changed = true
			continue
		}
		kept = append(kept, session)
	}
	state.Sessions = kept
	if !changed {
		return nil
	}
	return persistWebSessionStateLocked(state, now)
}

func sessionCookieSecure(r *http.Request) bool {
	if raw := strings.TrimSpace(strings.ToLower(os.Getenv("AUTH_COOKIE_SECURE"))); raw != "" {
		switch raw {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	if r != nil && r.TLS != nil {
		return true
	}
	if parsed, err := url.Parse(baseURLForRequest(r)); err == nil {
		return strings.EqualFold(parsed.Scheme, "https")
	}
	return false
}

func expireWebSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   sessionCookieSecure(r),
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *app) handleAuthLogoutAndRevoke(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil && strings.TrimSpace(cookie.Value) != "" {
		if err := revokeWebSession(cookie.Value, time.Now().UTC()); err != nil {
			expireWebSessionCookie(w, r)
			writeJSON(w, http.StatusInternalServerError, apiError{
				Error:      "Internal Server Error",
				Message:    "unable to revoke session",
				StatusCode: http.StatusInternalServerError,
			})
			return
		}
	}

	expireWebSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Logged out",
	})
}
