package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestSeedCatalogs_Idempotent(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Seed once manually.
	if err := s.seedCatalogs(); err != nil {
		t.Fatalf("first seed: %v", err)
	}

	ctx := context.Background()
	projects, err := s.ListTimelogProjects(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	initialCount := len(projects)
	if initialCount == 0 {
		t.Fatal("expected projects after seeding, got 0")
	}

	// Seed again — row count must be unchanged.
	if err := s.seedCatalogs(); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	projects2, err := s.ListTimelogProjects(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects2) != initialCount {
		t.Errorf("expected %d projects after re-seed, got %d", initialCount, len(projects2))
	}

	categories, err := s.ListTimelogCategories(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) == 0 {
		t.Fatal("expected categories after seeding, got 0")
	}
}

func TestListTimelogProjects_OrderedAlphabetically(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.seedCatalogs(); err != nil {
		t.Fatal(err)
	}

	projects, err := s.ListTimelogProjects(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(projects); i++ {
		if projects[i] < projects[i-1] {
			t.Errorf("projects not ordered at index %d: %q before %q", i, projects[i-1], projects[i])
		}
	}
}

// TestAddTimelogProject_SetsCreatedAt verifies AddTimelogProject stamps
// created_at on insert (used as the SweepStaleTimelogProjects fallback via
// COALESCE(last_used_at, created_at) when a project was never touched).
func TestAddTimelogProject_SetsCreatedAt(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.AddTimelogProject("New Project"); err != nil {
		t.Fatal(err)
	}

	var createdAt sql.NullString
	if err := s.db.QueryRow(`SELECT created_at FROM timelog_projects WHERE name = ?`, "New Project").Scan(&createdAt); err != nil {
		t.Fatal(err)
	}
	if !createdAt.Valid || createdAt.String == "" {
		t.Fatal("expected non-empty created_at after AddTimelogProject")
	}
}

// TestListTimelogProjects_ExcludesInactiveUnlessRequested verifies the new
// includeInactive parameter filters is_active=0 rows by default.
func TestListTimelogProjects_ExcludesInactiveUnlessRequested(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.AddTimelogProject("Active Project"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTimelogProject("Inactive Project"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeactivateTimelogProject(ctx, "Inactive Project"); err != nil {
		t.Fatal(err)
	}

	activeOnly, err := s.ListTimelogProjects(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if contains(activeOnly, "Inactive Project") {
		t.Errorf("expected Inactive Project excluded by default, got %v", activeOnly)
	}
	if !contains(activeOnly, "Active Project") {
		t.Errorf("expected Active Project included, got %v", activeOnly)
	}

	all, err := s.ListTimelogProjects(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(all, "Inactive Project") {
		t.Errorf("expected Inactive Project included with includeInactive=true, got %v", all)
	}
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

// TestDeactivateTimelogProject_NotFound verifies deactivating an unknown
// project name returns a "not found"-style error.
func TestDeactivateTimelogProject_NotFound(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	err = s.DeactivateTimelogProject(context.Background(), "Does Not Exist")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}

// TestRenameTimelogProject_RenamesEntry verifies a rename updates the
// catalog entry's name in place, preserving is_active.
func TestRenameTimelogProject_RenamesEntry(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.AddTimelogProject("Old Name"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameTimelogProject(ctx, "Old Name", "New Name"); err != nil {
		t.Fatal(err)
	}

	all, err := s.ListTimelogProjects(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if contains(all, "Old Name") {
		t.Error("expected Old Name gone after rename")
	}
	if !contains(all, "New Name") {
		t.Error("expected New Name present after rename")
	}
}

// TestRenameTimelogProject_NotFound verifies renaming an unknown project name
// returns a "not found"-style error.
func TestRenameTimelogProject_NotFound(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	err = s.RenameTimelogProject(context.Background(), "Does Not Exist", "New Name")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}

// TestRenameTimelogProject_RejectsEmptyName verifies a blank/whitespace-only
// new name is rejected instead of silently renaming to "".
func TestRenameTimelogProject_RejectsEmptyName(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.AddTimelogProject("Some Project"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameTimelogProject(context.Background(), "Some Project", "   "); err == nil {
		t.Fatal("expected error renaming to a blank name")
	}
}

// TestRenameTimelogProject_RejectsDuplicateName verifies renaming to a name
// already taken by another project is rejected rather than violating the
// table's UNIQUE constraint.
func TestRenameTimelogProject_RejectsDuplicateName(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.AddTimelogProject("Project A"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTimelogProject("Project B"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameTimelogProject(ctx, "Project A", "Project B"); err == nil {
		t.Fatal("expected error renaming to an already-taken name")
	}
}

// TestTouchTimelogProjectLastUsed_UnknownProjectIsNotAnError verifies
// touching a project name that was never added to the catalog (project is
// plain free TEXT on daily_activities, not a FK) is a silent no-op.
func TestTouchTimelogProjectLastUsed_UnknownProjectIsNotAnError(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.TouchTimelogProjectLastUsed(context.Background(), "Never Added"); err != nil {
		t.Fatalf("expected no error touching an uncataloged project, got %v", err)
	}
}

// TestSweepStaleTimelogProjects_DeactivatesOnlyStaleEntries verifies the
// sweep deactivates only projects whose last_used_at (or created_at, when
// never touched) predates the retention cutoff, and leaves recently-used
// projects alone.
func TestSweepStaleTimelogProjects_DeactivatesOnlyStaleEntries(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.AddTimelogProject("Stale Project"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTimelogProject("Fresh Project"); err != nil {
		t.Fatal(err)
	}

	old := time.Now().UTC().AddDate(0, 0, -100).Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE timelog_projects SET created_at = ?, last_used_at = NULL WHERE name = ?`, old, "Stale Project"); err != nil {
		t.Fatal(err)
	}

	n, err := s.SweepStaleTimelogProjects(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deactivated, got %d", n)
	}

	activeOnly, err := s.ListTimelogProjects(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if contains(activeOnly, "Stale Project") {
		t.Error("expected Stale Project deactivated by sweep")
	}
	if !contains(activeOnly, "Fresh Project") {
		t.Error("expected Fresh Project to remain active")
	}
}

// TestSweepStaleTimelogProjects_DisabledWhenRetentionDaysNotPositive
// verifies retentionDays<=0 is a no-op, per the "disabled" convention shared
// with SweepStaleBugActivities.
func TestSweepStaleTimelogProjects_DisabledWhenRetentionDaysNotPositive(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.AddTimelogProject("Ancient Project"); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().AddDate(0, 0, -3650).Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE timelog_projects SET created_at = ? WHERE name = ?`, old, "Ancient Project"); err != nil {
		t.Fatal(err)
	}

	for _, days := range []int{0, -5} {
		n, err := s.SweepStaleTimelogProjects(ctx, days)
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("retentionDays=%d: expected 0 deactivated, got %d", days, n)
		}
	}
}

func TestListTimelogCategories_OrderedAlphabetically(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.seedCatalogs(); err != nil {
		t.Fatal(err)
	}

	cats, err := s.ListTimelogCategories(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(cats); i++ {
		if cats[i].Name < cats[i-1].Name {
			t.Errorf("categories not ordered at index %d: %q before %q", i, cats[i-1].Name, cats[i].Name)
		}
	}
}

// TestListTimelogCategories_ExcludesInactiveUnlessRequested verifies the
// includeInactive parameter filters is_active=0 rows by default, mirroring
// TestListTimelogProjects_ExcludesInactiveUnlessRequested.
func TestListTimelogCategories_ExcludesInactiveUnlessRequested(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.AddTimelogCategory("Active Category", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTimelogCategory("Inactive Category", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.DeactivateTimelogCategory(ctx, "Inactive Category"); err != nil {
		t.Fatal(err)
	}

	activeOnly, err := s.ListTimelogCategories(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range activeOnly {
		if c.Name == "Inactive Category" {
			t.Errorf("expected Inactive Category excluded by default, got %v", activeOnly)
		}
	}

	all, err := s.ListTimelogCategories(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range all {
		if c.Name == "Inactive Category" {
			found = true
			if c.IsActive {
				t.Errorf("expected Inactive Category to have is_active=false")
			}
		}
	}
	if !found {
		t.Errorf("expected Inactive Category included with includeInactive=true, got %v", all)
	}
}

// TestDeactivateTimelogCategory_NotFound verifies deactivating an unknown
// category name returns a "not found"-style error, mirroring
// TestDeactivateTimelogProject_NotFound.
func TestDeactivateTimelogCategory_NotFound(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	err = s.DeactivateTimelogCategory(context.Background(), "Does Not Exist")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}

// TestReactivateTimelogProject_FlipsInactiveBackToActive verifies
// ReactivateTimelogProject undoes DeactivateTimelogProject, and that an
// unknown name returns a "not found"-style error.
func TestReactivateTimelogProject_FlipsInactiveBackToActive(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.AddTimelogProject("Toggle Project"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeactivateTimelogProject(ctx, "Toggle Project"); err != nil {
		t.Fatal(err)
	}

	if err := s.ReactivateTimelogProject(ctx, "Toggle Project"); err != nil {
		t.Fatal(err)
	}

	activeOnly, err := s.ListTimelogProjects(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(activeOnly, "Toggle Project") {
		t.Errorf("expected Toggle Project active again, got %v", activeOnly)
	}

	err = s.ReactivateTimelogProject(ctx, "Does Not Exist")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}

// TestReactivateTimelogCategory_FlipsInactiveBackToActive verifies
// ReactivateTimelogCategory undoes DeactivateTimelogCategory, and that an
// unknown name returns a "not found"-style error.
func TestReactivateTimelogCategory_FlipsInactiveBackToActive(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.AddTimelogCategory("Toggle Category", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.DeactivateTimelogCategory(ctx, "Toggle Category"); err != nil {
		t.Fatal(err)
	}

	if err := s.ReactivateTimelogCategory(ctx, "Toggle Category"); err != nil {
		t.Fatal(err)
	}

	activeOnly, err := s.ListTimelogCategories(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range activeOnly {
		if c.Name == "Toggle Category" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Toggle Category active again, got %v", activeOnly)
	}

	err = s.ReactivateTimelogCategory(ctx, "Does Not Exist")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}
