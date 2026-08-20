package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	humachi "github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	obs "github.com/janisto/huma-observability/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/janisto/huma-playground/internal/platform/pagination"
	"github.com/janisto/huma-playground/internal/platform/portable"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
)

type mockGitHubService struct {
	err       error
	calls     []string
	owner     githubsvc.Owner
	repos     githubsvc.Page[githubsvc.RepositorySummary]
	repo      githubsvc.Repository
	activity  githubsvc.Page[githubsvc.Activity]
	languages []githubsvc.Language
	tags      githubsvc.Page[githubsvc.Tag]
	limit     int
	cursor    *pagination.Cursor
	ownerArg  string
	repoArg   string
}

func (mock *mockGitHubService) record(operation string, limit int, cursor *pagination.Cursor) error {
	mock.calls = append(mock.calls, operation)
	mock.limit = limit
	mock.cursor = cursor
	return mock.err
}

func (mock *mockGitHubService) capturePath(owner, repo string) {
	mock.ownerArg = owner
	mock.repoArg = repo
}

func (mock *mockGitHubService) GetOwner(_ context.Context, owner string) (githubsvc.Owner, error) {
	mock.capturePath(owner, "")
	return mock.owner, mock.record("owner", 0, nil)
}

func (mock *mockGitHubService) ListOwnerRepositories(
	_ context.Context,
	owner string,
	limit int,
	cursor *pagination.Cursor,
) (githubsvc.Page[githubsvc.RepositorySummary], error) {
	mock.capturePath(owner, "")
	return mock.repos, mock.record("repos", limit, cursor)
}

func (mock *mockGitHubService) GetRepository(
	_ context.Context,
	owner, repo string,
) (githubsvc.Repository, error) {
	mock.capturePath(owner, repo)
	return mock.repo, mock.record("repo", 0, nil)
}

func (mock *mockGitHubService) ListRepositoryActivity(
	_ context.Context,
	owner, repo string,
	limit int,
	cursor *pagination.Cursor,
) (githubsvc.Page[githubsvc.Activity], error) {
	mock.capturePath(owner, repo)
	return mock.activity, mock.record("activity", limit, cursor)
}

func (mock *mockGitHubService) ListRepositoryLanguages(
	_ context.Context,
	owner, repo string,
) ([]githubsvc.Language, error) {
	mock.capturePath(owner, repo)
	return mock.languages, mock.record("languages", 0, nil)
}

func (mock *mockGitHubService) ListRepositoryTags(
	_ context.Context,
	owner, repo string,
	limit int,
	cursor *pagination.Cursor,
) (githubsvc.Page[githubsvc.Tag], error) {
	mock.capturePath(owner, repo)
	return mock.tags, mock.record("tags", limit, cursor)
}

func newGitHubTestRouter(t *testing.T, service githubsvc.Service) http.Handler {
	t.Helper()
	portable.ConfigureHuma()
	config := huma.DefaultConfig("GitHub test", "test")
	config.DocsPath = ""
	config.OpenAPIPath = ""
	config.SchemasPath = ""
	config.Formats = portable.Formats()
	config.DefaultFormat = portable.MediaTypeJSON
	config.NoFormatFallback = true
	router := chi.NewRouter()
	router.Use(portable.RequestPolicy("/v1"))
	api := humachi.New(router, config)
	group := huma.NewGroup(api, "/v1")
	Register(group, service, "/v1")
	return router
}

func performGitHubRequest(t *testing.T, router http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func decodeObject(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &object); err != nil {
		t.Fatalf("decode JSON: %v; body=%s", err, response.Body.String())
	}
	return object
}

func objectArray(t *testing.T, object map[string]any, field string) []any {
	t.Helper()
	values, ok := object[field].([]any)
	if !ok {
		t.Fatalf("%s has type %T, want array", field, object[field])
	}
	return values
}

func firstObject(t *testing.T, values []any) map[string]any {
	t.Helper()
	if len(values) == 0 {
		t.Fatal("expected a non-empty object array")
	}
	object, ok := values[0].(map[string]any)
	if !ok {
		t.Fatalf("array value has type %T, want object", values[0])
	}
	return object
}

