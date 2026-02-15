package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func newTestClient(serverURL string) *Client {
	return NewClient(http.DefaultClient, WithBaseURL(serverURL))
}

func TestGetOwner(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/octocat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"login":      "octocat",
			"name":       "The Octocat",
			"avatar_url": "https://avatars.githubusercontent.com/u/583231",
			"html_url":   "https://github.com/octocat",
			"bio":        "",
			"location":   "San Francisco",
			"blog":       "https://github.blog",
			"company":    "@github",
			"created_at": "2011-01-25T18:44:36Z",
			"updated_at": "2024-01-01T00:00:00Z",
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	owner, err := client.GetOwner(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	if owner.Company != "@github" {
		t.Errorf("expected company @github, got %s", owner.Company)
	}
	if owner.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestListRepos(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/octocat/repos" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("per_page") != "30" {
			t.Errorf("expected per_page=30, got %s", r.URL.Query().Get("per_page"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"name":              "hello-world",
				"full_name":         "octocat/hello-world",
				"description":       "My first repo",
				"html_url":          "https://github.com/octocat/hello-world",
				"language":          "Go",
				"stargazers_count":  42,
				"forks_count":       10,
				"open_issues_count": 2,
				"created_at":        "2020-01-01T00:00:00Z",
				"updated_at":        "2024-06-01T00:00:00Z",
			},
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	repos, err := client.ListRepos(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].Name != "hello-world" {
		t.Errorf("expected name hello-world, got %s", repos[0].Name)
	}
	if repos[0].Stars != 42 {
		t.Errorf("expected 42 stars, got %d", repos[0].Stars)
	}
	if repos[0].Language != "Go" {
		t.Errorf("expected language Go, got %s", repos[0].Language)
	}
}

func TestGetRepo(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/octocat/hello-world" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":              "hello-world",
			"full_name":         "octocat/hello-world",
			"description":       "My first repo",
			"html_url":          "https://github.com/octocat/hello-world",
			"language":          "Go",
			"stargazers_count":  42,
			"forks_count":       10,
			"open_issues_count": 2,
			"default_branch":    "main",
			"topics":            []string{"go", "demo"},
			"archived":          false,
			"disabled":          false,
			"license":           map[string]any{"name": "MIT License"},
			"created_at":        "2020-01-01T00:00:00Z",
			"updated_at":        "2024-06-01T00:00:00Z",
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	repo, err := client.GetRepo(context.Background(), "octocat", "hello-world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Name != "hello-world" {
		t.Errorf("expected name hello-world, got %s", repo.Name)
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("expected default branch main, got %s", repo.DefaultBranch)
	}
	if repo.License != "MIT License" {
		t.Errorf("expected license MIT License, got %s", repo.License)
	}
	if len(repo.Topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(repo.Topics))
	}
}

func TestGetRepoNilLicense(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":              "no-license",
			"full_name":         "octocat/no-license",
			"description":       "",
			"html_url":          "https://github.com/octocat/no-license",
			"stargazers_count":  0,
			"forks_count":       0,
			"open_issues_count": 0,
			"default_branch":    "main",
			"license":           nil,
			"created_at":        "2020-01-01T00:00:00Z",
			"updated_at":        "2024-06-01T00:00:00Z",
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	repo, err := client.GetRepo(context.Background(), "octocat", "no-license")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.License != "" {
		t.Errorf("expected empty license, got %s", repo.License)
	}
}

func TestListActivity(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/octocat/hello-world/activity" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("per_page") != "10" {
			t.Errorf("expected per_page=10, got %s", r.URL.Query().Get("per_page"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `<https://api.github.com/repos/octocat/hello-world/activity?after=abc123>; rel="next"`)
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":            1,
				"ref":           "refs/heads/main",
				"timestamp":     "2024-01-15T10:30:00Z",
				"activity_type": "push",
				"actor": map[string]any{
					"login":      "octocat",
					"avatar_url": "https://avatars.githubusercontent.com/u/583231",
				},
			},
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	page, err := client.ListActivity(context.Background(), "octocat", "hello-world", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(page.Activities))
	}
	if page.Activities[0].Actor != "octocat" {
		t.Errorf("expected actor octocat, got %s", page.Activities[0].Actor)
	}
	if page.Activities[0].ActivityType != "push" {
		t.Errorf("expected activity_type push, got %s", page.Activities[0].ActivityType)
	}
	if page.NextCursor != "abc123" {
		t.Errorf("expected next cursor abc123, got %s", page.NextCursor)
	}
}

func TestListActivityWithCursor(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("after") != "cursor-xyz" {
			t.Errorf("expected after=cursor-xyz, got %s", r.URL.Query().Get("after"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	page, err := client.ListActivity(context.Background(), "octocat", "hello-world", 10, "cursor-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.NextCursor != "" {
		t.Errorf("expected empty next cursor, got %s", page.NextCursor)
	}
}

func TestListActivityNilActor(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":            1,
				"ref":           "refs/heads/main",
				"timestamp":     "2024-01-15T10:30:00Z",
				"activity_type": "push",
				"actor":         nil,
			},
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	page, err := client.ListActivity(context.Background(), "octocat", "hello-world", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Activities[0].Actor != "" {
		t.Errorf("expected empty actor, got %s", page.Activities[0].Actor)
	}
}

func TestListLanguages(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/octocat/hello-world/languages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{
			"Go":         12345,
			"JavaScript": 6789,
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	langs, err := client.ListLanguages(context.Background(), "octocat", "hello-world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(langs) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(langs))
	}
	if langs["Go"] != 12345 {
		t.Errorf("expected Go=12345, got %d", langs["Go"])
	}
}

func TestListTags(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/octocat/hello-world/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("per_page") != "30" {
			t.Errorf("expected per_page=30, got %s", r.URL.Query().Get("per_page"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"name": "v1.0.0",
				"commit": map[string]any{
					"sha": "abc123def456",
				},
			},
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	tags, err := client.ListTags(context.Background(), "octocat", "hello-world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Name != "v1.0.0" {
		t.Errorf("expected tag v1.0.0, got %s", tags[0].Name)
	}
	if tags[0].Commit.SHA != "abc123def456" {
		t.Errorf("expected sha abc123def456, got %s", tags[0].Commit.SHA)
	}
}

func TestNotFoundError(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	client := newTestClient(srv.URL)

	_, err := client.GetOwner(context.Background(), "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected UpstreamError, got %T", err)
	}
	if upstreamErr.Kind != UpstreamErrorKindNotFound {
		t.Fatalf("expected kind %q, got %q", UpstreamErrorKindNotFound, upstreamErr.Kind)
	}
	if upstreamErr.Status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", upstreamErr.Status)
	}
}

func TestForbiddenError(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "10")
		w.WriteHeader(http.StatusForbidden)
	})
	defer srv.Close()

	client := newTestClient(srv.URL)

	_, err := client.GetOwner(context.Background(), "octocat")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected UpstreamError, got %T", err)
	}
	if upstreamErr.Kind != UpstreamErrorKindForbidden {
		t.Fatalf("expected kind %q, got %q", UpstreamErrorKindForbidden, upstreamErr.Kind)
	}
	if upstreamErr.Status != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", upstreamErr.Status)
	}
}

