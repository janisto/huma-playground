package logging

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestLogAuditEvent(t *testing.T) {
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

	ctx := context.Background()
	LogAuditEvent(ctx, "create", "user-123", "profile", "user-123", "success", nil)

	logger := Logger()
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

	if payload["message"] != "Audit event" {
		t.Errorf("expected message 'Audit event', got %v", payload["message"])
	}
	if payload["audit.action"] != "create" {
		t.Errorf("expected audit.action 'create', got %v", payload["audit.action"])
	}
	if payload["audit.user_id"] != "user-123" {
		t.Errorf("expected audit.user_id 'user-123', got %v", payload["audit.user_id"])
	}
	if payload["audit.resource_type"] != "profile" {
		t.Errorf("expected audit.resource_type 'profile', got %v", payload["audit.resource_type"])
	}
	if payload["audit.resource_id"] != "user-123" {
		t.Errorf("expected audit.resource_id 'user-123', got %v", payload["audit.resource_id"])
	}
	if payload["audit.result"] != "success" {
		t.Errorf("expected audit.result 'success', got %v", payload["audit.result"])
	}
}

func TestLogAuditEventWithDetails(t *testing.T) {
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

	ctx := context.Background()
	details := map[string]any{"fields": []string{"firstname", "email"}}
	LogAuditEvent(ctx, "update", "user-456", "profile", "user-456", "success", details)

	logger := Logger()
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

	if payload["audit.action"] != "update" {
		t.Errorf("expected audit.action 'update', got %v", payload["audit.action"])
	}

	auditDetails, ok := payload["audit.details"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit.details to be a map, got %T", payload["audit.details"])
	}

	fields, ok := auditDetails["fields"].([]any)
	if !ok {
		t.Fatalf("expected fields to be an array, got %T", auditDetails["fields"])
	}
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
}

func TestLogAuditEventFailure(t *testing.T) {
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

	ctx := context.Background()
	details := map[string]any{"reason": "not found"}
	LogAuditEvent(ctx, "delete", "user-789", "profile", "user-789", "failure", details)

	logger := Logger()
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

	if payload["audit.action"] != "delete" {
		t.Errorf("expected audit.action 'delete', got %v", payload["audit.action"])
	}
	if payload["audit.result"] != "failure" {
		t.Errorf("expected audit.result 'failure', got %v", payload["audit.result"])
	}

	auditDetails, ok := payload["audit.details"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit.details to be a map, got %T", payload["audit.details"])
	}
	if auditDetails["reason"] != "not found" {
		t.Errorf("expected reason 'not found', got %v", auditDetails["reason"])
	}
}
