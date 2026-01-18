package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	humachi "github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

type testOutput struct {
	Body struct {
		UserID string `json:"user_id"`
	}
}

func setupTestAPI(verifier Verifier, requireAuth bool) *chi.Mux {
	router := chi.NewRouter()
	api := humachi.New(router, huma.DefaultConfig("Test", "1.0.0"))

	api.UseMiddleware(NewAuthMiddleware(api, verifier))

	var security []map[string][]string
	if requireAuth {
		security = []map[string][]string{{"bearer": {}}}
	}

	huma.Register(api, huma.Operation{
		OperationID: "test-endpoint",
		Method:      http.MethodGet,
		Path:        "/test",
		Security:    security,
	}, func(ctx context.Context, _ *struct{}) (*testOutput, error) {
		user := UserFromContext(ctx)
		out := &testOutput{}
		if user != nil {
			out.Body.UserID = user.UID
		}
		return out, nil
	})

	return router
}

func TestMiddlewareSkipsUnsecuredEndpoints(t *testing.T) {
	verifier := &MockVerifier{Error: ErrInvalidToken}
	router := setupTestAPI(verifier, false)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for unsecured endpoint, got %d", rec.Code)
	}
}

func TestMiddlewareRequiresAuthHeader(t *testing.T) {
	verifier := &MockVerifier{User: TestUser()}
	router := setupTestAPI(verifier, true)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth header, got %d", rec.Code)
	}
	if wwwAuth := rec.Header().Get("WWW-Authenticate"); wwwAuth != "Bearer" {
		t.Fatalf("expected WWW-Authenticate: Bearer, got %q", wwwAuth)
	}
}

func TestMiddlewareRejectsInvalidAuthFormat(t *testing.T) {
	verifier := &MockVerifier{User: TestUser()}
	router := setupTestAPI(verifier, true)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for Basic auth, got %d", rec.Code)
	}
}

func TestMiddlewareAuthenticatesValidToken(t *testing.T) {
	user := &FirebaseUser{UID: "verified-user-789"}
	verifier := &MockVerifier{User: user}
	router := setupTestAPI(verifier, true)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid token, got %d", rec.Code)
	}

	var body struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.UserID != user.UID {
		t.Fatalf("expected user ID %s, got %s", user.UID, body.UserID)
	}
}

func TestMiddlewareRejectsExpiredToken(t *testing.T) {
	verifier := &MockVerifier{Error: ErrTokenExpired}
	router := setupTestAPI(verifier, true)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", rec.Code)
	}
	if wwwAuth := rec.Header().Get("WWW-Authenticate"); wwwAuth != "Bearer" {
		t.Fatalf("expected WWW-Authenticate: Bearer, got %q", wwwAuth)
	}
}

func TestMiddlewareRejectsRevokedToken(t *testing.T) {
	verifier := &MockVerifier{Error: ErrTokenRevoked}
	router := setupTestAPI(verifier, true)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer revoked-token")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for revoked token, got %d", rec.Code)
	}
}

func TestMiddlewareHandlesCertificateFetchError(t *testing.T) {
	verifier := &MockVerifier{Error: ErrCertificateFetch}
	router := setupTestAPI(verifier, true)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for certificate fetch error, got %d", rec.Code)
	}
	if retryAfter := rec.Header().Get("Retry-After"); retryAfter != "30" {
		t.Fatalf("expected Retry-After: 30, got %q", retryAfter)
	}
}

func TestMiddlewareRejectsDisabledUser(t *testing.T) {
	verifier := &MockVerifier{Error: ErrUserDisabled}
	router := setupTestAPI(verifier, true)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer disabled-user-token")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for disabled user, got %d", rec.Code)
	}
}

func TestUserFromContextReturnsNilWithoutAuth(t *testing.T) {
	ctx := context.Background()
	user := UserFromContext(ctx)
	if user != nil {
		t.Fatal("expected nil user from unauthenticated context")
	}
}

func TestUserFromContextReturnsUser(t *testing.T) {
	expected := &FirebaseUser{UID: "context-user"}
	ctx := context.WithValue(context.Background(), userContextKey{}, expected)

	user := UserFromContext(ctx)
	if user == nil {
		t.Fatal("expected user from context")
	}
	if user.UID != expected.UID {
		t.Fatalf("expected UID %s, got %s", expected.UID, user.UID)
	}
}
