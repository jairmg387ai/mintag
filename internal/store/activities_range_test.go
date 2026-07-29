package store

import (
	"context"
	"testing"
)

// TestListActivitiesRange_InclusiveBothEnds verifies activities dated exactly
// on `from` and exactly on `to` are both included, while one dated the day
// before `from` and one dated the day after `to` are excluded.
func TestListActivitiesRange_InclusiveBothEnds(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	mustCreateActivity(t, s, ctx, "2026-06-09", "before range")
	mustCreateActivity(t, s, ctx, "2026-06-10", "on from")
	mustCreateActivity(t, s, ctx, "2026-06-15", "middle")
	mustCreateActivity(t, s, ctx, "2026-06-20", "on to")
	mustCreateActivity(t, s, ctx, "2026-06-21", "after range")

	list, err := s.ListActivitiesRange(ctx, "2026-06-10", "2026-06-20", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 results (inclusive both ends), got %d", len(list))
	}
	for _, a := range list {
		if a.Date < "2026-06-10" || a.Date > "2026-06-20" {
			t.Errorf("unexpected activity outside range: %q", a.Date)
		}
	}
}

// TestListActivitiesRange_FromAfterToRejected verifies from > to is a
// validation error, matching the spec's "Invalid range is rejected" scenario.
func TestListActivitiesRange_FromAfterToRejected(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	_, err = s.ListActivitiesRange(ctx, "2026-06-20", "2026-06-10", "")
	if err == nil {
		t.Error("expected error when from > to")
	}
}

// TestListActivitiesRange_BadDateFormatRejected verifies malformed dates on
// either bound are rejected before any query runs.
func TestListActivitiesRange_BadDateFormatRejected(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	tests := []struct {
		name, from, to string
	}{
		{"bad from", "10/06/2026", "2026-06-20"},
		{"bad to", "2026-06-10", "20-06-2026"},
		{"empty from", "", "2026-06-20"},
		{"empty to", "2026-06-10", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.ListActivitiesRange(ctx, tc.from, tc.to, ""); err == nil {
				t.Errorf("expected error for %s", tc.name)
			}
		})
	}
}

// TestListActivitiesRange_StatusFilter verifies the optional status filter
// behaves the same way as ListActivities.
func TestListActivitiesRange_StatusFilter(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a1 := mustCreateActivity(t, s, ctx, "2026-06-12", "will be approved")
	mustCreateActivity(t, s, ctx, "2026-06-13", "stays pending")
	if _, err := s.ApproveActivities(ctx, []int64{a1.ID}); err != nil {
		t.Fatal(err)
	}

	approved, err := s.ListActivitiesRange(ctx, "2026-06-01", "2026-06-30", "approved")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(approved) != 1 {
		t.Fatalf("expected 1 approved, got %d", len(approved))
	}
	if approved[0].Status != "approved" {
		t.Errorf("expected status=approved, got %q", approved[0].Status)
	}
}

// TestListActivitiesRange_Ordering verifies results come back ordered by
// date ASC then created_at ASC, matching the design's interface contract.
func TestListActivitiesRange_Ordering(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	mustCreateActivity(t, s, ctx, "2026-06-20", "second")
	mustCreateActivity(t, s, ctx, "2026-06-10", "first")
	mustCreateActivity(t, s, ctx, "2026-06-15", "middle")

	list, err := s.ListActivitiesRange(ctx, "2026-06-01", "2026-06-30", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 results, got %d", len(list))
	}
	wantDates := []string{"2026-06-10", "2026-06-15", "2026-06-20"}
	for i, want := range wantDates {
		if list[i].Date != want {
			t.Errorf("index %d: expected date %q, got %q", i, want, list[i].Date)
		}
	}
}

// TestListActivitiesRange_EmptyRangeReturnsEmptySlice verifies a range with
// no matching activities returns a non-nil empty slice, matching the spec's
// "Empty range still produces a valid file" scenario at the store layer.
func TestListActivitiesRange_EmptyRangeReturnsEmptySlice(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	list, err := s.ListActivitiesRange(ctx, "2026-01-01", "2026-01-31", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if list == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(list) != 0 {
		t.Errorf("expected 0 results, got %d", len(list))
	}
}

// mustCreateActivity is a small local helper to reduce boilerplate across the
// range-query tests above.
func mustCreateActivity(t *testing.T, s *Store, ctx context.Context, date, registroDiario string) *DailyActivity {
	t.Helper()
	a, err := s.CreateActivity(ctx, date, 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", registroDiario, "manual")
	if err != nil {
		t.Fatalf("CreateActivity(%q): %v", date, err)
	}
	return a
}
