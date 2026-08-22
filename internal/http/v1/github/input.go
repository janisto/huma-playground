package github

import "github.com/janisto/huma-playground/internal/platform/pagination"

// OwnerGetInput defines the path parameter for an owner point read.
type OwnerGetInput struct {
	Owner string `path:"owner" minLength:"1" maxLength:"39" pattern:"^([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9_-]{0,37}[A-Za-z0-9])$" doc:"Safe GitHub account or organization login" example:"octocat"`
}

// OwnerRepositoriesListInput defines the path and closed pagination query.
type OwnerRepositoriesListInput struct {
	pagination.Params
	Owner string `path:"owner" minLength:"1" maxLength:"39" pattern:"^([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9_-]{0,37}[A-Za-z0-9])$" doc:"Safe GitHub account or organization login" example:"octocat"`
}

// RepositoryGetInput defines owner and repository path parameters.
type RepositoryGetInput struct {
	Owner string `path:"owner" minLength:"1" maxLength:"39"  pattern:"^([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9_-]{0,37}[A-Za-z0-9])$" doc:"Safe GitHub account or organization login" example:"octocat"`
	Repo  string `path:"repo"  minLength:"1" maxLength:"100" pattern:"^[A-Za-z0-9._-]*[A-Za-z0-9_-][A-Za-z0-9._-]*$"             doc:"Safe non-dot-only repository name"         example:"git-consortium"`
}

// RepositoryPageListInput defines a repository collection path and pagination query.
type RepositoryPageListInput struct {
	pagination.Params
	Owner string `path:"owner" minLength:"1" maxLength:"39"  pattern:"^([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9_-]{0,37}[A-Za-z0-9])$" doc:"Safe GitHub account or organization login" example:"octocat"`
	Repo  string `path:"repo"  minLength:"1" maxLength:"100" pattern:"^[A-Za-z0-9._-]*[A-Za-z0-9_-][A-Za-z0-9._-]*$"             doc:"Safe non-dot-only repository name"         example:"git-consortium"`
}
