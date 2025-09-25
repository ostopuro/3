package suppliers

import "strings"

func trimStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			cleaned = append(cleaned, t)
		}
	}
	return cleaned
}
