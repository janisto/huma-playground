package github

import (
	"github.com/janisto/huma-playground/internal/platform/timeutil"
)

// Owner represents a GitHub user or organization.
type Owner struct {
	Login     string        `json:"login"     doc:"GitHub username"     example:"octocat"`
	Name      string        `json:"name"      doc:"Display name"        example:"The Octocat"`
	AvatarURL string        `json:"avatarUrl" doc:"Avatar image URL"    example:"https://avatars.githubusercontent.com/u/583231"`
	HTMLURL   string        `json:"htmlUrl"   doc:"GitHub profile URL"  example:"https://github.com/octocat"`
	Bio       string        `json:"bio"       doc:"Profile biography"   example:""`
	Location  string        `json:"location"  doc:"Geographic location" example:"San Francisco"`
	Blog      string        `json:"blog"      doc:"Blog URL"            example:"https://github.blog"`
	Company   string        `json:"company"   doc:"Company name"        example:"@github"`
	CreatedAt timeutil.Time `json:"createdAt" doc:"Account creation"    example:"2011-01-25T18:44:36.000Z"`
	UpdatedAt timeutil.Time `json:"updatedAt" doc:"Last profile update" example:"2024-06-01T00:00:00.000Z"`
}

// RepoSummary contains basic repository information.
type RepoSummary struct {
	Name        string        `json:"name"        doc:"Repository name"                   example:"git-consortium"`
	FullName    string        `json:"fullName"    doc:"Full repository name (owner/repo)" example:"octocat/git-consortium"`
	Description string        `json:"description" doc:"Repository description"`
	HTMLURL     string        `json:"htmlUrl"     doc:"GitHub repository URL"             example:"https://github.com/octocat/git-consortium"`
	Language    string        `json:"language"    doc:"Primary language"                  example:"Ruby"`
	Stars       int           `json:"stars"       doc:"Stargazer count"                   example:"16"`
	Forks       int           `json:"forks"       doc:"Fork count"                        example:"10"`
	OpenIssues  int           `json:"openIssues"  doc:"Open issue count"                  example:"0"`
	CreatedAt   timeutil.Time `json:"createdAt"   doc:"Creation timestamp"                example:"2011-01-25T18:44:36.000Z"`
	UpdatedAt   timeutil.Time `json:"updatedAt"   doc:"Last update timestamp"             example:"2024-06-01T00:00:00.000Z"`
}

// Repo contains detailed repository information.
type Repo struct {
	RepoSummary
	DefaultBranch string   `json:"defaultBranch" doc:"Default branch name"      example:"master"`
	License       string   `json:"license"       doc:"License name"             example:"MIT License"`
	Topics        []string `json:"topics"        doc:"Repository topics"`
	Archived      bool     `json:"archived"      doc:"Whether repo is archived" example:"false"`
	Disabled      bool     `json:"disabled"      doc:"Whether repo is disabled" example:"false"`
}

// Activity represents a repository activity event.
type Activity struct {
	ID             int64         `json:"id"             doc:"Activity ID"      example:"1"`
	Actor          string        `json:"actor"          doc:"Actor username"   example:"octocat"`
	Ref            string        `json:"ref"            doc:"Git reference"    example:"refs/heads/master"`
	Timestamp      timeutil.Time `json:"timestamp"      doc:"Event timestamp"  example:"2024-01-15T10:30:00.000Z"`
	ActivityType   string        `json:"activityType"   doc:"Type of activity" example:"push"`
	ActorAvatarURL string        `json:"actorAvatarUrl" doc:"Actor avatar URL" example:"https://avatars.githubusercontent.com/u/583231"`
}

// Tag represents a repository tag.
type Tag struct {
	Name   string    `json:"name"   doc:"Tag name"   example:"v1.0"`
	Commit TagCommit `json:"commit" doc:"Tag commit"`
}

// TagCommit contains the commit SHA for a tag.
type TagCommit struct {
	SHA string `json:"sha" doc:"Commit SHA" example:"abc123"`
}

// Language represents a programming language used in a repository.
type Language struct {
	Name  string `json:"name"  doc:"Language name" example:"Ruby"`
	Bytes int64  `json:"bytes" doc:"Bytes of code" example:"6789"`
}
