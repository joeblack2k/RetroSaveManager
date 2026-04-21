package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContractAdditionalAuthAndWebRoutes(t *testing.T) {
	cases := []struct {
		method string
		path   string
		status int
		body   string
	}{
		{method: http.MethodPost, path: "/auth/signup", status: http.StatusOK, body: `{"email":"a@b.c","password":"test"}`},
		{method: http.MethodPost, path: "/auth/logout", status: http.StatusOK, body: `{}`},
		{method: http.MethodPost, path: "/auth/resend-verification", status: http.StatusOK, body: `{"email":"a@b.c"}`},
		{method: http.MethodPost, path: "/auth/verify-email", status: http.StatusOK, body: `{"token":"abc"}`},
		{method: http.MethodPost, path: "/auth/forgot-password", status: http.StatusOK, body: `{"email":"a@b.c"}`},
		{method: http.MethodPost, path: "/auth/reset-password", status: http.StatusOK, body: `{"token":"abc","newPassword":"x"}`},
		{method: http.MethodPost, path: "/auth/delete-account", status: http.StatusOK, body: `{}`},
		{method: http.MethodPost, path: "/auth/cancel-deletion", status: http.StatusOK, body: `{}`},
		{method: http.MethodPost, path: "/auth/2fa/verify", status: http.StatusOK, body: `{"code":"123456"}`},
		{method: http.MethodPost, path: "/auth/2fa/setup/totp", status: http.StatusOK, body: `{}`},
		{method: http.MethodPost, path: "/auth/2fa/verify-setup", status: http.StatusOK, body: `{"code":"123456"}`},
		{method: http.MethodGet, path: "/auth/2fa/status", status: http.StatusOK},
		{method: http.MethodPost, path: "/auth/2fa/disable", status: http.StatusOK, body: `{"password":"ignored"}`},
		{method: http.MethodPost, path: "/auth/2fa/backup-codes/regenerate", status: http.StatusOK, body: `{}`},
		{method: http.MethodGet, path: "/auth/2fa/trusted-devices", status: http.StatusOK},
		{method: http.MethodDelete, path: "/auth/2fa/trusted-devices/trusted-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/auth/app-passwords", status: http.StatusOK},
		{method: http.MethodPost, path: "/auth/app-passwords", status: http.StatusOK, body: `{"name":"ci"}`},
		{method: http.MethodDelete, path: "/auth/app-passwords/app-password-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/referral", status: http.StatusOK},
		{method: http.MethodPost, path: "/dev/signup", status: http.StatusOK, body: `{}`},
		{method: http.MethodGet, path: "/parser/wasm?game_id=281", status: http.StatusOK},
		{method: http.MethodGet, path: "/catalog", status: http.StatusOK},
		{method: http.MethodGet, path: "/catalog/cat-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/catalog/cat-1/download", status: http.StatusOK},
		{method: http.MethodGet, path: "/games/library", status: http.StatusOK},
		{method: http.MethodPost, path: "/games/library", status: http.StatusOK, body: `{"catalogId":"cat-1"}`},
		{method: http.MethodDelete, path: "/games/library/lib-1", status: http.StatusOK},
		{method: http.MethodGet, path: "/roadmap/items", status: http.StatusOK},
		{method: http.MethodPost, path: "/roadmap/items/roadmap-1/vote", status: http.StatusOK, body: `{}`},
		{method: http.MethodPost, path: "/roadmap/suggestions", status: http.StatusOK, body: `{"title":"Offline diff viewer","description":"For handheld workflows"}`},
		{method: http.MethodGet, path: "/roadmap/suggestions/mine", status: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			h := newContractHarness(t)
			var rr *httptest.ResponseRecorder
			if tc.body != "" {
				rr = h.json(tc.method, tc.path, strings.NewReader(tc.body))
			} else {
				rr = h.request(tc.method, tc.path, nil)
			}
			assertStatus(t, rr, tc.status)

			// Keep binary content for parser/wasm and catalog downloads.
			if !strings.HasPrefix(tc.path, "/parser/wasm") && !strings.HasSuffix(tc.path, "/download") {
				assertJSONContentType(t, rr)
			}

			hV1 := newContractHarness(t)
			v1Path := "/v1" + tc.path
			if tc.body != "" {
				rr = hV1.json(tc.method, v1Path, strings.NewReader(tc.body))
			} else {
				rr = hV1.request(tc.method, v1Path, nil)
			}
			assertStatus(t, rr, tc.status)
		})
	}
}

func TestContractAdditionalSaveRollbackAliasParity(t *testing.T) {
	h := newContractHarness(t)
	h.app.mu.Lock()
	if len(h.app.saveRecords) == 0 {
		h.app.mu.Unlock()
		t.Fatal("expected seeded save record")
	}
	saveID := h.app.saveRecords[0].Summary.ID
	h.app.mu.Unlock()

	body := fmt.Sprintf(`{"saveId":"%s"}`, saveID)
	root := h.json(http.MethodPost, "/save/rollback", strings.NewReader(body))
	assertStatus(t, root, http.StatusOK)
	assertJSONContentType(t, root)

	hV1 := newContractHarness(t)
	hV1.app.mu.Lock()
	saveID = hV1.app.saveRecords[0].Summary.ID
	hV1.app.mu.Unlock()
	body = fmt.Sprintf(`{"saveId":"%s"}`, saveID)
	v1 := hV1.json(http.MethodPost, "/v1/save/rollback", strings.NewReader(body))
	assertStatus(t, v1, http.StatusOK)
	assertJSONContentType(t, v1)
}
