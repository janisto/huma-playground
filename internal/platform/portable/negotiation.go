package portable

import (
	"net/http"
	"strconv"
	"strings"
)

const (
	MediaTypeJSON        = "application/json"
	MediaTypeJSONUTF8    = "application/json; charset=utf-8"
	MediaTypeCBOR        = "application/cbor"
	MediaTypeProblemJSON = "application/problem+json"
)

type acceptMatch struct {
	parameters  int
	quality     float64
	specificity int
}

// AcceptHeader combines repeated Accept field lines in received order.
func AcceptHeader(header http.Header) string {
	return strings.Join(header.Values("Accept"), ",")
}

// NegotiateSuccess selects the GCP success representation. CBOR is selected
// only by an exact media range; wildcards deterministically select JSON.
func NegotiateSuccess(accept string, jsonOnly bool) (string, bool) {
	if accept == "" {
		return MediaTypeJSON, true
	}
	candidates := []struct {
		media        string
		withCharset  bool
		explicitOnly bool
	}{
		{MediaTypeJSON, false, false},
		{MediaTypeJSON, true, false},
	}
	if !jsonOnly {
		candidates = append(candidates, struct {
			media        string
			withCharset  bool
			explicitOnly bool
		}{MediaTypeCBOR, false, true})
	}
	selected := ""
	selectedQuality := float64(0)
	for _, candidate := range candidates {
		quality, ok := mediaTypeQuality(accept, candidate.media, candidate.explicitOnly, candidate.withCharset)
		if ok && quality > selectedQuality {
			selectedQuality = quality
			if candidate.withCharset {
				selected = MediaTypeJSONUTF8
			} else {
				selected = candidate.media
			}
		}
	}
	return selected, selected != ""
}

// NegotiateProblem selects the GCP Problem Details representation. It falls
// back to JSON when the request has no acceptable error representation.
func NegotiateProblem(accept string) string {
	baseQuality, _ := mediaTypeQuality(accept, MediaTypeProblemJSON, false, false)
	exactBaseQuality, exactBase := mediaTypeQuality(accept, MediaTypeProblemJSON, true, false)
	if exactBase {
		baseQuality = exactBaseQuality
	}
	utf8Quality, _ := mediaTypeQuality(accept, MediaTypeProblemJSON, false, true)
	exactUTF8Quality, exactUTF8 := mediaTypeQuality(accept, MediaTypeProblemJSON, true, true)
	if exactUTF8 {
		utf8Quality = exactUTF8Quality
	}
	cborQuality, _ := mediaTypeQuality(accept, MediaTypeCBOR, true, false)
	jsonQuality := max(baseQuality, utf8Quality)
	if cborQuality > jsonQuality && cborQuality > 0 {
		return MediaTypeCBOR
	}
	if utf8Quality > baseQuality && utf8Quality > 0 {
		return MediaTypeProblemJSON + "; charset=utf-8"
	}
	return MediaTypeProblemJSON
}

func mediaTypeQuality(accept, mediaType string, explicitOnly, withCharset bool) (float64, bool) {
	bestParameters := -1
	bestSpecificity := -1
	bestQuality := float64(0)
	for _, rawRange := range splitOutsideQuotes(accept, ',') {
		match, ok := matchMediaRange(rawRange, mediaType, explicitOnly, withCharset)
		if !ok {
			continue
		}
		if match.specificity > bestSpecificity ||
			(match.specificity == bestSpecificity && match.parameters > bestParameters) {
			bestSpecificity = match.specificity
			bestParameters = match.parameters
			bestQuality = match.quality
		} else if match.specificity == bestSpecificity && match.parameters == bestParameters {
			bestQuality = max(bestQuality, match.quality)
		}
	}
	return bestQuality, bestSpecificity >= 0
}

