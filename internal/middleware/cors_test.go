package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	exposeHeaders := resp.Header().Get("Access-Control-Expose-Headers")
	if exposeHeaders == "" {
		t.Fatalf("expected Access-Control-Expose-Headers to be set")
	}
	for _, h := range []string{"Link", "Location", "X-Request-Id"} {
		if !containsHeader(exposeHeaders, h) {
			t.Fatalf("expected Access-Control-Expose-Headers to contain %q, got %q", h, exposeHeaders)
		}
	}
}

func containsHeader(headerValue, target string) bool {
	for part := range strings.SplitSeq(headerValue, ",") {
		if strings.EqualFold(strings.TrimSpace(part), target) {
			return true
		}
	}
	return false
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

func TestCORSAllowsXRequestIDHeader(t *testing.T) {
	fn := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := CORS()(fn)
	req := httptest.NewRequest(http.MethodOptions, "http://localhost/resource", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "X-Request-ID")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 for preflight with X-Request-ID, got %d", resp.Code)
	}
	allowHeaders := resp.Header().Get("Access-Control-Allow-Headers")
	if !containsHeader(allowHeaders, "X-Request-Id") {
		t.Fatalf("expected Access-Control-Allow-Headers to contain X-Request-ID, got %q", allowHeaders)
	}
}

func TestCORSAllowsTraceparentHeader(t *testing.T) {
	fn := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := CORS()(fn)
	req := httptest.NewRequest(http.MethodOptions, "http://localhost/resource", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "traceparent")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 for preflight with traceparent, got %d", resp.Code)
	}
	allowHeaders := resp.Header().Get("Access-Control-Allow-Headers")
	if !containsHeader(allowHeaders, "traceparent") {
		t.Fatalf("expected Access-Control-Allow-Headers to contain traceparent, got %q", allowHeaders)
	}
}
