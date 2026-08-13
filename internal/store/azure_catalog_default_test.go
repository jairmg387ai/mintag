package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

// --- Default invariant (task 2.1) ---

func TestSetDefaultAzureActivity_ClearsPreviousDefault(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	// The migration seed (A) is already the default.
	seedDefault, err := s.GetDefaultAzureActivity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.AddAzureActivity(ctx, "RUNT2QA", 999101, "Activity B", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.SetDefaultAzureActivity(ctx, b.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all, err := s.ListAzureActivities(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	var defaultCount int
	var bIsDefault bool
	for _, a := range all {
		if a.IsDefault {
			defaultCount++
		}
		if a.ID == b.ID && a.IsDefault {
			bIsDefault = true
		}
		if a.ID == seedDefault.ID && a.IsDefault {
			t.Errorf("expected previous default (id=%d) to be cleared", seedDefault.ID)
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly one default after SetDefaultAzureActivity, got %d", defaultCount)
	}
	if !bIsDefault {
		t.Fatal("expected activity B to be the new default")
	}
}

func TestSetDefaultAzureActivity_RejectsMissingID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	seedDefault, err := s.GetDefaultAzureActivity(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.SetDefaultAzureActivity(ctx, 999999); err == nil {
		t.Fatal("expected error when setting default to a missing id, got nil")
	}

	stillDefault, err := s.GetDefaultAzureActivity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stillDefault.ID != seedDefault.ID {
		t.Fatalf("expected default to remain %d after rejected call, got %d", seedDefault.ID, stillDefault.ID)
	}
}

func TestSetDefaultAzureActivity_RejectsInactiveID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	seedDefault, err := s.GetDefaultAzureActivity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	inactive, err := s.AddAzureActivity(ctx, "RUNT2QA", 999102, "Inactive Activity", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeactivateAzureActivity(ctx, inactive.ID); err != nil {
		t.Fatal(err)
	}

	if err := s.SetDefaultAzureActivity(ctx, inactive.ID); err == nil {
		t.Fatal("expected error when setting an inactive activity as default, got nil")
	}

	stillDefault, err := s.GetDefaultAzureActivity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stillDefault.ID != seedDefault.ID {
		t.Fatalf("expected default to remain %d after rejected call, got %d", seedDefault.ID, stillDefault.ID)
	}
}

// --- Soft-delete guard on the default (task 2.2) ---

func TestDeactivateAzureActivity_BlocksWhenTargetIsDefault(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	def, err := s.GetDefaultAzureActivity(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = s.DeactivateAzureActivity(ctx, def.ID)
	if err == nil {
		t.Fatal("expected error when deactivating the current default, got nil")
	}

	got, err := s.getAzureActivity(ctx, def.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsActive {
		t.Error("expected default activity to remain active after blocked deactivation")
	}
	if !got.IsDefault {
		t.Error("expected default activity to remain default after blocked deactivation")
	}
}

// TestDeactivateAzureActivity_ConcurrentWithSetDefault_NeverLeavesInactiveDefault
// reproduces the TOCTOU race between DeactivateAzureActivity and
// SetDefaultAzureActivity: for each of several non-default activities, one
// goroutine tries to promote it to default while another concurrently tries
// to deactivate it. Both operations are transactional, so SQLite serializes
// them — exactly one of the two must win per activity, and the other must
// fail cleanly (no state where is_default=1 and is_active=0 is ever
// observable). Uses a real file-backed DB (Open()) so the goroutines can
// actually contend for SQLite's writer lock instead of being serialized by
// Go's connection pool first.
func TestDeactivateAzureActivity_ConcurrentWithSetDefault_NeverLeavesInactiveDefault(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mintag.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	const n = 5
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999400+i, "Race Activity", "")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, id := range ids {
		wg.Add(2)
		go func(activityID int64) {
			defer wg.Done()
			<-start
			// Result intentionally ignored: either outcome (won or lost the
			// race) is acceptable as long as it errors cleanly instead of
			// corrupting state.
			_ = s.SetDefaultAzureActivity(ctx, activityID)
		}(id)
		go func(activityID int64) {
			defer wg.Done()
			<-start
			_ = s.DeactivateAzureActivity(ctx, activityID)
		}(id)
	}
	close(start)
	wg.Wait()

	var inconsistent int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM azure_activities WHERE is_default = 1 AND is_active = 0`,
	).Scan(&inconsistent)
	if err != nil {
		t.Fatal(err)
	}
	if inconsistent != 0 {
		t.Fatalf("invariant violated: found %d row(s) with is_default=1 AND is_active=0", inconsistent)
	}

	var defaultCount int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM azure_activities WHERE is_default = 1`).Scan(&defaultCount)
	if err != nil {
		t.Fatal(err)
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly one default after concurrent set/deactivate races, got %d", defaultCount)
	}
}

// --- Concurrency (task 2.3) ---

// TestSetDefaultAzureActivity_ConcurrentRequestsYieldExactlyOneDefault runs
// against OpenInMemory(), which calls db.SetMaxOpenConns(1) — so these
// goroutines are serialized by Go's connection pool before any of them reach
// SQLite, and there is never real engine-level contention here. It still
// validates the logical invariant (exactly one default survives N concurrent
// calls), but TestSetDefaultAzureActivity_ConcurrentRequestsRealDB below is
// the test that actually exercises concurrent SQLite writers and the
// _busy_timeout configured in Open().
func TestSetDefaultAzureActivity_ConcurrentRequestsYieldExactlyOneDefault(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	const n = 8
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999200+i, "Concurrent Activity", "")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i, id := range ids {
		wg.Add(1)
		go func(idx int, activityID int64) {
			defer wg.Done()
			errs[idx] = s.SetDefaultAzureActivity(ctx, activityID)
		}(i, id)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: unexpected error: %v", i, err)
		}
	}

	all, err := s.ListAzureActivities(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	var defaultCount int
	for _, a := range all {
		if a.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly one default after %d concurrent SetDefaultAzureActivity calls, got %d", n, defaultCount)
	}
}

// TestSetDefaultAzureActivity_ConcurrentRequestsRealDB uses a real,
// file-backed SQLite database opened via Open() (production path, default
// connection pool — no SetMaxOpenConns(1)), so the concurrent goroutines can
// actually contend for SQLite's writer lock instead of being serialized by
// Go's connection pool first. This exercises the busy_timeout + txlock=immediate
// pragmas configured in Open(): without them, a second concurrent writer
// fails immediately with SQLITE_BUSY instead of waiting for the first writer
// to commit.
func TestSetDefaultAzureActivity_ConcurrentRequestsRealDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mintag.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	const n = 8
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999300+i, "Real DB Concurrent Activity", "")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i, id := range ids {
		wg.Add(1)
		go func(idx int, activityID int64) {
			defer wg.Done()
			errs[idx] = s.SetDefaultAzureActivity(ctx, activityID)
		}(i, id)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: unexpected error (busy_timeout should have absorbed contention): %v", i, err)
		}
	}

	all, err := s.ListAzureActivities(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	var defaultCount int
	for _, a := range all {
		if a.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly one default after %d concurrent SetDefaultAzureActivity calls against a real DB, got %d", n, defaultCount)
	}
}
