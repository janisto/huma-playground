package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/janisto/huma-observability/v2"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/http/health"
	"github.com/janisto/huma-playground/internal/platform/auth"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

type stubVerifier struct {
	User  *auth.FirebaseUser
	Error error
}

type countingBody struct {
	reads int
}

func (body *countingBody) Read([]byte) (int, error) {
	body.reads++
	return 0, io.EOF
}

func (*countingBody) Close() error { return nil }

type trackedRequestBody struct {
	reader    *strings.Reader
	reads     int
	bytesRead int
}

func newTrackedRequestBody(document string) *trackedRequestBody {
	return &trackedRequestBody{reader: strings.NewReader(document)}
}

func (body *trackedRequestBody) Read(buffer []byte) (int, error) {
	body.reads++
	count, err := body.reader.Read(buffer)
	body.bytesRead += count
	return count, err
}

func (*trackedRequestBody) Close() error { return nil }

type boundaryProfileStore struct {
	unavailableProfileStore
	mu    sync.Mutex
	calls int
}

func (store *boundaryProfileStore) Create(
	context.Context,
	string,
	profilesvc.CreateParams,
) (*profilesvc.Profile, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.calls++
	return boundaryProfile(), nil
}

func (store *boundaryProfileStore) Update(
	context.Context,
	string,
	profilesvc.UpdateParams,
) (*profilesvc.Profile, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.calls++
	return boundaryProfile(), nil
}

func (store *boundaryProfileStore) callCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.calls
}

func boundaryProfile() *profilesvc.Profile {
	now := time.Date(2026, 8, 20, 7, 0, 0, 0, time.UTC)
	return &profilesvc.Profile{
		ID: "test-user-123", FirstName: "Ada", LastName: "Lovelace", ContactEmail: "ada@example.com",
		PhoneNumber: "+358401234567", TermsAccepted: true, CreatedAt: now, UpdatedAt: now,
	}
}

type panicProfileStore struct {
	unavailableProfileStore
}

func (panicProfileStore) Get(context.Context, string) (*profilesvc.Profile, error) {
	panic("secret profile value")
}

type fixedProfileStore struct {
	unavailableProfileStore
}

func (fixedProfileStore) Get(context.Context, string) (*profilesvc.Profile, error) {
	now := time.Date(2026, 8, 20, 7, 0, 0, 0, time.UTC)
	return &profilesvc.Profile{
		ID: "test-user-123", FirstName: "Ada", LastName: "Lovelace", ContactEmail: "ada@example.com",
		PhoneNumber: "+358401234567", TermsAccepted: true, CreatedAt: now, UpdatedAt: now,
	}, nil
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
	return testRouterWithLogger(t, cfg, zap.NewNop())
}

func testRouterWithLogger(t *testing.T, cfg config, logger *zap.Logger) http.Handler {
	t.Helper()
	return testRouterWithProfileStore(t, cfg, unavailableProfileStore{}, logger)
}

