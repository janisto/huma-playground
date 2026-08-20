package health

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Response is the payload for the health endpoint.
type Response struct {
	Status string `json:"status" enum:"healthy" doc:"Dependency-free liveness status"`
}

type output struct {
	Body Response
}

// Register adds the portable dependency-free liveness operation.
func Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "getHealth",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Get application liveness",
		Tags:        []string{"Health"},
		Security:    []map[string][]string{},
		Errors: []int{
			http.StatusBadRequest,
			http.StatusNotAcceptable,
			http.StatusInternalServerError,
		},
	}, func(context.Context, *struct{}) (*output, error) {
		return &output{Body: Response{Status: "healthy"}}, nil
	})
}

// Handler is a plain HTTP handler for the health check endpoint.
func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(Response{Status: "healthy"}); err != nil {
		return
	}
}
