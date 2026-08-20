package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/janisto/huma-playground/internal/platform/pagination"
)

const (
	ownerFixture      = `{"id":1,"login":"octocat","type":"User","name":"The Octocat","avatar_url":"https://avatars.example/1","html_url":"https://github.com/octocat","company":null,"blog":"","location":"Earth","bio":null,"public_repos":8,"followers":10,"following":2,"created_at":"2011-01-25T18:44:36Z","updated_at":"2024-06-01T00:00:00.123Z"}`
	repositoryFixture = `{"id":2,"name":"repo","full_name":"octocat/repo","description":null,"html_url":"https://github.com/octocat/repo","fork":false,"private":false,"visibility":"public","language":null,"stargazers_count":3,"forks_count":4,"open_issues_count":5,"archived":false,"created_at":"2020-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00.000Z","pushed_at":null,"default_branch":"main","license":{"spdx_id":"MIT"},"topics":["zeta","alpha"],"disabled":false}`
)

func newTestClient(t *testing.T, handler http.Handler, options ...Option) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	options = append([]Option{WithBaseURL(server.URL)}, options...)
	client, err := NewClient(server.Client(), options...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client, server
}

func writeJSON(t *testing.T, response http.ResponseWriter, document string) {
	t.Helper()
	response.Header().Set("Content-Type", "application/json")
	if _, err := io.WriteString(response, document); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

func assertProviderRequest(t *testing.T, request *http.Request) {
	t.Helper()
	want := map[string]string{
		"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2026-03-10",
		"User-Agent": "huma-playground", "Accept-Encoding": "identity",
	}
	for name, value := range want {
		if values := request.Header.Values(name); len(values) != 1 || values[0] != value {
			t.Errorf("%s = %q, want exactly %q", name, values, value)
		}
	}
	for _, name := range []string{"Authorization", "Cookie", "X-Request-ID", "Forwarded", "X-Forwarded-For"} {
		if values := request.Header.Values(name); len(values) != 0 {
			t.Errorf("forbidden outbound %s = %q", name, values)
		}
	}
}

type trackedResponseBody struct {
	reader    *strings.Reader
	reads     int
	bytesRead int
	closed    bool
}

func newTrackedResponseBody(document string) *trackedResponseBody {
	return &trackedResponseBody{reader: strings.NewReader(document)}
}

func (body *trackedResponseBody) Read(buffer []byte) (int, error) {
	body.reads++
	count, err := body.reader.Read(buffer)
	body.bytesRead += count
	return count, err
}

func (body *trackedResponseBody) Close() error {
	body.closed = true
	return nil
}

func TestClientOwnerProjectionAndTransportHeaders(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assertProviderRequest(t, request)
		if request.Method != http.MethodGet || request.URL.RequestURI() != "/users/octocat" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.RequestURI())
		}
		writeJSON(t, response, ownerFixture)
	}))

	owner, err := client.GetOwner(t.Context(), "octocat")
	if err != nil {
		t.Fatalf("GetOwner: %v", err)
	}
	if owner.ID != 1 || owner.Login != "octocat" || owner.Name == nil || *owner.Name != "The Octocat" ||
		owner.Company != nil || owner.Blog != nil || owner.Location == nil || *owner.Location != "Earth" || owner.Bio != nil {
		t.Fatalf("unexpected owner projection: %#v", owner)
	}
	if got := owner.UpdatedAt.Format("2006-01-02T15:04:05.000Z"); got != "2024-06-01T00:00:00.123Z" {
		t.Fatalf("updated time = %q", got)
	}
}

func TestClientRepositoryOperationsProjectExactPublicData(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assertProviderRequest(t, request)
		switch request.URL.Path {
		case "/repos/octocat/repo":
			writeJSON(t, response, repositoryFixture)
		case "/repos/octocat/repo/languages":
			writeJSON(t, response, `{"Zig":5,"\uE000":7,"\ud800\udc00":7,"Go":10}`)
		case "/repos/octocat/repo/tags":
			if request.URL.Query().Get("per_page") != "2" {
				t.Errorf("per_page = %q", request.URL.Query().Get("per_page"))
			}
			writeJSON(
				t,
				response,
				`[{"name":"v1","commit":{"sha":"0123456789abcdef0123456789abcdef01234567"}},{"name":"v2","commit":{"sha":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}]`,
			)
		default:
			http.NotFound(response, request)
		}
	}))

	repository, err := client.GetRepository(t.Context(), "octocat", "repo")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if repository.Description != nil || repository.Language != nil || repository.PushedAt != nil ||
		repository.License == nil || *repository.License != "MIT" || strings.Join(repository.Topics, ",") != "alpha,zeta" {
		t.Fatalf("unexpected repository projection: %#v", repository)
	}
	languages, err := client.ListRepositoryLanguages(t.Context(), "octocat", "repo")
	if err != nil {
		t.Fatalf("ListRepositoryLanguages: %v", err)
	}
	if len(languages) != 4 || languages[0].Name != "Go" || languages[1].Name != "" ||
		languages[2].Name != "𐀀" || languages[3].Name != "Zig" {
		t.Fatalf("language order = %#v", languages)
	}
	tags, err := client.ListRepositoryTags(t.Context(), "octocat", "repo", 2, nil)
	if err != nil || len(tags.Entries) != 2 || len(tags.Entries[0].SHA) != 40 || len(tags.Entries[1].SHA) != 64 {
		t.Fatalf("tags = %#v, err = %v", tags, err)
	}
}

func TestClientPreservesRepositoryEmptyNullableStrings(t *testing.T) {
	document := strings.Replace(repositoryFixture, `"description":null`, `"description":""`, 1)
	document = strings.Replace(document, `"language":null`, `"language":""`, 1)
	document = strings.Replace(document, `"spdx_id":"MIT"`, `"spdx_id":""`, 1)
	client, _ := newTestClient(
		t,
		http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { writeJSON(t, response, document) }),
	)
	repository, err := client.GetRepository(t.Context(), "octocat", "repo")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if repository.Description == nil || *repository.Description != "" ||
		repository.Language == nil || *repository.Language != "" || repository.License != nil {
		t.Fatalf("empty nullable projection=%#v", repository)
	}
}

func TestClientRepositoryProjectionHandlesAbsentAndScalarOrderedTopics(t *testing.T) {
	t.Run("absent topics", func(t *testing.T) {
		document := strings.Replace(repositoryFixture, `,"topics":["zeta","alpha"]`, "", 1)
		client, _ := newTestClient(
			t,
			http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { writeJSON(t, response, document) }),
		)
		repository, err := client.GetRepository(t.Context(), "octocat", "repo")
		if err != nil || repository.Topics == nil || len(repository.Topics) != 0 {
			t.Fatalf("topics=%#v err=%v", repository.Topics, err)
		}
	})

	t.Run("Unicode scalar order", func(t *testing.T) {
		document := strings.Replace(
			repositoryFixture,
			`"topics":["zeta","alpha"]`,
			`"topics":["\ud800\udc00","\uE000"]`,
			1,
		)
		client, _ := newTestClient(
			t,
			http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { writeJSON(t, response, document) }),
		)
		repository, err := client.GetRepository(t.Context(), "octocat", "repo")
		if err != nil || len(repository.Topics) != 2 || repository.Topics[0] != "" || repository.Topics[1] != "𐀀" {
			t.Fatalf("topics=%#v err=%v", repository.Topics, err)
		}
	})

	t.Run("empty languages", func(t *testing.T) {
		client, _ := newTestClient(
			t,
			http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { writeJSON(t, response, `{}`) }),
		)
		languages, err := client.ListRepositoryLanguages(t.Context(), "octocat", "repo")
		if err != nil || languages == nil || len(languages) != 0 {
			t.Fatalf("languages=%#v err=%v", languages, err)
		}
	})
}

