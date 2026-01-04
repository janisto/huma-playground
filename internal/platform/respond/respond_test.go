package respond

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
)

// testProblem is used for testing to capture $schema field.
type testProblem struct {
	Schema string              `json:"$schema,omitempty"`
	Title  string              `json:"title,omitempty"`
	Status int                 `json:"status,omitempty"`
	Detail string              `json:"detail,omitempty"`
	Errors []*huma.ErrorDetail `json:"errors,omitempty"`
}

func TestNotFoundHandlerReturnsProblemDetails(t *testing.T) {
	router := chi.NewRouter()
	router.NotFound(NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %q", ct)
	}

	// Verify Link header for schema discovery
	link := resp.Header().Get("Link")
	if !strings.Contains(link, "/schemas/ErrorModel.json") || !strings.Contains(link, "describedBy") {
		t.Fatalf("expected Link header with schema, got %q", link)
	}

	var problem testProblem
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal problem: %v", err)
	}
	if problem.Status != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", problem.Status)
	}
	if problem.Title != "Not Found" {
		t.Fatalf("unexpected title: %s", problem.Title)
	}
	if problem.Detail != "resource not found" {
		t.Fatalf("unexpected detail: %s", problem.Detail)
	}
	// Verify $schema field is present
	if !strings.Contains(problem.Schema, "/schemas/ErrorModel.json") {
		t.Fatalf("expected $schema field, got %q", problem.Schema)
	}
}

func TestMethodNotAllowedHandlerReturnsProblemDetails(t *testing.T) {
	router := chi.NewRouter()
	router.MethodNotAllowed(MethodNotAllowedHandler())
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %q", ct)
	}
	if allow := resp.Header().Get("Allow"); !strings.Contains(allow, http.MethodGet) {
		t.Fatalf("expected Allow header to list GET, got %q", allow)
	}

	// Verify Link header for schema discovery
	link := resp.Header().Get("Link")
	if !strings.Contains(link, "/schemas/ErrorModel.json") || !strings.Contains(link, "describedBy") {
		t.Fatalf("expected Link header with schema, got %q", link)
	}

	var problem testProblem
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal problem: %v", err)
	}
	if problem.Status != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: %d", problem.Status)
	}
	if problem.Title != "Method Not Allowed" {
		t.Fatalf("unexpected title: %s", problem.Title)
	}
	if !strings.Contains(problem.Detail, "POST") {
		t.Fatalf("expected detail to mention POST, got %s", problem.Detail)
	}
	// Verify $schema field is present
	if !strings.Contains(problem.Schema, "/schemas/ErrorModel.json") {
		t.Fatalf("expected $schema field, got %q", problem.Schema)
	}
}

func TestRecovererReturnsProblemDetails(t *testing.T) {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		Recoverer(),
	)
	api := humachi.New(router, huma.DefaultConfig("Test", "test"))
	huma.Get(api, "/panic", func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %q", ct)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal problem: %v", err)
	}
	if problem.Status != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", problem.Status)
	}
	if problem.Title != "Internal Server Error" {
		t.Fatalf("unexpected title: %s", problem.Title)
	}
	if problem.Detail != "internal server error" {
		t.Fatalf("unexpected detail: %s", problem.Detail)
	}
}

func TestRecovererRePanicsOnErrAbortHandler(t *testing.T) {
	router := chi.NewRouter()
	router.Use(Recoverer())
	router.Get("/abort", func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})

	defer func() {
		rec := recover()
		err, ok := rec.(error)
		if !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("expected http.ErrAbortHandler to be re-panicked, got %v", rec)
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	t.Fatal("expected panic to propagate, but handler returned normally")
}

func TestWriteRedirect(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteRedirect(w, r, "/destination", http.StatusMovedPermanently)
	})

	req := httptest.NewRequest(http.MethodGet, "/redirect", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", resp.Code)
	}
	if loc := resp.Header().Get("Location"); loc != "/destination" {
		t.Fatalf("expected location /destination, got %q", loc)
	}
}

func TestStatus304NotModifiedHasNoBody(t *testing.T) {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
	)
	api := humachi.New(router, huma.DefaultConfig("NoBody", "test"))
	huma.Get(api, "/etag", func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		return nil, Status304NotModified()
	})

	req := httptest.NewRequest(http.MethodGet, "/etag", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-304-req")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", resp.Code)
	}
	if resp.Body.Len() != 0 {
		t.Fatalf("expected empty body for 304 response, got %q", resp.Body.String())
	}
}

