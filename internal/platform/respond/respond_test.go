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
	"github.com/fxamacker/cbor/v2"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/platform/portable"
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
				if got := response.Header().Get("Content-Type"); got != "application/cbor" {
					t.Fatalf("unexpected content type %q", got)
				}
				var problem portable.ProblemError
				if err := cbor.Unmarshal(response.Body.Bytes(), &problem); err != nil {
					t.Fatalf("decode CBOR: %v", err)
				}
				if problem.Code != portable.CodeNotFound {
					t.Fatalf("unexpected problem: %#v", problem)
				}
			} else {
				if got := response.Header().Get("Content-Type"); got != "application/problem+json" {
					t.Fatalf("unexpected content type %q", got)
				}
				var problem portable.ProblemError
				if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
					t.Fatalf("decode JSON: %v", err)
				}
				if problem.Status != http.StatusNotFound || problem.Code != portable.CodeNotFound {
					t.Fatalf("unexpected problem: %#v", problem)
				}
			}
			if link := response.Header().Get("Link"); link != "" {
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

func TestRecovererUsesOriginalAcceptAfterSuccessNegotiation(t *testing.T) {
	api := testAPI()
	handler := Recoverer(api)(portable.RequestPolicy("/v1")(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			panic("boom")
		},
	)))
	const accept = "application/json, application/problem+json;q=0.1, application/cbor;q=0.5"
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/hello", nil)
	request.Header.Set("Accept", accept)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != portable.MediaTypeCBOR {
		t.Fatalf("content type=%q want=%q body=%x", got, portable.MediaTypeCBOR, response.Body.Bytes())
	}
	if got := request.Header.Get("Accept"); got != accept {
		t.Fatalf("outer request Accept=%q want=%q", got, accept)
	}
	var problem portable.ProblemError
	if err := cbor.Unmarshal(response.Body.Bytes(), &problem); err != nil ||
		problem.Code != portable.CodeInternalError {
		t.Fatalf("problem=%#v err=%v body=%x", problem, err, response.Body.Bytes())
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
