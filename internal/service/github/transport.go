package github

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/janisto/huma-playground/internal/platform/pagination"
	"github.com/janisto/huma-playground/internal/platform/portable"
)

const (
	providerOrigin       = "https://api.github.com"
	providerAPIVersion   = "2026-03-10"
	providerUserAgent    = "huma-playground"
	providerTimeout      = 10 * time.Second
	maximumProviderBody  = 4_194_304
	maximumRedirectCount = 3
)

type Client struct {
	origin     *url.URL
	httpClient *http.Client
	clock      func() time.Time
}

type clientConfig struct {
	baseURL string
	clock   func() time.Time
}

type Option func(*clientConfig)

// WithBaseURL is a constructor-only test seam. Public HTTP input cannot select it.
func WithBaseURL(value string) Option {
	return func(config *clientConfig) { config.baseURL = value }
}

// WithClock supplies deterministic quota timing in transport tests.
func WithClock(clock func() time.Time) Option {
	return func(config *clientConfig) { config.clock = clock }
}

func NewClient(httpClient *http.Client, options ...Option) (*Client, error) {
	if httpClient == nil {
		return nil, errors.New("github HTTP client is required")
	}
	config := clientConfig{baseURL: providerOrigin, clock: time.Now}
	for _, option := range options {
		option(&config)
	}
	if config.clock == nil {
		return nil, errors.New("github clock is required")
	}
	origin, err := url.Parse(config.baseURL)
	if err != nil || !origin.IsAbs() || origin.Scheme != "http" && origin.Scheme != "https" || origin.Host == "" ||
		origin.User != nil || origin.Path != "" && origin.Path != "/" || origin.RawPath != "" ||
		origin.RawQuery != "" || origin.Fragment != "" {
		return nil, errors.New(
			"github base URL must be an HTTP(S) origin without credentials, path, query, or fragment",
		)
	}
	origin.Path = ""
	copyClient := *httpClient
	copyClient.Timeout = 0
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{origin: origin, httpClient: &copyClient, clock: config.clock}, nil
}

func (client *Client) GetOwner(ctx context.Context, owner string) (Owner, error) {
	spec := client.pointSpec("/users/"+url.PathEscape(owner), "/user/", "")
	return executeProjected(client, ctx, spec, func(document []byte, _ http.Header) (Owner, error) {
		return projectOwner(document)
	})
}

func (client *Client) ListOwnerRepositories(
	ctx context.Context,
	owner string,
	limit int,
	cursor *pagination.Cursor,
) (Page[RepositorySummary], error) {
	scope := pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: owner, Limit: limit}
	query := url.Values{
		"type": {"owner"}, "sort": {"full_name"}, "direction": {"asc"}, "per_page": {strconv.Itoa(limit)},
	}
	current, err := applyNumberedCursor(query, scope, cursor)
	if err != nil {
		return Page[RepositorySummary]{}, err
	}
	spec := client.pageSpec(
		"/users/"+url.PathEscape(owner)+"/repos",
		"/user/",
		"/repos",
		query,
		numberedPagination,
		current,
		scope,
	)
	return executeProjected(
		client,
		ctx,
		spec,
		func(document []byte, headers http.Header) (Page[RepositorySummary], error) {
			entries, err := projectRepositorySummaries(document, limit)
			if err != nil {
				return Page[RepositorySummary]{}, err
			}
			navigation, err := parseNavigation(headers, spec, len(entries) == 0)
			return Page[RepositorySummary]{
				Entries:    entries,
				NextCursor: navigation.nextCursor,
				PrevCursor: navigation.prevCursor,
			}, err
		},
	)
}

