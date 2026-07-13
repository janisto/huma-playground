package github

import "github.com/janisto/huma-playground/internal/platform/pagination"

// OwnerGetInput defines path parameters for retrieving a GitHub owner.
type OwnerGetInput struct {
	Owner string `path:"owner" doc:"GitHub account or organization login" example:"octocat" maxLength:"39" pattern:"^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$"`
}

// RepoGetInput defines path parameters for retrieving a GitHub repository.
type RepoGetInput struct {
	Owner string `path:"owner" doc:"GitHub account or organization login" example:"octocat"        maxLength:"39"  pattern:"^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$"`
	Repo  string `path:"repo"  doc:"Repository name"                      example:"git-consortium" maxLength:"100" pattern:"^[a-zA-Z0-9_.-]*[a-zA-Z0-9_-][a-zA-Z0-9_.-]*$"`
}

// RepoActivityListInput defines path and query parameters for listing repository activity.
type RepoActivityListInput struct {
	pagination.Params
	Owner string `path:"owner" doc:"GitHub account or organization login" example:"octocat"        maxLength:"39"  pattern:"^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$"`
	Repo  string `path:"repo"  doc:"Repository name"                      example:"git-consortium" maxLength:"100" pattern:"^[a-zA-Z0-9_.-]*[a-zA-Z0-9_-][a-zA-Z0-9_.-]*$"`
}
