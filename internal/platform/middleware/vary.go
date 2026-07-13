package middleware

import (
	"net/http"
	"strings"
)

// AddVary adds canonical field names without duplicating existing values.
func AddVary(header http.Header, values ...string) {
	existing := make(map[string]struct{})
	for _, value := range header.Values("Vary") {
		for field := range strings.SplitSeq(value, ",") {
			existing[http.CanonicalHeaderKey(strings.TrimSpace(field))] = struct{}{}
		}
	}
	for _, value := range values {
		field := http.CanonicalHeaderKey(value)
		if field == "" {
			continue
		}
		if _, ok := existing[field]; !ok {
			header.Add("Vary", field)
			existing[field] = struct{}{}
		}
	}
}

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
			AddVary(w.Header(), "Accept")
			next.ServeHTTP(w, r)
		})
	}
}
