package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	humachi "github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	applog "github.com/janisto/huma-playground/internal/platform/logging"
	appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
	"github.com/janisto/huma-playground/internal/platform/pagination"
	"github.com/janisto/huma-playground/internal/platform/respond"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
)

type mockGitHubService struct {
	owner     *githubsvc.Owner
	repos     []githubsvc.RepoSummary
	repo      *githubsvc.Repo
	activity  *githubsvc.ActivityPage
	languages map[string]int64
	tags      []githubsvc.Tag
	err       error
}

func (m *mockGitHubService) GetOwner(_ context.Context, _ string) (*githubsvc.Owner, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.owner, nil
}

func (m *mockGitHubService) ListRepos(_ context.Context, _ string) ([]githubsvc.RepoSummary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.repos, nil
}

func (m *mockGitHubService) GetRepo(_ context.Context, _, _ string) (*githubsvc.Repo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.repo, nil
}

func (m *mockGitHubService) ListActivity(
	_ context.Context,
	_, _ string,
	_ int,
	_ string,
) (*githubsvc.ActivityPage, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.activity, nil
}

func (m *mockGitHubService) ListLanguages(_ context.Context, _, _ string) (map[string]int64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.languages, nil
}

func (m *mockGitHubService) ListTags(_ context.Context, _, _ string) ([]githubsvc.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tags, nil
}

var _ githubsvc.Service = (*mockGitHubService)(nil)

func newTestRouter(svc githubsvc.Service) chi.Router {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		applog.RequestLogger(),
		respond.Recoverer(),
	)
	api := humachi.New(router, huma.DefaultConfig("GitHubTest", "test"))
	Register(api, svc, "")
	return router
}

func testOwner() *githubsvc.Owner {
	return &githubsvc.Owner{
		Login:     "octocat",
		Name:      "The Octocat",
		AvatarURL: "https://avatars.githubusercontent.com/u/583231",
		HTMLURL:   "https://github.com/octocat",
		Bio:       "",
		Location:  "San Francisco",
		Blog:      "https://github.blog",
		Company:   "@github",
		CreatedAt: time.Date(2011, 1, 25, 18, 44, 36, 0, time.UTC),
		UpdatedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}
}

func testRepoSummary() githubsvc.RepoSummary {
	return githubsvc.RepoSummary{
		Name:        "git-consortium",
		FullName:    "octocat/git-consortium",
		Description: "This repo is for demonstration purposes.",
		HTMLURL:     "https://github.com/octocat/git-consortium",
		Language:    "Ruby",
		Stars:       16,
		Forks:       10,
		OpenIssues:  0,
		CreatedAt:   time.Date(2011, 1, 25, 18, 44, 36, 0, time.UTC),
		UpdatedAt:   time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}
}

func testRepo() *githubsvc.Repo {
	return &githubsvc.Repo{
		RepoSummary:   testRepoSummary(),
		DefaultBranch: "master",
		License:       "MIT License",
		Topics:        []string{},
		Archived:      false,
		Disabled:      false,
	}
}

func testActivity() githubsvc.Activity {
	return githubsvc.Activity{
		ID:             1,
		Actor:          "octocat",
		Ref:            "refs/heads/master",
		Timestamp:      time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		ActivityType:   "push",
		ActorAvatarURL: "https://avatars.githubusercontent.com/u/583231",
	}
}

// --- GetOwner ---

func TestGetOwnerSuccess(t *testing.T) {
	svc := &mockGitHubService{owner: testOwner()}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/octocat", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "get-owner-test")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	ct := resp.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected application/json, got %s", ct)
	}

	var owner Owner
	if err := json.Unmarshal(resp.Body.Bytes(), &owner); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if owner.Login != "octocat" {
		t.Errorf("expected login octocat, got %s", owner.Login)
	}
	if owner.Name != "The Octocat" {
		t.Errorf("expected name The Octocat, got %s", owner.Name)
	}
	if owner.Location != "San Francisco" {
		t.Errorf("expected location San Francisco, got %s", owner.Location)
	}
}

