package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// RequestID returns middleware that injects a UUIDv4 request identifier.
// If the incoming request already provides the header (default X-Request-Id),
// that value is reused.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get(middleware.RequestIDHeader)
			if reqID == "" {
				reqID = uuid.NewString()
			}

			r = r.WithContext(context.WithValue(r.Context(), middleware.RequestIDKey, reqID))
			w.Header().Set(middleware.RequestIDHeader, reqID)
			next.ServeHTTP(w, r)
		})
	}
}
