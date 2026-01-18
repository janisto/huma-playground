package routes

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/janisto/huma-playground/internal/http/v1/hello"
	"github.com/janisto/huma-playground/internal/http/v1/items"
	"github.com/janisto/huma-playground/internal/http/v1/profile"
	"github.com/janisto/huma-playground/internal/platform/auth"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

// Register wires all HTTP routes into the provided API router.
func Register(api huma.API, verifier auth.Verifier, profileService profilesvc.Service) {
	// Apply auth middleware for protected endpoints
	api.UseMiddleware(auth.NewAuthMiddleware(api, verifier))

	hello.Register(api)
	items.Register(api)
	profile.Register(api, profileService)
}
