package portable

import (
	"encoding/json"
	"errors"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	maxJSONNestedLevels   = 32
	maxJSONContainerItems = 1024
)

var errJSONContainerCardinality = errors.New("JSON container cardinality exceeds limit")

// ParseStrictJSON parses exactly one RFC 8259 value while rejecting duplicate
// object names, invalid UTF-8, a BOM, lone surrogates, and trailing content.
func ParseStrictJSON(data []byte) (any, error) {
	return parseStrictJSON(data, 0)
}

func parseStrictJSON(data []byte, maxContainerItems int) (any, error) {
	if len(data) == 0 {
		return nil, errors.New("empty JSON document")
	}
	if !utf8.Valid(data) {
		return nil, errors.New("invalid UTF-8")
	}
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return nil, errors.New("JSON byte-order mark is not supported")
	}
	parser := jsonParser{data: data, maxContainerItems: maxContainerItems}
	value, err := parser.parseValueAfterSpace(0, true)
	if err != nil {
		return nil, err
	}
	parser.skipSpace()
	if parser.offset != len(data) {
		return nil, errors.New("trailing JSON content")
	}
	if parser.containerLimitExceeded {
		return nil, errJSONContainerCardinality
	}
	return value, nil
}

// StrictJSONUnmarshal is a Huma format decoder backed by ParseStrictJSON.
func StrictJSONUnmarshal(data []byte, target any) error {
	value, err := parseStrictJSON(data, maxJSONContainerItems)
	if errors.Is(err, errJSONContainerCardinality) {
		if generic, ok := target.(*any); ok {
			// Huma first decodes into any for schema validation. A container over
			// this bound cannot match any accepted inbound schema, so preserve the
			// required 422 boundary without retaining attacker-controlled entries.
			*generic = nil
			return nil
		}
	}
	if err != nil {
		return err
	}
	if generic, ok := target.(*any); ok {
		*generic = value
		return nil
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(normalized, target)
}

type jsonParser struct {
	data                   []byte
	offset                 int
	maxContainerItems      int
	containerLimitExceeded bool
}

func (p *jsonParser) skipSpace() {
	for p.offset < len(p.data) {
		switch p.data[p.offset] {
		case ' ', '\t', '\n', '\r':
			p.offset++
		default:
			return
		}
	}
}

func (p *jsonParser) parseValueAfterSpace(depth int, materialize bool) (any, error) {
	p.skipSpace()
	if p.offset >= len(p.data) {
		return nil, errors.New("missing JSON value")
	}
	switch p.data[p.offset] {
	case '{':
		if depth >= maxJSONNestedLevels {
			return nil, errors.New("JSON nesting exceeds limit")
		}
		return p.parseObject(depth+1, materialize)
	case '[':
		if depth >= maxJSONNestedLevels {
			return nil, errors.New("JSON nesting exceeds limit")
		}
		return p.parseArray(depth+1, materialize)
	case '"':
		value, err := p.parseString(materialize)
		return value, err
	case 't':
		return p.parseLiteral("true", materialize, true)
	case 'f':
		return p.parseLiteral("false", materialize, false)
	case 'n':
		return p.parseLiteral("null", materialize, nil)
	default:
		return p.parseNumber(materialize)
	}
}

func (p *jsonParser) parseObject(depth int, materialize bool) (map[string]any, error) {
	p.offset++
	var result map[string]any
	if materialize {
		result = make(map[string]any)
	}
	var seen map[string]struct{}
	if p.maxContainerItems == 0 || materialize {
		seen = make(map[string]struct{})
	}
	members := 0
	p.skipSpace()
	if p.consume('}') {
		return result, nil
	}
	for {
		if p.offset >= len(p.data) || p.data[p.offset] != '"' {
			return nil, errors.New("JSON object name must be a string")
		}
		members++
		withinLimit := p.maxContainerItems == 0 || members <= p.maxContainerItems
		// Once a bounded inbound container exceeds the accepted cardinality,
		// validate syntax without retaining any additional attacker-controlled
		// names. The cardinality failure controls later independent defects.
		trackName := p.maxContainerItems == 0 || materialize && withinLimit
		name, err := p.parseString(trackName)
		if err != nil {
			return nil, err
		}
		if trackName {
			if _, exists := seen[name]; exists {
				return nil, errors.New("duplicate JSON object name")
			}
			seen[name] = struct{}{}
		}
		if !withinLimit {
			p.containerLimitExceeded = true
		}
		p.skipSpace()
		if !p.consume(':') {
			return nil, errors.New("missing JSON object separator")
		}
		store := materialize && withinLimit
		value, err := p.parseValueAfterSpace(depth, store)
		if err != nil {
			return nil, err
		}
		if store {
			result[name] = value
		}
		p.skipSpace()
		if p.consume('}') {
			return result, nil
		}
		if !p.consume(',') {
			return nil, errors.New("missing JSON object delimiter")
		}
		p.skipSpace()
	}
}

func (p *jsonParser) parseArray(depth int, materialize bool) ([]any, error) {
	p.offset++
	var result []any
	if materialize {
		result = make([]any, 0)
	}
	elements := 0
	p.skipSpace()
	if p.consume(']') {
		return result, nil
	}
	for {
		elements++
		withinLimit := p.maxContainerItems == 0 || elements <= p.maxContainerItems
		if !withinLimit {
			p.containerLimitExceeded = true
		}
		store := materialize && withinLimit
		value, err := p.parseValueAfterSpace(depth, store)
		if err != nil {
			return nil, err
		}
		if store {
			result = append(result, value)
		}
		p.skipSpace()
		if p.consume(']') {
			return result, nil
		}
		if !p.consume(',') {
			return nil, errors.New("missing JSON array delimiter")
		}
		p.skipSpace()
	}
}

func (p *jsonParser) parseString(materialize bool) (string, error) {
	start := p.offset
	p.offset++
	for p.offset < len(p.data) {
		current := p.data[p.offset]
		switch {
		case current == '"':
			p.offset++
			if !materialize {
				return "", nil
			}
			var value string
			if err := json.Unmarshal(p.data[start:p.offset], &value); err != nil {
				return "", err
			}
			return value, nil
		case current < 0x20:
			return "", errors.New("unescaped JSON control character")
		case current == '\\':
			if err := p.validateEscape(); err != nil {
				return "", err
			}
		default:
			_, size := utf8.DecodeRune(p.data[p.offset:])
			p.offset += size
		}
	}
	return "", errors.New("unterminated JSON string")
}

func (p *jsonParser) validateEscape() error {
	p.offset++
	if p.offset >= len(p.data) {
		return errors.New("unterminated JSON escape")
	}
	switch p.data[p.offset] {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		p.offset++
		return nil
	case 'u':
		first, err := p.readUnicodeEscape()
		if err != nil {
			return err
		}
		if first >= 0xdc00 && first <= 0xdfff {
			return errors.New("lone low surrogate")
		}
		if first < 0xd800 || first > 0xdbff {
			return nil
		}
		if p.offset+2 > len(p.data) || p.data[p.offset] != '\\' || p.data[p.offset+1] != 'u' {
			return errors.New("lone high surrogate")
		}
		p.offset++
		second, err := p.readUnicodeEscape()
		if err != nil || second < 0xdc00 || second > 0xdfff || !utf16.IsSurrogate(rune(second)) {
			return errors.New("invalid surrogate pair")
		}
		return nil
	default:
		return errors.New("invalid JSON escape")
	}
}

func (p *jsonParser) readUnicodeEscape() (uint16, error) {
	if p.offset >= len(p.data) || p.data[p.offset] != 'u' || p.offset+5 > len(p.data) {
		return 0, errors.New("invalid Unicode escape")
	}
	var value uint16
	for _, digit := range p.data[p.offset+1 : p.offset+5] {
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value |= uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value |= uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value |= uint16(digit-'A') + 10
		default:
			return 0, errors.New("invalid Unicode escape")
		}
	}
	p.offset += 5
	return value, nil
}

