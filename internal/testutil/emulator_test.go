package testutil

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestEmulatorAvailable(t *testing.T) {
	if emulatorAvailable("127.0.0.1:0") {
		t.Fatal("expected unreachable endpoint")
	}

	server := httptest.NewServer(nil)
	t.Cleanup(server.Close)
	if !emulatorAvailable(server.Listener.Addr().String()) {
		t.Fatal("expected reachable endpoint")
	}
}

func TestSetupEmulator(t *testing.T) {
	SetupEmulator(t)
	if got := os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"); got != AuthEmulatorHost {
		t.Fatalf("Auth emulator host = %q, want %q", got, AuthEmulatorHost)
	}
	if got := os.Getenv("FIRESTORE_EMULATOR_HOST"); got != FirestoreEmulatorHost {
		t.Fatalf("Firestore emulator host = %q, want %q", got, FirestoreEmulatorHost)
	}
}

func TestUnexpectedStatusError(t *testing.T) {
	tests := []struct {
		name       string
		response   *http.Response
		wantStatus int
		want       string
		wantPrefix string
		wantLength int
	}{
		{
			name:       "expected status",
			response:   &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "unexpected status with bounded body",
			response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 9<<10))),
			},
			wantStatus: http.StatusOK,
			wantPrefix: "status 500: " + strings.Repeat("x", 16),
			wantLength: len("status 500: ") + 8<<10,
		},
		{
			name: "response read failure",
			response: &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(errorReader{}),
			},
			wantStatus: http.StatusOK,
			want:       "status 502; read response: read failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := unexpectedStatusError(test.response, test.wantStatus)
			if test.want == "" && test.wantPrefix == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if test.want != "" && err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if test.wantPrefix != "" &&
				(!strings.HasPrefix(err.Error(), test.wantPrefix) || len(err.Error()) != test.wantLength) {
				t.Fatalf("error prefix or length = %q, %d; want %q, %d",
					err.Error()[:min(len(err.Error()), len(test.wantPrefix))], len(err.Error()),
					test.wantPrefix, test.wantLength)
			}
		})
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}
