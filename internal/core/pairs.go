package core

import (
	"fmt"
	"net/url"
	"strings"
)

func queryValues(raw string) (url.Values, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL query: %w", err)
	}
	return values, nil
}

func ParsePairs(text string, separator string, unique bool) ([]Pair, error) {
	var pairs []Pair
	seen := map[string]bool{}
	for index, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, found := strings.Cut(line, separator)
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return nil, fmt.Errorf("line %d: expected key%svalue", index+1, separator)
		}
		if unique && seen[key] {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		seen[key] = true
		pairs = append(pairs, Pair{Key: key, Value: strings.TrimSpace(value)})
	}
	return pairs, nil
}

func FormatPairs(pairs []Pair, separator string) string {
	lines := make([]string, len(pairs))
	for index, pair := range pairs {
		lines[index] = pair.Key + separator + pair.Value
	}
	return strings.Join(lines, "\n")
}