func (p *jsonParser) parseLiteral(text string, materialize bool, value any) (any, error) {
	if p.offset+len(text) > len(p.data) || string(p.data[p.offset:p.offset+len(text)]) != text {
		return nil, errors.New("invalid JSON literal")
	}
	p.offset += len(text)
	if !materialize {
		return nil, nil
	}
	return value, nil
}

func (p *jsonParser) parseNumber(materialize bool) (any, error) {
	start := p.offset
	if p.consume('-') && p.offset >= len(p.data) {
		return nil, errors.New("invalid JSON number")
	}
	if p.consume('0') {
		if p.offset < len(p.data) && p.data[p.offset] >= '0' && p.data[p.offset] <= '9' {
			return nil, errors.New("invalid leading zero in JSON number")
		}
	} else {
		if p.offset >= len(p.data) || p.data[p.offset] < '1' || p.data[p.offset] > '9' {
			return nil, errors.New("invalid JSON value")
		}
		for p.offset < len(p.data) && p.data[p.offset] >= '0' && p.data[p.offset] <= '9' {
			p.offset++
		}
	}
	if p.consume('.') {
		fractionStart := p.offset
		for p.offset < len(p.data) && p.data[p.offset] >= '0' && p.data[p.offset] <= '9' {
			p.offset++
		}
		if fractionStart == p.offset {
			return nil, errors.New("invalid JSON fraction")
		}
	}
	if p.offset < len(p.data) && (p.data[p.offset] == 'e' || p.data[p.offset] == 'E') {
		p.offset++
		if p.offset < len(p.data) && (p.data[p.offset] == '+' || p.data[p.offset] == '-') {
			p.offset++
		}
		exponentStart := p.offset
		for p.offset < len(p.data) && p.data[p.offset] >= '0' && p.data[p.offset] <= '9' {
			p.offset++
		}
		if exponentStart == p.offset {
			return nil, errors.New("invalid JSON exponent")
		}
	}
	if !materialize {
		return nil, nil
	}
	return json.Number(string(p.data[start:p.offset])), nil
}

func (p *jsonParser) consume(expected byte) bool {
	if p.offset < len(p.data) && p.data[p.offset] == expected {
		p.offset++
		return true
	}
	return false
}
