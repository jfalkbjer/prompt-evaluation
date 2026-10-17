package bedrock

import (
	"fmt"
	"strings"
)

// ParseLabel accepts the single word true or false, with surrounding space
// and an optional trailing period.
func ParseLabel(raw string) (bool, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	s = strings.Trim(s, ".")
	s = strings.TrimSpace(s)
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("expected true or false, got %q", raw)
	}
}

// ParseChoice accepts a reply that names exactly one allowed option.
func ParseChoice(raw string, allowed []string) (string, error) {
	known := map[string]bool{}
	for _, name := range allowed {
		known[name] = true
	}
	var hits []string
	seen := map[string]bool{}
	for _, field := range strings.Fields(strings.ToLower(raw)) {
		word := strings.Trim(field, ".,:;\"'`")
		if known[word] && !seen[word] {
			seen[word] = true
			hits = append(hits, word)
		}
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	return "", fmt.Errorf("expected one of %s, got %q", strings.Join(allowed, ", "), raw)
}
