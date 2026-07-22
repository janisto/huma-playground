package middleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/janisto/huma-observability/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AccessLogger writes structured access logs for non-Huma HTTP handlers.
func AccessLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer logHTTPAccess(r, ww, start)

			next.ServeHTTP(ww, r)
		})
	}
}

func logHTTPAccess(r *http.Request, ww chimiddleware.WrapResponseWriter, start time.Time) {
	if rec := recover(); rec != nil {
		if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
			panic(rec)
		}
		logHTTPRequest(r, ww, start, ww.Status(), "panic")
		panic(rec)
	}

	status := ww.Status()
	if status == 0 {
		status = http.StatusOK
	}
	logHTTPRequest(r, ww, start, status, "")
}

func logHTTPRequest(
	r *http.Request,
	ww chimiddleware.WrapResponseWriter,
	start time.Time,
	status int,
	terminalReason string,
) {
	fields := []zap.Field{
		zap.String("method", r.Method),
		zap.Int("bytes_written", ww.BytesWritten()),
		zap.Float64("duration_ms", float64(time.Since(start))/float64(time.Millisecond)),
	}
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if pathTemplate := routeContext.RoutePattern(); pathTemplate != "" {
			fields = append(fields, zap.String("path_template", pathTemplate))
		}
	}
	if status != 0 {
		fields = append(fields, zap.Int("status", status))
	}
	level := zapcore.InfoLevel
	if terminalReason != "" {
		fields = append(fields, zap.String("terminal_reason", terminalReason))
		level = zapcore.ErrorLevel
	} else if status != 0 {
		level = obs.DefaultStatusLevel(status)
	}

	obs.Logger(r.Context()).Log(level, "http request completed", fields...)
}
