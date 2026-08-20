package portable

import (
	"encoding/json"
	"io"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
	"github.com/fxamacker/cbor/v2"
)

var (
	strictCBORDecodeMode = mustCBORDecodeMode()
	strictCBOREncodeMode = mustCBOREncodeMode()
)

func mustCBORDecodeMode() cbor.DecMode {
	mode, err := (cbor.DecOptions{
		DupMapKey:        cbor.DupMapKeyEnforcedAPF,
		IndefLength:      cbor.IndefLengthForbidden,
		TagsMd:           cbor.TagsForbidden,
		IntDec:           cbor.IntDecConvertNone,
		DefaultMapType:   reflect.TypeFor[map[string]any](),
		UTF8:             cbor.UTF8RejectInvalid,
		MaxNestedLevels:  32,
		MaxArrayElements: 1024,
		MaxMapPairs:      1024,
	}).DecMode()
	if err != nil {
		panic(err)
	}
	return mode
}

func mustCBOREncodeMode() cbor.EncMode {
	mode, err := (cbor.EncOptions{
		Sort:          cbor.SortCanonical,
		ShortestFloat: cbor.ShortestFloat16,
		NaNConvert:    cbor.NaNConvertReject,
		InfConvert:    cbor.InfConvertReject,
		IndefLength:   cbor.IndefLengthForbidden,
		TagsMd:        cbor.TagsForbidden,
	}).EncMode()
	if err != nil {
		panic(err)
	}
	return mode
}

// Formats returns the only GCP portable wire codecs. JSON parameter aliases
// are runtime negotiation candidates, not additional OpenAPI media types.
func Formats() map[string]huma.Format {
	jsonFormat := huma.Format{
		Marshal: func(w io.Writer, value any) error {
			encoder := json.NewEncoder(w)
			encoder.SetEscapeHTML(false)
			return encoder.Encode(value)
		},
		Unmarshal: StrictJSONUnmarshal,
	}
	cborFormat := huma.Format{
		Marshal: func(w io.Writer, value any) error {
			return strictCBOREncodeMode.NewEncoder(w).Encode(value)
		},
		Unmarshal: strictCBORDecodeMode.Unmarshal,
	}
	return map[string]huma.Format{
		"application/json":                        jsonFormat,
		"application/json; charset=utf-8":         jsonFormat,
		"application/problem+json":                jsonFormat,
		"application/problem+json; charset=utf-8": jsonFormat,
		"application/cbor":                        cborFormat,
	}
}

func marshalCBOR(w io.Writer, value any) error {
	return strictCBOREncodeMode.NewEncoder(w).Encode(value)
}
