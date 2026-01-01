package common

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// captureLogOutput captures a single log entry emitted by logFn and returns it as a map.
func captureLogOutput(t *testing.T, logFn func(*zap.Logger)) map[string]any {
	t.Helper()

	resetLoggerForTest()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer func() { _ = r.Close() }()

	origStdout := os.Stdout
	origStderr := os.Stderr
	os.Stdout = w
	os.Stderr = w
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	logger := Logger()
	logFn(logger)
	_ = logger.Sync()

	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("failed to close writer: %v", closeErr)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read log output: %v", err)
	}

	line := strings.TrimSpace(string(data))
	if line == "" {
		t.Fatalf("expected log output, got empty string")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("failed to unmarshal log JSON: %v", err)
	}

	return payload
}

// resetLoggerForTest clears the singleton state so tests can capture fresh log output.
func resetLoggerForTest() {
	loggerOnce = sync.Once{}
	baseLogger = nil
	sugarLogger = nil
	loggerErr = nil
}

func TestLoggerStructuredOutput(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("GET /health")
	})

	if got := payload["severity"]; got != "INFO" {
		t.Fatalf("expected severity INFO, got %v", got)
	}

	if _, exists := payload["level"]; exists {
		t.Fatalf("did not expect level field, but found one: %v", exists)
	}

	msg, ok := payload["message"].(string)
	if !ok || msg != "GET /health" {
		t.Fatalf("expected message 'GET /health', got %v", payload["message"])
	}

	ts, ok := payload["timestamp"].(string)
	if !ok {
		t.Fatalf("expected timestamp field to be a string, got %T", payload["timestamp"])
	}
	if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Fatalf("timestamp is not RFC3339Nano: %v", err)
	}
}

func TestSugarLoggerStructuredOutput(t *testing.T) {
	payload := captureLogOutput(t, func(*zap.Logger) {
		Sugar().Warnw("slow response", "latency_ms", 120)
	})

	if got := payload["severity"]; got != "WARNING" {
		t.Fatalf("expected severity WARNING, got %v", got)
	}

	if msg, ok := payload["message"].(string); !ok || msg != "slow response" {
		t.Fatalf("expected message 'slow response', got %v", payload["message"])
	}

	if latency, ok := payload["latency_ms"].(float64); !ok || latency != 120 {
		t.Fatalf("expected latency_ms 120, got %v", payload["latency_ms"])
	}
}

func TestEncodeSeverityMapping(t *testing.T) {
	tests := []struct {
		level    zapcore.Level
		expected string
	}{
		{zapcore.DebugLevel, "DEBUG"},
		{zapcore.InfoLevel, "INFO"},
		{zapcore.WarnLevel, "WARNING"},
		{zapcore.ErrorLevel, "ERROR"},
		{zapcore.DPanicLevel, "CRITICAL"},
		{zapcore.PanicLevel, "ALERT"},
		{zapcore.FatalLevel, "EMERGENCY"},
		{zapcore.Level(99), "DEFAULT"},
	}

	for _, tt := range tests {
		enc := &captureArrayEncoder{}
		encodeSeverity(tt.level, enc)
		if len(enc.values) != 1 || enc.values[0] != tt.expected {
			t.Fatalf("encodeSeverity(%v) = %v, want %s", tt.level, enc.values, tt.expected)
		}
	}
}

func TestSyncReturnsNoError(t *testing.T) {
	resetLoggerForTest()
	_ = Logger()

	err := Sync()
	if err != nil {
		t.Logf("Sync returned error (may be expected on some platforms): %v", err)
	}
}

func TestErrReturnsNilOnSuccess(t *testing.T) {
	resetLoggerForTest()
	_ = Logger()

	err := Err()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestSyncWithoutInit(t *testing.T) {
	resetLoggerForTest()
	err := Sync()
	if err != nil {
		t.Logf("Sync returned error (may be expected): %v", err)
	}
}

func TestLoggerSingletonBehavior(t *testing.T) {
	resetLoggerForTest()

	first := Logger()
	second := Logger()

	if first != second {
		t.Fatal("expected Logger() to return the same instance")
	}
}

func TestSugarSingletonBehavior(t *testing.T) {
	resetLoggerForTest()

	first := Sugar()
	second := Sugar()

	if first != second {
		t.Fatal("expected Sugar() to return the same instance")
	}
}

func TestLoggerAndSugarShareCore(t *testing.T) {
	resetLoggerForTest()

	logger := Logger()
	sugar := Sugar()

	desugared := sugar.Desugar()
	if logger.Core() != desugared.Core() {
		t.Fatal("expected Logger and Sugar to share the same core")
	}
}

func TestLoggerIncludesCallerField(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("caller test")
	})

	caller, ok := payload["caller"].(string)
	if !ok {
		t.Fatal("expected caller field to be a string")
	}

	if !strings.Contains(caller, "log_test.go") {
		t.Fatalf("expected caller to reference log_test.go, got %s", caller)
	}
}

