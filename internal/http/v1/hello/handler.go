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
	huma.Get(api, "/hello", getHandler)

	huma.Register(api, huma.Operation{
		OperationID:   "create-hello",
		Method:        http.MethodPost,
		Path:          "/hello",
		Summary:       "Create a personalized greeting",
		DefaultStatus: http.StatusCreated,
	}, createHandler)
}

func getHandler(ctx context.Context, _ *struct{}) (*HelloGetOutput, error) {
	obs.Logger(ctx).Info("hello get", zap.String("path", "/hello"))
	return &HelloGetOutput{Body: Data{Message: "Hello, World!"}}, nil
}

func createHandler(ctx context.Context, input *HelloCreateInput) (*HelloCreateOutput, error) {
	obs.Logger(ctx).Info("hello post", zap.String("path", "/hello"), zap.String("name", input.Body.Name))
	message := fmt.Sprintf("Hello, %s!", input.Body.Name)
	return &HelloCreateOutput{Body: Data{Message: message}}, nil
}
