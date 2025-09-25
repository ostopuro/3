package merge_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/athom/hotel-merge/internal/domain"
	"github.com/athom/hotel-merge/internal/merge"
)

func TestMergePrefersRicherTextAndFillsLocation(t *testing.T) {
	lat1 := 1.0
	lat2 := 2.0
	lng1 := 100.0
	lng2 := 102.0

	inputs := map[string]domain.Hotel{
		"acme": {
			ID:            "H1",
			DestinationID: 10,
			Name:          "Short",
			Description:   "Tiny",
			Location: domain.Location{
				Lat: &lat1,
				Lng: &lng1,
			},
			Amenities: domain.Amenities{
				General: []string{"WiFi", "BusinessCenter"},
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
				Lat:        &lat2,
				Lng:        &lng2,
			},
			Amenities: domain.Amenities{
				General: []string{"indoor pool", "Breakfast"},
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
	if merged.Location.Lat == nil || !almostEqual(*merged.Location.Lat, (lat1+lat2)/2) {
		t.Fatalf("expected averaged latitude, got %v", merged.Location.Lat)
	}
	if merged.Location.Lng == nil || !almostEqual(*merged.Location.Lng, (lng1+lng2)/2) {
		t.Fatalf("expected averaged longitude, got %v", merged.Location.Lng)
	}
	expectedGeneral := []string{"breakfast", "business center", "indoor pool", "pool", "wifi"}
	if !reflect.DeepEqual(merged.Amenities.General, expectedGeneral) {
		t.Fatalf("expected general amenities %v, got %v", expectedGeneral, merged.Amenities.General)
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

func TestMergeAveragesSingleValue(t *testing.T) {
	lat := 51.0
	lng := -0.1
	inputs := map[string]domain.Hotel{
		"acme": {
			ID:            "H1",
			DestinationID: 99,
			Location: domain.Location{
				Lat: &lat,
				Lng: &lng,
			},
			Amenities: domain.Amenities{
				General: []string{"WiFi"},
				Room:    []string{"AirCon"},
			},
		},
	}

	merger := merge.Merger{Priority: []string{"acme"}}
	merged, err := merger.Merge(inputs)
	if err != nil {
		t.Fatalf("merge returned error: %v", err)
	}
	if merged.Location.Lat == nil || !almostEqual(*merged.Location.Lat, lat) {
		t.Fatalf("expected latitude %v, got %v", lat, merged.Location.Lat)
	}
	if merged.Location.Lng == nil || !almostEqual(*merged.Location.Lng, lng) {
		t.Fatalf("expected longitude %v, got %v", lng, merged.Location.Lng)
	}

	expectedGeneral := []string{"wifi"}
	expectedRoom := []string{"aircon"}
	if !reflect.DeepEqual(merged.Amenities.General, expectedGeneral) {
		t.Fatalf("expected general amenities %v, got %v", expectedGeneral, merged.Amenities.General)
	}
	if !reflect.DeepEqual(merged.Amenities.Room, expectedRoom) {
		t.Fatalf("expected room amenities %v, got %v", expectedRoom, merged.Amenities.Room)
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
