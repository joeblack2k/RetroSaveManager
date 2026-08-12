package main

import (
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	loginFailureWindow     = 5 * time.Minute
	loginFailureLimit      = 5
	loginBlockDuration     = time.Minute
	loginThrottleMaxKeys   = 1024
	csrfProtectionHeader   = "X-CSRF-Protection"
	csrfProtectionExpected = "1"
)

type loginAttemptState struct {
	Failures      int
	WindowStarted time.Time
	BlockedUntil  time.Time
	LastAttempt   time.Time
}

var loginThrottleState = struct {
	sync.Mutex
	attempts map[string]loginAttemptState
}{attempts: map[string]loginAttemptState{}}

func trustProxyHeaders() bool {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("TRUST_PROXY_HEADERS"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func isUnsafeHTTPMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// requireBrowserWriteProtection adds a second security boundary for browser
// sessions. A cross-site HTML form can carry cookies, but cannot set this
// custom header without a successful CORS preflight. Helper app-password calls
// remain protocol-authenticated and do not need browser CSRF state.
func (a *app) requireBrowserWriteProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authMode() != "enabled" || !isUnsafeHTTPMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if isHelperProtocolRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		browserAuthenticated := false
		if cookie, err := r.Cookie("session"); err == nil && validateWebSession(cookie.Value, time.Now().UTC()) {
			browserAuthenticated = true
		}
		if trustRemoteUserHeader() && strings.TrimSpace(r.Header.Get("Remote-User")) != "" {
			browserAuthenticated = true
		}
		if !browserAuthenticated {
			// Public bootstrap endpoints and direct app-password API clients do not
			// rely on RSM's browser session cookie, so CSRF does not apply here.
			next.ServeHTTP(w, r)
			return
		}

		if strings.TrimSpace(r.Header.Get(csrfProtectionHeader)) != csrfProtectionExpected {
			writeJSON(w, http.StatusForbidden, apiError{
				Error:      "Forbidden",
				Message:    "browser write request requires CSRF protection",
				StatusCode: http.StatusForbidden,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loginThrottleKey(r *http.Request, email string) string {
	return loginClientIP(r) + "|" + strings.TrimSpace(strings.ToLower(email))
}

func loginClientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	remote := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remote); err == nil && strings.TrimSpace(host) != "" {
		return strings.TrimSpace(host)
	}
	if remote == "" {
		return "unknown"
	}
	return remote
}

func pruneLoginThrottleLocked(now time.Time) {
	for key, state := range loginThrottleState.attempts {
		staleAfter := state.LastAttempt.Add(loginFailureWindow + loginBlockDuration)
		if !state.BlockedUntil.IsZero() && state.BlockedUntil.After(staleAfter) {
			staleAfter = state.BlockedUntil
		}
		if now.After(staleAfter) {
			delete(loginThrottleState.attempts, key)
		}
	}
	if len(loginThrottleState.attempts) <= loginThrottleMaxKeys {
		return
	}
	// Defensive bound: discard the oldest records first. The normal single-user
	// deployment should never approach this size.
	for len(loginThrottleState.attempts) > loginThrottleMaxKeys {
		oldestKey := ""
		var oldest time.Time
		for key, state := range loginThrottleState.attempts {
			if oldestKey == "" || state.LastAttempt.Before(oldest) {
				oldestKey = key
				oldest = state.LastAttempt
			}
		}
		if oldestKey == "" {
			break
		}
		delete(loginThrottleState.attempts, oldestKey)
	}
}

func loginRetryAfter(r *http.Request, email string, now time.Time) time.Duration {
	key := loginThrottleKey(r, email)
	loginThrottleState.Lock()
	defer loginThrottleState.Unlock()
	pruneLoginThrottleLocked(now)
	state, ok := loginThrottleState.attempts[key]
	if !ok || state.BlockedUntil.IsZero() || !now.Before(state.BlockedUntil) {
		return 0
	}
	return state.BlockedUntil.Sub(now)
}

func recordFailedLogin(r *http.Request, email string, now time.Time) time.Duration {
	key := loginThrottleKey(r, email)
	loginThrottleState.Lock()
	defer loginThrottleState.Unlock()
	pruneLoginThrottleLocked(now)
	state := loginThrottleState.attempts[key]
	if state.WindowStarted.IsZero() || now.Sub(state.WindowStarted) >= loginFailureWindow || (!state.BlockedUntil.IsZero() && !now.Before(state.BlockedUntil)) {
		state = loginAttemptState{WindowStarted: now}
	}
	state.Failures++
	state.LastAttempt = now
	if state.Failures >= loginFailureLimit {
		state.BlockedUntil = now.Add(loginBlockDuration)
	}
	loginThrottleState.attempts[key] = state
	if now.Before(state.BlockedUntil) {
		return state.BlockedUntil.Sub(now)
	}
	return 0
}

func clearFailedLogins(r *http.Request, email string) {
	key := loginThrottleKey(r, email)
	loginThrottleState.Lock()
	delete(loginThrottleState.attempts, key)
	loginThrottleState.Unlock()
}

func writeLoginRateLimited(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSON(w, http.StatusTooManyRequests, apiError{
		Error:      "Too Many Requests",
		Message:    "too many failed login attempts; retry later",
		StatusCode: http.StatusTooManyRequests,
	})
}

func resetLoginThrottleForTest() {
	loginThrottleState.Lock()
	loginThrottleState.attempts = map[string]loginAttemptState{}
	loginThrottleState.Unlock()
}
