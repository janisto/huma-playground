package logging

import (
	"context"

	"go.uber.org/zap"
)

// ctxLoggerKey is used to store the request-specific logger in context.
type (
	ctxLoggerKey  struct{}
	ctxTraceIDKey struct{}
)

// LoggerFromContext returns the request-scoped logger if present, otherwise falls back to the global logger.
func LoggerFromContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return Logger()
	}
	if l, ok := ctx.Value(ctxLoggerKey{}).(*zap.Logger); ok && l != nil {
		return l
	}
	return Logger()
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
