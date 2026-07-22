package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/janisto/huma-observability/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestAccessLoggerLogsHTTPRequest(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	router := chi.NewRouter()
	router.Use(AccessLogger())
	router.Get("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("tea"))
	})
	handler := obs.HTTPRequestContext(obs.HTTPRequestContextConfig{Logger: logger})(
		router,
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ready", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "access-req")
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "203.0.113.10:12345"
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", resp.Code)
	}
	entries := recorded.FilterMessage("http request completed").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 access log, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	assertLogField(t, fields, "request_id", "access-req")
	assertLogField(t, fields, "method", http.MethodGet)
	assertLogField(t, fields, "path_template", "/ready")
	assertLogField(t, fields, "status", int64(http.StatusTeapot))
	assertLogField(t, fields, "bytes_written", int64(3))
	assertNoLogFields(t, fields, "path", "peer_ip", "remote_ip", "user_agent")
	if _, ok := fields["duration_ms"]; !ok {
		t.Fatalf("expected duration_ms field, got %#v", fields)
	}
	if entries[0].Level != zapcore.WarnLevel {
		t.Fatalf("expected warning access log, got %s", entries[0].Level)
	}
}

func TestAccessLoggerLogsPanicTerminalReasonAndRepanics(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	handler := obs.HTTPRequestContext(obs.HTTPRequestContextConfig{Logger: logger})(
		AccessLogger()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom")
		})),
	)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "panic-req")
	resp := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != "boom" {
			t.Fatalf("panic = %#v, want %q", rec, "boom")
		}
		entries := recorded.FilterMessage("http request completed").All()
		if len(entries) != 1 {
			t.Fatalf("expected 1 access log, got %d", len(entries))
		}
		fields := entries[0].ContextMap()
		assertLogField(t, fields, "request_id", "panic-req")
		assertLogField(t, fields, "terminal_reason", "panic")
		assertNoLogFields(t, fields, "status")
		if entries[0].Level != zapcore.ErrorLevel {
			t.Fatalf("expected error access log, got %s", entries[0].Level)
		}
	}()

	handler.ServeHTTP(resp, req)
}

func TestAccessLoggerPanicRetainsCommittedStatus(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	handler := obs.HTTPRequestContext(obs.HTTPRequestContextConfig{Logger: logger})(
		AccessLogger()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			panic("boom")
		})),
	)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "partial-panic-req")
	resp := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != "boom" {
			t.Fatalf("panic = %#v, want %q", rec, "boom")
		}
		entries := recorded.FilterMessage("http request completed").All()
		if len(entries) != 1 {
			t.Fatalf("expected 1 access log, got %d", len(entries))
		}
		fields := entries[0].ContextMap()
		assertLogField(t, fields, "status", int64(http.StatusAccepted))
		assertLogField(t, fields, "terminal_reason", "panic")
		if entries[0].Level != zapcore.ErrorLevel {
			t.Fatalf("expected error access log, got %s", entries[0].Level)
		}
	}()

	handler.ServeHTTP(resp, req)
}

func TestAccessLoggerLogsImplicitOK(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	handler := obs.HTTPRequestContext(obs.HTTPRequestContextConfig{Logger: logger})(
		AccessLogger()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})),
	)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ok", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "ok-req")
	req.RemoteAddr = ""
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	entries := recorded.FilterMessage("http request completed").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 access log, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	assertLogField(t, fields, "request_id", "ok-req")
	assertLogField(t, fields, "status", int64(http.StatusOK))
	assertLogField(t, fields, "bytes_written", int64(0))
	assertNoLogFields(t, fields, "path", "peer_ip", "remote_ip", "user_agent", "terminal_reason")
	if entries[0].Level != zapcore.InfoLevel {
		t.Fatalf("expected info access log, got %s", entries[0].Level)
	}
}

func TestAccessLoggerRepanicsAbortHandlerWithoutLogging(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	handler := obs.HTTPRequestContext(obs.HTTPRequestContextConfig{Logger: logger})(
		AccessLogger()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic(http.ErrAbortHandler)
		})),
	)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/abort", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "abort-req")
	resp := httptest.NewRecorder()

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected panic")
		}
		err, ok := rec.(error)
		if !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("expected http.ErrAbortHandler panic, got %#v", rec)
		}
		entries := recorded.FilterMessage("http request completed").All()
		if len(entries) != 0 {
			t.Fatalf("expected no access logs, got %d", len(entries))
		}
	}()

	handler.ServeHTTP(resp, req)
}

func assertLogField(t *testing.T, fields map[string]any, key string, want any) {
	t.Helper()
	if got := fields[key]; got != want {
		t.Fatalf("expected log field %s=%v, got %v in %#v", key, want, got, fields)
	}
}

func assertNoLogFields(t *testing.T, fields map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, ok := fields[key]; ok {
			t.Fatalf("did not expect log field %q in %#v", key, fields)
		}
	}
}
