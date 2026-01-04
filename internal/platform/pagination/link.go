package pagination

import (
	"fmt"
	"net/url"
	"strings"
)

// BuildLinkHeader constructs RFC 8288 Link header, preserving existing query params.
func BuildLinkHeader(baseURL string, query url.Values, nextCursor, prevCursor string) string {
	var links []string
	if nextCursor != "" {
		q := cloneValues(query)
		q.Set("cursor", nextCursor)
		links = append(links, fmt.Sprintf("<%s?%s>; rel=\"next\"", baseURL, q.Encode()))
	}
	if prevCursor != "" {
		q := cloneValues(query)
		q.Set("cursor", prevCursor)
		links = append(links, fmt.Sprintf("<%s?%s>; rel=\"prev\"", baseURL, q.Encode()))
	}
	return strings.Join(links, ", ")
}

func cloneValues(v url.Values) url.Values {
	if v == nil {
		return make(url.Values)
	}
	out := make(url.Values, len(v))
	for k, vals := range v {
		out[k] = append([]string(nil), vals...)
	}
	return out
}
