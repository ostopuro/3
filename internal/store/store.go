package store

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/athom/hotel-merge/internal/domain"
)

// Key uniquely identifies a hotel within a destination.
type Key struct {
	DestinationID int
	HotelID       string
}

// Record captures all supplier payloads and merged result for a hotel.
type Record struct {
	Raw         map[string][]byte
	Normalized  map[string]domain.Hotel
	Canonical   *domain.Hotel
	NeedsMerge  bool
	LastFetched map[string]time.Time
	LastMerged  time.Time
}

// RecordSnapshot is a read-only copy used outside the store lock.
type RecordSnapshot struct {
	Key        Key
	Raw        map[string][]byte
	Normalized map[string]domain.Hotel
	Canonical  *domain.Hotel
	NeedsMerge bool
	Meta       recordMeta
}

type recordMeta struct {
	LastFetched map[string]time.Time
	LastMerged  time.Time
}

// Store defines the persistence contract used by the service layer.
type Store interface {
	UpsertSupplier(ctx context.Context, supplier string, hotel domain.Hotel, raw []byte, fetchedAt time.Time) (updated bool, err error)
	ListPendingMerge(ctx context.Context) ([]RecordSnapshot, error)
	UpdateMerged(ctx context.Context, key Key, merged domain.Hotel, mergedAt time.Time) error
	QueryMerged(ctx context.Context, destinations []int, hotels []string) ([]domain.Hotel, error)
	ListRecords(ctx context.Context) ([]RecordSnapshot, error)
	GetRecord(ctx context.Context, key Key) (RecordSnapshot, bool, error)
}

// MemoryStore is an in-memory implementation of Store.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[int]map[string]*Record
}

// NewMemoryStore creates a MemoryStore instance.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[int]map[string]*Record)}
}

// UpsertSupplier stores supplier payload; returns true if data changed.
func (s *MemoryStore) UpsertSupplier(ctx context.Context, supplier string, hotel domain.Hotel, raw []byte, fetchedAt time.Time) (bool, error) {
	if supplier == "" {
		return false, errors.New("supplier name required")
	}
	if hotel.ID == "" {
		return false, errors.New("hotel id required")
	}
	destID := hotel.DestinationID

	s.mu.Lock()
	defer s.mu.Unlock()

	byDest, ok := s.items[destID]
	if !ok {
		byDest = make(map[string]*Record)
		s.items[destID] = byDest
	}
	rec, ok := byDest[hotel.ID]
	if !ok {
		rec = &Record{
			Raw:         make(map[string][]byte),
			Normalized:  make(map[string]domain.Hotel),
			LastFetched: make(map[string]time.Time),
		}
		byDest[hotel.ID] = rec
	}

	existing, exists := rec.Raw[supplier]
	changed := !exists || !bytes.Equal(existing, raw)
	if changed {
		rawCopy := make([]byte, len(raw))
		copy(rawCopy, raw)
		rec.Raw[supplier] = rawCopy
		rec.Normalized[supplier] = cloneHotel(hotel)
		rec.NeedsMerge = true
	}
	rec.LastFetched[supplier] = fetchedAt
	return changed, nil
}

// ListPendingMerge returns snapshots for records requiring merge.
func (s *MemoryStore) ListPendingMerge(ctx context.Context) ([]RecordSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var snapshots []RecordSnapshot
	for destID, hotels := range s.items {
		for hotelID, rec := range hotels {
			if !rec.NeedsMerge {
				continue
			}
			snapshots = append(snapshots, snapshotFromRecord(Key{DestinationID: destID, HotelID: hotelID}, rec))
		}
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Key.DestinationID == snapshots[j].Key.DestinationID {
			return snapshots[i].Key.HotelID < snapshots[j].Key.HotelID
		}
		return snapshots[i].Key.DestinationID < snapshots[j].Key.DestinationID
	})
	return snapshots, nil
}

// ListRecords returns snapshots for all stored records.
func (s *MemoryStore) ListRecords(ctx context.Context) ([]RecordSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var snapshots []RecordSnapshot
	for destID, hotels := range s.items {
		for hotelID, rec := range hotels {
			snapshots = append(snapshots, snapshotFromRecord(Key{DestinationID: destID, HotelID: hotelID}, rec))
		}
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Key.DestinationID == snapshots[j].Key.DestinationID {
			return snapshots[i].Key.HotelID < snapshots[j].Key.HotelID
		}
		return snapshots[i].Key.DestinationID < snapshots[j].Key.DestinationID
	})
	return snapshots, nil
}

