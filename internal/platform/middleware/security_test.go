package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersApplyToEveryRoute(t *testing.T) {
	handler := Security("/v1")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, path := range []string{"/health", "/v1/api-docs", "/v1/api-docs-lookalike", "/v1/profile"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
			expected := map[string]string{
				"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
				"Referrer-Policy":         "no-referrer",
				"X-Content-Type-Options":  "nosniff",
				"X-Frame-Options":         "DENY",
			}
			for name, want := range expected {
				if got := response.Header().Get(name); got != want {
					t.Errorf("%s: expected %q, got %q", name, want, got)
				}
			}
		})
	}
}

func TestCachePolicy(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{method: http.MethodGet, path: "/health", want: "no-store"},
		{method: http.MethodGet, path: "/v1/profile", want: "no-store"},
		{method: http.MethodGet, path: "/v1/api-docs", want: "no-cache"},
		{method: http.MethodGet, path: "/v1/openapi.json", want: "no-cache"},
		{method: http.MethodGet, path: "/v1/openapi-3.0.json", want: "no-cache"},
		{method: http.MethodGet, path: "/v1/schemas/ErrorModel.json", want: "no-cache"},
		{method: http.MethodGet, path: "/v1/items", want: "no-cache"},
		{method: http.MethodGet, path: "/v1/items-lookalike", want: "no-store"},
		{method: http.MethodPost, path: "/v1/hello", want: "no-store"},
		{method: http.MethodGet, path: "/api/items", want: "no-store"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), test.method, test.path, nil)
			if got := cachePolicy(request, "/v1"); got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestCachePolicyUsesConfiguredPrefix(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/items", nil)
	if got := cachePolicy(request, "/api"); got != "no-cache" {
		t.Fatalf("expected no-cache, got %q", got)
	}
}
