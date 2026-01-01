package pagination

import (
	"encoding/base64"
	"errors"
	"strings"
)

// ErrInvalidCursor indicates the cursor could not be decoded.
var ErrInvalidCursor = errors.New("invalid cursor format")

// Cursor represents a pagination position.
type Cursor struct {
	Type  string // resource type identifier
	Value string // last seen value (ID, timestamp, etc.)
}

// Encode returns a URL-safe opaque Base64 representation.
func (c Cursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.Type + ":" + c.Value),
	)
}

// DecodeCursor parses a URL-safe Base64 cursor string.
func DecodeCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		return Cursor{}, ErrInvalidCursor
	}
	return Cursor{Type: parts[0], Value: parts[1]}, nil
}
