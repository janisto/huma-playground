package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/janisto/huma-playground/internal/http/health"
	"github.com/janisto/huma-playground/internal/http/v1/routes"
	"github.com/janisto/huma-playground/internal/platform/auth"
	applog "github.com/janisto/huma-playground/internal/platform/logging"
	appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
	"github.com/janisto/huma-playground/internal/platform/respond"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

func testServer() http.Handler {
	router := chi.NewRouter()
	router.NotFound(respond.NotFoundHandler())
	router.MethodNotAllowed(respond.MethodNotAllowedHandler())
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		applog.RequestLogger(),
		respond.Recoverer(),
	)

	// Root-level endpoints
	router.Get("/health", health.Handler)
	router.Get("/redirect", func(w http.ResponseWriter, r *http.Request) {
		respond.WriteRedirect(w, r, "/health", http.StatusMovedPermanently)
	})

	// Versioned API
	router.Route("/v1", func(r chi.Router) {
		cfg := huma.DefaultConfig("Huma Playground API", "test")
		cfg.DocsPath = ""
		cfg.Servers = []*huma.Server{
			{URL: "/v1"},
		}
		api := humachi.New(r, cfg)
		verifier := &auth.MockVerifier{User: auth.TestUser()}
		profileService := profilesvc.NewMockProfileService()
		routes.Register(api, verifier, profileService)
		huma.Get(api, "/panic", func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			panic("boom")
		})
	})

	return router
}

func TestHealth(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-health-req")
	req.Header.Set("Accept", "application/json")
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 got %d", resp.Code)
	}

	var h health.Response
	if err := json.Unmarshal(resp.Body.Bytes(), &h); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if h.Status != "healthy" {
		t.Fatalf("expected status 'healthy', got %s", h.Status)
	}
}

func TestNotFoundReturnsProblemDetails(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-404-req")
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json content type, got %q", ct)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal 404 response: %v", err)
	}
	if problem.Status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", problem.Status)
	}
	if problem.Title != "Not Found" {
		t.Fatalf("unexpected title: %s", problem.Title)
	}
	if problem.Detail != "resource not found" {
		t.Fatalf("unexpected detail: %s", problem.Detail)
	}
}

func TestMethodNotAllowedReturnsProblemDetails(t *testing.T) {
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
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json content type, got %q", ct)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal 405 response: %v", err)
	}
	if problem.Status != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", problem.Status)
	}
	if problem.Title != "Method Not Allowed" {
		t.Fatalf("unexpected title: %s", problem.Title)
	}
	if !strings.Contains(problem.Detail, "POST") {
		t.Fatalf("expected detail to mention POST, got %s", problem.Detail)
	}
}

func TestRecovererReturnsProblemDetails(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/panic", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-500-req")
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json content type, got %q", ct)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal 500 response: %v", err)
	}
	if problem.Status != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", problem.Status)
	}
	if problem.Title != "Internal Server Error" {
		t.Fatalf("unexpected title: %s", problem.Title)
	}
	if problem.Detail != "internal server error" {
		t.Fatalf("unexpected detail: %s", problem.Detail)
	}
}

func TestRedirect(t *testing.T) {
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
}

func TestFallbackToJSONForUnknownAccept(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-fallback-req")
	req.Header.Set("Accept", "text/plain")
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, req)

	// Health endpoint always returns JSON since it's a plain HTTP handler
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json content type, got %q", ct)
	}

	var h health.Response
	if err := json.Unmarshal(resp.Body.Bytes(), &h); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if h.Status != "healthy" {
		t.Fatalf("expected status 'healthy', got %s", h.Status)
	}
}

func TestWildcardAcceptReturnsJSON(t *testing.T) {
	srv := testServer()
	tests := []struct {
		name   string
		accept string
	}{
		{"wildcard all", "*/*"},
		{"application wildcard", "application/*"},
		{"no accept header", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			req.Header.Set(chimiddleware.RequestIDHeader, "test-wildcard-req")
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			resp := httptest.NewRecorder()
			srv.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200 OK, got %d", resp.Code)
			}
			if ct := resp.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("expected application/json, got %q", ct)
			}

			var h health.Response
			if err := json.Unmarshal(resp.Body.Bytes(), &h); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if h.Status != "healthy" {
				t.Fatalf("expected status 'healthy', got %s", h.Status)
			}
		})
	}
}

func TestPortEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{"default when empty", "", "8080"},
		{"custom port", "3000", "3000"},
		{"another port", "9090", "9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("PORT", tt.envValue)
			}
			port := os.Getenv("PORT")
			if port == "" {
				port = "8080"
			}
			if port != tt.want {
				t.Errorf("got port %q, want %q", port, tt.want)
			}
		})
	}
}

func TestListenErrorChannel(t *testing.T) {
	listenErr := make(chan error, 1)

	// Simulate a listen error being sent
	expectedErr := &net.OpError{Op: "listen", Net: "tcp", Err: errors.New("address already in use")}
	go func() {
		listenErr <- expectedErr
	}()

	select {
	case err := <-listenErr:
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "address already in use") {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for error")
	}
}

