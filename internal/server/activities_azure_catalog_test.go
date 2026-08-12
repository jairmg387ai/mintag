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

// TestPatchActivityOmittingAzureActivityIDLeavesFKUntouched is a regression
// test for a bug found during review: a bare *int64 can't distinguish
// "azure_activity_id omitted from the request" from "explicitly null", so an
// earlier version of applyAzureActivityID re-validated (and could reject) an
// unrelated, already-set FK on every PATCH — even one that only changes
// hours. It reproduces the reported failure mode: assign a non-default
// activity, deactivate it (an explicitly supported state per
// DeactivateAzureActivity's contract — historical references must survive
// deactivation), then PATCH an unrelated field and confirm it still succeeds
// and the FK is left exactly as it was.
func TestPatchActivityOmittingAzureActivityIDLeavesFKUntouched(t *testing.T) {
	base, _ := newTestServer(t)

	addResp := doJSON(t, http.MethodPost, base+"/api/activities/azure-catalog", map[string]any{
		"org": "RUNT2QA", "work_item_id": 999222, "label": "To be deactivated",
	})
	assertStatus(t, addResp, http.StatusCreated)
	var added struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, addResp, &added)

	createResp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-07-19", "hours": 1.0, "project": "RNCEA",
		"category":          "Actividades de arquitectura, diseño y código",
		"registro_diario":   "assigned to a soon-to-be-deactivated activity",
		"source":            "manual",
		"azure_activity_id": added.ID,
	})
	assertStatus(t, createResp, http.StatusCreated)
	var created struct {
		ID              int64  `json:"id"`
		AzureActivityID *int64 `json:"azure_activity_id"`
	}
	decodeJSON(t, createResp, &created)
	if created.AzureActivityID == nil || *created.AzureActivityID != added.ID {
		t.Fatalf("expected azure_activity_id=%d, got %#v", added.ID, created.AzureActivityID)
	}

	// added.ID is not the default (seeded id=1 is), so it can be deactivated
	// directly — deactivation must preserve the historical FK reference.
	deactivateResp := doJSON(t, http.MethodDelete, base+"/api/activities/azure-catalog/"+strconv.FormatInt(added.ID, 10), nil)
	assertStatus(t, deactivateResp, http.StatusNoContent)
	deactivateResp.Body.Close()

	// PATCH an unrelated field only — azure_activity_id is omitted, not null.
	patchResp := doJSON(t, http.MethodPatch, base+"/api/activities/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"hours": 3.5, "project": "RNCEA", "category": "Actividades de arquitectura, diseño y código",
		"registro_diario": "hours updated, azure activity untouched",
	})
	assertStatus(t, patchResp, http.StatusOK)
	var patched struct {
		Hours           float64 `json:"hours"`
		AzureActivityID *int64  `json:"azure_activity_id"`
	}
	decodeJSON(t, patchResp, &patched)
	if patched.Hours != 3.5 {
		t.Fatalf("expected hours=3.5, got %v", patched.Hours)
	}
	if patched.AzureActivityID == nil || *patched.AzureActivityID != added.ID {
		t.Fatalf("expected azure_activity_id to remain %d untouched, got %#v", added.ID, patched.AzureActivityID)
	}
}

// TestPatchActivityExplicitNullClearsAzureActivityID verifies that sending
// azure_activity_id as an explicit JSON null (as opposed to omitting the
// field) clears the FK back to "use the current default", closing the gap
// where the UI previously had no way to revert an explicit assignment.
func TestPatchActivityExplicitNullClearsAzureActivityID(t *testing.T) {
	base, _ := newTestServer(t)

	createResp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-07-19", "hours": 1.0, "project": "RNCEA",
		"category":          "Actividades de arquitectura, diseño y código",
		"registro_diario":   "will be cleared back to default",
		"source":            "manual",
		"azure_activity_id": 1,
	})
	assertStatus(t, createResp, http.StatusCreated)
	var created struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, createResp, &created)

	patchResp := doJSON(t, http.MethodPatch, base+"/api/activities/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"azure_activity_id": nil,
	})
	assertStatus(t, patchResp, http.StatusOK)
	var patched struct {
		AzureActivityID *int64 `json:"azure_activity_id"`
	}
	decodeJSON(t, patchResp, &patched)
	if patched.AzureActivityID != nil {
		t.Fatalf("expected azure_activity_id to be cleared to nil, got %#v", *patched.AzureActivityID)
	}
}

