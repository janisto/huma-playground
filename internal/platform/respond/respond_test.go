package respond

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func testAPI() huma.API {
	router := chi.NewRouter()
	config := huma.DefaultConfig("Respond Test", "test")
	config.DocsPath = ""
	config.OpenAPIPath = ""
	config.Servers = []*huma.Server{{URL: "/v1"}}
	api := humachi.New(router, config)
	huma.Get(api, "/probe", func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return nil, huma.Error500InternalServerError("probe")
	})
	return api
}

func TestNotFoundUsesHumaProblemDetails(t *testing.T) {
	api := testAPI()
	for _, accept := range []string{"application/json", "application/cbor"} {
		t.Run(accept, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/missing", nil)
			request.Header.Set("Accept", accept)
			response := httptest.NewRecorder()
			NotFoundHandler(api).ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d", response.Code)
			}
			if accept == "application/cbor" {
				if got := response.Header().Get("Content-Type"); got != "application/problem+cbor" {
					t.Fatalf("unexpected content type %q", got)
				}
				var problem huma.ErrorModel
				if err := cbor.Unmarshal(response.Body.Bytes(), &problem); err != nil {
					t.Fatalf("decode CBOR: %v", err)
				}
			} else {
				if got := response.Header().Get("Content-Type"); got != "application/problem+json" {
					t.Fatalf("unexpected content type %q", got)
				}
				var problem huma.ErrorModel
				if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
					t.Fatalf("decode JSON: %v", err)
				}
				if problem.Status != http.StatusNotFound {
					t.Fatalf("unexpected problem: %#v", problem)
				}
			}
			if link := response.Header().Get("Link"); link != "</v1/schemas/ErrorModel.json>; rel=\"describedBy\"" {
				t.Fatalf("unexpected schema link %q", link)
			}
		})
	}
}

func TestMethodNotAllowedIncludesAllow(t *testing.T) {
	api := testAPI()
	router := chi.NewRouter()
	router.MethodNotAllowed(MethodNotAllowedHandler(api))
	router.Get("/resource", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/resource", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
	if allow := response.Header().Get("Allow"); allow != "GET" {
		t.Fatalf("unexpected Allow header %q", allow)
	}
}

func TestRecoverer(t *testing.T) {
	api := testAPI()
	handler := Recoverer(api, zap.NewNop())(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", response.Code)
	}
}

func TestRecovererPreservesAbortHandler(t *testing.T) {
	api := testAPI()
	handler := Recoverer(api)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		recovered := recover()
		recoveredErr, ok := recovered.(error)
		if !ok || !errors.Is(recoveredErr, http.ErrAbortHandler) {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
}

func TestRecovererAbortsPartiallyWrittenResponse(t *testing.T) {
	api := testAPI()
	response := httptest.NewRecorder()
	handler := Recoverer(api)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		panic("boom")
	}))
	defer func() {
		recovered := recover()
		recoveredErr, ok := recovered.(error)
		if !ok || !errors.Is(recoveredErr, http.ErrAbortHandler) {
			t.Fatalf("expected http.ErrAbortHandler, got %v", recovered)
		}
		if response.Code != http.StatusOK || response.Body.String() != "partial" {
			t.Fatalf("unexpected partial response: %d %q", response.Code, response.Body.String())
		}
	}()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
}
