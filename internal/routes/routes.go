package routes

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"go.uber.org/zap"

	appmiddleware "github.com/janisto/huma-playground/internal/middleware"
	"github.com/janisto/huma-playground/internal/respond"
)

// Register wires all HTTP routes into the provided API router.
func Register(api huma.API) {
	registerHealth(api)
}

// healthData models the success payload for the health route.
type healthData struct {
	Message string `json:"message" doc:"Health status message" example:"healthy"`
}

func registerHealth(api huma.API) {
	huma.Get(api, "/health", func(ctx context.Context, _ *struct{}) (*respond.Body[healthData], error) {
		appmiddleware.LogInfo(ctx, "health check", zap.String("path", "/health"))
		resp := respond.Success(ctx, healthData{Message: "healthy"})
		return &resp, nil
	})
}
