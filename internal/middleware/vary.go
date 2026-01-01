package middleware

import "net/http"

// Vary returns middleware that adds Accept to the Vary header on all responses.
// Per RFC 9110 Section 12.5.5, the Vary header lists request headers
// that influence response selection. This API uses:
//   - Accept: Content negotiation selects JSON or CBOR format
//
// Note: The CORS middleware separately adds "Origin" to Vary, so this
// middleware only adds "Accept" to avoid duplication.
func Vary() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Accept")
			next.ServeHTTP(w, r)
		})
	}
}