func testRouterWithProfileStore(
	t *testing.T,
	cfg config,
	profiles profilesvc.Store,
	logger *zap.Logger,
) http.Handler {
	t.Helper()
	githubClient, err := githubsvc.NewClient(http.DefaultClient)
	if err != nil {
		t.Fatalf("create GitHub client: %v", err)
	}
	return newRouter(cfg, dependencies{
		verifier: &stubVerifier{User: testUser()},
		profiles: profiles,
		github:   githubClient,
	}, logger)
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
			name: "Auth emulator host has surrounding whitespace",
			env: map[string]string{
				"FIREBASE_MODE":               "emulator",
				"FIREBASE_AUTH_EMULATOR_HOST": " localhost:7110",
				"FIRESTORE_EMULATOR_HOST":     "localhost:7130",
			},
		},
		{
			name: "Firestore emulator host has surrounding whitespace",
			env: map[string]string{
				"FIREBASE_MODE":               "emulator",
				"FIREBASE_AUTH_EMULATOR_HOST": "localhost:7110",
				"FIRESTORE_EMULATOR_HOST":     "localhost:7130 ",
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
		{
			name: "production prefix wildcard CORS",
			env: map[string]string{
				"APP_ENVIRONMENT":      "production",
				"FIREBASE_MODE":        "live",
				"FIREBASE_PROJECT_ID":  "real-project",
				"CORS_ALLOWED_ORIGINS": "https://*",
			},
		},
		{
			name: "production subdomain wildcard CORS",
			env: map[string]string{
				"APP_ENVIRONMENT":      "production",
				"FIREBASE_MODE":        "live",
				"FIREBASE_PROJECT_ID":  "real-project",
				"CORS_ALLOWED_ORIGINS": "https://*.example.com",
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

func TestRouterServesOnlyCanonicalHealthAndOpenAPIPaths(t *testing.T) {
	cfg := testConfig(t)
	router := testRouter(t, cfg)
	for _, test := range []struct {
		path string
		want int
	}{
		{path: "/health", want: http.StatusOK},
		{path: "/openapi.json", want: http.StatusOK},
		{path: "/v1/api-docs", want: http.StatusNotFound},
		{path: "/v1/openapi.json", want: http.StatusNotFound},
		{path: "/v1/schemas/ErrorModel.json", want: http.StatusNotFound},
		{path: "/health/", want: http.StatusNotFound},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("%s: expected %d, got %d: %s", test.path, test.want, response.Code, response.Body.String())
		}
	}
}

func TestRouterDoesNotInventAutomaticHeadOperations(t *testing.T) {
	server := httptest.NewServer(testRouter(t, testConfig(t)))
	defer server.Close()
	for _, test := range []struct {
		path, allow string
	}{
		{path: "/health", allow: "GET"},
		{path: "/v1/hello", allow: "GET, POST"},
		{path: "/openapi.json", allow: "GET"},
		{path: "/v1/items", allow: "GET"},
		{path: "/v1/profile", allow: "GET, POST, PATCH, DELETE"},
		{path: "/v1/github/owners/octocat", allow: "GET"},
		{path: "/v1/github/owners/octocat/repos", allow: "GET"},
		{path: "/v1/github/repos/octocat/repo", allow: "GET"},
		{path: "/v1/github/repos/octocat/repo/activity", allow: "GET"},
		{path: "/v1/github/repos/octocat/repo/languages", allow: "GET"},
		{path: "/v1/github/repos/octocat/repo/tags", allow: "GET"},
	} {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodHead, server.URL+test.path, nil)
		if err != nil {
			t.Fatalf("%s: create request: %v", test.path, err)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatalf("%s: request: %v", test.path, err)
		}
		if response.StatusCode != http.StatusMethodNotAllowed {
			_ = response.Body.Close()
			t.Fatalf("%s: expected 405, got %d", test.path, response.StatusCode)
		}
		if allow := response.Header.Get("Allow"); allow != test.allow {
			_ = response.Body.Close()
			t.Fatalf("%s: Allow=%q want=%q", test.path, allow, test.allow)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatalf("%s: read body: %v", test.path, err)
		}
		if len(body) != 0 {
			t.Fatalf("%s: HEAD response included %d body bytes", test.path, len(body))
		}
	}
}

func TestRouterRequestSizeBoundary(t *testing.T) {
	const limit = 1_000_000
	const requestID = "limit-boundary"
	operations := []struct {
		name, method, target, base string
		accepted                   int
		profile                    bool
	}{
		{name: "hello", method: http.MethodPost, target: "/v1/hello", base: `{"name":"Ada"}`, accepted: 200},
		{
			name:     "profile-create",
			method:   http.MethodPost,
			target:   "/v1/profile",
			base:     `{"firstName":"Ada","lastName":"Lovelace","contactEmail":"ada@example.com","phoneNumber":"+358401234567","termsAccepted":true}`,
			accepted: 201,
			profile:  true,
		},
		{
			name: "profile-update", method: http.MethodPatch, target: "/v1/profile",
			base: `{"firstName":"Grace"}`, accepted: 200, profile: true,
		},
	}
	transfers := []struct {
		name     string
		declared bool
	}{{name: "declared", declared: true}, {name: "streamed"}}
	sizes := []int{limit - 1, limit, limit + 1}

	for _, operation := range operations {
		for _, transfer := range transfers {
			for _, size := range sizes {
				name := operation.name + "/" + transfer.name + "/" + strconv.Itoa(size)
				t.Run(name, func(t *testing.T) {
					store := &boundaryProfileStore{}
					router := testRouterWithProfileStore(t, testConfig(t), store, zap.NewNop())
					document := operation.base + strings.Repeat(" ", size-len(operation.base))
					body := newTrackedRequestBody(document)
					request := httptest.NewRequestWithContext(t.Context(), operation.method, operation.target, body)
					request.Header.Set("Accept", "application/json")
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set(middleware.RequestIDHeader, requestID)
					if operation.profile {
						request.Header.Set("Authorization", "Bearer valid-token")
					}
					if transfer.declared {
						request.ContentLength = int64(size)
						request.Header.Set("Content-Length", strconv.Itoa(size))
					} else {
						request.ContentLength = -1
						request.TransferEncoding = []string{"chunked"}
						request.Header.Del("Content-Length")
					}

					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					want := operation.accepted
					if size > limit {
						want = http.StatusRequestEntityTooLarge
					}
					if response.Code != want {
						t.Fatalf("status=%d want=%d body=%s", response.Code, want, response.Body.String())
					}
					if got := response.Header().Get(middleware.RequestIDHeader); got != requestID {
						t.Fatalf("response request ID=%q want=%q", got, requestID)
					}
					wantCalls := 0
					if operation.profile && size <= limit {
						wantCalls = 1
					}
					if calls := store.callCount(); calls != wantCalls {
						t.Fatalf("persistence calls=%d want=%d", calls, wantCalls)
					}
					if transfer.declared && size > limit && body.bytesRead != 0 {
						t.Fatalf("declared overflow read %d bytes in %d reads", body.bytesRead, body.reads)
					}
					if size > limit {
						var problem struct {
							Code string `json:"code"`
						}
						if err := json.Unmarshal(
							response.Body.Bytes(),
							&problem,
						); err != nil ||
							problem.Code != "payload_too_large" {
							t.Fatalf("problem=%#v err=%v body=%s", problem, err, response.Body.String())
						}
					}
				})
			}
		}
	}
}

func TestRouterStreamedRequestStopsAfterFirstOverflowByte(t *testing.T) {
	const limit = 1_000_000
	operations := []struct {
		name, method, target, base string
		profile                    bool
	}{
		{name: "hello", method: http.MethodPost, target: "/v1/hello", base: `{"name":"Ada"}`},
		{
			name:    "profile-create",
			method:  http.MethodPost,
			target:  "/v1/profile",
			base:    `{"firstName":"Ada","lastName":"Lovelace","contactEmail":"ada@example.com","phoneNumber":"+358401234567","termsAccepted":true}`,
			profile: true,
		},
		{
			name:    "profile-update",
			method:  http.MethodPatch,
			target:  "/v1/profile",
			base:    `{"firstName":"Grace"}`,
			profile: true,
		},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			store := &boundaryProfileStore{}
			router := testRouterWithProfileStore(t, testConfig(t), store, zap.NewNop())
			document := operation.base + strings.Repeat(" ", limit+4096-len(operation.base))
			body := newTrackedRequestBody(document)
			request := httptest.NewRequestWithContext(t.Context(), operation.method, operation.target, body)
			request.ContentLength = -1
			request.TransferEncoding = []string{"chunked"}
			request.Header.Set("Accept", "application/json")
			request.Header.Set("Content-Type", "application/json")
			if operation.profile {
				request.Header.Set("Authorization", "Bearer valid-token")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if body.bytesRead != limit+1 {
				t.Fatalf("streamed overflow consumed %d bytes, want exactly %d", body.bytesRead, limit+1)
			}
			if calls := store.callCount(); calls != 0 {
				t.Fatalf("persistence calls=%d", calls)
			}
		})
	}
}

func TestRouterOpenAPIUsesMachineReadableSecurityPolicy(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.json", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", response.Code, response.Body.String())
	}
	const want = "default-src 'none'; frame-ancestors 'none'"
	if got := response.Header().Get("Content-Security-Policy"); got != want {
		t.Fatalf("unexpected OpenAPI Content-Security-Policy: want %q got %q", want, got)
	}
}

func TestAllOpenAPISchemasResolve(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.json", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("openapi: expected 200, got %d", response.Code)
	}
	var document any
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode OpenAPI: %v", err)
	}
	root, ok := document.(map[string]any)
	if !ok || len(root) == 0 {
		t.Fatal("OpenAPI is not an object")
	}
	var references int
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if rawReference, exists := typed["$ref"]; exists {
				reference, ok := rawReference.(string)
				if !ok || !strings.HasPrefix(reference, "#/") {
					t.Errorf("non-local OpenAPI reference %#v", rawReference)
				} else if _, ok := resolveJSONPointer(document, reference); !ok {
					t.Errorf("unresolved OpenAPI reference %q", reference)
				}
				references++
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(document)
	if references == 0 {
		t.Fatal("OpenAPI contains no references")
	}
}

