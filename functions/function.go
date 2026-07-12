// Package hello provides a small HTTP Cloud Run function example.
package hello

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

const (
	rfc3339Millis = "2006-01-02T15:04:05.000Z"
	maxBodyBytes  = 1 << 20
	maxNameRunes  = 100
)

func init() {
	functions.HTTP("Hello", helloHandler)
}

type helloRequest struct {
	Name string `json:"name"`
}

type helloResponse struct {
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req *helloRequest
	if r.Method == http.MethodPost {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid JSON request body", http.StatusBadRequest)
			return
		}
		if req == nil {
			http.Error(w, "request body must contain one JSON object", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "request body must contain one JSON object", http.StatusBadRequest)
			return
		}
	}

	name := r.URL.Query().Get("name")
	if req != nil && req.Name != "" {
		name = req.Name
	}
	if name == "" {
		name = "World"
	}
	if utf8.RuneCountInString(name) > maxNameRunes {
		http.Error(w, "name must be at most 100 characters", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(helloResponse{
		Message:   "Hello, " + name + "!",
		Timestamp: time.Now().UTC().Format(rfc3339Millis),
	}); err != nil {
		return
	}
}