func stringField(t *testing.T, object map[string]any, field string) string {
	t.Helper()
	value, ok := object[field].(string)
	if !ok {
		t.Fatalf("%s has type %T, want string", field, object[field])
	}
	return value
}

func publicLinkTarget(header, relation string) string {
	for value := range strings.SplitSeq(header, ",") {
		value = strings.TrimSpace(value)
		if !strings.HasSuffix(value, `rel="`+relation+`"`) || !strings.HasPrefix(value, "<") {
			continue
		}
		end := strings.IndexByte(value, '>')
		if end > 1 {
			return value[1:end]
		}
	}
	return ""
}

func githubFixtures() *mockGitHubService {
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := created.Add(123 * time.Millisecond)
	return &mockGitHubService{
		owner: githubsvc.Owner{
			ID: 1, Login: "octocat", Type: "User", Name: new("Octo"),
			AvatarURL: "https://avatars.example/1", HTMLURL: "https://github.com/octocat",
			PublicRepos: 2, Followers: 3, Following: 4, CreatedAt: created, UpdatedAt: updated,
		},
		repos: githubsvc.Page[githubsvc.RepositorySummary]{
			Entries: []githubsvc.RepositorySummary{{
				ID: 2, Name: "repo", FullName: "octocat/repo", HTMLURL: "https://github.com/octocat/repo",
			}},
			NextCursor: pagination.NewCursor(
				pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "octocat", Limit: 2},
				"next", "2",
			).Encode(),
		},
		repo: githubsvc.Repository{
			RepositorySummary: githubsvc.RepositorySummary{
				ID: 2, Name: "repo", FullName: "octocat/repo", HTMLURL: "https://github.com/octocat/repo",
			},
			StargazersCount: 3, ForksCount: 4, OpenIssuesCount: 5,
			CreatedAt: created, UpdatedAt: updated, DefaultBranch: "main", Topics: []string{},
		},
		activity: githubsvc.Page[githubsvc.Activity]{
			Entries: []githubsvc.Activity{{ID: 3, Ref: "refs/heads/main", Timestamp: created, ActivityType: "push"}},
		},
		languages: []githubsvc.Language{{Name: "Go", Bytes: 42}},
		tags: githubsvc.Page[githubsvc.Tag]{
			Entries: []githubsvc.Tag{{Name: "v1.0.0", SHA: "0123456789abcdef0123456789abcdef01234567"}},
		},
	}
}

