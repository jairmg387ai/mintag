package store

import (
	"context"
	"errors"
	"testing"
)

// --- Migration & seed (tasks 1.1-1.3) ---

func TestMigrate_SeedsDefaultAzureActivity(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	all, err := s.ListAzureActivities(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected exactly one seeded activity, got %d", len(all))
	}
	seed := all[0]
	if seed.Org != "RUNT2PSW" {
		t.Errorf("expected seed org=RUNT2PSW, got %q", seed.Org)
	}
	if seed.WorkItemID != 156263 {
		t.Errorf("expected seed work_item_id=156263, got %d", seed.WorkItemID)
	}
	if !seed.IsActive {
		t.Error("expected seed activity to be active")
	}
	if !seed.IsDefault {
		t.Error("expected seed activity to be default")
	}
}

func TestMigrate_SeedIsIdempotent(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// migrate() already ran once inside OpenInMemory(); re-run it directly to
	// simulate a restart against an existing database.
	if err := s.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	ctx := context.Background()
	all, err := s.ListAzureActivities(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected re-running migrate to not duplicate the seed row, got %d rows", len(all))
	}
}

func TestMigrate_DailyActivitiesGetsAzureActivityIDColumn(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if !columnExists(t, s.db, "daily_activities", "azure_activity_id") {
		t.Fatal("expected daily_activities.azure_activity_id column to exist")
	}
}

// --- Catalog CRUD (task 1.4) ---

func TestAddAzureActivity_FirstRowIsAutoDefault(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	// Clear the seeded default so this test observes "first row" behavior in isolation.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM azure_activities`); err != nil {
		t.Fatal(err)
	}

	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999001, "QA Activity", "Bug", AzureActivityMapping{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.IsDefault {
		t.Error("expected first added activity to become default automatically")
	}
	if !a.IsActive {
		t.Error("expected new activity to be active")
	}
	if a.WorkItemType != "Bug" {
		t.Errorf("expected work_item_type=Bug, got %q", a.WorkItemType)
	}
}

