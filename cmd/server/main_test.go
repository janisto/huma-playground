package main

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/http/health"
	"github.com/janisto/huma-playground/internal/platform/auth"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
)

type stubVerifier struct {
	User  *auth.FirebaseUser
	Error error
}

func (v *stubVerifier) Verify(context.Context, string) (*auth.FirebaseUser, error) {
	return v.User, v.Error
}

func testUser() *auth.FirebaseUser {
	return &auth.FirebaseUser{UID: "test-user-123", Email: "test@example.com", EmailVerified: true}
}

func testConfig(t *testing.T) config {
	t.Helper()
	cfg, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func testRouter(t *testing.T, cfg config) http.Handler {
	t.Helper()
	githubClient, err := githubsvc.NewClient(http.DefaultClient)
	if err != nil {
		t.Fatalf("create GitHub client: %v", err)
	}
	return newRouter(cfg, dependencies{
		verifier: &stubVerifier{User: testUser()},
		profiles: unavailableProfileStore{},
		github:   githubClient,
	}, zap.NewNop())
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg := testConfig(t)
	if cfg.Address != "0.0.0.0:8080" || cfg.Environment != environmentDevelopment {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
	if cfg.FirebaseMode != firebaseModeOffline || cfg.FirebaseProjectID != "demo-test-project" {
		t.Fatalf("unexpected Firebase defaults: %#v", cfg)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "*" {
		t.Fatalf("unexpected CORS defaults: %v", cfg.CORSOrigins)
	}
}

func TestLoadConfigRejectsUnsafeCombinations(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "invalid port", env: map[string]string{"PORT": "70000"}},
		{name: "invalid host", env: map[string]string{"HOST": "not a host"}},
		{name: "invalid environment", env: map[string]string{"APP_ENVIRONMENT": "prod"}},
		{name: "unsafe log level", env: map[string]string{"LOG_LEVEL": "fatal"}},
		{name: "undocumented log level alias", env: map[string]string{"LOG_LEVEL": "warning"}},
		{name: "invalid CORS origin", env: map[string]string{"CORS_ALLOWED_ORIGINS": "example.com/path"}},
		{name: "offline production", env: map[string]string{"APP_ENVIRONMENT": "production"}},
		{name: "live missing project", env: map[string]string{"FIREBASE_MODE": "live"}},
		{
			name: "live demo project",
			env:  map[string]string{"FIREBASE_MODE": "live", "FIREBASE_PROJECT_ID": "demo-prod"},
		},
		{
			name: "partial emulators",
			env:  map[string]string{"FIREBASE_MODE": "emulator", "FIREBASE_AUTH_EMULATOR_HOST": "localhost:7110"},
		},
		{
			name: "invalid emulator address",
			env: map[string]string{
				"FIREBASE_MODE":               "emulator",
				"FIREBASE_AUTH_EMULATOR_HOST": "http://localhost:7110",
				"FIRESTORE_EMULATOR_HOST":     "localhost:7130",
			},
		},
		{
			name: "emulator host contains whitespace",
			env: map[string]string{
				"FIREBASE_MODE":               "emulator",
				"FIREBASE_AUTH_EMULATOR_HOST": "bad host:7110",
				"FIRESTORE_EMULATOR_HOST":     "localhost:7130",
			},
		},
		{
			name: "production emulators",
			env: map[string]string{
				"APP_ENVIRONMENT":             "production",
				"FIREBASE_MODE":               "emulator",
				"FIREBASE_PROJECT_ID":         "demo-test",
				"FIREBASE_AUTH_EMULATOR_HOST": "localhost:7110",
				"FIRESTORE_EMULATOR_HOST":     "localhost:7130",
			},
		},
		{
			name: "production wildcard CORS",
			env: map[string]string{
				"APP_ENVIRONMENT":      "production",
				"FIREBASE_MODE":        "live",
				"FIREBASE_PROJECT_ID":  "real-project",
				"CORS_ALLOWED_ORIGINS": "*",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(func(key string) string { return tt.env[key] })
			if err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func TestLoadConfigProduction(t *testing.T) {
	values := map[string]string{
		"APP_ENVIRONMENT":      "production",
		"FIREBASE_MODE":        "live",
		"FIREBASE_PROJECT_ID":  "real-project",
		"CORS_ALLOWED_ORIGINS": "https://example.com, https://admin.example.com",
		"LOG_LEVEL":            "warn",
	}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.FirebaseProjectID != "real-project" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadConfigEmulator(t *testing.T) {
	values := map[string]string{
		"FIREBASE_MODE":               "emulator",
		"FIREBASE_PROJECT_ID":         "demo-local",
		"FIREBASE_AUTH_EMULATOR_HOST": "[::1]:7110",
		"FIRESTORE_EMULATOR_HOST":     "firestore:7130",
	}
	if _, err := loadConfig(func(key string) string { return values[key] }); err != nil {
		t.Fatalf("expected valid host:port emulator configuration: %v", err)
	}
}

func TestRouterServesHealthDocsAndOpenAPI(t *testing.T) {
	cfg := testConfig(t)
	router := testRouter(t, cfg)
	for _, test := range []struct {
		path string
		want int
	}{
		{path: "/health", want: http.StatusOK},
		{path: "/v1/api-docs", want: http.StatusOK},
		{path: "/v1/openapi.json", want: http.StatusOK},
		{path: "/v1/schemas/ErrorModel.json", want: http.StatusOK},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("%s: expected %d, got %d: %s", test.path, test.want, response.Code, response.Body.String())
		}
	}
}

func TestAllOpenAPISchemasResolve(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/openapi.json", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("openapi: expected 200, got %d", response.Code)
	}
	var document struct {
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode OpenAPI: %v", err)
	}
	if len(document.Components.Schemas) == 0 {
		t.Fatal("OpenAPI contains no component schemas")
	}
	for name := range document.Components.Schemas {
		t.Run(name, func(t *testing.T) {
			path := "/v1/schemas/" + url.PathEscape(name) + ".json"
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("%s: expected 200, got %d: %s", path, response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
				t.Fatalf("%s: expected application/json, got %q", path, contentType)
			}
		})
	}
}

func TestRouterHealthAndRequestID(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	request.Header.Set(middleware.RequestIDHeader, "health-request")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if got := response.Header().Get(middleware.RequestIDHeader); got != "health-request" {
		t.Fatalf("unexpected request ID %q", got)
	}
	var body health.Response
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health: %v", err)
	}
}

func TestRouterDoesNotTrustForwardedHostForSchemaLinks(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/v1/hello", nil)
	request.Header.Set("Forwarded", "host=forwarded-attacker.example")
	request.Header.Set("X-Forwarded-Host", "x-forwarded-attacker.example")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	schema, _ := body["$schema"].(string)
	if !strings.HasPrefix(schema, "https://api.example.com/v1/schemas/") {
		t.Fatalf("unexpected schema URL %q", schema)
	}
}

func TestRouterRejectsUnknownQuery(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/hello?typo=true", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", response.Code, response.Body.String())
	}
}

func TestRouterMethodNotAllowedIncludesAllow(t *testing.T) {
	router := testRouter(t, testConfig(t))
	tests := []struct {
		path  string
		allow string
	}{
		{path: "/health", allow: "GET"},
		{path: "/v1/hello", allow: "GET, POST"},
		{path: "/v1/profile", allow: "GET, POST, PATCH, DELETE"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405, got %d: %s", response.Code, response.Body.String())
			}
			if allow := response.Header().Get("Allow"); allow != test.allow {
				t.Fatalf("expected Allow %q, got %q", test.allow, allow)
			}
		})
	}
}

