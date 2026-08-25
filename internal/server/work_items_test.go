package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/store"
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

// TestGetAzureWorkItemStates_Success verifies the ids query param is parsed
// and forwarded, and the response carries the resolved title/type/state.
func TestGetAzureWorkItemStates_Success(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	var workitemsQuery string
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case strings.Contains(r.URL.Path, "/_apis/wit/workitems"):
			workitemsQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"count":2,"value":[
				{"id":101,"fields":{"System.Title":"Fix login bug","System.WorkItemType":"Bug","System.State":"Active"}},
				{"id":202,"fields":{"System.Title":"Add export button","System.WorkItemType":"Task","System.State":"New"}}
			]}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s", r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := get(t, base+"/api/activities/azure-work-items/states?ids=101,202")
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		Org         string `json:"org"`
		TeamProject string `json:"team_project"`
		Items       []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
			Type  string `json:"type"`
			State string `json:"state"`
		} `json:"items"`
	}
	decodeJSON(t, resp, &body)
	if !strings.Contains(workitemsQuery, "ids=101,202") {
		t.Errorf("expected workitems query to forward both ids, got %s", workitemsQuery)
	}
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 items, got %+v", body.Items)
	}
	if body.Items[0].ID != 101 || body.Items[0].State != "Active" {
		t.Errorf("unexpected first item: %+v", body.Items[0])
	}
	if body.TeamProject != "RUNTPRO" {
		t.Errorf("expected default team_project=RUNTPRO, got %q", body.TeamProject)
	}
}

// TestGetAzureWorkItemStates_PersistsLiveStateAndType verifies a states
// refresh writes the fetched state/type back into the local azure_activities
// catalog entry for each work item id that has one, so the portal has a
// "last known" state/type on load without needing another manual refresh.
func TestGetAzureWorkItemStates_PersistsLiveStateAndType(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case strings.Contains(r.URL.Path, "/_apis/wit/workitems"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"count":1,"value":[
				{"id":303,"fields":{"System.Title":"Reclassified item","System.WorkItemType":"Bug","System.State":"Closed"}}
			]}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s", r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	ctx := context.Background()
	added, err := st.AddAzureActivity(ctx, "RUNT2QA", 303, "Reclassified item", "Task", store.AzureActivityMapping{})
	mustNoErr(t, err)

	resp := get(t, base+"/api/activities/azure-work-items/states?ids=303")
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	updated, err := st.GetAzureActivity(ctx, added.ID)
	mustNoErr(t, err)
	if updated.LastKnownState != "Closed" {
		t.Errorf("expected last_known_state=%q persisted, got %q", "Closed", updated.LastKnownState)
	}
	if updated.WorkItemType != "Bug" {
		t.Errorf("expected work_item_type synced to %q, got %q", "Bug", updated.WorkItemType)
	}
}

