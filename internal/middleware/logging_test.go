package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestTraceFields(t *testing.T) {
	header := "00-3d23d071b5bfd6579171efce907685cb-08f067aa0ba902b7-01"
	projectID := "test-project"

	fields := traceFields(header, projectID)
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}

	wantTrace := fmt.Sprintf("projects/%s/traces/%s", projectID, "3d23d071b5bfd6579171efce907685cb")
	if fields[0].Key != "logging.googleapis.com/trace" || fields[0].String != wantTrace {
		t.Fatalf("unexpected trace field: %+v", fields[0])
	}
	if fields[1].Key != "logging.googleapis.com/spanId" || fields[1].String != "08f067aa0ba902b7" {
		t.Fatalf("unexpected span field: %+v", fields[1])
	}
	if fields[2].Key != "logging.googleapis.com/trace_sampled" || fields[2].Type != zapcore.BoolType ||
		fields[2].Integer != 1 {
		t.Fatalf("unexpected sampled field: %+v", fields[2])
	}
}

func TestTraceFieldsNotSampled(t *testing.T) {
	header := "00-3d23d071b5bfd6579171efce907685cb-08f067aa0ba902b7-00"
	projectID := "test-project"

	fields := traceFields(header, projectID)
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}

	wantTrace := fmt.Sprintf("projects/%s/traces/%s", projectID, "3d23d071b5bfd6579171efce907685cb")
	if fields[0].Key != "logging.googleapis.com/trace" || fields[0].String != wantTrace {
		t.Fatalf("unexpected trace field: %+v", fields[0])
	}
	if fields[1].Key != "logging.googleapis.com/spanId" || fields[1].String != "08f067aa0ba902b7" {
		t.Fatalf("unexpected span field: %+v", fields[1])
	}
	if fields[2].Key != "logging.googleapis.com/trace_sampled" || fields[2].Type != zapcore.BoolType ||
		fields[2].Integer != 0 {
		t.Fatalf("expected unsampled trace field, got %+v", fields[2])
	}
}

func TestTraceFieldsInvalid(t *testing.T) {
	if fields := traceFields("invalid", "test-project"); fields != nil {
		t.Fatalf("expected nil fields for invalid header, got %v", fields)
	}

	if fields := traceFields("", "test-project"); fields != nil {
		t.Fatalf("expected nil fields for empty header, got %v", fields)
	}

	if fields := traceFields("trace/span;o=1", ""); fields != nil {
		t.Fatalf("expected nil fields when projectID missing, got %v", fields)
	}
}

func TestLoggerWithTraceAddsRequestID(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	base := zap.New(core)

	logger := loggerWithTrace(base, "", "test-project", "req-123")
	logger.Info("hello")

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	found := false
	for _, f := range entries[0].Context {
		if f.Key == "requestId" && f.String == "req-123" {
			found = true
		}
	}
	if !found {
		t.Fatalf("requestId field not found in log context: %+v", entries[0].Context)
	}
}

func TestTraceIDFromContext(t *testing.T) {
	if got := TraceIDFromContext(context.Background()); got != nil {
		t.Fatalf("expected nil trace ID, got %v", got)
	}
	ctx := contextWithTraceID(context.Background(), "trace-abc")
	got := TraceIDFromContext(ctx)
	if got == nil || *got != "trace-abc" {
		t.Fatalf("expected trace-abc, got %v", got)
	}
}

