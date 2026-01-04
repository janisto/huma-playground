package middleware

import (
	"net/http"
	"strings"
)

// Security returns middleware that sets security headers on all responses.
// Headers follow OWASP REST Security Cheat Sheet recommendations (2025).
//
// Paths in skipPaths are excluded from security headers (e.g., "/api-docs").
//
// Headers set:
//   - Cache-Control: no-store - Prevents caching of API responses
//   - Content-Security-Policy: frame-ancestors 'none' - Prevents framing (CSP Level 2)
//   - Cross-Origin-Opener-Policy: same-origin - Isolates browsing context (Specter mitigation)
//   - Cross-Origin-Resource-Policy: same-origin - Prevents cross-origin reads (Specter mitigation)
//   - Permissions-Policy: disables browser features not needed by REST APIs
//   - Referrer-Policy: strict-origin-when-cross-origin - Controls referrer information leakage
//   - X-Content-Type-Options: nosniff - Prevents MIME-sniffing attacks
//   - X-Frame-Options: DENY - Prevents clickjacking (legacy browser support)
func Security(skipPaths ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, p := range skipPaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}
			h := w.Header()
			h.Set("Cache-Control", "no-store")
			h.Set("Content-Security-Policy", "frame-ancestors 'none'")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			h.Set(
				"Permissions-Policy",
				"accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
			)
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			next.ServeHTTP(w, r)
		})
	}
}
