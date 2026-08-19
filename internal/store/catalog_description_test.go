package store

import (
	"context"
	"testing"
)

// TestAddTimelogCategory_PersistsDescription verifies AddTimelogCategory
// stores a non-empty description alongside the category name.
func TestAddTimelogCategory_PersistsDescription(t *testing.T) {
	s := openTestDB(t)
	mustNoErr(t, s.seedCatalogs())

	mustNoErr(t, s.AddTimelogCategory("Testing Category", "A useful description"))

	cats, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	found := false
	for _, c := range cats {
		if c.Name == "Testing Category" {
			found = true
			if c.Description != "A useful description" {
				t.Fatalf("expected description %q, got %q", "A useful description", c.Description)
			}
		}
	}
	if !found {
		t.Fatal("expected newly added category to be present")
	}
}

// TestUpdateTimelogCategoryDescription_UpdatesExistingCategory verifies the
// description of an existing category can be changed and the returned row
// reflects the new value.
func TestUpdateTimelogCategoryDescription_UpdatesExistingCategory(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	mustNoErr(t, s.seedCatalogs())

	cats, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	if len(cats) == 0 {
		t.Fatal("expected seeded categories")
	}
	target := cats[0]

	updated, err := s.UpdateTimelogCategoryDescription(ctx, target.ID, "New description")
	mustNoErr(t, err)
	if updated == nil {
		t.Fatal("expected updated category, got nil")
	}
	if updated.ID != target.ID {
		t.Fatalf("expected id=%d, got %d", target.ID, updated.ID)
	}
	if updated.Description != "New description" {
		t.Fatalf("expected description=%q, got %q", "New description", updated.Description)
	}

	// Verify the change is persisted, not just reflected in the return value.
	reread, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	for _, c := range reread {
		if c.ID == target.ID && c.Description != "New description" {
			t.Fatalf("expected persisted description=%q, got %q", "New description", c.Description)
		}
	}
}

// TestUpdateTimelogCategoryDescription_UnknownIDReturnsError verifies the
// package's not-found convention: an update against a nonexistent category id
// surfaces as an error, not a silent no-op.
func TestUpdateTimelogCategoryDescription_UnknownIDReturnsError(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	mustNoErr(t, s.seedCatalogs())

	if _, err := s.UpdateTimelogCategoryDescription(ctx, 999999, "does not matter"); err == nil {
		t.Fatal("expected error for unknown category id")
	}
}

// TestSeedCategoriesTable_ParsesPipeDelimitedDescriptions verifies the
// "Name|Description" seed format is parsed correctly for both an empty
// description (trailing "|") and a non-empty description, using the real
// embedded seed file.
func TestSeedCategoriesTable_ParsesPipeDelimitedDescriptions(t *testing.T) {
	s := openTestDB(t)
	mustNoErr(t, s.seedCatalogs())

	cats, err := s.ListTimelogCategories()
	mustNoErr(t, err)

	byName := make(map[string]string, len(cats))
	for _, c := range cats {
		byName[c.Name] = c.Description
	}

	desc, ok := byName["Bridge"]
	if !ok {
		t.Fatal("expected seeded category \"Bridge\"")
	}
	if desc != "" {
		t.Fatalf("expected empty description for \"Bridge\", got %q", desc)
	}

	desc, ok = byName["Arquitectura"]
	if !ok {
		t.Fatal("expected seeded category \"Arquitectura\"")
	}
	if desc != "Esta actividad solo puede ser utilizada por los 2 Arquitectos" {
		t.Fatalf("expected non-empty seed description for \"Arquitectura\", got %q", desc)
	}
}

// TestSeedCategoriesTable_NonDestructiveToEditedDescriptions verifies that
// re-running the seeding entrypoint (as happens on every Open()) does not
// overwrite a description a user has already edited on a category that was
// originally seeded — INSERT OR IGNORE only inserts brand-new names.
func TestSeedCategoriesTable_NonDestructiveToEditedDescriptions(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	mustNoErr(t, s.seedCatalogs())

	cats, err := s.ListTimelogCategories()
	mustNoErr(t, err)

	var target TimelogCategory
	found := false
	for _, c := range cats {
		if c.Name == "Bridge" {
			target = c
			found = true
		}
	}
	if !found {
		t.Fatal("expected seeded category \"Bridge\"")
	}

	_, err = s.UpdateTimelogCategoryDescription(ctx, target.ID, "user-edited description")
	mustNoErr(t, err)

	// Simulate what happens on a second Open(): the seeding entrypoint runs
	// again against a DB that already has this category.
	mustNoErr(t, s.seedCatalogs())

	reread, err := s.ListTimelogCategories()
	mustNoErr(t, err)
	for _, c := range reread {
		if c.Name == "Bridge" && c.Description != "user-edited description" {
			t.Fatalf("expected re-seeding to leave user-edited description intact, got %q", c.Description)
		}
	}
}
