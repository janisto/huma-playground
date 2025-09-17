package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS returns a middleware that applies permissive defaults suitable for APIs.
// The configuration mirrors chi's recommended settings and can be adjusted as
// requirements evolve.
func CORS() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-CSRF-Token",
		},
		ExposedHeaders: []string{"Link"},
		MaxAge:         300,
	})
}
