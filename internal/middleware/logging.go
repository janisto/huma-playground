package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/common"
)

const cloudTraceHeader = "X-Cloud-Trace-Context"

var traceHeaderRe = regexp.MustCompile(`^([0-9a-fA-F]+)/([0-9a-fA-F]+)(?:;o=(\d))?$`)

var (
	projectIDOnce   sync.Once
	cachedProjectID string
)

// ctxLoggerKey is used to store the request-specific logger in context.
type (
	ctxLoggerKey  struct{}
	ctxTraceIDKey struct{}
)

// RequestLogger enriches the request context with a zap logger that embeds Cloud Trace metadata.
func RequestLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get(cloudTraceHeader)
			projectID := resolveProjectID()
			reqID := chimiddleware.GetReqID(r.Context())

			traceID := traceResource(header, projectID)
			if traceID == "" && reqID != "" {
				traceID = reqID
			}
			logger := loggerWithTrace(common.Logger(), header, projectID, reqID)
			ctx := r.Context()
			ctx = contextWithTraceID(ctx, traceID)
			ctx = contextWithLogger(ctx, logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AccessLogger writes structured request summaries using the request-scoped logger.
func AccessLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			logger := LoggerFromContext(r.Context())
			logger.Info(
				"request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Int("bytes", ww.BytesWritten()),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}

// LoggerFromContext returns the request-scoped logger if present, otherwise falls back to the global logger.
func LoggerFromContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return common.Logger()
	}
	if l, ok := ctx.Value(ctxLoggerKey{}).(*zap.Logger); ok && l != nil {
		return l
	}
	return common.Logger()
}

// SugarFromContext returns a sugared logger derived from the request context.
func SugarFromContext(ctx context.Context) *zap.SugaredLogger {
	return LoggerFromContext(ctx).Sugar()
}

// TraceIDFromContext returns the correlation identifier (trace or request ID) if present.
func TraceIDFromContext(ctx context.Context) *string {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(ctxTraceIDKey{}).(*string); ok && v != nil && *v != "" {
		return v
	}
	return nil
}

// LogInfo writes an informational message using the request-aware logger.
func LogInfo(ctx context.Context, msg string, fields ...zap.Field) {
	LoggerFromContext(ctx).Info(msg, fields...)
}

// LogWarn writes a warning message using the request-aware logger.
func LogWarn(ctx context.Context, msg string, fields ...zap.Field) {
	LoggerFromContext(ctx).Warn(msg, fields...)
}

// LogError writes an error message using the request-aware logger and appends the error field when provided.
func LogError(ctx context.Context, msg string, err error, fields ...zap.Field) {
	logger := LoggerFromContext(ctx)
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	logger.Error(msg, fields...)
}

// LogFatal logs with fatal severity and terminates the process. It attaches the error field when provided.
func LogFatal(ctx context.Context, msg string, err error, fields ...zap.Field) {
	logger := LoggerFromContext(ctx)
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	logger.Fatal(msg, fields...)
}

func contextWithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxLoggerKey{}, logger)
}

func contextWithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	traceCopy := traceID
	return context.WithValue(ctx, ctxTraceIDKey{}, &traceCopy)
}

func loggerWithTrace(base *zap.Logger, header, projectID, requestID string) *zap.Logger {
	if base == nil {
		base = zap.NewNop()
	}
	fields := traceFields(header, projectID)
	if requestID != "" {
		fields = append(fields, zap.String("requestId", requestID))
	}
	if len(fields) == 0 {
		return base
	}
	return base.With(fields...)
}

func traceFields(header, projectID string) []zap.Field {
	if projectID == "" {
		return nil
	}
	matches := traceHeaderRe.FindStringSubmatch(header)
	if len(matches) != 4 {
		return nil
	}
	traceID := matches[1]
	spanID := matches[2]
	sampled := matches[3] == "1"
	resource := fmt.Sprintf("projects/%s/traces/%s", projectID, traceID)

	return []zap.Field{
		zap.String("logging.googleapis.com/trace", resource),
		zap.String("logging.googleapis.com/spanId", spanID),
		zap.Bool("logging.googleapis.com/trace_sampled", sampled),
	}
}

func traceResource(header, projectID string) string {
	if projectID == "" {
		return ""
	}
	matches := traceHeaderRe.FindStringSubmatch(header)
	if len(matches) != 4 {
		return ""
	}
	return fmt.Sprintf("projects/%s/traces/%s", projectID, matches[1])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolveProjectID() string {
	projectIDOnce.Do(func() {
		cachedProjectID = firstNonEmpty(
			os.Getenv("GOOGLE_CLOUD_PROJECT"),
			os.Getenv("GCP_PROJECT"),
			os.Getenv("GCLOUD_PROJECT"),
			os.Getenv("PROJECT_ID"),
		)
	})
	return cachedProjectID
}