func TestGetOwnerNotFound(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrNotFound}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/unknown", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", problem.Status)
	}
}

func TestGetOwnerUpstreamError(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrUpstream}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/octocat", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.Code, resp.Body.String())
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Status != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", problem.Status)
	}
}

// --- ListOwnerRepos ---

func TestListOwnerReposSuccess(t *testing.T) {
	svc := &mockGitHubService{repos: []githubsvc.RepoSummary{testRepoSummary()}}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/octocat/repos", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "list-repos-test")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var data OwnerReposListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if data.Count != 1 {
		t.Errorf("expected count 1, got %d", data.Count)
	}
	if data.Repos[0].Name != "git-consortium" {
		t.Errorf("expected repo git-consortium, got %s", data.Repos[0].Name)
	}
}

func TestListOwnerReposNotFound(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrNotFound}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/unknown/repos", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestListOwnerReposUpstreamError(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrUpstream}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/octocat/repos", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.Code, resp.Body.String())
	}
}

// --- GetRepo ---

func TestGetRepoSuccess(t *testing.T) {
	svc := &mockGitHubService{repo: testRepo()}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/git-consortium", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "get-repo-test")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var repo Repo
	if err := json.Unmarshal(resp.Body.Bytes(), &repo); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if repo.Name != "git-consortium" {
		t.Errorf("expected name git-consortium, got %s", repo.Name)
	}
	if repo.DefaultBranch != "master" {
		t.Errorf("expected defaultBranch master, got %s", repo.DefaultBranch)
	}
	if repo.License != "MIT License" {
		t.Errorf("expected license MIT License, got %s", repo.License)
	}
}

func TestGetRepoNotFound(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrNotFound}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/unknown", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestGetRepoUpstreamError(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrUpstream}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/git-consortium", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.Code, resp.Body.String())
	}
}

// --- ListActivity ---

func TestListActivitySuccess(t *testing.T) {
	svc := &mockGitHubService{activity: &githubsvc.ActivityPage{
		Activities: []githubsvc.Activity{testActivity()},
		NextCursor: "",
	}}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/git-consortium/activity", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "list-activity-test")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var data RepoActivityListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if data.Count != 1 {
		t.Errorf("expected count 1, got %d", data.Count)
	}
	if data.Activities[0].Actor != "octocat" {
		t.Errorf("expected actor octocat, got %s", data.Activities[0].Actor)
	}
	if data.Activities[0].ActivityType != "push" {
		t.Errorf("expected activityType push, got %s", data.Activities[0].ActivityType)
	}

	linkHeader := resp.Header().Get("Link")
	if strings.Contains(linkHeader, `rel="next"`) {
		t.Errorf("expected no rel=next in Link header when no more pages, got %s", linkHeader)
	}
}

func TestListActivityWithPagination(t *testing.T) {
	svc := &mockGitHubService{activity: &githubsvc.ActivityPage{
		Activities: []githubsvc.Activity{testActivity()},
		NextCursor: "next-page-cursor",
	}}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/git-consortium/activity", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "list-activity-paginated")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	linkHeader := resp.Header().Get("Link")
	if !strings.Contains(linkHeader, "rel=\"next\"") {
		t.Error("expected Link header with rel=next when more pages exist")
	}
}

func TestListActivityInvalidCursor(t *testing.T) {
	svc := &mockGitHubService{}
	router := newTestRouter(svc)

	req := httptest.NewRequest(
		http.MethodGet,
		"/github/repos/octocat/git-consortium/activity?cursor=not-valid-base64!",
		nil,
	)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Status != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", problem.Status)
	}
}

func TestListActivityCursorTypeMismatch(t *testing.T) {
	svc := &mockGitHubService{}
	router := newTestRouter(svc)

	cursor := pagination.Cursor{Type: "wrong-type", Value: "some-value"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/git-consortium/activity?cursor="+cursor, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Status != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", problem.Status)
	}
}

