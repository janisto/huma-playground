package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS applies the configured browser origins without allowing credentials.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-Request-ID",
			"traceparent",
			"tracestate",
		},
		ExposedHeaders: []string{
			"Link",
			"Location",
			"Retry-After",
			"X-RateLimit-Reset",
			"X-Request-ID",
		},
		MaxAge: 300,
	})
}
