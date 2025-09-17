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
	defer r.Close()

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

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
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