func TestGitHubHandlersReturnExactOperationProjections(t *testing.T) {
	tests := []struct {
		name, target, operation string
		assert                  func(*testing.T, *httptest.ResponseRecorder, map[string]any)
	}{
		{
			name: "owner", target: "/v1/github/owners/octocat", operation: "owner",
			assert: func(t *testing.T, _ *httptest.ResponseRecorder, body map[string]any) {
				t.Helper()
				if body["id"] != float64(1) || body["login"] != "octocat" || body["name"] != "Octo" ||
					body["company"] != nil || body["createdAt"] != "2024-01-01T00:00:00.000Z" ||
					body["updatedAt"] != "2024-01-01T00:00:00.123Z" {
					t.Fatalf("unexpected owner: %#v", body)
				}
			},
		},
		{
			name: "owner repositories", target: "/v1/github/owners/octocat/repos?limit=2", operation: "repos",
			assert: func(t *testing.T, response *httptest.ResponseRecorder, body map[string]any) {
				t.Helper()
				if body["count"] != float64(1) || len(objectArray(t, body, "repos")) != 1 {
					t.Fatalf("unexpected repository page: %#v", body)
				}
				if link := response.Header().
					Get("Link"); !strings.HasPrefix(link, "</v1/github/owners/octocat/repos?") ||
					!strings.Contains(link, "limit=2") ||
					!strings.Contains(link, `rel="next"`) {
					t.Fatalf("Link = %q", link)
				}
			},
		},
		{
			name: "repository", target: "/v1/github/repos/octocat/repo", operation: "repo",
			assert: func(t *testing.T, _ *httptest.ResponseRecorder, body map[string]any) {
				t.Helper()
				if body["fullName"] != "octocat/repo" || body["language"] != nil || body["pushedAt"] != nil ||
					body["license"] != nil || len(objectArray(t, body, "topics")) != 0 {
					t.Fatalf("unexpected repository: %#v", body)
				}
			},
		},
		{
			name: "activity", target: "/v1/github/repos/octocat/repo/activity?limit=1", operation: "activity",
			assert: func(t *testing.T, _ *httptest.ResponseRecorder, body map[string]any) {
				t.Helper()
				entry := firstObject(t, objectArray(t, body, "activities"))
				if body["count"] != float64(1) || entry["actor"] != nil || entry["actorAvatarUrl"] != nil {
					t.Fatalf("unexpected activity page: %#v", body)
				}
			},
		},
		{
			name: "languages", target: "/v1/github/repos/octocat/repo/languages", operation: "languages",
			assert: func(t *testing.T, _ *httptest.ResponseRecorder, body map[string]any) {
				t.Helper()
				entry := firstObject(t, objectArray(t, body, "languages"))
				if entry["name"] != "Go" || entry["bytes"] != float64(42) {
					t.Fatalf("unexpected languages: %#v", body)
				}
			},
		},
		{
			name: "tags", target: "/v1/github/repos/octocat/repo/tags?limit=1", operation: "tags",
			assert: func(t *testing.T, _ *httptest.ResponseRecorder, body map[string]any) {
				t.Helper()
				entry := firstObject(t, objectArray(t, body, "tags"))
				commit, ok := entry["commit"].(map[string]any)
				if !ok || entry["name"] != "v1.0.0" || len(stringField(t, commit, "sha")) != 40 {
					t.Fatalf("unexpected tags: %#v", body)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := githubFixtures()
			response := performGitHubRequest(t, newGitHubTestRouter(t, service), test.target)
			if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json" {
				t.Fatalf(
					"status=%d content-type=%q body=%s",
					response.Code,
					response.Header().Get("Content-Type"),
					response.Body.String(),
				)
			}
			if len(service.calls) != 1 || service.calls[0] != test.operation {
				t.Fatalf("calls = %v", service.calls)
			}
			test.assert(t, response, decodeObject(t, response))
		})
	}
}

func TestGitHubHandlersAcceptExactPathAndLimitBoundaries(t *testing.T) {
	maximumOwner := "a_" + strings.Repeat("b", 36) + "c"
	maximumRepo := strings.Repeat(".", 99) + "a"
	pathTests := []struct {
		name, target, operation, owner, repo string
	}{
		{name: "one-character owner", target: "/v1/github/owners/a", operation: "owner", owner: "a"},
		{
			name: "39-character owner with underscore", target: "/v1/github/owners/" + maximumOwner,
			operation: "owner", owner: maximumOwner,
		},
		{
			name: "one-character repository", target: "/v1/github/repos/a/_", operation: "repo",
			owner: "a", repo: "_",
		},
		{
			name: "100-character repository", target: "/v1/github/repos/a/" + maximumRepo,
			operation: "repo", owner: "a", repo: maximumRepo,
		},
	}
	for _, test := range pathTests {
		t.Run(test.name, func(t *testing.T) {
			service := githubFixtures()
			response := performGitHubRequest(t, newGitHubTestRouter(t, service), test.target)
			if response.Code != http.StatusOK || len(service.calls) != 1 || service.calls[0] != test.operation ||
				service.ownerArg != test.owner || service.repoArg != test.repo {
				t.Fatalf("status=%d calls=%v owner=%q repo=%q body=%s",
					response.Code, service.calls, service.ownerArg, service.repoArg, response.Body.String())
			}
		})
	}

	collections := []struct {
		name, path, operation string
	}{
		{name: "owner repositories", path: "/v1/github/owners/octocat/repos", operation: "repos"},
		{name: "activity", path: "/v1/github/repos/octocat/repo/activity", operation: "activity"},
		{name: "tags", path: "/v1/github/repos/octocat/repo/tags", operation: "tags"},
	}
	limits := []struct {
		name, query string
		want        int
	}{
		{name: "omitted", want: 20},
		{name: "minimum", query: "?limit=1", want: 1},
		{name: "explicit default", query: "?limit=20", want: 20},
		{name: "maximum", query: "?limit=100", want: 100},
	}
	for _, collection := range collections {
		for _, limit := range limits {
			t.Run(collection.name+"/"+limit.name, func(t *testing.T) {
				service := githubFixtures()
				service.repos.NextCursor = ""
				response := performGitHubRequest(
					t, newGitHubTestRouter(t, service), collection.path+limit.query,
				)
				if response.Code != http.StatusOK || len(service.calls) != 1 ||
					service.calls[0] != collection.operation || service.limit != limit.want {
					t.Fatalf("status=%d calls=%v limit=%d body=%s",
						response.Code, service.calls, service.limit, response.Body.String())
				}
			})
		}
	}
}

