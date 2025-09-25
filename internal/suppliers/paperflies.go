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

type paperfliesSupplier struct {
	cfg Config
}

// NewPaperfliesSupplier returns the Paperflies supplier adapter.
func NewPaperfliesSupplier(cfg Config) Supplier {
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &paperfliesSupplier{cfg: cfg}
}

func (s *paperfliesSupplier) Name() string { return "paperflies" }

func (s *paperfliesSupplier) Fetch(ctx context.Context) ([]HotelData, error) {
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
		return nil, fmt.Errorf("supplier paperflies: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload []paperfliesHotel
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]HotelData, 0, len(payload))
	for _, src := range payload {
		if src.HotelID == "" || src.DestinationID == 0 {
			continue
		}
		raw, err := json.Marshal(src)
		if err != nil {
			return nil, err
		}
		hotel := convertPaperfliesHotel(src)
		out = append(out, HotelData{Hotel: hotel, Raw: raw})
	}
	return out, nil
}

type paperfliesHotel struct {
	HotelID           string                           `json:"hotel_id"`
	DestinationID     int                              `json:"destination_id"`
	HotelName         string                           `json:"hotel_name"`
	Location          paperfliesLocation               `json:"location"`
	Details           string                           `json:"details"`
	Amenities         paperfliesAmenities              `json:"amenities"`
	Images            map[string][]paperfliesImageItem `json:"images"`
	BookingConditions []string                         `json:"booking_conditions"`
}

type paperfliesLocation struct {
	Address string `json:"address"`
	Country string `json:"country"`
}

type paperfliesAmenities struct {
	General []string `json:"general"`
	Room    []string `json:"room"`
}

type paperfliesImageItem struct {
	Link    string `json:"link"`
	Caption string `json:"caption"`
}

func convertPaperfliesHotel(src paperfliesHotel) domain.Hotel {
	hotel := domain.Hotel{
		ID:            strings.TrimSpace(src.HotelID),
		DestinationID: src.DestinationID,
		Name:          strings.TrimSpace(src.HotelName),
		Description:   strings.TrimSpace(src.Details),
		Location: domain.Location{
			Address: strings.TrimSpace(src.Location.Address),
			Country: strings.TrimSpace(src.Location.Country),
		},
		Amenities: domain.Amenities{
			General: trimStrings(src.Amenities.General),
			Room:    trimStrings(src.Amenities.Room),
		},
		Images:            make(map[string][]domain.Image),
		BookingConditions: trimStrings(src.BookingConditions),
	}

	for category, images := range src.Images {
		if len(images) == 0 {
			continue
		}
		list := make([]domain.Image, 0, len(images))
		for _, img := range images {
			if strings.TrimSpace(img.Link) == "" {
				continue
			}
			list = append(list, domain.Image{
				Link:        strings.TrimSpace(img.Link),
				Description: strings.TrimSpace(img.Caption),
			})
		}
		if len(list) > 0 {
			hotel.Images[category] = list
		}
	}
	if len(hotel.Images) == 0 {
		hotel.Images = nil
	}
	return hotel
}
