package middleware

import (
	"net/http"
	"strings"
)

// Security sets browser and route-aware cache policy headers for every response.
func Security(apiPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers := w.Header()
			headers.Set("Cache-Control", cachePolicy(r, apiPrefix))
			headers.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			headers.Set("Cross-Origin-Opener-Policy", "same-origin")
			headers.Set("Cross-Origin-Resource-Policy", "same-origin")
			headers.Set(
				"Permissions-Policy",
				"accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
			)
			headers.Set("Referrer-Policy", "no-referrer")
			headers.Set("X-Content-Type-Options", "nosniff")
			headers.Set("X-Frame-Options", "DENY")
			next.ServeHTTP(w, r)
		})
	}
}

func cachePolicy(r *http.Request, apiPrefix string) string {
	path := r.URL.Path
	if path == apiPrefix+"/api-docs" ||
		path == apiPrefix+"/openapi.json" ||
		path == apiPrefix+"/openapi.yaml" ||
		path == apiPrefix+"/openapi-3.0.json" ||
		path == apiPrefix+"/openapi-3.0.yaml" ||
		strings.HasPrefix(path, apiPrefix+"/schemas/") {
		return "no-cache"
	}
	if r.Method == http.MethodGet &&
		(path == apiPrefix+"/hello" ||
			path == apiPrefix+"/items" ||
			strings.HasPrefix(path, apiPrefix+"/github/")) {
		return "no-cache"
	}
	return "no-store"
}
