package firebase

import (
	"context"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

// Config holds Firebase configuration.
// Credentials are loaded via Application Default Credentials (ADC).
// Set GOOGLE_APPLICATION_CREDENTIALS env var to specify a service account key file.
type Config struct {
	ProjectID string
}

// Clients holds initialized Firebase clients.
type Clients struct {
	Auth      *auth.Client
	Firestore *firestore.Client
}

// InitializeClients sets up Firebase and returns clients directly.
// Credentials are loaded via Application Default Credentials (ADC).
// Prefer this over Initialize() + global getters for better testability.
func InitializeClients(ctx context.Context, cfg Config) (*Clients, error) {
	config := &firebase.Config{ProjectID: cfg.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		return nil, err
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		return nil, err
	}

	fc, err := fbApp.Firestore(ctx)
	if err != nil {
		return nil, err
	}

	return &Clients{
		Auth:      ac,
		Firestore: fc,
	}, nil
}

// Close closes the Firestore client.
func (c *Clients) Close() error {
	if c.Firestore != nil {
		return c.Firestore.Close()
	}
	return nil
}
