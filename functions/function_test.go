package hello

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHelloHandler(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		target      string
		body        string
		contentType string
		wantStatus  int
		wantMessage string
	}{
		{name: "default", method: http.MethodGet, target: "/", wantStatus: http.StatusOK, wantMessage: "Hello, World!"},
		{
			name:        "query",
			method:      http.MethodGet,
			target:      "/?name=Ada",
			wantStatus:  http.StatusOK,
			wantMessage: "Hello, Ada!",
		},
		{
			name: "JSON with charset", method: http.MethodPost, target: "/?name=query",
			body: `{"name":"Grace"}`, contentType: "application/json; charset=utf-8",
			wantStatus: http.StatusOK, wantMessage: "Hello, Grace!",
		},
		{
			name:        "empty body",
			method:      http.MethodPost,
			target:      "/",
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "malformed JSON",
			method:      http.MethodPost,
			target:      "/",
			body:        `{`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "null",
			method:      http.MethodPost,
			target:      "/",
			body:        `null`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "unknown field",
			method:      http.MethodPost,
			target:      "/",
			body:        `{"unknown":true}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "multiple values",
			method:      http.MethodPost,
			target:      "/",
			body:        `{} {}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:       "name too long",
			method:     http.MethodGet,
			target:     "/?name=" + strings.Repeat("a", 101),
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "body too large", method: http.MethodPost, target: "/",
			body:        `{"name":"` + strings.Repeat("a", maxBodyBytes) + `"}`,
			contentType: "application/json", wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name: "body too large after object", method: http.MethodPost, target: "/",
			body:        `{}` + strings.Repeat(" ", maxBodyBytes),
			contentType: "application/json", wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:       "missing content type",
			method:     http.MethodPost,
			target:     "/",
			body:       `{}`,
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:        "unsupported content type",
			method:      http.MethodPost,
			target:      "/",
			body:        `{}`,
			contentType: "text/plain",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{name: "unsupported method", method: http.MethodDelete, target: "/", wantStatus: http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.target, strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			response := httptest.NewRecorder()

			helloHandler(response, req)

			if response.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, response.Code, response.Body.String())
			}
			if tt.method == http.MethodDelete && response.Header().Get("Allow") != "GET, POST" {
				t.Fatalf("expected Allow header, got %q", response.Header().Get("Allow"))
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			if got := response.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected application/json, got %q", got)
			}
			var body helloResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Message != tt.wantMessage {
				t.Fatalf("expected %q, got %q", tt.wantMessage, body.Message)
			}
			if _, err := time.Parse(rfc3339Millis, body.Timestamp); err != nil {
				t.Fatalf("invalid timestamp %q: %v", body.Timestamp, err)
			}
		})
	}
}
