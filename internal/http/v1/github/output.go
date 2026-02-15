package github

// OwnerGetOutput is the response wrapper for GET /github/owners/{owner}.
type OwnerGetOutput struct {
	Body Owner
}

// OwnerReposListData is the response body for listing an owner's repositories.
type OwnerReposListData struct {
	Repos []RepoSummary `json:"repos" doc:"List of repositories"`
	Count int           `json:"count" doc:"Number of repositories returned" example:"1"`
}

// OwnerReposListOutput is the response wrapper for GET /github/owners/{owner}/repos.
type OwnerReposListOutput struct {
	Body OwnerReposListData
}

// RepoGetOutput is the response wrapper for GET /github/repos/{owner}/{repo}.
type RepoGetOutput struct {
	Body Repo
}

// RepoActivityListData is the response body for listing repository activity.
type RepoActivityListData struct {
	Activities []Activity `json:"activities" doc:"List of activity events"`
	Count      int        `json:"count"      doc:"Number of activities returned" example:"1"`
}

// RepoActivityListOutput is the response wrapper for GET /github/repos/{owner}/{repo}/activity.
type RepoActivityListOutput struct {
	Link string `header:"Link" doc:"RFC 8288 pagination links"`
	Body RepoActivityListData
}

// LanguagesData is the response body for repository languages.
type LanguagesData struct {
	Languages []Language `json:"languages" doc:"List of languages used"`
}

// RepoLanguagesGetOutput is the response wrapper for GET /github/repos/{owner}/{repo}/languages.
type RepoLanguagesGetOutput struct {
	Body LanguagesData
}

// RepoTagsListData is the response body for listing repository tags.
type RepoTagsListData struct {
	Tags  []Tag `json:"tags"  doc:"List of tags"`
	Count int   `json:"count" doc:"Number of tags returned" example:"1"`
}

// RepoTagsListOutput is the response wrapper for GET /github/repos/{owner}/{repo}/tags.
type RepoTagsListOutput struct {
	Body RepoTagsListData
}
