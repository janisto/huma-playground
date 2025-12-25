package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestTraceFields(t *testing.T) {
	header := "3d23d071b5bfd6579171efce907685cb/643745351650131537;o=1"
	projectID := "test-project"

	fields := traceFields(header, projectID)
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}

	wantTrace := fmt.Sprintf("projects/%s/traces/%s", projectID, "3d23d071b5bfd6579171efce907685cb")
	if fields[0].Key != "logging.googleapis.com/trace" || fields[0].String != wantTrace {
		t.Fatalf("unexpected trace field: %+v", fields[0])
	}
	if fields[1].Key != "logging.googleapis.com/spanId" || fields[1].String != "643745351650131537" {
		t.Fatalf("unexpected span field: %+v", fields[1])
	}
	if fields[2].Key != "logging.googleapis.com/trace_sampled" || fields[2].Type != zapcore.BoolType ||
		fields[2].Integer != 1 {
		t.Fatalf("unexpected sampled field: %+v", fields[2])
	}
}

func TestTraceFieldsWithoutSamplingDirective(t *testing.T) {
	header := "3d23d071b5bfd6579171efce907685cb/643745351650131537"
	projectID := "test-project"

	fields := traceFields(header, projectID)
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}

	wantTrace := fmt.Sprintf("projects/%s/traces/%s", projectID, "3d23d071b5bfd6579171efce907685cb")
	if fields[0].Key != "logging.googleapis.com/trace" || fields[0].String != wantTrace {
		t.Fatalf("unexpected trace field: %+v", fields[0])
	}
	if fields[1].Key != "logging.googleapis.com/spanId" || fields[1].String != "643745351650131537" {
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
	header := "3d23d071b5bfd6579171efce907685cb/643745351650131537;o=1"
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
	if f, ok := ctxFields["logging.googleapis.com/spanId"]; !ok || f.String != "643745351650131537" {
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
