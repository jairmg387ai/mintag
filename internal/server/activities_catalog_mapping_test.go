package server

import (
	"net/http"
	"strconv"
	"testing"
)

// TestSetCategoryAzureActivity_PUT verifies PUT
// /api/activities/catalog/categories/{id}/azure-activity assigns and clears
// a category's default Azure activity mapping, and rejects unknown/inactive
// azure activity ids and unknown categories with the correct status codes.
func TestSetCategoryAzureActivity_PUT(t *testing.T) {
	base, _ := newTestServer(t)

	addResp := doJSON(t, http.MethodPost, base+"/api/activities/azure-catalog", map[string]any{
		"org": "RUNT2QA", "work_item_id": 777, "label": "QA Mapping Target",
	})
	assertStatus(t, addResp, http.StatusCreated)
	var az struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, addResp, &az)

	catalogResp := get(t, base+"/api/activities/catalog")
	var catalog struct {
		Categories []struct {
			ID              int64  `json:"id"`
			Name            string `json:"name"`
			AzureActivityID *int64 `json:"azure_activity_id"`
		} `json:"categories"`
	}
	decodeJSON(t, catalogResp, &catalog)
	if len(catalog.Categories) == 0 {
		t.Fatal("expected seeded categories")
	}
	categoryID := catalog.Categories[0].ID

	setURL := base + "/api/activities/catalog/categories/" + strconv.FormatInt(categoryID, 10) + "/azure-activity"

	// Assign
	setResp := doJSON(t, http.MethodPut, setURL, map[string]any{"azure_activity_id": az.ID})
	assertStatus(t, setResp, http.StatusOK)
	var updated struct {
		ID              int64  `json:"id"`
		AzureActivityID *int64 `json:"azure_activity_id"`
	}
	decodeJSON(t, setResp, &updated)
	if updated.AzureActivityID == nil || *updated.AzureActivityID != az.ID {
		t.Fatalf("expected azure_activity_id=%d, got %#v", az.ID, updated.AzureActivityID)
	}

	// Clear
	clearResp := doJSON(t, http.MethodPut, setURL, map[string]any{"azure_activity_id": nil})
	assertStatus(t, clearResp, http.StatusOK)
	var cleared struct {
		AzureActivityID *int64 `json:"azure_activity_id"`
	}
	decodeJSON(t, clearResp, &cleared)
	if cleared.AzureActivityID != nil {
		t.Fatalf("expected cleared mapping, got %#v", cleared.AzureActivityID)
	}

	// Unknown azure activity id -> 422
	unknownResp := doJSON(t, http.MethodPut, setURL, map[string]any{"azure_activity_id": 999999})
	assertStatus(t, unknownResp, http.StatusUnprocessableEntity)
	unknownResp.Body.Close()

	// Inactive azure activity id -> 422
	deactivateResp := doJSON(t, http.MethodDelete, base+"/api/activities/azure-catalog/"+strconv.FormatInt(az.ID, 10), nil)
	assertStatus(t, deactivateResp, http.StatusNoContent)
	deactivateResp.Body.Close()
	inactiveResp := doJSON(t, http.MethodPut, setURL, map[string]any{"azure_activity_id": az.ID})
	assertStatus(t, inactiveResp, http.StatusUnprocessableEntity)
	inactiveResp.Body.Close()

	// Unknown category -> 404
	missingResp := doJSON(t, http.MethodPut, base+"/api/activities/catalog/categories/999999/azure-activity", map[string]any{"azure_activity_id": nil})
	assertStatus(t, missingResp, http.StatusNotFound)
	missingResp.Body.Close()
}

// TestCatalogGET_DecoratesStaleMappingWithAzureActivityLabel verifies D5a:
// GET /api/activities/catalog resolves a category's azure_activity_label
// even when the mapped Azure activity has since been deactivated — the raw
// (stale) azure_activity_id is preserved and its real label is still shown.
func TestCatalogGET_DecoratesStaleMappingWithAzureActivityLabel(t *testing.T) {
	base, _ := newTestServer(t)

	addResp := doJSON(t, http.MethodPost, base+"/api/activities/azure-catalog", map[string]any{
		"org": "RUNT2QA", "work_item_id": 778, "label": "Stale Mapping Target",
	})
	assertStatus(t, addResp, http.StatusCreated)
	var az struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, addResp, &az)

	catalogResp := get(t, base+"/api/activities/catalog")
	var catalog struct {
		Categories []struct {
			ID int64 `json:"id"`
		} `json:"categories"`
	}
	decodeJSON(t, catalogResp, &catalog)
	if len(catalog.Categories) == 0 {
		t.Fatal("expected seeded categories")
	}
	categoryID := catalog.Categories[0].ID

	setURL := base + "/api/activities/catalog/categories/" + strconv.FormatInt(categoryID, 10) + "/azure-activity"
	assignResp := doJSON(t, http.MethodPut, setURL, map[string]any{"azure_activity_id": az.ID})
	assertStatus(t, assignResp, http.StatusOK)
	assignResp.Body.Close()

	deactivateResp := doJSON(t, http.MethodDelete, base+"/api/activities/azure-catalog/"+strconv.FormatInt(az.ID, 10), nil)
	assertStatus(t, deactivateResp, http.StatusNoContent)
	deactivateResp.Body.Close()

	finalCatalogResp := get(t, base+"/api/activities/catalog")
	var finalCatalog struct {
		Categories []struct {
			ID                 int64   `json:"id"`
			AzureActivityID    *int64  `json:"azure_activity_id"`
			AzureActivityLabel *string `json:"azure_activity_label"`
		} `json:"categories"`
	}
	decodeJSON(t, finalCatalogResp, &finalCatalog)

	var found bool
	for _, c := range finalCatalog.Categories {
		if c.ID == categoryID {
			found = true
			if c.AzureActivityID == nil || *c.AzureActivityID != az.ID {
				t.Fatalf("expected stale azure_activity_id=%d to remain, got %#v", az.ID, c.AzureActivityID)
			}
			if c.AzureActivityLabel == nil || *c.AzureActivityLabel != "Stale Mapping Target (#778)" {
				t.Fatalf("expected azure_activity_label to resolve the inactive activity's label, got %#v", c.AzureActivityLabel)
			}
		}
	}
	if !found {
		t.Fatal("mapped category not found in catalog response")
	}
}
