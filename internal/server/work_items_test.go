package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// azureWorkItemsSuccessServer returns an httptest server that fakes the
// create+activate Azure round trip (id 9001, full activation success) so
// catalog-registration tests don't need to repeat that boilerplate.
func azureWorkItemsSuccessServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"route-user-id","providerDisplayName":"Route User"}}`)) //nolint:errcheck
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/_apis/wit/workitems/$Task"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":9001}`)) //nolint:errcheck
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":9001}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	}))
}

// TestCreateAzureWorkItem_NotConfigured verifies the 503 guard matches the
// existing assigned-work-items endpoint's behavior when no Azure token is
// configured.
func TestCreateAzureWorkItem_NotConfigured(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	base, _ := newTestServer(t)

	resp, err := http.Post(base+"/api/activities/azure-work-items", "application/json", strings.NewReader(`{"title":"T","area_path":"A","iteration_path":"I"}`))
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusServiceUnavailable)
}

// TestCreateAzureWorkItem_Success exercises the full create+activate path
// through the REST handler, mocking Azure with an httptest server the same
// way TestListAssignedAzureWorkItems does.
func TestCreateAzureWorkItem_Success(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	var createBody, patchBodies []string
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"route-user-id","providerDisplayName":"Route User"}}`)) //nolint:errcheck
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/_apis/wit/workitems/$Task"):
			createBody = append(createBody, "create")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":9001}`)) //nolint:errcheck
		case r.Method == http.MethodPatch:
			patchBodies = append(patchBodies, "patch")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":9001}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)

	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items", map[string]any{
		"title": "Fix the thing", "area_path": `PROJ\Team`, "iteration_path": `PROJ\Sprint 1`, "original_estimate": 8,
	})
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		ID              int    `json:"id"`
		State           string `json:"state"`
		ActivationError string `json:"activation_error"`
	}
	decodeJSON(t, resp, &body)
	if body.ID != 9001 || body.State != "Active" {
		t.Errorf("expected {9001 Active}, got %+v", body)
	}
	if body.ActivationError != "" {
		t.Errorf("expected no activation_error on full success, got %q", body.ActivationError)
	}
	if len(createBody) != 1 {
		t.Errorf("expected exactly 1 create call, got %d", len(createBody))
	}
	if len(patchBodies) != 2 {
		t.Errorf("expected exactly 2 activation PATCH calls, got %d", len(patchBodies))
	}
}

// TestCreateAzureWorkItem_ValidationErrors covers the required-field 400s
// before any Azure call is attempted.
func TestCreateAzureWorkItem_ValidationErrors(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureCalled := false
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureCalled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)
	azureCalled = false // reset: the PUT above already resolved identity via connectiondata

	tests := []map[string]any{
		{"title": "", "area_path": "A", "iteration_path": "I"},
		{"title": "T", "area_path": "", "iteration_path": "I"},
		{"title": "T", "area_path": "A", "iteration_path": ""},
	}
	for _, body := range tests {
		resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items", body)
		assertStatus(t, resp, http.StatusBadRequest)
	}
	if azureCalled {
		t.Error("expected no Azure call for a locally-rejected validation error")
	}
}

// TestCreateAzureWorkItem_ActivationFailurePartialSuccess verifies a failed
// activation is reported as 200 with activation_error, not a 5xx — the work
// item was still created in Azure.
func TestCreateAzureWorkItem_ActivationFailurePartialSuccess(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":9002}`)) //nolint:errcheck
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"not allowed"}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items", map[string]any{
		"title": "T", "area_path": "A", "iteration_path": "I",
	})
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		ID              int    `json:"id"`
		State           string `json:"state"`
		ActivationError string `json:"activation_error"`
	}
	decodeJSON(t, resp, &body)
	if body.ID != 9002 || body.State != "Proposed" {
		t.Errorf("expected the created work item preserved as {9002 Proposed}, got %+v", body)
	}
	if body.ActivationError == "" {
		t.Error("expected a non-empty activation_error")
	}
}

// TestCreateAzureWorkItem_RegistersCatalogEntryWithProjectAndCategory verifies
// that supplying project + category_id registers the created work item into
// the azure_activities catalog and returns its id.
func TestCreateAzureWorkItem_RegistersCatalogEntryWithProjectAndCategory(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := azureWorkItemsSuccessServer(t)
	defer azureServer.Close()

	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	categoryID := seedTimelogCategory(t, st, "Desarrollo")

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items", map[string]any{
		"title": "Fix the thing", "area_path": `PROJ\Team`, "iteration_path": `PROJ\Sprint 1`,
		"project": "Mintag", "category_id": categoryID,
	})
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		ID              int    `json:"id"`
		State           string `json:"state"`
		AzureActivityID int64  `json:"azure_activity_id"`
		CatalogError    string `json:"catalog_error"`
	}
	decodeJSON(t, resp, &body)
	if body.ID != 9001 || body.State != "Active" {
		t.Errorf("expected {9001 Active}, got %+v", body)
	}
	if body.CatalogError != "" {
		t.Errorf("expected no catalog_error, got %q", body.CatalogError)
	}
	if body.AzureActivityID == 0 {
		t.Fatal("expected a non-zero azure_activity_id")
	}

	activities, err := st.ListAzureActivities(context.Background(), true)
	mustNoErr(t, err)
	var found *struct {
		Project    *string
		CategoryID *int64
	}
	for _, a := range activities {
		if a.ID == body.AzureActivityID {
			found = &struct {
				Project    *string
				CategoryID *int64
			}{a.Project, a.CategoryID}
		}
	}
	if found == nil {
		t.Fatal("expected the new catalog entry to be present in ListAzureActivities")
	}
	if found.Project == nil || *found.Project != "Mintag" {
		t.Errorf("expected project=Mintag, got %+v", found.Project)
	}
	if found.CategoryID == nil || *found.CategoryID != categoryID {
		t.Errorf("expected category_id=%d, got %+v", categoryID, found.CategoryID)
	}
}

