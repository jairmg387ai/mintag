package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// TestUpdateCatalogCategory_PATCH verifies PATCH
// /api/activities/catalog/categories/{id} updates and returns the category
// with its new name and description.
func TestUpdateCatalogCategory_PATCH(t *testing.T) {
	base, _ := newTestServer(t)

	catalogResp := get(t, base+"/api/activities/catalog")
	var catalog struct {
		Categories []struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"categories"`
	}
	decodeJSON(t, catalogResp, &catalog)
	if len(catalog.Categories) == 0 {
		t.Fatal("expected seeded categories")
	}
	categoryID := catalog.Categories[0].ID

	setURL := base + "/api/activities/catalog/categories/" + strconv.FormatInt(categoryID, 10)

	resp := doJSON(t, http.MethodPatch, setURL, map[string]any{"name": "Renamed Category", "description": "Updated description"})
	assertStatus(t, resp, http.StatusOK)
	var updated struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	decodeJSON(t, resp, &updated)
	if updated.ID != categoryID {
		t.Fatalf("expected id=%d, got %d", categoryID, updated.ID)
	}
	if updated.Name != "Renamed Category" {
		t.Fatalf("expected name=%q, got %q", "Renamed Category", updated.Name)
	}
	if updated.Description != "Updated description" {
		t.Fatalf("expected description=%q, got %q", "Updated description", updated.Description)
	}
}

// TestUpdateCatalogCategory_BadIDReturns400 verifies a non-numeric {id} path
// param is rejected with 400.
func TestUpdateCatalogCategory_BadIDReturns400(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPatch, base+"/api/activities/catalog/categories/not-a-number",
		map[string]any{"name": "irrelevant", "description": "irrelevant"})
	assertStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}

// TestUpdateCatalogCategory_UnknownCategoryReturns404 verifies a well-formed
// but nonexistent category id is rejected with 404.
func TestUpdateCatalogCategory_UnknownCategoryReturns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPatch, base+"/api/activities/catalog/categories/999999",
		map[string]any{"name": "irrelevant", "description": "irrelevant"})
	assertStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()
}

// TestUpdateCatalogCategory_EmptyNameReturns422 verifies an empty name is
// rejected with 422, matching UpdateTimelogCategory's validation.
func TestUpdateCatalogCategory_EmptyNameReturns422(t *testing.T) {
	base, _ := newTestServer(t)

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

	resp := doJSON(t, http.MethodPatch, base+"/api/activities/catalog/categories/"+strconv.FormatInt(categoryID, 10),
		map[string]any{"name": "   ", "description": "irrelevant"})
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	resp.Body.Close()
}

// TestUpdateCatalogCategory_MalformedJSONReturns400 verifies a body that
// fails to decode at all is a 400, not a 500.
func TestUpdateCatalogCategory_MalformedJSONReturns400(t *testing.T) {
	base, _ := newTestServer(t)

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
	setURL := base + "/api/activities/catalog/categories/" + strconv.FormatInt(categoryID, 10)

	req, err := http.NewRequest(http.MethodPatch, setURL, strings.NewReader(`{not valid json`))
	mustNoErr(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}

// TestCatalogCategory_DeactivateAndReactivate verifies DELETE soft-deletes
// (not hard-deletes) a category, and POST .../reactivate flips it back to
// active, including that GET ?include_inactive=true now surfaces it.
func TestCatalogCategory_DeactivateAndReactivate(t *testing.T) {
	base, _ := newTestServer(t)

	addResp := doJSON(t, http.MethodPost, base+"/api/activities/catalog/categories", map[string]any{"name": "Temp Category", "description": ""})
	assertStatus(t, addResp, http.StatusOK)
	addResp.Body.Close()

	deactivateResp := doJSON(t, http.MethodDelete, base+"/api/activities/catalog/categories/"+url.PathEscape("Temp Category"), nil)
	assertStatus(t, deactivateResp, http.StatusNoContent)
	deactivateResp.Body.Close()

	activeResp := get(t, base+"/api/activities/catalog")
	var active struct {
		Categories []struct {
			Name     string `json:"name"`
			IsActive bool   `json:"is_active"`
		} `json:"categories"`
	}
	decodeJSON(t, activeResp, &active)
	for _, c := range active.Categories {
		if c.Name == "Temp Category" {
			t.Fatalf("expected Temp Category excluded from default catalog listing, got %#v", c)
		}
	}

	inactiveResp := get(t, base+"/api/activities/catalog?include_inactive=true")
	var withInactive struct {
		Categories []struct {
			Name     string `json:"name"`
			IsActive bool   `json:"is_active"`
		} `json:"categories"`
	}
	decodeJSON(t, inactiveResp, &withInactive)
	found := false
	for _, c := range withInactive.Categories {
		if c.Name == "Temp Category" {
			found = true
			if c.IsActive {
				t.Fatalf("expected Temp Category to have is_active=false, got %#v", c)
			}
		}
	}
	if !found {
		t.Fatalf("expected Temp Category present with include_inactive=true, got %#v", withInactive.Categories)
	}

	reactivateResp := doJSON(t, http.MethodPost, base+"/api/activities/catalog/categories/"+url.PathEscape("Temp Category")+"/reactivate", nil)
	assertStatus(t, reactivateResp, http.StatusNoContent)
	reactivateResp.Body.Close()

	afterResp := get(t, base+"/api/activities/catalog")
	var after struct {
		Categories []struct {
			Name     string `json:"name"`
			IsActive bool   `json:"is_active"`
		} `json:"categories"`
	}
	decodeJSON(t, afterResp, &after)
	found = false
	for _, c := range after.Categories {
		if c.Name == "Temp Category" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Temp Category active again, got %#v", after.Categories)
	}
}

// TestCatalogCategory_ReactivateUnknownReturns404 verifies reactivating an
// unknown category name is rejected with 404.
func TestCatalogCategory_ReactivateUnknownReturns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/catalog/categories/"+url.PathEscape("Does Not Exist")+"/reactivate", nil)
	assertStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()
}

// TestCatalogProject_ReactivateUnknownReturns404 verifies reactivating an
// unknown project name is rejected with 404, mirroring the category case.
func TestCatalogProject_ReactivateUnknownReturns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/catalog/projects/"+url.PathEscape("Does Not Exist")+"/reactivate", nil)
	assertStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()
}

// TestCatalogProject_DeactivateAndReactivate verifies DELETE soft-deletes a
// project and POST .../reactivate flips it back to active.
func TestCatalogProject_DeactivateAndReactivate(t *testing.T) {
	base, _ := newTestServer(t)

	addResp := doJSON(t, http.MethodPost, base+"/api/activities/catalog/projects", map[string]any{"name": "Temp Project"})
	assertStatus(t, addResp, http.StatusOK)
	addResp.Body.Close()

	deactivateResp := doJSON(t, http.MethodDelete, base+"/api/activities/catalog/projects/"+url.PathEscape("Temp Project"), nil)
	assertStatus(t, deactivateResp, http.StatusNoContent)
	deactivateResp.Body.Close()

	reactivateResp := doJSON(t, http.MethodPost, base+"/api/activities/catalog/projects/"+url.PathEscape("Temp Project")+"/reactivate", nil)
	assertStatus(t, reactivateResp, http.StatusNoContent)
	reactivateResp.Body.Close()

	afterResp := get(t, base+"/api/activities/catalog")
	var after struct {
		Projects []struct {
			Name     string `json:"name"`
			IsActive bool   `json:"is_active"`
		} `json:"projects"`
	}
	decodeJSON(t, afterResp, &after)
	found := false
	for _, p := range after.Projects {
		if p.Name == "Temp Project" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Temp Project active again, got %#v", after.Projects)
	}
}

// TestCatalogProject_RenamePATCH verifies PATCH
// /api/activities/catalog/projects/{name} renames the catalog entry.
func TestCatalogProject_RenamePATCH(t *testing.T) {
	base, _ := newTestServer(t)

	addResp := doJSON(t, http.MethodPost, base+"/api/activities/catalog/projects", map[string]any{"name": "Old Project Name"})
	assertStatus(t, addResp, http.StatusOK)
	addResp.Body.Close()

	renameResp := doJSON(t, http.MethodPatch, base+"/api/activities/catalog/projects/"+url.PathEscape("Old Project Name"),
		map[string]any{"name": "New Project Name"})
	assertStatus(t, renameResp, http.StatusOK)
	var renamed struct {
		Name string `json:"name"`
	}
	decodeJSON(t, renameResp, &renamed)
	if renamed.Name != "New Project Name" {
		t.Fatalf("expected name=%q, got %q", "New Project Name", renamed.Name)
	}

	afterResp := get(t, base+"/api/activities/catalog")
	var after struct {
		Projects []struct {
			Name string `json:"name"`
		} `json:"projects"`
	}
	decodeJSON(t, afterResp, &after)
	foundOld, foundNew := false, false
	for _, p := range after.Projects {
		if p.Name == "Old Project Name" {
			foundOld = true
		}
		if p.Name == "New Project Name" {
			foundNew = true
		}
	}
	if foundOld {
		t.Error("expected Old Project Name gone after rename")
	}
	if !foundNew {
		t.Error("expected New Project Name present after rename")
	}
}

// TestCatalogProject_RenameUnknownReturns404 verifies renaming an unknown
// project name is rejected with 404.
func TestCatalogProject_RenameUnknownReturns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPatch, base+"/api/activities/catalog/projects/"+url.PathEscape("Does Not Exist"),
		map[string]any{"name": "irrelevant"})
	assertStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()
}

// TestCatalogProject_RenameEmptyNameReturns422 verifies an empty new name is
// rejected with 422.
func TestCatalogProject_RenameEmptyNameReturns422(t *testing.T) {
	base, _ := newTestServer(t)

	addResp := doJSON(t, http.MethodPost, base+"/api/activities/catalog/projects", map[string]any{"name": "Some Project"})
	assertStatus(t, addResp, http.StatusOK)
	addResp.Body.Close()

	resp := doJSON(t, http.MethodPatch, base+"/api/activities/catalog/projects/"+url.PathEscape("Some Project"),
		map[string]any{"name": "   "})
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	resp.Body.Close()
}

// TestCatalogProject_RenameDuplicateNameReturns422 verifies renaming to a
// name already taken by another project is rejected with 422.
func TestCatalogProject_RenameDuplicateNameReturns422(t *testing.T) {
	base, _ := newTestServer(t)

	for _, name := range []string{"Project A", "Project B"} {
		addResp := doJSON(t, http.MethodPost, base+"/api/activities/catalog/projects", map[string]any{"name": name})
		assertStatus(t, addResp, http.StatusOK)
		addResp.Body.Close()
	}

	resp := doJSON(t, http.MethodPatch, base+"/api/activities/catalog/projects/"+url.PathEscape("Project A"),
		map[string]any{"name": "Project B"})
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	resp.Body.Close()
}