func (client *Client) GetRepository(ctx context.Context, owner, repository string) (Repository, error) {
	if !hasNonDotRepositoryCharacter(repository) {
		return Repository{}, ErrUpstream
	}
	spec := client.pointSpec("/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repository), "/repositories/", "")
	return executeProjected(client, ctx, spec, func(document []byte, _ http.Header) (Repository, error) {
		return projectRepository(document)
	})
}

func (client *Client) ListRepositoryActivity(
	ctx context.Context,
	owner, repository string,
	limit int,
	cursor *pagination.Cursor,
) (Page[Activity], error) {
	if !hasNonDotRepositoryCharacter(repository) {
		return Page[Activity]{}, ErrUpstream
	}
	scope := pagination.Scope{Operation: "listGitHubRepositoryActivity", Owner: owner, Repo: repository, Limit: limit}
	query := url.Values{"direction": {"desc"}, "per_page": {strconv.Itoa(limit)}}
	current, err := applyActivityCursor(query, scope, cursor)
	if err != nil {
		return Page[Activity]{}, err
	}
	spec := client.pageSpec(
		"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repository)+"/activity",
		"/repositories/", "/activity", query, activityPagination, current, scope,
	)
	return executeProjected(client, ctx, spec, func(document []byte, headers http.Header) (Page[Activity], error) {
		entries, err := projectActivities(document, limit)
		if err != nil {
			return Page[Activity]{}, err
		}
		navigation, err := parseNavigation(headers, spec, len(entries) == 0)
		return Page[Activity]{
			Entries:    entries,
			NextCursor: navigation.nextCursor,
			PrevCursor: navigation.prevCursor,
		}, err
	})
}

func (client *Client) ListRepositoryLanguages(ctx context.Context, owner, repository string) ([]Language, error) {
	if !hasNonDotRepositoryCharacter(repository) {
		return nil, ErrUpstream
	}
	spec := client.pointSpec(
		"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repository)+"/languages",
		"/repositories/", "/languages",
	)
	return executeProjected(client, ctx, spec, func(document []byte, _ http.Header) ([]Language, error) {
		return projectLanguages(document)
	})
}

func (client *Client) ListRepositoryTags(
	ctx context.Context,
	owner, repository string,
	limit int,
	cursor *pagination.Cursor,
) (Page[Tag], error) {
	if !hasNonDotRepositoryCharacter(repository) {
		return Page[Tag]{}, ErrUpstream
	}
	scope := pagination.Scope{Operation: "listGitHubRepositoryTags", Owner: owner, Repo: repository, Limit: limit}
	query := url.Values{"per_page": {strconv.Itoa(limit)}}
	current, err := applyNumberedCursor(query, scope, cursor)
	if err != nil {
		return Page[Tag]{}, err
	}
	spec := client.pageSpec(
		"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repository)+"/tags",
		"/repositories/", "/tags", query, numberedPagination, current, scope,
	)
	return executeProjected(client, ctx, spec, func(document []byte, headers http.Header) (Page[Tag], error) {
		entries, err := projectTags(document, limit)
		if err != nil {
			return Page[Tag]{}, err
		}
		navigation, err := parseNavigation(headers, spec, len(entries) == 0)
		return Page[Tag]{Entries: entries, NextCursor: navigation.nextCursor, PrevCursor: navigation.prevCursor}, err
	})
}

func (client *Client) pointSpec(namedPath, numericPrefix, numericSuffix string) providerSpec {
	return providerSpec{
		origin:        client.origin,
		namedPath:     namedPath,
		numericPrefix: numericPrefix,
		numericSuffix: numericSuffix,
		query:         url.Values{},
	}
}

func (client *Client) pageSpec(
	namedPath, numericPrefix, numericSuffix string,
	query url.Values,
	paginationType paginationKind,
	current string,
	scope pagination.Scope,
) providerSpec {
	return providerSpec{
		origin: client.origin, namedPath: namedPath, numericPrefix: numericPrefix, numericSuffix: numericSuffix,
		query: query, pagination: paginationType, currentValue: current, scope: scope,
	}
}

func executeProjected[T any](
	client *Client,
	parent context.Context,
	spec providerSpec,
	project func([]byte, http.Header) (T, error),
) (T, error) {
	var zero T
	operationContext, cancel := context.WithTimeout(parent, providerTimeout)
	defer cancel()
	document, headers, err := client.fetch(operationContext, spec)
	if err == nil {
		result, projectionErr := project(document, headers)
		if projectionErr == nil && parent.Err() == nil && operationContext.Err() == nil {
			return result, nil
		}
		err = projectionErr
	}
	if parent.Err() != nil {
		return zero, parent.Err()
	}
	if errors.Is(operationContext.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return zero, ErrTimeout
	}
	var rateLimit *RateLimitError
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUpstream) || errors.Is(err, ErrInvalidCursor) ||
		errors.As(err, &rateLimit) {
		return zero, err
	}
	return zero, ErrUpstream
}

