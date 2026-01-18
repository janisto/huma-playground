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
		ProjectID:                    "test-project",
		GoogleApplicationCredentials: "/path/to/creds.json",
	}

	if cfg.ProjectID != "test-project" {
		t.Fatalf("expected ProjectID 'test-project', got %s", cfg.ProjectID)
	}
	if cfg.GoogleApplicationCredentials != "/path/to/creds.json" {
		t.Fatalf("expected credentials path '/path/to/creds.json', got %s", cfg.GoogleApplicationCredentials)
	}
}