func TestOfflineModeFailsProtectedRoutesClosed(t *testing.T) {
	cfg := testConfig(t)
	clients, err := newApplicationClients(t.Context(), cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("new clients: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := clients.Close(); closeErr != nil {
			t.Errorf("close Firebase clients: %v", closeErr)
		}
	})
	router := newRouter(cfg, clients.dependencies, zap.NewNop())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/profile", nil)
	request.Header.Set("Authorization", "Bearer local-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", response.Code, response.Body.String())
	}
}

func TestRouterUsesConfiguredPrefix(t *testing.T) {
	cfg := testConfig(t)
	cfg.APIPrefix = "/api"
	router := testRouter(t, cfg)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/items?limit=1", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if link := response.Header().Get("Link"); !strings.Contains(link, "</api/items?") {
		t.Fatalf("unexpected Link header %q", link)
	}
}

func TestRequestContextTimeout(t *testing.T) {
	handler := requestContextTimeout(time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		if !errors.Is(r.Context().Err(), context.DeadlineExceeded) {
			t.Errorf("unexpected context error: %v", r.Context().Err())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.Code)
	}
}

func TestServerConfiguration(t *testing.T) {
	cfg := testConfig(t)
	server := newServer(cfg, http.NotFoundHandler())
	if server.Addr != cfg.Address ||
		server.ReadTimeout != 5*time.Second ||
		server.WriteTimeout != 10*time.Second ||
		server.MaxHeaderBytes != 64<<10 {
		t.Fatalf("unexpected server: %#v", server)
	}
}

func TestServeReturnsListenError(t *testing.T) {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := listener.Close(); closeErr != nil {
			t.Errorf("close listener: %v", closeErr)
		}
	})
	server := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           http.NotFoundHandler(),
		ReadHeaderTimeout: time.Second,
	}
	err = serve(t.Context(), server, time.Second, zap.NewNop())
	if err == nil || !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("expected address-in-use error, got %v", err)
	}
}

func TestServeDoesNotStartWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	server := &http.Server{
		Addr:              "127.0.0.1:0",
		Handler:           http.NotFoundHandler(),
		ReadHeaderTimeout: time.Second,
	}
	if err := serve(ctx, server, time.Second, zap.NewNop()); err != nil {
		t.Fatalf("expected canceled startup to be a clean no-op, got %v", err)
	}
}

