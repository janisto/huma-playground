package respond

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	appmiddleware "github.com/janisto/huma-playground/internal/middleware"
)

// problemWithSchema wraps huma.ErrorModel to include the $schema field.
// This ensures Chi-level error responses (404, 405, panic recovery) include
// the JSON Schema reference, matching Huma's internal error responses.
type problemWithSchema struct {
	Schema   string              `json:"$schema,omitempty"`
	Type     string              `json:"type,omitempty"`
	Title    string              `json:"title,omitempty"`
	Status   int                 `json:"status,omitempty"`
	Detail   string              `json:"detail,omitempty"`
	Instance string              `json:"instance,omitempty"`
	Errors   []*huma.ErrorDetail `json:"errors,omitempty"`
}

// mediaRange represents a parsed Accept header media range with quality value.
type mediaRange struct {
	typ     string  // e.g., "application", "*"
	subtype string  // e.g., "cbor", "json", "*"
	q       float64 // quality value 0.0-1.0, default 1.0
}

// parseAccept parses an Accept header value into media ranges per RFC 9110.
// Returns ranges sorted by precedence (most specific first, then by q-value).
func parseAccept(header string) []mediaRange {
	if header == "" {
		return nil
	}

	var ranges []mediaRange
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		mr := mediaRange{q: 1.0}
		mediaType := part
		if idx := strings.Index(part, ";"); idx != -1 {
			mediaType = strings.TrimSpace(part[:idx])
			params := part[idx+1:]
			for param := range strings.SplitSeq(params, ";") {
				param = strings.TrimSpace(param)
				if strings.HasPrefix(strings.ToLower(param), "q=") {
					if qval, err := strconv.ParseFloat(param[2:], 64); err == nil && qval >= 0 && qval <= 1 {
						mr.q = qval
					}
				}
			}
		}

		if slash := strings.Index(mediaType, "/"); slash != -1 {
			mr.typ = strings.ToLower(strings.TrimSpace(mediaType[:slash]))
			mr.subtype = strings.ToLower(strings.TrimSpace(mediaType[slash+1:]))
		} else {
			mr.typ = strings.ToLower(strings.TrimSpace(mediaType))
			mr.subtype = "*"
		}
		ranges = append(ranges, mr)
	}
	return ranges
}

