package routes

import (
	"net/url"

	"github.com/danielgtaylor/huma/v2"

	githubhandler "github.com/janisto/huma-playground/internal/http/v1/github"
	"github.com/janisto/huma-playground/internal/http/v1/hello"
	"github.com/janisto/huma-playground/internal/http/v1/items"
	"github.com/janisto/huma-playground/internal/http/v1/profile"
	"github.com/janisto/huma-playground/internal/platform/auth"
	githubsvc "github.com/janisto/huma-playground/internal/service/github"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

// Register wires all HTTP routes into the provided API router.
func Register(
	api huma.API,
	verifier auth.Verifier,
	profileService profilesvc.Service,
	githubService githubsvc.Service,
) {
	prefix := apiPrefix(api)

	// Apply auth middleware for protected endpoints
	api.UseMiddleware(auth.NewAuthMiddleware(api, verifier))

	hello.Register(api)
	items.Register(api, prefix)
	profile.Register(api, profileService)
	githubhandler.Register(api, githubService, prefix)
}

func apiPrefix(api huma.API) string {
	for _, s := range api.OpenAPI().Servers {
		if u, err := url.Parse(s.URL); err == nil && u.Path != "" {
			return u.Path
		}
	}
	return ""
}
