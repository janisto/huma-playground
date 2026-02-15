package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	applog "github.com/janisto/huma-playground/internal/platform/logging"
)

const (
	defaultBaseURL = "https://api.github.com"
	userAgent      = "huma-playground"
	apiVersion     = "2022-11-28"
	acceptHeader   = "application/vnd.github+json"
)

// Client implements Service using the GitHub REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL sets a custom base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithToken sets the Bearer token for authenticated requests.
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}

// NewClient creates a new GitHub API client.
func NewClient(httpClient *http.Client, opts ...Option) *Client {
	c := &Client{
		httpClient: httpClient,
		baseURL:    defaultBaseURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GitHub API response types (snake_case JSON tags matching GitHub's API).

type githubOwner struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
	Bio       string `json:"bio"`
	Location  string `json:"location"`
	Blog      string `json:"blog"`
	Company   string `json:"company"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type githubRepoSummary struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Language    string `json:"language"`
	Stars       int    `json:"stargazers_count"`
	Forks       int    `json:"forks_count"`
	OpenIssues  int    `json:"open_issues_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type githubRepo struct {
	githubRepoSummary
	DefaultBranch string   `json:"default_branch"`
	Topics        []string `json:"topics"`
	Archived      bool     `json:"archived"`
	Disabled      bool     `json:"disabled"`
	License       *struct {
		Name string `json:"name"`
	} `json:"license"`
}

type githubActivity struct {
	ID           int64  `json:"id"`
	Ref          string `json:"ref"`
	Timestamp    string `json:"timestamp"`
	ActivityType string `json:"activity_type"`
	Actor        *struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"actor"`
}

type githubTag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

func (c *Client) doRequest(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", acceptHeader)
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.httpClient.Do(req)
}

func (c *Client) decodeResponse(ctx context.Context, resp *http.Response, target any) error {
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("decoding github response: %w", err)
		}
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return upstreamErrorFromResponse(resp, UpstreamErrorKindNotFound, ErrNotFound)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		logRateLimited(ctx, resp)
		return upstreamErrorFromResponse(resp, UpstreamErrorKindRateLimited, ErrRateLimited)
	}
	if resp.StatusCode == http.StatusForbidden {
		if isGitHubRateLimitResponse(resp) {
			logRateLimited(ctx, resp)
			return upstreamErrorFromResponse(resp, UpstreamErrorKindRateLimited, ErrRateLimited)
		}
		remaining := strings.TrimSpace(resp.Header.Get("X-RateLimit-Remaining"))
		reset := strings.TrimSpace(resp.Header.Get("X-RateLimit-Reset"))
		applog.LogWarn(ctx, "github api access denied",
			zap.Int("status", resp.StatusCode),
			zap.String("X-RateLimit-Remaining", remaining),
			zap.String("X-RateLimit-Reset", reset),
		)
		return upstreamErrorFromResponse(resp, UpstreamErrorKindForbidden, ErrForbidden)
	}

	return upstreamErrorFromResponse(resp, UpstreamErrorKindUpstream, ErrUpstream)
}

func (c *Client) GetOwner(ctx context.Context, owner string) (*Owner, error) {
	resp, err := c.doRequest(ctx, "/users/"+url.PathEscape(owner), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching owner: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var gh githubOwner
	if err := c.decodeResponse(ctx, resp, &gh); err != nil {
		return nil, err
	}

	var createdAt, updatedAt time.Time
	var parseErr error
	if createdAt, parseErr = parseTime(gh.CreatedAt); parseErr != nil {
		return nil, fmt.Errorf("decoding owner: %w", parseErr)
	}
	if updatedAt, parseErr = parseTime(gh.UpdatedAt); parseErr != nil {
		return nil, fmt.Errorf("decoding owner: %w", parseErr)
	}

	return &Owner{
		Login:     gh.Login,
		Name:      gh.Name,
		AvatarURL: gh.AvatarURL,
		HTMLURL:   gh.HTMLURL,
		Bio:       gh.Bio,
		Location:  gh.Location,
		Blog:      gh.Blog,
		Company:   gh.Company,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func (c *Client) ListRepos(ctx context.Context, owner string) ([]RepoSummary, error) {
	q := url.Values{"per_page": {"30"}}
	resp, err := c.doRequest(ctx, "/users/"+url.PathEscape(owner)+"/repos", q)
	if err != nil {
		return nil, fmt.Errorf("fetching repos: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var gh []githubRepoSummary
	if err := c.decodeResponse(ctx, resp, &gh); err != nil {
		return nil, err
	}

	repos := make([]RepoSummary, len(gh))
	for i, r := range gh {
		s, err := toRepoSummary(r)
		if err != nil {
			return nil, fmt.Errorf("decoding repo %d: %w", i, err)
		}
		repos[i] = s
	}
	return repos, nil
}

func (c *Client) GetRepo(ctx context.Context, owner, repo string) (*Repo, error) {
	resp, err := c.doRequest(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching repo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var gh githubRepo
	if decodeErr := c.decodeResponse(ctx, resp, &gh); decodeErr != nil {
		return nil, decodeErr
	}

	license := ""
	if gh.License != nil {
		license = gh.License.Name
	}

	topics := gh.Topics
	if topics == nil {
		topics = []string{}
	}

	summary, err := toRepoSummary(gh.githubRepoSummary)
	if err != nil {
		return nil, fmt.Errorf("decoding repo: %w", err)
	}

	return &Repo{
		RepoSummary:   summary,
		DefaultBranch: gh.DefaultBranch,
		License:       license,
		Topics:        topics,
		Archived:      gh.Archived,
		Disabled:      gh.Disabled,
	}, nil
}

func (c *Client) ListActivity(
	ctx context.Context, owner, repo string, limit int, afterCursor string,
) (*ActivityPage, error) {
	q := url.Values{"per_page": {strconv.Itoa(limit)}}
	if afterCursor != "" {
		q.Set("after", afterCursor)
	}

	resp, err := c.doRequest(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/activity", q)
	if err != nil {
		return nil, fmt.Errorf("fetching activity: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	linkHeader := resp.Header.Get("Link")

	var gh []githubActivity
	if err := c.decodeResponse(ctx, resp, &gh); err != nil {
		return nil, err
	}

	activities := make([]Activity, len(gh))
	for i, a := range gh {
		ts, err := parseTime(a.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("decoding activity %d: %w", i, err)
		}
		actor := ""
		avatarURL := ""
		if a.Actor != nil {
			actor = a.Actor.Login
			avatarURL = a.Actor.AvatarURL
		}
		activities[i] = Activity{
			ID:             a.ID,
			Actor:          actor,
			Ref:            a.Ref,
			Timestamp:      ts,
			ActivityType:   a.ActivityType,
			ActorAvatarURL: avatarURL,
		}
	}

	nextCursor := parseLinkHeader(linkHeader)

	return &ActivityPage{
		Activities: activities,
		NextCursor: nextCursor,
	}, nil
}

func (c *Client) ListLanguages(ctx context.Context, owner, repo string) (map[string]int64, error) {
	resp, err := c.doRequest(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/languages", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching languages: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var languages map[string]int64
	if err := c.decodeResponse(ctx, resp, &languages); err != nil {
		return nil, err
	}
	return languages, nil
}

func (c *Client) ListTags(ctx context.Context, owner, repo string) ([]Tag, error) {
	q := url.Values{"per_page": {"30"}}
	resp, err := c.doRequest(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/tags", q)
	if err != nil {
		return nil, fmt.Errorf("fetching tags: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var gh []githubTag
	if err := c.decodeResponse(ctx, resp, &gh); err != nil {
		return nil, err
	}

	tags := make([]Tag, len(gh))
	for i, t := range gh {
		tags[i] = Tag{
			Name: t.Name,
			Commit: TagCommit{
				SHA: t.Commit.SHA,
			},
		}
	}
	return tags, nil
}

// parseLinkHeader extracts the "after" cursor from a GitHub Link header.
func parseLinkHeader(header string) string {
	if header == "" {
		return ""
	}

	for raw := range strings.SplitSeq(header, ",") {
		part := strings.TrimSpace(raw)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}

		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start < 0 || end < 0 || end <= start {
			continue
		}

		linkURL, err := url.Parse(part[start+1 : end])
		if err != nil {
			continue
		}

		if after := linkURL.Query().Get("after"); after != "" {
			return after
		}
	}
	return ""
}

func toRepoSummary(r githubRepoSummary) (RepoSummary, error) {
	createdAt, err := parseTime(r.CreatedAt)
	if err != nil {
		return RepoSummary{}, err
	}
	updatedAt, err := parseTime(r.UpdatedAt)
	if err != nil {
		return RepoSummary{}, err
	}
	return RepoSummary{
		Name:        r.Name,
		FullName:    r.FullName,
		Description: r.Description,
		HTMLURL:     r.HTMLURL,
		Language:    r.Language,
		Stars:       r.Stars,
		Forks:       r.Forks,
		OpenIssues:  r.OpenIssues,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("missing required timestamp")
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing time %q: %w", s, err)
	}
	return t, nil
}

func upstreamErrorFromResponse(resp *http.Response, kind UpstreamErrorKind, cause error) *UpstreamError {
	return &UpstreamError{
		Kind:           kind,
		Status:         resp.StatusCode,
		RetryAfter:     strings.TrimSpace(resp.Header.Get("Retry-After")),
		RateLimitReset: strings.TrimSpace(resp.Header.Get("X-RateLimit-Reset")),
		cause:          cause,
	}
}

func isGitHubRateLimitResponse(resp *http.Response) bool {
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if resp.StatusCode != http.StatusForbidden {
		return false
	}
	if strings.TrimSpace(resp.Header.Get("X-RateLimit-Remaining")) == "0" {
		return true
	}
	return strings.TrimSpace(resp.Header.Get("Retry-After")) != ""
}

func logRateLimited(ctx context.Context, resp *http.Response) {
	fields := []zap.Field{
		zap.Int("status", resp.StatusCode),
		zap.String("X-RateLimit-Remaining", resp.Header.Get("X-RateLimit-Remaining")),
		zap.String("X-RateLimit-Reset", resp.Header.Get("X-RateLimit-Reset")),
	}
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		fields = append(fields, zap.String("Retry-After", retryAfter))
	}
	applog.LogWarn(ctx, "github api rate limit exceeded", fields...)
}

// Compile-time interface check
var _ Service = (*Client)(nil)
