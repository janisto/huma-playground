package middleware

import (
	"net/http"
)

// Security sets browser and route-aware cache policy headers for every response.
func Security(apiPrefix string) func(http.Handler) http.Handler {
	_ = apiPrefix
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers := w.Header()
			headers.Set("Cache-Control", "no-store")
			headers.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			headers.Set("Cross-Origin-Opener-Policy", "same-origin")
			headers.Set("Cross-Origin-Resource-Policy", "same-origin")
			headers.Set(
				"Permissions-Policy",
				"accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
			)
			headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			headers.Set("X-Content-Type-Options", "nosniff")
			headers.Set("X-Frame-Options", "DENY")
			next.ServeHTTP(w, r)
		})
	}
}
