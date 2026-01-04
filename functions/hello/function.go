// Package hello provides an HTTP Cloud Function example.
package hello

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

// RFC3339Millis matches the main project's timestamp format.
const RFC3339Millis = "2006-01-02T15:04:05.000Z"

func init() {
	functions.HTTP("Hello", helloHandler)
}

// Request represents the optional request body.
type Request struct {
	Name string `json:"name"`
}

// Response represents the function response.
type Response struct {
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	var req Request
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	name := req.Name
	if name == "" {
		name = r.URL.Query().Get("name")
	}
	if name == "" {
		name = "World"
	}

	resp := Response{
		Message:   "Hello, " + name + "!",
		Timestamp: time.Now().UTC().Format(RFC3339Millis),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
