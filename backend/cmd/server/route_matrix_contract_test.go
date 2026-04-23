package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type contractRouteMatrix struct {
	Counts map[string]int  `json:"counts"`
	Routes []contractRoute `json:"routes"`
}

type contractRoute struct {
	Method           string `json:"method"`
	Path             string `json:"path"`
	Scope            string `json:"scope"`
	ExpectedStatuses []int  `json:"expected_statuses"`
}

type compatRequest struct {
	Method      string
	Path        string
	Body        []byte
	ContentType string
	IsSSE       bool
}

func TestRouteMatrixRequiredAndStubRequiredParity(t *testing.T) {
	matrix := loadContractRouteMatrix(t)
	routes := make([]contractRoute, 0)
	for _, route := range matrix.Routes {
		if route.Scope == "required" || route.Scope == "stub-required" {
			routes = append(routes, route)
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})

	const expectedSurfacedRoutes = 54
	if len(routes) != expectedSurfacedRoutes {
		t.Fatalf("unexpected surfaced route count: got %d want %d", len(routes), expectedSurfacedRoutes)
	}

	for _, route := range routes {
		route := route
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			rootHarness := newContractHarness(t)
			rootReq := buildCompatRequest(t, rootHarness, "", route)
			rootResp := executeCompatRequest(t, rootHarness, rootReq)

			v1Harness := newContractHarness(t)
			v1Req := buildCompatRequest(t, v1Harness, "/v1", route)
			v1Resp := executeCompatRequest(t, v1Harness, v1Req)

			if rootResp.Code != v1Resp.Code {
				t.Fatalf("status mismatch root=%d v1=%d", rootResp.Code, v1Resp.Code)
			}
			if rootResp.Code == http.StatusNotFound {
				t.Fatalf("unexpected 404 for surfaced route %s %s", route.Method, route.Path)
			}
			if !isExpectedStatus(rootResp.Code, route.ExpectedStatuses) {
				t.Fatalf("unexpected status %d for %s %s; expected one of %v", rootResp.Code, route.Method, route.Path, route.ExpectedStatuses)
			}
		})
	}
}

