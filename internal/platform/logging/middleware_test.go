package logging

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestAccessLoggerUsesRequestLogger(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	access := AccessLogger()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/tea", nil)
	req = req.WithContext(contextWithLogger(req.Context(), logger))
	resp := httptest.NewRecorder()

	access.ServeHTTP(resp, req)

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Message != "request completed" {
		t.Fatalf("unexpected log message: %s", entry.Message)
	}

	fields := map[string]zap.Field{}
	for _, f := range entry.Context {
		fields[f.Key] = f
	}

	if f, ok := fields["status"]; !ok || f.Integer != http.StatusTeapot {
		t.Fatalf("expected status 418, got %+v", f)
	}
	if f, ok := fields["path"]; !ok || f.String != "/tea" {
		t.Fatalf("expected path '/tea', got %+v", f)
	}
	if _, ok := fields["duration"]; !ok {
		t.Fatalf("expected duration field, got %+v", fields)
	}
}

func TestRequestLoggerMiddleware(t *testing.T) {
	handler := RequestLogger()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := LoggerFromContext(r.Context())
		if logger == nil {
			t.Fatal("expected non-nil logger in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestRequestLoggerWithTraceHeader(t *testing.T) {
	origProjectID := cachedProjectID
	cachedProjectID = "test-project"
	projectIDOnce = sync.Once{}
	projectIDOnce.Do(func() {})
	defer func() {
		cachedProjectID = origProjectID
	}()

	handler := RequestLogger()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := TraceIDFromContext(r.Context())
		if traceID == nil {
			t.Fatal("expected trace ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("traceparent", "00-3d23d071b5bfd6579171efce907685cb-08f067aa0ba902b7-01")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestRequestLoggerFallsBackToRequestID(t *testing.T) {
	origProjectID := cachedProjectID
	cachedProjectID = ""
	projectIDOnce = sync.Once{}
	projectIDOnce.Do(func() {})
	defer func() {
		cachedProjectID = origProjectID
	}()

	// Simulate RequestID middleware by setting the chi request ID using chi's key
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := TraceIDFromContext(r.Context())
		if traceID == nil {
			t.Fatal("expected trace ID in context")
		}
		if *traceID != "test-request-id" {
			t.Fatalf("expected trace ID to be request ID 'test-request-id', got %s", *traceID)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Use chi's RequestIDKey to match what chimiddleware.GetReqID() expects
	handler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), chimiddleware.RequestIDKey, "test-request-id")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}(RequestLogger()(inner))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}
