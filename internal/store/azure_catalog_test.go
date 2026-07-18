package store

import (
	"context"
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

	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999001, "QA Activity")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.IsDefault {
		t.Error("expected first added activity to become default automatically")
	}
	if !a.IsActive {
		t.Error("expected new activity to be active")
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
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999002, "Second Activity")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.IsDefault {
		t.Error("expected second added activity to NOT become default when one already exists")
	}
}

func TestListAzureActivities_ExcludesInactiveUnlessRequested(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	extra, err := s.AddAzureActivity(ctx, "RUNT2QA", 999003, "To Deactivate")
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
	a, err := s.AddAzureActivity(ctx, "RUNT2QA", 999004, "Old Label")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpdateAzureActivity(ctx, a.ID, "RUNT2QA-NEW", "New Label")
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

	azureActivity, err := s.AddAzureActivity(ctx, "RUNT2QA", 999005, "Assigned Activity")
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
	inactive, err := s.AddAzureActivity(ctx, "RUNT2QA", 999007, "Inactive Assignable")
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
	azureActivity, err := s.AddAzureActivity(ctx, "RUNT2QA", 999006, "Listed Activity")
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