func buildCompatRequest(t *testing.T, h *contractHarness, prefix string, route contractRoute) compatRequest {
	t.Helper()

	req := compatRequest{
		Method: route.Method,
		Path:   prefix + route.Path,
	}

	switch route.Path {
	case "/auth/login":
		req.Body = jsonBody(t, map[string]any{"email": "internal@local", "password": "test", "device_type": "retroarch", "fingerprint": "seed-1"})
		req.ContentType = "application/json"
	case "/auth/token":
		req.Body = jsonBody(t, map[string]any{"email": "internal@local", "password": "test"})
		req.ContentType = "application/json"
	case "/auth/token/app-password":
		req.Body = jsonBody(t, map[string]any{"name": "default"})
		req.ContentType = "application/json"
	case "/auth/signup":
		req.Body = jsonBody(t, map[string]any{"email": "internal@local", "password": "test"})
		req.ContentType = "application/json"
	case "/auth/resend-verification":
		req.Body = jsonBody(t, map[string]any{"email": "internal@local"})
		req.ContentType = "application/json"
	case "/auth/verify-email":
		req.Body = jsonBody(t, map[string]any{"token": "abc"})
		req.ContentType = "application/json"
	case "/auth/forgot-password":
		req.Body = jsonBody(t, map[string]any{"email": "internal@local"})
		req.ContentType = "application/json"
	case "/auth/reset-password":
		req.Body = jsonBody(t, map[string]any{"token": "abc", "newPassword": "test"})
		req.ContentType = "application/json"
	case "/auth/delete-account":
		req.Body = jsonBody(t, map[string]any{"confirm": true})
		req.ContentType = "application/json"
	case "/auth/cancel-deletion":
		req.Body = jsonBody(t, map[string]any{"confirm": true})
		req.ContentType = "application/json"
	case "/auth/device":
		req.Body = jsonBody(t, map[string]any{"client": "helper"})
		req.ContentType = "application/json"
	case "/auth/device/token":
		req.Body = jsonBody(t, map[string]any{"deviceCode": "device-code"})
		req.ContentType = "application/json"
	case "/auth/device/verify":
		req.Body = jsonBody(t, map[string]any{"userCode": "ABCD"})
		req.ContentType = "application/json"
	case "/auth/device/confirm":
		req.Body = jsonBody(t, map[string]any{"userCode": "ABCD"})
		req.ContentType = "application/json"
	case "/auth/2fa/verify":
		req.Body = jsonBody(t, map[string]any{"code": "123456"})
		req.ContentType = "application/json"
	case "/auth/2fa/setup/totp":
		req.Body = jsonBody(t, map[string]any{"confirm": true})
		req.ContentType = "application/json"
	case "/auth/2fa/verify-setup":
		req.Body = jsonBody(t, map[string]any{"code": "123456"})
		req.ContentType = "application/json"
	case "/auth/2fa/disable":
		req.Body = jsonBody(t, map[string]any{"password": "ignored"})
		req.ContentType = "application/json"
	case "/auth/2fa/backup-codes/regenerate":
		req.Body = jsonBody(t, map[string]any{"confirm": true})
		req.ContentType = "application/json"
	case "/auth/2fa/trusted-devices/{id}":
		req.Path = prefix + "/auth/2fa/trusted-devices/trusted-1"
	case "/auth/app-passwords":
		if route.Method == http.MethodPost {
			req.Body = jsonBody(t, map[string]any{"name": "cli"})
			req.ContentType = "application/json"
		}
	case "/auth/app-passwords/{id}":
		req.Path = prefix + "/auth/app-passwords/app-password-1"
	case "/save/latest":
		req.Path += "?romSha1=missing-rom&slotName=default"
	case "/save":
		if route.Method == http.MethodGet {
			req.Path += "?gameId=" + seededSaveGameID(t, h)
		} else if route.Method == http.MethodDelete {
			req.Path += "?id=" + seededSaveID(t, h)
		}
	case "/saves":
		if route.Method == http.MethodGet {
			req.Path += "?limit=5&offset=0"
		} else if route.Method == http.MethodPost {
			helperKey := createHelperAppPassword(t, h, prefix, "matrix-helper")
			body, contentType := multipartBody(t, map[string]string{
				"app_password": helperKey,
				"rom_sha1":     "compat-rom",
				"system":       "gameboy",
				"slotName":     "default",
				"device_type":  "retroarch",
				"fingerprint":  "seed-1",
			}, "file", "compat.srm", []byte("compat-save"))
			req.Body = body
			req.ContentType = contentType
		} else if route.Method == http.MethodDelete {
			req.Path += "?ids=" + seededSaveID(t, h)
		}
	case "/saves/download":
		req.Path += "?id=" + seededSaveID(t, h)
	case "/saves/download_many":
		req.Path += "?ids=" + seededSaveID(t, h)
	case "/saves/systems":
		// No-op.
	case "/game/saves":
		req.Path += "?gameIds=" + seededSaveGameID(t, h)
	case "/rom/lookup":
		if route.Method == http.MethodGet {
			req.Path += "?filenameStem=Wario%20Land%20II"
		} else {
			req.Body = jsonBody(t, map[string]any{"filenameStem": "Wario Land II"})
			req.ContentType = "application/json"
		}
	case "/conflicts/check":
		if route.Method == http.MethodGet {
			req.Path += "?romSha1=missing-rom&slotName=default"
		} else {
			req.Body = jsonBody(t, map[string]any{"romSha1": "missing-rom", "slotName": "default"})
			req.ContentType = "application/json"
		}
	case "/conflicts/report":
		body, contentType := multipartBody(t, map[string]string{
			"romSha1":     "compat-conflict-rom",
			"slotName":    "default",
			"localSha256": "local-sha",
			"cloudSha256": "cloud-sha",
			"deviceType":  "retroarch",
			"fingerprint": "seed-1",
		}, "file", "conflict.srm", []byte("conflict-bytes"))
		req.Body = body
		req.ContentType = contentType
	case "/conflicts/{id}":
		req.Path = prefix + "/conflicts/" + seededConflictID(t, h, prefix)
	case "/conflicts/{id}/resolve":
		req.Path = prefix + "/conflicts/" + seededConflictID(t, h, prefix) + "/resolve"
		req.Body = jsonBody(t, map[string]any{"resolution": "cloud"})
		req.ContentType = "application/json"
	case "/games/lookup":
		req.Body = jsonBody(t, map[string]any{
			"items": []map[string]any{
				{"type": "name", "value": map[string]any{"name": "Wario Land II"}},
			},
		})
		req.ContentType = "application/json"
	case "/devices/{id}":
		req.Path = prefix + "/devices/1"
		if route.Method == http.MethodPatch {
			req.Body = jsonBody(t, map[string]any{"alias": "Living Room"})
			req.ContentType = "application/json"
		}
	case "/events":
		req.IsSSE = true
	}

	return req
}