// GetRecord returns a single record snapshot.
func (s *MemoryStore) GetRecord(ctx context.Context, key Key) (RecordSnapshot, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hotels, ok := s.items[key.DestinationID]
	if !ok {
		return RecordSnapshot{}, false, nil
	}
	rec, ok := hotels[key.HotelID]
	if !ok {
		return RecordSnapshot{}, false, nil
	}
	return snapshotFromRecord(key, rec), true, nil
}

// UpdateMerged stores the merged hotel result.
func (s *MemoryStore) UpdateMerged(ctx context.Context, key Key, merged domain.Hotel, mergedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hotels, ok := s.items[key.DestinationID]
	if !ok {
		return errors.New("record not found")
	}
	rec, ok := hotels[key.HotelID]
	if !ok {
		return errors.New("record not found")
	}

	mergedCopy := cloneHotel(merged)
	if mergedCopy.LastFetched == nil {
		mergedCopy.LastFetched = make(map[string]time.Time)
	}
	for supplier, ts := range rec.LastFetched {
		mergedCopy.LastFetched[supplier] = ts
	}
	mergedCopy.LastMerged = mergedAt
	mergedCopy.LastUpdated = mergedAt

	rec.Canonical = &mergedCopy
	rec.NeedsMerge = false
	rec.LastMerged = mergedAt
	return nil
}

// QueryMerged retrieves merged hotels filtered by destination/hotel ids.
func (s *MemoryStore) QueryMerged(ctx context.Context, destinations []int, hotels []string) ([]domain.Hotel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	destFilter := make(map[int]struct{})
	if len(destinations) > 0 {
		for _, id := range destinations {
			destFilter[id] = struct{}{}
		}
	}
	hotelFilter := make(map[string]struct{})
	if len(hotels) > 0 {
		for _, id := range hotels {
			hotelFilter[id] = struct{}{}
		}
	}

	var results []domain.Hotel
	for destID, hotelsMap := range s.items {
		if len(destFilter) > 0 {
			if _, ok := destFilter[destID]; !ok {
				continue
			}
		}
		for hotelID, rec := range hotelsMap {
			if len(hotelFilter) > 0 {
				if _, ok := hotelFilter[hotelID]; !ok {
					continue
				}
			}
			if rec.NeedsMerge || rec.Canonical == nil {
				continue
			}
			results = append(results, cloneHotel(*rec.Canonical))
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].DestinationID == results[j].DestinationID {
			return results[i].ID < results[j].ID
		}
		return results[i].DestinationID < results[j].DestinationID
	})

	return results, nil
}

func snapshotFromRecord(key Key, rec *Record) RecordSnapshot {
	snap := RecordSnapshot{
		Key:        key,
		Raw:        make(map[string][]byte, len(rec.Raw)),
		Normalized: make(map[string]domain.Hotel, len(rec.Normalized)),
		NeedsMerge: rec.NeedsMerge,
		Meta: recordMeta{
			LastFetched: make(map[string]time.Time, len(rec.LastFetched)),
			LastMerged:  rec.LastMerged,
		},
	}
	for supplier, payload := range rec.Raw {
		copyPayload := make([]byte, len(payload))
		copy(copyPayload, payload)
		snap.Raw[supplier] = copyPayload
	}
	for supplier, hotel := range rec.Normalized {
		snap.Normalized[supplier] = cloneHotel(hotel)
	}
	for supplier, ts := range rec.LastFetched {
		snap.Meta.LastFetched[supplier] = ts
	}
	if rec.Canonical != nil {
		canonicalCopy := cloneHotel(*rec.Canonical)
		snap.Canonical = &canonicalCopy
	}
	return snap
}

// cloneHotel deep copies a Hotel value.
func cloneHotel(h domain.Hotel) domain.Hotel {
	copyHotel := h

	if h.Amenities.General != nil {
		copyHotel.Amenities.General = append([]string(nil), h.Amenities.General...)
	}
	if h.Amenities.Room != nil {
		copyHotel.Amenities.Room = append([]string(nil), h.Amenities.Room...)
	}
	if h.Images != nil {
		copiedImages := make(map[string][]domain.Image, len(h.Images))
		for k, imgs := range h.Images {
			copiedImages[k] = append([]domain.Image(nil), imgs...)
		}
		copyHotel.Images = copiedImages
	}
	if h.BookingConditions != nil {
		copyHotel.BookingConditions = append([]string(nil), h.BookingConditions...)
	}
	if h.LastFetched != nil {
		copied := make(map[string]time.Time, len(h.LastFetched))
		for k, v := range h.LastFetched {
			copied[k] = v
		}
		copyHotel.LastFetched = copied
	}

	if h.Location.Lat != nil {
		lat := *h.Location.Lat
		copyHotel.Location.Lat = &lat
	}
	if h.Location.Lng != nil {
		lng := *h.Location.Lng
		copyHotel.Location.Lng = &lng
	}

	return copyHotel
}