func TestGitHubHandlersDecodeAndBindScopedCursor(t *testing.T) {
	service := githubFixtures()
	cursor := pagination.NewCursor(
		pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "octocat", Limit: 7},
		"next", "3",
	).Encode()
	response := performGitHubRequest(
		t,
		newGitHubTestRouter(t, service),
		"/v1/github/owners/octocat/repos?limit=7&cursor="+cursor,
	)
	if response.Code != http.StatusOK || service.limit != 7 || service.cursor == nil || service.cursor.Anchor != "3" {
		t.Fatalf(
			"status=%d limit=%d cursor=%#v body=%s",
			response.Code,
			service.limit,
			service.cursor,
			response.Body.String(),
		)
	}
}

func TestGitHubCollectionsTraverseThreeProviderPagesThroughFrameworkBoundary(t *testing.T) {
	tests := []struct {
		name, publicPath, providerPath, numericPath, bodyMember string
		fixedQuery                                              func(map[string][]string) bool
		document                                                func(int) string
	}{
		{
			name: "owner repositories", publicPath: "/v1/github/owners/octocat/repos?limit=1",
			providerPath: "/users/octocat/repos", numericPath: "/user/42/repos", bodyMember: "repos",
			fixedQuery: func(query map[string][]string) bool {
				return (len(query) == 4 || len(query) == 5) && query["type"][0] == "owner" &&
					query["sort"][0] == "full_name" &&
					query["direction"][0] == "asc" && query["per_page"][0] == "1"
			},
			document: func(page int) string {
				return fmt.Sprintf(
					`[{"id":%d,"name":"repo%d","full_name":"octocat/repo%d","description":null,"html_url":"https://github.com/octocat/repo%d","fork":false,"private":false,"visibility":"public"}]`,
					page,
					page,
					page,
					page,
				)
			},
		},
		{
			name: "repository tags", publicPath: "/v1/github/repos/octocat/repo/tags?limit=1",
			providerPath: "/repos/octocat/repo/tags", numericPath: "/repositories/42/tags", bodyMember: "tags",
			fixedQuery: func(query map[string][]string) bool {
				return (len(query) == 1 || len(query) == 2) && query["per_page"][0] == "1"
			},
			document: func(page int) string {
				return fmt.Sprintf(
					`[{"name":"v%d","commit":{"sha":"0123456789abcdef0123456789abcdef01234567"}}]`,
					page,
				)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var providerURL string
			provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.providerPath || !test.fixedQuery(request.URL.Query()) {
					t.Errorf("provider request=%s", request.URL.RequestURI())
				}
				page := 1
				if rawPage := request.URL.Query().Get("page"); rawPage != "" {
					parsed, err := strconv.Atoi(rawPage)
					if err != nil {
						t.Errorf("provider page=%q", rawPage)
					} else {
						page = parsed
					}
				}
				providerTarget := func(targetPage int) string {
					query := "per_page=1&page=" + strconv.Itoa(targetPage)
					if test.bodyMember == "repos" {
						query = "direction=asc&page=" + strconv.Itoa(targetPage) +
							"&per_page=1&sort=full_name&type=owner"
					}
					return providerURL + test.numericPath + "?" + query
				}
				links := make([]string, 0, 2)
				if page > 1 {
					links = append(links, "<"+providerTarget(page-1)+`>; rel="prev"`)
				}
				if page < 3 {
					links = append(links, "<"+providerTarget(page+1)+`>; title="page,next"; rel="next"`)
				}
				if len(links) > 0 {
					response.Header().Set("Link", strings.Join(links, ", "))
				}
				response.Header().Set("Content-Type", "application/json")
				if _, err := io.WriteString(response, test.document(page)); err != nil {
					t.Errorf("write provider response: %v", err)
				}
			}))
			t.Cleanup(provider.Close)
			providerURL = provider.URL
			client, err := githubsvc.NewClient(provider.Client(), githubsvc.WithBaseURL(provider.URL))
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			router := newGitHubTestRouter(t, client)

			target := test.publicPath
			forward := make([]string, 0, 3)
			var backwardTarget string
			for page := 1; page <= 3; page++ {
				response := performGitHubRequest(t, router, target)
				if response.Code != http.StatusOK {
					t.Fatalf("forward page %d status=%d body=%s", page, response.Code, response.Body.String())
				}
				body := decodeObject(t, response)
				entries := objectArray(t, body, test.bodyMember)
				if len(entries) != 1 || body["count"] != float64(1) {
					t.Fatalf("forward page %d body=%#v", page, body)
				}
				forward = append(forward, stringField(t, firstObject(t, entries), "name"))
				nextTarget := publicLinkTarget(response.Header().Get("Link"), "next")
				previousTarget := publicLinkTarget(response.Header().Get("Link"), "prev")
				if page == 1 && previousTarget != "" || page == 3 && nextTarget != "" {
					t.Fatalf("page %d Link=%q", page, response.Header().Get("Link"))
				}
				for _, publicTarget := range []string{nextTarget, previousTarget} {
					if publicTarget != "" && (!strings.HasPrefix(publicTarget, "/v1/") ||
						!strings.Contains(publicTarget, "limit=1") || !strings.Contains(publicTarget, "cursor=") ||
						strings.Contains(publicTarget, provider.URL) || strings.Contains(publicTarget, "page=")) {
						t.Fatalf("unsafe public target=%q", publicTarget)
					}
				}
				if page < 3 {
					target = nextTarget
				} else {
					backwardTarget = previousTarget
				}
			}
			wantForward := []string{"repo1", "repo2", "repo3"}
			if test.bodyMember == "tags" {
				wantForward = []string{"v1", "v2", "v3"}
			}
			if strings.Join(forward, ",") != strings.Join(wantForward, ",") {
				t.Fatalf("forward=%v want=%v", forward, wantForward)
			}

			backward := make([]string, 0, 2)
			for page := 2; page >= 1; page-- {
				response := performGitHubRequest(t, router, backwardTarget)
				if response.Code != http.StatusOK {
					t.Fatalf("backward page %d status=%d body=%s", page, response.Code, response.Body.String())
				}
				body := decodeObject(t, response)
				backward = append(
					backward,
					stringField(t, firstObject(t, objectArray(t, body, test.bodyMember)), "name"),
				)
				backwardTarget = publicLinkTarget(response.Header().Get("Link"), "prev")
				if page == 1 && backwardTarget != "" {
					t.Fatalf("first page unexpectedly has prev: %s", response.Header().Get("Link"))
				}
			}
			wantBackward := []string{"repo2", "repo1"}
			if test.bodyMember == "tags" {
				wantBackward = []string{"v2", "v1"}
			}
			if strings.Join(backward, ",") != strings.Join(wantBackward, ",") {
				t.Fatalf("backward=%v want=%v", backward, wantBackward)
			}
		})
	}
}