func executeCompatRequest(t *testing.T, h *contractHarness, req compatRequest) *httptest.ResponseRecorder {
	t.Helper()
	if req.IsSSE {
		return h.ssePrelude(req.Path)
	}

	httpReq := httptest.NewRequest(req.Method, req.Path, bytes.NewReader(req.Body))
	if req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	}
	httpReq.Header.Set("Authorization", "Bearer ignored-in-no-auth")
	httpReq.Header.Set("X-CSRF-Protection", "1")
	return h.do(httpReq)
}

func seededSaveID(t *testing.T, h *contractHarness) string {
	t.Helper()
	h.app.mu.Lock()
	defer h.app.mu.Unlock()
	if len(h.app.saveRecords) == 0 {
		t.Fatalf("expected at least one seeded save record")
	}
	return h.app.saveRecords[0].Summary.ID
}

func seededSaveGameID(t *testing.T, h *contractHarness) string {
	t.Helper()
	h.app.mu.Lock()
	defer h.app.mu.Unlock()
	if len(h.app.saveRecords) == 0 {
		t.Fatalf("expected at least one seeded save record")
	}
	return fmt.Sprintf("%d", canonicalSummaryForRecord(h.app.saveRecords[0]).Game.ID)
}

func seededConflictID(t *testing.T, h *contractHarness, prefix string) string {
	t.Helper()
	body, contentType := multipartBody(t, map[string]string{
		"romSha1":     "seed-conflict-rom",
		"slotName":    "default",
		"localSha256": "local-seed-sha",
		"cloudSha256": "cloud-seed-sha",
		"deviceType":  "retroarch",
		"fingerprint": "seed-1",
	}, "file", "seed-conflict.srm", []byte("seed-conflict"))

	req := compatRequest{
		Method:      http.MethodPost,
		Path:        prefix + "/conflicts/report",
		Body:        body,
		ContentType: contentType,
	}
	rr := executeCompatRequest(t, h, req)
	assertStatus(t, rr, http.StatusOK)
	response := decodeJSONMap(t, rr.Body)
	return mustString(t, response["conflictId"], "conflictId")
}

func isExpectedStatus(status int, allowed []int) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if candidate == status {
			return true
		}
	}
	return false
}

func jsonBody(t *testing.T, payload any) []byte {
	t.Helper()
	out, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal json body: %v", err)
	}
	return out
}

func multipartBody(t *testing.T, fields map[string]string, fileField, fileName string, payload []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := writer.WriteField(key, fields[key]); err != nil {
			t.Fatalf("write multipart field %s: %v", key, err)
		}
	}

	if fileField != "" {
		part, err := writer.CreateFormFile(fileField, fileName)
		if err != nil {
			t.Fatalf("create multipart file %s: %v", fileName, err)
		}
		payload = normalizeTestUploadPayload(fields, fileName, payload)
		if _, err := part.Write(payload); err != nil {
			t.Fatalf("write multipart payload: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return buf.Bytes(), writer.FormDataContentType()
}

func loadContractRouteMatrix(t *testing.T) contractRouteMatrix {
	t.Helper()

	path := filepath.Join("..", "..", "contracts", "route-matrix.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read route matrix %s: %v", path, err)
	}

	var matrix contractRouteMatrix
	if err := json.Unmarshal(raw, &matrix); err != nil {
		t.Fatalf("decode route matrix json: %v", err)
	}

	required := matrix.Counts["required"]
	stubRequired := matrix.Counts["stub-required"]
	if required+stubRequired != 54 {
		t.Fatalf("unexpected route matrix counts: required=%d stub-required=%d", required, stubRequired)
	}

	for i := range matrix.Routes {
		matrix.Routes[i].Method = strings.ToUpper(strings.TrimSpace(matrix.Routes[i].Method))
	}
	return matrix
}
