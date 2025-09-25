package suppliers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/athom/hotel-merge/internal/domain"
)

type patagoniaSupplier struct {
	cfg Config
}

// NewPatagoniaSupplier builds the Patagonia supplier adapter.
func NewPatagoniaSupplier(cfg Config) Supplier {
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &patagoniaSupplier{cfg: cfg}
}

func (s *patagoniaSupplier) Name() string { return "patagonia" }

func (s *patagoniaSupplier) Fetch(ctx context.Context) ([]HotelData, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.Endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.cfg.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("supplier patagonia: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload []patagoniaHotel
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]HotelData, 0, len(payload))
	for _, src := range payload {
		if src.ID == "" || src.DestinationID == 0 {
			continue
		}
		raw, err := json.Marshal(src)
		if err != nil {
			return nil, err
		}
		hotel := convertPatagoniaHotel(src)
		out = append(out, HotelData{Hotel: hotel, Raw: raw})
	}
	return out, nil
}

type patagoniaHotel struct {
	ID            string                          `json:"id"`
	DestinationID int                             `json:"destination"`
	Name          string                          `json:"name"`
	Lat           *float64                        `json:"lat"`
	Lng           *float64                        `json:"lng"`
	Address       *string                         `json:"address"`
	Info          *string                         `json:"info"`
	Amenities     *[]string                       `json:"amenities"`
	Images        map[string][]patagoniaImageItem `json:"images"`
}

type patagoniaImageItem struct {
	URL         string  `json:"url"`
	Description *string `json:"description"`
}

func convertPatagoniaHotel(src patagoniaHotel) domain.Hotel {
	hotel := domain.Hotel{
		ID:            strings.TrimSpace(src.ID),
		DestinationID: src.DestinationID,
		Name:          strings.TrimSpace(src.Name),
		Description:   strings.TrimSpace(valueOrDefault(src.Info)),
		Location: domain.Location{
			Address: strings.TrimSpace(valueOrDefault(src.Address)),
		},
		Amenities: domain.Amenities{},
		Images:    make(map[string][]domain.Image),
	}
	if src.Lat != nil {
		hotel.Location.Lat = src.Lat
	}
	if src.Lng != nil {
		hotel.Location.Lng = src.Lng
	}
	if src.Amenities != nil {
		hotel.Amenities.Room = trimStrings(*src.Amenities)
	}
	for category, images := range src.Images {
		if len(images) == 0 {
			continue
		}
		items := make([]domain.Image, 0, len(images))
		for _, img := range images {
			if strings.TrimSpace(img.URL) == "" {
				continue
			}
			items = append(items, domain.Image{
				Link:        strings.TrimSpace(img.URL),
				Description: strings.TrimSpace(valueOrDefault(img.Description)),
			})
		}
		if len(items) > 0 {
			hotel.Images[category] = items
		}
	}
	if len(hotel.Images) == 0 {
		hotel.Images = nil
	}
	return hotel
}

func valueOrDefault(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
