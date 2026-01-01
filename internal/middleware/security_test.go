package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityMiddlewareSetsHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := Security()(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	tests := []struct {
		header string
		want   string
	}{
		{"Cache-Control", "no-store"},
		{"Content-Security-Policy", "frame-ancestors 'none'"},
		{"Cross-Origin-Opener-Policy", "same-origin"},
		{"Cross-Origin-Resource-Policy", "same-origin"},
		{
			"Permissions-Policy",
			"accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
		},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
	}

	for _, tt := range tests {
		got := resp.Header().Get(tt.header)
		if got != tt.want {
			t.Errorf("%s: expected %q, got %q", tt.header, tt.want, got)
		}
	}
}

func TestSecurityMiddlewarePreservesDownstreamResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("test body"))
	})

	h := Security()(handler)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.Code)
	}
	if resp.Header().Get("X-Custom") != "value" {
		t.Fatalf("expected X-Custom header to be preserved")
	}
	if resp.Body.String() != "test body" {
		t.Fatalf("expected body to be preserved")
	}
}

func TestSecurityMiddlewareDoesNotOverrideDownstreamHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=3600")
		w.WriteHeader(http.StatusOK)
	})

	h := Security()(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	got := resp.Header().Get("Cache-Control")
	if got != "max-age=3600" {
		t.Errorf("expected downstream Cache-Control to be preserved, got %q", got)
	}
}

func TestSecurityMiddlewareSkipsExcludedPaths(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := Security("/api-docs", "/health")(handler)

	tests := []struct {
		path        string
		wantHeaders bool
	}{
		{"/api-docs", false},
		{"/api-docs/", false},
		{"/health", false},
		{"/health/live", false},
		{"/api", true},
		{"/users", true},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		resp := httptest.NewRecorder()

		h.ServeHTTP(resp, req)

		hasHeaders := resp.Header().Get("X-Content-Type-Options") == "nosniff"
		if hasHeaders != tt.wantHeaders {
			t.Errorf("%s: expected headers=%v, got headers=%v", tt.path, tt.wantHeaders, hasHeaders)
		}
	}
}