func resolveJSONPointer(document any, reference string) (any, bool) {
	current := document
	for rawPart := range strings.SplitSeq(strings.TrimPrefix(reference, "#/"), "/") {
		part := strings.ReplaceAll(strings.ReplaceAll(rawPart, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
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

func TestRouterPreservesOnlyPortableRequestIDs(t *testing.T) {
	router := testRouter(t, testConfig(t))
	for _, candidate := range []string{"A", "A._:-z", "A" + strings.Repeat(":", 127)} {
		t.Run("accepted "+candidate[:1], func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
			request.Header.Set(middleware.RequestIDHeader, candidate)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Header().Get(middleware.RequestIDHeader) != candidate {
				t.Fatalf(
					"status=%d selected request ID=%q",
					response.Code,
					response.Header().Get(middleware.RequestIDHeader),
				)
			}
		})
	}

	tests := []struct {
		name   string
		values []string
	}{
		{name: "missing"},
		{name: "invalid first character", values: []string{"_candidate"}},
		{name: "whitespace", values: []string{"bad id"}},
		{name: "unicode", values: []string{"réquest"}},
		{name: "too long", values: []string{"A" + strings.Repeat("a", 128)}},
		{name: "comma combined", values: []string{"first,second"}},
		{name: "repeated", values: []string{"first", "second"}},
	}
	generated := make(map[string]struct{}, len(tests))
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/hello", nil)
			for _, value := range test.values {
				request.Header.Add(middleware.RequestIDHeader, value)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			selected := response.Header().Get(middleware.RequestIDHeader)
			if response.Code != http.StatusOK || !isGeneratedRequestID(selected) {
				t.Fatalf("status=%d selected request ID=%q", response.Code, selected)
			}
			if _, duplicate := generated[selected]; duplicate {
				t.Fatalf("generated duplicate request ID %q", selected)
			}
			generated[selected] = struct{}{}
		})
	}

	const concurrency = 32
	type requestResult struct {
		status int
		id     string
	}
	start := make(chan struct{})
	results := make(chan requestResult, concurrency)
	baseContext := t.Context()
	var wait sync.WaitGroup
	for range concurrency {
		wait.Go(func() {
			<-start
			request := httptest.NewRequestWithContext(baseContext, http.MethodGet, "/health", nil)
			request.Header.Set(middleware.RequestIDHeader, "invalid concurrent id")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			results <- requestResult{status: response.Code, id: response.Header().Get(middleware.RequestIDHeader)}
		})
	}
	close(start)
	wait.Wait()
	close(results)
	for result := range results {
		if result.status != http.StatusOK || !isGeneratedRequestID(result.id) {
			t.Errorf("concurrent status=%d selected request ID=%q", result.status, result.id)
			continue
		}
		if _, duplicate := generated[result.id]; duplicate {
			t.Errorf("concurrently generated duplicate request ID %q", result.id)
		}
		generated[result.id] = struct{}{}
	}
}

func TestRouterEmitsRequiredHeadersAcrossOutcomes(t *testing.T) {
	router := testRouter(t, testConfig(t))
	tests := []struct {
		name, method, target, authorization string
		want                                int
	}{
		{name: "success", method: http.MethodGet, target: "/health", want: http.StatusOK},
		{
			name:   "malformed query",
			method: http.MethodGet,
			target: "/v1/hello?unknown=true",
			want:   http.StatusBadRequest,
		},
		{name: "unauthorized", method: http.MethodGet, target: "/v1/profile", want: http.StatusUnauthorized},
		{
			name: "dependency unavailable", method: http.MethodGet, target: "/v1/profile",
			authorization: "Bearer token", want: http.StatusServiceUnavailable,
		},
		{name: "not found", method: http.MethodGet, target: "/missing", want: http.StatusNotFound},
		{name: "method not allowed", method: http.MethodPut, target: "/health", want: http.StatusMethodNotAllowed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), test.method, test.target, nil)
			request.Header.Set("Accept", "application/json")
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
			for name, want := range map[string]string{
				"Cache-Control":          "no-store",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
			} {
				if got := response.Header().Get(name); got != want {
					t.Errorf("%s=%q want=%q", name, got, want)
				}
			}
			if !isGeneratedRequestID(response.Header().Get(middleware.RequestIDHeader)) {
				t.Errorf("request ID=%q", response.Header().Get(middleware.RequestIDHeader))
			}
			if !slices.Contains(strings.Split(response.Header().Get("Vary"), ", "), "Accept") {
				t.Errorf("Vary=%q does not contain Accept", response.Header().Get("Vary"))
			}
		})
	}
}

func TestBodylessOperationDoesNotReadRequestBody(t *testing.T) {
	providerCalls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		providerCalls++
		response.Header().Set("Content-Type", "application/json")
		var document string
		switch request.URL.Path {
		case "/users/octocat":
			document = `{"id":1,"login":"octocat","type":"User","avatar_url":"https://avatars.example/1","html_url":"https://github.com/octocat","public_repos":1,"followers":2,"following":3,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}`
		case "/users/octocat/repos", "/repos/octocat/repo/activity", "/repos/octocat/repo/tags":
			document = `[]`
		case "/repos/octocat/repo":
			document = `{"id":2,"name":"repo","full_name":"octocat/repo","description":null,"html_url":"https://github.com/octocat/repo","fork":false,"private":false,"visibility":"public","language":null,"stargazers_count":0,"forks_count":0,"open_issues_count":0,"archived":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z","pushed_at":null,"default_branch":"main","topics":[],"disabled":false}`
		case "/repos/octocat/repo/languages":
			document = `{}`
		default:
			http.NotFound(response, request)
			return
		}
		if _, err := io.WriteString(response, document); err != nil {
			t.Errorf("write provider response: %v", err)
		}
	}))
	t.Cleanup(provider.Close)
	githubClient, err := githubsvc.NewClient(provider.Client(), githubsvc.WithBaseURL(provider.URL))
	if err != nil {
		t.Fatalf("create GitHub client: %v", err)
	}
	router := newRouter(testConfig(t), dependencies{
		verifier: &stubVerifier{User: testUser()}, profiles: fixedProfileStore{}, github: githubClient,
	}, zap.NewNop())
	tests := []struct {
		name, path, authorization string
	}{
		{name: "health", path: "/health"},
		{name: "OpenAPI", path: "/openapi.json"},
		{name: "hello", path: "/v1/hello"},
		{name: "items", path: "/v1/items"},
		{name: "profile", path: "/v1/profile", authorization: "Bearer local-token"},
		{name: "GitHub owner", path: "/v1/github/owners/octocat"},
		{name: "GitHub repositories", path: "/v1/github/owners/octocat/repos"},
		{name: "GitHub repository", path: "/v1/github/repos/octocat/repo"},
		{name: "GitHub activity", path: "/v1/github/repos/octocat/repo/activity"},
		{name: "GitHub languages", path: "/v1/github/repos/octocat/repo/languages"},
		{name: "GitHub tags", path: "/v1/github/repos/octocat/repo/tags"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := &countingBody{}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil)
			request.Body = body
			request.ContentLength = -1
			request.Header.Set("Accept", "application/json")
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK || body.reads != 0 {
				t.Fatalf("status=%d request body reads=%d body=%s", response.Code, body.reads, response.Body.String())
			}
		})
	}
	if providerCalls != 6 {
		t.Fatalf("provider calls=%d want=6", providerCalls)
	}
}