func TestClientRepositoryListAndCursors(t *testing.T) {
	var serverURL string
	client, server := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assertProviderRequest(t, request)
		query := request.URL.Query()
		if query.Get("type") != "owner" || query.Get("sort") != "full_name" || query.Get("direction") != "asc" ||
			query.Get("per_page") != "1" {
			t.Errorf("fixed query lost: %s", request.URL.RawQuery)
		}
		switch query.Get("page") {
		case "":
			response.Header().
				Set("Link", "<"+serverURL+"/user/42/repos?direction=asc&page=2&per_page=1&sort=full_name&type=owner>; rel=\"next\"")
		case "2":
			response.Header().
				Set("Link", "<"+serverURL+"/users/octocat/repos?direction=asc&page=1&per_page=1&sort=full_name&type=owner>; rel=\"prev\"")
		default:
			t.Errorf("unexpected page %q", query.Get("page"))
		}
		writeJSON(
			t,
			response,
			`[{"id":2,"name":"repo","full_name":"octocat/repo","description":"demo","html_url":"https://github.com/octocat/repo","fork":false,"private":false,"visibility":"public"}]`,
		)
	}))
	serverURL = server.URL
	link := "<" + serverURL + "/user/42/repos?direction=asc&page=2&per_page=1&sort=full_name&type=owner>; rel=\"next\""
	scope := pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "octocat", Limit: 1}
	spec := client.pageSpec("/users/octocat/repos", "/user/", "/repos", url.Values{
		"type": {"owner"}, "sort": {"full_name"}, "direction": {"asc"}, "per_page": {"1"},
	}, numberedPagination, "", scope)
	if _, err := parseNavigation(http.Header{"Link": {link}}, spec, false); err != nil {
		t.Fatalf("test Link is invalid: %v", err)
	}

	first, err := client.ListOwnerRepositories(t.Context(), "octocat", 1, nil)
	if err != nil || len(first.Entries) != 1 || first.NextCursor == "" || first.PrevCursor != "" {
		t.Fatalf("first page = %#v, err = %v", first, err)
	}
	cursor, err := pagination.DecodeCursor(first.NextCursor)
	if err != nil || cursor.Operation != "listGitHubOwnerRepositories" || cursor.Owner != "octocat" ||
		cursor.Limit != 1 ||
		cursor.Anchor != "2" {
		t.Fatalf("next cursor = %#v, err = %v", cursor, err)
	}
	second, err := client.ListOwnerRepositories(t.Context(), "octocat", 1, &cursor)
	if err != nil || second.NextCursor != "" || second.PrevCursor == "" {
		t.Fatalf("second page = %#v, err = %v", second, err)
	}
}

func TestClientActivitySupportsDeletedActorsAndOpaqueNavigation(t *testing.T) {
	var serverURL string
	client, server := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("direction") != "desc" || request.URL.Query().Get("per_page") != "2" {
			t.Errorf("unexpected activity query: %s", request.URL.RawQuery)
		}
		response.Header().
			Set("Link", "<"+serverURL+"/repositories/42/activity?after=opaque-token&direction=desc&per_page=2>; rel=\"next\"")
		writeJSON(
			t,
			response,
			`[{"id":2,"actor":{"login":"octocat","avatar_url":"https://avatars.example/1"},"ref":"refs/heads/dev","timestamp":"2024-01-01T00:00:00.001Z","activity_type":"force_push"},{"id":1,"actor":null,"ref":"refs/heads/main","timestamp":"2024-01-01T00:00:00Z","activity_type":"push"}]`,
		)
	}))
	serverURL = server.URL

	page, err := client.ListRepositoryActivity(t.Context(), "octocat", "repo", 2, nil)
	if err != nil || len(page.Entries) != 2 || page.Entries[0].Actor == nil ||
		page.Entries[0].Timestamp.Before(page.Entries[1].Timestamp) || page.Entries[1].Actor != nil ||
		page.Entries[1].ActorAvatarURL != nil {
		t.Fatalf("activity page = %#v, err = %v", page, err)
	}
	cursor, err := pagination.DecodeCursor(page.NextCursor)
	if err != nil || cursor.Anchor != "opaque-token" || cursor.Repo != "repo" {
		t.Fatalf("activity cursor = %#v, err = %v", cursor, err)
	}
}

func TestClientUsesLocallyValidExternalCursorAfterProviderMutation(t *testing.T) {
	tests := []struct {
		name, expectedPath, expectedMember, expectedValue string
		invoke                                            func(*Client) (string, string, error)
		link                                              func(string) string
	}{
		{
			name: "owner repositories", expectedPath: "/users/octocat/repos",
			expectedMember: "page", expectedValue: "7",
			invoke: func(client *Client) (string, string, error) {
				cursor := pagination.NewCursor(
					pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "octocat", Limit: 2},
					"next", "7",
				)
				page, err := client.ListOwnerRepositories(t.Context(), "octocat", 2, &cursor)
				return page.NextCursor, page.PrevCursor, err
			},
			link: func(origin string) string {
				return "<" + origin +
					`/user/42/repos?direction=asc&page=2&per_page=2&sort=full_name&type=owner>; rel="prev"`
			},
		},
		{
			name: "repository tags", expectedPath: "/repos/octocat/repo/tags",
			expectedMember: "page", expectedValue: "7",
			invoke: func(client *Client) (string, string, error) {
				cursor := pagination.NewCursor(
					pagination.Scope{
						Operation: "listGitHubRepositoryTags", Owner: "octocat", Repo: "repo", Limit: 2,
					},
					"next", "7",
				)
				page, err := client.ListRepositoryTags(t.Context(), "octocat", "repo", 2, &cursor)
				return page.NextCursor, page.PrevCursor, err
			},
			link: func(origin string) string {
				return "<" + origin + `/repositories/42/tags?page=2&per_page=2>; rel="prev"`
			},
		},
		{
			name: "repository activity", expectedPath: "/repos/octocat/repo/activity",
			expectedMember: "after", expectedValue: "old-provider-state",
			invoke: func(client *Client) (string, string, error) {
				cursor := pagination.NewCursor(
					pagination.Scope{
						Operation: "listGitHubRepositoryActivity", Owner: "octocat", Repo: "repo", Limit: 2,
					},
					"next", "old-provider-state",
				)
				page, err := client.ListRepositoryActivity(t.Context(), "octocat", "repo", 2, &cursor)
				return page.NextCursor, page.PrevCursor, err
			},
			link: func(origin string) string {
				return "<" + origin +
					`/repositories/42/activity?before=newer-provider-state&direction=desc&per_page=2>; rel="prev"`
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var serverURL string
			client, server := newTestClient(
				t,
				http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
					if request.URL.Path != test.expectedPath ||
						request.URL.Query().Get(test.expectedMember) != test.expectedValue {
						t.Errorf("provider request=%s want path=%s %s=%s",
							request.URL.RequestURI(), test.expectedPath, test.expectedMember, test.expectedValue)
					}
					otherMember := "before"
					if test.expectedMember == "before" {
						otherMember = "after"
					}
					if test.expectedMember != "page" && request.URL.Query().Get(otherMember) != "" {
						t.Errorf("provider request unexpectedly includes %s", otherMember)
					}
					response.Header().Set("Link", test.link(serverURL))
					writeJSON(t, response, `[]`)
				}),
			)
			serverURL = server.URL
			next, previous, err := test.invoke(client)
			if err != nil || next != "" || previous == "" {
				t.Fatalf("next=%q prev=%q err=%v", next, previous, err)
			}
		})
	}
}

