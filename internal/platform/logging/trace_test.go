package logging

import (
	"fmt"
	"sync"
	"testing"

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

func TestResolveProjectID(t *testing.T) {
	result := resolveProjectID()
	if result != cachedProjectID {
		t.Fatalf("expected cached value %s, got %s", cachedProjectID, result)
	}
}

func TestResolveProjectIDPriority(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "FIREBASE_PROJECT_ID takes priority",
			envVars:  map[string]string{"FIREBASE_PROJECT_ID": "firebase-proj", "GOOGLE_CLOUD_PROJECT": "gcloud-proj"},
			expected: "firebase-proj",
		},
		{
			name:     "GOOGLE_CLOUD_PROJECT when FIREBASE_PROJECT_ID is empty",
			envVars:  map[string]string{"GOOGLE_CLOUD_PROJECT": "gcloud-proj", "GCP_PROJECT": "gcp-proj"},
			expected: "gcloud-proj",
		},
		{
			name:     "GCP_PROJECT fallback",
			envVars:  map[string]string{"GCP_PROJECT": "gcp-proj", "GCLOUD_PROJECT": "gcloud2-proj"},
			expected: "gcp-proj",
		},
		{
			name:     "GCLOUD_PROJECT fallback",
			envVars:  map[string]string{"GCLOUD_PROJECT": "gcloud2-proj", "PROJECT_ID": "proj-id"},
			expected: "gcloud2-proj",
		},
		{
			name:     "PROJECT_ID fallback",
			envVars:  map[string]string{"PROJECT_ID": "proj-id"},
			expected: "proj-id",
		},
		{
			name:     "empty when no env vars set",
			envVars:  map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectIDOnce = sync.Once{}
			cachedProjectID = ""

			// Clear all project ID env vars first
			for _, key := range []string{"FIREBASE_PROJECT_ID", "GOOGLE_CLOUD_PROJECT", "GCP_PROJECT", "GCLOUD_PROJECT", "PROJECT_ID"} {
				t.Setenv(key, "")
			}

			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			result := resolveProjectID()
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}