func TestGitHubHandlersRejectInvalidInputsBeforeService(t *testing.T) {
	tests := []struct {
		name, target, code, source, leak string
		status                           int
	}{
		{
			name: "one-character underscore owner", target: "/v1/github/owners/_",
			status: 422, code: "validation_failed", source: "owner",
		},
		{
			name: "owner starts hyphen", target: "/v1/github/owners/-bad",
			status: 422, code: "validation_failed", source: "owner", leak: "-bad",
		},
		{
			name: "owner ends hyphen", target: "/v1/github/owners/bad-",
			status: 422, code: "validation_failed", source: "owner", leak: "bad-",
		},
		{
			name:   "owner too long",
			target: "/v1/github/owners/" + strings.Repeat("a", 40),
			status: 422,
			code:   "validation_failed",
			source: "owner",
			leak:   strings.Repeat("a", 40),
		},
		{
			name: "Unicode owner", target: "/v1/github/owners/octoc%C3%A1t",
			status: 422, code: "validation_failed", source: "owner", leak: "octocát",
		},
		{
			name: "dot repository", target: "/v1/github/repos/octocat/...",
			status: 422, code: "validation_failed", source: "repo", leak: "...",
		},
		{
			name: "repository too long", target: "/v1/github/repos/octocat/" + strings.Repeat("a", 101),
			status: 422, code: "validation_failed", source: "repo", leak: strings.Repeat("a", 101),
		},
		{
			name: "Unicode repository", target: "/v1/github/repos/octocat/r%C3%A9po",
			status: 422, code: "validation_failed", source: "repo", leak: "répo",
		},
		{
			name: "query on owner point read", target: "/v1/github/owners/octocat?limit=1",
			status: 400, code: "invalid_request", leak: "limit",
		},
		{
			name: "query on repository point read", target: "/v1/github/repos/octocat/repo?limit=1",
			status: 400, code: "invalid_request", leak: "limit",
		},
		{
			name: "query on languages point read", target: "/v1/github/repos/octocat/repo/languages?limit=1",
			status: 400, code: "invalid_request", leak: "limit",
		},
		{
			name: "unknown query", target: "/v1/github/owners/octocat/repos?page=2",
			status: 400, code: "invalid_request", leak: "page",
		},
		{
			name:   "repeated query",
			target: "/v1/github/owners/octocat/repos?limit=2&limit=3",
			status: 400,
			code:   "invalid_request",
			leak:   "limit=2",
		},
		{
			name: "malformed query escape", target: "/v1/github/owners/octocat/repos?cursor=%ZZ",
			status: 400, code: "invalid_request", leak: "%ZZ",
		},
		{
			name: "invalid UTF-8 query", target: "/v1/github/owners/octocat/repos?cursor=%FF",
			status: 400, code: "invalid_request",
		},
		{
			name:   "signed limit",
			target: "/v1/github/owners/octocat/repos?limit=%2B1",
			status: 422,
			code:   "validation_failed",
			leak:   "+1",
		},
		{
			name: "fractional limit", target: "/v1/github/repos/octocat/repo/activity?limit=1.5",
			status: 422, code: "validation_failed", leak: "1.5",
		},
		{
			name: "exponent limit", target: "/v1/github/repos/octocat/repo/tags?limit=1e2",
			status: 422, code: "validation_failed", leak: "1e2",
		},
		{
			name: "zero limit", target: "/v1/github/owners/octocat/repos?limit=0",
			status: 422, code: "validation_failed", leak: "0",
		},
		{
			name: "limit maximum plus one", target: "/v1/github/repos/octocat/repo/activity?limit=101",
			status: 422, code: "validation_failed", leak: "101",
		},
		{
			name:   "overflow limit",
			target: "/v1/github/repos/octocat/repo/tags?limit=18446744073709551616",
			status: 422, code: "validation_failed", leak: "18446744073709551616",
		},
		{
			name: "empty cursor", target: "/v1/github/owners/octocat/repos?cursor=",
			status: 400, code: "invalid_request",
		},
		{
			name: "non-ASCII cursor", target: "/v1/github/repos/octocat/repo/activity?cursor=%C3%A9",
			status: 400, code: "invalid_request", leak: "é",
		},
		{
			name:   "oversized cursor",
			target: "/v1/github/repos/octocat/repo/tags?cursor=" + strings.Repeat("a", 2049),
			status: 400, code: "invalid_request", leak: strings.Repeat("a", 128),
		},
		{
			name:   "bad cursor",
			target: "/v1/github/owners/octocat/repos?cursor=not-a-cursor",
			status: 400,
			code:   "invalid_request",
			leak:   "not-a-cursor",
		},
		{
			name: "noncanonical cursor",
			target: "/v1/github/owners/octocat/repos?cursor=" + pagination.NewCursor(
				pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "octocat", Limit: 20},
				"next", "2",
			).Encode() + "%3D",
			status: 400, code: "invalid_request",
		},
		{
			name:   "repeated cursor",
			target: "/v1/github/owners/octocat/repos?cursor=a&cursor=b",
			status: 400, code: "invalid_request", leak: "cursor",
		},
		{
			name: "wrong scope cursor",
			target: "/v1/github/owners/octocat/repos?limit=2&cursor=" + pagination.NewCursor(
				pagination.Scope{Operation: "listGitHubRepositoryTags", Owner: "octocat", Repo: "repo", Limit: 2},
				"next", "2",
			).Encode(),
			status: 400, code: "invalid_request",
		},
		{
			name: "changed owner cursor",
			target: "/v1/github/owners/octocat/repos?limit=2&cursor=" + pagination.NewCursor(
				pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "other", Limit: 2},
				"next", "2",
			).Encode(),
			status: 400, code: "invalid_request",
		},
		{
			name: "changed limit cursor",
			target: "/v1/github/owners/octocat/repos?limit=3&cursor=" + pagination.NewCursor(
				pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "octocat", Limit: 2},
				"next", "2",
			).Encode(),
			status: 400, code: "invalid_request",
		},
		{
			name: "changed activity repository cursor",
			target: "/v1/github/repos/octocat/repo/activity?limit=2&cursor=" + pagination.NewCursor(
				pagination.Scope{
					Operation: "listGitHubRepositoryActivity", Owner: "octocat", Repo: "other", Limit: 2,
				},
				"next", "opaque",
			).Encode(),
			status: 400, code: "invalid_request",
		},
		{
			name: "changed tag repository cursor",
			target: "/v1/github/repos/octocat/repo/tags?limit=2&cursor=" + pagination.NewCursor(
				pagination.Scope{
					Operation: "listGitHubRepositoryTags", Owner: "octocat", Repo: "other", Limit: 2,
				},
				"next", "2",
			).Encode(),
			status: 400, code: "invalid_request",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := githubFixtures()
			response := performGitHubRequest(t, newGitHubTestRouter(t, service), test.target)
			if response.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.status, response.Body.String())
			}
			body := decodeObject(t, response)
			if body["code"] != test.code || test.leak != "" && strings.Contains(response.Body.String(), test.leak) {
				t.Fatalf("problem = %s", response.Body.String())
			}
			if test.source != "" {
				issues := objectArray(t, body, "errors")
				issue := firstObject(t, issues)
				source, ok := issue["source"].(map[string]any)
				if !ok || source["parameter"] != test.source || len(source) != 1 {
					t.Fatalf("unsafe issue source=%#v", issue["source"])
				}
			}
			if len(service.calls) != 0 {
				t.Fatalf("service called: %v", service.calls)
			}
		})
	}
}

