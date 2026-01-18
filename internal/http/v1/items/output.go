package items

// ListData is the response body containing paginated items.
type ListData struct {
	Items []Item `json:"items" doc:"List of items"`
	Total int    `json:"total" doc:"Total count of items matching the filter" example:"30"`
}

// ItemsListOutput is the response wrapper with pagination Link header.
type ItemsListOutput struct {
	Link string `header:"Link" doc:"RFC 8288 pagination links"`
	Body ListData
}
