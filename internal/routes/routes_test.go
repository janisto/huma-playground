package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	apiinternal "github.com/janisto/huma-playground/internal/api"
	appmiddleware "github.com/janisto/huma-playground/internal/middleware"
	"github.com/janisto/huma-playground/internal/respond"
)

func TestRegisterHealthRoute(t *testing.T) {
	respond.Install()

	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		appmiddleware.RequestLogger(),
		respond.Recoverer(),
	)

	api := humachi.New(router, huma.DefaultConfig("RoutesTest", "test"))
	Register(api)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "routes-test-trace")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.Code)
	}

	var env apiinternal.Envelope[struct {
		Message string `json:"message"`
	}]
	if err := json.Unmarshal(resp.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}

	if env.Data == nil || env.Data.Message != "healthy" {
		t.Fatalf("unexpected data payload: %+v", env.Data)
	}
	if env.Error != nil {
		t.Fatalf("expected error to be nil, got %+v", env.Error)
	}
	if env.Meta.TraceID == nil || *env.Meta.TraceID != "routes-test-trace" {
		t.Fatalf("expected traceId routes-test-trace, got %+v", env.Meta.TraceID)
	}
}