func TestAddAzureActivity_SecondRowIsNotDefault(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	// A default already exists from the migration seed.
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999002, "Second Activity", "", AzureActivityMapping{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.IsDefault {
		t.Error("expected second added activity to NOT become default when one already exists")
	}
}

func TestFindAzureActivityByWorkItemID_Found(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	added, err := s.AddAzureActivity(ctx, "RUNT2QA", 999004, "Find Me", "Task", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	found, err := s.FindAzureActivityByWorkItemID(ctx, 999004)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != added.ID {
		t.Errorf("expected to find catalog row %d, got %d", added.ID, found.ID)
	}
}

func TestFindAzureActivityByWorkItemID_NotFound(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, err = s.FindAzureActivityByWorkItemID(context.Background(), 424242)
	if !errors.Is(err, ErrAzureActivityNotFound) {
		t.Fatalf("expected ErrAzureActivityNotFound, got %v", err)
	}
}

func TestReassignAzureActivityWorkItem_UpdatesWorkItemID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	added, err := s.AddAzureActivity(ctx, "RUNT2QA", 999005, "Reassign Me", "Task", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.ReassignAzureActivityWorkItem(ctx, added.ID, 999006)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.WorkItemID != 999006 {
		t.Errorf("expected work_item_id=999006, got %d", updated.WorkItemID)
	}
	// Label/project/category/active/default must be untouched by a reassign.
	if updated.Label != "Reassign Me" {
		t.Errorf("expected label to remain unchanged, got %q", updated.Label)
	}

	// The old work item id must no longer resolve.
	if _, err := s.FindAzureActivityByWorkItemID(ctx, 999005); !errors.Is(err, ErrAzureActivityNotFound) {
		t.Errorf("expected old work_item_id to no longer resolve, got %v", err)
	}
}

func TestReassignAzureActivityWorkItem_RejectsNonPositiveID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	added, err := s.AddAzureActivity(ctx, "RUNT2QA", 999007, "Reject Me", "Task", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.ReassignAzureActivityWorkItem(ctx, added.ID, 0); err == nil {
		t.Error("expected error for newWorkItemID=0")
	}
	if _, err := s.ReassignAzureActivityWorkItem(ctx, added.ID, -5); err == nil {
		t.Error("expected error for negative newWorkItemID")
	}
}

func TestReassignAzureActivityWorkItem_NotFound(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, err = s.ReassignAzureActivityWorkItem(context.Background(), 999999, 111)
	if err == nil {
		t.Error("expected error for a non-existent catalog id")
	}
}

func TestListAzureActivities_ExcludesInactiveUnlessRequested(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	extra, err := s.AddAzureActivity(ctx, "RUNT2QA", 999003, "To Deactivate", "", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeactivateAzureActivity(ctx, extra.ID); err != nil {
		t.Fatal(err)
	}

	activeOnly, err := s.ListAzureActivities(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range activeOnly {
		if a.ID == extra.ID {
			t.Errorf("expected deactivated activity %d to be excluded from active-only list", extra.ID)
		}
	}

	all, err := s.ListAzureActivities(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, a := range all {
		if a.ID == extra.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected deactivated activity %d to still appear when includeInactive=true", extra.ID)
	}
}

func TestUpdateAzureActivity_ChangesOrgAndLabelOnly(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999004, "Old Label", "", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpdateAzureActivity(ctx, a.ID, "RUNT2QA-NEW", "New Label", "", AzureActivityMapping{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Label != "New Label" {
		t.Errorf("expected label=New Label, got %q", updated.Label)
	}
	if updated.Org != "RUNT2QA-NEW" {
		t.Errorf("expected org=RUNT2QA-NEW, got %q", updated.Org)
	}
	if updated.WorkItemID != 999004 {
		t.Errorf("expected work_item_id unchanged at 999004, got %d", updated.WorkItemID)
	}
}

// TestGetAzureActivity_ReturnsRowByID verifies the public getter round-trips
// a catalog entry by id and errors for a missing one.
func TestGetAzureActivity_ReturnsRowByID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	created, err := s.AddAzureActivity(ctx, "RUNT2QA", 999040, "Test WI", "Task", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetAzureActivity(ctx, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkItemID != 999040 {
		t.Errorf("expected work_item_id=999040, got %d", got.WorkItemID)
	}

	if _, err := s.GetAzureActivity(ctx, 999999); err == nil {
		t.Fatal("expected an error for a non-existent id")
	}
}

func TestGetDefaultAzureActivity_ReturnsTheDefaultRow(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	def, err := s.GetDefaultAzureActivity(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.WorkItemID != 156263 {
		t.Errorf("expected the migration-seeded default work_item_id=156263, got %d", def.WorkItemID)
	}
}

// --- DailyActivity FK wiring (tasks 1.5-1.6) ---

func TestSetActivityAzureActivity_PersistsAndClearsFK(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	act, err := s.CreateActivity(ctx, "2026-06-12", 1.5, "RNCEA", "Development", "Some work", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if act.AzureActivityID != nil {
		t.Fatal("expected new activity to start with nil azure_activity_id")
	}

	azureActivity, err := s.AddAzureActivity(ctx, "RUNT2QA", 999005, "Assigned Activity", "", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.SetActivityAzureActivity(ctx, act.ID, &azureActivity.ID); err != nil {
		t.Fatalf("unexpected error setting FK: %v", err)
	}
	got, err := s.GetActivity(ctx, act.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AzureActivityID == nil || *got.AzureActivityID != azureActivity.ID {
		t.Fatalf("expected azure_activity_id=%d, got %v", azureActivity.ID, got.AzureActivityID)
	}

	// Clearing back to nil must also work.
	if err := s.SetActivityAzureActivity(ctx, act.ID, nil); err != nil {
		t.Fatalf("unexpected error clearing FK: %v", err)
	}
	cleared, err := s.GetActivity(ctx, act.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.AzureActivityID != nil {
		t.Fatalf("expected azure_activity_id to be cleared to nil, got %v", *cleared.AzureActivityID)
	}
}

func TestSetActivityAzureActivity_RejectsMissingAzureActivityID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	act, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Development", "Some work", "manual")
	if err != nil {
		t.Fatal(err)
	}

	missingID := int64(999999)
	if err := s.SetActivityAzureActivity(ctx, act.ID, &missingID); err == nil {
		t.Fatal("expected error when assigning a nonexistent azure_activity_id, got nil")
	}

	got, err := s.GetActivity(ctx, act.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AzureActivityID != nil {
		t.Fatalf("expected azure_activity_id to remain nil after rejected assignment, got %v", *got.AzureActivityID)
	}
}

func TestSetActivityAzureActivity_RejectsInactiveAzureActivityID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	act, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Development", "Some work", "manual")
	if err != nil {
		t.Fatal(err)
	}
	inactive, err := s.AddAzureActivity(ctx, "RUNT2QA", 999007, "Inactive Assignable", "", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeactivateAzureActivity(ctx, inactive.ID); err != nil {
		t.Fatal(err)
	}

	if err := s.SetActivityAzureActivity(ctx, act.ID, &inactive.ID); err == nil {
		t.Fatal("expected error when assigning an inactive azure_activity_id, got nil")
	}

	got, err := s.GetActivity(ctx, act.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AzureActivityID != nil {
		t.Fatalf("expected azure_activity_id to remain nil after rejected assignment, got %v", *got.AzureActivityID)
	}
}

func TestListActivities_IncludesAzureActivityID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	act, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Development", "Work", "manual")
	if err != nil {
		t.Fatal(err)
	}
	azureActivity, err := s.AddAzureActivity(ctx, "RUNT2QA", 999006, "Listed Activity", "", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetActivityAzureActivity(ctx, act.ID, &azureActivity.ID); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListActivities(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, a := range list {
		if a.ID == act.ID {
			found = true
			if a.AzureActivityID == nil || *a.AzureActivityID != azureActivity.ID {
				t.Fatalf("expected listed activity to carry azure_activity_id=%d, got %v", azureActivity.ID, a.AzureActivityID)
			}
		}
	}
	if !found {
		t.Fatal("expected created activity to appear in ListActivities")
	}
}

// --- AzureActivityMapping round-trip (project + category_id, task 1.2) ---

func mustStrPtr(s string) *string { return &s }

// TestAddAzureActivity_MappingRoundTrip_BothSet verifies AddAzureActivity
// persists both project and category_id when the mapping specifies both.
func TestAddAzureActivity_MappingRoundTrip_BothSet(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	cats, err := s.ListTimelogCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) == 0 {
		t.Fatal("expected seeded categories")
	}
	category := cats[0]

	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999010, "Mapped Both", "",
		AzureActivityMapping{Project: mustStrPtr("Alpha"), CategoryID: &category.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Project == nil || *a.Project != "Alpha" {
		t.Fatalf("expected project=Alpha, got %v", a.Project)
	}
	if a.CategoryID == nil || *a.CategoryID != category.ID {
		t.Fatalf("expected category_id=%d, got %v", category.ID, a.CategoryID)
	}
}

// TestAddAzureActivity_MappingRoundTrip_ProjectOnly verifies a project-only
// mapping leaves category_id nil.
func TestAddAzureActivity_MappingRoundTrip_ProjectOnly(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999011, "Mapped Project Only", "",
		AzureActivityMapping{Project: mustStrPtr("Beta")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Project == nil || *a.Project != "Beta" {
		t.Fatalf("expected project=Beta, got %v", a.Project)
	}
	if a.CategoryID != nil {
		t.Fatalf("expected category_id=nil, got %v", *a.CategoryID)
	}
}

// TestAddAzureActivity_MappingRoundTrip_CategoryOnly verifies a category-only
// mapping leaves project nil.
func TestAddAzureActivity_MappingRoundTrip_CategoryOnly(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	cats, err := s.ListTimelogCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) == 0 {
		t.Fatal("expected seeded categories")
	}
	category := cats[0]

	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999012, "Mapped Category Only", "",
		AzureActivityMapping{CategoryID: &category.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Project != nil {
		t.Fatalf("expected project=nil, got %v", *a.Project)
	}
	if a.CategoryID == nil || *a.CategoryID != category.ID {
		t.Fatalf("expected category_id=%d, got %v", category.ID, a.CategoryID)
	}
}

// TestAddAzureActivity_MappingRoundTrip_Neither verifies a zero-value mapping
// (both nil) leaves both fields nil — NULL in the database, not the caller's
// responsibility to special-case.
func TestAddAzureActivity_MappingRoundTrip_Neither(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999013, "Unmapped", "", AzureActivityMapping{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Project != nil {
		t.Fatalf("expected project=nil, got %v", *a.Project)
	}
	if a.CategoryID != nil {
		t.Fatalf("expected category_id=nil, got %v", *a.CategoryID)
	}
}

// TestAddAzureActivity_MappingBlankProjectStoredAsNil verifies a
// whitespace-only project string normalizes to nil (NULL), not an empty
// string, mirroring the trim behavior used elsewhere in this package (e.g.
// AddTimelogProject).
func TestAddAzureActivity_MappingBlankProjectStoredAsNil(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999014, "Blank Project", "",
		AzureActivityMapping{Project: mustStrPtr("   ")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Project != nil {
		t.Fatalf("expected blank/whitespace project to normalize to nil, got %q", *a.Project)
	}
}

// TestAddAzureActivity_MappingRejectsUnknownCategoryID verifies validateMapping
// performs an existence-only check against timelog_categories (no is_active
// column exists on that table — design correction #1).
func TestAddAzureActivity_MappingRejectsUnknownCategoryID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	unknown := int64(999999)
	_, err = s.AddAzureActivity(ctx, "RUNT2QA", 999015, "Bad Category", "",
		AzureActivityMapping{CategoryID: &unknown})
	if err == nil {
		t.Fatal("expected error for unknown category_id")
	}
	wantMsg := "timelog category not found: 999999"
	if err.Error() != wantMsg {
		t.Fatalf("expected error %q, got %q", wantMsg, err.Error())
	}
}

// TestUpdateAzureActivity_MappingRoundTrip_ReplacesMapping verifies
// UpdateAzureActivity is a full replace of the mapping: an omitted
// project/category_id in the call clears any previously-stored mapping.
func TestUpdateAzureActivity_MappingRoundTrip_ReplacesMapping(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	cats, err := s.ListTimelogCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) == 0 {
		t.Fatal("expected seeded categories")
	}
	category := cats[0]

	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999016, "Initially Mapped", "",
		AzureActivityMapping{Project: mustStrPtr("Alpha"), CategoryID: &category.ID})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpdateAzureActivity(ctx, a.ID, "RUNT2QA", "Initially Mapped", "", AzureActivityMapping{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Project != nil {
		t.Fatalf("expected project cleared to nil, got %q", *updated.Project)
	}
	if updated.CategoryID != nil {
		t.Fatalf("expected category_id cleared to nil, got %v", *updated.CategoryID)
	}
}

// TestUpdateAzureActivity_MappingRejectsUnknownCategoryID mirrors the Add
// path's rejection for Update.
func TestUpdateAzureActivity_MappingRejectsUnknownCategoryID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999017, "To Update", "", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	unknown := int64(999999)
	_, err = s.UpdateAzureActivity(ctx, a.ID, "RUNT2QA", "To Update", "", AzureActivityMapping{CategoryID: &unknown})
	if err == nil {
		t.Fatal("expected error for unknown category_id")
	}
}
