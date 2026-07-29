package server

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// TestUpdateCatalogCategoryDescription_PUT verifies PUT
// /api/activities/catalog/categories/{id}/description updates and returns
// the category with its new description.
func TestUpdateCatalogCategoryDescription_PUT(t *testing.T) {
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

	setURL := base + "/api/activities/catalog/categories/" + strconv.FormatInt(categoryID, 10) + "/description"

	resp := doJSON(t, http.MethodPut, setURL, map[string]any{"description": "Updated description"})
	assertStatus(t, resp, http.StatusOK)
	var updated struct {
		ID          int64  `json:"id"`
		Description string `json:"description"`
	}
	decodeJSON(t, resp, &updated)
	if updated.ID != categoryID {
		t.Fatalf("expected id=%d, got %d", categoryID, updated.ID)
	}
	if updated.Description != "Updated description" {
		t.Fatalf("expected description=%q, got %q", "Updated description", updated.Description)
	}
}

// TestUpdateCatalogCategoryDescription_BadIDReturns400 verifies a
// non-numeric {id} path param is rejected with 400.
func TestUpdateCatalogCategoryDescription_BadIDReturns400(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPut, base+"/api/activities/catalog/categories/not-a-number/description",
		map[string]any{"description": "irrelevant"})
	assertStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}

// TestUpdateCatalogCategoryDescription_UnknownCategoryReturns404 verifies a
// well-formed but nonexistent category id is rejected with 404.
func TestUpdateCatalogCategoryDescription_UnknownCategoryReturns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPut, base+"/api/activities/catalog/categories/999999/description",
		map[string]any{"description": "irrelevant"})
	assertStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()
}

// TestUpdateCatalogCategoryDescription_MalformedJSONReturns400 verifies a
// body that fails to decode at all is a 400, not a 500.
func TestUpdateCatalogCategoryDescription_MalformedJSONReturns400(t *testing.T) {
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
	setURL := base + "/api/activities/catalog/categories/" + strconv.FormatInt(categoryID, 10) + "/description"

	req, err := http.NewRequest(http.MethodPut, setURL, strings.NewReader(`{not valid json`))
	mustNoErr(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}
