package routes

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/janisto/huma-playground/internal/http/v1/hello"
	"github.com/janisto/huma-playground/internal/http/v1/items"
)

// Register wires all HTTP routes into the provided API router.
func Register(api huma.API) {
	hello.Register(api)
	items.Register(api)
}
