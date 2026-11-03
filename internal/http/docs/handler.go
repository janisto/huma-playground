// Package docs serves the application's explicitly named local usage documents.
package docs

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	assets "github.com/janisto/huma-playground"
	"github.com/janisto/huma-playground/internal/platform/portable"
)

const (
	IndexLink   = `</llms.txt>; rel="describedby"; type="text/plain"`
	catalogLink = `</.well-known/api-catalog>; rel="api-catalog"; type="application/linkset+json"`
	catalogType = `application/linkset+json; profile="https://www.rfc-editor.org/info/rfc9727"`
)

var catalog = []byte(`{
  "linkset": [{
    "anchor": "/",
    "service-desc": [{"href": "/openapi.json", "type": "application/json"}],
    "service-doc": [
      {"href": "/llms.txt", "type": "text/plain"},
      {"href": "/llms-full.txt", "type": "text/plain"}
    ],
    "status": [{"href": "/health", "type": "application/json"}]
  }]
}
`)

// Register adds public documentation GET routes and catalog HEAD.
func Register(router chi.Router) {
	catalogHandler := document(catalogType, catalog, catalogLink, IndexLink)
	router.Get("/.well-known/api-catalog", catalogHandler)
	router.Head("/.well-known/api-catalog", catalogHandler)
	router.Get("/llms.txt", document("text/plain; charset=utf-8", assets.UsageIndex))
	router.Get("/llms-full.txt", document("text/plain; charset=utf-8", assets.UsageGuide, IndexLink))
}

func document(contentType string, body []byte, links ...string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		accept := portable.AcceptHeader(request.Header)
		if len(request.Header.Values("Accept")) > 0 && accept == "" || !portable.AcceptsDocument(accept, contentType) {
			portable.WriteProblem(writer, request, portable.CodeNotAcceptable)
			return
		}
		for _, link := range links {
			writer.Header().Add("Link", link)
		}
		writer.Header().Set("Content-Type", contentType)
		writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
		writer.WriteHeader(http.StatusOK)
		if request.Method != http.MethodHead {
			if _, err := writer.Write(body); err != nil {
				panic(err)
			}
		}
	}
}