func isGeneratedRequestID(value string) bool {
	return len(value) == 32 && strings.IndexFunc(value, func(character rune) bool {
		return character < '0' || character > '9' && (character < 'a' || character > 'f')
	}) == -1
}

func TestRouterHumaAccessLogUsesV2PrivacyContract(t *testing.T) {
	logger, output := testObservabilityLogger(t)
	router := testRouterWithLogger(t, testConfig(t), logger)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/hello", nil)
	request.Header.Set(middleware.RequestIDHeader, "observability-request")
	request.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-03")
	request.Header.Set("User-Agent", "private-test-agent")
	request.RemoteAddr = "203.0.113.10:12345"
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get(middleware.RequestIDHeader); got != "observability-request" {
		t.Fatalf("response request ID = %q, want observability-request", got)
	}
	records := decodeLogRecords(t, output.Bytes())
	application := singleLogRecord(t, records, "hello get")
	access := singleLogRecord(t, records, "request completed")
	if got := countLogRecords(records, "http request completed"); got != 0 {
		t.Fatalf("Huma request emitted %d Chi access logs", got)
	}
	for name, record := range map[string]map[string]any{"application": application, "access": access} {
		assertJSONLogField(t, record, "request_id", "observability-request")
		assertJSONLogField(t, record, "correlation_id", "4bf92f3577b34da6a3ce929d0e0e4736")
		assertJSONLogField(t, record, "trace_sampled", true)
		if _, ok := record["trace_id_random"]; ok {
			t.Fatalf("%s record unexpectedly used Trace Context Level 2: %#v", name, record)
		}
	}
	assertJSONLogField(t, access, "severity", "INFO")
	assertJSONLogField(t, access, "method", http.MethodGet)
	assertJSONLogField(t, access, "path_template", "/v1/hello")
	assertJSONLogField(t, access, "operation_id", "getHello")
	assertJSONLogField(t, access, "status", float64(http.StatusOK))
	assertNoJSONLogFields(t, access, "path", "peer_ip", "remote_ip", "user_agent")
	httpRequest, ok := access["httpRequest"].(map[string]any)
	if !ok {
		t.Fatalf("httpRequest = %#v, want object", access["httpRequest"])
	}
	assertJSONLogField(t, httpRequest, "requestMethod", http.MethodGet)
	assertJSONLogField(t, httpRequest, "status", float64(http.StatusOK))
	assertNoJSONLogFields(t, httpRequest, "requestUrl", "remoteIp", "userAgent")
}

func TestRouterTerminalTelemetryUsesResponseRequestID(t *testing.T) {
	tests := []struct {
		name, method, target, body, contentType, authorization string
		want, event                                            int
		profileStore                                           profilesvc.Store
	}{
		{name: "success", method: http.MethodGet, target: "/health", want: http.StatusOK},
		{name: "not-found", method: http.MethodGet, target: "/missing", want: http.StatusNotFound, event: 1},
		{name: "method", method: http.MethodPut, target: "/health", want: http.StatusMethodNotAllowed, event: 1},
		{
			name: "body-limit", method: http.MethodPost, target: "/v1/hello",
			body: strings.Repeat(" ", 1_000_001), contentType: "application/json",
			want: http.StatusRequestEntityTooLarge, event: 1,
		},
		{name: "authentication", method: http.MethodGet, target: "/v1/profile", want: http.StatusUnauthorized},
		{
			name: "validation", method: http.MethodPost, target: "/v1/hello",
			body: `{"name":" Ada"}`, contentType: "application/json", want: http.StatusUnprocessableEntity,
		},
		{
			name: "dependency", method: http.MethodGet, target: "/v1/profile", authorization: "Bearer token",
			want: http.StatusServiceUnavailable,
		},
		{
			name: "profile-success", method: http.MethodGet, target: "/v1/profile", authorization: "Bearer token",
			want: http.StatusOK, profileStore: fixedProfileStore{},
		},
		{
			name: "panic", method: http.MethodGet, target: "/v1/profile", authorization: "Bearer token",
			want: http.StatusInternalServerError, profileStore: panicProfileStore{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger, output := testObservabilityLogger(t)
			store := profilesvc.Store(unavailableProfileStore{})
			if test.profileStore != nil {
				store = test.profileStore
			}
			router := testRouterWithProfileStore(t, testConfig(t), store, logger)
			var body io.Reader
			if test.body != "" {
				body = strings.NewReader(test.body)
			}
			request := httptest.NewRequestWithContext(t.Context(), test.method, test.target, body)
			request.Header.Set(middleware.RequestIDHeader, "terminal-request")
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want || response.Header().Get(middleware.RequestIDHeader) != "terminal-request" {
				t.Fatalf(
					"status=%d want=%d selected request ID=%q body=%s",
					response.Code,
					test.want,
					response.Header().Get(middleware.RequestIDHeader),
					response.Body.String(),
				)
			}
			records := decodeLogRecords(t, output.Bytes())
			if got := countLogRecords(
				records,
				"request completed",
			) + countLogRecords(
				records,
				"http request completed",
			); got != 1 {
				t.Fatalf("terminal access logs=%d records=%#v", got, records)
			}
			eventName := "request completed"
			if test.event == 1 {
				eventName = "http request completed"
			}
			terminal := singleLogRecord(t, records, eventName)
			assertJSONLogField(t, terminal, "request_id", "terminal-request")
			if _, panics := test.profileStore.(panicProfileStore); panics {
				assertJSONLogField(t, terminal, "terminal_reason", "panic")
			} else {
				assertJSONLogField(t, terminal, "status", float64(test.want))
			}
			if test.body != "" && strings.Contains(output.String(), test.body) ||
				test.authorization != "" && strings.Contains(output.String(), test.authorization) ||
				strings.Contains(output.String(), "secret profile value") {
				t.Fatalf("captured telemetry contains request content or credentials: %s", output.String())
			}
		})
	}
}