func TestNoBodyStatusErrorMethods(t *testing.T) {
	err := &noBodyStatusError{status: http.StatusNotModified, message: "Not Modified"}
	if err.GetStatus() != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", err.GetStatus())
	}
	if err.Error() != "Not Modified" {
		t.Fatalf("expected 'Not Modified', got %q", err.Error())
	}

	errEmpty := &noBodyStatusError{status: http.StatusNoContent, message: ""}
	if errEmpty.Error() != "No Content" {
		t.Fatalf("expected 'No Content' from status text, got %q", errEmpty.Error())
	}
}

func TestResponseWriterMethods(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	if rw.wroteHeader {
		t.Fatal("expected wroteHeader to be false initially")
	}

	rw.WriteHeader(http.StatusCreated)
	if !rw.wroteHeader {
		t.Fatal("expected wroteHeader to be true after WriteHeader")
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	rw2 := &responseWriter{ResponseWriter: rec2}
	n, err := rw2.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 bytes, got %d", n)
	}
	if !rw2.wroteHeader {
		t.Fatal("expected wroteHeader to be true after Write")
	}

	underlying := rw2.Unwrap()
	if underlying != rec2 {
		t.Fatal("expected Unwrap to return underlying ResponseWriter")
	}
}

func TestRecovererSkipsWriteWhenHeaderAlreadyWritten(t *testing.T) {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		Recoverer(),
	)
	router.Get("/partial", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial response"))
		panic("panic after write")
	})

	req := httptest.NewRequest(http.MethodGet, "/partial", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected original 200 status to be preserved, got %d", resp.Code)
	}
	body := resp.Body.String()
	if body != "partial response" {
		t.Fatalf("expected original body to be preserved, got %q", body)
	}
}

func TestNotFoundHandlerReturnsCBORWhenAccepted(t *testing.T) {
	router := chi.NewRouter()
	router.NotFound(NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set("Accept", "application/cbor")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Fatalf("expected application/problem+cbor, got %q", ct)
	}

	var problem huma.ErrorModel
	if err := cbor.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal CBOR problem: %v", err)
	}
	if problem.Status != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", problem.Status)
	}
	if problem.Title != "Not Found" {
		t.Fatalf("unexpected title: %s", problem.Title)
	}
}

func TestMethodNotAllowedHandlerReturnsCBORWhenAccepted(t *testing.T) {
	router := chi.NewRouter()
	router.MethodNotAllowed(MethodNotAllowedHandler())
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Accept", "application/cbor")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Fatalf("expected application/problem+cbor, got %q", ct)
	}

	var problem huma.ErrorModel
	if err := cbor.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal CBOR problem: %v", err)
	}
	if problem.Status != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: %d", problem.Status)
	}
	if problem.Title != "Method Not Allowed" {
		t.Fatalf("unexpected title: %s", problem.Title)
	}
}

func TestRecovererReturnsCBORWhenAccepted(t *testing.T) {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		Recoverer(),
	)
	api := humachi.New(router, huma.DefaultConfig("Test", "test"))
	huma.Get(api, "/panic", func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("Accept", "application/cbor")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Fatalf("expected application/problem+cbor, got %q", ct)
	}

	var problem huma.ErrorModel
	if err := cbor.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal CBOR problem: %v", err)
	}
	if problem.Status != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", problem.Status)
	}
	if problem.Title != "Internal Server Error" {
		t.Fatalf("unexpected title: %s", problem.Title)
	}
}

func TestAcceptsCBOREdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		accept      string
		expectCBOR  bool
		description string
	}{
		{
			name:        "empty accept defaults to JSON",
			accept:      "",
			expectCBOR:  false,
			description: "no Accept header should default to JSON",
		},
		{
			name:        "wildcard defaults to JSON",
			accept:      "*/*",
			expectCBOR:  false,
			description: "wildcard should default to JSON",
		},
		{
			name:        "application wildcard defaults to JSON",
			accept:      "application/*",
			expectCBOR:  false,
			description: "application/* should default to JSON",
		},
		{
			name:        "explicit JSON",
			accept:      "application/json",
			expectCBOR:  false,
			description: "explicit JSON should return JSON",
		},
		{
			name:        "explicit CBOR",
			accept:      "application/cbor",
			expectCBOR:  true,
			description: "explicit CBOR should return CBOR",
		},
		{
			name:        "CBOR with quality parameter",
			accept:      "application/cbor;q=1.0",
			expectCBOR:  true,
			description: "CBOR with q param should return CBOR",
		},
		{
			name:        "multiple types with equal q-values defaults to JSON",
			accept:      "application/json, application/cbor",
			expectCBOR:  false,
			description: "equal q-values should default to JSON per RFC 9110",
		},
		{
			name:        "CBOR preferred with quality",
			accept:      "application/json;q=0.9, application/cbor;q=1.0",
			expectCBOR:  true,
			description: "CBOR with higher q should return CBOR",
		},
		{
			name:        "text/html defaults to JSON",
			accept:      "text/html",
			expectCBOR:  false,
			description: "unsupported type should default to JSON",
		},
		{
			name:        "problem+cbor explicit",
			accept:      "application/problem+cbor",
			expectCBOR:  true,
			description: "RFC 9457 problem+cbor should return CBOR",
		},
		{
			name:        "problem+json explicit",
			accept:      "application/problem+json",
			expectCBOR:  false,
			description: "RFC 9457 problem+json should return JSON",
		},
		{
			name:        "problem+cbor preferred over problem+json",
			accept:      "application/problem+cbor;q=1.0, application/problem+json;q=0.5",
			expectCBOR:  true,
			description: "higher q for problem+cbor should return CBOR",
		},
		{
			name:        "problem+json preferred over problem+cbor",
			accept:      "application/problem+cbor;q=0.5, application/problem+json;q=1.0",
			expectCBOR:  false,
			description: "higher q for problem+json should return JSON",
		},
		{
			name:        "problem+cbor over base cbor same q",
			accept:      "application/cbor, application/problem+cbor",
			expectCBOR:  true,
			description: "problem+cbor has higher specificity than base cbor",
		},
		{
			name:        "CBOR excluded with q=0",
			accept:      "application/cbor;q=0, application/json",
			expectCBOR:  false,
			description: "q=0 means not acceptable per RFC 9110",
		},
		{
			name:        "JSON preferred with higher quality",
			accept:      "application/cbor;q=0.5, application/json;q=0.9",
			expectCBOR:  false,
			description: "JSON with higher q should return JSON",
		},
		{
			name:        "CBOR only with low quality still accepted",
			accept:      "application/cbor;q=0.1",
			expectCBOR:  true,
			description: "any q > 0 should accept CBOR",
		},
		{
			name:        "wildcard with CBOR explicit prefers CBOR",
			accept:      "*/*;q=0.1, application/cbor;q=1.0",
			expectCBOR:  true,
			description: "explicit CBOR over wildcard per specificity",
		},
		{
			name:        "wildcard with JSON explicit prefers JSON",
			accept:      "*/*;q=0.1, application/json;q=1.0",
			expectCBOR:  false,
			description: "explicit JSON over wildcard per specificity",
		},
		{
			name:        "q-value wins over specificity - JSON base over CBOR problem",
			accept:      "application/problem+cbor;q=0.1, application/json;q=1.0",
			expectCBOR:  false,
			description: "RFC 9110: q-value is primary ranking factor, specificity is tie-breaker",
		},
		{
			name:        "q-value wins over specificity - CBOR base over JSON problem",
			accept:      "application/problem+json;q=0.1, application/cbor;q=1.0",
			expectCBOR:  true,
			description: "RFC 9110: q-value is primary ranking factor, specificity is tie-breaker",
		},
		{
			name:        "equal q-values use specificity as tie-breaker - CBOR wins",
			accept:      "application/json;q=0.8, application/problem+cbor;q=0.8",
			expectCBOR:  true,
			description: "equal q-values should use specificity as tie-breaker",
		},
		{
			name:        "equal q-values use specificity as tie-breaker - JSON wins",
			accept:      "application/cbor;q=0.8, application/problem+json;q=0.8",
			expectCBOR:  false,
			description: "equal q-values should use specificity as tie-breaker",
		},
		{
			name:        "malformed quality defaults to 1.0",
			accept:      "application/cbor;q=invalid",
			expectCBOR:  true,
			description: "invalid q value should default to 1.0",
		},
		{
			name:        "whitespace handling",
			accept:      "  application/cbor  ;  q=1.0  ",
			expectCBOR:  true,
			description: "should handle whitespace around media type",
		},
		{
			name:        "case insensitive type matching",
			accept:      "Application/CBOR",
			expectCBOR:  true,
			description: "media types are case insensitive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := chi.NewRouter()
			router.NotFound(NotFoundHandler())

			req := httptest.NewRequest(http.MethodGet, "/missing", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d", resp.Code)
			}

			ct := resp.Header().Get("Content-Type")
			if tt.expectCBOR {
				if ct != "application/problem+cbor" {
					t.Fatalf("%s: expected application/problem+cbor, got %q", tt.description, ct)
				}
				var problem huma.ErrorModel
				if err := cbor.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
					t.Fatalf("failed to unmarshal CBOR: %v", err)
				}
			} else {
				if ct != "application/problem+json" {
					t.Fatalf("%s: expected application/problem+json, got %q", tt.description, ct)
				}
				var problem huma.ErrorModel
				if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
					t.Fatalf("failed to unmarshal JSON: %v", err)
				}
			}
		})
	}
}