func (client *Client) fetch(ctx context.Context, spec providerSpec) ([]byte, http.Header, error) {
	target := cloneURL(client.origin)
	target.Path = spec.namedPath
	target.RawPath = ""
	target.RawQuery = spec.query.Encode()
	visited := map[string]struct{}{target.String(): {}}
	for redirects := 0; ; redirects++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, nil, ErrUpstream
		}
		request.Header = http.Header{
			"Accept": {"application/vnd.github+json"}, "X-Github-Api-Version": {providerAPIVersion},
			"User-Agent": {providerUserAgent}, "Accept-Encoding": {"identity"},
		}
		response, err := client.httpClient.Do(request)
		if err != nil {
			return nil, nil, err
		}
		if !identityEncoded(response.Header) {
			closeResponse(response)
			return nil, nil, ErrUpstream
		}
		if isRedirect(response.StatusCode) {
			if redirects >= maximumRedirectCount {
				closeResponse(response)
				return nil, nil, ErrUpstream
			}
			next, err := redirectTarget(target, response.Header, spec)
			closeResponse(response)
			if err != nil {
				return nil, nil, err
			}
			if _, loop := visited[next.String()]; loop {
				return nil, nil, ErrUpstream
			}
			visited[next.String()] = struct{}{}
			target = next
			continue
		}
		switch response.StatusCode {
		case http.StatusOK:
			document, err := readSuccess(response)
			return document, response.Header.Clone(), err
		case http.StatusNotFound:
			closeResponse(response)
			return nil, nil, ErrNotFound
		case http.StatusForbidden, http.StatusTooManyRequests:
			rateLimit := client.rateLimitError(response.Header)
			closeResponse(response)
			return nil, nil, rateLimit
		default:
			closeResponse(response)
			return nil, nil, ErrUpstream
		}
	}
}

func (client *Client) rateLimitError(header http.Header) *RateLimitError {
	now := client.clock()
	retryValue, retryOK := canonicalHeaderInteger(header, "Retry-After")
	resetValue, resetOK := canonicalHeaderInteger(header, "X-RateLimit-Reset")
	nowUnix := now.Unix()
	var nowSeconds uint64
	if nowUnix < 0 {
		resetOK = false
	} else {
		nowSeconds = uint64(nowUnix)
		resetOK = resetOK && resetValue > nowSeconds
	}
	retryAfter := "60"
	if retryOK {
		retryAfter = strconv.FormatUint(retryValue, 10)
	} else if resetOK {
		delay := resetValue - nowSeconds
		retryAfter = strconv.FormatUint(delay, 10)
	}
	reset := ""
	if resetOK {
		reset = strconv.FormatUint(resetValue, 10)
	}
	return &RateLimitError{RetryAfter: retryAfter, Reset: reset}
}

func applyNumberedCursor(query url.Values, scope pagination.Scope, cursor *pagination.Cursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	if cursor.Version != 1 || !cursor.Matches(scope) || cursor.Upstream != "" ||
		(cursor.Direction != "next" && cursor.Direction != "prev") || !canonicalSafeDecimal(cursor.Anchor) ||
		cursor.Direction == "next" && cursor.Anchor == "1" {
		return "", ErrInvalidCursor
	}
	query.Set("page", cursor.Anchor)
	return cursor.Anchor, nil
}

