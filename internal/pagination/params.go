package pagination

// Params embeds into Huma input structs for pagination.
type Params struct {
	Cursor string `query:"cursor" doc:"Opaque pagination cursor from previous response"`
	Limit  int    `query:"limit"  doc:"Maximum items per page"                          default:"20" minimum:"1" maximum:"100"`
}

// DefaultLimit returns the limit, defaulting to 20 if zero.
func (p Params) DefaultLimit() int {
	if p.Limit <= 0 {
		return 20
	}
	return p.Limit
}
