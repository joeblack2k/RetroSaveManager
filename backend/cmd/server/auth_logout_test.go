package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleAuthLogoutClearsSessionCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rr := httptest.NewRecorder()

	newApp().handleAuthLogout(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1; Set-Cookie=%q", len(cookies), rr.Header().Get("Set-Cookie"))
	}

	cookie := cookies[0]
	if cookie.Name != "session" {
		t.Fatalf("cookie name = %q, want session", cookie.Name)
	}
	if cookie.Value != "" {
		t.Fatalf("cookie value = %q, want empty", cookie.Value)
	}
	if cookie.Path != "/" {
		t.Fatalf("cookie path = %q, want /", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("session deletion cookie must remain HttpOnly")
	}
	if cookie.MaxAge >= 0 {
		t.Fatalf("cookie MaxAge = %d, want negative deletion value", cookie.MaxAge)
	}
	if !cookie.Expires.Before(time.Now()) {
		t.Fatalf("cookie expiry = %s, want a time in the past", cookie.Expires)
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie SameSite = %d, want Lax", cookie.SameSite)
	}
}
