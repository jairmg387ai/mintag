package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestAddAzureActivity_SetsCreatedAt verifies AddAzureActivity stamps
// created_at on insert (used as the SweepStaleBugActivities fallback via
// COALESCE(last_used_at, created_at) when an entry was never touched).
func TestAddAzureActivity_SetsCreatedAt(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999010, "New Bug", "Bug", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	var createdAt sql.NullString
	if err := s.db.QueryRow(`SELECT created_at FROM azure_activities WHERE id = ?`, a.ID).Scan(&createdAt); err != nil {
		t.Fatal(err)
	}
	if !createdAt.Valid || createdAt.String == "" {
		t.Fatal("expected non-empty created_at after AddAzureActivity")
	}
}

// TestMigrate_BackfillsCreatedAtForPreExistingRows verifies the seeded
// default activity (created before this feature existed, from the caller's
// perspective) gets a non-NULL created_at from the migration's backfill
// pass, not just newly-inserted rows.
func TestMigrate_BackfillsCreatedAtForPreExistingRows(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var createdAt sql.NullString
	if err := s.db.QueryRow(`SELECT created_at FROM azure_activities WHERE is_default = 1`).Scan(&createdAt); err != nil {
		t.Fatal(err)
	}
	if !createdAt.Valid || createdAt.String == "" {
		t.Fatal("expected the migration-seeded default activity to have a backfilled created_at")
	}
}

// TestTouchAzureActivityLastUsed_UnknownIDIsNotAnError mirrors
// TouchTimelogProjectLastUsed's not-an-error convention for a missing target.
func TestTouchAzureActivityLastUsed_UnknownIDIsNotAnError(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.TouchAzureActivityLastUsed(context.Background(), 999999); err != nil {
		t.Fatalf("expected no error touching an unknown id, got %v", err)
	}
}

// TestTouchAzureActivityLastUsed_SetsLastUsedAt verifies the timestamp is
// actually written and round-trips.
func TestTouchAzureActivityLastUsed_SetsLastUsedAt(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999011, "Touch Me", "Bug", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.TouchAzureActivityLastUsed(ctx, a.ID); err != nil {
		t.Fatal(err)
	}

	var lastUsedAt sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT last_used_at FROM azure_activities WHERE id = ?`, a.ID).Scan(&lastUsedAt); err != nil {
		t.Fatal(err)
	}
	if !lastUsedAt.Valid || lastUsedAt.String == "" {
		t.Fatal("expected last_used_at to be set after TouchAzureActivityLastUsed")
	}
}

// TestSweepStaleBugActivities_OnlyDeactivatesStaleBugs verifies the sweep:
//   - deactivates a stale Bug-type entry
//   - leaves a stale Task-type entry alone (e.g. "Vacaciones" — legitimately
//     used rarely, must never be auto-deactivated)
//   - leaves a recently-used Bug-type entry alone
//   - never deactivates the current default, even if it is a stale Bug
func TestSweepStaleBugActivities_OnlyDeactivatesStaleBugs(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	old := time.Now().UTC().AddDate(0, 0, -100).Format(time.RFC3339)

	staleBug, err := s.AddAzureActivity(ctx, "RUNT2QA", 999020, "Stale Bug", "Bug", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	staleTask, err := s.AddAzureActivity(ctx, "RUNT2QA", 999021, "Vacaciones", "Task", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	freshBug, err := s.AddAzureActivity(ctx, "RUNT2QA", 999022, "Fresh Bug", "Bug", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []int64{staleBug.ID, staleTask.ID} {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE azure_activities SET created_at = ?, last_used_at = NULL WHERE id = ?`, old, id,
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.TouchAzureActivityLastUsed(ctx, freshBug.ID); err != nil {
		t.Fatal(err)
	}

	// Make the stale bug the current default, and stale-age it too, to prove
	// the default exemption survives the sweep.
	if err := s.SetDefaultAzureActivity(ctx, staleBug.ID); err != nil {
		t.Fatal(err)
	}

	n, err := s.SweepStaleBugActivities(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0 deactivated (the only stale bug is the default), got %d", n)
	}

	// Promote the fresh bug to default instead, freeing the stale bug up to
	// be swept.
	if err := s.SetDefaultAzureActivity(ctx, freshBug.ID); err != nil {
		t.Fatal(err)
	}

	n, err = s.SweepStaleBugActivities(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deactivated, got %d", n)
	}

	all, err := s.ListAzureActivities(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[int64]bool, len(all))
	for _, a := range all {
		byID[a.ID] = a.IsActive
	}
	if byID[staleBug.ID] {
		t.Error("expected stale Bug to be deactivated")
	}
	if !byID[staleTask.ID] {
		t.Error("expected stale Task (Vacaciones) to remain active — Task types are never auto-deactivated")
	}
	if !byID[freshBug.ID] {
		t.Error("expected recently-used Bug to remain active")
	}
}

// TestSweepStaleBugActivities_DisabledWhenRetentionDaysNotPositive verifies
// retentionDays<=0 short-circuits without querying.
func TestSweepStaleBugActivities_DisabledWhenRetentionDaysNotPositive(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	old := time.Now().UTC().AddDate(0, 0, -3650).Format(time.RFC3339)
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999023, "Ancient Bug", "Bug", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetDefaultAzureActivity(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	// Clear default status so it's eligible, then age it.
	if _, err := s.db.ExecContext(ctx, `UPDATE azure_activities SET is_default = 0, created_at = ? WHERE id = ?`, old, a.ID); err != nil {
		t.Fatal(err)
	}

	for _, days := range []int{0, -1} {
		n, err := s.SweepStaleBugActivities(ctx, days)
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("retentionDays=%d: expected 0 deactivated, got %d", days, n)
		}
	}
}