func TestClientActivityCursorDirectionSelectsOneProviderMember(t *testing.T) {
	for _, direction := range []string{"next", "prev"} {
		t.Run(direction, func(t *testing.T) {
			client, _ := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				selected := "after"
				rejected := "before"
				if direction == "prev" {
					selected, rejected = rejected, selected
				}
				if request.URL.Query().Get(selected) != "opaque-state" || request.URL.Query().Get(rejected) != "" {
					t.Errorf("query=%s", request.URL.RawQuery)
				}
				writeJSON(t, response, `[]`)
			}))
			cursor := pagination.NewCursor(
				pagination.Scope{
					Operation: "listGitHubRepositoryActivity", Owner: "octocat", Repo: "repo", Limit: 20,
				},
				direction, "opaque-state",
			)
			if _, err := client.ListRepositoryActivity(t.Context(), "octocat", "repo", 20, &cursor); err != nil {
				t.Fatalf("ListRepositoryActivity: %v", err)
			}
		})
	}
}

func TestClientRejectsInvalidProjectionAndProviderResponses(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		headers     http.Header
		document    string
		contentType string
	}{
		{name: "missing required member", document: strings.Replace(ownerFixture, `"login":"octocat",`, "", 1)},
		{name: "wrong required type", document: strings.Replace(ownerFixture, `"login":"octocat"`, `"login":7`, 1)},
		{name: "duplicate member", document: strings.Replace(ownerFixture, `"id":1`, `"id":1,"id":2`, 1)},
		{name: "trailing JSON", document: ownerFixture + `{}`},
		{name: "negative integer", document: strings.Replace(ownerFixture, `"id":1`, `"id":-1`, 1)},
		{name: "unsafe integer", document: strings.Replace(ownerFixture, `"id":1`, `"id":9007199254740992`, 1)},
		{
			name: "invalid URL", document: strings.Replace(
				ownerFixture, `"avatar_url":"https://avatars.example/1"`, `"avatar_url":"mailto:test@example.com"`, 1,
			),
		},
		{
			name: "invalid calendar time", document: strings.Replace(
				ownerFixture, `"created_at":"2011-01-25T18:44:36Z"`, `"created_at":"2011-02-30T18:44:36Z"`, 1,
			),
		},
		{name: "sub millisecond time", document: strings.Replace(ownerFixture, `.123Z`, `.1234Z`, 1)},
		{name: "compressed", headers: http.Header{"Content-Encoding": {"gzip"}}, document: ownerFixture},
		{name: "wrong content type", contentType: "text/plain", document: ownerFixture},
		{
			name:     "multiple content types",
			headers:  http.Header{"Content-Type": {"application/json", "application/problem+json"}},
			document: ownerFixture,
		},
		{name: "unexpected status", status: http.StatusCreated, document: ownerFixture},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, _ := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				for name, values := range test.headers {
					for _, value := range values {
						response.Header().Add(name, value)
					}
				}
				if response.Header().Get("Content-Type") == "" {
					contentType := test.contentType
					if contentType == "" {
						contentType = "application/json"
					}
					response.Header().Set("Content-Type", contentType)
				}
				status := test.status
				if status == 0 {
					status = http.StatusOK
				}
				response.WriteHeader(status)
				_, _ = io.WriteString(response, test.document)
			}))
			_, err := client.GetOwner(t.Context(), "octocat")
			if !errors.Is(err, ErrUpstream) {
				t.Fatalf("error = %v, want ErrUpstream", err)
			}
		})
	}
}

func TestClientRejectsInconsistentActivityActorData(t *testing.T) {
	valid := `{"id":1,"actor":{"login":"octocat","avatar_url":"https://avatars.example/1"},"ref":"refs/heads/main","timestamp":"2024-01-01T00:00:00Z","activity_type":"push"}`
	for _, test := range []struct {
		name, document string
	}{
		{name: "missing actor", document: strings.Replace(valid, `"actor":{"login":"octocat","avatar_url":"https://avatars.example/1"},`, "", 1)},
		{name: "actor wrong type", document: strings.Replace(valid, `{"login":"octocat","avatar_url":"https://avatars.example/1"}`, `"octocat"`, 1)},
		{name: "missing login", document: strings.Replace(valid, `"login":"octocat",`, "", 1)},
		{name: "empty login", document: strings.Replace(valid, `"login":"octocat"`, `"login":""`, 1)},
		{name: "missing avatar", document: strings.Replace(valid, `,"avatar_url":"https://avatars.example/1"`, "", 1)},
		{name: "invalid avatar", document: strings.Replace(valid, `https://avatars.example/1`, `data:text/plain,private`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, _ := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				writeJSON(t, response, "["+test.document+"]")
			}))
			if _, err := client.ListRepositoryActivity(
				t.Context(),
				"octocat",
				"repo",
				1,
				nil,
			); !errors.Is(
				err,
				ErrUpstream,
			) {
				t.Fatalf("error=%v want ErrUpstream", err)
			}
		})
	}
}

func TestClientIgnoresUnrelatedAdditiveProviderMembers(t *testing.T) {
	document := strings.Replace(
		ownerFixture,
		`"id":1`,
		`"id":1,"email":"must-not-project@example.com","future":{"token":"must-not-project"}`,
		1,
	)
	client, _ := newTestClient(
		t,
		http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { writeJSON(t, response, document) }),
	)
	owner, err := client.GetOwner(t.Context(), "octocat")
	if err != nil || owner.ID != 1 || owner.Login != "octocat" {
		t.Fatalf("owner=%#v err=%v", owner, err)
	}
}

