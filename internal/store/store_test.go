package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/athom/hotel-merge/internal/domain"
	"github.com/athom/hotel-merge/internal/store"
)

func TestMemoryStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()

	hotel := domain.Hotel{
		ID:            "H1",
		DestinationID: 99,
		Name:          "Sample",
		Location:      domain.Location{},
		Amenities:     domain.Amenities{},
	}
	raw := []byte(`{"id":"H1"}`)

	changed, err := s.UpsertSupplier(ctx, "acme", hotel, raw, time.Unix(100, 0))
	if err != nil {
		t.Fatalf("upsert returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected change on first upsert")
	}

	snapshots, err := s.ListPendingMerge(ctx)
	if err != nil {
		t.Fatalf("ListPendingMerge returned error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	snap := snapshots[0]
	if len(snap.Normalized) != 1 {
		t.Fatalf("expected normalized data present")
	}

	merged := hotel
	merged.Name = "Merged"
	merged.Amenities.General = []string{"wifi"}

	if err := s.UpdateMerged(ctx, snap.Key, merged, time.Unix(200, 0)); err != nil {
		t.Fatalf("UpdateMerged returned error: %v", err)
	}

	results, err := s.QueryMerged(ctx, []int{99}, nil)
	if err != nil {
		t.Fatalf("QueryMerged returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "Merged" {
		t.Fatalf("expected merged name propagates, got %q", results[0].Name)
	}
	if results[0].LastMerged.IsZero() {
		t.Fatalf("expected last merged timestamp to be set")
	}
}
