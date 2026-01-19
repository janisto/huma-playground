package firebase

import (
	"testing"
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