func TestClientRejectsPrivateRepositoriesAndOversizedPages(t *testing.T) {
	for _, test := range []struct {
		name, document string
	}{
		{name: "private repository", document: strings.Replace(repositoryFixture, `"private":false`, `"private":true`, 1)},
		{name: "missing private", document: strings.Replace(repositoryFixture, `,"private":false`, "", 1)},
		{name: "wrong private type", document: strings.Replace(repositoryFixture, `"private":false`, `"private":"false"`, 1)},
		{name: "missing visibility", document: strings.Replace(repositoryFixture, `,"visibility":"public"`, "", 1)},
		{name: "non-public visibility", document: strings.Replace(repositoryFixture, `"visibility":"public"`, `"visibility":"internal"`, 1)},
		{name: "duplicate topic", document: strings.Replace(repositoryFixture, `["zeta","alpha"]`, `["alpha","alpha"]`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, _ := newTestClient(
				t,
				http.HandlerFunc(
					func(response http.ResponseWriter, _ *http.Request) { writeJSON(t, response, test.document) },
				),
			)
			_, err := client.GetRepository(t.Context(), "octocat", "repo")
			if !errors.Is(err, ErrUpstream) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	repositoryEntry := `{"id":2,"name":"repo","full_name":"octocat/repo","description":null,"html_url":"https://github.com/octocat/repo","fork":false,"private":false,"visibility":"public"}`
	activityEntry := `{"id":1,"actor":null,"ref":"refs/heads/main","timestamp":"2024-01-01T00:00:00Z","activity_type":"push"}`
	tagEntry := `{"name":"v1","commit":{"sha":"0123456789abcdef0123456789abcdef01234567"}}`
	for _, test := range []struct {
		name, path, entry string
		invoke            func(*Client) error
	}{
		{
			name: "repositories", path: "/users/octocat/repos", entry: repositoryEntry,
			invoke: func(client *Client) error {
				_, err := client.ListOwnerRepositories(t.Context(), "octocat", 1, nil)
				return err
			},
		},
		{
			name: "activity", path: "/repos/octocat/repo/activity", entry: activityEntry,
			invoke: func(client *Client) error {
				_, err := client.ListRepositoryActivity(t.Context(), "octocat", "repo", 1, nil)
				return err
			},
		},
		{
			name: "tags", path: "/repos/octocat/repo/tags", entry: tagEntry,
			invoke: func(client *Client) error {
				_, err := client.ListRepositoryTags(t.Context(), "octocat", "repo", 1, nil)
				return err
			},
		},
	} {
		t.Run("over-limit "+test.name, func(t *testing.T) {
			client, _ := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.path {
					t.Errorf("path=%q want=%q", request.URL.Path, test.path)
				}
				writeJSON(t, response, "["+test.entry+","+test.entry+"]")
			}))
			if err := test.invoke(client); !errors.Is(err, ErrUpstream) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestClientResponseSizeBoundary(t *testing.T) {
	tests := []struct {
		name          string
		size          int
		contentLength int64
		wantAccepted  bool
		wantRead      int
	}{
		{
			name: "exact missing length", size: maximumProviderBody, contentLength: -1,
			wantAccepted: true, wantRead: maximumProviderBody,
		},
		{
			name: "exact truthful length", size: maximumProviderBody, contentLength: maximumProviderBody,
			wantAccepted: true, wantRead: maximumProviderBody,
		},
		{
			name: "exact misleading small length", size: maximumProviderBody, contentLength: 1,
			wantAccepted: true, wantRead: maximumProviderBody,
		},
		{
			name: "exact misleading large length", size: maximumProviderBody,
			contentLength: maximumProviderBody + 1, wantRead: 0,
		},
		{
			name: "over missing length", size: maximumProviderBody + 1, contentLength: -1,
			wantRead: maximumProviderBody + 1,
		},
		{
			name: "over truthful length", size: maximumProviderBody + 1,
			contentLength: maximumProviderBody + 1, wantRead: 0,
		},
		{
			name: "over misleading small length", size: maximumProviderBody + 1, contentLength: 1,
			wantRead: maximumProviderBody + 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prefix := strings.TrimSuffix(ownerFixture, "}") + `,"padding":"`
			document := prefix + strings.Repeat("x", test.size-len(prefix)-2) + `"}`
			if len(document) != test.size {
				t.Fatalf("fixture size=%d want=%d", len(document), test.size)
			}
			body := newTrackedResponseBody(document)
			client, err := NewClient(
				&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					headers := http.Header{"Content-Type": {"application/json"}}
					if test.contentLength >= 0 {
						headers.Set("Content-Length", strconv.FormatInt(test.contentLength, 10))
					}
					return &http.Response{
						StatusCode: http.StatusOK, Header: headers, Body: body,
						ContentLength: test.contentLength, Request: request,
					}, nil
				})},
				WithBaseURL("https://api.github.test"),
			)
			if err != nil {
				t.Fatal(err)
			}
			owner, err := client.GetOwner(t.Context(), "octocat")
			if test.wantAccepted {
				if err != nil || owner.ID != 1 {
					t.Fatalf("owner=%#v err=%v", owner, err)
				}
			} else if !errors.Is(err, ErrUpstream) {
				t.Fatalf("error=%v want ErrUpstream", err)
			}
			if body.bytesRead != test.wantRead {
				t.Fatalf("response body bytes read=%d want=%d", body.bytesRead, test.wantRead)
			}
			if !body.closed {
				t.Fatal("response body was not closed")
			}
		})
	}
}

func TestClientDoesNotReadRedirectOrMappedErrorBodies(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		encoding string
		wantRate bool
		wantNot  bool
	}{
		{name: "not found", status: http.StatusNotFound, wantNot: true},
		{name: "forbidden quota", status: http.StatusForbidden, wantRate: true},
		{name: "rate limit", status: http.StatusTooManyRequests, wantRate: true},
		{name: "unexpected success", status: http.StatusCreated},
		{name: "unauthorized", status: http.StatusUnauthorized},
		{name: "gone", status: http.StatusGone},
		{name: "unprocessable", status: http.StatusUnprocessableEntity},
		{name: "server error", status: http.StatusInternalServerError},
		{name: "unavailable", status: http.StatusServiceUnavailable},
		{name: "encoding rejected before rate mapping", status: http.StatusTooManyRequests, encoding: "gzip"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := newTrackedResponseBody(`{"secret":"must not be parsed"}`)
			client, err := NewClient(
				&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					headers := http.Header{"Retry-After": {"7"}}
					if test.encoding != "" {
						headers.Set("Content-Encoding", test.encoding)
					}
					return &http.Response{
						StatusCode: test.status, Header: headers, Body: body,
						ContentLength: int64(body.reader.Len()), Request: request,
					}, nil
				})},
				WithBaseURL("https://api.github.test"),
			)
			if err != nil {
				t.Fatal(err)
			}
			_, requestErr := client.GetOwner(t.Context(), "octocat")
			var rateLimit *RateLimitError
			switch {
			case test.wantRate && !errors.As(requestErr, &rateLimit):
				t.Fatalf("error=%v want RateLimitError", requestErr)
			case test.wantNot && !errors.Is(requestErr, ErrNotFound):
				t.Fatalf("error=%v want ErrNotFound", requestErr)
			case !test.wantRate && !test.wantNot && !errors.Is(requestErr, ErrUpstream):
				t.Fatalf("error=%v want ErrUpstream", requestErr)
			}
			if body.reads != 0 || body.bytesRead != 0 || !body.closed {
				t.Fatalf("mapped response body reads=%d bytes=%d closed=%t", body.reads, body.bytesRead, body.closed)
			}
		})
	}

	t.Run("redirect", func(t *testing.T) {
		redirectBody := newTrackedResponseBody(`{"secret":"redirect body"}`)
		successBody := newTrackedResponseBody(ownerFixture)
		calls := 0
		client, err := NewClient(
			&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{
						StatusCode: http.StatusMovedPermanently,
						Header:     http.Header{"Location": {"https://api.github.test/user/42"}},
						Body:       redirectBody, ContentLength: int64(redirectBody.reader.Len()), Request: request,
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
					Body: successBody, ContentLength: int64(len(ownerFixture)), Request: request,
				}, nil
			})},
			WithBaseURL("https://api.github.test"),
		)
		if err != nil {
			t.Fatal(err)
		}
		owner, requestErr := client.GetOwner(t.Context(), "octocat")
		if requestErr != nil || owner.ID != 1 || calls != 2 {
			t.Fatalf("owner=%#v err=%v calls=%d", owner, requestErr, calls)
		}
		if redirectBody.reads != 0 || redirectBody.bytesRead != 0 || !redirectBody.closed {
			t.Fatalf("redirect body reads=%d bytes=%d closed=%t",
				redirectBody.reads, redirectBody.bytesRead, redirectBody.closed)
		}
	})
}

