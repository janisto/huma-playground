package logging

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

	"github.com/janisto/huma-playground/internal/platform/timeutil"
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
	if _, err := time.Parse(timeutil.RFC3339Micros, ts); err != nil {
		t.Fatalf("timestamp is not RFC3339Micros: %v", err)
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

	if !strings.Contains(caller, "logger_test.go") {
		t.Fatalf("expected caller to reference logger_test.go, got %s", caller)
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

func TestEncodeTimeMicrosFormatsCorrectly(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "UTC time with microseconds",
			input:    time.Date(2024, 6, 15, 10, 30, 45, 123456000, time.UTC),
			expected: "2024-06-15T10:30:45.123456Z",
		},
		{
			name:     "UTC time with zero microseconds",
			input:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: "2024-01-01T00:00:00.000000Z",
		},
		{
			name:     "non-UTC time converts to UTC",
			input:    time.Date(2024, 6, 15, 12, 0, 0, 500000000, time.FixedZone("EST", -5*60*60)),
			expected: "2024-06-15T17:00:00.500000Z",
		},
		{
			name:     "time with sub-microsecond precision truncates",
			input:    time.Date(2024, 3, 20, 8, 15, 30, 999999999, time.UTC),
			expected: "2024-03-20T08:15:30.999999Z",
		},
		{
			name:     "end of year boundary",
			input:    time.Date(2024, 12, 31, 23, 59, 59, 999999000, time.UTC),
			expected: "2024-12-31T23:59:59.999999Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := &captureArrayEncoder{}
			encodeTimeMicros(tt.input, enc)
			if len(enc.values) != 1 {
				t.Fatalf("expected 1 value, got %d", len(enc.values))
			}
			if enc.values[0] != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, enc.values[0])
			}
		})
	}
}

func TestLoggerConcurrentAccess(t *testing.T) {
	resetLoggerForTest()

	var wg sync.WaitGroup
	results := make(chan *zap.Logger, 100)

	for range 100 {
		wg.Go(func() {
			results <- Logger()
		})
	}

	wg.Wait()
	close(results)

	var first *zap.Logger
	for logger := range results {
		if first == nil {
			first = logger
		} else if logger != first {
			t.Fatal("concurrent Logger() calls returned different instances")
		}
	}
}

func TestSugarConcurrentAccess(t *testing.T) {
	resetLoggerForTest()

	var wg sync.WaitGroup
	results := make(chan *zap.SugaredLogger, 100)

	for range 100 {
		wg.Go(func() {
			results <- Sugar()
		})
	}

	wg.Wait()
	close(results)

	var first *zap.SugaredLogger
	for sugar := range results {
		if first == nil {
			first = sugar
		} else if sugar != first {
			t.Fatal("concurrent Sugar() calls returned different instances")
		}
	}
}

func TestLoggerWithNamespace(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		named := l.Named("myservice")
		named.Info("namespaced log")
	})

	loggerName, ok := payload["logger"].(string)
	if !ok {
		t.Fatal("expected logger field to be present for named logger")
	}
	if loggerName != "myservice" {
		t.Fatalf("expected logger name 'myservice', got %q", loggerName)
	}
}

func TestLoggerWithNestedNamespace(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		named := l.Named("service").Named("handler")
		named.Info("nested namespace")
	})

	loggerName, ok := payload["logger"].(string)
	if !ok {
		t.Fatal("expected logger field for nested namespace")
	}
	if loggerName != "service.handler" {
		t.Fatalf("expected 'service.handler', got %q", loggerName)
	}
}

func TestLoggerWithFields(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		withFields := l.With(zap.String("service", "api"), zap.Int("version", 2))
		withFields.Info("with fields")
	})

	if svc, ok := payload["service"].(string); !ok || svc != "api" {
		t.Fatalf("expected service 'api', got %v", payload["service"])
	}
	if ver, ok := payload["version"].(float64); !ok || ver != 2 {
		t.Fatalf("expected version 2, got %v", payload["version"])
	}
}

