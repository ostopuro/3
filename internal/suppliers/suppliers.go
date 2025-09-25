package suppliers

import (
	"context"
	"net/http"
	"time"

	"github.com/athom/hotel-merge/internal/domain"
)

// HotelData bundles the raw supplier payload together with the normalized view.
type HotelData struct {
	Hotel domain.Hotel
	Raw   []byte
}

// Supplier represents a single upstream provider.
type Supplier interface {
	Name() string
	Fetch(ctx context.Context) ([]HotelData, error)
}

// HTTPClient abstracts *http.Client for easier testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Config captures supplier runtime configuration.
type Config struct {
	Client   HTTPClient
	Endpoint string
	Timeout  time.Duration
}
