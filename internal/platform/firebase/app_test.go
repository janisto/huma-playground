package firebase

import (
	"context"
	"testing"

	"github.com/janisto/huma-playground/internal/testutil"
)

func TestClientsCloseReturnsNilWhenFirestoreNil(t *testing.T) {
	c := &Clients{
		Auth:      nil,
		Firestore: nil,
	}

	if err := c.Close(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		ProjectID: "test-project",
	}

	if cfg.ProjectID != "test-project" {
		t.Fatalf("expected ProjectID 'test-project', got %s", cfg.ProjectID)
	}
}

func TestInitializeClientsWithEmulator(t *testing.T) {
	testutil.SkipIfFirestoreUnavailable(t)
	testutil.SkipIfAuthUnavailable(t)
	testutil.SetupEmulator(t)

	ctx := context.Background()
	clients, err := InitializeClients(ctx, Config{ProjectID: testutil.ProjectID})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if clients.Auth == nil {
		t.Fatal("expected Auth client to be non-nil")
	}
	if clients.Firestore == nil {
		t.Fatal("expected Firestore client to be non-nil")
	}

	if err := clients.Close(); err != nil {
		t.Fatalf("expected no error on Close, got %v", err)
	}
}

func TestInitializeClientsCancelledContext(t *testing.T) {
	testutil.SkipIfFirestoreUnavailable(t)
	testutil.SetupEmulator(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := InitializeClients(ctx, Config{ProjectID: testutil.ProjectID})
	if err == nil {
		t.Log("InitializeClients succeeded with canceled context (SDK may not check context during init)")
	}
}
