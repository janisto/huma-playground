package respond

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	apiinternal "github.com/janisto/huma-playground/internal/api"
	appmiddleware "github.com/janisto/huma-playground/internal/middleware"
)

const (
	codeOK                = "OK"
	codeRedirect          = "REDIRECT"
	codeNotFound          = "NOT_FOUND"
	msgNotFound           = "resource not found"
	codeMethodNotAllowed  = "METHOD_NOT_ALLOWED"
	msgMethodNotAllowed   = "method not allowed"
	codeInternalServerErr = "INTERNAL_SERVER_ERROR"
	msgInternalServerErr  = "internal server error"
)

var installOnce sync.Once

// Install ensures Huma uses the shared envelope + logging for all error responses.
func Install() {
	installOnce.Do(func() {
		huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
			return statusError(context.Background(), status, statusCodeName(status), messageOrDefault(status, msg), issuesFromErrors(errs))
		}

		huma.NewErrorWithContext = func(hctx huma.Context, status int, msg string, errs ...error) huma.StatusError {
			code := statusCodeName(status)
			message := messageOrDefault(status, msg)
			issues := issuesFromErrors(errs)
			goCtx := context.Background()
			if hctx != nil {
				goCtx = hctx.Context()
			}
			return statusError(goCtx, status, code, message, issues, errs...)
		}
	})
}

// Body is a helper for handlers returning structured envelopes.
type Body[T any] struct {
	Body apiinternal.Envelope[T] `json:"body"`
}

// Success wraps the provided payload using the shared envelope helpers.
func Success[T any](ctx context.Context, data T) Body[T] {
	env := apiinternal.NewSuccessEnvelope[T](appmiddleware.TraceIDFromContext(ctx), data)
	return Body[T]{Body: env}
}

// Error returns a status error with the shared envelope + logging semantics.
func Error(ctx context.Context, status int, code, msg string, issues []apiinternal.FieldIssue, errs ...error) huma.StatusError {
	if code == "" {
		code = statusCodeName(status)
	}
	if msg == "" {
		msg = messageOrDefault(status, msg)
	}
	return statusError(ctx, status, code, msg, issues, errs...)
}

// Write serializes an envelope directly to the ResponseWriter.
func Write[T any](w http.ResponseWriter, status int, env apiinternal.Envelope[T]) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(env)
}

// WriteSuccess renders a success envelope with the given status code.
func WriteSuccess[T any](w http.ResponseWriter, ctx context.Context, status int, data T) error {
	return Write(w, status, Success(ctx, data).Body)
}

// WriteError renders an error envelope with optional issues, logging as needed.
func WriteError(w http.ResponseWriter, ctx context.Context, status int, code, msg string, issues []apiinternal.FieldIssue, errs ...error) error {
	se := Error(ctx, status, code, msg, issues, errs...)
	env, ok := se.(*statusEnvelopeError)
	if !ok {
		return se
	}
	return Write(w, se.GetStatus(), env.Envelope)
}

// WriteRedirect writes a redirect response including the Location header.
func WriteRedirect(w http.ResponseWriter, ctx context.Context, status int, location, message string) error {
	if message == "" {
		message = http.StatusText(status)
	}
	fields := []apiinternal.FieldIssue{{Field: "Location", Issue: location}}
	se := Error(ctx, status, codeRedirect, message, fields)
	env, ok := se.(*statusEnvelopeError)
	if !ok {
		return se
	}
	w.Header().Set("Location", location)
	return Write(w, se.GetStatus(), env.Envelope)
}

// NotFoundHandler emits a shared-envelope 404 response.
func NotFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := WriteError(w, r.Context(), http.StatusNotFound, codeNotFound, msgNotFound, nil); err != nil {
			appmiddleware.LogError(r.Context(), "failed to render not found", err)
		}
	}
}

// MethodNotAllowedHandler emits a shared-envelope 405 response.
func MethodNotAllowedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if allow := allowedMethods(r); len(allow) > 0 {
			w.Header().Set("Allow", strings.Join(allow, ", "))
		}
		if err := WriteError(w, r.Context(), http.StatusMethodNotAllowed, codeMethodNotAllowed, msgMethodNotAllowed, nil); err != nil {
			appmiddleware.LogError(r.Context(), "failed to render method not allowed", err)
		}
	}
}

