package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	AuthEmulatorHost      = "127.0.0.1:7110"
	FirestoreEmulatorHost = "127.0.0.1:7130"
	ProjectID             = "demo-test-project"
	fakeAPIKey            = "fake-api-key" //nolint:gosec // Test-only fake key for emulator
)

var emulatorHTTPClient = &http.Client{Timeout: 5 * time.Second}

// EmulatorAvailable checks if the Firebase emulators (Auth + Firestore) are reachable.
func EmulatorAvailable() bool {
	return emulatorAvailable(AuthEmulatorHost) && emulatorAvailable(FirestoreEmulatorHost)
}

func emulatorAvailable(host string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// SkipIfEmulatorUnavailable skips the test if the Firebase emulators are not running.
func SkipIfEmulatorUnavailable(t *testing.T) {
	t.Helper()
	if !EmulatorAvailable() {
		if os.Getenv("REQUIRE_FIREBASE_EMULATORS") == "1" {
			t.Fatal("Firebase emulators are required but unavailable")
		}
		t.Skip("Firebase emulators not available")
	}
}

// SetupEmulator configures the environment for emulator testing.
func SetupEmulator(t *testing.T) {
	t.Helper()
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", AuthEmulatorHost)
	t.Setenv("FIRESTORE_EMULATOR_HOST", FirestoreEmulatorHost)
}

// ClearAccounts removes all users from the Auth emulator.
func ClearAccounts(t *testing.T) {
	t.Helper()
	url := fmt.Sprintf("http://%s/emulator/v1/projects/%s/accounts", AuthEmulatorHost, ProjectID)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	doEmulatorRequest(t, req, http.StatusOK)
}

// ClearFirestore removes all documents from the Firestore emulator.
func ClearFirestore(t *testing.T) {
	t.Helper()
	url := fmt.Sprintf("http://%s/emulator/v1/projects/%s/databases/(default)/documents",
		FirestoreEmulatorHost, ProjectID)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	doEmulatorRequest(t, req, http.StatusOK)
}

// SignUpResponse from the emulator.
type SignUpResponse struct {
	IDToken      string `json:"idToken"`
	RefreshToken string `json:"refreshToken"`
	LocalID      string `json:"localId"`
	Email        string `json:"email"`
}

// CreateTestUser creates a user in the emulator and returns the ID token.
func CreateTestUser(t *testing.T, email, password string) *SignUpResponse {
	t.Helper()
	url := fmt.Sprintf("http://%s/identitytoolkit.googleapis.com/v1/accounts:signUp?key=%s",
		AuthEmulatorHost, fakeAPIKey)

	body, err := json.Marshal(map[string]any{
		"email":             email,
		"password":          password,
		"returnSecureToken": true,
	})
	if err != nil {
		t.Fatalf("encode sign-up request: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := emulatorHTTPClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := unexpectedStatusError(resp, http.StatusOK); err != nil {
		t.Fatalf("create test user: %v", err)
	}

	var result SignUpResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.IDToken == "" || result.LocalID == "" {
		t.Fatalf(
			"create test user returned incomplete identity: token=%t local_id=%q",
			result.IDToken != "",
			result.LocalID,
		)
	}
	return &result
}

func doEmulatorRequest(t *testing.T, req *http.Request, expectedStatus int) {
	t.Helper()
	resp, err := emulatorHTTPClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := unexpectedStatusError(resp, expectedStatus); err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
}

func unexpectedStatusError(resp *http.Response, expectedStatus int) error {
	if resp.StatusCode == expectedStatus {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if err != nil {
		return fmt.Errorf("status %d; read response: %w", resp.StatusCode, err)
	}
	return fmt.Errorf("status %d: %s", resp.StatusCode, data)
}
