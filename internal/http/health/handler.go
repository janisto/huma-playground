package health

import (
	"encoding/json"
	"net/http"
)

// Response is the payload for the health endpoint.
type Response struct {
	Status string `json:"status"`
}

// Handler is a plain HTTP handler for the health check endpoint.
func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(Response{Status: "healthy"}); err != nil {
		return
	}
}
