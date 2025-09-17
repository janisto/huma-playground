package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	apiinternal "github.com/janisto/huma-playground/internal/api"
	appmiddleware "github.com/janisto/huma-playground/internal/middleware"
	"github.com/janisto/huma-playground/internal/respond"
	"github.com/janisto/huma-playground/internal/routes"
)

// test setup replicates main router with only health route
func testServer() http.Handler {
	respond.Install()
	router := chi.NewRouter()
	router.NotFound(respond.NotFoundHandler())
	router.MethodNotAllowed(respond.MethodNotAllowedHandler())
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		appmiddleware.RequestLogger(),
		chimiddleware.Logger,
		respond.Recoverer(),
	)
	router.Get("/redirect", func(w http.ResponseWriter, r *http.Request) {
		_ = respond.WriteRedirect(w, r.Context(), http.StatusMovedPermanently, "/health", "resource moved")
	})
	api := humachi.New(router, huma.DefaultConfig("Huma Playground API", "test"))
	routes.Register(api)
	huma.Get(api, "/panic", func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		panic("boom")
	})
	return router
}

func TestHealth(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-health-req")
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 got %d", resp.Code)
	}

	var envelope apiinternal.Envelope[struct {
		Message string `json:"message"`
	}]
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if envelope.Data == nil || envelope.Data.Message != "healthy" {
		t.Fatalf("expected message 'healthy', got %+v", envelope.Data)
	}
	if envelope.Meta.TraceID == nil || *envelope.Meta.TraceID != "test-health-req" {
		t.Fatalf("expected traceId test-health-req, got %+v", envelope.Meta.TraceID)
	}
	if envelope.Error != nil {
		t.Fatalf("expected error to be null, got %+v", envelope.Error)
	}
}

func TestNotFoundReturnsEnvelope(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-404-req")
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("expected application/json content type, got %q", ct)
	}

	var envelope apiinternal.Envelope[struct{}]
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal 404 response: %v", err)
	}
	if envelope.Data != nil {
		t.Fatalf("expected nil data, got %+v", envelope.Data)
	}
	if envelope.Error == nil {
		t.Fatalf("expected error body")
	}
	if envelope.Meta.TraceID == nil || *envelope.Meta.TraceID != "test-404-req" {
		t.Fatalf("expected traceId test-404-req, got %+v", envelope.Meta.TraceID)
	}
	if envelope.Error.Code != "NOT_FOUND" {
		t.Fatalf("unexpected error code: %s", envelope.Error.Code)
	}
	if envelope.Error.Message != "resource not found" {
		t.Fatalf("unexpected error message: %s", envelope.Error.Message)
	}
}

func TestMethodNotAllowedReturnsEnvelope(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-405-req")
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 got %d", resp.Code)
	}
	if allow := resp.Header().Get("Allow"); !strings.Contains(allow, http.MethodGet) {
		t.Fatalf("expected Allow header to list GET, got %q", allow)
	}

	var envelope apiinternal.Envelope[struct{}]
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal 405 response: %v", err)
	}
	if envelope.Error == nil {
		t.Fatalf("expected error body")
	}
	if envelope.Meta.TraceID == nil || *envelope.Meta.TraceID != "test-405-req" {
		t.Fatalf("expected traceId test-405-req, got %+v", envelope.Meta.TraceID)
	}
	if envelope.Error.Code != "METHOD_NOT_ALLOWED" {
		t.Fatalf("unexpected code: %s", envelope.Error.Code)
	}
	if envelope.Error.Message != "method not allowed" {
		t.Fatalf("unexpected message: %s", envelope.Error.Message)
	}
}

func TestRecovererReturnsEnvelope(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-500-req")
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", resp.Code)
	}

	var envelope apiinternal.Envelope[struct{}]
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal 500 response: %v", err)
	}
	if envelope.Error == nil {
		t.Fatalf("expected error body")
	}
	if envelope.Meta.TraceID == nil || *envelope.Meta.TraceID != "test-500-req" {
		t.Fatalf("expected traceId test-500-req, got %+v", envelope.Meta.TraceID)
	}
	if envelope.Error.Code != "INTERNAL_SERVER_ERROR" {
		t.Fatalf("unexpected code: %s", envelope.Error.Code)
	}
	if envelope.Error.Message != "internal server error" {
		t.Fatalf("unexpected message: %s", envelope.Error.Message)
	}
}

func TestRedirectReturnsEnvelope(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest(http.MethodGet, "/redirect", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-301-req")
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, req)

	if resp.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 got %d", resp.Code)
	}
	if loc := resp.Header().Get("Location"); loc != "/health" {
		t.Fatalf("expected Location /health, got %q", loc)
	}

	var envelope apiinternal.Envelope[struct{}]
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal 3xx response: %v", err)
	}
	if envelope.Error == nil {
		t.Fatalf("expected error body")
	}
	if envelope.Meta.TraceID == nil || *envelope.Meta.TraceID != "test-301-req" {
		t.Fatalf("expected traceId test-301-req, got %+v", envelope.Meta.TraceID)
	}
	if envelope.Error.Code != "REDIRECT" {
		t.Fatalf("unexpected code: %s", envelope.Error.Code)
	}
	if envelope.Error.Message != "resource moved" {
		t.Fatalf("unexpected message: %s", envelope.Error.Message)
	}
}
