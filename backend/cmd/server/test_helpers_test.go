package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

type contractHarness struct {
	t       *testing.T
	app     *app
	handler http.Handler
}

func newContractHarness(t *testing.T) *contractHarness {
	t.Helper()

	saveRoot := filepath.Join(t.TempDir(), "saves")
	stateRoot := filepath.Join(t.TempDir(), "state")
	t.Setenv("SAVE_ROOT", saveRoot)
	t.Setenv("STATE_ROOT", stateRoot)
	t.Setenv("BOOTSTRAP_DEMO_DATA", "true")

	app := newApp()
	if err := app.initSaveStore(); err != nil {
		t.Fatalf("init save store: %v", err)
	}

	return &contractHarness{
		t:       t,
		app:     app,
		handler: newRouter(app),
	}
}

func (h *contractHarness) do(req *http.Request) *httptest.ResponseRecorder {
	h.t.Helper()
	rr := httptest.NewRecorder()
	h.handler.ServeHTTP(rr, req)
	return rr
}

func (h *contractHarness) json(method, path string, body io.Reader) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer ignored-in-no-auth")
	req.Header.Set("X-CSRF-Protection", "1")
	return h.do(req)
}

func (h *contractHarness) request(method, path string, body io.Reader) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer ignored-in-no-auth")
	req.Header.Set("X-CSRF-Protection", "1")
	return h.do(req)
}

func (h *contractHarness) multipart(path string, fields map[string]string, fileField, fileName string, payload []byte) *httptest.ResponseRecorder {
	h.t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := writer.WriteField(key, fields[key]); err != nil {
			h.t.Fatalf("write multipart field %s: %v", key, err)
		}
	}
	if fileField != "" {
		part, err := writer.CreateFormFile(fileField, fileName)
		if err != nil {
			h.t.Fatalf("create multipart file: %v", err)
		}
		if _, err := part.Write(payload); err != nil {
			h.t.Fatalf("write multipart payload: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		h.t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer ignored-in-no-auth")
	req.Header.Set("X-CSRF-Protection", "1")
	return h.do(req)
}

func decodeJSONMap(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body.Bytes(), &out); err != nil {
		t.Fatalf("decode json: %v\nbody=%s", err, body.String())
	}
	return out
}

func mustObject(t *testing.T, value any, field string) map[string]any {
	t.Helper()
	obj, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("field %s is not an object: %#v", field, value)
	}
	return obj
}

func mustArray(t *testing.T, value any, field string) []any {
	t.Helper()
	arr, ok := value.([]any)
	if !ok {
		t.Fatalf("field %s is not an array: %#v", field, value)
	}
	return arr
}

func mustString(t *testing.T, value any, field string) string {
	t.Helper()
	s, ok := value.(string)
	if !ok {
		t.Fatalf("field %s is not a string: %#v", field, value)
	}
	return s
}

func mustBool(t *testing.T, value any, field string) bool {
	t.Helper()
	b, ok := value.(bool)
	if !ok {
		t.Fatalf("field %s is not a bool: %#v", field, value)
	}
	return b
}

func mustNumber(t *testing.T, value any, field string) float64 {
	t.Helper()
	n, ok := value.(float64)
	if !ok {
		t.Fatalf("field %s is not a number: %#v", field, value)
	}
	return n
}

func assertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("status mismatch: got %d want %d body=%s", rr.Code, want, rr.Body.String())
	}
}

func assertJSONContentType(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	got := rr.Header().Get("Content-Type")
	if !strings.Contains(got, "application/json") {
		t.Fatalf("expected application/json content type, got %q", got)
	}
}

func assertEqualJSONValue(t *testing.T, got, want any, field string) {
	t.Helper()
	var gotBuf bytes.Buffer
	gotEnc := json.NewEncoder(&gotBuf)
	gotEnc.SetEscapeHTML(false)
	if err := gotEnc.Encode(got); err != nil {
		t.Fatalf("marshal got %s: %v", field, err)
	}
	var wantBuf bytes.Buffer
	wantEnc := json.NewEncoder(&wantBuf)
	wantEnc.SetEscapeHTML(false)
	if err := wantEnc.Encode(want); err != nil {
		t.Fatalf("marshal want %s: %v", field, err)
	}
	if gotBuf.String() != wantBuf.String() {
		t.Fatalf("json mismatch for %s: got=%s want=%s", field, gotBuf.String(), wantBuf.String())
	}
}

