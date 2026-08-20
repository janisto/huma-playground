// Package github implements the anonymous, fixed-origin GitHub provider boundary.
package github

import (
	"context"
	"errors"
	"time"

	"github.com/janisto/huma-playground/internal/platform/pagination"
)

var (
	ErrInvalidCursor = errors.New("invalid GitHub cursor")
	ErrNotFound      = errors.New("GitHub resource not found")
	ErrUpstream      = errors.New("GitHub upstream response is invalid or unavailable")
	ErrTimeout       = errors.New("GitHub request timed out")
)

type RateLimitError struct {
	RetryAfter string
	Reset      string
}

func (*RateLimitError) Error() string { return "GitHub rate limit exceeded" }

type Owner struct {
	ID          uint64
	Login       string
	Type        string
	Name        *string
	AvatarURL   string
	HTMLURL     string
	Company     *string
	Blog        *string
	Location    *string
	Bio         *string
	PublicRepos uint64
	Followers   uint64
	Following   uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type RepositorySummary struct {
	ID          uint64
	Name        string
	FullName    string
	Description *string
	HTMLURL     string
	Fork        bool
}

type Repository struct {
	RepositorySummary
	Language        *string
	StargazersCount uint64
	ForksCount      uint64
	OpenIssuesCount uint64
	Archived        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	PushedAt        *time.Time
	DefaultBranch   string
	License         *string
	Topics          []string
	Disabled        bool
}

type Activity struct {
	ID             uint64
	Actor          *string
	ActorAvatarURL *string
	Ref            string
	Timestamp      time.Time
	ActivityType   string
}

type Language struct {
	Name  string
	Bytes uint64
}

type Tag struct {
	Name string
	SHA  string
}

type Page[T any] struct {
	Entries    []T
	NextCursor string
	PrevCursor string
}

type Service interface {
	GetOwner(context.Context, string) (Owner, error)
	ListOwnerRepositories(context.Context, string, int, *pagination.Cursor) (Page[RepositorySummary], error)
	GetRepository(context.Context, string, string) (Repository, error)
	ListRepositoryActivity(context.Context, string, string, int, *pagination.Cursor) (Page[Activity], error)
	ListRepositoryLanguages(context.Context, string, string) ([]Language, error)
	ListRepositoryTags(context.Context, string, string, int, *pagination.Cursor) (Page[Tag], error)
}