func TestClientStatusAndRateLimitMapping(t *testing.T) {
	now := time.Unix(2_000, 250_000_000)
	maximum := strconv.FormatUint(maximumSafeInteger, 10)
	maximumPlusOne := strconv.FormatUint(maximumSafeInteger+1, 10)
	tests := []struct {
		name                 string
		status               int
		retry, reset         []string
		wantRetry, wantReset string
	}{
		{
			name: "usable both prefers retry and exposes reset", status: 429,
			retry: []string{"7"}, reset: []string{"2010"}, wantRetry: "7", wantReset: "2010",
		},
		{
			name: "valid retry with invalid reset", status: 403,
			retry: []string{"7"}, reset: []string{"02010"}, wantRetry: "7",
		},
		{
			name: "invalid retry with valid reset", status: 429,
			retry: []string{"07"}, reset: []string{"2002"}, wantRetry: "2", wantReset: "2002",
		},
		{name: "zero retry", status: 429, retry: []string{"0"}, wantRetry: "0"},
		{name: "maximum retry", status: 403, retry: []string{maximum}, wantRetry: maximum},
		{name: "maximum plus one retry", status: 429, retry: []string{maximumPlusOne}, wantRetry: "60"},
		{
			name: "overflow retry", status: 429, retry: []string{"18446744073709551616"}, wantRetry: "60",
		},
		{name: "repeated retry", status: 429, retry: []string{"1", "2"}, wantRetry: "60"},
		{name: "comma retry", status: 403, retry: []string{"1,2"}, wantRetry: "60"},
		{name: "reset at now", status: 429, reset: []string{"2000"}, wantRetry: "60"},
		{
			name:      "quarter-second reset rounds up",
			status:    403,
			reset:     []string{"2002"},
			wantRetry: "2",
			wantReset: "2002",
		},
		{
			name: "maximum reset remains exact", status: 429, reset: []string{maximum},
			wantRetry: strconv.FormatUint(maximumSafeInteger-2_000, 10), wantReset: maximum,
		},
		{
			name: "valid retry ignores repeated reset", status: 429,
			retry: []string{"8"}, reset: []string{"2002", "2003"}, wantRetry: "8",
		},
		{name: "missing hints", status: 403, wantRetry: "60"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, _ := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				for _, value := range test.retry {
					response.Header().Add("Retry-After", value)
				}
				for _, value := range test.reset {
					response.Header().Add("X-Ratelimit-Reset", value)
				}
				response.WriteHeader(test.status)
			}), WithClock(func() time.Time { return now }))
			_, err := client.GetOwner(t.Context(), "octocat")
			var rateLimit *RateLimitError
			if !errors.As(err, &rateLimit) || rateLimit.RetryAfter != test.wantRetry ||
				rateLimit.Reset != test.wantReset {
				t.Fatalf("rate limit = %#v, err = %v", rateLimit, err)
			}
		})
	}

	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		client, _ := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.Header().Set("Retry-After", "7")
			response.Header().Set("X-Ratelimit-Reset", "2010")
			response.WriteHeader(status)
		}))
		_, err := client.GetOwner(t.Context(), "octocat")
		var rateLimit *RateLimitError
		if errors.As(err, &rateLimit) || status == http.StatusNotFound && !errors.Is(err, ErrNotFound) ||
			status == http.StatusInternalServerError && !errors.Is(err, ErrUpstream) {
			t.Fatalf("status=%d error=%v rateLimit=%#v", status, err, rateLimit)
		}
	}
}

func TestClientFollowsEveryExactNamedToNumericRedirect(t *testing.T) {
	tests := []struct {
		name, namedPath, numericPath, rawQuery, document string
		status                                           int
		invoke                                           func(*Client) error
	}{
		{
			name: "owner", namedPath: "/users/octocat", numericPath: "/user/42",
			status: http.StatusMovedPermanently, document: ownerFixture,
			invoke: func(client *Client) error {
				_, err := client.GetOwner(t.Context(), "octocat")
				return err
			},
		},
		{
			name: "owner repositories", namedPath: "/users/octocat/repos", numericPath: "/user/42/repos",
			rawQuery: "direction=asc&per_page=20&sort=full_name&type=owner",
			status:   http.StatusFound, document: `[]`,
			invoke: func(client *Client) error {
				_, err := client.ListOwnerRepositories(t.Context(), "octocat", 20, nil)
				return err
			},
		},
		{
			name: "repository", namedPath: "/repos/octocat/repo", numericPath: "/repositories/42",
			status: http.StatusSeeOther, document: repositoryFixture,
			invoke: func(client *Client) error {
				_, err := client.GetRepository(t.Context(), "octocat", "repo")
				return err
			},
		},
		{
			name: "activity", namedPath: "/repos/octocat/repo/activity", numericPath: "/repositories/42/activity",
			rawQuery: "direction=desc&per_page=20", status: http.StatusTemporaryRedirect, document: `[]`,
			invoke: func(client *Client) error {
				_, err := client.ListRepositoryActivity(t.Context(), "octocat", "repo", 20, nil)
				return err
			},
		},
		{
			name: "languages", namedPath: "/repos/octocat/repo/languages",
			numericPath: "/repositories/42/languages", status: http.StatusPermanentRedirect, document: `{}`,
			invoke: func(client *Client) error {
				_, err := client.ListRepositoryLanguages(t.Context(), "octocat", "repo")
				return err
			},
		},
		{
			name: "tags", namedPath: "/repos/octocat/repo/tags", numericPath: "/repositories/42/tags",
			rawQuery: "per_page=20", status: http.StatusMovedPermanently, document: `[]`,
			invoke: func(client *Client) error {
				_, err := client.ListRepositoryTags(t.Context(), "octocat", "repo", 20, nil)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			redirectBody := newTrackedResponseBody(`{"ignored":"redirect"}`)
			calls := 0
			client, err := NewClient(
				&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					calls++
					assertProviderRequest(t, request)
					if request.Method != http.MethodGet || request.URL.RawQuery != test.rawQuery {
						t.Errorf("request %s %s?%s", request.Method, request.URL.Path, request.URL.RawQuery)
					}
					if calls == 1 {
						if request.URL.Path != test.namedPath {
							t.Errorf("named path=%q want=%q", request.URL.Path, test.namedPath)
						}
						location := test.numericPath
						if test.rawQuery != "" {
							location += "?" + test.rawQuery
						}
						return &http.Response{
							StatusCode: test.status, Header: http.Header{"Location": {location}},
							Body: redirectBody, ContentLength: int64(redirectBody.reader.Len()), Request: request,
						}, nil
					}
					if request.URL.Path != test.numericPath {
						t.Errorf("numeric path=%q want=%q", request.URL.Path, test.numericPath)
					}
					return &http.Response{
						StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
						Body: io.NopCloser(strings.NewReader(test.document)), ContentLength: int64(len(test.document)),
						Request: request,
					}, nil
				})},
				WithBaseURL("https://api.github.test"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := test.invoke(client); err != nil || calls != 2 {
				t.Fatalf("redirect error=%v calls=%d", err, calls)
			}
			if redirectBody.reads != 0 || redirectBody.bytesRead != 0 || !redirectBody.closed {
				t.Fatalf("redirect body reads=%d bytes=%d closed=%t",
					redirectBody.reads, redirectBody.bytesRead, redirectBody.closed)
			}
		})
	}
}

