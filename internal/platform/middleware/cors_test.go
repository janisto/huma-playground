package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowsGETOrigin(t *testing.T) {
	called := false
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	h := CORS([]string{"*"})(fn)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/resource", nil)
	req.Header.Set("Origin", "http://example.com")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if !called {
		t.Fatalf("expected downstream handler to be called for GET request")
	}
	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin '*', got %q", got)
	}
	exposeHeaders := resp.Header().Get("Access-Control-Expose-Headers")
	if exposeHeaders == "" {
		t.Fatalf("expected Access-Control-Expose-Headers to be set")
	}
	for _, h := range []string{"Link", "Location", "Retry-After", "X-RateLimit-Reset", "X-Request-Id"} {
		if !containsHeader(exposeHeaders, h) {
			t.Fatalf("expected Access-Control-Expose-Headers to contain %q, got %q", h, exposeHeaders)
		}
	}
}

func containsHeader(headerValue, target string) bool {
	for part := range strings.SplitSeq(headerValue, ",") {
		if strings.EqualFold(strings.TrimSpace(part), target) {
			return true
		}
	}
	return false
}

func TestCORSHandlesPreflightWithoutCallingNext(t *testing.T) {
	called := false
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	h := CORS([]string{"*"})(fn)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "http://localhost/resource", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if called {
		t.Fatalf("expected preflight request to be handled by CORS middleware without calling downstream handler")
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 for preflight, got %d", resp.Code)
	}
	if got := resp.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Methods header to be set")
	}
	if got := resp.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Headers header to be set")
	}
	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin '*', got %q", got)
	}
}

func TestCORSAllowsXRequestIDHeader(t *testing.T) {
	fn := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := CORS([]string{"*"})(fn)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "http://localhost/resource", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "X-Request-ID")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 for preflight with X-Request-ID, got %d", resp.Code)
	}
	allowHeaders := resp.Header().Get("Access-Control-Allow-Headers")
	if !containsHeader(allowHeaders, "X-Request-Id") {
		t.Fatalf("expected Access-Control-Allow-Headers to contain X-Request-ID, got %q", allowHeaders)
	}
}

func TestCORSAllowsTraceContextHeaders(t *testing.T) {
	fn := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := CORS([]string{"*"})(fn)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "http://localhost/resource", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "traceparent, tracestate")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 for preflight with trace context, got %d", resp.Code)
	}
	allowHeaders := resp.Header().Get("Access-Control-Allow-Headers")
	for _, header := range []string{"traceparent", "tracestate"} {
		if !containsHeader(allowHeaders, header) {
			t.Fatalf("expected Access-Control-Allow-Headers to contain %s, got %q", header, allowHeaders)
		}
	}
}

func TestCORSRestrictsConfiguredOrigins(t *testing.T) {
	handler := CORS([]string{"https://example.com"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct {
		origin string
		want   string
	}{
		{origin: "https://example.com", want: "https://example.com"},
		{origin: "https://attacker.example", want: ""},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/resource", nil)
		request.Header.Set("Origin", test.origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.want {
			t.Fatalf("%s: expected %q, got %q", test.origin, test.want, got)
		}
		if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Fatalf("credentials unexpectedly enabled: %q", got)
		}
	}
}
