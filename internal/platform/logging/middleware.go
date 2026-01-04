package logging

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// RequestLogger enriches the request context with a zap logger that embeds Cloud Trace metadata.
func RequestLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get(traceparentHeader)
			projectID := resolveProjectID()
			reqID := chimiddleware.GetReqID(r.Context())

			traceID := traceResource(header, projectID)
			if traceID == "" && reqID != "" {
				traceID = reqID
			}
			logger := loggerWithTrace(Logger(), header, projectID, reqID)
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