// Recoverer converts panics into structured 500 responses using the shared envelope.
func Recoverer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					var err error
					switch v := rec.(type) {
					case error:
						err = v
					default:
						err = fmt.Errorf("%v", v)
					}
					err = fmt.Errorf("%w\n%s", err, debug.Stack())
					if writeErr := WriteError(w, r.Context(), http.StatusInternalServerError, codeInternalServerErr, msgInternalServerErr, nil, err); writeErr != nil {
						appmiddleware.LogError(r.Context(), "failed to render internal error", writeErr)
					}
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Status304NotModified returns a shared-envelope 304 response.
func Status304NotModified() huma.StatusError {
	return Error(context.Background(), http.StatusNotModified, statusCodeName(http.StatusNotModified), http.StatusText(http.StatusNotModified), nil)
}

// allowedMethods inspects chi's routing context to discover allowed methods.
func allowedMethods(r *http.Request) []string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil || rctx.Routes == nil {
		return nil
	}

	routePath := rctx.RoutePath
	if routePath == "" {
		if r.URL.RawPath != "" {
			routePath = r.URL.RawPath
		} else {
			routePath = r.URL.Path
		}
		if routePath == "" {
			routePath = "/"
		}
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
	seen := make(map[string]struct{})
	for _, method := range methods {
		tctx := chi.NewRouteContext()
		if rctx.Routes.Match(tctx, method, routePath) {
			if _, ok := seen[method]; !ok {
				allowed = append(allowed, method)
				seen[method] = struct{}{}
			}
		}
	}
	return allowed
}

type statusEnvelopeError struct {
	apiinternal.Envelope[struct{}]
	status int
}

func (e *statusEnvelopeError) Error() string {
	if e.Envelope.Error != nil && e.Envelope.Error.Message != "" {
		return e.Envelope.Error.Message
	}
	return http.StatusText(e.status)
}

func (e *statusEnvelopeError) GetStatus() int {
	return e.status
}

func statusError(ctx context.Context, status int, code, msg string, issues []apiinternal.FieldIssue, errs ...error) huma.StatusError {
	fields := []zap.Field{
		zap.Int("status", status),
		zap.String("code", code),
		zap.String("message", msg),
	}
	if len(issues) > 0 {
		fields = append(fields, zap.Any("details", issues))
	}
	logWithStatus(ctx, status, msg, joinErrors(errs), fields...)
	env := apiinternal.NewErrorEnvelope[struct{}](appmiddleware.TraceIDFromContext(ctx), code, msg, issues)
	return &statusEnvelopeError{Envelope: env, status: status}
}

func issuesFromErrors(errs []error) []apiinternal.FieldIssue {
	if len(errs) == 0 {
		return nil
	}
	issues := make([]apiinternal.FieldIssue, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		issue := apiinternal.FieldIssue{Issue: err.Error()}
		if detailer, ok := err.(huma.ErrorDetailer); ok {
			if detail := detailer.ErrorDetail(); detail != nil {
				issue.Issue = detail.Message
				if detail.Location != "" {
					issue.Field = detail.Location
				}
			}
		}
		issues = append(issues, issue)
	}
	if len(issues) == 0 {
		return nil
	}
	return issues
}

func joinErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return errors.Join(errs...)
	}
}

func statusCodeName(status int) string {
	name := strings.ToUpper(strings.ReplaceAll(http.StatusText(status), " ", "_"))
	name = strings.ReplaceAll(name, "-", "_")
	if strings.TrimSpace(name) == "" {
		return fmt.Sprintf("HTTP_%d", status)
	}
	return name
}

func messageOrDefault(status int, msg string) string {
	if strings.TrimSpace(msg) != "" {
		return msg
	}
	if text := http.StatusText(status); strings.TrimSpace(text) != "" {
		return text
	}
	return fmt.Sprintf("HTTP %d", status)
}

func logWithStatus(ctx context.Context, status int, msg string, err error, fields ...zap.Field) {
	if ctx == nil {
		ctx = context.Background()
	}
	if msg == "" {
		msg = "request failed"
	}
	switch {
	case status >= 500:
		appmiddleware.LogError(ctx, msg, err, fields...)
	case status >= 400:
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		appmiddleware.LogWarn(ctx, msg, fields...)
	default:
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		appmiddleware.LogInfo(ctx, msg, fields...)
	}
}
