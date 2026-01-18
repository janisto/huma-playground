package items

import "github.com/janisto/huma-playground/internal/platform/pagination"

// ItemsListInput defines query parameters for listing items.
type ItemsListInput struct {
	pagination.Params
	Category string `query:"category" doc:"Filter by category" example:"electronics" enum:"electronics,tools,accessories,robotics,power,components"`
}
