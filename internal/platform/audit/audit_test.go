package audit

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/janisto/huma-observability"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogEventUsesRequestLogger(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	details := map[string]any{"field": "value"}
	handler := obs.HTTPRequestContext(obs.HTTPRequestContextConfig{Logger: logger})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			LogEvent(r.Context(), "create", "user-1", "profile", "profile-1", "success", details)
		}),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/audit", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "audit-req")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	entries := recorded.FilterMessage("Audit event").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	assertAuditLogField(t, fields, "request_id", "audit-req")
	assertAuditLogField(t, fields, "audit.action", "create")
	assertAuditLogField(t, fields, "audit.user_id", "user-1")
	assertAuditLogField(t, fields, "audit.resource_type", "profile")
	assertAuditLogField(t, fields, "audit.resource_id", "profile-1")
	assertAuditLogField(t, fields, "audit.result", "success")
	if got := fields["audit.details"]; !reflect.DeepEqual(got, details) {
		t.Fatalf("expected audit.details=%#v, got %#v", details, got)
	}
}

func assertAuditLogField(t *testing.T, fields map[string]any, key string, want any) {
	t.Helper()
	if got := fields[key]; got != want {
		t.Fatalf("expected log field %s=%v, got %v in %#v", key, want, got, fields)
	}
}