func TestOpenAPIMediaTypesMatchRuntime(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/openapi.json", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	type content struct {
		Content map[string]json.RawMessage `json:"content"`
	}
	var document struct {
		Paths map[string]map[string]struct {
			RequestBody *content           `json:"requestBody"`
			Responses   map[string]content `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode OpenAPI: %v", err)
	}

	problemResponses := 0
	for path, methods := range document.Paths {
		for method, operation := range methods {
			if operation.RequestBody != nil {
				_, hasJSON := operation.RequestBody.Content["application/json"]
				_, hasCBOR := operation.RequestBody.Content["application/cbor"]
				if hasJSON != hasCBOR {
					t.Errorf("%s %s request JSON/CBOR mismatch", method, path)
				}
			}
			for status, response := range operation.Responses {
				_, hasJSON := response.Content["application/json"]
				_, hasCBOR := response.Content["application/cbor"]
				if hasJSON != hasCBOR {
					t.Errorf("%s %s response %s JSON/CBOR mismatch", method, path, status)
				}
				_, hasProblemJSON := response.Content["application/problem+json"]
				_, hasProblemCBOR := response.Content["application/problem+cbor"]
				if hasProblemJSON != hasProblemCBOR {
					t.Errorf("%s %s response %s Problem Details JSON/CBOR mismatch", method, path, status)
				}
				if hasProblemJSON {
					problemResponses++
				}
			}
		}
	}
	if problemResponses == 0 {
		t.Fatal("OpenAPI contains no Problem Details responses")
	}
}

func TestOpenAPIResponseStatusesAndSecurityMatchRuntime(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/openapi.json", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	var document struct {
		Paths map[string]map[string]struct {
			OperationID string                     `json:"operationId"`
			Responses   map[string]json.RawMessage `json:"responses"`
			Security    []map[string][]string      `json:"security"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode OpenAPI: %v", err)
	}

	githubStatuses := []string{"200", "403", "404", "422", "429", "500", "502", "503"}
	expected := map[string]map[string][]string{
		"/hello": {
			"get":  {"200", "422", "500"},
			"post": {"200", "400", "408", "413", "415", "422", "500"},
		},
		"/items": {"get": {"200", "400", "422", "500"}},
		"/profile": {
			"delete": {"204", "401", "404", "422", "500", "503"},
			"get":    {"200", "401", "404", "422", "500", "503"},
			"patch":  {"200", "400", "401", "404", "408", "413", "415", "422", "500", "503"},
			"post":   {"201", "400", "401", "408", "409", "413", "415", "422", "500", "503"},
		},
		"/github/owners/{owner}":       {"get": githubStatuses},
		"/github/owners/{owner}/repos": {"get": githubStatuses},
		"/github/repos/{owner}/{repo}": {"get": githubStatuses},
		"/github/repos/{owner}/{repo}/activity": {
			"get": {"200", "400", "403", "404", "422", "429", "500", "502", "503"},
		},
		"/github/repos/{owner}/{repo}/languages": {"get": githubStatuses},
		"/github/repos/{owner}/{repo}/tags":      {"get": githubStatuses},
	}
	operationIDs := make(map[string]string)
	operationCount := 0
	for path, methods := range expected {
		for method, want := range methods {
			operation, ok := document.Paths[path][method]
			if !ok {
				t.Fatalf("missing %s operation for %s", method, path)
			}
			operationCount++
			got := slices.Sorted(maps.Keys(operation.Responses))
			if !slices.Equal(got, want) {
				t.Errorf("%s %s response statuses = %v, want %v", method, path, got, want)
			}
			if operation.OperationID == "" {
				t.Errorf("%s %s has no operation ID", method, path)
			} else if previous, duplicate := operationIDs[operation.OperationID]; duplicate {
				t.Errorf("duplicate operation ID %q on %s %s and %s", operation.OperationID, method, path, previous)
			} else {
				operationIDs[operation.OperationID] = method + " " + path
			}
			hasBearer := false
			for _, requirement := range operation.Security {
				if _, ok := requirement[auth.BearerAuthScheme]; ok {
					hasBearer = true
				}
			}
			if wantBearer := path == "/profile"; hasBearer != wantBearer {
				t.Errorf("%s %s bearer security = %t, want %t", method, path, hasBearer, wantBearer)
			}
		}
	}
	actualOperationCount := 0
	for _, methods := range document.Paths {
		actualOperationCount += len(methods)
	}
	if len(document.Paths) != len(expected) {
		t.Errorf("OpenAPI paths = %d, want %d", len(document.Paths), len(expected))
	}
	if actualOperationCount != operationCount {
		t.Errorf("OpenAPI operations = %d, want %d", actualOperationCount, operationCount)
	}
	if len(operationIDs) != operationCount {
		t.Errorf("unique operation IDs = %d, operations = %d", len(operationIDs), operationCount)
	}
}

func TestVersionDefault(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("unexpected default version %q", Version)
	}
}