func TestRateLimitedError(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1700000000")
		w.WriteHeader(http.StatusForbidden)
	})
	defer srv.Close()

	client := newTestClient(srv.URL)

	_, err := client.GetOwner(context.Background(), "octocat")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}

	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected UpstreamError, got %T", err)
	}
	if upstreamErr.Kind != UpstreamErrorKindRateLimited {
		t.Fatalf("expected kind %q, got %q", UpstreamErrorKindRateLimited, upstreamErr.Kind)
	}
	if upstreamErr.Status != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", upstreamErr.Status)
	}
	if upstreamErr.RateLimitReset != "1700000000" {
		t.Fatalf("expected reset 1700000000, got %q", upstreamErr.RateLimitReset)
	}
}

func TestRateLimited403WithRetryAfter(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.Header().Set("X-RateLimit-Remaining", "10")
		w.Header().Set("X-RateLimit-Reset", "1700000000")
		w.WriteHeader(http.StatusForbidden)
	})
	defer srv.Close()

	client := newTestClient(srv.URL)

	_, err := client.GetOwner(context.Background(), "octocat")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}

	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected UpstreamError, got %T", err)
	}
	if upstreamErr.Kind != UpstreamErrorKindRateLimited {
		t.Fatalf("expected kind %q, got %q", UpstreamErrorKindRateLimited, upstreamErr.Kind)
	}
	if upstreamErr.RetryAfter != "60" {
		t.Fatalf("expected Retry-After 60, got %q", upstreamErr.RetryAfter)
	}
}