func TestAllowedMethodsReturned(t *testing.T) {
	router := chi.NewRouter()
	router.MethodNotAllowed(MethodNotAllowedHandler())
	router.Get("/resource", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Post("/resource", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodDelete, "/resource", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}

	allow := resp.Header().Get("Allow")
	if !strings.Contains(allow, "GET") {
		t.Fatalf("expected Allow header to contain GET, got %q", allow)
	}
	if !strings.Contains(allow, "POST") {
		t.Fatalf("expected Allow header to contain POST, got %q", allow)
	}
}

func TestNotFoundHandlerWithEmptyPath(t *testing.T) {
	router := chi.NewRouter()
	router.NotFound(NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Detail != "resource not found" {
		t.Fatalf("unexpected detail: %s", problem.Detail)
	}
}

func TestRecovererWithErrorPanic(t *testing.T) {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		Recoverer(),
	)
	api := humachi.New(router, huma.DefaultConfig("Test", "test"))
	huma.Get(api, "/panic-error", func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		panic(errors.New("wrapped error"))
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-error", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Detail != "internal server error" {
		t.Fatalf("unexpected detail: %s", problem.Detail)
	}
}

func TestMethodNotAllowedWithRawPath(t *testing.T) {
	router := chi.NewRouter()
	router.MethodNotAllowed(MethodNotAllowedHandler())
	router.Get("/path%2Fencoded", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/path%2Fencoded", nil)
	req.URL.RawPath = "/path%2Fencoded"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal problem: %v", err)
	}
	if problem.Status != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: %d", problem.Status)
	}
}

func TestAllowedMethodsNilRouteContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	methods := allowedMethods(req)
	if methods != nil {
		t.Fatalf("expected nil for request without chi route context, got %v", methods)
	}
}

