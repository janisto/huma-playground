package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVaryMiddlewareSetsHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := Vary()(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	vary := resp.Header().Get("Vary")
	if vary != "Accept" {
		t.Fatalf("expected Vary: Accept, got %q", vary)
	}
}

func TestVaryMiddlewarePreservesDownstreamResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("test body"))
	})

	h := Vary()(handler)
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
	if resp.Header().Get("Vary") != "Accept" {
		t.Fatalf("expected Vary header to be set")
	}
}
