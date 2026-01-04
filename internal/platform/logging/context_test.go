package logging

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

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

func TestLoggerFromContextNilContext(t *testing.T) {
	var nilCtx context.Context //nolint:revive // testing nil context handling
	logger := LoggerFromContext(nilCtx)
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
	var nilCtx context.Context //nolint:revive // testing nil context handling
	traceID := TraceIDFromContext(nilCtx)
	if traceID != nil {
		t.Fatalf("expected nil trace ID for nil context, got %v", traceID)
	}
}

func TestContextWithLoggerNilContext(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	var nilCtx context.Context //nolint:revive // testing nil context handling
	ctx := contextWithLogger(nilCtx, logger)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	LoggerFromContext(ctx).Info("test")
	if len(recorded.All()) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(recorded.All()))
	}
}

func TestContextWithTraceIDNilContext(t *testing.T) {
	var nilCtx context.Context //nolint:revive // testing nil context handling
	ctx := contextWithTraceID(nilCtx, "trace-123")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	traceID := TraceIDFromContext(ctx)
	if traceID == nil || *traceID != "trace-123" {
		t.Fatalf("expected trace-123, got %v", traceID)
	}
}