func TestVaryHeaderPresent(t *testing.T) {
	router := chi.NewRouter()
	router.NotFound(NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set("Accept", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Vary header may contain multiple values either comma-separated or as separate entries
	varyValues := resp.Header().Values("Vary")
	varySet := make(map[string]struct{})
	for _, v := range varyValues {
		for part := range strings.SplitSeq(v, ",") {
			varySet[strings.TrimSpace(part)] = struct{}{}
		}
	}
	if _, ok := varySet["Origin"]; !ok {
		t.Fatalf("expected Vary to contain Origin, got %v", varyValues)
	}
	if _, ok := varySet["Accept"]; !ok {
		t.Fatalf("expected Vary to contain Accept, got %v", varyValues)
	}
}

func TestVaryHeaderMergesWithExisting(t *testing.T) {
	// Simulate a middleware that sets Vary: Accept-Encoding before our handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		NotFoundHandler().ServeHTTP(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	varyValues := resp.Header().Values("Vary")
	varySet := make(map[string]struct{})
	for _, v := range varyValues {
		for part := range strings.SplitSeq(v, ",") {
			varySet[strings.TrimSpace(part)] = struct{}{}
		}
	}
	// Should contain all three: Accept-Encoding from middleware, plus Origin and Accept from writeProblem
	if _, ok := varySet["Accept-Encoding"]; !ok {
		t.Fatalf("expected Vary to preserve Accept-Encoding from middleware, got %v", varyValues)
	}
	if _, ok := varySet["Origin"]; !ok {
		t.Fatalf("expected Vary to contain Origin, got %v", varyValues)
	}
	if _, ok := varySet["Accept"]; !ok {
		t.Fatalf("expected Vary to contain Accept, got %v", varyValues)
	}
}

func TestVaryHeaderNoDuplicates(t *testing.T) {
	// Simulate a middleware that already sets Vary: Accept before our handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept")
		NotFoundHandler().ServeHTTP(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	varyValues := resp.Header().Values("Vary")
	acceptCount := 0
	for _, v := range varyValues {
		for part := range strings.SplitSeq(v, ",") {
			if strings.TrimSpace(part) == "Accept" {
				acceptCount++
			}
		}
	}
	if acceptCount != 1 {
		t.Fatalf("expected Accept to appear exactly once in Vary, got %d times in %v", acceptCount, varyValues)
	}
}

func TestJSONResponseHasNoHTMLEscaping(t *testing.T) {
	router := chi.NewRouter()
	router.NotFound(NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/path?foo=<bar>&baz=1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if strings.Contains(body, `\u003c`) || strings.Contains(body, `\u003e`) {
		t.Fatalf("response should not contain HTML-escaped characters: %s", body)
	}
}

func TestParseAcceptNoSlash(t *testing.T) {
	ranges := parseAccept("text")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].typ != "text" || ranges[0].subtype != "*" {
		t.Fatalf("expected text/*, got %s/%s", ranges[0].typ, ranges[0].subtype)
	}
}

func TestParseAcceptEmptyPart(t *testing.T) {
	ranges := parseAccept("application/json, , text/html")
	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges (empty part skipped), got %d", len(ranges))
	}
}

func TestParseAcceptInvalidQValue(t *testing.T) {
	ranges := parseAccept("application/json;q=invalid")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].q != 1.0 {
		t.Fatalf("expected q=1.0 for invalid q value, got %f", ranges[0].q)
	}
}

func TestParseAcceptQValueOutOfRange(t *testing.T) {
	ranges := parseAccept("application/json;q=2.0")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].q != 1.0 {
		t.Fatalf("expected q=1.0 for out-of-range q value, got %f", ranges[0].q)
	}

	ranges = parseAccept("application/json;q=-0.5")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].q != 1.0 {
		t.Fatalf("expected q=1.0 for negative q value, got %f", ranges[0].q)
	}
}

func TestSelectFormatStructuredSuffixWildcard(t *testing.T) {
	if selectFormat("application/*+cbor") != true {
		t.Fatal("expected CBOR for application/*+cbor")
	}

	if selectFormat("application/*+json") != false {
		t.Fatal("expected JSON for application/*+json")
	}
}

func TestSelectFormatNoMatchingType(t *testing.T) {
	if selectFormat("text/html") != false {
		t.Fatal("expected JSON as default when no matching type")
	}

	if selectFormat("image/png, text/plain") != false {
		t.Fatal("expected JSON as default for non-matching types")
	}
}

func TestAllowedMethodsEmptyRoutePath(t *testing.T) {
	router := chi.NewRouter()
	router.MethodNotAllowed(MethodNotAllowedHandler())
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.URL.Path = ""
	req.URL.RawPath = ""
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}
}

func TestSchemaURLUsesHTTPSWhenForwardedProtoSet(t *testing.T) {
	router := chi.NewRouter()
	router.NotFound(NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "example.com"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}

	var problem testProblem
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal problem: %v", err)
	}

	if !strings.HasPrefix(problem.Schema, "https://") {
		t.Fatalf("expected schema URL to use https, got %q", problem.Schema)
	}
	if !strings.Contains(problem.Schema, "example.com") {
		t.Fatalf("expected schema URL to include host, got %q", problem.Schema)
	}
}

func TestSchemaURLUsesHTTPSWhenTLSPresent(t *testing.T) {
	router := chi.NewRouter()
	router.NotFound(NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "https://secure.example.com/missing", nil)
	req.TLS = &tls.ConnectionState{}
	req.Host = "secure.example.com"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}

	var problem testProblem
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal problem: %v", err)
	}

	if !strings.HasPrefix(problem.Schema, "https://") {
		t.Fatalf("expected schema URL to use https when TLS present, got %q", problem.Schema)
	}
	if !strings.Contains(problem.Schema, "secure.example.com") {
		t.Fatalf("expected schema URL to include host, got %q", problem.Schema)
	}
}

func TestWriteRedirectVariousCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		location string
	}{
		{"301 moved permanently", http.StatusMovedPermanently, "/new-location"},
		{"302 found", http.StatusFound, "/temporary"},
		{"303 see other", http.StatusSeeOther, "/other"},
		{"307 temporary redirect", http.StatusTemporaryRedirect, "/temp"},
		{"308 permanent redirect", http.StatusPermanentRedirect, "/permanent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				WriteRedirect(w, r, tt.location, tt.code)
			})

			req := httptest.NewRequest(http.MethodGet, "/redirect", nil)
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)

			if resp.Code != tt.code {
				t.Fatalf("expected %d, got %d", tt.code, resp.Code)
			}
			if loc := resp.Header().Get("Location"); loc != tt.location {
				t.Fatalf("expected location %q, got %q", tt.location, loc)
			}
		})
	}
}

func TestEnsureVaryWithCommaSeparatedExisting(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding, Accept-Language")
		NotFoundHandler().ServeHTTP(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	varyValues := resp.Header().Values("Vary")
	varySet := make(map[string]struct{})
	for _, v := range varyValues {
		for part := range strings.SplitSeq(v, ",") {
			varySet[strings.TrimSpace(part)] = struct{}{}
		}
	}

	expected := []string{"Accept-Encoding", "Accept-Language", "Origin", "Accept"}
	for _, exp := range expected {
		if _, ok := varySet[exp]; !ok {
			t.Fatalf("expected Vary to contain %q, got %v", exp, varyValues)
		}
	}
}

func TestSelectFormatBothExcludedWithQ0(t *testing.T) {
	if selectFormat("application/json;q=0, application/cbor;q=0") != false {
		t.Fatal("expected JSON default when both formats excluded with q=0")
	}
}

func TestSelectFormatOnlyWildcardWithQ0(t *testing.T) {
	if selectFormat("*/*;q=0") != false {
		t.Fatal("expected JSON default when wildcard has q=0")
	}
}

func TestParseAcceptMultipleQParams(t *testing.T) {
	ranges := parseAccept("application/json;q=0.5;q=0.9")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].q != 0.9 {
		t.Fatalf("expected last q value (0.9) to be used, got %f", ranges[0].q)
	}
}

func TestResponseWriterUnwrapReturnsUnderlying(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	unwrapped := rw.Unwrap()
	if unwrapped != rec {
		t.Fatal("Unwrap should return the underlying ResponseWriter")
	}

	rec.WriteHeader(http.StatusAccepted)
	if rec.Code != http.StatusAccepted {
		t.Fatal("underlying ResponseWriter should be writable after Unwrap")
	}
}

func TestMethodNotAllowedWithMultipleMethods(t *testing.T) {
	router := chi.NewRouter()
	router.MethodNotAllowed(MethodNotAllowedHandler())
	router.Get("/multi", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Post("/multi", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	router.Put("/multi", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Delete("/multi", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPatch, "/multi", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}

	allow := resp.Header().Get("Allow")
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		if !strings.Contains(allow, method) {
			t.Fatalf("expected Allow header to contain %s, got %q", method, allow)
		}
	}
}

func TestRecovererWithNonErrorPanic(t *testing.T) {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		Recoverer(),
	)
	router.Get("/panic-int", func(_ http.ResponseWriter, _ *http.Request) {
		panic(42)
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-int", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Detail != "internal server error" {
		t.Fatalf("unexpected detail: %s", problem.Detail)
	}
}

func TestSelectFormatCBORExplicitlyExcludedJSONAccepted(t *testing.T) {
	if selectFormat("application/cbor;q=0, application/json;q=1.0") != false {
		t.Fatal("expected JSON when CBOR is excluded with q=0")
	}
}

func TestSelectFormatJSONExplicitlyExcludedCBORAccepted(t *testing.T) {
	if selectFormat("application/json;q=0, application/cbor;q=1.0") != true {
		t.Fatal("expected CBOR when JSON is excluded with q=0")
	}
}

func TestEnsureVaryEmptyValuesInput(t *testing.T) {
	h := make(http.Header)
	ensureVary(h)

	if len(h.Values("Vary")) != 0 {
		t.Fatalf("expected no Vary header when no values provided, got %v", h.Values("Vary"))
	}
}

func TestEnsureVaryDuplicateInSingleCall(t *testing.T) {
	h := make(http.Header)
	ensureVary(h, "Accept", "Accept", "Origin")

	varyValues := h.Values("Vary")
	acceptCount := 0
	for _, v := range varyValues {
		for part := range strings.SplitSeq(v, ",") {
			if strings.TrimSpace(part) == "Accept" {
				acceptCount++
			}
		}
	}
	if acceptCount != 1 {
		t.Fatalf("expected Accept once, got %d times in %v", acceptCount, varyValues)
	}
}
