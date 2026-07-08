package middleware

import (
	"errors"
	"net"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/janisto/huma-observability"
	"go.uber.org/zap"
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
		logHTTPRequest(r, ww, start, http.StatusInternalServerError)
		panic(rec)
	}

	status := ww.Status()
	if status == 0 {
		status = http.StatusOK
	}
	logHTTPRequest(r, ww, start, status)
}

func logHTTPRequest(r *http.Request, ww chimiddleware.WrapResponseWriter, start time.Time, status int) {
	fields := []zap.Field{
		zap.String("method", r.Method),
		zap.String("path", r.URL.EscapedPath()),
		zap.Int("status", status),
		zap.Int("bytes_written", ww.BytesWritten()),
		zap.Float64("duration_ms", float64(time.Since(start))/float64(time.Millisecond)),
	}
	if remoteIP := requestRemoteIP(r.RemoteAddr); remoteIP != "" {
		fields = append(fields, zap.String("remote_ip", remoteIP))
	}
	if userAgent := r.UserAgent(); userAgent != "" {
		fields = append(fields, zap.String("user_agent", userAgent))
	}

	obs.Logger(r.Context()).Info("http request completed", fields...)
}

func requestRemoteIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
