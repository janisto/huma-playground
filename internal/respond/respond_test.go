package respond

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	apiinternal "github.com/janisto/huma-playground/internal/api"
	appmiddleware "github.com/janisto/huma-playground/internal/middleware"
)

func TestStatusErrorUsesEnvelope(t *testing.T) {
	Install()

	err := huma.NewError(http.StatusBadRequest, "bad request", errors.New("missing field"))
	env, ok := err.(*statusEnvelopeError)
	if !ok {
		t.Fatalf("expected statusEnvelopeError, got %T", err)
	}

	if env.status != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", env.status)
	}
	if env.Envelope.Error == nil {
		t.Fatalf("expected error body to be set")
	}
	if env.Envelope.Error.Code == "" {
		t.Fatalf("expected code to be populated")
	}
	if env.Envelope.Error.Message != "bad request" {
		t.Fatalf("unexpected message: %s", env.Envelope.Error.Message)
	}
	if len(env.Envelope.Error.Details) != 1 || env.Envelope.Error.Details[0].Issue != "missing field" {
		t.Fatalf("unexpected details: %+v", env.Envelope.Error.Details)
	}
}

func TestWriteRedirectProducesEnvelope(t *testing.T) {
	Install()

	rec := httptest.NewRecorder()
	location := "/dest"
	if err := WriteRedirect(rec, context.Background(), http.StatusFound, location, ""); err != nil {
		t.Fatalf("write redirect failed: %v", err)
	}

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != location {
		t.Fatalf("expected location %q, got %q", location, got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("expected application/json content type, got %q", ct)
	}

	var env apiinternal.Envelope[struct{}]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}
	if env.Error == nil {
		t.Fatalf("expected error body")
	}
	if env.Error.Code != codeRedirect {
		t.Fatalf("unexpected code: %s", env.Error.Code)
	}
	if env.Error.Message == "" {
		t.Fatalf("expected message to be populated")
	}
}

func TestHandlersEmitEnvelopes(t *testing.T) {
	Install()

	router := chi.NewRouter()
	router.NotFound(NotFoundHandler())
	router.MethodNotAllowed(MethodNotAllowedHandler())
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		chimiddleware.Logger,
		Recoverer(),
	)
	router.Get("/", func(http.ResponseWriter, *http.Request) {})
	router.Get("/redirect", func(w http.ResponseWriter, r *http.Request) {
		_ = WriteRedirect(w, r.Context(), http.StatusMovedPermanently, "/health", "resource moved")
	})
	api := humachi.New(router, huma.DefaultConfig("Test", "test"))
	huma.Get(api, "/panic", func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		panic("boom")
	})

	// 404
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", resp.Code)
	}

	// 405
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/", nil))
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 got %d", resp.Code)
	}

	// 500
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", resp.Code)
	}

	// 301
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/redirect", nil))
	if resp.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 got %d", resp.Code)
	}
}

func TestMessageOrDefaultFallback(t *testing.T) {
	if got := messageOrDefault(499, ""); got != "HTTP 499" {
		t.Fatalf("expected fallback message 'HTTP 499', got %q", got)
	}
	if got := messageOrDefault(200, "custom"); got != "custom" {
		t.Fatalf("expected custom message, got %q", got)
	}
}

func TestStatus304NotModifiedHasNoBody(t *testing.T) {
	Install()

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
		t.Fatalf("expected 304 got %d", resp.Code)
	}
	if resp.Body.Len() != 0 {
		t.Fatalf("expected empty body for 304 response, got %q", resp.Body.String())
	}
}
