package suppliers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/athom/hotel-merge/internal/domain"
)

type acmeSupplier struct {
	cfg Config
}

// NewAcmeSupplier creates a supplier implementation for ACME.
func NewAcmeSupplier(cfg Config) Supplier {
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &acmeSupplier{cfg: cfg}
}

func (s *acmeSupplier) Name() string { return "acme" }

func (s *acmeSupplier) Fetch(ctx context.Context) ([]HotelData, error) {
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
		return nil, fmt.Errorf("supplier acme: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload []acmeHotel
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	results := make([]HotelData, 0, len(payload))
	for _, src := range payload {
		if src.ID == "" || src.DestinationID == 0 {
			continue
		}
		raw, err := json.Marshal(src)
		if err != nil {
			return nil, err
		}
		hotel, err := convertAcmeHotel(src)
		if err != nil {
			return nil, err
		}
		results = append(results, HotelData{Hotel: hotel, Raw: raw})
	}
	return results, nil
}

type acmeHotel struct {
	ID            string          `json:"Id"`
	DestinationID int             `json:"DestinationId"`
	Name          string          `json:"Name"`
	Latitude      json.RawMessage `json:"Latitude"`
	Longitude     json.RawMessage `json:"Longitude"`
	Address       string          `json:"Address"`
	City          string          `json:"City"`
	Country       string          `json:"Country"`
	PostalCode    string          `json:"PostalCode"`
	Description   string          `json:"Description"`
	Facilities    []string        `json:"Facilities"`
}

func convertAcmeHotel(src acmeHotel) (domain.Hotel, error) {
	if src.ID == "" {
		return domain.Hotel{}, errors.New("acme hotel missing id")
	}
	if src.DestinationID == 0 {
		return domain.Hotel{}, fmt.Errorf("acme hotel %s missing destination id", src.ID)
	}

	hotel := domain.Hotel{
		ID:            strings.TrimSpace(src.ID),
		DestinationID: src.DestinationID,
		Name:          strings.TrimSpace(src.Name),
		Description:   strings.TrimSpace(src.Description),
		Location: domain.Location{
			Address:    strings.TrimSpace(src.Address),
			City:       strings.TrimSpace(src.City),
			Country:    strings.TrimSpace(src.Country),
			PostalCode: strings.TrimSpace(src.PostalCode),
		},
		Amenities: domain.Amenities{
			General: trimStrings(src.Facilities),
		},
	}
	if lat := parseOptionalFloat(src.Latitude); lat != nil {
		hotel.Location.Lat = lat
	}
	if lng := parseOptionalFloat(src.Longitude); lng != nil {
		hotel.Location.Lng = lng
	}
	return hotel, nil
}

func parseOptionalFloat(raw json.RawMessage) *float64 {
	if len(raw) == 0 {
		return nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return &f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return &v
		}
	}
	return nil
}