// selectFormat determines the preferred response format based on Accept header.
// Returns true for CBOR, false for JSON (default).
// Implements RFC 9110 content negotiation with q-values and precedence.
// Supports RFC 9457 Problem Details media types (application/problem+cbor, application/problem+json)
// as well as base types (application/cbor, application/json) and structured suffix types.
//
// Per RFC 9110 Section 12.5.1, specificity determines which q-value applies to a given
// representation, and q-value is the primary ranking factor for choosing between formats.
// Specificity is used only as a tie-breaker when q-values are equal.
func selectFormat(header string) bool {
	ranges := parseAccept(header)
	if len(ranges) == 0 {
		return false // default to JSON
	}

	var cborQ, jsonQ float64 = -1, -1
	cborSpecificity, jsonSpecificity := 0, 0

	for _, mr := range ranges {
		if mr.q == 0 {
			continue // q=0 means "not acceptable"
		}

		specificity := 0
		matchesCBOR, matchesJSON := false, false

		switch {
		// Exact Problem Details types (highest specificity)
		case mr.typ == "application" && mr.subtype == "problem+cbor":
			matchesCBOR = true
			specificity = 4
		case mr.typ == "application" && mr.subtype == "problem+json":
			matchesJSON = true
			specificity = 4
		// Base types
		case mr.typ == "application" && mr.subtype == "cbor":
			matchesCBOR = true
			specificity = 3
		case mr.typ == "application" && mr.subtype == "json":
			matchesJSON = true
			specificity = 3
		// Structured suffix wildcards (e.g., application/*+cbor, application/*+json)
		case mr.typ == "application" && strings.HasSuffix(mr.subtype, "+cbor"):
			matchesCBOR = true
			specificity = 3
		case mr.typ == "application" && strings.HasSuffix(mr.subtype, "+json"):
			matchesJSON = true
			specificity = 3
		case mr.typ == "application" && mr.subtype == "*":
			matchesCBOR = true
			matchesJSON = true
			specificity = 2
		case mr.typ == "*" && mr.subtype == "*":
			matchesCBOR = true
			matchesJSON = true
			specificity = 1
		}

		// Specificity determines which q-value applies: more specific matches override
		// less specific ones. Equal specificity uses the higher q-value.
		if matchesCBOR && (specificity > cborSpecificity || (specificity == cborSpecificity && mr.q > cborQ)) {
			cborQ = mr.q
			cborSpecificity = specificity
		}
		if matchesJSON && (specificity > jsonSpecificity || (specificity == jsonSpecificity && mr.q > jsonQ)) {
			jsonQ = mr.q
			jsonSpecificity = specificity
		}
	}

	// If neither matched explicitly with q>0, default to JSON
	if cborQ <= 0 && jsonQ <= 0 {
		return false
	}

	// Per RFC 9110: q-value is the primary ranking factor. Higher q wins.
	// Specificity is used only as a tie-breaker when q-values are equal.
	// When both q-values and specificities are equal, prefer JSON (stable default).
	if cborQ > jsonQ {
		return true
	}
	if jsonQ > cborQ {
		return false
	}
	// Equal q-values: use specificity as tie-breaker, prefer JSON if still tied
	if cborSpecificity > jsonSpecificity {
		return true
	}
	return false
}

// ensureVary adds values to the Vary header without duplicating existing entries.
// Per RFC 9110 Section 12.5.5, Vary is a comma-separated list of field names.
// This function merges new values with any existing Vary header entries.
func ensureVary(h http.Header, values ...string) {
	existing := make(map[string]struct{})
	for _, v := range h.Values("Vary") {
		for part := range strings.SplitSeq(v, ",") {
			existing[strings.TrimSpace(part)] = struct{}{}
		}
	}
	for _, v := range values {
		if _, ok := existing[v]; !ok {
			h.Add("Vary", v)
			existing[v] = struct{}{}
		}
	}
}

// writeProblem writes a Problem Details response honoring content negotiation per RFC 9110.
// Uses application/problem+json (RFC 9457) by default. When CBOR is preferred via Accept header,
// uses application/problem+cbor which follows the structured suffix convention (RFC 6839) but
// is not an IANA-registered media type. Clients requiring strict RFC 9457 compliance should
// request application/problem+json explicitly.
//
// Includes $schema field and Link header to match Huma's internal error responses.
func writeProblem(w http.ResponseWriter, r *http.Request, problem huma.ErrorModel) {
	// Build schema URL from request host
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	schemaURL := fmt.Sprintf("%s://%s/schemas/ErrorModel.json", scheme, r.Host)

	// Wrap with $schema field
	resp := problemWithSchema{
		Schema:   schemaURL,
		Type:     problem.Type,
		Title:    problem.Title,
		Status:   problem.Status,
		Detail:   problem.Detail,
		Instance: problem.Instance,
		Errors:   problem.Errors,
	}

	// Ensure Vary header includes Origin and Accept for content negotiation.
	// Uses Add() to merge with any Vary values set by middleware (CORS, compression, etc.)
	// rather than Set() which would discard existing values.
	ensureVary(w.Header(), "Origin", "Accept")

	// Add Link header for schema discovery (matches Huma's behavior)
	w.Header().Add("Link", "</schemas/ErrorModel.json>; rel=\"describedBy\"")

	if selectFormat(r.Header.Get("Accept")) {
		w.Header().Set("Content-Type", "application/problem+cbor")
		w.WriteHeader(resp.Status)
		_ = cbor.NewEncoder(w).Encode(resp)
	} else {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(resp.Status)
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(resp)
	}
}