func TestGitHubHandlersMapSafeDependencyErrors(t *testing.T) {
	rateLimit := &githubsvc.RateLimitError{RetryAfter: "17", Reset: "2000"}
	tests := []struct {
		name string
		err  error
		want int
		code string
	}{
		{name: "not found", err: githubsvc.ErrNotFound, want: 404, code: "github_not_found"},
		{name: "invalid provider cursor", err: githubsvc.ErrInvalidCursor, want: 400, code: "invalid_request"},
		{name: "rate limited", err: rateLimit, want: 429, code: "github_rate_limit"},
		{name: "upstream", err: githubsvc.ErrUpstream, want: 502, code: "github_upstream"},
		{name: "timeout", err: githubsvc.ErrTimeout, want: 504, code: "github_timeout"},
		{name: "canceled provider", err: context.Canceled, want: 502, code: "github_upstream"},
		{name: "unexpected", err: errors.New("secret provider diagnostic"), want: 500, code: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := githubFixtures()
			service.err = test.err
			response := performGitHubRequest(t, newGitHubTestRouter(t, service), "/v1/github/owners/octocat")
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
			body := decodeObject(t, response)
			if body["code"] != test.code || strings.Contains(response.Body.String(), "secret") ||
				strings.Contains(response.Body.String(), "provider") {
				t.Fatalf("unsafe problem = %s", response.Body.String())
			}
			if test.want == 429 {
				if response.Header().Get("Retry-After") != "17" ||
					response.Header().Get("X-Ratelimit-Reset") != "2000" {
					t.Fatalf("rate headers = %#v", response.Header())
				}
			} else if response.Header().Get("Retry-After") != "" || response.Header().Get("X-Ratelimit-Reset") != "" {
				t.Fatalf("unexpected rate headers = %#v", response.Header())
			}
		})
	}
}

