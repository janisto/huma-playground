package common

import (
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	loggerOnce  sync.Once
	baseLogger  *zap.Logger
	sugarLogger *zap.SugaredLogger
	loggerErr   error
)

// encodeTimeMicros formats timestamps as RFC 3339 with fixed microsecond precision.
func encodeTimeMicros(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format(RFC3339Micros))
}

// initLogger lazily constructs the shared zap logger instance.
func initLogger() {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stdout"}
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = encodeTimeMicros
	cfg.EncoderConfig.LevelKey = "severity"
	cfg.EncoderConfig.EncodeLevel = encodeSeverity
	cfg.EncoderConfig.MessageKey = "message"
	cfg.EncoderConfig.CallerKey = "caller"

	baseLogger, loggerErr = cfg.Build(zap.AddCaller())
	if loggerErr != nil {
		baseLogger = zap.NewNop()
	}
	sugarLogger = baseLogger.Sugar()
}

// encodeSeverity maps zap levels to Cloud Logging severity names.
func encodeSeverity(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	var severity string
	switch level {
	case zapcore.DebugLevel:
		severity = "DEBUG"
	case zapcore.InfoLevel:
		severity = "INFO"
	case zapcore.WarnLevel:
		severity = "WARNING"
	case zapcore.ErrorLevel:
		severity = "ERROR"
	case zapcore.DPanicLevel:
		severity = "CRITICAL"
	case zapcore.PanicLevel:
		severity = "ALERT"
	case zapcore.FatalLevel:
		severity = "EMERGENCY"
	default:
		severity = "DEFAULT"
	}
	enc.AppendString(severity)
}

// Logger returns the process-wide zap.Logger instance.
func Logger() *zap.Logger {
	loggerOnce.Do(initLogger)
	return baseLogger
}

// Sugar returns a sugared logger sharing the same core as Logger.
func Sugar() *zap.SugaredLogger {
	loggerOnce.Do(initLogger)
	return sugarLogger
}

// Sync flushes buffered log entries. Call during shutdown.
func Sync() error {
	loggerOnce.Do(initLogger)
	return baseLogger.Sync()
}

// Err reports initialization failure, if any.
func Err() error {
	loggerOnce.Do(initLogger)
	return loggerErr
}
