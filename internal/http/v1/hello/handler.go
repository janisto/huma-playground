package hello

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"

	applog "github.com/janisto/huma-playground/internal/platform/logging"
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

func getHandler(ctx context.Context, _ *struct{}) (*GetOutput, error) {
	applog.LogInfo(ctx, "hello get", zap.String("path", "/hello"))
	return &GetOutput{Body: Data{Message: "Hello, World!"}}, nil
}

func createHandler(ctx context.Context, input *CreateInput) (*CreateOutput, error) {
	applog.LogInfo(ctx, "hello post", zap.String("path", "/hello"), zap.String("name", input.Body.Name))
	message := fmt.Sprintf("Hello, %s!", input.Body.Name)
	return &CreateOutput{Body: Data{Message: message}}, nil
}