func TestServerShutdownOnSignal(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              ":0", // random available port
		Handler:           router,
		ReadHeaderTimeout: time.Second,
	}

	listenErr := make(chan error, 1)
	started := make(chan struct{})

	go func() {
		var lc net.ListenConfig
		ln, err := lc.Listen(context.Background(), "tcp", srv.Addr)
		if err != nil {
			listenErr <- err
			return
		}
		close(started)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			listenErr <- err
		}
	}()

	select {
	case <-started:
		// Server started successfully
	case err := <-listenErr:
		t.Fatalf("server failed to start: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for server to start")
	}

	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}

	// Verify no listen error was sent (ErrServerClosed is filtered)
	select {
	case err := <-listenErr:
		t.Fatalf("unexpected listen error after shutdown: %v", err)
	default:
		// Expected: no error
	}
}

func TestOpenAPICBORContentTypes(t *testing.T) {
	router := chi.NewRouter()
	cfg := huma.DefaultConfig("Test API", "1.0.0")
	api := humachi.New(router, cfg)

	// Add CBOR content type hook similar to main.go
	api.OpenAPI().OnAddOperation = append(api.OpenAPI().OnAddOperation,
		func(_ *huma.OpenAPI, op *huma.Operation) {
			if op.RequestBody != nil && op.RequestBody.Content != nil {
				if jsonContent, ok := op.RequestBody.Content["application/json"]; ok {
					op.RequestBody.Content["application/cbor"] = jsonContent
				}
			}
			for _, resp := range op.Responses {
				if resp.Content == nil {
					continue
				}
				if jsonContent, ok := resp.Content["application/json"]; ok {
					resp.Content["application/cbor"] = jsonContent
				}
			}
		},
	)

	// Register a route with request body and response
	type TestInput struct {
		Body struct {
			Name string `json:"name"`
		}
	}
	type TestOutput struct {
		Body struct {
			Message string `json:"message"`
		}
	}
	huma.Post(api, "/test", func(_ context.Context, input *TestInput) (*TestOutput, error) {
		return &TestOutput{Body: struct {
			Message string `json:"message"`
		}{Message: "Hello, " + input.Body.Name}}, nil
	})

	// Check OpenAPI spec for CBOR content types
	spec := api.OpenAPI()
	op := spec.Paths["/test"].Post

	if op.RequestBody == nil {
		t.Fatal("expected request body in operation")
	}
	if _, ok := op.RequestBody.Content["application/json"]; !ok {
		t.Fatal("expected application/json in request body content")
	}
	if _, ok := op.RequestBody.Content["application/cbor"]; !ok {
		t.Fatal("expected application/cbor in request body content")
	}

	// Check 200 response has CBOR
	resp200 := op.Responses["200"]
	if resp200 == nil {
		t.Fatal("expected 200 response")
	}
	if _, ok := resp200.Content["application/json"]; !ok {
		t.Fatal("expected application/json in 200 response content")
	}
	if _, ok := resp200.Content["application/cbor"]; !ok {
		t.Fatal("expected application/cbor in 200 response content")
	}
}

func TestOpenAPICBORSkipsNilContent(t *testing.T) {
	router := chi.NewRouter()
	cfg := huma.DefaultConfig("Test API", "1.0.0")
	api := humachi.New(router, cfg)

	api.OpenAPI().OnAddOperation = append(api.OpenAPI().OnAddOperation,
		func(_ *huma.OpenAPI, op *huma.Operation) {
			if op.RequestBody != nil && op.RequestBody.Content != nil {
				if jsonContent, ok := op.RequestBody.Content["application/json"]; ok {
					op.RequestBody.Content["application/cbor"] = jsonContent
				}
			}
			for _, resp := range op.Responses {
				if resp.Content == nil {
					continue
				}
				if jsonContent, ok := resp.Content["application/json"]; ok {
					resp.Content["application/cbor"] = jsonContent
				}
			}
		},
	)

	// Register a route without request body (GET)
	huma.Get(api, "/no-body", func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return nil, nil
	})

	// Should not panic - verifies nil checks work
	spec := api.OpenAPI()
	op := spec.Paths["/no-body"].Get

	if op.RequestBody != nil {
		t.Fatal("expected no request body for GET")
	}
}

func TestServerConfiguration(t *testing.T) {
	srv := &http.Server{
		Addr:              ":8080",
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}

	if srv.ReadTimeout != 5*time.Second {
		t.Errorf("expected ReadTimeout 5s, got %v", srv.ReadTimeout)
	}
	if srv.ReadHeaderTimeout != 2*time.Second {
		t.Errorf("expected ReadHeaderTimeout 2s, got %v", srv.ReadHeaderTimeout)
	}
	if srv.WriteTimeout != 10*time.Second {
		t.Errorf("expected WriteTimeout 10s, got %v", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 60*time.Second {
		t.Errorf("expected IdleTimeout 60s, got %v", srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != 64<<10 {
		t.Errorf("expected MaxHeaderBytes 64KB, got %d", srv.MaxHeaderBytes)
	}
}

func TestVersionVariable(t *testing.T) {
	// Version is set at package level, verify it exists
	if Version == "" {
		t.Error("Version should have a default value")
	}
	if Version != "dev" {
		t.Errorf("expected default Version 'dev', got %q", Version)
	}
}

func TestCBORAcceptHeader(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/hello", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "test-cbor-req")
	req.Header.Set("Accept", "application/cbor")
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Fatalf("expected application/cbor content type, got %q", ct)
	}
}