func TestRouterHumaAccessLogPreservesErrorStatus(t *testing.T) {
	logger, output := testObservabilityLogger(t)
	router := testRouterWithLogger(t, testConfig(t), logger)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/profile", nil)
	request.Header.Set(middleware.RequestIDHeader, "auth-observability-request")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d: %s", response.Code, response.Body.String())
	}
	records := decodeLogRecords(t, output.Bytes())
	access := singleLogRecord(t, records, "request completed")
	if got := countLogRecords(records, "http request completed"); got != 0 {
		t.Fatalf("Huma request emitted %d Chi access logs", got)
	}
	assertJSONLogField(t, access, "severity", "WARNING")
	assertJSONLogField(t, access, "request_id", "auth-observability-request")
	assertJSONLogField(t, access, "method", http.MethodGet)
	assertJSONLogField(t, access, "path_template", "/v1/profile")
	assertJSONLogField(t, access, "operation_id", "getProfile")
	assertJSONLogField(t, access, "status", float64(http.StatusUnauthorized))
	assertNoJSONLogFields(t, access, "terminal_reason")
	httpRequest, ok := access["httpRequest"].(map[string]any)
	if !ok {
		t.Fatalf("httpRequest = %#v, want object", access["httpRequest"])
	}
	assertJSONLogField(t, httpRequest, "requestMethod", http.MethodGet)
	assertJSONLogField(t, httpRequest, "status", float64(http.StatusUnauthorized))
}

func TestRouterChiAccessLogUsesV2PrivacyContract(t *testing.T) {
	logger, output := testObservabilityLogger(t)
	router := testRouterWithLogger(t, testConfig(t), logger)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/missing", nil)
	request.Header.Set(middleware.RequestIDHeader, "missing-observability-request")
	request.Header.Set("User-Agent", "private-test-agent")
	request.RemoteAddr = "203.0.113.10:12345"
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d: %s", response.Code, response.Body.String())
	}
	records := decodeLogRecords(t, output.Bytes())
	access := singleLogRecord(t, records, "http request completed")
	if got := countLogRecords(records, "request completed"); got != 0 {
		t.Fatalf("Chi request emitted %d Huma access logs", got)
	}
	assertJSONLogField(t, access, "request_id", "missing-observability-request")
	assertJSONLogField(t, access, "correlation_id", "missing-observability-request")
	assertJSONLogField(t, access, "severity", "WARNING")
	assertJSONLogField(t, access, "method", http.MethodGet)
	assertJSONLogField(t, access, "status", float64(http.StatusNotFound))
	assertNoJSONLogFields(t, access, "path", "path_template", "peer_ip", "remote_ip", "user_agent", "httpRequest")
}

func TestRouterDoesNotReflectForwardedHostIntoClosedResponses(t *testing.T) {
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
	if len(body) != 1 || body["message"] != "Hello, World!" || body["$schema"] != nil ||
		strings.Contains(response.Body.String(), "attacker") {
		t.Fatalf("unexpected closed response %#v", body)
	}
}

func TestRouterRejectsUnknownQuery(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/hello?typo=true", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
}

