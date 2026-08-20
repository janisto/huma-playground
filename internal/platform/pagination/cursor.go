package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

const MaxCursorLength = 2048

// ErrInvalidCursor indicates the cursor is malformed or non-canonical.
var ErrInvalidCursor = errors.New("invalid cursor format")

// Scope is every result-shaping value to which a cursor is bound.
type Scope struct {
	Operation string
	Owner     string
	Repo      string
	Filter    string
	Limit     int
}

// Cursor is versioned portable pagination state. It is transport state, not a
// credential, and contains only validated public scope and provider values.
type Cursor struct {
	Version   int    `json:"v"`
	Operation string `json:"operation"`
	Owner     string `json:"owner,omitempty"`
	Repo      string `json:"repo,omitempty"`
	Limit     int    `json:"limit"`
	Filter    string `json:"filter,omitempty"`
	Direction string `json:"direction"`
	Anchor    string `json:"anchor,omitempty"`
	Upstream  string `json:"upstream,omitempty"`
}

func NewCursor(scope Scope, direction, position string) Cursor {
	return Cursor{
		Version: 1, Operation: scope.Operation, Owner: scope.Owner, Repo: scope.Repo,
		Limit: scope.Limit, Filter: scope.Filter, Direction: direction, Anchor: position,
	}
}

func (cursor Cursor) Matches(scope Scope) bool {
	return cursor.Operation == scope.Operation && cursor.Owner == scope.Owner && cursor.Repo == scope.Repo &&
		cursor.Filter == scope.Filter && cursor.Limit == scope.Limit
}

// Encode returns a canonical URL-safe opaque cursor.
func (cursor Cursor) Encode() string {
	data, err := json.Marshal(cursor)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeCursor validates canonical base64url and JSON encoding.
func DecodeCursor(value string) (Cursor, error) {
	if value == "" {
		return Cursor{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(data) != value {
		return Cursor{}, ErrInvalidCursor
	}
	var cursor Cursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.Encode() != value {
		return Cursor{}, ErrInvalidCursor
	}
	if cursor.Version != 1 || cursor.Operation == "" || cursor.Limit < 1 || cursor.Limit > 100 ||
		(cursor.Direction != "next" && cursor.Direction != "prev") {
		return Cursor{}, ErrInvalidCursor
	}
	return cursor, nil
}