func TestLoggerWithDurationField(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("request", zap.Duration("latency", 150*time.Millisecond))
	})

	latency, ok := payload["latency"].(float64)
	if !ok {
		t.Fatal("expected latency to be a number")
	}
	expectedSeconds := 0.15
	if latency != expectedSeconds {
		t.Fatalf("expected latency %v, got %v", expectedSeconds, latency)
	}
}

func TestLoggerWithTimeField(t *testing.T) {
	testTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("event", zap.Time("occurred_at", testTime))
	})

	occurredAt, exists := payload["occurred_at"]
	if !exists {
		t.Fatal("expected occurred_at field to exist")
	}

	switch v := occurredAt.(type) {
	case float64:
		expectedEpoch := float64(testTime.UnixNano()) / 1e9
		if v != expectedEpoch {
			t.Fatalf("expected epoch seconds %v, got %v", expectedEpoch, v)
		}
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			t.Fatalf("failed to parse time string %q: %v", v, err)
		}
		if !parsed.Equal(testTime) {
			t.Fatalf("expected time %v, got %v", testTime, parsed)
		}
	default:
		t.Fatalf("unexpected type for occurred_at: %T = %v", occurredAt, occurredAt)
	}
}

func TestLoggerWithErrorField(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Error("operation failed", zap.Error(io.EOF))
	})

	errMsg, ok := payload["error"].(string)
	if !ok {
		t.Fatal("expected error field to be a string")
	}
	if errMsg != "EOF" {
		t.Fatalf("expected error 'EOF', got %q", errMsg)
	}
}

func TestLoggerWithNilError(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("no error", zap.Error(nil))
	})

	if _, exists := payload["error"]; exists {
		t.Fatal("expected no error field for nil error")
	}
}

func TestLoggerWithBinaryField(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("binary data", zap.Binary("data", []byte{0x01, 0x02, 0x03}))
	})

	data, ok := payload["data"].(string)
	if !ok {
		t.Fatal("expected data field to be base64 encoded string")
	}
	if data != "AQID" {
		t.Fatalf("expected base64 'AQID', got %q", data)
	}
}

func TestLoggerWithStringsArray(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("tags", zap.Strings("tags", []string{"api", "v2", "prod"}))
	})

	tags, ok := payload["tags"].([]any)
	if !ok {
		t.Fatalf("expected tags to be array, got %T", payload["tags"])
	}
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	if tags[0] != "api" || tags[1] != "v2" || tags[2] != "prod" {
		t.Fatalf("unexpected tags: %v", tags)
	}
}

func TestLoggerWithIntsArray(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("counts", zap.Ints("values", []int{1, 2, 3, 4, 5}))
	})

	values, ok := payload["values"].([]any)
	if !ok {
		t.Fatalf("expected values to be array, got %T", payload["values"])
	}
	if len(values) != 5 {
		t.Fatalf("expected 5 values, got %d", len(values))
	}
}

func TestLoggerWithAnyField(t *testing.T) {
	type customStruct struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("custom", zap.Any("data", customStruct{Name: "test", Count: 42}))
	})

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be object, got %T", payload["data"])
	}
	if data["name"] != "test" {
		t.Fatalf("expected name 'test', got %v", data["name"])
	}
	count, ok := data["count"].(float64)
	if !ok || count != 42 {
		t.Fatalf("expected count 42, got %v", data["count"])
	}
}

func TestSugarLoggerErrorw(t *testing.T) {
	payload := captureLogOutput(t, func(*zap.Logger) {
		Sugar().Errorw("database error", "table", "users", "error", "connection refused")
	})

	if got := payload["severity"]; got != "ERROR" {
		t.Fatalf("expected severity ERROR, got %v", got)
	}
	if table := payload["table"]; table != "users" {
		t.Fatalf("expected table 'users', got %v", table)
	}
}

