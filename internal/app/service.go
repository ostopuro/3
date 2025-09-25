package app

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/athom/hotel-merge/internal/domain"
	"github.com/athom/hotel-merge/internal/merge"
	"github.com/athom/hotel-merge/internal/store"
	"github.com/athom/hotel-merge/internal/suppliers"
)

// Service wires together suppliers, storage, merging, and querying.
type Service struct {
	store         store.Store
	audit         store.AuditStore
	suppliers     []suppliers.Supplier
	supplierOrder []string
	merger        merge.Merger
	fetchInterval time.Duration
	mergeInterval time.Duration
	logger        *log.Logger

	stop chan struct{}
	wg   sync.WaitGroup
	once sync.Once
}

// Config provides the dependencies required by Service.
type Config struct {
	Store         store.Store
	Audit         store.AuditStore
	Suppliers     []suppliers.Supplier
	FetchInterval time.Duration
	MergeInterval time.Duration
	Logger        *log.Logger
}

// NewService builds a Service instance.
func NewService(cfg Config) *Service {
	order := make([]string, 0, len(cfg.Suppliers))
	for _, s := range cfg.Suppliers {
		order = append(order, s.Name())
	}
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}
	return &Service{
		store:         cfg.Store,
		audit:         cfg.Audit,
		suppliers:     cfg.Suppliers,
		supplierOrder: order,
		merger:        merge.Merger{Priority: order},
		fetchInterval: defaultIfZero(cfg.FetchInterval, 5*time.Minute),
		mergeInterval: defaultIfZero(cfg.MergeInterval, time.Minute),
		logger:        logger,
		stop:          make(chan struct{}),
	}
}

// Start launches the background fetch and merge loops.
func (s *Service) Start(ctx context.Context) {
	s.wg.Add(2)
	go s.fetchLoop(ctx)
	go s.mergeLoop(ctx)
}

// Stop signals background loops to cease.
func (s *Service) Stop() {
	s.once.Do(func() {
		close(s.stop)
	})
	s.wg.Wait()
}

// Query returns merged hotels filtered by destination and hotel identifiers.
func (s *Service) Query(ctx context.Context, destinations []int, hotelIDs []string) ([]domain.Hotel, error) {
	return s.store.QueryMerged(ctx, destinations, hotelIDs)
}

// ListRecords returns all records for debugging purposes.
func (s *Service) ListRecords(ctx context.Context) ([]store.RecordSnapshot, error) {
	return s.store.ListRecords(ctx)
}

// GetRecord returns a single record snapshot.
func (s *Service) GetRecord(ctx context.Context, destinationID int, hotelID string) (store.RecordSnapshot, bool, error) {
	return s.store.GetRecord(ctx, store.Key{DestinationID: destinationID, HotelID: hotelID})
}

func (s *Service) fetchLoop(ctx context.Context) {
	defer s.wg.Done()
	s.runFetchOnce(ctx)
	ticker := time.NewTicker(s.fetchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			s.runFetchOnce(ctx)
		}
	}
}

func (s *Service) mergeLoop(ctx context.Context) {
	defer s.wg.Done()
	s.runMergeOnce(ctx)
	ticker := time.NewTicker(s.mergeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			s.runMergeOnce(ctx)
		}
	}
}

func (s *Service) runFetchOnce(ctx context.Context) {
	for _, supplier := range s.suppliers {
		data, err := supplier.Fetch(ctx)
		if err != nil {
			s.logger.Printf("supplier %s fetch error: %v", supplier.Name(), err)
			continue
		}
		for _, item := range data {
			changed, err := s.store.UpsertSupplier(ctx, supplier.Name(), item.Hotel, item.Raw, time.Now())
			if err != nil {
				s.logger.Printf("supplier %s upsert error: %v", supplier.Name(), err)
				continue
			}
			if !changed {
				continue
			}
			if s.audit != nil {
				_ = s.audit.Append(ctx, item.Hotel.DestinationID, item.Hotel.ID, store.AuditRecord{
					Supplier:   supplier.Name(),
					RawPayload: cloneBytes(item.Raw),
					Timestamp:  time.Now(),
				})
			}
		}
	}
}

func (s *Service) runMergeOnce(ctx context.Context) {
	snapshots, err := s.store.ListPendingMerge(ctx)
	if err != nil {
		s.logger.Printf("list pending merge error: %v", err)
		return
	}
	for _, snap := range snapshots {
		merged, err := s.merger.Merge(snap.Normalized)
		if err != nil {
			s.logger.Printf("merge error for hotel %s/%d: %v", snap.Key.HotelID, snap.Key.DestinationID, err)
			continue
		}
		if merged.LastFetched == nil {
			merged.LastFetched = make(map[string]time.Time)
		}
		for supplier, ts := range snap.Meta.LastFetched {
			merged.LastFetched[supplier] = ts
		}
		mergedAt := time.Now()
		if err := s.store.UpdateMerged(ctx, snap.Key, merged, mergedAt); err != nil {
			s.logger.Printf("update merged error for hotel %s/%d: %v", snap.Key.HotelID, snap.Key.DestinationID, err)
			continue
		}
		if s.audit != nil {
			mergedCopy := merged
			mergedCopy.LastMerged = mergedAt
			mergedCopy.LastUpdated = mergedAt
			_ = s.audit.Append(ctx, snap.Key.DestinationID, snap.Key.HotelID, store.AuditRecord{
				Supplier:  "merged",
				Merged:    &mergedCopy,
				Timestamp: mergedAt,
			})
		}
	}
}

func defaultIfZero[T comparable](value T, fallback T) T {
	var zero T
	if value == zero {
		return fallback
	}
	return value
}

func cloneBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
