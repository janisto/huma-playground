package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIgnoreForwardedHeaders(t *testing.T) {
	handler := IgnoreForwardedHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, header := range untrustedForwardingHeaders {
			if value := r.Header.Get(header); value != "" {
				t.Errorf("expected %s to be removed, got %q", header, value)
			}
		}
		if r.Host != "api.example.com" {
			t.Errorf("expected Host to be preserved, got %q", r.Host)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/probe", nil)
	for _, header := range untrustedForwardingHeaders {
		request.Header.Set(header, "attacker.example")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.Code)
	}
}