func TestClientRejectsUnsafeRedirectSequences(t *testing.T) {
	tests := []struct {
		name      string
		locations []string
	}{
		{name: "cross origin", locations: []string{"https://evil.example/user/42"}},
		{name: "missing location", locations: []string{""}},
		{name: "invalid location", locations: []string{"%"}},
		{name: "wrong path", locations: []string{"/users/other"}},
		{name: "unexpected query", locations: []string{"/user/42?extra=true"}},
		{name: "loop", locations: []string{"/user/42", "/users/octocat"}},
		{name: "fourth redirect", locations: []string{"/user/1", "/user/2", "/user/3", "/user/4"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bodies := make([]*trackedResponseBody, 0, len(test.locations))
			calls := 0
			client, err := NewClient(
				&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					if calls >= len(test.locations) {
						t.Fatalf("unexpected provider request %s", request.URL)
					}
					location := test.locations[calls]
					calls++
					body := newTrackedResponseBody(`{"ignored":"redirect"}`)
					bodies = append(bodies, body)
					header := http.Header{}
					if location != "" {
						header.Set("Location", location)
					}
					return &http.Response{
						StatusCode: http.StatusFound, Header: header, Body: body,
						ContentLength: int64(body.reader.Len()), Request: request,
					}, nil
				})},
				WithBaseURL("https://api.github.test"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.GetOwner(t.Context(), "octocat"); !errors.Is(err, ErrUpstream) {
				t.Fatalf("error=%v want ErrUpstream", err)
			}
			if calls != len(test.locations) {
				t.Fatalf("provider calls=%d want=%d", calls, len(test.locations))
			}
			for index, body := range bodies {
				if body.reads != 0 || body.bytesRead != 0 || !body.closed {
					t.Errorf("body %d reads=%d bytes=%d closed=%t", index, body.reads, body.bytesRead, body.closed)
				}
			}
		})
	}
}

func TestClientRedirectPolicy(t *testing.T) {
	var calls int
	client, server := newTestClient(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		switch request.URL.Path {
		case "/users/octocat":
			response.Header().Set("Location", "/user/42")
			response.WriteHeader(http.StatusMovedPermanently)
		case "/user/42":
			writeJSON(t, response, ownerFixture)
		default:
			http.NotFound(response, request)
		}
	}))
	owner, err := client.GetOwner(t.Context(), "octocat")
	if err != nil || owner.ID != 1 || calls != 2 {
		t.Fatalf("redirect result = %#v, err = %v, calls = %d", owner, err, calls)
	}

	current, _ := url.Parse(server.URL + "/users/octocat")
	origin, _ := url.Parse(server.URL)
	spec := providerSpec{origin: origin, namedPath: "/users/octocat", numericPrefix: "/user/"}
	for _, location := range []string{
		"https://example.com/user/42", "/evil", "/user/42?unexpected=1", "/user/42#fragment", "/user/0042",
	} {
		if _, err := redirectTarget(current, http.Header{"Location": {location}}, spec); !errors.Is(err, ErrUpstream) {
			t.Errorf("location %q error = %v", location, err)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestClientCancellationAndTimeoutClassification(t *testing.T) {
	transportCalls := 0
	client, err := NewClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		transportCalls++
		return nil, context.DeadlineExceeded
	})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetOwner(t.Context(), "octocat"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("transport deadline error = %v", err)
	}
	if transportCalls != 1 {
		t.Fatalf("transport calls=%d want=1", transportCalls)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.GetOwner(ctx, "octocat"); !errors.Is(err, context.Canceled) {
		t.Fatalf("caller cancellation error = %v", err)
	}

	t.Run("redirects share one ten-second budget", func(t *testing.T) {
		var deadlines []time.Time
		calls := 0
		redirectClient, newErr := NewClient(
			&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				deadline, ok := request.Context().Deadline()
				if !ok {
					t.Fatal("provider request has no deadline")
				}
				deadlines = append(deadlines, deadline)
				if calls == 1 {
					return &http.Response{
						StatusCode: http.StatusMovedPermanently,
						Header:     http.Header{"Location": {"https://api.github.test/user/42"}},
						Body:       newTrackedResponseBody("ignored"), Request: request,
					}, nil
				}
				return nil, context.DeadlineExceeded
			})},
			WithBaseURL("https://api.github.test"),
		)
		if newErr != nil {
			t.Fatal(newErr)
		}
		started := time.Now()
		if _, requestErr := redirectClient.GetOwner(t.Context(), "octocat"); !errors.Is(requestErr, ErrTimeout) {
			t.Fatalf("error=%v want ErrTimeout", requestErr)
		}
		if calls != 2 || len(deadlines) != 2 || !deadlines[0].Equal(deadlines[1]) {
			t.Fatalf("calls=%d deadlines=%v", calls, deadlines)
		}
		remaining := deadlines[0].Sub(started)
		if remaining < providerTimeout-100*time.Millisecond || remaining > providerTimeout+100*time.Millisecond {
			t.Fatalf("provider budget=%s want=%s", remaining, providerTimeout)
		}
	})
}