func TestRouterMethodNotAllowedIncludesAllow(t *testing.T) {
	router := testRouter(t, testConfig(t))
	tests := []struct {
		path  string
		allow string
	}{
		{path: "/health", allow: "GET"},
		{path: "/openapi.json", allow: "GET"},
		{path: "/v1/hello", allow: "GET, POST"},
		{path: "/v1/items", allow: "GET"},
		{path: "/v1/profile", allow: "GET, POST, PATCH, DELETE"},
		{path: "/v1/github/owners/octocat", allow: "GET"},
		{path: "/v1/github/owners/octocat/repos", allow: "GET"},
		{path: "/v1/github/repos/octocat/repo", allow: "GET"},
		{path: "/v1/github/repos/octocat/repo/activity", allow: "GET"},
		{path: "/v1/github/repos/octocat/repo/languages", allow: "GET"},
		{path: "/v1/github/repos/octocat/repo/tags", allow: "GET"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, test.path, nil)
			body := &countingBody{}
			request.Body = body
			request.ContentLength = -1
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405, got %d: %s", response.Code, response.Body.String())
			}
			if allow := response.Header().Get("Allow"); allow != test.allow {
				t.Fatalf("expected Allow %q, got %q", test.allow, allow)
			}
			if body.reads != 0 {
				t.Fatalf("unsupported method read request body %d times", body.reads)
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
		server.WriteTimeout != 15*time.Second ||
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

func TestServeListenerStartsAndShutsDownCleanly(t *testing.T) {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
		ReadHeaderTimeout: time.Second,
	}
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() {
		result <- serveListener(ctx, server, listener, time.Second, zap.NewNop())
	}()

	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://"+listener.Addr().String(),
		nil,
	)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	response, err := (&http.Client{Timeout: time.Second}).Do(request)
	if err != nil {
		t.Fatalf("request started server: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.StatusCode)
	}

	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("serveListener: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestOpenAPIMediaTypesMatchRuntime(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.json", nil)
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
				if _, hasJSON := operation.RequestBody.Content["application/json"]; !hasJSON {
					t.Errorf("%s %s request missing JSON", method, path)
				}
				if _, hasCBOR := operation.RequestBody.Content["application/cbor"]; !hasCBOR {
					t.Errorf("%s %s request missing CBOR", method, path)
				}
				if len(operation.RequestBody.Content) != 2 {
					t.Errorf("%s %s request media types = %v", method, path, maps.Keys(operation.RequestBody.Content))
				}
			}
			for status, response := range operation.Responses {
				_, hasProblemJSON := response.Content["application/problem+json"]
				_, hasCBOR := response.Content["application/cbor"]
				if strings.HasPrefix(status, "2") && status != "204" {
					_, hasJSON := response.Content["application/json"]
					if !hasJSON || !hasCBOR || len(response.Content) != 2 {
						t.Errorf(
							"%s %s response %s success media = %v",
							method,
							path,
							status,
							maps.Keys(response.Content),
						)
					}
				} else if status == "204" {
					if len(response.Content) != 0 {
						t.Errorf("%s %s response 204 has content %v", method, path, maps.Keys(response.Content))
					}
				} else if !hasProblemJSON || !hasCBOR || len(response.Content) != 2 {
					t.Errorf("%s %s response %s problem media = %v", method, path, status, maps.Keys(response.Content))
				}
				if _, forbidden := response.Content["application/problem+cbor"]; forbidden {
					t.Errorf("%s %s response %s advertises forbidden application/problem+cbor", method, path, status)
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
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.json", nil)
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

	githubStatuses := []string{"200", "400", "404", "406", "422", "429", "500", "502", "504"}
	expected := map[string]map[string][]string{
		"/health": {"get": {"200", "400", "406", "500"}},
		"/v1/hello": {
			"get":  {"200", "400", "406", "500"},
			"post": {"200", "400", "406", "413", "415", "422", "500"},
		},
		"/v1/items": {"get": {"200", "400", "406", "422", "500"}},
		"/v1/profile": {
			"delete": {"204", "400", "401", "404", "500", "503"},
			"get":    {"200", "400", "401", "404", "406", "500", "503"},
			"patch":  {"200", "400", "401", "404", "406", "413", "415", "422", "500", "503"},
			"post":   {"201", "400", "401", "406", "409", "413", "415", "422", "500", "503"},
		},
		"/v1/github/owners/{owner}":                 {"get": githubStatuses},
		"/v1/github/owners/{owner}/repos":           {"get": githubStatuses},
		"/v1/github/repos/{owner}/{repo}":           {"get": githubStatuses},
		"/v1/github/repos/{owner}/{repo}/activity":  {"get": githubStatuses},
		"/v1/github/repos/{owner}/{repo}/languages": {"get": githubStatuses},
		"/v1/github/repos/{owner}/{repo}/tags":      {"get": githubStatuses},
	}
	expectedOperationIDs := map[string]string{
		"get /health":                                   "getHealth",
		"get /v1/hello":                                 "getHello",
		"post /v1/hello":                                "createHello",
		"get /v1/items":                                 "listItems",
		"post /v1/profile":                              "createProfile",
		"get /v1/profile":                               "getProfile",
		"patch /v1/profile":                             "updateProfile",
		"delete /v1/profile":                            "deleteProfile",
		"get /v1/github/owners/{owner}":                 "getGitHubOwner",
		"get /v1/github/owners/{owner}/repos":           "listGitHubOwnerRepositories",
		"get /v1/github/repos/{owner}/{repo}":           "getGitHubRepository",
		"get /v1/github/repos/{owner}/{repo}/activity":  "listGitHubRepositoryActivity",
		"get /v1/github/repos/{owner}/{repo}/languages": "listGitHubRepositoryLanguages",
		"get /v1/github/repos/{owner}/{repo}/tags":      "listGitHubRepositoryTags",
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
			operationKey := method + " " + path
			if wantID := expectedOperationIDs[operationKey]; operation.OperationID != wantID {
				t.Errorf("%s operation ID = %q, want %q", operationKey, operation.OperationID, wantID)
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
			if wantBearer := path == "/v1/profile"; wantBearer {
				if !hasBearer || len(operation.Security) != 1 {
					t.Errorf("%s %s security = %v, want only bearer", method, path, operation.Security)
				}
			} else if operation.Security == nil || len(operation.Security) != 0 {
				t.Errorf("%s %s security = %v, want explicit public security: []", method, path, operation.Security)
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

func TestOpenAPIContractInvariantsMatchRuntime(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.json", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("openapi: expected 200, got %d: %s", response.Code, response.Body.String())
	}

	type schema struct {
		Ref                  string            `json:"$ref"`
		Type                 json.RawMessage   `json:"type"`
		Format               string            `json:"format"`
		Pattern              string            `json:"pattern"`
		Required             []string          `json:"required"`
		AdditionalProperties *bool             `json:"additionalProperties"`
		Properties           map[string]schema `json:"properties"`
		Items                *schema           `json:"items"`
		Enum                 []json.RawMessage `json:"enum"`
		Minimum              *float64          `json:"minimum"`
		Maximum              *float64          `json:"maximum"`
		MinLength            *int              `json:"minLength"`
		MaxLength            *int              `json:"maxLength"`
		MinItems             *int              `json:"minItems"`
		MaxItems             *int              `json:"maxItems"`
		MinProperties        *int              `json:"minProperties"`
		Default              json.RawMessage   `json:"default"`
	}
	type media struct {
		Schema schema `json:"schema"`
	}
	type requestBody struct {
		Required bool             `json:"required"`
		Content  map[string]media `json:"content"`
	}
	type parameter struct {
		Name     string `json:"name"`
		In       string `json:"in"`
		Required *bool  `json:"required"`
		Schema   schema `json:"schema"`
	}
	type operation struct {
		Parameters  []parameter  `json:"parameters"`
		RequestBody *requestBody `json:"requestBody"`
		Responses   map[string]struct {
			Headers map[string]json.RawMessage `json:"headers"`
			Content map[string]media           `json:"content"`
		} `json:"responses"`
	}
	var document struct {
		OpenAPI           string `json:"openapi"`
		JSONSchemaDialect string `json:"jsonSchemaDialect"`
		Info              struct {
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"info"`
		Components struct {
			Schemas map[string]schema `json:"schemas"`
		} `json:"components"`
		Paths map[string]map[string]operation `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode OpenAPI: %v", err)
	}

	if !strings.HasPrefix(document.OpenAPI, "3.1.") {
		t.Errorf("OpenAPI version = %q, want 3.1.x", document.OpenAPI)
	}
	if document.JSONSchemaDialect != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("JSON Schema dialect = %q, want Draft 2020-12", document.JSONSchemaDialect)
	}
	if document.Info.Title == "" || document.Info.Version == "" {
		t.Errorf("OpenAPI info = %#v, want non-empty title and version", document.Info)
	}

	updateSchema := document.Paths["/v1/profile"]["patch"].RequestBody.Content["application/json"].Schema
	if updateSchema.MinProperties == nil || *updateSchema.MinProperties != 1 {
		t.Fatalf("PATCH /v1/profile minProperties = %v, want 1", updateSchema.MinProperties)
	}
	if updateSchema.Ref != "#/components/schemas/ProfileUpdateBody" {
		t.Errorf("PATCH /v1/profile schema ref = %q", updateSchema.Ref)
	}

	for name, value := range document.Components.Schemas {
		if string(value.Type) == `"object"` && (value.AdditionalProperties == nil || *value.AdditionalProperties) {
			t.Errorf("component %s is not a closed object", name)
		}
	}

	requiredFields := map[string][]string{
		"HelloCreateInputBody": {"name"},
		"Money":                {"amountMinor", "currency"},
		"ProfileCreateInputBody": {
			"contactEmail", "firstName", "lastName", "phoneNumber", "termsAccepted",
		},
		"Profile": {
			"contactEmail", "createdAt", "firstName", "id", "lastName", "marketingOptIn",
			"phoneNumber", "termsAccepted", "updatedAt",
		},
		"Repository": {
			"archived", "createdAt", "defaultBranch", "description", "disabled", "fork", "forksCount",
			"fullName", "htmlUrl", "id", "language", "license", "name", "openIssuesCount", "pushedAt",
			"stargazersCount", "topics", "updatedAt",
		},
	}
	for schemaName, want := range requiredFields {
		got := slices.Clone(document.Components.Schemas[schemaName].Required)
		slices.Sort(got)
		want = slices.Clone(want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s required = %v, want %v", schemaName, got, want)
		}
	}
	if got := document.Components.Schemas["ProfileUpdateBody"].Required; len(got) != 0 {
		t.Errorf("ProfileUpdateBody required = %v, want no individually required fields", got)
	}

	for schemaName, fields := range map[string][]string{
		"Activity":   {"timestamp"},
		"Item":       {"createdAt"},
		"Owner":      {"createdAt", "updatedAt"},
		"Profile":    {"createdAt", "updatedAt"},
		"Repository": {"createdAt", "pushedAt", "updatedAt"},
	} {
		for _, field := range fields {
			property := document.Components.Schemas[schemaName].Properties[field]
			if got := property.Format; got != "date-time" {
				t.Errorf("%s.%s format = %q, want date-time", schemaName, field, got)
			}
			if property.Pattern == "" {
				t.Errorf("%s.%s has no exact millisecond UTC pattern", schemaName, field)
			}
		}
	}

	for schemaName, field := range map[string]string{
		"ActivityPage":   "activities",
		"Languages":      "languages",
		"ListData":       "items",
		"Repository":     "topics",
		"RepositoryPage": "repos",
		"TagPage":        "tags",
	} {
		got := string(document.Components.Schemas[schemaName].Properties[field].Type)
		if got != `"array"` {
			t.Errorf("%s.%s type = %s, want non-null array", schemaName, field, got)
		}
	}

	for schemaName, field := range map[string]string{
		"Activity":   "actor",
		"Owner":      "name",
		"Repository": "description",
	} {
		var types []string
		if err := json.Unmarshal(document.Components.Schemas[schemaName].Properties[field].Type, &types); err != nil {
			t.Errorf("%s.%s type is not a nullable union: %v", schemaName, field, err)
		} else {
			slices.Sort(types)
			if !slices.Equal(types, []string{"null", "string"}) {
				t.Errorf("%s.%s type = %v, want [null string]", schemaName, field, types)
			}
		}
	}

	money := document.Components.Schemas["Money"]
	amount := money.Properties["amountMinor"]
	if amount.Minimum == nil || *amount.Minimum != 0 || amount.Maximum == nil || *amount.Maximum != 9007199254740991 {
		t.Errorf("Money.amountMinor bounds = %v..%v", amount.Minimum, amount.Maximum)
	}
	if got := string(money.Properties["currency"].Enum[0]); got != `"USD"` {
		t.Errorf("Money.currency enum = %s, want USD", got)
	}
	if got := string(document.Components.Schemas["Profile"].Properties["termsAccepted"].Enum[0]); got != "true" {
		t.Errorf("Profile.termsAccepted enum = %s, want true", got)
	}

	expectedBodies := map[string]string{
		"post /v1/hello":    "#/components/schemas/HelloCreateInputBody",
		"post /v1/profile":  "#/components/schemas/ProfileCreateInputBody",
		"patch /v1/profile": "#/components/schemas/ProfileUpdateBody",
	}
	expectedParameters := map[string][]string{
		"get /health":    {"header:X-Request-ID"},
		"get /v1/hello":  {"header:X-Request-ID"},
		"post /v1/hello": {"header:X-Request-ID"},
		"get /v1/items": {
			"header:X-Request-ID",
			"query:category",
			"query:cursor",
			"query:limit",
		},
		"post /v1/profile":              {"header:X-Request-ID"},
		"get /v1/profile":               {"header:X-Request-ID"},
		"patch /v1/profile":             {"header:X-Request-ID"},
		"delete /v1/profile":            {"header:X-Request-ID"},
		"get /v1/github/owners/{owner}": {"header:X-Request-ID", "path:owner"},
		"get /v1/github/owners/{owner}/repos": {
			"header:X-Request-ID",
			"path:owner",
			"query:cursor",
			"query:limit",
		},
		"get /v1/github/repos/{owner}/{repo}": {"header:X-Request-ID", "path:owner", "path:repo"},
		"get /v1/github/repos/{owner}/{repo}/activity": {
			"header:X-Request-ID",
			"path:owner",
			"path:repo",
			"query:cursor",
			"query:limit",
		},
		"get /v1/github/repos/{owner}/{repo}/languages": {"header:X-Request-ID", "path:owner", "path:repo"},
		"get /v1/github/repos/{owner}/{repo}/tags": {
			"header:X-Request-ID",
			"path:owner",
			"path:repo",
			"query:cursor",
			"query:limit",
		},
	}
	for path, methods := range document.Paths {
		for method, operation := range methods {
			key := method + " " + path
			wantBodyRef, wantsBody := expectedBodies[key]
			if wantsBody {
				if operation.RequestBody == nil || !operation.RequestBody.Required {
					t.Errorf("%s request body is missing or optional", key)
				} else {
					for mediaType, medium := range operation.RequestBody.Content {
						if medium.Schema.Ref != wantBodyRef {
							t.Errorf(
								"%s %s request schema = %q, want %q",
								key,
								mediaType,
								medium.Schema.Ref,
								wantBodyRef,
							)
						}
					}
				}
			} else if operation.RequestBody != nil {
				t.Errorf("%s unexpectedly advertises a request body", key)
			}

			gotParameters := make([]string, 0, len(operation.Parameters))
			for _, value := range operation.Parameters {
				gotParameters = append(gotParameters, value.In+":"+value.Name)
				if value.In == "path" && (value.Required == nil || !*value.Required) {
					t.Errorf("%s path parameter %s is not required", key, value.Name)
				}
				if value.Name == "X-Request-ID" {
					if value.Schema.MaxLength == nil || *value.Schema.MaxLength != 128 ||
						value.Schema.Pattern != "^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$" {
						t.Errorf("%s X-Request-ID schema = %#v", key, value.Schema)
					}
				}
				if value.Name == "limit" {
					if value.Schema.Minimum == nil || *value.Schema.Minimum != 1 ||
						value.Schema.Maximum == nil || *value.Schema.Maximum != 100 ||
						string(value.Schema.Default) != "20" {
						t.Errorf("%s limit schema = %#v", key, value.Schema)
					}
				}
			}
			slices.Sort(gotParameters)
			wantParameters := slices.Clone(expectedParameters[key])
			slices.Sort(wantParameters)
			if !slices.Equal(gotParameters, wantParameters) {
				t.Errorf("%s parameters = %v, want %v", key, gotParameters, wantParameters)
			}

			for status, definition := range operation.Responses {
				for _, header := range []string{
					"X-Request-ID", "Cache-Control", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Vary",
				} {
					if _, ok := definition.Headers[header]; !ok {
						t.Errorf("%s response %s missing %s header", key, status, header)
					}
				}
				for mediaType, medium := range definition.Content {
					if medium.Schema.Ref == "" && len(medium.Schema.Type) == 0 {
						t.Errorf("%s response %s %s has no wire schema", key, status, mediaType)
					}
				}
			}
		}
	}

	for _, test := range []struct {
		path   string
		method string
		status string
		header string
	}{
		{path: "/v1/profile", method: "post", status: "201", header: "Location"},
		{path: "/v1/profile", method: "get", status: "401", header: "WWW-Authenticate"},
		{path: "/v1/items", method: "get", status: "200", header: "Link"},
		{path: "/v1/github/owners/{owner}/repos", method: "get", status: "200", header: "Link"},
		{path: "/v1/github/repos/{owner}/{repo}/activity", method: "get", status: "200", header: "Link"},
		{path: "/v1/github/repos/{owner}/{repo}/tags", method: "get", status: "200", header: "Link"},
		{
			path: "/v1/github/owners/{owner}", method: "get",
			status: "429", header: "Retry-After",
		},
		{
			path: "/v1/github/owners/{owner}", method: "get",
			status: "429", header: "X-RateLimit-Reset",
		},
	} {
		headers := document.Paths[test.path][test.method].Responses[test.status].Headers
		if _, ok := headers[test.header]; !ok {
			t.Errorf("%s %s response %s missing %s header", test.method, test.path, test.status, test.header)
		}
	}

	profileServiceUnavailableHeaders := document.Paths["/v1/profile"]["get"].Responses["503"].Headers
	if _, ok := profileServiceUnavailableHeaders["Retry-After"]; ok {
		t.Error("GET /v1/profile response 503 unexpectedly documents Retry-After")
	}

	problemCodes := map[string]struct {
		code   string
		status string
	}{
		"ProblemDependencyUnavailable": {"dependency_unavailable", "503"},
		"ProblemGithubNotFound":        {"github_not_found", "404"},
		"ProblemGithubRateLimit":       {"github_rate_limit", "429"},
		"ProblemGithubTimeout":         {"github_timeout", "504"},
		"ProblemGithubUpstream":        {"github_upstream", "502"},
		"ProblemInternalError":         {"internal_error", "500"},
		"ProblemInvalidRequest":        {"invalid_request", "400"},
		"ProblemNotAcceptable":         {"not_acceptable", "406"},
		"ProblemPayloadTooLarge":       {"payload_too_large", "413"},
		"ProblemProfileExists":         {"profile_exists", "409"},
		"ProblemProfileNotFound":       {"profile_not_found", "404"},
		"ProblemUnauthorized":          {"unauthorized", "401"},
		"ProblemUnsupportedMediaType":  {"unsupported_media_type", "415"},
		"ProblemValidationFailed":      {"validation_failed", "422"},
	}
	for name, want := range problemCodes {
		value := document.Components.Schemas[name]
		if got := string(value.Properties["code"].Enum[0]); got != `"`+want.code+`"` {
			t.Errorf("%s code enum = %s, want %q", name, got, want.code)
		}
		if got := string(value.Properties["status"].Enum[0]); got != want.status {
			t.Errorf("%s status enum = %s, want %s", name, got, want.status)
		}
	}
}

func TestVersionDefault(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("unexpected default version %q", Version)
	}
}

func testObservabilityLogger(t *testing.T) (*zap.Logger, *bytes.Buffer) {
	t.Helper()
	output := &bytes.Buffer{}
	logger, err := obs.NewLogger(obs.LoggerConfig{
		Preset:      obs.PresetGCP,
		Writer:      output,
		ErrorWriter: output,
	})
	if err != nil {
		t.Fatalf("create observability logger: %v", err)
	}
	return logger, output
}

func decodeLogRecords(t *testing.T, output []byte) []map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(output), []byte{'\n'})
	records := make([]map[string]any, 0, len(lines))
	for index, line := range lines {
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode log record %d: %v; line=%q", index, err, line)
		}
		records = append(records, record)
	}
	return records
}

func singleLogRecord(t *testing.T, records []map[string]any, message string) map[string]any {
	t.Helper()
	var match map[string]any
	for _, record := range records {
		if record["message"] != message {
			continue
		}
		if match != nil {
			t.Fatalf("multiple log records with message %q: %#v", message, records)
		}
		match = record
	}
	if match == nil {
		t.Fatalf("no log record with message %q: %#v", message, records)
	}
	return match
}

func countLogRecords(records []map[string]any, message string) int {
	count := 0
	for _, record := range records {
		if record["message"] == message {
			count++
		}
	}
	return count
}

func assertJSONLogField(t *testing.T, record map[string]any, key string, want any) {
	t.Helper()
	if got := record[key]; got != want {
		t.Fatalf("log field %q = %#v, want %#v; record=%#v", key, got, want, record)
	}
}

func assertNoJSONLogFields(t *testing.T, record map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, ok := record[key]; ok {
			t.Fatalf("log record unexpectedly contains %q: %#v", key, record)
		}
	}
}