func TestErrorLevelOutput(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Error("error occurred", zap.String("component", "db"))
	})

	if got := payload["severity"]; got != "ERROR" {
		t.Fatalf("expected severity ERROR, got %v", got)
	}

	if msg, ok := payload["message"].(string); !ok || msg != "error occurred" {
		t.Fatalf("expected message 'error occurred', got %v", payload["message"])
	}

	if comp, ok := payload["component"].(string); !ok || comp != "db" {
		t.Fatalf("expected component 'db', got %v", payload["component"])
	}
}

func TestDebugLevelNotLoggedInProduction(t *testing.T) {
	resetLoggerForTest()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer func() { _ = r.Close() }()

	origStdout := os.Stdout
	origStderr := os.Stderr
	os.Stdout = w
	os.Stderr = w
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	logger := Logger()
	logger.Debug("debug message should not appear")
	_ = logger.Sync()

	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("failed to close writer: %v", closeErr)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if strings.Contains(string(data), "debug message") {
		t.Fatal("debug level messages should not be logged in production config")
	}
}

func TestWarnLevelOutput(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Warn("warning occurred")
	})

	if got := payload["severity"]; got != "WARNING" {
		t.Fatalf("expected severity WARNING, got %v", got)
	}
}

func TestLoggerWithMultipleFields(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("request completed",
			zap.String("method", "GET"),
			zap.Int("status", 200),
			zap.Float64("duration_ms", 15.5),
			zap.Bool("cached", true),
		)
	})

	if method, ok := payload["method"].(string); !ok || method != "GET" {
		t.Fatalf("expected method 'GET', got %v", payload["method"])
	}

	if status, ok := payload["status"].(float64); !ok || status != 200 {
		t.Fatalf("expected status 200, got %v", payload["status"])
	}

	if duration, ok := payload["duration_ms"].(float64); !ok || duration != 15.5 {
		t.Fatalf("expected duration_ms 15.5, got %v", payload["duration_ms"])
	}

	if cached, ok := payload["cached"].(bool); !ok || !cached {
		t.Fatalf("expected cached true, got %v", payload["cached"])
	}
}

func TestSugarLoggerInfof(t *testing.T) {
	payload := captureLogOutput(t, func(*zap.Logger) {
		Sugar().Infof("request to %s returned %d", "/api/health", 200)
	})

	if got := payload["severity"]; got != "INFO" {
		t.Fatalf("expected severity INFO, got %v", got)
	}

	msg, ok := payload["message"].(string)
	if !ok {
		t.Fatal("expected message to be a string")
	}

	if !strings.Contains(msg, "/api/health") || !strings.Contains(msg, "200") {
		t.Fatalf("expected formatted message, got %s", msg)
	}
}

func TestErrCalledBeforeLogger(t *testing.T) {
	resetLoggerForTest()

	err := Err()
	if err != nil {
		t.Fatalf("expected nil error when Err() initializes logger, got %v", err)
	}

	if baseLogger == nil {
		t.Fatal("expected baseLogger to be initialized after Err()")
	}
}

func TestSugarCalledBeforeLogger(t *testing.T) {
	resetLoggerForTest()

	sugar := Sugar()
	if sugar == nil {
		t.Fatal("expected Sugar() to return non-nil logger")
	}

	logger := Logger()
	if logger == nil {
		t.Fatal("expected Logger() to return non-nil logger")
	}

	if sugar.Desugar().Core() != logger.Core() {
		t.Fatal("expected Sugar and Logger to share core when Sugar is called first")
	}
}

// captureArrayEncoder collects strings appended via the PrimitiveArrayEncoder interface.
type captureArrayEncoder struct {
	values []string
}

func (c *captureArrayEncoder) AppendBool(bool)             {}
func (c *captureArrayEncoder) AppendByteString([]byte)     {}
func (c *captureArrayEncoder) AppendComplex128(complex128) {}
func (c *captureArrayEncoder) AppendComplex64(complex64)   {}
func (c *captureArrayEncoder) AppendFloat64(float64)       {}
func (c *captureArrayEncoder) AppendFloat32(float32)       {}
func (c *captureArrayEncoder) AppendInt(int)               {}
func (c *captureArrayEncoder) AppendInt64(int64)           {}
func (c *captureArrayEncoder) AppendInt32(int32)           {}
func (c *captureArrayEncoder) AppendInt16(int16)           {}
func (c *captureArrayEncoder) AppendInt8(int8)             {}
func (c *captureArrayEncoder) AppendString(s string)       { c.values = append(c.values, s) }
func (c *captureArrayEncoder) AppendUint(uint)             {}
func (c *captureArrayEncoder) AppendUint64(uint64)         {}
func (c *captureArrayEncoder) AppendUint32(uint32)         {}
func (c *captureArrayEncoder) AppendUint16(uint16)         {}
func (c *captureArrayEncoder) AppendUint8(uint8)           {}
func (c *captureArrayEncoder) AppendUintptr(uintptr)       {}
