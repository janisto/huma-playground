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

func TestRequestIDSetsResponseHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(rec, req)

	header := rec.Header().Get(chimiddleware.RequestIDHeader)
	if header == "" {
		t.Fatal("expected X-Request-Id response header to be set")
	}

	_, err := uuid.Parse(header)
	if err != nil {
		t.Fatalf("response header %q is not a valid UUID: %v", header, err)
	}
}

func TestRequestIDContextValueMatchesHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "trace-correlation-123")

	var contextValue string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextValue = chimiddleware.GetReqID(r.Context())
	}))

	h.ServeHTTP(rec, req)

	headerValue := rec.Header().Get(chimiddleware.RequestIDHeader)
	if contextValue != headerValue {
		t.Fatalf("context value %q does not match header value %q", contextValue, headerValue)
	}
}

func TestRequestIDMultipleRequests(t *testing.T) {
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ids := make(map[string]bool)
	for i := range 10 {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		h.ServeHTTP(rec, req)

		id := rec.Header().Get(chimiddleware.RequestIDHeader)
		if ids[id] {
			t.Fatalf("duplicate request ID generated on iteration %d: %s", i, id)
		}
		ids[id] = true
	}
}

func TestRequestIDWithDifferentHTTPMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/resource", nil)
			req.Header.Set(chimiddleware.RequestIDHeader, "method-test-id")

			var captured string
			h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				captured = chimiddleware.GetReqID(r.Context())
			}))

			h.ServeHTTP(rec, req)

			if captured != "method-test-id" {
				t.Fatalf("expected method-test-id for %s, got %q", method, captured)
			}
		})
	}
}

func TestRequestIDPreservesOtherHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Authorization", "Bearer token")

	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Fatal("X-Custom-Header was modified")
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatal("Authorization header was modified")
		}
		w.Header().Set("X-Response-Custom", "response-value")
	}))

	h.ServeHTTP(rec, req)

	if rec.Header().Get("X-Response-Custom") != "response-value" {
		t.Fatal("downstream response header was not preserved")
	}
}

func TestRequestIDHandlerChaining(t *testing.T) {
	var capturedID string

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = chimiddleware.GetReqID(r.Context())
		w.WriteHeader(http.StatusCreated)
	})

	outer := RequestID()(inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/items", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "chained-request-id")

	outer.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if capturedID != "chained-request-id" {
		t.Fatalf("expected chained-request-id, got %q", capturedID)
	}
}

func TestIsValidRequestIDAllControlCharacters(t *testing.T) {
	for c := range byte(0x20) {
		id := "test" + string(c) + "value"
		if isValidRequestID(id) {
			t.Errorf("control character 0x%02X should be rejected", c)
		}
	}
}

func TestIsValidRequestIDAllHighBytes(t *testing.T) {
	for c := 0x80; c <= 0xFF; c++ {
		id := "test" + string(byte(c)) + "value"
		if isValidRequestID(id) {
			t.Errorf("high byte 0x%02X should be rejected", c)
		}
	}
}

func TestIsValidRequestIDPrintableASCIIRange(t *testing.T) {
	for c := byte(0x20); c <= 0x7E; c++ {
		id := string(c)
		if !isValidRequestID(id) {
			t.Errorf("printable ASCII character 0x%02X (%q) should be accepted", c, string(c))
		}
	}
}

func TestIsValidRequestIDBoundaryCharacters(t *testing.T) {
	tests := []struct {
		name  string
		char  byte
		valid bool
	}{
		{"just below space (0x1F)", 0x1F, false},
		{"space (0x20)", 0x20, true},
		{"tilde (0x7E)", 0x7E, true},
		{"DEL (0x7F)", 0x7F, false},
		{"first high byte (0x80)", 0x80, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id := "prefix" + string(tc.char) + "suffix"
			got := isValidRequestID(id)
			if got != tc.valid {
				t.Errorf("isValidRequestID with byte 0x%02X = %v, want %v", tc.char, got, tc.valid)
			}
		})
	}
}

func TestRequestIDWithW3CTraceparentFormat(t *testing.T) {
	traceparent := "00-ab42124a3c573678d4d8b21ba52df3bf-d21f7bc17caa5aba-01"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, traceparent)

	var captured string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = chimiddleware.GetReqID(r.Context())
	}))

	h.ServeHTTP(rec, req)

	if captured != traceparent {
		t.Fatalf("expected traceparent format to be preserved, got %q", captured)
	}
}

func TestRequestIDWithBase64EncodedValue(t *testing.T) {
	base64ID := "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo="

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, base64ID)

	var captured string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = chimiddleware.GetReqID(r.Context())
	}))

	h.ServeHTTP(rec, req)

	if captured != base64ID {
		t.Fatalf("expected base64 ID to be preserved, got %q", captured)
	}
}

func TestRequestIDWithURLEncodedCharacters(t *testing.T) {
	urlEncoded := "trace%2Fspan%3Fid%3D123"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, urlEncoded)

	var captured string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = chimiddleware.GetReqID(r.Context())
	}))

	h.ServeHTTP(rec, req)

	if captured != urlEncoded {
		t.Fatalf("expected URL-encoded ID to be preserved, got %q", captured)
	}
}

func TestRequestIDEmptyVsWhitespaceOnly(t *testing.T) {
	tests := []struct {
		name    string
		inputID string
		wantNew bool
	}{
		{"empty string", "", true},
		{"single space", " ", false},
		{"multiple spaces", "   ", false},
		{"tabs only", "\t\t", true},
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
				if captured == tc.inputID {
					t.Fatalf("expected new UUID, got original %q", captured)
				}
				_, err := uuid.Parse(captured)
				if err != nil {
					t.Fatalf("expected valid UUID, got %q: %v", captured, err)
				}
			} else {
				if captured != tc.inputID {
					t.Fatalf("expected %q, got %q", tc.inputID, captured)
				}
			}
		})
	}
}

func TestIsValidRequestIDLengthBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		length int
		valid  bool
	}{
		{"length 0", 0, false},
		{"length 1", 1, true},
		{"length 127", 127, true},
		{"length 128 (max)", 128, true},
		{"length 129 (over max)", 129, false},
		{"length 256", 256, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id := strings.Repeat("x", tc.length)
			got := isValidRequestID(id)
			if got != tc.valid {
				t.Errorf("isValidRequestID(len=%d) = %v, want %v", tc.length, got, tc.valid)
			}
		})
	}
}