// TestGetAzureWorkItemStates_EmptyIDsReturnsEmptyItemsWithoutAzureCall
// verifies a missing/empty ids param short-circuits without hitting Azure's
// workitems endpoint (mirrors FetchWorkItemsByIDs' own empty-slice guard).
func TestGetAzureWorkItemStates_EmptyIDsReturnsEmptyItemsWithoutAzureCall(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	workitemsCalled := false
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "connectiondata") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
			return
		}
		workitemsCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"count":0,"value":[]}`)) //nolint:errcheck
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := get(t, base+"/api/activities/azure-work-items/states")
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		Items []any `json:"items"`
	}
	decodeJSON(t, resp, &body)
	if len(body.Items) != 0 {
		t.Errorf("expected empty items, got %+v", body.Items)
	}
	if workitemsCalled {
		t.Error("expected no workitems call for an empty ids param")
	}
}

// TestGetAzureWorkItemStates_InvalidIDReturns400 verifies a non-integer id
// in the list is rejected before any Azure call.
func TestGetAzureWorkItemStates_InvalidIDReturns400(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureCalled := false
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "connectiondata") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
			return
		}
		azureCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)
	azureCalled = false // reset: the PUT above already resolved identity

	resp, err := http.Get(base + "/api/activities/azure-work-items/states?ids=101,notanumber")
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusBadRequest)
	if azureCalled {
		t.Error("expected no Azure call for an invalid id")
	}
}

// TestGetAzureWorkItemStates_NotConfigured mirrors the 503 guard already
// used by the other azure-work-items endpoints.
func TestGetAzureWorkItemStates_NotConfigured(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	base, _ := newTestServer(t)

	resp, err := http.Get(base + "/api/activities/azure-work-items/states?ids=1")
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusServiceUnavailable)
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

// TestCloseAzureWorkItem_Success exercises the full fetch -> effort sync ->
// two-step close path through the REST handler.
func TestCloseAzureWorkItem_Success(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	var patchCount int
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/555"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555,"fields":{"System.Title":"T","System.WorkItemType":"Task","System.State":"Active","System.AreaPath":"A","System.IterationPath":"I","Microsoft.VSTS.Scheduling.OriginalEstimate":8,"System.AssignedTo":{"displayName":"Name","id":"id"}}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "TimeLogData/Documents"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"workItemId":555,"minutes":120}]`)) //nolint:errcheck
		case r.Method == http.MethodPatch:
			patchCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/555/close", nil)
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		State         string  `json:"state"`
		HoursSynced   float64 `json:"hours_synced"`
		AlreadyClosed bool    `json:"already_closed"`
		EffortSyncErr string  `json:"effort_sync_error"`
	}
	decodeJSON(t, resp, &body)
	if body.State != "Closed" || body.HoursSynced != 2 || body.AlreadyClosed {
		t.Errorf("unexpected response: %+v", body)
	}
	if body.EffortSyncErr != "" {
		t.Errorf("expected no effort_sync_error, got %q", body.EffortSyncErr)
	}
	// 1 effort-sync PATCH + 2 close PATCHes = 3.
	if patchCount != 3 {
		t.Errorf("expected 3 PATCH calls, got %d", patchCount)
	}
}

// TestCloseAzureWorkItem_AlreadyClosed verifies the no-mutation info path:
// no TimeLog fetch, no PATCH, just an already_closed report.
func TestCloseAzureWorkItem_AlreadyClosed(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/555"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555,"fields":{"System.Title":"T","System.WorkItemType":"Task","System.State":"Closed","System.AssignedTo":{"displayName":"Name","id":"id"}}}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request (no TimeLog/PATCH call expected): %s %s", r.Method, r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/555/close", nil)
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		State         string  `json:"state"`
		HoursSynced   float64 `json:"hours_synced"`
		AlreadyClosed bool    `json:"already_closed"`
	}
	decodeJSON(t, resp, &body)
	if body.State != "Closed" || body.HoursSynced != 0 || !body.AlreadyClosed {
		t.Errorf("unexpected response: %+v", body)
	}
}

// TestCloseAzureWorkItem_RejectsWhenNotAssignedToCaller verifies a Task
// assigned to someone else is rejected with 403, and that no TimeLog fetch
// or PATCH is attempted — the same "transversal task" scenario the
// ensureAssignedToCaller doc comment describes (a TL or teammate registering
// time against a task that isn't theirs to close).
func TestCloseAzureWorkItem_RejectsWhenNotAssignedToCaller(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/555"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555,"fields":{"System.Title":"T","System.WorkItemType":"Task","System.State":"Active","System.AssignedTo":{"displayName":"Someone Else","id":"other-id"}}}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request (no TimeLog/PATCH call expected): %s %s", r.Method, r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/555/close", nil)
	assertStatus(t, resp, http.StatusForbidden)
	resp.Body.Close()
}

// TestCloseAzureWorkItem_RejectsWhenUnassigned verifies an unassigned Task
// (no System.AssignedTo at all) is also rejected — "no one" is not "me".
func TestCloseAzureWorkItem_RejectsWhenUnassigned(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/555"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555,"fields":{"System.Title":"T","System.WorkItemType":"Task","System.State":"Active"}}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request (no TimeLog/PATCH call expected): %s %s", r.Method, r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/555/close", nil)
	assertStatus(t, resp, http.StatusForbidden)
	resp.Body.Close()
}

