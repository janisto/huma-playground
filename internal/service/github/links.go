package github

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/janisto/huma-playground/internal/platform/pagination"
)

type paginationKind uint8

const (
	noPagination paginationKind = iota
	numberedPagination
	activityPagination
)

type providerSpec struct {
	origin        *url.URL
	namedPath     string
	numericPrefix string
	numericSuffix string
	query         url.Values
	pagination    paginationKind
	currentValue  string
	scope         pagination.Scope
}

type navigation struct {
	nextCursor string
	prevCursor string
}

func parseNavigation(header http.Header, spec providerSpec, emptyPage bool) (navigation, error) {
	var result navigation
	seen := map[string]bool{"next": false, "prev": false}
	for _, field := range header.Values("Link") {
		for _, rawValue := range splitLinkValues(field) {
			target, relations, anchored, err := parseLinkValue(rawValue)
			if err != nil {
				return navigation{}, ErrUpstream
			}
			if anchored {
				continue
			}
			for _, relation := range relations {
				if relation != "next" && relation != "prev" {
					continue
				}
				if seen[relation] {
					return navigation{}, ErrUpstream
				}
				seen[relation] = true
				position, err := validateLinkTarget(target, relation, spec)
				if err != nil {
					return navigation{}, err
				}
				cursor := pagination.NewCursor(spec.scope, relation, position).Encode()
				if len(cursor) > pagination.MaxCursorLength {
					return navigation{}, ErrUpstream
				}
				if relation == "next" {
					result.nextCursor = cursor
				} else {
					result.prevCursor = cursor
				}
			}
		}
	}
	if emptyPage && result.nextCursor != "" {
		return navigation{}, ErrUpstream
	}
	return result, nil
}

func splitLinkValues(value string) []string {
	parts := make([]string, 0, 2)
	start, quoted, escaped, inTarget := 0, false, false, false
	for index := range len(value) {
		switch {
		case escaped:
			escaped = false
		case value[index] == '\\' && quoted:
			escaped = true
		case value[index] == '"':
			quoted = !quoted
		case value[index] == '<' && !quoted:
			inTarget = true
		case value[index] == '>' && !quoted:
			inTarget = false
		case value[index] == ',' && !quoted && !inTarget:
			parts = append(parts, strings.TrimSpace(value[start:index]))
			start = index + 1
		}
	}
	return append(parts, strings.TrimSpace(value[start:]))
}

func parseLinkValue(raw string) (string, []string, bool, error) {
	if raw == "" || raw[0] != '<' {
		return "", nil, false, ErrUpstream
	}
	closeIndex := strings.IndexByte(raw, '>')
	if closeIndex < 2 {
		return "", nil, false, ErrUpstream
	}
	target := raw[1:closeIndex]
	remainder := strings.TrimSpace(raw[closeIndex+1:])
	if remainder == "" || remainder[0] != ';' {
		return "", nil, false, ErrUpstream
	}
	parameters := splitLinkParameters(remainder[1:])
	var relations []string
	anchored := false
	seenParameters := make(map[string]struct{})
	for _, rawParameter := range parameters {
		name, rawValue, ok := strings.Cut(strings.TrimSpace(rawParameter), "=")
		name = strings.ToLower(strings.TrimSpace(name))
		if !ok || name == "" {
			return "", nil, false, ErrUpstream
		}
		if _, duplicate := seenParameters[name]; duplicate {
			return "", nil, false, ErrUpstream
		}
		seenParameters[name] = struct{}{}
		value, err := parseLinkParameterValue(strings.TrimSpace(rawValue))
		if err != nil {
			return "", nil, false, ErrUpstream
		}
		switch name {
		case "anchor":
			anchored = true
		case "rel":
			for relation := range strings.FieldsSeq(strings.ToLower(value)) {
				relations = append(relations, relation)
			}
		}
	}
	return target, relations, anchored, nil
}

func splitLinkParameters(value string) []string {
	parts := make([]string, 0, 3)
	start, quoted, escaped := 0, false, false
	for index := range len(value) {
		switch {
		case escaped:
			escaped = false
		case value[index] == '\\' && quoted:
			escaped = true
		case value[index] == '"':
			quoted = !quoted
		case value[index] == ';' && !quoted:
			parts = append(parts, value[start:index])
			start = index + 1
		}
	}
	return append(parts, value[start:])
}

func parseLinkParameterValue(value string) (string, error) {
	if value == "" {
		return "", ErrUpstream
	}
	if value[0] != '"' {
		if strings.ContainsAny(value, " \t,;") {
			return "", ErrUpstream
		}
		return value, nil
	}
	if len(value) < 2 || value[len(value)-1] != '"' {
		return "", ErrUpstream
	}
	var decoded strings.Builder
	escaped := false
	for index := 1; index < len(value)-1; index++ {
		character := value[index]
		if escaped {
			decoded.WriteByte(character)
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == '"' || character < 0x20 || character == 0x7f {
			return "", ErrUpstream
		}
		decoded.WriteByte(character)
	}
	if escaped {
		return "", ErrUpstream
	}
	return decoded.String(), nil
}

func validateLinkTarget(target, relation string, spec providerSpec) (string, error) {
	parsed, err := url.Parse(target)
	if err != nil || spec.origin == nil || parsed.Scheme != spec.origin.Scheme || parsed.Host != spec.origin.Host ||
		parsed.User != nil || parsed.Fragment != "" || parsed.ForceQuery ||
		parsed.Path != spec.namedPath && !matchesNumericPath(parsed.Path, spec.numericPrefix, spec.numericSuffix) {
		return "", ErrUpstream
	}
	if parsed.EscapedPath() != (&url.URL{Path: parsed.Path}).EscapedPath() {
		return "", ErrUpstream
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", ErrUpstream
	}
	for _, values := range query {
		if len(values) != 1 {
			return "", ErrUpstream
		}
	}
	expected := cloneQuery(spec.query)
	var position string
	switch spec.pagination {
	case numberedPagination:
		position = query.Get("page")
		if !canonicalSafeDecimal(position) || position == "0" {
			return "", ErrUpstream
		}
		expected.Set("page", position)
		current := uint64(1)
		if spec.currentValue != "" {
			current, _ = strconv.ParseUint(spec.currentValue, 10, 64)
		}
		targetPage, _ := strconv.ParseUint(position, 10, 64)
		if relation == "next" && targetPage <= current || relation == "prev" && targetPage >= current {
			return "", ErrUpstream
		}
	case activityPagination:
		expected.Del("after")
		expected.Del("before")
		member := "after"
		if relation == "prev" {
			member = "before"
		}
		position = query.Get(member)
		if !printableASCII(position, pagination.MaxCursorLength) || spec.currentValue == "" && relation == "prev" ||
			position == spec.currentValue {
			return "", ErrUpstream
		}
		expected.Set(member, position)
	default:
		return "", ErrUpstream
	}
	if query.Encode() != expected.Encode() {
		return "", ErrUpstream
	}
	return position, nil
}

func matchesNumericPath(path, prefix, suffix string) bool {
	if prefix == "" || !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return id != "" && !strings.Contains(id, "/") && canonicalSafeDecimal(id)
}

func canonicalSafeDecimal(value string) bool {
	if value == "" || value != "0" && value[0] == '0' {
		return false
	}
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return err == nil && parsed <= maximumSafeInteger
}

func printableASCII(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for index := range len(value) {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func cloneQuery(query url.Values) url.Values {
	cloned := make(url.Values, len(query))
	for name, values := range query {
		cloned[name] = append([]string(nil), values...)
	}
	return cloned
}