func TestCursorScopeAndRepositoryNameGuards(t *testing.T) {
	providerCalls := 0
	client, _ := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		providerCalls++
		t.Fatal("provider must not be called for local rejection")
	}))
	repositoryScope := pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "octocat", Limit: 10}
	activityScope := pagination.Scope{
		Operation: "listGitHubRepositoryActivity", Owner: "octocat", Repo: "repo", Limit: 10,
	}
	tagScope := pagination.Scope{Operation: "listGitHubRepositoryTags", Owner: "octocat", Repo: "repo", Limit: 10}

	wrongVersion := pagination.NewCursor(repositoryScope, "next", "2")
	wrongVersion.Version = 2
	wrongDirection := pagination.NewCursor(repositoryScope, "sideways", "2")
	withUpstream := pagination.NewCursor(repositoryScope, "next", "2")
	withUpstream.Upstream = "https://api.github.test/private"
	tests := []struct {
		name   string
		invoke func() error
	}{
		{
			name: "repositories operation",
			invoke: func() error {
				cursor := pagination.NewCursor(
					pagination.Scope{Operation: "listGitHubRepositoryTags", Owner: "octocat", Repo: "repo", Limit: 10},
					"next", "2",
				)
				_, err := client.ListOwnerRepositories(t.Context(), "octocat", 10, &cursor)
				return err
			},
		},
		{
			name: "repositories owner",
			invoke: func() error {
				cursor := pagination.NewCursor(
					pagination.Scope{Operation: repositoryScope.Operation, Owner: "other", Limit: 10}, "next", "2",
				)
				_, err := client.ListOwnerRepositories(t.Context(), "octocat", 10, &cursor)
				return err
			},
		},
		{
			name: "repositories limit",
			invoke: func() error {
				cursor := pagination.NewCursor(
					pagination.Scope{Operation: repositoryScope.Operation, Owner: "octocat", Limit: 9}, "next", "2",
				)
				_, err := client.ListOwnerRepositories(t.Context(), "octocat", 10, &cursor)
				return err
			},
		},
		{
			name: "repositories impossible next page one",
			invoke: func() error {
				cursor := pagination.NewCursor(repositoryScope, "next", "1")
				_, err := client.ListOwnerRepositories(t.Context(), "octocat", 10, &cursor)
				return err
			},
		},
		{
			name: "repositories version",
			invoke: func() error {
				_, err := client.ListOwnerRepositories(t.Context(), "octocat", 10, &wrongVersion)
				return err
			},
		},
		{
			name: "repositories direction",
			invoke: func() error {
				_, err := client.ListOwnerRepositories(t.Context(), "octocat", 10, &wrongDirection)
				return err
			},
		},
		{
			name: "repositories complete upstream URL",
			invoke: func() error {
				_, err := client.ListOwnerRepositories(t.Context(), "octocat", 10, &withUpstream)
				return err
			},
		},
		{
			name: "activity operation",
			invoke: func() error {
				cursor := pagination.NewCursor(tagScope, "next", "opaque")
				_, err := client.ListRepositoryActivity(t.Context(), "octocat", "repo", 10, &cursor)
				return err
			},
		},
		{
			name: "activity owner",
			invoke: func() error {
				wrong := activityScope
				wrong.Owner = "other"
				cursor := pagination.NewCursor(wrong, "next", "opaque")
				_, err := client.ListRepositoryActivity(t.Context(), "octocat", "repo", 10, &cursor)
				return err
			},
		},
		{
			name: "activity repository",
			invoke: func() error {
				wrong := activityScope
				wrong.Repo = "other"
				cursor := pagination.NewCursor(wrong, "next", "opaque")
				_, err := client.ListRepositoryActivity(t.Context(), "octocat", "repo", 10, &cursor)
				return err
			},
		},
		{
			name: "activity limit",
			invoke: func() error {
				wrong := activityScope
				wrong.Limit = 9
				cursor := pagination.NewCursor(wrong, "next", "opaque")
				_, err := client.ListRepositoryActivity(t.Context(), "octocat", "repo", 10, &cursor)
				return err
			},
		},
		{
			name: "activity direction",
			invoke: func() error {
				cursor := pagination.NewCursor(activityScope, "sideways", "opaque")
				_, err := client.ListRepositoryActivity(t.Context(), "octocat", "repo", 10, &cursor)
				return err
			},
		},
		{
			name: "tags owner",
			invoke: func() error {
				wrong := tagScope
				wrong.Owner = "other"
				cursor := pagination.NewCursor(wrong, "next", "2")
				_, err := client.ListRepositoryTags(t.Context(), "octocat", "repo", 10, &cursor)
				return err
			},
		},
		{
			name: "tags repository",
			invoke: func() error {
				wrong := tagScope
				wrong.Repo = "other"
				cursor := pagination.NewCursor(wrong, "next", "2")
				_, err := client.ListRepositoryTags(t.Context(), "octocat", "repo", 10, &cursor)
				return err
			},
		},
		{
			name: "tags limit",
			invoke: func() error {
				wrong := tagScope
				wrong.Limit = 9
				cursor := pagination.NewCursor(wrong, "next", "2")
				_, err := client.ListRepositoryTags(t.Context(), "octocat", "repo", 10, &cursor)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.invoke(); !errors.Is(err, ErrInvalidCursor) {
				t.Fatalf("error=%v want ErrInvalidCursor", err)
			}
		})
	}
	for _, repository := range []string{".", "..", "..."} {
		if _, err := client.GetRepository(t.Context(), "octocat", repository); !errors.Is(err, ErrUpstream) {
			t.Errorf("repository %q error = %v", repository, err)
		}
	}
	if providerCalls != 0 {
		t.Fatalf("provider calls=%d want=0", providerCalls)
	}
}

