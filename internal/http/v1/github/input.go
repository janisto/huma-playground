package github

import "github.com/janisto/huma-playground/internal/platform/pagination"

// OwnerGetInput defines path parameters for retrieving a GitHub owner.
type OwnerGetInput struct {
	Owner string `path:"owner" doc:"GitHub username" default:"octocat" example:"octocat" pattern:"^[a-zA-Z0-9][a-zA-Z0-9\\-\\.]{0,38}$"`
}

// RepoGetInput defines path parameters for retrieving a GitHub repository.
type RepoGetInput struct {
	Owner string `path:"owner" doc:"GitHub username" default:"octocat"        example:"octocat"        pattern:"^[a-zA-Z0-9][a-zA-Z0-9\\-\\.]{0,38}$"`
	Repo  string `path:"repo"  doc:"Repository name" default:"git-consortium" example:"git-consortium" pattern:"^[a-zA-Z0-9_\\-\\.]{1,100}$"`
}

// RepoActivityListInput defines path and query parameters for listing repository activity.
type RepoActivityListInput struct {
	pagination.Params
	Owner string `path:"owner" doc:"GitHub username" default:"octocat"        example:"octocat"        pattern:"^[a-zA-Z0-9][a-zA-Z0-9\\-\\.]{0,38}$"`
	Repo  string `path:"repo"  doc:"Repository name" default:"git-consortium" example:"git-consortium" pattern:"^[a-zA-Z0-9_\\-\\.]{1,100}$"`
}
