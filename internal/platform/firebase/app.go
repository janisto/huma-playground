package firebase

import (
	"context"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// Config holds Firebase configuration.
type Config struct {
	ProjectID                    string
	GoogleApplicationCredentials string // Path to service account JSON (optional)
}

// Clients holds initialized Firebase clients.
type Clients struct {
	Auth      *auth.Client
	Firestore *firestore.Client
}

// InitializeClients sets up Firebase and returns clients directly.
// Prefer this over Initialize() + global getters for better testability.
func InitializeClients(ctx context.Context, cfg Config) (*Clients, error) {
	var opts []option.ClientOption
	if cfg.GoogleApplicationCredentials != "" {
		creds, err := os.ReadFile(cfg.GoogleApplicationCredentials)
		if err != nil {
			return nil, err
		}
		opts = append(opts, option.WithCredentialsJSON(creds))
	}

	config := &firebase.Config{ProjectID: cfg.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config, opts...)
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
