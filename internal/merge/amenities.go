package merge

import "sort"

import "strings"

// AmenityStrategy defines how raw amenity lists are normalised.
type AmenityStrategy interface {
	Normalize(category string, values []string) []string
}

// DefaultAmenityStrategy applies trimming, casing, de-duplication and business rules.
type DefaultAmenityStrategy struct{}

// Normalize implements AmenityStrategy.
func (DefaultAmenityStrategy) Normalize(category string, values []string) []string {
	if len(values) == 0 {
		return nil
	}

	set := make(map[string]struct{})
	for _, value := range values {
		normalized := normalizeAmenity(value)
		if normalized == "" {
			continue
		}
		normalized = applySynonyms(normalized)
		set[normalized] = struct{}{}
	}

	// Business-specific rules.
	if containsKey(set, "indoor pool") || containsKey(set, "outdoor pool") {
		set["pool"] = struct{}{}
	}

	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeAmenity(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

func applySynonyms(value string) string {
	switch value {
	case "businesscenter":
		return "business center"
	default:
		return value
	}
}

func containsKey(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}
