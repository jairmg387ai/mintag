package store

import (
	"context"
	"testing"
)

// TestSetCategoryAzureActivity_PersistsAndClears verifies a category's
// azure_activity_id mapping can be assigned and then cleared back to nil.
func TestSetCategoryAzureActivity_PersistsAndClears(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	mustNoErr(t, s.seedCatalogs())

	az, err := s.AddAzureActivity(ctx, "RUNT2QA", 555, "QA Activity")
	mustNoErr(t, err)

	cats, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	if len(cats) == 0 {
		t.Fatal("expected seeded categories")
	}
	target := cats[0]
	if target.AzureActivityID != nil {
		t.Fatalf("expected freshly seeded category to have no mapping, got %#v", target.AzureActivityID)
	}

	mustNoErr(t, s.SetCategoryAzureActivity(ctx, target.ID, &az.ID))

	updated, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	found := false
	for _, c := range updated {
		if c.ID == target.ID {
			found = true
			if c.AzureActivityID == nil || *c.AzureActivityID != az.ID {
				t.Fatalf("expected azure_activity_id=%d, got %#v", az.ID, c.AzureActivityID)
			}
		}
	}
	if !found {
		t.Fatal("target category not found after mapping")
	}

	mustNoErr(t, s.SetCategoryAzureActivity(ctx, target.ID, nil))

	cleared, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	for _, c := range cleared {
		if c.ID == target.ID && c.AzureActivityID != nil {
			t.Fatalf("expected cleared mapping, got %#v", c.AzureActivityID)
		}
	}
}

// TestSetCategoryAzureActivity_RejectsUnknownAzureActivityID mirrors
// SetActivityAzureActivity's app-level FK validation for a non-existent id.
func TestSetCategoryAzureActivity_RejectsUnknownAzureActivityID(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	mustNoErr(t, s.seedCatalogs())

	cats, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	if len(cats) == 0 {
		t.Fatal("expected seeded categories")
	}

	unknown := int64(999999)
	if err := s.SetCategoryAzureActivity(ctx, cats[0].ID, &unknown); err == nil {
		t.Fatal("expected error for unknown azure activity id")
	}
}

// TestSetCategoryAzureActivity_RejectsInactiveAzureActivityID mirrors
// SetActivityAzureActivity's app-level FK validation for an inactive id.
func TestSetCategoryAzureActivity_RejectsInactiveAzureActivityID(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	mustNoErr(t, s.seedCatalogs())

	az, err := s.AddAzureActivity(ctx, "RUNT2QA", 555, "QA Activity")
	mustNoErr(t, err)
	// az is the second row (the seeded row is the default), so it is safe
	// to deactivate directly.
	mustNoErr(t, s.DeactivateAzureActivity(ctx, az.ID))

	cats, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	if len(cats) == 0 {
		t.Fatal("expected seeded categories")
	}

	if err := s.SetCategoryAzureActivity(ctx, cats[0].ID, &az.ID); err == nil {
		t.Fatal("expected error for inactive azure activity id")
	}
}

// TestSetCategoryAzureActivity_UnknownCategoryReturnsError verifies a
// nonexistent category id surfaces as an error, not a silent no-op.
func TestSetCategoryAzureActivity_UnknownCategoryReturnsError(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	mustNoErr(t, s.seedCatalogs())

	az, err := s.AddAzureActivity(ctx, "RUNT2QA", 555, "QA Activity")
	mustNoErr(t, err)

	if err := s.SetCategoryAzureActivity(ctx, 999999, &az.ID); err == nil {
		t.Fatal("expected error for unknown category id")
	}
}

// TestRemoveTimelogCategory_DropsMappingLeavesActivitiesIntact verifies D3:
// deleting a category (by name) removes its mapping row, but daily
// activities referencing the category's name string are left untouched —
// there is no cascade.
func TestRemoveTimelogCategory_DropsMappingLeavesActivitiesIntact(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	mustNoErr(t, s.seedCatalogs())

	az, err := s.AddAzureActivity(ctx, "RUNT2QA", 555, "QA Activity")
	mustNoErr(t, err)

	cats, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	if len(cats) == 0 {
		t.Fatal("expected seeded categories")
	}
	target := cats[0]
	mustNoErr(t, s.SetCategoryAzureActivity(ctx, target.ID, &az.ID))

	act, err := s.CreateActivity(ctx, "2026-07-28", 1.0, "RNCEA", target.Name, "test entry", "manual")
	mustNoErr(t, err)

	mustNoErr(t, s.RemoveTimelogCategory(target.Name))

	remaining, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	for _, c := range remaining {
		if c.ID == target.ID {
			t.Fatalf("expected category %d to be removed", target.ID)
		}
	}

	got, err := s.GetActivity(ctx, act.ID)
	mustNoErr(t, err)
	if got.Category != target.Name {
		t.Fatalf("expected activity's category name to survive category deletion, got %q", got.Category)
	}
}
