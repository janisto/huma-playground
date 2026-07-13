package timeutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
)

const RFC3339Millis = "2006-01-02T15:04:05.000Z"

// Time serializes time.Time in UTC with fixed millisecond precision.
type Time struct {
	time.Time
}

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.UTC().Format(RFC3339Millis))
}

func (t *Time) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	return t.parse(value)
}

func (t Time) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  0,
		Content: t.UTC().Format(RFC3339Millis),
	})
}

func (t *Time) UnmarshalCBOR(data []byte) error {
	if len(data) == 0 {
		return errors.New("timeutil: empty CBOR data")
	}
	var value any
	if err := cbor.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("timeutil: decode CBOR: %w", err)
	}
	switch typed := value.(type) {
	case time.Time:
		t.Time = typed
		return nil
	case cbor.Tag:
		if typed.Number != 0 {
			return fmt.Errorf("timeutil: expected date/time tag 0, got %d", typed.Number)
		}
		text, ok := typed.Content.(string)
		if !ok {
			return errors.New("timeutil: expected tagged text string")
		}
		return t.parse(text)
	case string:
		return t.parse(typed)
	default:
		return errors.New("timeutil: expected CBOR text string")
	}
}

func (t *Time) parse(value string) error {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

func NewTime(t time.Time) Time {
	return Time{Time: t}
}
