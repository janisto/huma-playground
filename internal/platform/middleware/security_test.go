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
	for _, path := range []string{"/health", "/openapi.json", "/v1/items", "/v1/profile", "/missing"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
			expected := map[string]string{
				"Cache-Control":                "no-store",
				"Content-Security-Policy":      "default-src 'none'; frame-ancestors 'none'",
				"Cross-Origin-Opener-Policy":   "same-origin",
				"Cross-Origin-Resource-Policy": "same-origin",
				"Permissions-Policy":           "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
				"Referrer-Policy":              "strict-origin-when-cross-origin",
				"X-Content-Type-Options":       "nosniff",
				"X-Frame-Options":              "DENY",
			}
			for name, want := range expected {
				if got := response.Header().Get(name); got != want {
					t.Errorf("%s: expected %q, got %q", name, want, got)
				}
			}
		})
	}
}
