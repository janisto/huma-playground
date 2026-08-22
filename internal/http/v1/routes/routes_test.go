package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	humachi "github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/janisto/huma-playground/internal/platform/auth"
	"github.com/janisto/huma-playground/internal/platform/pagination"
	"github.com/janisto/huma-playground/internal/platform/portable"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

type routeVerifier struct {
	mu    sync.Mutex
	calls int
}

func (verifier *routeVerifier) Verify(context.Context, string) (*auth.FirebaseUser, error) {
	verifier.mu.Lock()
	defer verifier.mu.Unlock()
	verifier.calls++
	return &auth.FirebaseUser{UID: "principal-a"}, nil
}

func (verifier *routeVerifier) count() int {
	verifier.mu.Lock()
	defer verifier.mu.Unlock()
	return verifier.calls
}

type routeProfileStore struct {
	mu         sync.Mutex
	calls      int
	principals []string
}

func (store *routeProfileStore) record(principal string) *profilesvc.Profile {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.calls++
	store.principals = append(store.principals, principal)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	return &profilesvc.Profile{
		ID: principal, FirstName: "Ada", LastName: "Lovelace",
		ContactEmail: "Ada@example.com", PhoneNumber: "+358401234567",
		TermsAccepted: true, CreatedAt: now, UpdatedAt: now,
	}
}

func (store *routeProfileStore) Create(
	_ context.Context,
	principal string,
	_ profilesvc.CreateParams,
) (*profilesvc.Profile, error) {
	return store.record(principal), nil
}

func (store *routeProfileStore) Get(_ context.Context, principal string) (*profilesvc.Profile, error) {
	return store.record(principal), nil
}

func (store *routeProfileStore) Update(
	_ context.Context,
	principal string,
	_ profilesvc.UpdateParams,
) (*profilesvc.Profile, error) {
	return store.record(principal), nil
}

func (store *routeProfileStore) Delete(_ context.Context, principal string) error {
	store.record(principal)
	return nil
}

func (store *routeProfileStore) count() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.calls
}

type routeGitHubService struct {
	mu    sync.Mutex
	calls int
}

func (service *routeGitHubService) record() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.calls++
}

func (service *routeGitHubService) GetOwner(context.Context, string) (githubsvc.Owner, error) {
	service.record()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return githubsvc.Owner{
		ID: 1, Login: "octocat", Type: "User", AvatarURL: "https://avatars.example/1",
		HTMLURL: "https://github.com/octocat", CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (service *routeGitHubService) ListOwnerRepositories(
	context.Context,
	string,
	int,
	*pagination.Cursor,
) (githubsvc.Page[githubsvc.RepositorySummary], error) {
	service.record()
	return githubsvc.Page[githubsvc.RepositorySummary]{Entries: []githubsvc.RepositorySummary{}}, nil
}

func (service *routeGitHubService) GetRepository(context.Context, string, string) (githubsvc.Repository, error) {
	service.record()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return githubsvc.Repository{
		RepositorySummary: githubsvc.RepositorySummary{
			ID: 1, Name: "repo", FullName: "octocat/repo", HTMLURL: "https://github.com/octocat/repo",
		},
		CreatedAt: now, UpdatedAt: now, DefaultBranch: "main", Topics: []string{},
	}, nil
}

func (service *routeGitHubService) ListRepositoryActivity(
	context.Context,
	string,
	string,
	int,
	*pagination.Cursor,
) (githubsvc.Page[githubsvc.Activity], error) {
	service.record()
	return githubsvc.Page[githubsvc.Activity]{Entries: []githubsvc.Activity{}}, nil
}

func (service *routeGitHubService) ListRepositoryLanguages(
	context.Context,
	string,
	string,
) ([]githubsvc.Language, error) {
	service.record()
	return []githubsvc.Language{}, nil
}

func (service *routeGitHubService) ListRepositoryTags(
	context.Context,
	string,
	string,
	int,
	*pagination.Cursor,
) (githubsvc.Page[githubsvc.Tag], error) {
	service.record()
	return githubsvc.Page[githubsvc.Tag]{Entries: []githubsvc.Tag{}}, nil
}

func (service *routeGitHubService) count() int {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.calls
}

func routeTestRouter(
	t *testing.T,
	verifier auth.Verifier,
	profiles profilesvc.Store,
	github githubsvc.Service,
) http.Handler {
	t.Helper()
	portable.ConfigureHuma()
	config := huma.DefaultConfig("Routes test", "test")
	config.DocsPath = ""
	config.OpenAPIPath = ""
	config.SchemasPath = ""
	config.CreateHooks = nil
	config.Transformers = nil
	config.Formats = portable.Formats()
	config.DefaultFormat = portable.MediaTypeJSON
	config.NoFormatFallback = true
	router := chi.NewRouter()
	router.Use(portable.RequestPolicy("/v1"))
	api := humachi.New(router, config)
	Register(api, "/v1", verifier, profiles, github)
	return router
}

func TestPublicRoutesNeverInvokeProfileAuthenticationOrPersistence(t *testing.T) {
	verifier := &routeVerifier{}
	profiles := &routeProfileStore{}
	github := &routeGitHubService{}
	router := routeTestRouter(t, verifier, profiles, github)
	tests := []struct {
		method, target, body string
	}{
		{method: "GET", target: "/v1/hello"},
		{method: "POST", target: "/v1/hello", body: `{"name":"Ada"}`},
		{method: "GET", target: "/v1/items?limit=1"},
		{method: "GET", target: "/v1/github/owners/octocat"},
		{method: "GET", target: "/v1/github/owners/octocat/repos"},
		{method: "GET", target: "/v1/github/repos/octocat/repo"},
		{method: "GET", target: "/v1/github/repos/octocat/repo/activity"},
		{method: "GET", target: "/v1/github/repos/octocat/repo/languages"},
		{method: "GET", target: "/v1/github/repos/octocat/repo/tags"},
	}
	for _, test := range tests {
		request := httptest.NewRequestWithContext(t.Context(), test.method, test.target, strings.NewReader(test.body))
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Authorization", "Bearer token-that-must-be-ignored")
		if test.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("%s %s status=%d body=%s", test.method, test.target, response.Code, response.Body.String())
		}
	}
	if verifier.count() != 0 || profiles.count() != 0 {
		t.Fatalf("public route side effects: verifier=%d profiles=%d", verifier.count(), profiles.count())
	}
	if github.count() != 6 {
		t.Fatalf("GitHub calls=%d want=6", github.count())
	}
}

func TestProfileRouteRequiresAuthThenUsesVerifiedPrincipal(t *testing.T) {
	verifier := &routeVerifier{}
	profiles := &routeProfileStore{}
	router := routeTestRouter(t, verifier, profiles, &routeGitHubService{})

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/profile", nil))
	if missing.Code != http.StatusUnauthorized || missing.Header().Get("WWW-Authenticate") != "Bearer" ||
		verifier.count() != 0 || profiles.count() != 0 {
		t.Fatalf("missing auth status=%d verifier=%d profiles=%d body=%s",
			missing.Code, verifier.count(), profiles.count(), missing.Body.String())
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/profile", nil)
	request.Header.Set("Authorization", "Bearer token-a")
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || verifier.count() != 1 || profiles.count() != 1 {
		t.Fatalf("status=%d verifier=%d profiles=%d body=%s",
			response.Code, verifier.count(), profiles.count(), response.Body.String())
	}
	profiles.mu.Lock()
	defer profiles.mu.Unlock()
	if len(profiles.principals) != 1 || profiles.principals[0] != "principal-a" {
		t.Fatalf("principals=%v", profiles.principals)
	}
}
