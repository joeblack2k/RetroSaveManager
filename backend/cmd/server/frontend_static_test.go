package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouterServesFrontendSPA(t *testing.T) {
	distDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(filepath.Join(distDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte("<!doctype html><html><body>RetroSaveManager UI</body></html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "assets", "app.js"), []byte("console.log('ok');"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	t.Setenv("FRONTEND_DIST_DIR", distDir)

	app := newApp()
	handler := newRouter(app)

	t.Run("spa fallback", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/app/my-games", nil)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "RetroSaveManager UI") {
			t.Fatalf("expected index html body, got %s", rr.Body.String())
		}
		if !strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("expected html content type, got %q", rr.Header().Get("Content-Type"))
		}
	})

	t.Run("static asset", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "console.log") {
			t.Fatalf("expected static asset body, got %s", rr.Body.String())
		}
	})
}

func TestRouterUnknownAPIPathReturnsAPI404(t *testing.T) {
	distDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte("<!doctype html><html>UI</html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	t.Setenv("FRONTEND_DIST_DIR", distDir)

	app := newApp()
	handler := newRouter(app)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/does-not-exist", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("expected json content type, got %q", rr.Header().Get("Content-Type"))
	}
	if strings.Contains(strings.ToLower(rr.Body.String()), "<html") {
		t.Fatalf("expected api 404 payload instead of html body=%s", rr.Body.String())
	}
}

func TestIsReservedAPIPath(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		want  bool
	}{
		{name: "auth root", path: "/auth", want: true},
		{name: "auth child", path: "/auth/me", want: true},
		{name: "authentic should not match auth", path: "/authentic", want: false},
		{name: "saves child", path: "/saves/download", want: true},
		{name: "catalog child", path: "/catalog/123", want: true},
		{name: "healthz exact", path: "/healthz", want: true},
		{name: "healthz child should not match", path: "/healthz/extra", want: false},
		{name: "spa route", path: "/app/my-games", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isReservedAPIPath(tc.path)
			if got != tc.want {
				t.Fatalf("path=%q got %v want %v", tc.path, got, tc.want)
			}
		})
	}
}
