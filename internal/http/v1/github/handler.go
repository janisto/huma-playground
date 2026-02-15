package github

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/janisto/huma-playground/internal/platform/pagination"
	"github.com/janisto/huma-playground/internal/platform/timeutil"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
)

const activityCursorType = "gh-activity"

// Register wires GitHub routes into the provided API router.
func Register(api huma.API, svc githubsvc.Service, prefix string) {
	huma.Register(api, huma.Operation{
		OperationID: "get-github-owner",
		Method:      http.MethodGet,
		Path:        "/github/owners/{owner}",
		Summary:     "Get a GitHub user or organization",
		Description: "Returns public profile information for the specified GitHub user or organization.",
		Tags:        []string{"GitHub"},
	}, func(ctx context.Context, input *OwnerGetInput) (*OwnerGetOutput, error) {
		owner, err := svc.GetOwner(ctx, input.Owner)
		if err != nil {
			return nil, mapServiceError(err)
		}
		result := toHTTPOwner(owner)
		return &OwnerGetOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-github-owner-repos",
		Method:      http.MethodGet,
		Path:        "/github/owners/{owner}/repos",
		Summary:     "List repositories for a GitHub user",
		Description: "Returns up to 30 repositories for the specified GitHub user or organization.",
		Tags:        []string{"GitHub"},
	}, func(ctx context.Context, input *OwnerGetInput) (*OwnerReposListOutput, error) {
		repos, err := svc.ListRepos(ctx, input.Owner)
		if err != nil {
			return nil, mapServiceError(err)
		}
		httpRepos := toHTTPRepoSummaries(repos)
		return &OwnerReposListOutput{Body: OwnerReposListData{
			Repos: httpRepos,
			Count: len(httpRepos),
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-github-repo",
		Method:      http.MethodGet,
		Path:        "/github/repos/{owner}/{repo}",
		Summary:     "Get a GitHub repository",
		Description: "Returns detailed information for the specified GitHub repository.",
		Tags:        []string{"GitHub"},
	}, func(ctx context.Context, input *RepoGetInput) (*RepoGetOutput, error) {
		repo, err := svc.GetRepo(ctx, input.Owner, input.Repo)
		if err != nil {
			return nil, mapServiceError(err)
		}
		result := toHTTPRepo(repo)
		return &RepoGetOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-github-repo-activity",
		Method:      http.MethodGet,
		Path:        "/github/repos/{owner}/{repo}/activity",
		Summary:     "List repository activity",
		Description: "Returns paginated activity events for the specified GitHub repository.",
		Tags:        []string{"GitHub"},
	}, func(ctx context.Context, input *RepoActivityListInput) (*RepoActivityListOutput, error) {
		cursor, err := pagination.DecodeCursor(input.Cursor)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid cursor format")
		}

		if cursor.Type != "" && cursor.Type != activityCursorType {
			return nil, huma.Error400BadRequest("cursor type mismatch")
		}

		page, err := svc.ListActivity(ctx, input.Owner, input.Repo, input.DefaultLimit(), cursor.Value)
		if err != nil {
			return nil, mapServiceError(err)
		}

		var linkHeader string
		if page.NextCursor != "" {
			nextEncoded := pagination.Cursor{
				Type:  activityCursorType,
				Value: page.NextCursor,
			}.Encode()
			linkHeader = pagination.BuildLinkHeader(
				prefix+"/github/repos/"+url.PathEscape(input.Owner)+"/"+url.PathEscape(input.Repo)+"/activity",
				url.Values{"limit": {strconv.Itoa(input.DefaultLimit())}},
				nextEncoded,
				"",
			)
		}

		httpActivities := toHTTPActivities(page.Activities)
		return &RepoActivityListOutput{
			Link: linkHeader,
			Body: RepoActivityListData{
				Activities: httpActivities,
				Count:      len(httpActivities),
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-github-repo-languages",
		Method:      http.MethodGet,
		Path:        "/github/repos/{owner}/{repo}/languages",
		Summary:     "Get repository languages",
		Description: "Returns programming languages used in the specified repository with byte counts.",
		Tags:        []string{"GitHub"},
	}, func(ctx context.Context, input *RepoGetInput) (*RepoLanguagesGetOutput, error) {
		languages, err := svc.ListLanguages(ctx, input.Owner, input.Repo)
		if err != nil {
			return nil, mapServiceError(err)
		}
		return &RepoLanguagesGetOutput{Body: LanguagesData{
			Languages: toHTTPLanguages(languages),
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-github-repo-tags",
		Method:      http.MethodGet,
		Path:        "/github/repos/{owner}/{repo}/tags",
		Summary:     "List repository tags",
		Description: "Returns up to 30 tags for the specified GitHub repository.",
		Tags:        []string{"GitHub"},
	}, func(ctx context.Context, input *RepoGetInput) (*RepoTagsListOutput, error) {
		tags, err := svc.ListTags(ctx, input.Owner, input.Repo)
		if err != nil {
			return nil, mapServiceError(err)
		}
		httpTags := toHTTPTags(tags)
		return &RepoTagsListOutput{Body: RepoTagsListData{
			Tags:  httpTags,
			Count: len(httpTags),
		}}, nil
	})
}

func mapServiceError(err error) error {
	var upstreamErr *githubsvc.UpstreamError

	if errors.As(err, &upstreamErr) {
		switch upstreamErr.Kind {
		case githubsvc.UpstreamErrorKindNotFound:
			return huma.Error404NotFound("resource not found")
		case githubsvc.UpstreamErrorKindRateLimited:
			rateLimitErr := huma.Error429TooManyRequests("rate limit exceeded")
			headers := make(http.Header)
			if upstreamErr.RetryAfter != "" {
				headers.Set("Retry-After", upstreamErr.RetryAfter)
			}
			if upstreamErr.RateLimitReset != "" {
				headers.Set("X-RateLimit-Reset", upstreamErr.RateLimitReset)
			}
			if len(headers) > 0 {
				return huma.ErrorWithHeaders(rateLimitErr, headers)
			}
			return rateLimitErr
		case githubsvc.UpstreamErrorKindForbidden:
			return huma.Error403Forbidden("access denied")
		default:
			return huma.Error502BadGateway("upstream error")
		}
	}

	switch {
	case errors.Is(err, githubsvc.ErrNotFound):
		return huma.Error404NotFound("resource not found")
	case errors.Is(err, githubsvc.ErrRateLimited):
		rateLimitErr := huma.Error429TooManyRequests("rate limit exceeded")
		return rateLimitErr
	case errors.Is(err, githubsvc.ErrForbidden):
		return huma.Error403Forbidden("access denied")
	default:
		return huma.Error502BadGateway("upstream error")
	}
}

func toHTTPOwner(o *githubsvc.Owner) Owner {
	return Owner{
		Login:     o.Login,
		Name:      o.Name,
		AvatarURL: o.AvatarURL,
		HTMLURL:   o.HTMLURL,
		Bio:       o.Bio,
		Location:  o.Location,
		Blog:      o.Blog,
		Company:   o.Company,
		CreatedAt: timeutil.Time{Time: o.CreatedAt},
		UpdatedAt: timeutil.Time{Time: o.UpdatedAt},
	}
}

func toHTTPRepoSummary(r *githubsvc.RepoSummary) RepoSummary {
	return RepoSummary{
		Name:        r.Name,
		FullName:    r.FullName,
		Description: r.Description,
		HTMLURL:     r.HTMLURL,
		Language:    r.Language,
		Stars:       r.Stars,
		Forks:       r.Forks,
		OpenIssues:  r.OpenIssues,
		CreatedAt:   timeutil.Time{Time: r.CreatedAt},
		UpdatedAt:   timeutil.Time{Time: r.UpdatedAt},
	}
}

func toHTTPRepoSummaries(repos []githubsvc.RepoSummary) []RepoSummary {
	result := make([]RepoSummary, len(repos))
	for i := range repos {
		result[i] = toHTTPRepoSummary(&repos[i])
	}
	return result
}

func toHTTPRepo(r *githubsvc.Repo) Repo {
	topics := r.Topics
	if topics == nil {
		topics = []string{}
	}
	return Repo{
		RepoSummary:   toHTTPRepoSummary(&r.RepoSummary),
		DefaultBranch: r.DefaultBranch,
		License:       r.License,
		Topics:        topics,
		Archived:      r.Archived,
		Disabled:      r.Disabled,
	}
}

func toHTTPActivity(a *githubsvc.Activity) Activity {
	return Activity{
		ID:             a.ID,
		Actor:          a.Actor,
		Ref:            a.Ref,
		Timestamp:      timeutil.Time{Time: a.Timestamp},
		ActivityType:   a.ActivityType,
		ActorAvatarURL: a.ActorAvatarURL,
	}
}

func toHTTPActivities(activities []githubsvc.Activity) []Activity {
	result := make([]Activity, len(activities))
	for i := range activities {
		result[i] = toHTTPActivity(&activities[i])
	}
	return result
}

func toHTTPTag(t *githubsvc.Tag) Tag {
	return Tag{
		Name:   t.Name,
		Commit: TagCommit{SHA: t.Commit.SHA},
	}
}

func toHTTPTags(tags []githubsvc.Tag) []Tag {
	result := make([]Tag, len(tags))
	for i := range tags {
		result[i] = toHTTPTag(&tags[i])
	}
	return result
}

func toHTTPLanguages(languages map[string]int64) []Language {
	result := make([]Language, 0, len(languages))
	for name, bytes := range languages {
		result = append(result, Language{Name: name, Bytes: bytes})
	}
	slices.SortFunc(result, func(a, b Language) int {
		return cmp.Compare(b.Bytes, a.Bytes)
	})
	return result
}