// TestRecreateAzureWorkItem_RejectsWhenNotAssignedToCaller mirrors the Close
// rejection above for Recreate, which also closes the original work item.
func TestRecreateAzureWorkItem_RejectsWhenNotAssignedToCaller(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/555"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555,"fields":{"System.Title":"T","System.WorkItemType":"Task","System.State":"Active","System.AssignedTo":{"displayName":"Someone Else","id":"other-id"}}}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request (no TimeLog/PATCH/create call expected): %s %s", r.Method, r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/555/recreate", nil)
	assertStatus(t, resp, http.StatusForbidden)
	resp.Body.Close()
}

// TestCloseAzureWorkItem_UnresolvedIdentityFallsOpen verifies that when the
// caller's identity was never resolved (UserID empty — a PAT-based setup
// that never configured MINTAG_AZURE_USER_ID), ensureAssignedToCaller cannot
// check assignment and lets the close proceed rather than blocking every
// such setup — see its doc comment for why falling open is intentional here.
func TestCloseAzureWorkItem_UnresolvedIdentityFallsOpen(t *testing.T) {
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/555"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555,"fields":{"System.Title":"T","System.WorkItemType":"Task","System.State":"Closed"}}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer azureServer.Close()

	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "pat-token")
	t.Setenv("MINTAG_AZURE_USER", "")
	t.Setenv("MINTAG_AZURE_USER_ID", "")

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/555/close", nil)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestCloseAzureWorkItem_NotFound verifies a 400/404 from Azure surfaces as
// a 404 through the handler.
func TestCloseAzureWorkItem_NotFound(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "connectiondata") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer azureServer.Close()

	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/9999/close", nil)
	assertStatus(t, resp, http.StatusNotFound)
}

