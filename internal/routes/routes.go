package routes

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"

	appmiddleware "github.com/janisto/huma-playground/internal/middleware"
)

// Register wires all HTTP routes into the provided API router.
func Register(api huma.API) {
	registerHealth(api)
	registerHello(api)
	registerItems(api)
}

// HealthData models the success payload for the health route.
type HealthData struct {
	Message string `json:"message" doc:"Health status message" example:"healthy"`
}

// HealthOutput is the response wrapper for the health endpoint.
type HealthOutput struct {
	Body HealthData
}

func registerHealth(api huma.API) {
	huma.Get(api, "/health", func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		appmiddleware.LogInfo(ctx, "health check", zap.String("path", "/health"))
		return &HealthOutput{Body: HealthData{Message: "healthy"}}, nil
	})
}

// HelloData models the response payload for the hello route.
type HelloData struct {
	Message string `json:"message" doc:"Greeting message" example:"Hello, World!"`
}

// HelloOutput is the response wrapper for the hello endpoint.
type HelloOutput struct {
	Body HelloData
}

// HelloInput is the request body for creating a greeting.
type HelloInput struct {
	Body struct {
		Name string `json:"name" doc:"Name to greet" example:"World" minLength:"1" maxLength:"100"`
	}
}

// HelloCreatedOutput is the response wrapper for the POST hello endpoint.
type HelloCreatedOutput struct {
	Body HelloData
}

func registerHello(api huma.API) {
	huma.Get(api, "/hello", func(ctx context.Context, _ *struct{}) (*HelloOutput, error) {
		appmiddleware.LogInfo(ctx, "hello get", zap.String("path", "/hello"))
		return &HelloOutput{Body: HelloData{Message: "Hello, World!"}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-hello",
		Method:        http.MethodPost,
		Path:          "/hello",
		Summary:       "Create a personalized greeting",
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, input *HelloInput) (*HelloCreatedOutput, error) {
		appmiddleware.LogInfo(ctx, "hello post", zap.String("path", "/hello"), zap.String("name", input.Body.Name))
		message := fmt.Sprintf("Hello, %s!", input.Body.Name)
		return &HelloCreatedOutput{Body: HelloData{Message: message}}, nil
	})
}
