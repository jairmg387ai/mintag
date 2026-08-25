package mcp

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// TestCatalogCategoryRemove_DeactivatesRatherThanHardDeletes mirrors
// TestCatalogProjectRemove_DeactivatesRatherThanHardDeletes for categories:
// catalog_category_remove now soft-deletes (is_active=0) instead of hard
// deleting, so historical daily_activities.category references survive.
func TestCatalogCategoryRemove_DeactivatesRatherThanHardDeletes(t *testing.T) {
	s, st := newTestMCPServer(t)

	if err := st.AddTimelogCategory("Removable Category", ""); err != nil {
		t.Fatal(err)
	}

	out := callTool(t, s, "catalog_category_remove", map[string]any{"name": "Removable Category"})
	if _, ok := out["error"]; ok {
		t.Fatalf("expected no error removing an existing category, got %#v", out)
	}

	all, err := st.ListTimelogCategories(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range all {
		if c.Name == "Removable Category" {
			found = true
			if c.IsActive {
				t.Fatalf("expected Removable Category to have is_active=false")
			}
		}
	}
	if !found {
		t.Fatalf("expected Removable Category to still exist (soft-deleted), got %v", all)
	}

	active, err := st.ListTimelogCategories(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range active {
		if c.Name == "Removable Category" {
			t.Fatalf("expected Removable Category excluded from the active listing, got %v", active)
		}
	}
}

// TestCatalogCategoryRemove_UnknownNameReturnsError mirrors
// TestCatalogProjectRemove_UnknownNameReturnsError for categories.
func TestCatalogCategoryRemove_UnknownNameReturnsError(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_category_remove", map[string]any{"name": "Never Added"})
	errMsg, ok := out["error"].(string)
	if !ok || errMsg == "" {
		t.Fatalf("expected an error result for unknown category name, got %#v", out)
	}
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' in error, got %q", errMsg)
	}
}

// TestCatalogProjectReactivate_FlipsInactiveBackToActive verifies the new
// catalog_project_reactivate tool undoes a prior catalog_project_remove.
func TestCatalogProjectReactivate_FlipsInactiveBackToActive(t *testing.T) {
	s, st := newTestMCPServer(t)

	if err := st.AddTimelogProject("Toggle Project"); err != nil {
		t.Fatal(err)
	}
	callTool(t, s, "catalog_project_remove", map[string]any{"name": "Toggle Project"})

	out := callTool(t, s, "catalog_project_reactivate", map[string]any{"name": "Toggle Project"})
	if _, ok := out["error"]; ok {
		t.Fatalf("expected no error reactivating, got %#v", out)
	}

	active, err := st.ListTimelogProjects(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(active, "Toggle Project") {
		t.Fatalf("expected Toggle Project active again, got %v", active)
	}
}

// TestCatalogProjectReactivate_UnknownNameReturnsError verifies reactivating
// an unknown project name surfaces a not-found error.
func TestCatalogProjectReactivate_UnknownNameReturnsError(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_project_reactivate", map[string]any{"name": "Never Added"})
	errMsg, ok := out["error"].(string)
	if !ok || errMsg == "" {
		t.Fatalf("expected an error result for unknown project name, got %#v", out)
	}
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' in error, got %q", errMsg)
	}
}

// TestCatalogCategoryReactivate_FlipsInactiveBackToActive verifies the new
// catalog_category_reactivate tool undoes a prior catalog_category_remove.
func TestCatalogCategoryReactivate_FlipsInactiveBackToActive(t *testing.T) {
	s, st := newTestMCPServer(t)

	if err := st.AddTimelogCategory("Toggle Category", ""); err != nil {
		t.Fatal(err)
	}
	callTool(t, s, "catalog_category_remove", map[string]any{"name": "Toggle Category"})

	out := callTool(t, s, "catalog_category_reactivate", map[string]any{"name": "Toggle Category"})
	if _, ok := out["error"]; ok {
		t.Fatalf("expected no error reactivating, got %#v", out)
	}

	active, err := st.ListTimelogCategories(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range active {
		if c.Name == "Toggle Category" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Toggle Category active again, got %v", active)
	}
}

// TestCatalogCategoryReactivate_UnknownNameReturnsError verifies reactivating
// an unknown category name surfaces a not-found error.
func TestCatalogCategoryReactivate_UnknownNameReturnsError(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_category_reactivate", map[string]any{"name": "Never Added"})
	errMsg, ok := out["error"].(string)
	if !ok || errMsg == "" {
		t.Fatalf("expected an error result for unknown category name, got %#v", out)
	}
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' in error, got %q", errMsg)
	}
}

// TestCatalogAzureActivityReactivate_FlipsInactiveBackToActive verifies the
// new catalog_azure_activity_reactivate tool undoes a prior
// catalog_azure_activity_remove.
func TestCatalogAzureActivityReactivate_FlipsInactiveBackToActive(t *testing.T) {
	s, st := newTestMCPServer(t)

	added, err := st.AddAzureActivity(context.Background(), "RUNT2QA", 888111, "Toggle Activity", "", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	callTool(t, s, "catalog_azure_activity_remove", map[string]any{"id": strconv.FormatInt(added.ID, 10)})

	out := callTool(t, s, "catalog_azure_activity_reactivate", map[string]any{"id": strconv.FormatInt(added.ID, 10)})
	if _, ok := out["error"]; ok {
		t.Fatalf("expected no error reactivating, got %#v", out)
	}

	activities, err := st.ListAzureActivities(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range activities {
		if a.ID == added.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected activity %d active again, got %#v", added.ID, activities)
	}
}

// TestCatalogAzureActivityReactivate_UnknownIDReturnsError verifies
// reactivating an unknown azure activity id surfaces a not-found error.
func TestCatalogAzureActivityReactivate_UnknownIDReturnsError(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_azure_activity_reactivate", map[string]any{"id": "999999"})
	errMsg, ok := out["error"].(string)
	if !ok || errMsg == "" {
		t.Fatalf("expected an error result for unknown azure activity id, got %#v", out)
	}
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' in error, got %q", errMsg)
	}
}
