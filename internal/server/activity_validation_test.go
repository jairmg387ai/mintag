package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// closedWorkItemAzureServer fakes the Azure endpoints needed for the
// closed-work-item validation: identity resolution and the batch
// workitems-by-id lookup used by Client.FetchWorkItemsByIDs. state is the
// System.State returned for every id requested.
func closedWorkItemAzureServer(t *testing.T, state string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"authenticatedUser":{"id":"route-user-id","providerDisplayName":"Route User"}}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"value":[{"id":5001,"fields":{"System.Title":"T","System.WorkItemType":"Task","System.State":%q}}]}`, state) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	}))
}

// configureAzure points the test server's Azure client at azureServer and
// sets a token, mirroring the setup already used by work_items_test.go's
// TestCreateAzureWorkItem_Success.
func configureAzure(t *testing.T, azureServerURL string) (base string, st *store.Store) {
	t.Helper()
	base, st = newTestServerWithAzureRedirect(t, azureServerURL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)
	return base, st
}

func createPendingActivity(t *testing.T, st *store.Store) int64 {
	t.Helper()
	a, err := st.CreateActivity(context.Background(), "2026-06-12", 1, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	return a.ID
}

// TestCreateActivity_ClosedWorkItem_RejectedWhenEnabled verifies POST
// /api/activities linking to a Closed work item's catalog entry is rejected
// with 422 once block_closed_work_item is enabled.
func TestCreateActivity_ClosedWorkItem_RejectedWhenEnabled(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := closedWorkItemAzureServer(t, "Closed")
	defer azureServer.Close()
	base, st := configureAzure(t, azureServer.URL)

	entry, err := st.AddAzureActivity(context.Background(), "RUNT2QA", 5001, "Closed WI", "Task", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	putResp := doJSON(t, http.MethodPut, base+"/api/settings/activity-validation", map[string]any{"block_closed_work_item": true})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-06-12", "hours": 1, "project": "RNCEA",
		"category": "Actividades de arquitectura, diseño y código", "registro_diario": "Trabajo",
		"azure_activity_id": entry.ID,
	})
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	resp.Body.Close()
}

// TestCreateActivity_ClosedWorkItem_AllowedWhenDisabled verifies the same
// link succeeds when the validation is off (the default).
func TestCreateActivity_ClosedWorkItem_AllowedWhenDisabled(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := closedWorkItemAzureServer(t, "Closed")
	defer azureServer.Close()
	base, st := configureAzure(t, azureServer.URL)

	entry, err := st.AddAzureActivity(context.Background(), "RUNT2QA", 5001, "Closed WI", "Task", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	resp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-06-12", "hours": 1, "project": "RNCEA",
		"category": "Actividades de arquitectura, diseño y código", "registro_diario": "Trabajo",
		"azure_activity_id": entry.ID,
	})
	assertStatus(t, resp, http.StatusCreated)
	resp.Body.Close()
}

// TestCreateActivity_OpenWorkItem_AlwaysAllowed verifies linking to an open
// (non-closed) work item always succeeds, even with the validation enabled.
func TestCreateActivity_OpenWorkItem_AlwaysAllowed(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := closedWorkItemAzureServer(t, "Active")
	defer azureServer.Close()
	base, st := configureAzure(t, azureServer.URL)

	entry, err := st.AddAzureActivity(context.Background(), "RUNT2QA", 5001, "Open WI", "Task", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	putResp := doJSON(t, http.MethodPut, base+"/api/settings/activity-validation", map[string]any{"block_closed_work_item": true})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPost, base+"/api/activities", map[string]any{
		"date": "2026-06-12", "hours": 1, "project": "RNCEA",
		"category": "Actividades de arquitectura, diseño y código", "registro_diario": "Trabajo",
		"azure_activity_id": entry.ID,
	})
	assertStatus(t, resp, http.StatusCreated)
	resp.Body.Close()
}

// TestPatchActivity_ClosedWorkItem_Rejected verifies PATCHing an existing
// activity's azure_activity_id to a closed work item's catalog entry is also
// rejected — proves the check lives in the shared applyAzureActivityID, not
// duplicated per-handler.
func TestPatchActivity_ClosedWorkItem_Rejected(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := closedWorkItemAzureServer(t, "Closed")
	defer azureServer.Close()
	base, st := configureAzure(t, azureServer.URL)

	entry, err := st.AddAzureActivity(context.Background(), "RUNT2QA", 5001, "Closed WI", "Task", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	putResp := doJSON(t, http.MethodPut, base+"/api/settings/activity-validation", map[string]any{"block_closed_work_item": true})
	assertStatus(t, putResp, http.StatusOK)

	id := createPendingActivity(t, st)
	resp := doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/activities/%d", base, id), map[string]any{
		"azure_activity_id": entry.ID,
	})
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	resp.Body.Close()
}

// TestPatchActivity_ClearingLink_NeverBlocked verifies explicitly clearing a
// link (azure_activity_id: null) is never blocked, even when the validation
// is enabled and the previously-linked work item is closed.
func TestPatchActivity_ClearingLink_NeverBlocked(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := closedWorkItemAzureServer(t, "Closed")
	defer azureServer.Close()
	base, st := configureAzure(t, azureServer.URL)

	entry, err := st.AddAzureActivity(context.Background(), "RUNT2QA", 5001, "Closed WI", "Task", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	// Link while the validation is still off, then enable it before clearing.
	id := createPendingActivity(t, st)
	if err := st.SetActivityAzureActivity(context.Background(), id, &entry.ID); err != nil {
		t.Fatal(err)
	}
	putResp := doJSON(t, http.MethodPut, base+"/api/settings/activity-validation", map[string]any{"block_closed_work_item": true})
	assertStatus(t, putResp, http.StatusOK)

	resp := doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/activities/%d", base, id), map[string]any{
		"azure_activity_id": nil,
	})
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestActivityValidationSettings_GetDefaultsToAllFalse verifies a fresh
// store reports all three toggles as false (off).
func TestActivityValidationSettings_GetDefaultsToAllFalse(t *testing.T) {
	base, _ := newTestServer(t)

	resp := get(t, base+"/api/settings/activity-validation")
	var body store.ActivityValidationSettings
	decodeJSON(t, resp, &body)
	if body.MaxHoursPerEntry || body.WeekendConfirm || body.BlockClosedWorkItem {
		t.Errorf("expected all three settings to default to false, got %+v", body)
	}
}

// TestActivityValidationSettings_PutRoundTrips verifies PUT persists and GET
// reads back the configured values.
func TestActivityValidationSettings_PutRoundTrips(t *testing.T) {
	base, _ := newTestServer(t)

	putResp := doJSON(t, http.MethodPut, base+"/api/settings/activity-validation", map[string]any{
		"max_hours_per_entry": true, "weekend_confirm": true, "block_closed_work_item": false,
	})
	assertStatus(t, putResp, http.StatusOK)
	var putBody store.ActivityValidationSettings
	decodeJSON(t, putResp, &putBody)
	want := store.ActivityValidationSettings{MaxHoursPerEntry: true, WeekendConfirm: true, BlockClosedWorkItem: false}
	if putBody != want {
		t.Errorf("expected %+v, got %+v", want, putBody)
	}

	getResp := get(t, base+"/api/settings/activity-validation")
	var getBody store.ActivityValidationSettings
	decodeJSON(t, getResp, &getBody)
	if getBody != want {
		t.Errorf("expected persisted %+v, got %+v", want, getBody)
	}
}
