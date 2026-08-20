package pagination

// Params embeds into Huma input structs for pagination.
type Params struct {
	Cursor string `query:"cursor" doc:"Optional non-empty printable-ASCII opaque cursor; unknown and repeated query parameters are rejected" minLength:"1" maxLength:"2048" pattern:"^[!-~]+$"`
	Limit  int    `query:"limit"  doc:"Maximum items per page"                                                                                                                                 default:"20" minimum:"1" maximum:"100"`
}

// DefaultLimit returns the limit, defaulting to 20 if zero.
func (p Params) DefaultLimit() int {
	if p.Limit <= 0 {
		return 20
	}
	return p.Limit
}
