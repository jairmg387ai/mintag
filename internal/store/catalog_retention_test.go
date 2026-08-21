package store

import (
	"context"
	"testing"
	"time"
)

func intPtr(n int) *int { return &n }

// TestGetCatalogRetentionDays_DefaultsToUnset verifies a fresh store has no
// retention configured for either catalog.
func TestGetCatalogRetentionDays_DefaultsToUnset(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	bugDays, projectDays, err := s.GetCatalogRetentionDays(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if bugDays != nil {
		t.Errorf("expected bugDays=nil, got %v", *bugDays)
	}
	if projectDays != nil {
		t.Errorf("expected projectDays=nil, got %v", *projectDays)
	}
}

// TestSetCatalogRetentionDays_RoundTrips verifies a configured value reads
// back exactly.
func TestSetCatalogRetentionDays_RoundTrips(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.SetCatalogRetentionDays(ctx, intPtr(30), intPtr(90)); err != nil {
		t.Fatal(err)
	}

	bugDays, projectDays, err := s.GetCatalogRetentionDays(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bugDays == nil || *bugDays != 30 {
		t.Errorf("expected bugDays=30, got %v", bugDays)
	}
	if projectDays == nil || *projectDays != 90 {
		t.Errorf("expected projectDays=90, got %v", projectDays)
	}
}

// TestSetCatalogRetentionDays_NilClears verifies passing nil disables a
// previously-configured retention window.
func TestSetCatalogRetentionDays_NilClears(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.SetCatalogRetentionDays(ctx, intPtr(30), intPtr(90)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCatalogRetentionDays(ctx, nil, intPtr(90)); err != nil {
		t.Fatal(err)
	}

	bugDays, projectDays, err := s.GetCatalogRetentionDays(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bugDays != nil {
		t.Errorf("expected bugDays cleared to nil, got %v", *bugDays)
	}
	if projectDays == nil || *projectDays != 90 {
		t.Errorf("expected projectDays to remain 90, got %v", projectDays)
	}
}

// TestSetCatalogRetentionDays_RejectsNonPositive verifies 0 and negative
// values are rejected rather than silently treated as "disabled" (nil is
// the only way to disable).
func TestSetCatalogRetentionDays_RejectsNonPositive(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	for _, days := range []int{0, -1, -100} {
		if err := s.SetCatalogRetentionDays(ctx, intPtr(days), nil); err == nil {
			t.Errorf("expected error for bugDays=%d, got nil", days)
		}
		if err := s.SetCatalogRetentionDays(ctx, nil, intPtr(days)); err == nil {
			t.Errorf("expected error for projectDays=%d, got nil", days)
		}
	}
}

// TestSweepStaleCatalogEntries_SweepsBothWhenConfigured verifies both
// catalogs are swept in a single call and their counts returned separately.
func TestSweepStaleCatalogEntries_SweepsBothWhenConfigured(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	old := time.Now().UTC().AddDate(0, 0, -100).Format(time.RFC3339)

	bug, err := s.AddAzureActivity(ctx, "RUNT2QA", 999030, "Stale Bug", "Bug", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE azure_activities SET created_at = ? WHERE id = ?`, old, bug.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTimelogProject("Stale Project"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE timelog_projects SET created_at = ? WHERE name = ?`, old, "Stale Project"); err != nil {
		t.Fatal(err)
	}

	if err := s.SetCatalogRetentionDays(ctx, intPtr(30), intPtr(30)); err != nil {
		t.Fatal(err)
	}

	bugN, projectN, err := s.SweepStaleCatalogEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bugN != 1 {
		t.Errorf("expected 1 bug deactivated, got %d", bugN)
	}
	if projectN != 1 {
		t.Errorf("expected 1 project deactivated, got %d", projectN)
	}
}

// TestSweepStaleCatalogEntries_NoopWhenBothUnset verifies calling it with no
// retention configured neither errors nor deactivates anything, but still
// updates the last-swept timestamp (checked indirectly via maybeSweepCatalogs
// below).
func TestSweepStaleCatalogEntries_NoopWhenBothUnset(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	bugN, projectN, err := s.SweepStaleCatalogEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bugN != 0 || projectN != 0 {
		t.Errorf("expected no-op (0, 0), got (%d, %d)", bugN, projectN)
	}

	last, ok, err := s.setting(ctx, settingCatalogLastSweptAt)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || last == "" {
		t.Fatal("expected settingCatalogLastSweptAt to be set even when both retention windows are unset")
	}
}

// TestMaybeSweepCatalogs_RunsWhenUnsetThenThrottles verifies the first call
// (no last-swept marker) runs the sweep, and an immediately-following call
// does not re-run it (throttled within catalogSweepInterval).
func TestMaybeSweepCatalogs_RunsWhenUnsetThenThrottles(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	old := time.Now().UTC().AddDate(0, 0, -100).Format(time.RFC3339)
	bug, err := s.AddAzureActivity(ctx, "RUNT2QA", 999031, "Stale Bug", "Bug", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE azure_activities SET created_at = ? WHERE id = ?`, old, bug.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCatalogRetentionDays(ctx, intPtr(30), nil); err != nil {
		t.Fatal(err)
	}

	if err := s.maybeSweepCatalogs(ctx); err != nil {
		t.Fatal(err)
	}
	all, err := s.ListAzureActivities(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range all {
		if a.ID == bug.ID && a.IsActive {
			t.Fatal("expected the first maybeSweepCatalogs call to deactivate the stale bug")
		}
	}

	// Manually reactivate to detect whether a second, immediate call re-sweeps.
	if _, err := s.db.ExecContext(ctx, `UPDATE azure_activities SET is_active = 1 WHERE id = ?`, bug.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.maybeSweepCatalogs(ctx); err != nil {
		t.Fatal(err)
	}
	all, err = s.ListAzureActivities(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range all {
		if a.ID == bug.ID && !a.IsActive {
			t.Fatal("expected the throttled second call to NOT re-run the sweep within catalogSweepInterval")
		}
	}
}

// TestMaybeSweepCatalogs_RunsAgainAfterInterval verifies a stale
// last-swept marker (older than catalogSweepInterval) allows the sweep to
// run again.
func TestMaybeSweepCatalogs_RunsAgainAfterInterval(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	old := time.Now().UTC().AddDate(0, 0, -100).Format(time.RFC3339)
	bug, err := s.AddAzureActivity(ctx, "RUNT2QA", 999032, "Stale Bug", "Bug", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE azure_activities SET created_at = ? WHERE id = ?`, old, bug.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCatalogRetentionDays(ctx, intPtr(30), nil); err != nil {
		t.Fatal(err)
	}

	// Simulate a last sweep from beyond catalogSweepInterval ago.
	staleMarker := time.Now().UTC().Add(-2 * catalogSweepInterval).Format(time.RFC3339)
	if err := s.setSetting(ctx, settingCatalogLastSweptAt, staleMarker); err != nil {
		t.Fatal(err)
	}

	if err := s.maybeSweepCatalogs(ctx); err != nil {
		t.Fatal(err)
	}
	all, err := s.ListAzureActivities(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range all {
		if a.ID == bug.ID && a.IsActive {
			t.Fatal("expected the sweep to run again once the last-swept marker is older than catalogSweepInterval")
		}
	}
}
