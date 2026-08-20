package github

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	obs "github.com/janisto/huma-observability/v2"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/platform/pagination"
	"github.com/janisto/huma-playground/internal/platform/portable"
	"github.com/janisto/huma-playground/internal/platform/timeutil"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
)

var githubErrors = []int{
	http.StatusBadRequest,
	http.StatusNotFound,
	http.StatusNotAcceptable,
	http.StatusUnprocessableEntity,
	http.StatusTooManyRequests,
	http.StatusInternalServerError,
	http.StatusBadGateway,
	http.StatusGatewayTimeout,
}

// Register wires the six portable anonymous GitHub routes into the API.
func Register(api huma.API, service githubsvc.Service, prefix string) {
	huma.Register(api, huma.Operation{
		OperationID: "getGitHubOwner",
		Method:      http.MethodGet,
		Path:        "/github/owners/{owner}",
		Summary:     "Get a public GitHub owner",
		Tags:        []string{"GitHub"},
		Security:    []map[string][]string{},
		Errors:      githubErrors,
	}, func(ctx context.Context, input *OwnerGetInput) (*OwnerGetOutput, error) {
		owner, err := service.GetOwner(ctx, input.Owner)
		if err != nil {
			return nil, mapServiceError(ctx, "getGitHubOwner", err)
		}
		return &OwnerGetOutput{Body: toHTTPOwner(owner)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listGitHubOwnerRepositories",
		Method:      http.MethodGet,
		Path:        "/github/owners/{owner}/repos",
		Summary:     "List a public GitHub owner's repositories",
		Description: "Accepts only limit and cursor; unknown or repeated query parameters are rejected.",
		Tags:        []string{"GitHub"},
		Security:    []map[string][]string{},
		Errors:      githubErrors,
	}, func(ctx context.Context, input *OwnerRepositoriesListInput) (*OwnerRepositoriesListOutput, error) {
		limit := input.DefaultLimit()
		scope := pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: input.Owner, Limit: limit}
		cursor, err := decodeCursor(ctx, input.Cursor, scope)
		if err != nil {
			return nil, err
		}
		page, err := service.ListOwnerRepositories(ctx, input.Owner, limit, cursor)
		if err != nil {
			return nil, mapServiceError(ctx, "listGitHubOwnerRepositories", err)
		}
		return &OwnerRepositoriesListOutput{
			Link: pageLink(
				prefix+"/github/owners/"+url.PathEscape(input.Owner)+"/repos",
				limit,
				page.NextCursor,
				page.PrevCursor,
			),
			Body: RepositoryPage{Repos: toHTTPRepositorySummaries(page.Entries), Count: len(page.Entries)},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getGitHubRepository",
		Method:      http.MethodGet,
		Path:        "/github/repos/{owner}/{repo}",
		Summary:     "Get a public GitHub repository",
		Tags:        []string{"GitHub"},
		Security:    []map[string][]string{},
		Errors:      githubErrors,
	}, func(ctx context.Context, input *RepositoryGetInput) (*RepositoryGetOutput, error) {
		repository, err := service.GetRepository(ctx, input.Owner, input.Repo)
		if err != nil {
			return nil, mapServiceError(ctx, "getGitHubRepository", err)
		}
		return &RepositoryGetOutput{Body: toHTTPRepository(repository)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listGitHubRepositoryActivity",
		Method:      http.MethodGet,
		Path:        "/github/repos/{owner}/{repo}/activity",
		Summary:     "List public GitHub repository activity",
		Description: "Accepts only limit and cursor; unknown or repeated query parameters are rejected.",
		Tags:        []string{"GitHub"},
		Security:    []map[string][]string{},
		Errors:      githubErrors,
	}, func(ctx context.Context, input *RepositoryPageListInput) (*RepositoryActivityListOutput, error) {
		limit := input.DefaultLimit()
		scope := pagination.Scope{
			Operation: "listGitHubRepositoryActivity", Owner: input.Owner, Repo: input.Repo, Limit: limit,
		}
		cursor, err := decodeCursor(ctx, input.Cursor, scope)
		if err != nil {
			return nil, err
		}
		page, err := service.ListRepositoryActivity(ctx, input.Owner, input.Repo, limit, cursor)
		if err != nil {
			return nil, mapServiceError(ctx, "listGitHubRepositoryActivity", err)
		}
		return &RepositoryActivityListOutput{
			Link: pageLink(
				prefix+"/github/repos/"+url.PathEscape(input.Owner)+"/"+url.PathEscape(input.Repo)+"/activity",
				limit, page.NextCursor, page.PrevCursor,
			),
			Body: ActivityPage{Activities: toHTTPActivities(page.Entries), Count: len(page.Entries)},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listGitHubRepositoryLanguages",
		Method:      http.MethodGet,
		Path:        "/github/repos/{owner}/{repo}/languages",
		Summary:     "List public GitHub repository languages",
		Tags:        []string{"GitHub"},
		Security:    []map[string][]string{},
		Errors:      githubErrors,
	}, func(ctx context.Context, input *RepositoryGetInput) (*RepositoryLanguagesListOutput, error) {
		languages, err := service.ListRepositoryLanguages(ctx, input.Owner, input.Repo)
		if err != nil {
			return nil, mapServiceError(ctx, "listGitHubRepositoryLanguages", err)
		}
		return &RepositoryLanguagesListOutput{Body: Languages{Languages: toHTTPLanguages(languages)}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listGitHubRepositoryTags",
		Method:      http.MethodGet,
		Path:        "/github/repos/{owner}/{repo}/tags",
		Summary:     "List public GitHub repository tags",
		Description: "Accepts only limit and cursor; unknown or repeated query parameters are rejected.",
		Tags:        []string{"GitHub"},
		Security:    []map[string][]string{},
		Errors:      githubErrors,
	}, func(ctx context.Context, input *RepositoryPageListInput) (*RepositoryTagsListOutput, error) {
		limit := input.DefaultLimit()
		scope := pagination.Scope{
			Operation: "listGitHubRepositoryTags", Owner: input.Owner, Repo: input.Repo, Limit: limit,
		}
		cursor, err := decodeCursor(ctx, input.Cursor, scope)
		if err != nil {
			return nil, err
		}
		page, err := service.ListRepositoryTags(ctx, input.Owner, input.Repo, limit, cursor)
		if err != nil {
			return nil, mapServiceError(ctx, "listGitHubRepositoryTags", err)
		}
		return &RepositoryTagsListOutput{
			Link: pageLink(
				prefix+"/github/repos/"+url.PathEscape(input.Owner)+"/"+url.PathEscape(input.Repo)+"/tags",
				limit, page.NextCursor, page.PrevCursor,
			),
			Body: TagPage{Tags: toHTTPTags(page.Entries), Count: len(page.Entries)},
		}, nil
	})
}

func decodeCursor(ctx context.Context, raw string, scope pagination.Scope) (*pagination.Cursor, error) {
	if raw == "" {
		return nil, nil
	}
	cursor, err := pagination.DecodeCursor(raw)
	if err != nil || !cursor.Matches(scope) || cursor.Anchor == "" || cursor.Upstream != "" {
		return nil, portable.ErrorForContext(ctx, portable.CodeInvalidRequest)
	}
	return &cursor, nil
}

func pageLink(path string, limit int, next, previous string) string {
	return pagination.BuildLinkHeader(path, url.Values{"limit": {strconv.Itoa(limit)}}, next, previous)
}

func mapServiceError(ctx context.Context, operation string, err error) error {
	var rateLimit *githubsvc.RateLimitError
	switch {
	case errors.Is(err, githubsvc.ErrInvalidCursor):
		return portable.ErrorForContext(ctx, portable.CodeInvalidRequest)
	case errors.Is(err, githubsvc.ErrNotFound):
		return portable.ErrorForContext(ctx, portable.CodeGitHubNotFound)
	case errors.As(err, &rateLimit):
		headers := http.Header{"Retry-After": {rateLimit.RetryAfter}}
		if rateLimit.Reset != "" {
			headers.Set("X-Ratelimit-Reset", rateLimit.Reset)
		}
		return huma.ErrorWithHeaders(portable.ErrorForContext(ctx, portable.CodeGitHubRateLimit), headers)
	case errors.Is(err, githubsvc.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		obs.Logger(ctx).Warn("github dependency timed out",
			zap.String("operation", operation), zap.String("error_category", "github_timeout"))
		return portable.ErrorForContext(ctx, portable.CodeGitHubTimeout)
	case errors.Is(err, githubsvc.ErrUpstream), errors.Is(err, context.Canceled):
		obs.Logger(ctx).Warn("github dependency failed",
			zap.String("operation", operation), zap.String("error_category", "github_upstream"))
		return portable.ErrorForContext(ctx, portable.CodeGitHubUpstream)
	default:
		obs.Logger(ctx).Error("github operation failed",
			zap.String("operation", operation), zap.String("error_category", "internal_error"))
		return portable.ErrorForContext(ctx, portable.CodeInternalError)
	}
}

func toHTTPOwner(owner githubsvc.Owner) Owner {
	return Owner{
		ID: owner.ID, Login: owner.Login, Type: owner.Type, Name: owner.Name,
		AvatarURL: owner.AvatarURL, HTMLURL: owner.HTMLURL, Company: owner.Company,
		Blog: owner.Blog, Location: owner.Location, Bio: owner.Bio,
		PublicRepos: owner.PublicRepos, Followers: owner.Followers, Following: owner.Following,
		CreatedAt: timeutil.NewTime(owner.CreatedAt), UpdatedAt: timeutil.NewTime(owner.UpdatedAt),
	}
}

func toHTTPRepositorySummary(repository githubsvc.RepositorySummary) RepositorySummary {
	return RepositorySummary{
		ID: repository.ID, Name: repository.Name, FullName: repository.FullName,
		Description: repository.Description, HTMLURL: repository.HTMLURL, Fork: repository.Fork,
	}
}

func toHTTPRepositorySummaries(repositories []githubsvc.RepositorySummary) []RepositorySummary {
	result := make([]RepositorySummary, len(repositories))
	for index := range repositories {
		result[index] = toHTTPRepositorySummary(repositories[index])
	}
	return result
}

func toHTTPRepository(repository githubsvc.Repository) Repository {
	return Repository{
		ID: repository.ID, Name: repository.Name, FullName: repository.FullName,
		Description: repository.Description, HTMLURL: repository.HTMLURL, Fork: repository.Fork,
		Language: repository.Language, StargazersCount: repository.StargazersCount,
		ForksCount: repository.ForksCount, OpenIssuesCount: repository.OpenIssuesCount,
		Archived: repository.Archived, CreatedAt: timeutil.NewTime(repository.CreatedAt),
		UpdatedAt: timeutil.NewTime(repository.UpdatedAt), PushedAt: optionalTime(repository.PushedAt),
		DefaultBranch: repository.DefaultBranch, License: repository.License,
		Topics: repository.Topics, Disabled: repository.Disabled,
	}
}

func optionalTime(value *time.Time) *timeutil.Time {
	if value == nil {
		return nil
	}
	timestamp := timeutil.NewTime(*value)
	return &timestamp
}

func toHTTPActivities(activities []githubsvc.Activity) []Activity {
	result := make([]Activity, len(activities))
	for index, activity := range activities {
		result[index] = Activity{
			ID: activity.ID, Actor: activity.Actor, ActorAvatarURL: activity.ActorAvatarURL,
			Ref: activity.Ref, Timestamp: timeutil.NewTime(activity.Timestamp), ActivityType: activity.ActivityType,
		}
	}
	return result
}

func toHTTPLanguages(languages []githubsvc.Language) []Language {
	result := make([]Language, len(languages))
	for index, language := range languages {
		result[index] = Language{Name: language.Name, Bytes: language.Bytes}
	}
	return result
}

func toHTTPTags(tags []githubsvc.Tag) []Tag {
	result := make([]Tag, len(tags))
	for index, tag := range tags {
		result[index] = Tag{Name: tag.Name, Commit: TagCommit{SHA: tag.SHA}}
	}
	return result
}
