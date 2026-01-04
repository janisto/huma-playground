package hello

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	applog "github.com/janisto/huma-playground/internal/platform/logging"
	appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
	"github.com/janisto/huma-playground/internal/platform/respond"
)

func newTestRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		applog.RequestLogger(),
		respond.Recoverer(),
	)
	api := humachi.New(router, huma.DefaultConfig("HelloTest", "test"))
	Register(api)
	return router
}

func TestGetJSON(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-get-json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var hello Data
	if err := json.Unmarshal(resp.Body.Bytes(), &hello); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if hello.Message != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %s", hello.Message)
	}
}

func TestGetCBOR(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-get-cbor")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Errorf("expected application/cbor, got %s", ct)
	}

	var hello Data
	if err := cbor.Unmarshal(resp.Body.Bytes(), &hello); err != nil {
		t.Fatalf("cbor unmarshal: %v", err)
	}
	if hello.Message != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %s", hello.Message)
	}
}

func TestPostJSONSuccess(t *testing.T) {
	router := newTestRouter()

	body := `{"name":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-post-json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var hello Data
	if err := json.Unmarshal(resp.Body.Bytes(), &hello); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if hello.Message != "Hello, Test!" {
		t.Errorf("expected 'Hello, Test!', got %s", hello.Message)
	}
}

func TestPostCBORSuccess(t *testing.T) {
	router := newTestRouter()

	cborBody, err := cbor.Marshal(map[string]string{"name": "CBOR"})
	if err != nil {
		t.Fatalf("cbor marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/hello", bytes.NewReader(cborBody))
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-post-cbor")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Errorf("expected application/cbor, got %s", ct)
	}

	var hello Data
	if err := cbor.Unmarshal(resp.Body.Bytes(), &hello); err != nil {
		t.Fatalf("cbor unmarshal: %v", err)
	}
	if hello.Message != "Hello, CBOR!" {
		t.Errorf("expected 'Hello, CBOR!', got %s", hello.Message)
	}
}

func TestPostJSONValidationErrorDefaultsToJSON(t *testing.T) {
	router := newTestRouter()

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-error-json-default")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", ct)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Status != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", problem.Status)
	}
	if problem.Title != "Unprocessable Entity" {
		t.Errorf("expected title 'Unprocessable Entity', got %s", problem.Title)
	}
}

func TestPostJSONValidationErrorWithCBORAccept(t *testing.T) {
	router := newTestRouter()

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-error-json-cbor-accept")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Errorf("expected application/problem+cbor, got %s", ct)
	}

	var problem huma.ErrorModel
	if err := cbor.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("cbor unmarshal: %v", err)
	}
	if problem.Status != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", problem.Status)
	}
}

func TestPostCBORValidationErrorDefaultsToJSON(t *testing.T) {
	router := newTestRouter()

	cborBody, err := cbor.Marshal(map[string]string{"name": ""})
	if err != nil {
		t.Fatalf("cbor marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/hello", bytes.NewReader(cborBody))
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-error-cbor-json-default")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected application/problem+json (default), got %s", ct)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Status != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", problem.Status)
	}
}

func TestPostCBORValidationErrorWithCBORAccept(t *testing.T) {
	router := newTestRouter()

	cborBody, err := cbor.Marshal(map[string]string{"name": ""})
	if err != nil {
		t.Fatalf("cbor marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/hello", bytes.NewReader(cborBody))
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-error-cbor-cbor-accept")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Errorf("expected application/problem+cbor, got %s", ct)
	}

	var problem huma.ErrorModel
	if err := cbor.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("cbor unmarshal: %v", err)
	}
	if problem.Status != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", problem.Status)
	}
	if problem.Title != "Unprocessable Entity" {
		t.Errorf("expected title 'Unprocessable Entity', got %s", problem.Title)
	}
}
