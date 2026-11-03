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

const (
	catalogLink = `</.well-known/api-catalog>; rel="api-catalog"; type="application/linkset+json"`
	schemaLink  = `</openapi.json>; rel="service-desc"; type="application/json"`
)

type output struct {
	Link []string `header:"Link" doc:"Typed API catalog and OpenAPI discovery links"`
	Body Response
}

// Register adds the portable dependency-free liveness operation.
func Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "getHealth",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Get application liveness",
		Description: "Returns process liveness without authentication or external-service calls; it does not establish Firebase readiness. " +
			"No query parameters are accepted.",
		Tags:     []string{"Health"},
		Security: []map[string][]string{},
		Errors: []int{
			http.StatusBadRequest,
			http.StatusNotAcceptable,
			http.StatusInternalServerError,
		},
	}, func(context.Context, *struct{}) (*output, error) {
		return &output{Link: []string{catalogLink, schemaLink}, Body: Response{Status: "healthy"}}, nil
	})
}

// Handler is a plain HTTP handler for the health check endpoint.
func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("Link", catalogLink)
	w.Header().Add("Link", schemaLink)
	if err := json.NewEncoder(w).Encode(Response{Status: "healthy"}); err != nil {
		return
	}
}
