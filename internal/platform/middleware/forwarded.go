package middleware

import "net/http"

var untrustedForwardingHeaders = []string{
	"Forwarded",
	"X-Forwarded-For",
	"X-Forwarded-Host",
	"X-Forwarded-Proto",
	"X-Real-IP",
}

// IgnoreForwardedHeaders enforces the absence of a trusted-proxy boundary.
func IgnoreForwardedHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, header := range untrustedForwardingHeaders {
				r.Header.Del(header)
			}
			next.ServeHTTP(w, r)
		})
	}
}