func applyActivityCursor(query url.Values, scope pagination.Scope, cursor *pagination.Cursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	if cursor.Version != 1 || !cursor.Matches(scope) || cursor.Upstream != "" ||
		(cursor.Direction != "next" && cursor.Direction != "prev") ||
		!printableASCII(cursor.Anchor, pagination.MaxCursorLength) {
		return "", ErrInvalidCursor
	}
	member := "after"
	if cursor.Direction == "prev" {
		member = "before"
	}
	query.Set(member, cursor.Anchor)
	return cursor.Anchor, nil
}

func redirectTarget(current *url.URL, header http.Header, spec providerSpec) (*url.URL, error) {
	locations := header.Values("Location")
	if len(locations) != 1 || strings.TrimSpace(locations[0]) == "" {
		return nil, ErrUpstream
	}
	reference, err := url.Parse(strings.TrimSpace(locations[0]))
	if err != nil {
		return nil, ErrUpstream
	}
	target := current.ResolveReference(reference)
	if !validProviderTarget(target, spec) {
		return nil, ErrUpstream
	}
	return target, nil
}

func validProviderTarget(target *url.URL, spec providerSpec) bool {
	if target.Scheme != spec.origin.Scheme || target.Host != spec.origin.Host || target.User != nil ||
		target.Fragment != "" || target.ForceQuery ||
		target.Path != spec.namedPath && !matchesNumericPath(target.Path, spec.numericPrefix, spec.numericSuffix) {
		return false
	}
	if target.EscapedPath() != (&url.URL{Path: target.Path}).EscapedPath() {
		return false
	}
	query, err := url.ParseQuery(target.RawQuery)
	if err != nil {
		return false
	}
	for _, values := range query {
		if len(values) != 1 {
			return false
		}
	}
	return query.Encode() == spec.query.Encode()
}

func readSuccess(response *http.Response) ([]byte, error) {
	defer func() { _ = response.Body.Close() }()
	contentTypes := response.Header.Values("Content-Type")
	if len(contentTypes) != 1 {
		return nil, ErrUpstream
	}
	mediaType, _, err := mime.ParseMediaType(contentTypes[0])
	if err != nil || mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json") {
		return nil, ErrUpstream
	}
	if response.ContentLength < -1 || response.ContentLength > maximumProviderBody {
		return nil, ErrUpstream
	}
	document, err := io.ReadAll(io.LimitReader(response.Body, maximumProviderBody+1))
	if err != nil || len(document) > maximumProviderBody {
		return nil, ErrUpstream
	}
	if _, err := portable.ParseStrictJSON(document); err != nil {
		return nil, ErrUpstream
	}
	return document, nil
}

func closeResponse(response *http.Response) { _ = response.Body.Close() }

func identityEncoded(header http.Header) bool {
	values := header.Values("Content-Encoding")
	return len(values) == 0 || len(values) == 1 && strings.EqualFold(strings.TrimSpace(values[0]), "identity")
}

func canonicalHeaderInteger(header http.Header, name string) (uint64, bool) {
	values := header.Values(name)
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return 0, false
	}
	value := strings.TrimSpace(values[0])
	if !canonicalSafeDecimal(value) {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil && parsed <= maximumSafeInteger
}

func isRedirect(status int) bool {
	return status == http.StatusMovedPermanently || status == http.StatusFound || status == http.StatusSeeOther ||
		status == http.StatusTemporaryRedirect || status == http.StatusPermanentRedirect
}

func hasNonDotRepositoryCharacter(repository string) bool {
	return strings.ContainsAny(repository, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-")
}

func cloneURL(source *url.URL) *url.URL {
	clone := *source
	return &clone
}

var _ Service = (*Client)(nil)