func TestRateLimitedHTTP429(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1700000000")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer srv.Close()

	client := newTestClient(srv.URL)

	_, err := client.GetOwner(context.Background(), "octocat")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}

	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected UpstreamError, got %T", err)
	}
	if upstreamErr.Kind != UpstreamErrorKindRateLimited {
		t.Fatalf("expected kind %q, got %q", UpstreamErrorKindRateLimited, upstreamErr.Kind)
	}
	if upstreamErr.Status != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", upstreamErr.Status)
	}
	if upstreamErr.RetryAfter != "60" {
		t.Fatalf("expected Retry-After 60, got %q", upstreamErr.RetryAfter)
	}
}

func TestUpstreamError(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()

	client := newTestClient(srv.URL)

	_, err := client.GetOwner(context.Background(), "octocat")
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}

	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected UpstreamError, got %T", err)
	}
	if upstreamErr.Kind != UpstreamErrorKindUpstream {
		t.Fatalf("expected kind %q, got %q", UpstreamErrorKindUpstream, upstreamErr.Kind)
	}
	if upstreamErr.Status != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", upstreamErr.Status)
	}
}

func TestMalformedJSON(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{invalid json"))
	})
	defer srv.Close()

	client := newTestClient(srv.URL)

	_, err := client.GetOwner(context.Background(), "octocat")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestContextCancellation(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"login": "octocat"})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetOwner(ctx, "octocat")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestTokenSentAsBearer(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token-123" {
			t.Errorf("expected Bearer test-token-123, got %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"login":      "octocat",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
		})
	})
	defer srv.Close()

	client := NewClient(http.DefaultClient, WithBaseURL(srv.URL), WithToken("test-token-123"))
	_, err := client.GetOwner(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNoTokenNoAuthHeader(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" {
			t.Errorf("expected no Authorization header, got %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"login":      "octocat",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.GetOwner(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequiredHeaders(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "huma-playground" {
			t.Errorf("expected User-Agent huma-playground, got %s", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("expected Accept application/vnd.github+json, got %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Errorf("expected X-GitHub-Api-Version 2022-11-28, got %s", r.Header.Get("X-GitHub-Api-Version"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"login":      "octocat",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.GetOwner(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseLinkHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "empty header",
			header:   "",
			expected: "",
		},
		{
			name:     "next link with after cursor",
			header:   `<https://api.github.com/repos/octocat/hello/activity?after=abc123>; rel="next"`,
			expected: "abc123",
		},
		{
			name:     "multiple links",
			header:   `<https://api.github.com/repos/octocat/hello/activity?before=xyz>; rel="prev", <https://api.github.com/repos/octocat/hello/activity?after=def456>; rel="next"`,
			expected: "def456",
		},
		{
			name:     "no next link",
			header:   `<https://api.github.com/repos/octocat/hello/activity?before=xyz>; rel="prev"`,
			expected: "",
		},
		{
			name:     "next link without after param",
			header:   `<https://api.github.com/repos/octocat/hello/activity?page=2>; rel="next"`,
			expected: "",
		},
		{
			name:     "malformed link",
			header:   `no angle brackets; rel="next"`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLinkHeader(tt.header)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestInterfaceCompliance(t *testing.T) {
	var _ Service = (*Client)(nil)
}

func TestPathEscaping(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("expected empty query string, got %s", r.URL.RawQuery)
		}
		if !strings.Contains(r.RequestURI, "%3F") {
			t.Errorf("expected percent-encoded question mark in RequestURI, got %s", r.RequestURI)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"login":      "octocat",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.GetOwner(context.Background(), "octocat?foo=bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseTimeInvalid(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"login":      "octocat",
			"created_at": "not-a-date",
			"updated_at": "2024-01-01T00:00:00Z",
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.GetOwner(context.Background(), "octocat")
	if err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
}

func TestParseTimeEmpty(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"login":      "octocat",
			"created_at": "",
			"updated_at": "",
		})
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.GetOwner(context.Background(), "octocat")
	if err == nil {
		t.Fatal("expected error for missing timestamp")
	}
}