func matchMediaRange(rawRange, target string, explicitOnly, withCharset bool) (acceptMatch, bool) {
	parts, valid := splitSemicolonFields(rawRange)
	if !valid || len(parts) == 0 {
		return acceptMatch{}, false
	}
	rangeValue := strings.ToLower(strings.TrimSpace(parts[0]))
	specificity := rangeSpecificity(rangeValue, target)
	if specificity < 0 || (explicitOnly && specificity < 2) {
		return acceptMatch{}, false
	}
	quality := float64(1)
	qualitySeen := false
	mediaParameters := make(map[string]struct{})
	for _, rawParameter := range parts[1:] {
		parameter := strings.TrimSpace(rawParameter)
		name, value, found := strings.Cut(parameter, "=")
		name = strings.ToLower(strings.TrimSpace(name))
		if !isHTTPToken(name) {
			return acceptMatch{}, false
		}
		if name == "q" {
			if qualitySeen || !found {
				return acceptMatch{}, false
			}
			parsed, ok := parseQuality(strings.TrimSpace(value))
			if !ok {
				return acceptMatch{}, false
			}
			quality = parsed
			qualitySeen = true
			continue
		}
		decoded, ok := decodeParameterValue(strings.TrimSpace(value), found)
		if !ok {
			return acceptMatch{}, false
		}
		if qualitySeen {
			continue
		}
		if name != "charset" || !strings.HasSuffix(target, "+json") && target != MediaTypeJSON ||
			!strings.EqualFold(decoded, "utf-8") {
			return acceptMatch{}, false
		}
		if _, duplicate := mediaParameters[name]; duplicate {
			return acceptMatch{}, false
		}
		mediaParameters[name] = struct{}{}
	}
	if withCharset != (len(mediaParameters) == 1) {
		return acceptMatch{}, false
	}
	return acceptMatch{parameters: len(mediaParameters), quality: quality, specificity: specificity}, true
}

func rangeSpecificity(value, target string) int {
	if value == target {
		return 2
	}
	if value == "*/*" {
		return 0
	}
	targetType, _, _ := strings.Cut(target, "/")
	if value == targetType+"/*" {
		return 1
	}
	return -1
}

func splitOutsideQuotes(value string, delimiter byte) []string {
	result := make([]string, 0, strings.Count(value, string(delimiter))+1)
	start := 0
	quoted := false
	escaped := false
	for index := range len(value) {
		current := value[index]
		if quoted {
			switch {
			case escaped:
				escaped = false
			case current == '\\':
				escaped = true
			case current == '"':
				quoted = false
			}
		} else if current == '"' {
			quoted = true
		} else if current == delimiter {
			result = append(result, value[start:index])
			start = index + 1
		}
	}
	return append(result, value[start:])
}

func splitSemicolonFields(value string) ([]string, bool) {
	parts := splitOutsideQuotes(value, ';')
	quoted := false
	escaped := false
	for index := range len(value) {
		current := value[index]
		if quoted {
			switch {
			case escaped:
				escaped = false
			case current == '\\':
				escaped = true
			case current == '"':
				quoted = false
			}
		} else if current == '"' {
			quoted = true
		}
	}
	return parts, !quoted && !escaped
}

func decodeParameterValue(value string, present bool) (string, bool) {
	if !present {
		return "", false
	}
	if isHTTPToken(value) {
		return value, true
	}
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", false
	}
	var decoded strings.Builder
	for index := 1; index < len(value)-1; index++ {
		current := value[index]
		if current == '\\' {
			index++
			if index >= len(value)-1 {
				return "", false
			}
			current = value[index]
		}
		if current != '\t' && (current < 0x20 || current == 0x7f || current == '"') {
			return "", false
		}
		decoded.WriteByte(current)
	}
	return decoded.String(), true
}

func parseQuality(value string) (float64, bool) {
	if value == "0" || value == "1" {
		parsed, _ := strconv.ParseFloat(value, 64)
		return parsed, true
	}
	integer, fraction, found := strings.Cut(value, ".")
	if !found || len(fraction) > 3 {
		return 0, false
	}
	switch integer {
	case "0":
		for _, digit := range fraction {
			if digit < '0' || digit > '9' {
				return 0, false
			}
		}
	case "1":
		for _, digit := range fraction {
			if digit != '0' {
				return 0, false
			}
		}
	default:
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}

func isHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for _, current := range []byte(value) {
		if current >= '0' && current <= '9' || current >= 'A' && current <= 'Z' ||
			current >= 'a' && current <= 'z' || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(current)) {
			continue
		}
		return false
	}
	return true
}
