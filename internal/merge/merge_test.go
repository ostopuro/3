package merge_test

import (
	"testing"

	"github.com/athom/hotel-merge/internal/domain"
	"github.com/athom/hotel-merge/internal/merge"
)

func TestMergePrefersRicherTextAndFillsLocation(t *testing.T) {
	lat1 := 1.0
	lat2 := 2.0

	inputs := map[string]domain.Hotel{
		"acme": {
			ID:            "H1",
			DestinationID: 10,
			Name:          "Short",
			Description:   "Tiny",
			Location: domain.Location{
				Lat: &lat1,
			},
			Amenities: domain.Amenities{
				General: []string{"Pool", "WiFi"},
			},
			Images: map[string][]domain.Image{
				"rooms": {{Link: "room.jpg", Description: "Room"}},
			},
		},
		"paperflies": {
			ID:            "H1",
			DestinationID: 10,
			Name:          "A much longer hotel name",
			Description:   "A longer description for testing.",
			Location: domain.Location{
				Address:    " 123 Sample Street ",
				City:       "Singapore",
				Country:    "Singapore",
				PostalCode: "123456",
				Lng:        &lat2,
			},
			Amenities: domain.Amenities{
				General: []string{"wifi", "Breakfast"},
			},
			Images: map[string][]domain.Image{
				"rooms": {
					{Link: "room.jpg", Description: "Spacious room"},
					{Link: "bath.jpg", Description: "Bath"},
				},
			},
			BookingConditions: []string{" WiFi is available in all areas. "},
		},
	}

	merger := merge.Merger{Priority: []string{"paperflies", "acme"}}
	merged, err := merger.Merge(inputs)
	if err != nil {
		t.Fatalf("merge returned error: %v", err)
	}

	if merged.Name != "A much longer hotel name" {
		t.Fatalf("expected richer name, got %q", merged.Name)
	}
	if merged.Description != "A longer description for testing." {
		t.Fatalf("expected richer description, got %q", merged.Description)
	}
	if merged.Location.Address != "123 Sample Street" {
		t.Fatalf("expected trimmed address, got %q", merged.Location.Address)
	}
	if merged.Location.City != "Singapore" || merged.Location.Country != "Singapore" {
		t.Fatalf("expected populated location from paperflies, got city %q country %q", merged.Location.City, merged.Location.Country)
	}
	if merged.Location.Lat == nil || *merged.Location.Lat != lat1 {
		t.Fatalf("expected latitude from acme, got %v", merged.Location.Lat)
	}
	if merged.Location.Lng == nil || *merged.Location.Lng != lat2 {
		t.Fatalf("expected longitude from paperflies, got %v", merged.Location.Lng)
	}
	if len(merged.Amenities.General) != 3 {
		t.Fatalf("expected deduped amenities, got %v", merged.Amenities.General)
	}
	if len(merged.Images["rooms"]) != 2 {
		t.Fatalf("expected deduped room images, got %v", merged.Images["rooms"])
	}
	if merged.Images["rooms"][0].Description != "Spacious room" {
		t.Fatalf("expected richer description retained for duplicate link")
	}
	if len(merged.BookingConditions) != 1 {
		t.Fatalf("expected a single booking condition, got %v", merged.BookingConditions)
	}
}

func TestMergeRejectsEmptyInput(t *testing.T) {
	merger := merge.Merger{}
	if _, err := merger.Merge(map[string]domain.Hotel{}); err == nil {
		t.Fatalf("expected error for empty input")
	}
}
