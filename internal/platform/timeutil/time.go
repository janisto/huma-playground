package timeutil

import (
	"errors"
	"time"
)

// RFC3339Millis is RFC 3339 UTC with fixed millisecond precision.
// Use this format for consistent timestamp output across the API.
const RFC3339Millis = "2006-01-02T15:04:05.000Z"

// RFC3339Micros is RFC 3339 UTC with fixed microsecond precision.
// Use this format for log timestamps where higher precision is needed.
const RFC3339Micros = "2006-01-02T15:04:05.000000Z"

// Time wraps time.Time to ensure consistent RFC 3339 millisecond precision
// in JSON and CBOR marshaling. Output format is always "2024-01-15T10:30:00.000Z".
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

// MarshalCBOR implements cbor.Marshaler with fixed millisecond precision.
// Encodes as CBOR tag 0 (standard date/time string per RFC 8949 section 3.4.1).
func (t Time) MarshalCBOR() ([]byte, error) {
	s := t.UTC().Format(RFC3339Millis)
	data := make([]byte, 0, 2+len(s))
	data = append(data, 0xc0) // tag 0
	data = appendCBORTextString(data, s)
	return data, nil
}

// UnmarshalCBOR implements cbor.Unmarshaler, accepting CBOR tag 0 date/time
// strings and bare text strings.
func (t *Time) UnmarshalCBOR(data []byte) error {
	if len(data) == 0 {
		return errors.New("timeutil: empty CBOR data")
	}
	// Strip optional tag 0 (0xc0).
	if data[0] == 0xc0 {
		data = data[1:]
	}
	s, err := decodeCBORTextString(data)
	if err != nil {
		return err
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

// appendCBORTextString appends a CBOR text string (major type 3) to dst.
func appendCBORTextString(dst []byte, s string) []byte {
	n := len(s)
	switch {
	case n <= 23:
		dst = append(dst, 0x60+byte(n))
	case n <= 0xff:
		dst = append(dst, 0x78, byte(n))
	default:
		dst = append(dst, 0x79, byte(n>>8), byte(n))
	}
	return append(dst, s...)
}

// decodeCBORTextString decodes a CBOR text string (major type 3).
func decodeCBORTextString(data []byte) (string, error) {
	if len(data) == 0 {
		return "", errors.New("timeutil: empty CBOR text string")
	}
	major := data[0] & 0xe0
	if major != 0x60 {
		return "", errors.New("timeutil: expected CBOR text string")
	}
	info := data[0] & 0x1f
	var offset, length int
	switch {
	case info <= 23:
		length = int(info)
		offset = 1
	case info == 24:
		if len(data) < 2 {
			return "", errors.New("timeutil: truncated CBOR length")
		}
		length = int(data[1])
		offset = 2
	case info == 25:
		if len(data) < 3 {
			return "", errors.New("timeutil: truncated CBOR length")
		}
		length = int(data[1])<<8 | int(data[2])
		offset = 3
	default:
		return "", errors.New("timeutil: unsupported CBOR text string length encoding")
	}
	if len(data) < offset+length {
		return "", errors.New("timeutil: truncated CBOR text string")
	}
	return string(data[offset : offset+length]), nil //nolint:gosec // G602 false positive: bounds checked above
}

// NewTime creates a Time from a standard time.Time.
func NewTime(t time.Time) Time {
	return Time{Time: t}
}

// Now returns the current time as a Time.
func Now() Time {
	return Time{Time: time.Now()}
}