func TestGitHubDependencyFailureLogsRetainUnderlyingDiagnostic(t *testing.T) {
	tests := []struct {
		name, message, category, diagnostic string
		err                                 error
		level                               zapcore.Level
	}{
		{
			name: "timeout", message: "github dependency timed out", category: "github_timeout",
			diagnostic: "deadline sentinel",
			err:        errors.Join(githubsvc.ErrTimeout, errors.New("deadline sentinel")),
			level:      zapcore.WarnLevel,
		},
		{
			name: "upstream", message: "github dependency failed", category: "github_upstream",
			diagnostic: "transport sentinel",
			err:        errors.Join(githubsvc.ErrUpstream, errors.New("transport sentinel")),
			level:      zapcore.WarnLevel,
		},
		{
			name: "unexpected", message: "github operation failed", category: "internal_error",
			diagnostic: "projection sentinel",
			err:        errors.New("projection sentinel"),
			level:      zapcore.ErrorLevel,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			core, recorded := observer.New(zapcore.DebugLevel)
			handler := obs.HTTPRequestContext(obs.HTTPRequestContextConfig{Logger: zap.New(core)})(
				http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
					_ = mapServiceError(request.Context(), "getGitHubOwner", test.err)
				}),
			)
			handler.ServeHTTP(
				httptest.NewRecorder(),
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil),
			)

			entries := recorded.FilterMessage(test.message).All()
			if len(entries) != 1 {
				t.Fatalf("log entries=%d want=1; all=%#v", len(entries), recorded.All())
			}
			entry := entries[0]
			if entry.Level != test.level {
				t.Fatalf("level=%s want=%s", entry.Level, test.level)
			}
			fields := entry.ContextMap()
			if fields["operation"] != "getGitHubOwner" || fields["error_category"] != test.category {
				t.Fatalf("fields=%#v", fields)
			}
			loggedError, ok := fields["error"].(string)
			if !ok || !strings.Contains(loggedError, test.diagnostic) {
				t.Fatalf("error field=%#v want diagnostic %q", fields["error"], test.diagnostic)
			}
		})
	}
}

func TestGitHubSuccessNegotiatesJSONAndCBOR(t *testing.T) {
	for _, mediaType := range []string{"application/json", "application/cbor"} {
		t.Run(mediaType, func(t *testing.T) {
			router := newGitHubTestRouter(t, githubFixtures())
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/github/owners/octocat", nil)
			request.Header.Set("Accept", mediaType)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Header().Get("Content-Type") != mediaType {
				t.Fatalf(
					"status=%d content-type=%q body=%x",
					response.Code,
					response.Header().Get("Content-Type"),
					response.Body.Bytes(),
				)
			}
		})
	}
}

func TestGitHubNotAcceptableUsesIndependentProblemNegotiation(t *testing.T) {
	router := newGitHubTestRouter(t, githubFixtures())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/github/owners/octocat", nil)
	request.Header.Set("Accept", "text/plain")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotAcceptable ||
		response.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf(
			"status=%d content-type=%q body=%s",
			response.Code,
			response.Header().Get("Content-Type"),
			response.Body.String(),
		)
	}
	if body := decodeObject(t, response); body["code"] != "not_acceptable" {
		t.Fatalf("problem = %#v", body)
	}
}
