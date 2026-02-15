package github

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Service errors
var (
	ErrNotFound    = errors.New("github resource not found")
	ErrForbidden   = errors.New("github access forbidden")
	ErrRateLimited = errors.New("github rate limit exceeded")
	ErrUpstream    = errors.New("github upstream error")
)

// UpstreamErrorKind classifies GitHub upstream failures.
type UpstreamErrorKind string

const (
	UpstreamErrorKindNotFound    UpstreamErrorKind = "not_found"
	UpstreamErrorKindForbidden   UpstreamErrorKind = "forbidden"
	UpstreamErrorKindRateLimited UpstreamErrorKind = "rate_limited"
	UpstreamErrorKindUpstream    UpstreamErrorKind = "upstream"
)

// UpstreamError includes GitHub response metadata for error mapping.
type UpstreamError struct {
	Kind           UpstreamErrorKind
	Status         int
	RetryAfter     string
	RateLimitReset string
	cause          error
}

func (e *UpstreamError) Error() string {
	if e == nil {
		return "github upstream error"
	}
	if e.cause == nil {
		return fmt.Sprintf("github upstream error (kind=%s status=%d)", e.Kind, e.Status)
	}
	return fmt.Sprintf("github upstream error (kind=%s status=%d): %v", e.Kind, e.Status, e.cause)
}

// Unwrap enables errors.Is/As against sentinel service errors.
func (e *UpstreamError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Owner represents a GitHub user or organization.
type Owner struct {
	Login     string
	Name      string
	AvatarURL string
	HTMLURL   string
	Bio       string
	Location  string
	Blog      string
	Company   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RepoSummary contains basic repository information.
type RepoSummary struct {
	Name        string
	FullName    string
	Description string
	HTMLURL     string
	Language    string
	Stars       int
	Forks       int
	OpenIssues  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Repo contains detailed repository information.
type Repo struct {
	RepoSummary
	DefaultBranch string
	License       string
	Topics        []string
	Archived      bool
	Disabled      bool
}

// Activity represents a repository activity event.
type Activity struct {
	ID             int64
	Actor          string
	Ref            string
	Timestamp      time.Time
	ActivityType   string
	ActorAvatarURL string
}

// ActivityPage holds a page of activity results with cursor for next page.
type ActivityPage struct {
	Activities []Activity
	NextCursor string
}

// Tag represents a repository tag.
type Tag struct {
	Name   string
	Commit TagCommit
}

// TagCommit contains the commit SHA for a tag.
type TagCommit struct {
	SHA string
}

// Service defines GitHub API operations.
type Service interface {
	GetOwner(ctx context.Context, owner string) (*Owner, error)
	ListRepos(ctx context.Context, owner string) ([]RepoSummary, error)
	GetRepo(ctx context.Context, owner, repo string) (*Repo, error)
	ListActivity(ctx context.Context, owner, repo string, limit int, afterCursor string) (*ActivityPage, error)
	ListLanguages(ctx context.Context, owner, repo string) (map[string]int64, error)
	ListTags(ctx context.Context, owner, repo string) ([]Tag, error)
}
