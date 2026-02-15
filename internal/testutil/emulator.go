package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

const (
	AuthEmulatorHost      = "127.0.0.1:7110"
	FirestoreEmulatorHost = "127.0.0.1:7130"
	ProjectID             = "demo-test-project"
	fakeAPIKey            = "fake-api-key" //nolint:gosec // Test-only fake key for emulator
)

// EmulatorAvailable checks if the Firebase emulators (Auth + Firestore) are reachable.
func EmulatorAvailable() bool {
	return emulatorAvailable(AuthEmulatorHost) && emulatorAvailable(FirestoreEmulatorHost)
}

func emulatorAvailable(host string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
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
	ctx := context.Background()
	url := fmt.Sprintf("http://%s/emulator/v1/projects/%s/accounts", AuthEmulatorHost, ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to clear accounts: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
}

// ClearFirestore removes all documents from the Firestore emulator.
func ClearFirestore(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	url := fmt.Sprintf("http://%s/emulator/v1/projects/%s/databases/(default)/documents",
		FirestoreEmulatorHost, ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to clear Firestore: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
}

// ClearEmulators clears both Auth accounts and Firestore documents.
func ClearEmulators(t *testing.T) {
	t.Helper()
	ClearAccounts(t)
	ClearFirestore(t)
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
	ctx := context.Background()
	url := fmt.Sprintf("http://%s/identitytoolkit.googleapis.com/v1/accounts:signUp?key=%s",
		AuthEmulatorHost, fakeAPIKey)

	body, _ := json.Marshal(map[string]any{
		"email":             email,
		"password":          password,
		"returnSecureToken": true,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result SignUpResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return &result
}
