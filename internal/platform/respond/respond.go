package respond

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/janisto/huma-observability/v2"
	"go.uber.org/zap"

	appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
)

type responseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(status int) {
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Recoverer converts panics into the API's configured Problem Details format.
func Recoverer(api huma.API, loggers ...*zap.Logger) func(http.Handler) http.Handler {
	fallback := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		fallback = loggers[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tracked := &responseWriter{ResponseWriter: w}
			defer func(ctx context.Context) {
				recovered := recover()
				if recovered == nil {
					return
				}
				if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(recovered)
				}
				recoveryLogger(ctx, fallback).Error(
					"panic recovered",
					zap.Error(fmt.Errorf("%v", recovered)),
					zap.ByteString("stack", debug.Stack()),
				)
				if tracked.wroteHeader {
					panic(http.ErrAbortHandler)
				}
				if err := writeProblem(
					api,
					tracked,
					r,
					http.StatusInternalServerError,
					"internal server error",
				); err != nil {
					recoveryLogger(ctx, fallback).Error("write panic response", zap.Error(err))
				}
			}(r.Context())
			next.ServeHTTP(tracked, r)
		})
	}
}

func recoveryLogger(ctx context.Context, fallback *zap.Logger) *zap.Logger {
	if obs.RequestID(ctx) != "" {
		return obs.Logger(ctx)
	}
	if fallback == nil {
		return zap.NewNop()
	}
	return fallback
}

func NotFoundHandler(api huma.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := writeProblem(api, w, r, http.StatusNotFound, "resource not found"); err != nil {
			obs.Logger(r.Context()).Error("write not-found response", zap.Error(err))
		}
	}
}

func MethodNotAllowedHandler(api huma.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if allow := allowedMethods(r); len(allow) > 0 {
			w.Header().Set("Allow", strings.Join(allow, ", "))
		}
		if err := writeProblem(
			api,
			w,
			r,
			http.StatusMethodNotAllowed,
			fmt.Sprintf("method %s not allowed", r.Method),
		); err != nil {
			obs.Logger(r.Context()).Error("write method-not-allowed response", zap.Error(err))
		}
	}
}

func writeProblem(api huma.API, w http.ResponseWriter, r *http.Request, status int, detail string) error {
	appmiddleware.AddVary(w.Header(), "Accept", "Origin")
	ctx := humachi.NewContext(&huma.Operation{}, r, w)
	return huma.WriteErr(api, ctx, status, detail)
}

func allowedMethods(r *http.Request) []string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil || rctx.Routes == nil {
		return nil
	}
	routePath := r.URL.EscapedPath()
	if routePath == "" {
		routePath = "/"
	}
	methods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	}
	allowed := make([]string, 0, len(methods))
	for _, method := range methods {
		testContext := chi.NewRouteContext()
		if rctx.Routes.Match(testContext, method, routePath) {
			allowed = append(allowed, method)
		}
	}
	return allowed
}
