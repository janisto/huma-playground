package hello

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/janisto/huma-observability/v2"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/platform/portable"
)

// Register wires hello routes into the provided API router.
func Register(api huma.API) {
	huma.Get(api, "/hello", getHandler, func(operation *huma.Operation) {
		operation.OperationID = "getHello"
		operation.Summary = "Get the default greeting"
		operation.Tags = []string{"Hello"}
		operation.Security = []map[string][]string{}
		operation.Errors = []int{http.StatusBadRequest, http.StatusNotAcceptable, http.StatusInternalServerError}
	})

	huma.Register(api, huma.Operation{
		OperationID:  "createHello",
		Method:       http.MethodPost,
		Path:         "/hello",
		Summary:      "Generate a personalized greeting",
		Tags:         []string{"Hello"},
		Security:     []map[string][]string{},
		MaxBodyBytes: portable.MaxRequestBodyBytes + 1,
		Errors: []int{
			http.StatusBadRequest,
			http.StatusNotAcceptable,
			http.StatusRequestEntityTooLarge,
			http.StatusUnsupportedMediaType,
			http.StatusUnprocessableEntity,
			http.StatusInternalServerError,
		},
	}, createHandler)
}

func getHandler(ctx context.Context, _ *struct{}) (*HelloGetOutput, error) {
	obs.Logger(ctx).Info("hello get", zap.String("path", "/hello"))
	return &HelloGetOutput{Body: Data{Message: "Hello, World!"}}, nil
}

func createHandler(ctx context.Context, input *HelloCreateInput) (*HelloCreateOutput, error) {
	obs.Logger(ctx).Info("hello post", zap.String("path", "/hello"))
	if !portable.ValidBoundedName(input.Body.Name) {
		pointer := "/name"
		return nil, portable.ErrorForContext(ctx, portable.CodeValidationFailed, portable.Issue{
			Detail: "Invalid request body member",
			Source: &portable.Source{Pointer: &pointer},
		})
	}
	message := fmt.Sprintf("Hello, %s!", input.Body.Name)
	return &HelloCreateOutput{Body: Data{Message: message}}, nil
}
