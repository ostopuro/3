package merge

import (
	"errors"
	"strings"
	"time"

	"github.com/athom/hotel-merge/internal/domain"
)

// Merger merges hotels coming from multiple suppliers using deterministic rules.
type Merger struct {
	Priority []string
}

// Merge produces a single hotel object from supplier inputs keyed by supplier name.
func (m Merger) Merge(inputs map[string]domain.Hotel) (domain.Hotel, error) {
	if len(inputs) == 0 {
		return domain.Hotel{}, errors.New("merge: no inputs provided")
	}

	order := m.Priority
	seen := make(map[string]struct{}, len(order))
	for _, s := range order {
		seen[s] = struct{}{}
	}
	// Include any suppliers not in the priority list to guarantee determinism.
	for supplier := range inputs {
		if _, ok := seen[supplier]; !ok {
			order = append(order, supplier)
			seen[supplier] = struct{}{}
		}
	}

	merged := domain.Hotel{}
	var initialised bool

	var amenitiesGeneral []string
	var amenitiesRoom []string
	imageBuckets := make(map[string][]domain.Image)
	var bookingConditions []string

	for _, supplier := range order {
		hotel, ok := inputs[supplier]
		if !ok {
			continue
		}
		if !initialised {
			merged.ID = hotel.ID
			merged.DestinationID = hotel.DestinationID
			initialised = true
		}
		merged.Name = chooseRicherText(merged.Name, hotel.Name)
		merged.Description = chooseRicherText(merged.Description, hotel.Description)

		merged.Location.Address = chooseRicherText(merged.Location.Address, hotel.Location.Address)
		merged.Location.City = chooseRicherText(merged.Location.City, hotel.Location.City)
		merged.Location.Country = chooseRicherText(merged.Location.Country, hotel.Location.Country)
		merged.Location.PostalCode = chooseRicherText(merged.Location.PostalCode, hotel.Location.PostalCode)
		if merged.Location.Lat == nil && hotel.Location.Lat != nil {
			lat := *hotel.Location.Lat
			merged.Location.Lat = &lat
		}
		if merged.Location.Lng == nil && hotel.Location.Lng != nil {
			lng := *hotel.Location.Lng
			merged.Location.Lng = &lng
		}

		if len(hotel.Amenities.General) > 0 {
			amenitiesGeneral = append(amenitiesGeneral, hotel.Amenities.General...)
		}
		if len(hotel.Amenities.Room) > 0 {
			amenitiesRoom = append(amenitiesRoom, hotel.Amenities.Room...)
		}

		for category, images := range hotel.Images {
			if len(images) == 0 {
				continue
			}
			imageBuckets[category] = append(imageBuckets[category], images...)
		}

		if len(hotel.BookingConditions) > 0 {
			bookingConditions = append(bookingConditions, hotel.BookingConditions...)
		}
	}

	merged.Amenities = domain.Amenities{
		General: dedupeStrings(amenitiesGeneral),
		Room:    dedupeStrings(amenitiesRoom),
	}
	merged.Images = dedupeImages(imageBuckets)
	merged.BookingConditions = dedupeBookingConditions(bookingConditions)
	merged.LastUpdated = time.Now()

	return merged, nil
}

func chooseRicherText(current, candidate string) string {
	cand := strings.TrimSpace(candidate)
	cur := strings.TrimSpace(current)
	if cand == "" {
		return cur
	}
	if cur == "" || len(cand) > len(cur) {
		return cand
	}
	return cur
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func dedupeImages(buckets map[string][]domain.Image) map[string][]domain.Image {
	if len(buckets) == 0 {
		return nil
	}
	result := make(map[string][]domain.Image, len(buckets))
	for category, images := range buckets {
		seen := make(map[string]int)
		for _, img := range images {
			link := strings.TrimSpace(img.Link)
			if link == "" {
				continue
			}
			desc := strings.TrimSpace(img.Description)
			if idx, ok := seen[link]; ok {
				if desc != "" && len(desc) > len(strings.TrimSpace(result[category][idx].Description)) {
					result[category][idx].Description = desc
				}
				continue
			}
			result[category] = append(result[category], domain.Image{Link: link, Description: desc})
			seen[link] = len(result[category]) - 1
		}
		if len(result[category]) == 0 {
			delete(result, category)
		}
	}

	return result
}

func dedupeBookingConditions(conditions []string) []string {
	seen := make(map[string]struct{}, len(conditions))
	result := make([]string, 0, len(conditions))
	for _, cond := range conditions {
		trimmed := strings.TrimSpace(cond)
		if trimmed == "" {
			continue
		}
		key := normalizeWhitespace(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func normalizeWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.ToLower(strings.Join(fields, " "))
}
