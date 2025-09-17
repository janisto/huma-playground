package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsGETOrigin(t *testing.T) {
	called := false
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	h := CORS()(fn)
	req := httptest.NewRequest(http.MethodGet, "http://localhost/resource", nil)
	req.Header.Set("Origin", "http://example.com")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if !called {
		t.Fatalf("expected downstream handler to be called for GET request")
	}
	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin '*', got %q", got)
	}
	if got := resp.Header().Get("Access-Control-Expose-Headers"); got != "Link" {
		t.Fatalf("expected Access-Control-Expose-Headers 'Link', got %q", got)
	}
}

func TestCORSHandlesPreflightWithoutCallingNext(t *testing.T) {
	called := false
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	h := CORS()(fn)
	req := httptest.NewRequest(http.MethodOptions, "http://localhost/resource", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if called {
		t.Fatalf("expected preflight request to be handled by CORS middleware without calling downstream handler")
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 for preflight, got %d", resp.Code)
	}
	if got := resp.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Methods header to be set")
	}
	if got := resp.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Headers header to be set")
	}
	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin '*', got %q", got)
	}
}
