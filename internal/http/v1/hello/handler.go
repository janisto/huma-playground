package hello

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/janisto/huma-observability"
	"go.uber.org/zap"
)

// Register wires hello routes into the provided API router.
func Register(api huma.API) {
	huma.Get(api, "/hello", getHandler, func(operation *huma.Operation) {
		operation.OperationID = "get-hello"
		operation.Summary = "Get the default greeting"
		operation.Tags = []string{"Hello"}
		operation.Errors = []int{http.StatusUnprocessableEntity}
	})

	huma.Register(api, huma.Operation{
		OperationID: "generate-hello",
		Method:      http.MethodPost,
		Path:        "/hello",
		Summary:     "Generate a personalized greeting",
		Tags:        []string{"Hello"},
		Errors: []int{
			http.StatusBadRequest,
			http.StatusRequestTimeout,
			http.StatusRequestEntityTooLarge,
			http.StatusUnsupportedMediaType,
			http.StatusUnprocessableEntity,
		},
	}, createHandler)
}

func getHandler(ctx context.Context, _ *struct{}) (*HelloGetOutput, error) {
	obs.Logger(ctx).Info("hello get", zap.String("path", "/hello"))
	return &HelloGetOutput{Body: Data{Message: "Hello, World!"}}, nil
}

func createHandler(ctx context.Context, input *HelloCreateInput) (*HelloCreateOutput, error) {
	obs.Logger(ctx).Info("hello post", zap.String("path", "/hello"))
	message := fmt.Sprintf("Hello, %s!", input.Body.Name)
	return &HelloCreateOutput{Body: Data{Message: message}}, nil
}
