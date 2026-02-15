package github

import (
	"context"
	"time"
)

// MockGitHubService implements Service for unit tests with pre-populated demo data.
type MockGitHubService struct {
	owners     map[string]*Owner
	repos      map[string]map[string]*Repo
	activities map[string]map[string][]Activity
	languages  map[string]map[string]map[string]int64
	tags       map[string]map[string][]Tag
}

// NewMockGitHubService creates a mock pre-populated with octocat / git-consortium demo data.
func NewMockGitHubService() *MockGitHubService {
	created := time.Date(2011, 1, 25, 18, 44, 36, 0, time.UTC)
	updated := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	m := &MockGitHubService{
		owners: map[string]*Owner{
			"octocat": {
				Login:     "octocat",
				Name:      "The Octocat",
				AvatarURL: "https://avatars.githubusercontent.com/u/583231",
				HTMLURL:   "https://github.com/octocat",
				Bio:       "",
				Location:  "San Francisco",
				Blog:      "https://github.blog",
				Company:   "@github",
				CreatedAt: created,
				UpdatedAt: updated,
			},
		},
		repos: map[string]map[string]*Repo{
			"octocat": {
				"git-consortium": {
					RepoSummary: RepoSummary{
						Name:        "git-consortium",
						FullName:    "octocat/git-consortium",
						Description: "This repo is for demonstration purposes.",
						HTMLURL:     "https://github.com/octocat/git-consortium",
						Language:    "Ruby",
						Stars:       16,
						Forks:       10,
						OpenIssues:  0,
						CreatedAt:   created,
						UpdatedAt:   updated,
					},
					DefaultBranch: "master",
					License:       "MIT License",
					Topics:        []string{},
					Archived:      false,
					Disabled:      false,
				},
			},
		},
		activities: map[string]map[string][]Activity{
			"octocat": {
				"git-consortium": {
					{
						ID:             1,
						Actor:          "octocat",
						Ref:            "refs/heads/master",
						Timestamp:      time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
						ActivityType:   "push",
						ActorAvatarURL: "https://avatars.githubusercontent.com/u/583231",
					},
				},
			},
		},
		languages: map[string]map[string]map[string]int64{
			"octocat": {
				"git-consortium": {
					"Ruby": 6789,
				},
			},
		},
		tags: map[string]map[string][]Tag{
			"octocat": {
				"git-consortium": {
					{
						Name:   "v1.0",
						Commit: TagCommit{SHA: "abc123"},
					},
				},
			},
		},
	}
	return m
}

func (m *MockGitHubService) GetOwner(_ context.Context, owner string) (*Owner, error) {
	o, ok := m.owners[owner]
	if !ok {
		return nil, ErrNotFound
	}
	return o, nil
}

func (m *MockGitHubService) ListRepos(_ context.Context, owner string) ([]RepoSummary, error) {
	ownerRepos, ok := m.repos[owner]
	if !ok {
		return nil, ErrNotFound
	}
	summaries := make([]RepoSummary, 0, len(ownerRepos))
	for _, r := range ownerRepos {
		summaries = append(summaries, r.RepoSummary)
	}
	return summaries, nil
}

func (m *MockGitHubService) GetRepo(_ context.Context, owner, repo string) (*Repo, error) {
	ownerRepos, ok := m.repos[owner]
	if !ok {
		return nil, ErrNotFound
	}
	r, ok := ownerRepos[repo]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (m *MockGitHubService) ListActivity(
	_ context.Context, owner, repo string, _ int, _ string,
) (*ActivityPage, error) {
	ownerActivities, ok := m.activities[owner]
	if !ok {
		return nil, ErrNotFound
	}
	repoActivities, ok := ownerActivities[repo]
	if !ok {
		return nil, ErrNotFound
	}
	return &ActivityPage{
		Activities: repoActivities,
		NextCursor: "",
	}, nil
}

func (m *MockGitHubService) ListLanguages(_ context.Context, owner, repo string) (map[string]int64, error) {
	ownerLangs, ok := m.languages[owner]
	if !ok {
		return nil, ErrNotFound
	}
	repoLangs, ok := ownerLangs[repo]
	if !ok {
		return nil, ErrNotFound
	}
	return repoLangs, nil
}

func (m *MockGitHubService) ListTags(_ context.Context, owner, repo string) ([]Tag, error) {
	ownerTags, ok := m.tags[owner]
	if !ok {
		return nil, ErrNotFound
	}
	repoTags, ok := ownerTags[repo]
	if !ok {
		return nil, ErrNotFound
	}
	return repoTags, nil
}

// Compile-time interface check
var _ Service = (*MockGitHubService)(nil)
