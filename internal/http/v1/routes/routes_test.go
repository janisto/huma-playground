package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/janisto/huma-observability/v2"

	"github.com/janisto/huma-playground/internal/platform/auth"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
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

type mockProfileService struct{}

type mockGitHubService struct{}

func (mockGitHubService) GetOwner(context.Context, string) (*githubsvc.Owner, error) {
	return &githubsvc.Owner{
		Login:     "octocat",
		CreatedAt: time.Date(2011, 1, 25, 18, 44, 36, 0, time.UTC),
		UpdatedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}, nil
}

func (mockGitHubService) ListRepos(context.Context, string) ([]githubsvc.RepoSummary, error) {
	return []githubsvc.RepoSummary{}, nil
}

func (mockGitHubService) GetRepo(context.Context, string, string) (*githubsvc.Repo, error) {
	return &githubsvc.Repo{RepoSummary: githubsvc.RepoSummary{
		Name:      "git-consortium",
		FullName:  "octocat/git-consortium",
		CreatedAt: time.Date(2011, 1, 25, 18, 44, 36, 0, time.UTC),
		UpdatedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}}, nil
}

func (mockGitHubService) ListActivity(
	context.Context,
	string,
	string,
	int,
	string,
) (*githubsvc.ActivityPage, error) {
	return &githubsvc.ActivityPage{Activities: []githubsvc.Activity{}}, nil
}

func (mockGitHubService) ListLanguages(context.Context, string, string) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func (mockGitHubService) ListTags(context.Context, string, string) ([]githubsvc.Tag, error) {
	return []githubsvc.Tag{}, nil
}

func (m *mockProfileService) Create(
	_ context.Context,
	userID string,
	params profilesvc.CreateParams,
) (*profilesvc.Profile, error) {
	now := time.Now().UTC()
	return &profilesvc.Profile{
		ID:           userID,
		FirstName:    params.FirstName,
		LastName:     params.LastName,
		ContactEmail: params.ContactEmail,
		PhoneNumber:  params.PhoneNumber,
		Marketing:    params.Marketing,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (m *mockProfileService) Get(_ context.Context, userID string) (*profilesvc.Profile, error) {
	return &profilesvc.Profile{
		ID:           userID,
		FirstName:    "Test",
		LastName:     "User",
		ContactEmail: "test@example.com",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}, nil
}

func (m *mockProfileService) Update(
	_ context.Context,
	userID string,
	_ profilesvc.UpdateParams,
) (*profilesvc.Profile, error) {
	return &profilesvc.Profile{
		ID:           userID,
		FirstName:    "Updated",
		LastName:     "User",
		ContactEmail: "test@example.com",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}, nil
}

func (m *mockProfileService) Delete(_ context.Context, _ string) error {
	return nil
}

func newTestRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(
		chimiddleware.ClientIPFromRemoteAddr,
	)
	api := humachi.New(router, huma.DefaultConfig("RoutesTest", "test"))
	api.UseMiddleware(obs.RequestContext(obs.RequestContextConfig{}))
	api.UseMiddleware(obs.AccessLogger(obs.AccessLoggerConfig{}))
	verifier := &stubVerifier{User: testUser()}
	profileService := &mockProfileService{}
	githubService := mockGitHubService{}
	Register(api, "/v1", verifier, profileService, githubService)
	return router
}

func TestRegisterRoutesHello(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "routes-hello")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestRegisterRoutesItems(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "routes-items")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestRegisterRoutesProfileGet(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/profile", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "routes-profile-get")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestRegisterRoutesProfileUnauthorized(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/profile", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "routes-profile-noauth")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal problem: %v", err)
	}
	if problem.Status != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", problem.Status)
	}
}

func TestRegisterRoutesProfileDelete(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/profile", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "routes-profile-delete")
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.Code)
	}
}

func TestRegisterRoutesGitHubOwner(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/github/owners/octocat", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "routes-github-owner")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestRegisterRoutesGitHubRepo(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/github/repos/octocat/git-consortium",
		nil,
	)
	req.Header.Set(chimiddleware.RequestIDHeader, "routes-github-repo")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}
