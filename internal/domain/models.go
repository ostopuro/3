package domain

import "time"

// Hotel represents the unified hotel model exposed by the API.
type Hotel struct {
	ID                string               `json:"id"`
	DestinationID     int                  `json:"destination_id"`
	Name              string               `json:"name,omitempty"`
	Location          Location             `json:"location"`
	Description       string               `json:"description,omitempty"`
	Amenities         Amenities            `json:"amenities"`
	Images            map[string][]Image   `json:"images,omitempty"`
	BookingConditions []string             `json:"booking_conditions,omitempty"`
	LastFetched       map[string]time.Time `json:"last_fetched,omitempty"`
	LastUpdated       time.Time            `json:"last_updated,omitempty"`
	LastMerged        time.Time            `json:"last_merged,omitempty"`
}

// Location captures geographical and address details.
type Location struct {
	Lat        *float64 `json:"lat,omitempty"`
	Lng        *float64 `json:"lng,omitempty"`
	Address    string   `json:"address,omitempty"`
	City       string   `json:"city,omitempty"`
	Country    string   `json:"country,omitempty"`
	PostalCode string   `json:"postal_code,omitempty"`
}

// Amenities groups the amenities by their type.
type Amenities struct {
	General []string `json:"general,omitempty"`
	Room    []string `json:"room,omitempty"`
}

// Image represents a single image with its descriptive text.
type Image struct {
	Link        string `json:"link"`
	Description string `json:"description,omitempty"`
}