// responseWriter wraps http.ResponseWriter to track if headers have been written.
type responseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.wroteHeader = true
	return rw.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter for middleware compatibility.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Recoverer returns middleware that recovers from panics with Problem Details.
// Re-panics on http.ErrAbortHandler to preserve net/http abort semantics.
func Recoverer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &responseWriter{ResponseWriter: w}
			defer func() {
				if rec := recover(); rec != nil {
					// Re-panic on http.ErrAbortHandler to preserve abort semantics.
					// This sentinel error signals the server to abort the response
					// without logging or writing an error response.
					if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
						panic(rec)
					}

					stack := debug.Stack()
					appmiddleware.LogError(r.Context(), "panic recovered",
						fmt.Errorf("%v", rec),
						zap.ByteString("stack", stack),
					)

					// If a response was already started (e.g., Huma wrote a 406),
					// we cannot write another response. This can happen when Huma
					// panics after writing an error response to signal middleware.
					if rw.wroteHeader {
						return
					}

					problem := huma.ErrorModel{
						Title:  http.StatusText(http.StatusInternalServerError),
						Status: http.StatusInternalServerError,
						Detail: "internal server error",
					}
					writeProblem(w, r, problem)
				}
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

// NotFoundHandler returns a handler for 404 responses with Problem Details.
func NotFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		problem := huma.ErrorModel{
			Title:  http.StatusText(http.StatusNotFound),
			Status: http.StatusNotFound,
			Detail: "resource not found",
		}
		writeProblem(w, r, problem)
	}
}

// MethodNotAllowedHandler returns a handler for 405 responses with Problem Details.
func MethodNotAllowedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if allow := allowedMethods(r); len(allow) > 0 {
			w.Header().Set("Allow", strings.Join(allow, ", "))
		}
		problem := huma.ErrorModel{
			Title:  http.StatusText(http.StatusMethodNotAllowed),
			Status: http.StatusMethodNotAllowed,
			Detail: fmt.Sprintf("method %s not allowed", r.Method),
		}
		writeProblem(w, r, problem)
	}
}

// WriteRedirect sends a redirect response.
func WriteRedirect(w http.ResponseWriter, r *http.Request, url string, code int) {
	http.Redirect(w, r, url, code)
}

// Status304NotModified returns a 304 response without writing a body.
func Status304NotModified() huma.StatusError {
	return &noBodyStatusError{status: http.StatusNotModified, message: http.StatusText(http.StatusNotModified)}
}

// noBodyStatusError is a StatusError implementation that signals consumers to
// send only the status code. Huma skips marshaling bodies for 204/304, so
// returning this avoids emitting JSON that would violate the RFC.
type noBodyStatusError struct {
	status  int
	message string
}

func (e *noBodyStatusError) Error() string {
	if strings.TrimSpace(e.message) != "" {
		return e.message
	}
	return http.StatusText(e.status)
}

func (e *noBodyStatusError) GetStatus() int {
	return e.status
}

// allowedMethods inspects chi's routing context to discover allowed methods.
func allowedMethods(r *http.Request) []string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil || rctx.Routes == nil {
		return nil
	}

	routePath := rctx.RoutePath
	if routePath == "" {
		if r.URL.RawPath != "" {
			routePath = r.URL.RawPath
		} else {
			routePath = r.URL.Path
		}
		if routePath == "" {
			routePath = "/"
		}
	}

	methods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	}
	allowed := make([]string, 0, len(methods))
	seen := make(map[string]struct{})
	for _, method := range methods {
		tctx := chi.NewRouteContext()
		if rctx.Routes.Match(tctx, method, routePath) {
			if _, ok := seen[method]; !ok {
				allowed = append(allowed, method)
				seen[method] = struct{}{}
			}
		}
	}
	return allowed
}