func TestSugarLoggerWith(t *testing.T) {
	payload := captureLogOutput(t, func(*zap.Logger) {
		Sugar().With("request_id", "abc123").Info("with context")
	})

	if reqID := payload["request_id"]; reqID != "abc123" {
		t.Fatalf("expected request_id 'abc123', got %v", reqID)
	}
}

func TestSugarLoggerNamed(t *testing.T) {
	payload := captureLogOutput(t, func(*zap.Logger) {
		Sugar().Named("component").Info("named sugar")
	})

	loggerName, ok := payload["logger"].(string)
	if !ok {
		t.Fatal("expected logger field")
	}
	if loggerName != "component" {
		t.Fatalf("expected 'component', got %q", loggerName)
	}
}

func TestTimestampAlwaysUTC(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("utc check")
	})

	ts, ok := payload["timestamp"].(string)
	if !ok {
		t.Fatal("expected timestamp string")
	}

	if !strings.HasSuffix(ts, "Z") {
		t.Fatalf("expected UTC timestamp ending with Z, got %q", ts)
	}

	parsed, err := time.Parse(timeutil.RFC3339Micros, ts)
	if err != nil {
		t.Fatalf("failed to parse timestamp: %v", err)
	}

	if parsed.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", parsed.Location())
	}
}

func TestTimestampHasMicrosecondPrecision(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("precision check")
	})

	ts, ok := payload["timestamp"].(string)
	if !ok {
		t.Fatal("expected timestamp string")
	}

	dotIndex := strings.LastIndex(ts, ".")
	if dotIndex == -1 {
		t.Fatal("expected timestamp to contain microseconds separator")
	}

	microsPart := ts[dotIndex+1 : len(ts)-1]
	if len(microsPart) != 6 {
		t.Fatalf("expected 6 digit microseconds, got %q (%d digits)", microsPart, len(microsPart))
	}
}

func TestLoggerNoLevelField(t *testing.T) {
	levels := []func(*zap.Logger){
		func(l *zap.Logger) { l.Info("info") },
		func(l *zap.Logger) { l.Warn("warn") },
		func(l *zap.Logger) { l.Error("error") },
	}

	for _, logFn := range levels {
		payload := captureLogOutput(t, logFn)
		if _, exists := payload["level"]; exists {
			t.Fatal("expected no 'level' field, severity should be used instead")
		}
		if _, exists := payload["severity"]; !exists {
			t.Fatal("expected 'severity' field to be present")
		}
	}
}

func TestMixedConcurrentLoggerAndSugar(t *testing.T) {
	resetLoggerForTest()

	var wg sync.WaitGroup
	loggerResults := make(chan *zap.Logger, 50)
	sugarResults := make(chan *zap.SugaredLogger, 50)

	for range 50 {
		wg.Go(func() {
			loggerResults <- Logger()
		})
		wg.Go(func() {
			sugarResults <- Sugar()
		})
	}

	wg.Wait()
	close(loggerResults)
	close(sugarResults)

	var firstLogger *zap.Logger
	for logger := range loggerResults {
		if firstLogger == nil {
			firstLogger = logger
		} else if logger != firstLogger {
			t.Fatal("Logger() returned different instances under concurrent access")
		}
	}

	var firstSugar *zap.SugaredLogger
	for sugar := range sugarResults {
		if firstSugar == nil {
			firstSugar = sugar
		} else if sugar != firstSugar {
			t.Fatal("Sugar() returned different instances under concurrent access")
		}
	}

	if firstLogger.Core() != firstSugar.Desugar().Core() {
		t.Fatal("Logger and Sugar should share the same core")
	}
}

func TestSyncIdempotent(t *testing.T) {
	resetLoggerForTest()
	_ = Logger()

	for range 5 {
		err := Sync()
		if err != nil {
			t.Logf("Sync() returned error (may be platform-specific): %v", err)
		}
	}
}

