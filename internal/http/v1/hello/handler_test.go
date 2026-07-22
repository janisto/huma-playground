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
	"github.com/janisto/huma-observability/v2"
)

func newTestRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(
		chimiddleware.ClientIPFromRemoteAddr,
	)
	api := humachi.New(router, huma.DefaultConfig("HelloTest", "test"))
	api.UseMiddleware(obs.RequestContext(obs.RequestContextConfig{}))
	api.UseMiddleware(obs.AccessLogger(obs.AccessLoggerConfig{}))
	Register(api)
	return router
}

func TestGetJSON(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-get-json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if got := resp.Header().Get(chimiddleware.RequestIDHeader); got != "hello-get-json" {
		t.Fatalf("expected request ID response header, got %q", got)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", nil)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-post-json")
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", bytes.NewReader(cborBody))
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set(chimiddleware.RequestIDHeader, "hello-post-cbor")
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
	if hello.Message != "Hello, CBOR!" {
		t.Errorf("expected 'Hello, CBOR!', got %s", hello.Message)
	}
}

func TestPostJSONValidationErrorDefaultsToJSON(t *testing.T) {
	router := newTestRouter()

	body := `{"name":""}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", strings.NewReader(body))
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", strings.NewReader(body))
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", bytes.NewReader(cborBody))
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", bytes.NewReader(cborBody))
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

func TestPostJSONBoundaryContract(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		want        int
	}{
		{
			name:        "JSON with charset",
			body:        `{"name":"Ada"}`,
			contentType: "application/json; charset=utf-8",
			want:        http.StatusOK,
		},
		{name: "empty", body: "", contentType: "application/json", want: http.StatusBadRequest},
		{name: "null", body: "null", contentType: "application/json", want: http.StatusUnprocessableEntity},
		{
			name:        "unknown field",
			body:        `{"name":"Ada","unknown":true}`,
			contentType: "application/json",
			want:        http.StatusUnprocessableEntity,
		},
		{
			name:        "multiple values",
			body:        `{"name":"Ada"} {}`,
			contentType: "application/json",
			want:        http.StatusBadRequest,
		},
		{name: "malformed", body: `{"name":`, contentType: "application/json", want: http.StatusBadRequest},
		{
			name:        "unsupported media type",
			body:        `{"name":"Ada"}`,
			contentType: "text/plain",
			want:        http.StatusUnsupportedMediaType,
		},
	}
	router := newTestRouter()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/hello",
				strings.NewReader(test.body),
			)
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("expected %d, got %d: %s", test.want, response.Code, response.Body.String())
			}
		})
	}
}
