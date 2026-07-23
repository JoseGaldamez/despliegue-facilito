package domains

import "strings"

// ValidateHostname checks if the given hostname conforms to RFC 1123.
// Rules:
// - Total length ≤ 253 characters
// - At least 2 labels separated by dots
// - Each label: 1-63 characters, only alphanumeric and hyphens
// - Labels cannot start or end with a hyphen
// - Normalized to lowercase
func ValidateHostname(hostname string) (string, bool) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))

	if hostname == "" {
		return "", false
	}

	if len(hostname) > 253 {
		return "", false
	}

	labels := strings.Split(hostname, ".")
	if len(labels) < 2 {
		return "", false
	}

	for _, label := range labels {
		if !isValidLabel(label) {
			return "", false
		}
	}

	return hostname, true
}

// isValidLabel checks if a single DNS label conforms to RFC 1123.
func isValidLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 {
		return false
	}

	if label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}

	for _, ch := range label {
		if !isAlphanumericOrHyphen(ch) {
			return false
		}
	}

	return true
}

// isAlphanumericOrHyphen checks if a character is a-z, 0-9, or hyphen.
func isAlphanumericOrHyphen(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-'
}