// TestCreateAzureWorkItem_RegistersCatalogEntryWithProjectOnly verifies a
// project-only mapping (no category_id) still registers, with a nil CategoryID.
func TestCreateAzureWorkItem_RegistersCatalogEntryWithProjectOnly(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := azureWorkItemsSuccessServer(t)
	defer azureServer.Close()

	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items", map[string]any{
		"title": "Fix the thing", "area_path": `PROJ\Team`, "iteration_path": `PROJ\Sprint 1`,
		"project": "Mintag",
	})
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		AzureActivityID int64  `json:"azure_activity_id"`
		CatalogError    string `json:"catalog_error"`
	}
	decodeJSON(t, resp, &body)
	if body.CatalogError != "" {
		t.Errorf("expected no catalog_error, got %q", body.CatalogError)
	}
	if body.AzureActivityID == 0 {
		t.Fatal("expected a non-zero azure_activity_id")
	}

	activities, err := st.ListAzureActivities(context.Background(), true)
	mustNoErr(t, err)
	for _, a := range activities {
		if a.ID == body.AzureActivityID {
			if a.CategoryID != nil {
				t.Errorf("expected nil CategoryID, got %v", *a.CategoryID)
			}
			return
		}
	}
	t.Fatal("expected the new catalog entry to be present in ListAzureActivities")
}

// TestCreateAzureWorkItem_NoProjectOrCategorySkipsRegistration verifies that
// omitting both project and category_id never touches the catalog.
func TestCreateAzureWorkItem_NoProjectOrCategorySkipsRegistration(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := azureWorkItemsSuccessServer(t)
	defer azureServer.Close()

	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	before, err := st.ListAzureActivities(context.Background(), true)
	mustNoErr(t, err)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items", map[string]any{
		"title": "Fix the thing", "area_path": `PROJ\Team`, "iteration_path": `PROJ\Sprint 1`,
	})
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		AzureActivityID int64  `json:"azure_activity_id"`
		CatalogError    string `json:"catalog_error"`
	}
	decodeJSON(t, resp, &body)
	if body.AzureActivityID != 0 {
		t.Errorf("expected no azure_activity_id, got %d", body.AzureActivityID)
	}
	if body.CatalogError != "" {
		t.Errorf("expected no catalog_error, got %q", body.CatalogError)
	}

	after, err := st.ListAzureActivities(context.Background(), true)
	mustNoErr(t, err)
	if len(after) != len(before) {
		t.Errorf("expected no new catalog rows, before=%d after=%d", len(before), len(after))
	}
}

// TestCreateAzureWorkItem_InvalidCategoryReportsNonFatalCatalogError verifies
// that a category_id referencing no existing timelog_categories row still
// returns 200 with the created id/state, plus a non-empty catalog_error —
// the Azure work item must never be "lost" just because catalog registration
// failed.
func TestCreateAzureWorkItem_InvalidCategoryReportsNonFatalCatalogError(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := azureWorkItemsSuccessServer(t)
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items", map[string]any{
		"title": "Fix the thing", "area_path": `PROJ\Team`, "iteration_path": `PROJ\Sprint 1`,
		"category_id": 999999,
	})
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		ID              int    `json:"id"`
		State           string `json:"state"`
		AzureActivityID int64  `json:"azure_activity_id"`
		CatalogError    string `json:"catalog_error"`
	}
	decodeJSON(t, resp, &body)
	if body.ID != 9001 || body.State != "Active" {
		t.Errorf("expected the created work item preserved as {9001 Active}, got %+v", body)
	}
	if body.CatalogError == "" {
		t.Error("expected a non-empty catalog_error")
	}
	if body.AzureActivityID != 0 {
		t.Errorf("expected no azure_activity_id on a failed registration, got %d", body.AzureActivityID)
	}
}

// TestGetAzureClassificationTree_Success verifies the tree passthrough and
// that an invalid kind is rejected with 400 before any Azure call.
func TestGetAzureClassificationTree_Success(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case strings.Contains(r.URL.Path, "classificationnodes/areas"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"name":"PROJ","children":[{"name":"Team"}]}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s", r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := get(t, base+"/api/activities/azure-classification-nodes/areas")
	var tree struct {
		Name     string `json:"name"`
		Children []struct {
			Name string `json:"name"`
		} `json:"children"`
	}
	decodeJSON(t, resp, &tree)
	if tree.Name != "PROJ" || len(tree.Children) != 1 || tree.Children[0].Name != "Team" {
		t.Errorf("unexpected tree: %+v", tree)
	}

	badResp, err := http.Get(base + "/api/activities/azure-classification-nodes/bogus")
	mustNoErr(t, err)
	assertStatus(t, badResp, http.StatusBadRequest)
}