func TestLoggerWithTraceAddsCloudFields(t *testing.T) {
	header := "00-3d23d071b5bfd6579171efce907685cb-08f067aa0ba902b7-01"
	core, recorded := observer.New(zapcore.InfoLevel)
	base := zap.New(core)

	logger := loggerWithTrace(base, header, "test-project", "req-123")
	logger.Info("hello")

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	wantTrace := fmt.Sprintf("projects/%s/traces/%s", "test-project", "3d23d071b5bfd6579171efce907685cb")
	ctxFields := map[string]zap.Field{}
	for _, f := range entries[0].Context {
		ctxFields[f.Key] = f
	}

	if f, ok := ctxFields["logging.googleapis.com/trace"]; !ok || f.String != wantTrace {
		t.Fatalf("trace field mismatch: %+v", ctxFields)
	}
	if f, ok := ctxFields["logging.googleapis.com/spanId"]; !ok || f.String != "08f067aa0ba902b7" {
		t.Fatalf("span field mismatch: %+v", ctxFields)
	}
	if f, ok := ctxFields["logging.googleapis.com/trace_sampled"]; !ok || f.Type != zapcore.BoolType || f.Integer != 1 {
		t.Fatalf("trace_sampled field mismatch: %+v", ctxFields)
	}
	if f, ok := ctxFields["requestId"]; !ok || f.String != "req-123" {
		t.Fatalf("requestId field mismatch: %+v", ctxFields)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "value", "other"); got != "value" {
		t.Fatalf("expected 'value', got %q", got)
	}
	if got := firstNonEmpty(); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

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

func TestLogErrorAppendsErrorField(t *testing.T) {
	core, recorded := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	ctx := contextWithLogger(context.Background(), logger)

	err := errors.New("boom")
	LogError(ctx, "failed", err, zap.String("foo", "bar"))

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Message != "failed" {
		t.Fatalf("unexpected log message: %s", entry.Message)
	}
	if entry.Level != zapcore.ErrorLevel {
		t.Fatalf("unexpected log level: %s", entry.Level)
	}

	fields := map[string]zap.Field{}
	for _, f := range entry.Context {
		fields[f.Key] = f
	}

	if f, ok := fields["foo"]; !ok || f.String != "bar" {
		t.Fatalf("expected foo field, got %+v", fields)
	}
	if f, ok := fields["error"]; !ok || f.Type != zapcore.ErrorType {
		t.Fatalf("expected error field, got %+v", fields)
	}
}

func TestLogInfoWritesEntry(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	ctx := contextWithLogger(context.Background(), logger)

	LogInfo(ctx, "info message", zap.String("foo", "bar"))

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Message != "info message" {
		t.Fatalf("unexpected log message: %s", entry.Message)
	}
	if entry.Level != zapcore.InfoLevel {
		t.Fatalf("unexpected log level: %s", entry.Level)
	}
	if len(entry.Context) != 1 || entry.Context[0].Key != "foo" || entry.Context[0].String != "bar" {
		t.Fatalf("unexpected context fields: %+v", entry.Context)
	}
}

func TestLogWarnWritesEntry(t *testing.T) {
	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	ctx := contextWithLogger(context.Background(), logger)

	LogWarn(ctx, "warn message", zap.String("foo", "bar"))

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Message != "warn message" {
		t.Fatalf("unexpected log message: %s", entry.Message)
	}
	if entry.Level != zapcore.WarnLevel {
		t.Fatalf("unexpected log level: %s", entry.Level)
	}
	if len(entry.Context) != 1 || entry.Context[0].Key != "foo" || entry.Context[0].String != "bar" {
		t.Fatalf("unexpected context fields: %+v", entry.Context)
	}
}

func TestLogFatalAppendsErrorField(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core, zap.WithFatalHook(zapcore.WriteThenPanic))
	ctx := contextWithLogger(context.Background(), logger)

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic triggered by fatal hook")
		}

		entries := recorded.All()
		if len(entries) != 1 {
			t.Fatalf("expected 1 log entry, got %d", len(entries))
		}
		entry := entries[0]
		if entry.Message != "fatal failure" {
			t.Fatalf("unexpected log message: %s", entry.Message)
		}
		if entry.Level != zapcore.FatalLevel {
			t.Fatalf("unexpected log level: %s", entry.Level)
		}

		fields := map[string]zap.Field{}
		for _, f := range entry.Context {
			fields[f.Key] = f
		}

		if f, ok := fields["foo"]; !ok || f.String != "bar" {
			t.Fatalf("expected foo field, got %+v", fields)
		}
		if f, ok := fields["error"]; !ok || f.Type != zapcore.ErrorType {
			t.Fatalf("expected error field, got %+v", fields)
		}
	}()

	LogFatal(ctx, "fatal failure", errors.New("boom"), zap.String("foo", "bar"))
}

func TestSugarFromContext(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	ctx := contextWithLogger(context.Background(), logger)

	sugar := SugarFromContext(ctx)
	sugar.Infow("test message", "key", "value")

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Message != "test message" {
		t.Fatalf("unexpected message: %s", entries[0].Message)
	}
}

