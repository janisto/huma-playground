package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp := httptest.NewRecorder()
	Handler(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.Code)
	}

	if ct := resp.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var h Response
	if err := json.Unmarshal(resp.Body.Bytes(), &h); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if h.Status != "healthy" {
		t.Fatalf("expected status 'healthy', got %s", h.Status)
	}
}
