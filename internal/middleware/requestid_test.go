package middleware

import (
	"net/http"
	"net/http/httptest"
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