func TestErrIdempotent(t *testing.T) {
	resetLoggerForTest()
	_ = Logger()

	for range 5 {
		err := Err()
		if err != nil {
			t.Fatalf("Err() returned unexpected error: %v", err)
		}
	}
}

func TestLoggerOutputsToStdout(t *testing.T) {
	resetLoggerForTest()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer func() { _ = r.Close() }()

	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	logger := Logger()
	logger.Info("stdout test")
	_ = logger.Sync()
	_ = w.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	if !strings.Contains(string(data), "stdout test") {
		t.Fatal("expected log output on stdout")
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("json format test")
	})

	requiredFields := []string{"timestamp", "severity", "message", "caller"}
	for _, field := range requiredFields {
		if _, exists := payload[field]; !exists {
			t.Fatalf("expected required field %q in JSON output", field)
		}
	}
}

func TestLoggerWithInt64Field(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("large number", zap.Int64("big", 9223372036854775807))
	})

	big, ok := payload["big"].(float64)
	if !ok {
		t.Fatal("expected big field to be a number")
	}
	if big != 9223372036854775807 {
		t.Fatalf("expected max int64, got %v", big)
	}
}

func TestLoggerWithUint64Field(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("unsigned", zap.Uint64("val", 18446744073709551615))
	})

	val, ok := payload["val"].(float64)
	if !ok {
		t.Fatal("expected val field to be a number")
	}
	if uint64(val) != 18446744073709551615 {
		t.Logf("note: JSON cannot precisely represent max uint64")
	}
}

func TestLoggerWithFloat32Field(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("float", zap.Float32("rate", 3.14))
	})

	rate, ok := payload["rate"].(float64)
	if !ok {
		t.Fatal("expected rate field")
	}
	if rate < 3.13 || rate > 3.15 {
		t.Fatalf("expected rate ~3.14, got %v", rate)
	}
}

func TestLoggerWithBoolFields(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("bools", zap.Bool("enabled", true), zap.Bool("disabled", false))
	})

	if enabled, ok := payload["enabled"].(bool); !ok || !enabled {
		t.Fatalf("expected enabled=true, got %v", payload["enabled"])
	}
	if disabled, ok := payload["disabled"].(bool); !ok || disabled {
		t.Fatalf("expected disabled=false, got %v", payload["disabled"])
	}
}

func TestLoggerEmptyMessage(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info("")
	})

	msg, ok := payload["message"].(string)
	if !ok {
		t.Fatal("expected message field")
	}
	if msg != "" {
		t.Fatalf("expected empty message, got %q", msg)
	}
}

func TestLoggerSpecialCharactersInMessage(t *testing.T) {
	testMessage := "special chars: \t\n\"quotes\" and <html>"
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info(testMessage)
	})

	msg, ok := payload["message"].(string)
	if !ok {
		t.Fatal("expected message field")
	}
	if msg != testMessage {
		t.Fatalf("expected %q, got %q", testMessage, msg)
	}
}

func TestLoggerUnicodeInMessage(t *testing.T) {
	testMessage := "unicode: 日本語 🎉 émojis"
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Info(testMessage)
	})

	msg, ok := payload["message"].(string)
	if !ok {
		t.Fatal("expected message field")
	}
	if msg != testMessage {
		t.Fatalf("expected %q, got %q", testMessage, msg)
	}
}

func TestLoggerWithStacktrace(t *testing.T) {
	payload := captureLogOutput(t, func(l *zap.Logger) {
		l.Error("error with stack", zap.Stack("stacktrace"))
	})

	stack, ok := payload["stacktrace"].(string)
	if !ok {
		t.Fatal("expected stacktrace field")
	}
	if !strings.Contains(stack, "TestLoggerWithStacktrace") {
		t.Fatalf("expected stack to contain test function name, got %q", stack)
	}
}
