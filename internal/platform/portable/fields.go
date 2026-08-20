package portable

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

// ValidBoundedName implements FIELD-001 and FIELD-013 without depending on a
// runtime Unicode whitespace table.
func ValidBoundedName(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	count := utf8.RuneCountInString(value)
	if count < 1 || count > 100 {
		return false
	}
	first := true
	lastWhitespace := false
	for _, current := range value {
		if isPortableControl(current) {
			return false
		}
		whitespace := isPortableWhitespace(current)
		if first && whitespace {
			return false
		}
		first = false
		lastWhitespace = whitespace
	}
	return !lastWhitespace
}

// NormalizeContactEmail applies the portable ASCII trimming and domain-case
// rules, returning false when the normalized value is not a ContactEmail.
func NormalizeContactEmail(value string) (string, bool) {
	value = trimASCIIWhitespace(value)
	if len(value) > 254 || !isASCII(value) || strings.Count(value, "@") != 1 {
		return "", false
	}
	local, domain, _ := strings.Cut(value, "@")
	if len(local) < 1 || len(local) > 64 || local[0] == '.' || local[len(local)-1] == '.' ||
		strings.Contains(local, "..") {
		return "", false
	}
	for _, current := range []byte(local) {
		if isASCIIAlphaNumeric(current) || strings.ContainsRune("!#$%&'*+/=?^_{|}~.-", rune(current)) {
			continue
		}
		return "", false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", false
	}
	for _, label := range labels {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", false
		}
		for _, current := range []byte(label) {
			if !isASCIIAlphaNumeric(current) && current != '-' {
				return "", false
			}
		}
	}
	return local + "@" + strings.ToLower(domain), true
}

func NormalizePhoneNumber(value string) (string, bool) {
	value = trimASCIIWhitespace(value)
	return value, phonePattern.MatchString(value)
}

func trimASCIIWhitespace(value string) string {
	return strings.Trim(value, "\t\n\v\f\r ")
}

func isPortableControl(value rune) bool {
	return value >= 0 && value <= 0x1f || value >= 0x7f && value <= 0x9f
}

func isPortableWhitespace(value rune) bool {
	return value >= 0x09 && value <= 0x0d || value == 0x20 || value == 0x85 || value == 0xa0 ||
		value == 0x1680 || value >= 0x2000 && value <= 0x200a || value == 0x2028 || value == 0x2029 ||
		value == 0x202f || value == 0x205f || value == 0x3000
}

func isASCII(value string) bool {
	for _, current := range []byte(value) {
		if current > 0x7f {
			return false
		}
	}
	return true
}

func isASCIIAlphaNumeric(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}
