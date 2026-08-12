package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxMultipartBodyBytes(t *testing.T) {
	t.Setenv("MAX_MULTIPART_BODY_BYTES", "")
	if got := maxMultipartBodyBytes(); got != defaultMaxMultipartBodyBytes {
		t.Fatalf("empty environment value = %d, want default %d", got, defaultMaxMultipartBodyBytes)
	}

	t.Setenv("MAX_MULTIPART_BODY_BYTES", "4096")
	if got := maxMultipartBodyBytes(); got != 4096 {
		t.Fatalf("configured limit = %d, want 4096", got)
	}

	for _, value := range []string{"0", "-1", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("MAX_MULTIPART_BODY_BYTES", value)
			if got := maxMultipartBodyBytes(); got != defaultMaxMultipartBodyBytes {
				t.Fatalf("invalid environment value %q = %d, want default %d", value, got, defaultMaxMultipartBodyBytes)
			}
		})
	}
}

func TestLimitMultipartRequestBodyRejectsKnownOversizeRequest(t *testing.T) {
	t.Setenv("MAX_MULTIPART_BODY_BYTES", "64")

	req := httptest.NewRequest(http.MethodPost, "/saves", strings.NewReader(strings.Repeat("x", 65)))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	rr := httptest.NewRecorder()
	called := false

	limitMultipartRequestBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})).ServeHTTP(rr, req)

	if called {
		t.Fatal("oversize request reached the downstream handler")
	}
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

func TestLimitMultipartRequestBodyCapsUnknownLengthStream(t *testing.T) {
	t.Setenv("MAX_MULTIPART_BODY_BYTES", "64")

	req := httptest.NewRequest(http.MethodPost, "/saves", strings.NewReader(strings.Repeat("x", 65)))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	req.ContentLength = -1
	rr := httptest.NewRecorder()
	var readErr error

	limitMultipartRequestBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)

	var maxBytesErr *http.MaxBytesError
	if !errors.As(readErr, &maxBytesErr) {
		t.Fatalf("read error = %v, want *http.MaxBytesError", readErr)
	}
	if maxBytesErr.Limit != 64 {
		t.Fatalf("MaxBytesError limit = %d, want 64", maxBytesErr.Limit)
	}
}

func TestLimitMultipartRequestBodyDoesNotLimitJSON(t *testing.T) {
	t.Setenv("MAX_MULTIPART_BODY_BYTES", "8")

	body := strings.Repeat("x", 64)
	req := httptest.NewRequest(http.MethodPost, "/example", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	readBytes := 0
	var readErr error

	limitMultipartRequestBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		readBytes = len(data)
		readErr = err
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)

	if readErr != nil {
		t.Fatalf("read JSON body: %v", readErr)
	}
	if readBytes != len(body) {
		t.Fatalf("read %d JSON bytes, want %d", readBytes, len(body))
	}
}

func TestRouterInstallsMultipartRequestLimit(t *testing.T) {
	t.Setenv("AUTH_MODE", "disabled")
	t.Setenv("MAX_MULTIPART_BODY_BYTES", "32")

	req := httptest.NewRequest(http.MethodPost, "/saves", strings.NewReader(strings.Repeat("x", 33)))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	rr := httptest.NewRecorder()

	newRouter(newApp()).ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
}
