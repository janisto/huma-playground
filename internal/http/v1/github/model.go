package github

import "github.com/janisto/huma-playground/internal/platform/timeutil"

// Owner is the exact public GitHub owner projection.
type Owner struct {
	ID          uint64        `json:"id"          minimum:"0" maximum:"9007199254740991" example:"583231"`
	Login       string        `json:"login"                                              example:"octocat"                                        minLength:"1"`
	Type        string        `json:"type"                                               example:"User"                                           minLength:"1"`
	Name        *string       `json:"name"                                               example:"The Octocat"                                                  nullable:"true"`
	AvatarURL   string        `json:"avatarUrl"                                          example:"https://avatars.githubusercontent.com/u/583231"                               format:"uri"`
	HTMLURL     string        `json:"htmlUrl"                                            example:"https://github.com/octocat"                                                   format:"uri"`
	Company     *string       `json:"company"                                            example:"@github"                                                      nullable:"true"`
	Blog        *string       `json:"blog"                                               example:"https://github.blog"                                          nullable:"true"`
	Location    *string       `json:"location"                                           example:"San Francisco"                                                nullable:"true"`
	Bio         *string       `json:"bio"                                                example:"Public example biography"                                     nullable:"true"`
	PublicRepos uint64        `json:"publicRepos" minimum:"0" maximum:"9007199254740991" example:"8"`
	Followers   uint64        `json:"followers"   minimum:"0" maximum:"9007199254740991" example:"100"`
	Following   uint64        `json:"following"   minimum:"0" maximum:"9007199254740991" example:"2"`
	CreatedAt   timeutil.Time `json:"createdAt"                                          example:"2011-01-25T18:44:36.000Z"                                                     format:"date-time"`
	UpdatedAt   timeutil.Time `json:"updatedAt"                                          example:"2024-06-01T00:00:00.000Z"                                                     format:"date-time"`
}

// RepositorySummary is the exact public repository list projection.
type RepositorySummary struct {
	ID          uint64  `json:"id"          minimum:"0" maximum:"9007199254740991" example:"1296269"`
	Name        string  `json:"name"                                               example:"git-consortium"                            minLength:"1"`
	FullName    string  `json:"fullName"                                           example:"octocat/git-consortium"                    minLength:"1"`
	Description *string `json:"description"                                        example:"A public example repository"                             nullable:"true"`
	HTMLURL     string  `json:"htmlUrl"                                            example:"https://github.com/octocat/git-consortium"                               format:"uri"`
	Fork        bool    `json:"fork"                                               example:"false"`
}

// Repository is the exact public repository detail projection.
type Repository struct {
	ID              uint64         `json:"id"              minimum:"0" maximum:"9007199254740991" example:"1296269"`
	Name            string         `json:"name"                                                   example:"git-consortium"                            minLength:"1"`
	FullName        string         `json:"fullName"                                               example:"octocat/git-consortium"                    minLength:"1"`
	Description     *string        `json:"description"                                            example:"A public example repository"                             nullable:"true"`
	HTMLURL         string         `json:"htmlUrl"                                                example:"https://github.com/octocat/git-consortium"                                format:"uri"`
	Fork            bool           `json:"fork"                                                   example:"false"`
	Language        *string        `json:"language"                                               example:"Go"                                                      nullable:"true"`
	StargazersCount uint64         `json:"stargazersCount" minimum:"0" maximum:"9007199254740991" example:"16"`
	ForksCount      uint64         `json:"forksCount"      minimum:"0" maximum:"9007199254740991" example:"10"`
	OpenIssuesCount uint64         `json:"openIssuesCount" minimum:"0" maximum:"9007199254740991" example:"0"`
	Archived        bool           `json:"archived"                                               example:"false"`
	CreatedAt       timeutil.Time  `json:"createdAt"                                              example:"2011-01-25T18:44:36.000Z"                                                 format:"date-time"`
	UpdatedAt       timeutil.Time  `json:"updatedAt"                                              example:"2024-06-01T00:00:00.000Z"                                                 format:"date-time"`
	PushedAt        *timeutil.Time `json:"pushedAt"                                               example:"2024-05-31T23:00:00.000Z"                                nullable:"true"  format:"date-time"`
	DefaultBranch   string         `json:"defaultBranch"                                          example:"main"                                      minLength:"1"`
	License         *string        `json:"license"                                                example:"MIT"                                                     nullable:"true"`
	Topics          []string       `json:"topics"                                                 example:"[\"example\",\"portable-api\"]"                          nullable:"false"                    uniqueItems:"true"`
	Disabled        bool           `json:"disabled"                                               example:"false"`
}

// Activity is the exact public repository activity projection.
type Activity struct {
	ID             uint64        `json:"id"             minimum:"0" maximum:"9007199254740991" example:"1"`
	Actor          *string       `json:"actor"                                                 example:"octocat"                                        nullable:"true"`
	ActorAvatarURL *string       `json:"actorAvatarUrl"                                        example:"https://avatars.githubusercontent.com/u/583231" nullable:"true" format:"uri"`
	Ref            string        `json:"ref"                                                   example:"refs/heads/main"                                                                   minLength:"1"`
	Timestamp      timeutil.Time `json:"timestamp"                                             example:"2024-01-15T10:30:00.000Z"                                       format:"date-time"`
	ActivityType   string        `json:"activityType"                                          example:"push"                                                                              minLength:"1"`
}

// Language is one sorted public language projection.
type Language struct {
	Name  string `json:"name"  minLength:"1" example:"Go"`
	Bytes uint64 `json:"bytes"               example:"6789" minimum:"0" maximum:"9007199254740991"`
}

// Tag is one public repository tag projection.
type Tag struct {
	Name   string    `json:"name"   minLength:"1" example:"v1.0.0"`
	Commit TagCommit `json:"commit"`
}

// TagCommit contains an exact Git object identifier.
type TagCommit struct {
	SHA string `json:"sha" pattern:"^(?:[0-9a-f]{40}|[0-9a-f]{64})$" example:"0123456789abcdef0123456789abcdef01234567"`
}
