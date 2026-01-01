package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func TestRequestIDGeneratesUUIDv4(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	var captured string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = chimiddleware.GetReqID(r.Context())
	}))

	h.ServeHTTP(rec, req)

	if captured == "" {
		t.Fatalf("expected generated request ID")
	}
	if header := rec.Header().Get(chimiddleware.RequestIDHeader); header != captured {
		t.Fatalf("expected response header %q, got %q", captured, header)
	}
	parsed, err := uuid.Parse(captured)
	if err != nil {
		t.Fatalf("request ID %q is not a valid UUID: %v", captured, err)
	}
	if parsed.Version() != 4 {
		t.Fatalf("expected UUIDv4, got version %d", parsed.Version())
	}
}

func TestRequestIDPreservesIncomingHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "external-id")

	var captured string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = chimiddleware.GetReqID(r.Context())
	}))

	h.ServeHTTP(rec, req)

	if captured != "external-id" {
		t.Fatalf("expected request ID to remain external-id, got %q", captured)
	}
	if header := rec.Header().Get(chimiddleware.RequestIDHeader); header != "external-id" {
		t.Fatalf("expected header external-id, got %q", header)
	}
}

func TestRequestIDRejectsInvalidHeaders(t *testing.T) {
	tests := []struct {
		name    string
		inputID string
		wantNew bool
	}{
		{
			name:    "empty string generates new UUID",
			inputID: "",
			wantNew: true,
		},
		{
			name:    "valid alphanumeric is preserved",
			inputID: "abc123-XYZ",
			wantNew: false,
		},
		{
			name:    "valid UUID is preserved",
			inputID: "550e8400-e29b-41d4-a716-446655440000",
			wantNew: false,
		},
		{
			name:    "newline causes rejection (log injection)",
			inputID: "valid\ninjected-line",
			wantNew: true,
		},
		{
			name:    "carriage return causes rejection",
			inputID: "valid\rinjected",
			wantNew: true,
		},
		{
			name:    "null byte causes rejection",
			inputID: "valid\x00null",
			wantNew: true,
		},
		{
			name:    "tab causes rejection",
			inputID: "valid\ttab",
			wantNew: true,
		},
		{
			name:    "DEL character (0x7F) causes rejection",
			inputID: "valid\x7Fdel",
			wantNew: true,
		},
		{
			name:    "high byte (0x80+) causes rejection",
			inputID: "valid\x80high",
			wantNew: true,
		},
		{
			name:    "too long (>128 chars) causes rejection",
			inputID: strings.Repeat("a", 129),
			wantNew: true,
		},
		{
			name:    "exactly max length (128) is preserved",
			inputID: strings.Repeat("x", 128),
			wantNew: false,
		},
		{
			name:    "printable ASCII with special chars is preserved",
			inputID: "trace:abc-123_def.456!@#$%",
			wantNew: false,
		},
		{
			name:    "spaces are preserved",
			inputID: "trace id 123",
			wantNew: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(chimiddleware.RequestIDHeader, tc.inputID)

			var captured string
			h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				captured = chimiddleware.GetReqID(r.Context())
			}))

			h.ServeHTTP(rec, req)

			if tc.wantNew {
				// Should have generated a new UUID
				if captured == tc.inputID {
					t.Fatalf("expected new UUID, but got original: %q", captured)
				}
				parsed, err := uuid.Parse(captured)
				if err != nil {
					t.Fatalf("expected valid UUID, got %q: %v", captured, err)
				}
				if parsed.Version() != 4 {
					t.Fatalf("expected UUIDv4, got version %d", parsed.Version())
				}
			} else {
				// Should preserve the original
				if captured != tc.inputID {
					t.Fatalf("expected %q, got %q", tc.inputID, captured)
				}
			}
		})
	}
}

func TestIsValidRequestID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"", false},
		{"a", true},
		{"abc123", true},
		{"ABC-xyz_123.456", true},
		{strings.Repeat("a", 128), true},
		{strings.Repeat("a", 129), false},
		{"hello\nworld", false},
		{"hello\rworld", false},
		{"hello\tworld", false},
		{"hello\x00world", false},
		{"hello\x1fworld", false},
		{"hello\x7fworld", false},
		{"hello\x80world", false},
		{" leading space", true},
		{"trailing space ", true},
		{"special!@#$%^&*()", true},
	}

	for _, tc := range tests {
		got := isValidRequestID(tc.id)
		if got != tc.valid {
			t.Errorf("isValidRequestID(%q) = %v, want %v", tc.id, got, tc.valid)
		}
	}
}
