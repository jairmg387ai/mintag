package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestAzureCatalogList_IncludesSeededDefault verifies the migration-seeded
// legacy default (work_item_id=156263) is visible over REST.
func TestAzureCatalogList_IncludesSeededDefault(t *testing.T) {
	base, _ := newTestServer(t)

	resp := get(t, base+"/api/activities/azure-catalog")
	var activities []map[string]any
	decodeJSON(t, resp, &activities)

	if len(activities) != 1 {
		t.Fatalf("expected 1 seeded activity, got %d: %#v", len(activities), activities)
	}
	if activities[0]["work_item_id"].(float64) != 156263 {
		t.Fatalf("expected seeded work_item_id 156263, got %#v", activities[0]["work_item_id"])
	}
	if activities[0]["is_default"] != true {
		t.Fatalf("expected seeded row to be default, got %#v", activities[0])
	}
}

// TestAzureCatalogAddUpdateSetDefaultDeactivate exercises the full catalog
// CRUD lifecycle: add a non-default activity, rename it, promote it to
// default, then deactivate the now-non-default seeded row.
func TestAzureCatalogAddUpdateSetDefaultDeactivate(t *testing.T) {
	base, _ := newTestServer(t)

	addResp := doJSON(t, http.MethodPost, base+"/api/activities/azure-catalog", map[string]any{
		"org": "RUNT2QA", "work_item_id": 999111, "label": "QA Activity",
	})
	assertStatus(t, addResp, http.StatusCreated)
	var added struct {
		ID        int64  `json:"id"`
		Label     string `json:"label"`
		IsDefault bool   `json:"is_default"`
	}
	decodeJSON(t, addResp, &added)
	if added.IsDefault {
		t.Fatalf("expected second activity to not be default, got %#v", added)
	}

	updateResp := doJSON(t, http.MethodPatch, base+"/api/activities/azure-catalog/"+strconv.FormatInt(added.ID, 10), map[string]any{
		"org": "RUNT2QA", "label": "QA Renamed",
	})
	assertStatus(t, updateResp, http.StatusOK)
	var updated struct {
		Label string `json:"label"`
	}
	decodeJSON(t, updateResp, &updated)
	if updated.Label != "QA Renamed" {
		t.Fatalf("expected renamed label, got %q", updated.Label)
	}

	defaultResp := doJSON(t, http.MethodPost, base+"/api/activities/azure-catalog/"+strconv.FormatInt(added.ID, 10)+"/default", nil)
	assertStatus(t, defaultResp, http.StatusOK)
	var newDefault struct {
		ID        int64 `json:"id"`
		IsDefault bool  `json:"is_default"`
	}
	decodeJSON(t, defaultResp, &newDefault)
	if newDefault.ID != added.ID || !newDefault.IsDefault {
		t.Fatalf("expected activity %d to be default, got %#v", added.ID, newDefault)
	}

	// The seeded row (id=1) is no longer the default, so it can now be deactivated.
	deactivateResp := doJSON(t, http.MethodDelete, base+"/api/activities/azure-catalog/1", nil)
	assertStatus(t, deactivateResp, http.StatusNoContent)
	deactivateResp.Body.Close()

	listResp := get(t, base+"/api/activities/azure-catalog")
	var active []map[string]any
	decodeJSON(t, listResp, &active)
	if len(active) != 1 || int64(active[0]["id"].(float64)) != added.ID {
		t.Fatalf("expected only the new default active, got %#v", active)
	}
}

// TestAzureCatalogDeactivateCurrentDefaultReturns422 verifies the Phase 2.2
// block rule (cannot deactivate the current default) surfaces as 422, not 500.
func TestAzureCatalogDeactivateCurrentDefaultReturns422(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodDelete, base+"/api/activities/azure-catalog/1", nil)
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	resp.Body.Close()
}

// TestAzureCatalogDeactivateMissingReturns404 verifies not-found errors map to 404.
func TestAzureCatalogDeactivateMissingReturns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodDelete, base+"/api/activities/azure-catalog/999999", nil)
	assertStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()
}

// TestAzureCatalogMutationsRequireLocalRequest verifies POST/PATCH/DELETE
// azure-catalog routes are gated the same way as azure-config.
func TestAzureCatalogMutationsRequireLocalRequest(t *testing.T) {
	_, st := newTestServer(t)
	h := New(st).Handler()

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/activities/azure-catalog", strings.NewReader(`{"org":"X","work_item_id":1,"label":"L"}`))
	req.Host = "evil.example"
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for non-local host, got %d", rec.Code)
	}
}

// TestCreateActivityAssignsAzureActivityID verifies POST /activities accepts
// an optional azure_activity_id and persists the FK.
func TestCreateActivityAssignsAzureActivityID(t *testing.T) {
	base, _ := newTestServer(t)

	createResp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-07-19", "hours": 1.0, "project": "RNCEA",
		"category":          "Actividades de arquitectura, diseño y código",
		"registro_diario":   "azure activity id via create",
		"source":            "manual",
		"azure_activity_id": 1,
	})
	assertStatus(t, createResp, http.StatusCreated)
	var created struct {
		ID              int64  `json:"id"`
		AzureActivityID *int64 `json:"azure_activity_id"`
	}
	decodeJSON(t, createResp, &created)
	if created.AzureActivityID == nil || *created.AzureActivityID != 1 {
		t.Fatalf("expected azure_activity_id=1, got %#v", created.AzureActivityID)
	}
}

// TestPatchActivitySetsAzureActivityID verifies PATCH /activities/{id} accepts
// an optional azure_activity_id and persists the FK.
func TestPatchActivitySetsAzureActivityID(t *testing.T) {
	base, _ := newTestServer(t)

	createResp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-07-19", "hours": 1.0, "project": "RNCEA",
		"category":        "Actividades de arquitectura, diseño y código",
		"registro_diario": "azure activity id via patch",
		"source":          "manual",
	})
	assertStatus(t, createResp, http.StatusCreated)
	var created struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, createResp, &created)

	patchResp := doJSON(t, http.MethodPatch, base+"/api/activities/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"azure_activity_id": 1,
	})
	assertStatus(t, patchResp, http.StatusOK)
	var patched struct {
		AzureActivityID *int64 `json:"azure_activity_id"`
	}
	decodeJSON(t, patchResp, &patched)
	if patched.AzureActivityID == nil || *patched.AzureActivityID != 1 {
		t.Fatalf("expected azure_activity_id=1, got %#v", patched.AzureActivityID)
	}
}