// TestCreateActivitySetsReferenceID verifies POST /activities accepts an
// optional reference_id and persists it, mirroring
// TestCreateActivityAssignsAzureActivityID above.
func TestCreateActivitySetsReferenceID(t *testing.T) {
	base, _ := newTestServer(t)

	createResp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-07-19", "hours": 1.0, "project": "RNCEA",
		"category":        "Actividades de arquitectura, diseño y código",
		"registro_diario": "reference id via create",
		"source":          "manual",
		"reference_id":    "156789",
	})
	assertStatus(t, createResp, http.StatusCreated)
	var created struct {
		ID          int64   `json:"id"`
		ReferenceID *string `json:"reference_id"`
	}
	decodeJSON(t, createResp, &created)
	if created.ReferenceID == nil || *created.ReferenceID != "156789" {
		t.Fatalf("expected reference_id=156789, got %#v", created.ReferenceID)
	}
}

// TestPatchActivitySetsReferenceID verifies PATCH /activities/{id} accepts
// an optional reference_id and persists it.
func TestPatchActivitySetsReferenceID(t *testing.T) {
	base, _ := newTestServer(t)

	createResp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-07-19", "hours": 1.0, "project": "RNCEA",
		"category":        "Actividades de arquitectura, diseño y código",
		"registro_diario": "reference id via patch",
		"source":          "manual",
	})
	assertStatus(t, createResp, http.StatusCreated)
	var created struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, createResp, &created)

	patchResp := doJSON(t, http.MethodPatch, base+"/api/activities/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"reference_id": "MANTIS-1234",
	})
	assertStatus(t, patchResp, http.StatusOK)
	var patched struct {
		ReferenceID *string `json:"reference_id"`
	}
	decodeJSON(t, patchResp, &patched)
	if patched.ReferenceID == nil || *patched.ReferenceID != "MANTIS-1234" {
		t.Fatalf("expected reference_id=MANTIS-1234, got %#v", patched.ReferenceID)
	}
}

// TestPatchActivityOmittingReferenceIDLeavesItUntouched mirrors
// TestPatchActivityOmittingAzureActivityIDLeavesFKUntouched: a PATCH that
// only edits an unrelated field must not clear an already-set reference_id.
func TestPatchActivityOmittingReferenceIDLeavesItUntouched(t *testing.T) {
	base, _ := newTestServer(t)

	createResp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-07-19", "hours": 1.0, "project": "RNCEA",
		"category":        "Actividades de arquitectura, diseño y código",
		"registro_diario": "reference id must survive unrelated patch",
		"source":          "manual",
		"reference_id":    "LF-2026-045",
	})
	assertStatus(t, createResp, http.StatusCreated)
	var created struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, createResp, &created)

	patchResp := doJSON(t, http.MethodPatch, base+"/api/activities/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"hours": 3.5, "project": "RNCEA", "category": "Actividades de arquitectura, diseño y código",
		"registro_diario": "hours updated, reference untouched",
	})
	assertStatus(t, patchResp, http.StatusOK)
	var patched struct {
		Hours       float64 `json:"hours"`
		ReferenceID *string `json:"reference_id"`
	}
	decodeJSON(t, patchResp, &patched)
	if patched.Hours != 3.5 {
		t.Fatalf("expected hours=3.5, got %v", patched.Hours)
	}
	if patched.ReferenceID == nil || *patched.ReferenceID != "LF-2026-045" {
		t.Fatalf("expected reference_id to remain untouched, got %#v", patched.ReferenceID)
	}
}

// TestPatchActivityExplicitNullClearsReferenceID verifies sending
// reference_id as an explicit JSON null clears it, mirroring
// TestPatchActivityExplicitNullClearsAzureActivityID above.
func TestPatchActivityExplicitNullClearsReferenceID(t *testing.T) {
	base, _ := newTestServer(t)

	createResp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-07-19", "hours": 1.0, "project": "RNCEA",
		"category":        "Actividades de arquitectura, diseño y código",
		"registro_diario": "will be cleared",
		"source":          "manual",
		"reference_id":    "156789",
	})
	assertStatus(t, createResp, http.StatusCreated)
	var created struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, createResp, &created)

	patchResp := doJSON(t, http.MethodPatch, base+"/api/activities/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"reference_id": nil,
	})
	assertStatus(t, patchResp, http.StatusOK)
	var patched struct {
		ReferenceID *string `json:"reference_id"`
	}
	decodeJSON(t, patchResp, &patched)
	if patched.ReferenceID != nil {
		t.Fatalf("expected reference_id to be cleared to nil, got %#v", *patched.ReferenceID)
	}
}