func TestListActivityNotFound(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrNotFound}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/unknown/activity", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestListActivityUpstreamError(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrUpstream}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/git-consortium/activity", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.Code, resp.Body.String())
	}
}

// --- GetLanguages ---

func TestGetLanguagesSuccess(t *testing.T) {
	svc := &mockGitHubService{languages: map[string]int64{"Ruby": 6789, "Go": 12345}}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/git-consortium/languages", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "get-languages-test")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var data LanguagesData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if len(data.Languages) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(data.Languages))
	}
	if data.Languages[0].Name != "Go" {
		t.Errorf("expected first language Go (most bytes), got %s", data.Languages[0].Name)
	}
	if data.Languages[0].Bytes != 12345 {
		t.Errorf("expected 12345 bytes, got %d", data.Languages[0].Bytes)
	}
}

func TestGetLanguagesNotFound(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrNotFound}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/unknown/languages", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestGetLanguagesUpstreamError(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrUpstream}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/git-consortium/languages", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.Code, resp.Body.String())
	}
}

// --- ListTags ---

func TestListTagsSuccess(t *testing.T) {
	svc := &mockGitHubService{tags: []githubsvc.Tag{
		{Name: "v1.0", Commit: githubsvc.TagCommit{SHA: "abc123"}},
	}}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/git-consortium/tags", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "list-tags-test")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var data RepoTagsListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if data.Count != 1 {
		t.Errorf("expected count 1, got %d", data.Count)
	}
	if data.Tags[0].Name != "v1.0" {
		t.Errorf("expected tag v1.0, got %s", data.Tags[0].Name)
	}
	if data.Tags[0].Commit.SHA != "abc123" {
		t.Errorf("expected sha abc123, got %s", data.Tags[0].Commit.SHA)
	}
}

func TestListTagsNotFound(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrNotFound}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/unknown/tags", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestListTagsUpstreamError(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrUpstream}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/repos/octocat/git-consortium/tags", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.Code, resp.Body.String())
	}
}

// --- Forbidden ---

func TestGetOwnerForbidden(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrForbidden}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/octocat", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", resp.Code, resp.Body.String())
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Status != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", problem.Status)
	}
}

func TestGetOwnerRateLimited(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrRateLimited}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/octocat", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", resp.Code, resp.Body.String())
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Status != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", problem.Status)
	}
}

func TestGetOwnerRateLimitedPropagatesRetryHeaders(t *testing.T) {
	svc := &mockGitHubService{err: &githubsvc.UpstreamError{
		Kind:           githubsvc.UpstreamErrorKindRateLimited,
		Status:         http.StatusForbidden,
		RetryAfter:     "60",
		RateLimitReset: "1700000000",
	}}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/octocat", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", resp.Code, resp.Body.String())
	}

	if retryAfter := resp.Header().Get("Retry-After"); retryAfter != "60" {
		t.Fatalf("expected Retry-After 60, got %q", retryAfter)
	}
	if reset := resp.Header().Get("X-RateLimit-Reset"); reset != "1700000000" {
		t.Fatalf("expected X-RateLimit-Reset 1700000000, got %q", reset)
	}
}

// --- Content-Type ---

func TestResponseContentType(t *testing.T) {
	svc := &mockGitHubService{owner: testOwner()}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/octocat", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	ct := resp.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected application/json content type, got %s", ct)
	}
}

// --- Problem Details ---

func TestErrorProblemDetailsFormat(t *testing.T) {
	svc := &mockGitHubService{err: githubsvc.ErrNotFound}
	router := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/github/owners/unknown", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	ct := resp.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("expected application/problem+json, got %s", ct)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if problem.Title != "Not Found" {
		t.Errorf("expected title Not Found, got %s", problem.Title)
	}
	if problem.Status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", problem.Status)
	}
}
