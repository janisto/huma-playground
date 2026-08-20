package github

type OwnerGetOutput struct {
	Body Owner
}

type RepositoryPage struct {
	Repos []RepositorySummary `json:"repos" nullable:"false" maxItems:"100"`
	Count int                 `json:"count"                                 minimum:"0" maximum:"100" example:"1"`
}

type OwnerRepositoriesListOutput struct {
	Link string `header:"Link" doc:"Optional RFC 8288 next and previous navigation"`
	Body RepositoryPage
}

type RepositoryGetOutput struct {
	Body Repository
}

type ActivityPage struct {
	Activities []Activity `json:"activities" nullable:"false" maxItems:"100"`
	Count      int        `json:"count"                                      minimum:"0" maximum:"100" example:"1"`
}

type RepositoryActivityListOutput struct {
	Link string `header:"Link" doc:"Optional RFC 8288 next and previous navigation"`
	Body ActivityPage
}

type Languages struct {
	Languages []Language `json:"languages" nullable:"false"`
}

type RepositoryLanguagesListOutput struct {
	Body Languages
}

type TagPage struct {
	Tags  []Tag `json:"tags"  nullable:"false" maxItems:"100"`
	Count int   `json:"count"                                 minimum:"0" maximum:"100" example:"1"`
}

type RepositoryTagsListOutput struct {
	Link string `header:"Link" doc:"Optional RFC 8288 next and previous navigation"`
	Body TagPage
}
