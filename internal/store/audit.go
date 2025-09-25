package store

import (
	"context"
	"sync"
	"time"

	"github.com/athom/hotel-merge/internal/domain"
)

// AuditRecord captures one supplier fetch or merge event.
type AuditRecord struct {
	DestinationID int           `json:"destination_id"`
	HotelID       string        `json:"hotel_id"`
	Supplier      string        `json:"supplier"`
	RawPayload    []byte        `json:"raw_payload,omitempty"`
	Merged        *domain.Hotel `json:"merged,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
}

// AuditStore describes the behaviour needed for traceability.
type AuditStore interface {
	Append(ctx context.Context, destinationID int, hotelID string, record AuditRecord) error
	List(ctx context.Context, destinationID int) ([]AuditRecord, error)
}

// MemoryAuditStore is an in-memory implementation of AuditStore.
type MemoryAuditStore struct {
	mu   sync.RWMutex
	data map[int][]AuditRecord
}

// NewMemoryAuditStore creates a MemoryAuditStore.
func NewMemoryAuditStore() *MemoryAuditStore {
	return &MemoryAuditStore{data: make(map[int][]AuditRecord)}
}

// Append stores a new audit record.
func (s *MemoryAuditStore) Append(ctx context.Context, destinationID int, hotelID string, record AuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record.DestinationID = destinationID
	record.HotelID = hotelID
	s.data[destinationID] = append(s.data[destinationID], record)
	return nil
}

// List returns all audit records for a destination.
func (s *MemoryAuditStore) List(ctx context.Context, destinationID int) ([]AuditRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.data[destinationID]
	out := make([]AuditRecord, len(records))
	copy(out, records)
	return out, nil
}
