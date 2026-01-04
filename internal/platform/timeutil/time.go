package timeutil

import (
	"time"
)

// RFC3339Millis is RFC 3339 UTC with fixed millisecond precision.
// Use this format for consistent timestamp output across the API.
const RFC3339Millis = "2006-01-02T15:04:05.000Z"

// RFC3339Micros is RFC 3339 UTC with fixed microsecond precision.
// Use this format for log timestamps where higher precision is needed.
const RFC3339Micros = "2006-01-02T15:04:05.000000Z"

// Time wraps time.Time to ensure consistent RFC 3339 millisecond precision
// in JSON marshaling. Output format is always "2024-01-15T10:30:00.000Z".
//
// Null handling: When unmarshaling JSON null, the existing value is preserved
// (not zeroed). This matches the behavior of the standard library's time.Time.
type Time struct {
	time.Time
}

// MarshalJSON implements json.Marshaler with fixed millisecond precision.
func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.UTC().Format(RFC3339Millis) + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler, accepting RFC 3339 variants.
// JSON null preserves the existing value, matching time.Time stdlib behavior.
func (t *Time) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return err
		}
	}
	t.Time = parsed
	return nil
}

// NewTime creates a Time from a standard time.Time.
func NewTime(t time.Time) Time {
	return Time{Time: t}
}

// Now returns the current time as a Time.
func Now() Time {
	return Time{Time: time.Now()}
}
