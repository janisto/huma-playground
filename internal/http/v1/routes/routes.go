package routes

import (
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
	prefix string,
	verifier auth.Verifier,
	profileStore profilesvc.Store,
	githubService githubsvc.Service,
) {
	api.UseMiddleware(auth.NewAuthMiddleware(api, verifier))

	hello.Register(api)
	items.Register(api, prefix)
	profile.Register(api, prefix, profileStore)
	githubhandler.Register(api, githubService, prefix)
}
