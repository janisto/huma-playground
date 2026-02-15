package github

import (
	"context"
	"errors"
	"testing"
)

func TestMockGetOwner(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	owner, err := svc.GetOwner(ctx, "octocat")
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
}

func TestMockGetOwnerNotFound(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	_, err := svc.GetOwner(ctx, "unknown-user")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockListRepos(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	repos, err := svc.ListRepos(ctx, "octocat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].Name != "git-consortium" {
		t.Errorf("expected repo git-consortium, got %s", repos[0].Name)
	}
}

func TestMockListReposNotFound(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	_, err := svc.ListRepos(ctx, "unknown-user")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockGetRepo(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	repo, err := svc.GetRepo(ctx, "octocat", "git-consortium")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Name != "git-consortium" {
		t.Errorf("expected name git-consortium, got %s", repo.Name)
	}
	if repo.DefaultBranch != "master" {
		t.Errorf("expected default branch master, got %s", repo.DefaultBranch)
	}
	if repo.License != "MIT License" {
		t.Errorf("expected license MIT License, got %s", repo.License)
	}
}

func TestMockGetRepoUnknownOwner(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	_, err := svc.GetRepo(ctx, "unknown-user", "git-consortium")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockGetRepoUnknownRepo(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	_, err := svc.GetRepo(ctx, "octocat", "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockListActivity(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	page, err := svc.ListActivity(ctx, "octocat", "git-consortium", 10, "")
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
	if page.NextCursor != "" {
		t.Errorf("expected empty next cursor, got %s", page.NextCursor)
	}
}

func TestMockListActivityNotFound(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	_, err := svc.ListActivity(ctx, "unknown-user", "repo", 10, "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockListLanguages(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	langs, err := svc.ListLanguages(ctx, "octocat", "git-consortium")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(langs) != 1 {
		t.Fatalf("expected 1 language, got %d", len(langs))
	}
	if langs["Ruby"] != 6789 {
		t.Errorf("expected Ruby=6789, got %d", langs["Ruby"])
	}
}

func TestMockListLanguagesNotFound(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	_, err := svc.ListLanguages(ctx, "unknown-user", "repo")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockListTags(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	tags, err := svc.ListTags(ctx, "octocat", "git-consortium")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Name != "v1.0" {
		t.Errorf("expected tag v1.0, got %s", tags[0].Name)
	}
	if tags[0].Commit.SHA != "abc123" {
		t.Errorf("expected sha abc123, got %s", tags[0].Commit.SHA)
	}
}

func TestMockListTagsNotFound(t *testing.T) {
	svc := NewMockGitHubService()
	ctx := context.Background()

	_, err := svc.ListTags(ctx, "unknown-user", "repo")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockInterfaceCompliance(t *testing.T) {
	var _ Service = (*MockGitHubService)(nil)
}