func uploadSave(t *testing.T, h *contractHarness, path string, fields map[string]string, fileName string, payload []byte) map[string]any {
	t.Helper()
	rr := h.multipart(path, fields, "file", fileName, payload)
	assertStatus(t, rr, http.StatusOK)
	assertJSONContentType(t, rr)
	body := decodeJSONMap(t, rr.Body)
	if !mustBool(t, body["success"], "success") {
		t.Fatalf("expected upload success body=%s", rr.Body.String())
	}
	save := mustObject(t, body["save"], "save")
	if mustString(t, save["id"], "save.id") == "" {
		t.Fatalf("expected non-empty save id")
	}
	if mustString(t, save["sha256"], "save.sha256") == "" {
		t.Fatalf("expected non-empty save sha256")
	}
	return body
}

func createHelperAppPassword(t *testing.T, h *contractHarness, prefix string, name string) string {
	t.Helper()
	path := strings.TrimRight(prefix, "/") + "/auth/app-passwords"
	rr := h.json(http.MethodPost, path, strings.NewReader(`{"name":"`+name+`"}`))
	assertStatus(t, rr, http.StatusOK)
	assertJSONContentType(t, rr)
	body := decodeJSONMap(t, rr.Body)
	key := mustString(t, body["plainTextKey"], "plainTextKey")
	if _, _, ok := normalizeAppPasswordInput(key); !ok {
		t.Fatalf("expected plainTextKey format XXX-XXX, got %q", key)
	}
	return key
}

func createHelperAppPasswordRecord(t *testing.T, h *contractHarness, prefix string, name string) (id string, key string) {
	t.Helper()
	path := strings.TrimRight(prefix, "/") + "/auth/app-passwords"
	rr := h.json(http.MethodPost, path, strings.NewReader(`{"name":"`+name+`"}`))
	assertStatus(t, rr, http.StatusOK)
	assertJSONContentType(t, rr)
	body := decodeJSONMap(t, rr.Body)
	appPassword := mustObject(t, body["appPassword"], "appPassword")
	id = mustString(t, appPassword["id"], "appPassword.id")
	key = mustString(t, body["plainTextKey"], "plainTextKey")
	if _, _, ok := normalizeAppPasswordInput(key); !ok {
		t.Fatalf("expected plainTextKey format XXX-XXX, got %q", key)
	}
	return id, key
}

func normalizeForGolden(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			switch key {
			case "token", "deviceCode", "userCode", "verificationUri":
				if _, ok := child.(string); ok {
					out[key] = "<redacted>"
					continue
				}
			case "id":
				if _, ok := child.(string); ok {
					out[key] = "<redacted>"
					continue
				}
			case "createdAt", "lastSyncedAt", "time":
				if _, ok := child.(string); ok {
					out[key] = "<timestamp>"
					continue
				}
			}
			out[key] = normalizeForGolden(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = normalizeForGolden(child)
		}
		return out
	case float64:
		if typed == float64(int64(typed)) {
			return int64(typed)
		}
		return typed
	default:
		return typed
	}
}

func assertGoldenJSONResponse(t *testing.T, rr *httptest.ResponseRecorder, goldenFile string) {
	t.Helper()
	assertJSONContentType(t, rr)
	var payload any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response json: %v\nbody=%s", err, rr.Body.String())
	}
	normalized := normalizeForGolden(payload)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(normalized); err != nil {
		t.Fatalf("marshal normalized json: %v", err)
	}
	encoded := buf.Bytes()

	fixturePath := filepath.Join("testdata", "golden", goldenFile)
	expected, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read golden fixture %s: %v", fixturePath, err)
	}
	if !bytes.Equal(encoded, expected) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", goldenFile, encoded, expected)
	}
}

func assertGoldenText(t *testing.T, got string, goldenFile string) {
	t.Helper()
	fixturePath := filepath.Join("testdata", "golden", goldenFile)
	expected, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read golden fixture %s: %v", fixturePath, err)
	}
	if got != string(expected) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", goldenFile, got, expected)
	}
}

func (h *contractHarness) ssePrelude(path string) *httptest.ResponseRecorder {
	h.t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer ignored-in-no-auth")
	req.Header.Set("X-CSRF-Protection", "1")

	rr := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.handler.ServeHTTP(rr, req)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if strings.HasPrefix(rr.Body.String(), ": connected\n\n") {
			cancel()
			<-done
			return rr
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			h.t.Fatalf("timed out waiting for SSE prelude, got %q", rr.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func prettyJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(data)
}