func TestParseNavigationRejectsAmbiguousOrUnsafeLinks(t *testing.T) {
	origin, _ := url.Parse("https://api.github.test")
	firstPageSpec := providerSpec{
		origin:        origin,
		namedPath:     "/users/octocat/repos",
		numericPrefix: "/user/",
		numericSuffix: "/repos",
		query:         url.Values{"type": {"owner"}, "sort": {"full_name"}, "direction": {"asc"}, "per_page": {"10"}},
		pagination:    numberedPagination,
		scope:         pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "octocat", Limit: 10},
	}
	valid := `<https://api.github.test/user/42/repos?direction=asc&page=2&per_page=10&sort=full_name&type=owner>; title="a,b"; rel="next"`
	navigation, err := parseNavigation(http.Header{"Link": {valid}}, firstPageSpec, false)
	if err != nil || navigation.nextCursor == "" {
		t.Fatalf("valid navigation = %#v, err = %v", navigation, err)
	}

	middleSpec := firstPageSpec
	middleSpec.currentValue = "3"
	middleSpec.query = cloneQuery(firstPageSpec.query)
	middleSpec.query.Set("page", "3")
	next := `<https://api.github.test/user/42/repos?direction=asc&page=5&per_page=10&sort=full_name&type=owner>; title="a,b"; rel="next"`
	previous := `<https://api.github.test/users/octocat/repos?direction=asc&page=1&per_page=10&sort=full_name&type=owner>; rel="prev"`
	ignored := `<https://api.github.test/users/octocat/repos?direction=asc&page=1&per_page=10&sort=full_name&type=owner>; rel="first last custom"`
	anchored := `<https://evil.example/private>; anchor="https://elsewhere.example/"; rel="next"`
	navigation, err = parseNavigation(
		http.Header{"Link": {next + ", " + ignored, previous, anchored}},
		middleSpec,
		false,
	)
	if err != nil {
		t.Fatalf("multiple Link fields: %v", err)
	}
	decodedNext, nextErr := pagination.DecodeCursor(navigation.nextCursor)
	decodedPrevious, previousErr := pagination.DecodeCursor(navigation.prevCursor)
	if nextErr != nil || previousErr != nil || decodedNext.Anchor != "5" || decodedNext.Direction != "next" ||
		decodedPrevious.Anchor != "1" || decodedPrevious.Direction != "prev" {
		t.Fatalf("navigation=%#v next=%#v/%v prev=%#v/%v",
			navigation, decodedNext, nextErr, decodedPrevious, previousErr)
	}

	invalid := []string{
		valid + ", " + valid,
		`<https://evil.example/user/42/repos?direction=asc&page=2&per_page=10&sort=full_name&type=owner>; rel="next"`,
		`<https://api.github.test/user/42/repos?direction=asc&page=1&per_page=10&sort=full_name&type=owner>; rel="next"`,
		`<https://api.github.test/user/42/repos?direction=asc&page=&per_page=10&sort=full_name&type=owner>; rel="next"`,
		`<https://api.github.test/user/42/repos?direction=asc&page=2&page=3&per_page=10&sort=full_name&type=owner>; rel="next"`,
		`<https://api.github.test/user/42/repos?direction=asc&page=2&per_page=10&sort=full_name&type=owner>; rel="next"; rel="prev"`,
		`<https://api.github.test/user/0042/repos?direction=asc&page=2&per_page=10&sort=full_name&type=owner>; rel="next"`,
		`<https://api.github.test/user/9007199254740992/repos?direction=asc&page=2&per_page=10&sort=full_name&type=owner>; rel="next"`,
		`<https://user@api.github.test/user/42/repos?direction=asc&page=2&per_page=10&sort=full_name&type=owner>; rel="next"`,
		`<https://api.github.test/user/42/repos?direction=asc&page=2&per_page=9&sort=full_name&type=owner>; rel="next"`,
		`<https://api.github.test/user/42/repos?direction=asc&page=2&per_page=10&sort=full_name&type=owner&extra=1>; rel="next"`,
		`<https://api.github.test/user/42/repos?direction=asc&page=2&per_page=10&sort=full_name&type=owner#fragment>; rel="next"`,
		`<https://api.github.test/user/42/repos?direction=asc&page=2&per_page=10&sort=full_name&type=owner; rel="next"`,
	}
	for _, field := range invalid {
		if _, err := parseNavigation(http.Header{"Link": {field}}, firstPageSpec, false); !errors.Is(err, ErrUpstream) {
			t.Errorf("unsafe Link %q error = %v", field, err)
		}
	}

	activityInitial := providerSpec{
		origin: origin, namedPath: "/repos/octocat/repo/activity", numericPrefix: "/repositories/",
		numericSuffix: "/activity", query: url.Values{"direction": {"desc"}, "per_page": {"10"}},
		pagination: activityPagination,
		scope: pagination.Scope{
			Operation: "listGitHubRepositoryActivity", Owner: "octocat", Repo: "repo", Limit: 10,
		},
	}
	activityCurrent := activityInitial
	activityCurrent.currentValue = "current"
	activityCurrent.query = cloneQuery(activityInitial.query)
	activityCurrent.query.Set("after", "current")
	activityNext := `<https://api.github.test/repositories/42/activity?after=next-token&direction=desc&per_page=10>; rel="next"`
	activityPrevious := `<https://api.github.test/repos/octocat/repo/activity?before=previous-token&direction=desc&per_page=10>; rel="prev"`
	if navigation, err := parseNavigation(
		http.Header{"Link": {activityNext, activityPrevious}}, activityCurrent, false,
	); err != nil || navigation.nextCursor == "" || navigation.prevCursor == "" {
		t.Fatalf("activity navigation=%#v err=%v", navigation, err)
	}
	if navigation, err := parseNavigation(http.Header{"Link": {activityPrevious}}, activityCurrent, true); err != nil ||
		navigation.nextCursor != "" || navigation.prevCursor == "" {
		t.Fatalf("empty later activity page=%#v err=%v", navigation, err)
	}
	activityInvalid := []struct {
		name  string
		spec  providerSpec
		field string
		empty bool
	}{
		{name: "initial prev", spec: activityInitial, field: activityPrevious},
		{
			name:  "next uses before",
			spec:  activityCurrent,
			field: `<https://api.github.test/repositories/42/activity?before=next-token&direction=desc&per_page=10>; rel="next"`,
		},
		{
			name:  "prev uses after",
			spec:  activityCurrent,
			field: `<https://api.github.test/repositories/42/activity?after=previous-token&direction=desc&per_page=10>; rel="prev"`,
		},
		{
			name:  "exact replay",
			spec:  activityCurrent,
			field: `<https://api.github.test/repositories/42/activity?after=current&direction=desc&per_page=10>; rel="next"`,
		},
		{
			name: "missing value", spec: activityCurrent,
			field: `<https://api.github.test/repositories/42/activity?after=&direction=desc&per_page=10>; rel="next"`,
		},
		{
			name:  "nonprintable value",
			spec:  activityCurrent,
			field: `<https://api.github.test/repositories/42/activity?after=%20&direction=desc&per_page=10>; rel="next"`,
		},
		{name: "empty page next", spec: activityCurrent, field: activityNext, empty: true},
	}
	for _, test := range activityInvalid {
		if _, err := parseNavigation(
			http.Header{"Link": {test.field}},
			test.spec,
			test.empty,
		); !errors.Is(
			err,
			ErrUpstream,
		) {
			t.Errorf("%s error=%v", test.name, err)
		}
	}
}

func FuzzParseLinkHeader(f *testing.F) {
	f.Add(
		`<https://api.github.test/users/octocat/repos?direction=asc&page=2&per_page=10&sort=full_name&type=owner>; rel="next"`,
	)
	origin, _ := url.Parse("https://api.github.test")
	spec := providerSpec{
		origin:        origin,
		namedPath:     "/users/octocat/repos",
		numericPrefix: "/repositories/",
		numericSuffix: "/repos",
		query:         url.Values{"type": {"owner"}, "sort": {"full_name"}, "direction": {"asc"}, "per_page": {"10"}},
		pagination:    numberedPagination,
		scope:         pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "octocat", Limit: 10},
	}
	f.Fuzz(func(t *testing.T, value string) {
		_, _ = parseNavigation(http.Header{"Link": {value}}, spec, false)
	})
}

func FuzzParseLinkHeaderRoundTrip(f *testing.F) {
	f.Add(uint16(2))
	origin, _ := url.Parse("https://api.github.test")
	f.Fuzz(func(t *testing.T, rawPage uint16) {
		page := uint64(rawPage) + 2
		spec := providerSpec{
			origin:        origin,
			namedPath:     "/users/octocat/repos",
			numericPrefix: "/repositories/",
			numericSuffix: "/repos",
			query: url.Values{
				"type":      {"owner"},
				"sort":      {"full_name"},
				"direction": {"asc"},
				"per_page":  {"10"},
			},
			pagination: numberedPagination,
			scope:      pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "octocat", Limit: 10},
		}
		field := fmt.Sprintf(
			`<https://api.github.test/users/octocat/repos?direction=asc&page=%d&per_page=10&sort=full_name&type=owner>; rel="next"`,
			page,
		)
		navigation, err := parseNavigation(http.Header{"Link": {field}}, spec, false)
		if err != nil {
			t.Fatalf("parse generated link: %v", err)
		}
		cursor, err := pagination.DecodeCursor(navigation.nextCursor)
		if err != nil || cursor.Anchor != strconv.FormatUint(page, 10) {
			t.Fatalf("cursor = %#v, err = %v", cursor, err)
		}
	})
}
