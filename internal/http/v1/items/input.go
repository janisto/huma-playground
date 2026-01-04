package items

import "github.com/janisto/huma-playground/internal/platform/pagination"

// ListInput defines query parameters for listing items.
type ListInput struct {
	pagination.Params
	Category string `query:"category" doc:"Filter by category" example:"electronics" enum:"electronics,tools,accessories,robotics,power,components"`
}