// TestRecreateAzureWorkItem_Success_WithCatalogReassignment covers the full
// close-old -> create-new -> reassign-catalog happy path.
func TestRecreateAzureWorkItem_Success_WithCatalogReassignment(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/555"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555,"fields":{"System.Title":"Old title","System.WorkItemType":"Task","System.State":"Active","System.AreaPath":"A","System.IterationPath":"I","Microsoft.VSTS.Scheduling.OriginalEstimate":8,"System.AssignedTo":{"displayName":"Name","id":"id"}}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "TimeLogData/Documents"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"workItemId":555,"minutes":60}]`)) //nolint:errcheck
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/_apis/wit/workitems/$Task"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":777}`)) //nolint:errcheck
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":1}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	ctx := context.Background()
	added, err := st.AddAzureActivity(ctx, "ORG", 555, "Old title", "Task", store.AzureActivityMapping{})
	mustNoErr(t, err)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/555/recreate", nil)
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		ID                int     `json:"id"`
		State             string  `json:"state"`
		HoursSynced       float64 `json:"hours_synced"`
		CatalogReassigned bool    `json:"catalog_reassigned"`
		AzureActivityID   int64   `json:"azure_activity_id"`
		ActivationError   string  `json:"activation_error"`
		CatalogError      string  `json:"catalog_error"`
	}
	decodeJSON(t, resp, &body)
	if body.ID != 777 || body.State != "Active" || body.HoursSynced != 1 {
		t.Errorf("unexpected response: %+v", body)
	}
	if !body.CatalogReassigned || body.AzureActivityID != added.ID {
		t.Errorf("expected catalog reassignment to azure_activity_id=%d, got %+v", added.ID, body)
	}
	if body.ActivationError != "" || body.CatalogError != "" {
		t.Errorf("expected no errors, got %+v", body)
	}

	reassigned, err := st.FindAzureActivityByWorkItemID(ctx, 777)
	mustNoErr(t, err)
	if reassigned.ID != added.ID {
		t.Errorf("expected the same catalog row to now resolve for the new id, got %+v", reassigned)
	}
	if _, err := st.FindAzureActivityByWorkItemID(ctx, 555); !errors.Is(err, store.ErrAzureActivityNotFound) {
		t.Errorf("expected the old id to no longer resolve, got %v", err)
	}
}

// TestRecreateAzureWorkItem_NoCatalogEntry_NoReassignment verifies recreate
// works fine (and reports catalog_reassigned=false, no error) when the old
// work item was never catalog-registered.
func TestRecreateAzureWorkItem_NoCatalogEntry_NoReassignment(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/321"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":321,"fields":{"System.Title":"Unregistered","System.WorkItemType":"Task","System.State":"Active","System.AreaPath":"A","System.IterationPath":"I","Microsoft.VSTS.Scheduling.OriginalEstimate":8,"System.AssignedTo":{"displayName":"Name","id":"id"}}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "TimeLogData/Documents"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`)) //nolint:errcheck
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/_apis/wit/workitems/$Task"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":888}`)) //nolint:errcheck
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":1}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer azureServer.Close()

	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/321/recreate", nil)
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		ID                int    `json:"id"`
		CatalogReassigned bool   `json:"catalog_reassigned"`
		CatalogError      string `json:"catalog_error"`
	}
	decodeJSON(t, resp, &body)
	if body.ID != 888 {
		t.Errorf("unexpected id: %+v", body)
	}
	if body.CatalogReassigned {
		t.Error("expected catalog_reassigned=false when no catalog entry existed for the old id")
	}
	if body.CatalogError != "" {
		t.Errorf("expected no catalog_error when there was simply nothing to reassign, got %q", body.CatalogError)
	}
	_ = st
}

// TestRecreateAzureWorkItem_CloseFails_AbortsBeforeCreate verifies that a
// failure to close the old work item aborts the whole recreate — a new work
// item must never be created if the old one couldn't be closed (that would
// leave two open work items for the same effort).
func TestRecreateAzureWorkItem_CloseFails_AbortsBeforeCreate(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	createCalled := false
	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/555"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555,"fields":{"System.Title":"T","System.WorkItemType":"Task","System.State":"Active","System.AreaPath":"A","System.IterationPath":"I","Microsoft.VSTS.Scheduling.OriginalEstimate":8,"System.AssignedTo":{"displayName":"Name","id":"id"}}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "TimeLogData/Documents"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`)) //nolint:errcheck
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/_apis/wit/workitems/$Task"):
			createCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":999}`)) //nolint:errcheck
		case r.Method == http.MethodPatch:
			// Every PATCH fails — including the close attempt.
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

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/555/recreate", nil)
	assertStatus(t, resp, http.StatusBadGateway)

	if createCalled {
		t.Error("a new work item must never be created when closing the old one failed")
	}
}

// TestRecreateAzureWorkItem_ActivationPartialFailure mirrors
// TestCreateAzureWorkItem's partial-success handling: the new work item
// exists (Proposed) even though activation failed, so this is a 200 with
// activation_error, never a 5xx.
func TestRecreateAzureWorkItem_ActivationPartialFailure(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/555"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555,"fields":{"System.Title":"T","System.WorkItemType":"Task","System.State":"Active","System.AreaPath":"A","System.IterationPath":"I","Microsoft.VSTS.Scheduling.OriginalEstimate":8,"System.AssignedTo":{"displayName":"Name","id":"id"}}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "TimeLogData/Documents"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`)) //nolint:errcheck
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/_apis/wit/workitems/$Task"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":444}`)) //nolint:errcheck
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/555"):
			// Closing the OLD item succeeds.
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":555}`)) //nolint:errcheck
		case r.Method == http.MethodPatch:
			// Activating the NEW item (444) fails.
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

	resp := doJSON(t, http.MethodPost, base+"/api/activities/azure-work-items/555/recreate", nil)
	assertStatus(t, resp, http.StatusOK)

	var body struct {
		ID              int    `json:"id"`
		State           string `json:"state"`
		ActivationError string `json:"activation_error"`
	}
	decodeJSON(t, resp, &body)
	if body.ID != 444 || body.State != "Proposed" || body.ActivationError == "" {
		t.Errorf("expected partial success (444, Proposed, non-empty activation_error), got %+v", body)
	}
}