func TestContextWithTraceIDEmpty(t *testing.T) {
	original := context.Background()
	ctx := contextWithTraceID(original, "")
	if ctx != original {
		t.Fatal("expected same context for empty trace ID")
	}
}

func TestLoggerWithTraceNilBase(t *testing.T) {
	logger := loggerWithTrace(nil, "", "test-project", "req-123")
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerWithTraceNoFields(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	base := zap.New(core)

	logger := loggerWithTrace(base, "", "", "")
	logger.Info("test")

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if len(entries[0].Context) != 0 {
		t.Fatalf("expected no context fields, got %d", len(entries[0].Context))
	}
}

func TestTraceResource(t *testing.T) {
	header := "00-3d23d071b5bfd6579171efce907685cb-08f067aa0ba902b7-01"
	projectID := "test-project"

	resource := traceResource(header, projectID)
	want := "projects/test-project/traces/3d23d071b5bfd6579171efce907685cb"
	if resource != want {
		t.Fatalf("expected %s, got %s", want, resource)
	}
}

func TestTraceResourceEmptyProjectID(t *testing.T) {
	resource := traceResource("00-ab42124a3c573678d4d8b21ba52df3bf-d21f7bc17caa5aba-01", "")
	if resource != "" {
		t.Fatalf("expected empty string for empty project ID, got %s", resource)
	}
}

func TestTraceResourceInvalidHeader(t *testing.T) {
	resource := traceResource("invalid", "test-project")
	if resource != "" {
		t.Fatalf("expected empty string for invalid header, got %s", resource)
	}
}

func TestLogErrorNilError(t *testing.T) {
	core, recorded := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	ctx := contextWithLogger(context.Background(), logger)

	LogError(ctx, "no error", nil, zap.String("key", "value"))

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	for _, f := range entries[0].Context {
		if f.Key == "error" {
			t.Fatal("did not expect error field when err is nil")
		}
	}
}

func TestLogFatalNilError(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core, zap.WithFatalHook(zapcore.WriteThenPanic))
	ctx := contextWithLogger(context.Background(), logger)

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic triggered by fatal hook")
		}

		entries := recorded.All()
		if len(entries) != 1 {
			t.Fatalf("expected 1 log entry, got %d", len(entries))
		}

		for _, f := range entries[0].Context {
			if f.Key == "error" {
				t.Fatal("did not expect error field when err is nil")
			}
		}
	}()

	LogFatal(ctx, "fatal without error", nil)
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

func TestResolveProjectID(t *testing.T) {
	result := resolveProjectID()
	if result != cachedProjectID {
		t.Fatalf("expected cached value %s, got %s", cachedProjectID, result)
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

	handler := RequestID()(RequestLogger()(inner))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-Id", "test-request-id")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestLoggerFromContextNilContext(t *testing.T) {
	logger := LoggerFromContext(nil) //nolint:staticcheck // testing nil context handling
	if logger == nil {
		t.Fatal("expected non-nil logger for nil context")
	}
}

func TestLoggerFromContextNilLoggerInContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxLoggerKey{}, (*zap.Logger)(nil))
	logger := LoggerFromContext(ctx)
	if logger == nil {
		t.Fatal("expected non-nil logger when context has nil logger")
	}
}

func TestTraceIDFromContextNilContext(t *testing.T) {
	traceID := TraceIDFromContext(nil) //nolint:staticcheck // testing nil context handling
	if traceID != nil {
		t.Fatalf("expected nil trace ID for nil context, got %v", traceID)
	}
}

func TestContextWithLoggerNilContext(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	ctx := contextWithLogger(nil, logger) //nolint:staticcheck // testing nil context handling
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	LoggerFromContext(ctx).Info("test")
	if len(recorded.All()) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(recorded.All()))
	}
}

func TestContextWithTraceIDNilContext(t *testing.T) {
	ctx := contextWithTraceID(nil, "trace-123") //nolint:staticcheck // testing nil context handling
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	traceID := TraceIDFromContext(ctx)
	if traceID == nil || *traceID != "trace-123" {
		t.Fatalf("expected trace-123, got %v", traceID)
	}
}
