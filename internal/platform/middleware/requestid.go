package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

const (
	// maxRequestIDLength limits request ID size to prevent unbounded memory usage.
	maxRequestIDLength = 128
)

// isValidRequestID validates a request ID for safe logging.
// Only allows printable ASCII characters (0x20-0x7E) excluding control characters,
// newlines, and other problematic characters that could enable log injection.
func isValidRequestID(id string) bool {
	if len(id) == 0 || len(id) > maxRequestIDLength {
		return false
	}
	for i := range len(id) {
		c := id[i]
		// Allow printable ASCII: space (0x20) through tilde (0x7E)
		// This excludes control characters (0x00-0x1F, 0x7F) and high bytes (0x80-0xFF)
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return true
}

// RequestID returns middleware that injects a UUIDv4 request identifier.
// If the incoming request provides a valid X-Request-Id header, that value is reused.
// Invalid request IDs (too long, empty, or containing non-printable characters)
// are rejected and a new UUID is generated instead.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get(middleware.RequestIDHeader)
			if !isValidRequestID(reqID) {
				reqID = uuid.NewString()
			}

			r = r.WithContext(context.WithValue(r.Context(), middleware.RequestIDKey, reqID))
			w.Header().Set(middleware.RequestIDHeader, reqID)
			next.ServeHTTP(w, r)
		})
	}
}
