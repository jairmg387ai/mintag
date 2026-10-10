package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/azure"
	"github.com/Gentleman-Programming/mintag/internal/store"
)

const correctionBugJSON = `{
  "id": 171191,
  "fields": {
    "System.Title": "Error al radicar",
    "System.WorkItemType": "Bug",
    "System.State": "Activo",
    "System.TeamProject": "ControlesDeCambio",
    "System.AreaPath": "ControlesDeCambio\\RNET",
    "System.IterationPath": "ControlesDeCambio\\Sprint 7",
    "System.AssignedTo": {"displayName": "Dev Uno", "uniqueName": "dev@runt.com.co", "id": "dev-id"}
  },
  "relations": [
    {"rel": "System.LinkTypes.Hierarchy-Forward", "url": "https://dev.azure.com/ORG/_apis/wit/workItems/171306"}
  ]
}`

const correctionChildrenJSON = `{"value":[{"id":171306,"fields":{"System.Title":"Atención y/o Corrección del defecto 171306","System.WorkItemType":"Task","System.State":"Proposed"}}]}`

const correctionNotABugJSON = `{"id": 171191, "fields": {"System.Title": "Tarea", "System.WorkItemType": "Task", "System.TeamProject": "RUNTPRO"}}`

// correctionAzureFake is a stand-in Azure DevOps for the correction-task
// routes: it serves the bug read, its children, the Subárea allowed values,
// classification trees, and records the create POST.
type correctionAzureFake struct {
	t       *testing.T
	bugJSON string

	// Non-zero statuses make the matching Azure call fail with that status.
	bugStatus      int
	childrenStatus int
	createStatus   int
	fieldsStatus   int

	mu            sync.Mutex
	childrenCalls int
	createPath    string
	createOps     []map[string]any
	treePath      string
}

func (f *correctionAzureFake) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(r.URL.Path, "connectiondata"):
		azureConnectionDataOK(w)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/_apis/wit/workitems/171191"):
		if f.bugStatus != 0 {
			w.WriteHeader(f.bugStatus)
			w.Write([]byte(`{"message":"bug read failed"}`)) //nolint:errcheck
			return
		}
		w.Write([]byte(f.bugJSON)) //nolint:errcheck
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/_apis/wit/workitems"):
		f.childrenCalls++
		if f.childrenStatus != 0 {
			w.WriteHeader(f.childrenStatus)
			w.Write([]byte(`{"message":"children read failed"}`)) //nolint:errcheck
			return
		}
		w.Write([]byte(correctionChildrenJSON)) //nolint:errcheck
	case strings.Contains(r.URL.Path, "/_apis/wit/workitemtypes/Task/fields/"):
		f.treePath = r.URL.Path
		if f.fieldsStatus != 0 {
			w.WriteHeader(f.fieldsStatus)
			w.Write([]byte(`{"message":"fields read failed"}`)) //nolint:errcheck
			return
		}
		w.Write([]byte(`{"allowedValues":["Desarrollo","QA"]}`)) //nolint:errcheck
	case strings.Contains(r.URL.Path, "/_apis/wit/classificationnodes/"):
		f.treePath = r.URL.Path
		w.Write([]byte(`{"name":"ControlesDeCambio"}`)) //nolint:errcheck
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/_apis/wit/workitems/$Task"):
		f.createPath = r.URL.Path
		if f.createStatus != 0 {
			w.WriteHeader(f.createStatus)
			w.Write([]byte(`{"message":"create failed"}`)) //nolint:errcheck
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &f.createOps) //nolint:errcheck
		w.Write([]byte(`{"id":171400}`))   //nolint:errcheck
	default:
		f.t.Errorf("unexpected azure request: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusTeapot)
	}
}

func setupCorrectionTaskServer(t *testing.T, bugJSON string) (string, *store.Store, *correctionAzureFake) {
	t.Helper()
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	fake := &correctionAzureFake{t: t, bugJSON: bugJSON}
	azureServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(azureServer.Close)

	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)
	assertStatus(t, doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"}), http.StatusOK)
	return base, st, fake
}

func validCorrectionBody() map[string]any {
	return map[string]any{
		"title":             "Atención y/o Corrección del defecto 171191",
		"description":       "Corregir",
		"subarea":           "Desarrollo",
		"original_estimate": 6,
		"area_path":         `ControlesDeCambio\RNET`,
		"iteration_path":    `ControlesDeCambio\Sprint 7`,
		"assigned_to":       "dev@runt.com.co",
	}
}

func TestBugCorrectionTaskRoutesRequireLocalRequest(t *testing.T) {
	_, st := newTestServer(t)
	h := New(st).Handler()
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/azure/bugs/171191/correction-task-draft"},
		{http.MethodPost, "/api/azure/bugs/171191/correction-task"},
		{http.MethodGet, "/api/azure/work-item-fields/subarea/allowed-values?team_project=X"},
	}
	for _, rt := range routes {
		req := httptest.NewRequest(rt.method, "http://127.0.0.1"+rt.path, strings.NewReader(`{}`))
		req.Host = "evil.example"
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: expected 403, got %d", rt.method, rt.path, rec.Code)
		}
	}
}

func TestGetBugCorrectionTaskDraft_Success(t *testing.T) {
	base, _, _ := setupCorrectionTaskServer(t, correctionBugJSON)

	resp := get(t, base+"/api/azure/bugs/171191/correction-task-draft")
	assertStatus(t, resp, http.StatusOK)
	var body struct {
		BugID          int    `json:"bug_id"`
		BugTitle       string `json:"bug_title"`
		TeamProject    string `json:"team_project"`
		AreaPath       string `json:"area_path"`
		IterationPath  string `json:"iteration_path"`
		SuggestedTitle string `json:"suggested_title"`
		AssignedTo     struct {
			DisplayName string `json:"display_name"`
			UniqueName  string `json:"unique_name"`
		} `json:"assigned_to"`
		Existing []struct {
			ID    int    `json:"id"`
			State string `json:"state"`
		} `json:"existing_correction_tasks"`
	}
	decodeJSON(t, resp, &body)
	if body.BugID != 171191 || body.BugTitle != "Error al radicar" || body.TeamProject != "ControlesDeCambio" {
		t.Errorf("unexpected bug fields %+v", body)
	}
	if body.AreaPath != `ControlesDeCambio\RNET` || body.IterationPath != `ControlesDeCambio\Sprint 7` {
		t.Errorf("unexpected paths %+v", body)
	}
	if body.SuggestedTitle != "Atención y/o Corrección del defecto 171191" {
		t.Errorf("unexpected suggested title %q", body.SuggestedTitle)
	}
	if body.AssignedTo.DisplayName != "Dev Uno" || body.AssignedTo.UniqueName != "dev@runt.com.co" {
		t.Errorf("unexpected assignee %+v", body.AssignedTo)
	}
	if len(body.Existing) != 1 || body.Existing[0].ID != 171306 {
		t.Errorf("expected existing correction task 171306, got %+v", body.Existing)
	}
}

func TestGetBugCorrectionTaskDraft_NotABug(t *testing.T) {
	base, _, _ := setupCorrectionTaskServer(t, correctionNotABugJSON)
	resp, err := http.Get(base + "/api/azure/bugs/171191/correction-task-draft")
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusBadRequest)
	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["code"] != "not_a_bug" {
		t.Errorf("expected not_a_bug, got %v", body)
	}
}

func TestGetSubareaAllowedValues(t *testing.T) {
	base, _, fake := setupCorrectionTaskServer(t, correctionBugJSON)

	missing, err := http.Get(base + "/api/azure/work-item-fields/subarea/allowed-values")
	mustNoErr(t, err)
	assertStatus(t, missing, http.StatusBadRequest)
	missing.Body.Close()

	resp := get(t, base+"/api/azure/work-item-fields/subarea/allowed-values?team_project=ControlesDeCambio")
	assertStatus(t, resp, http.StatusOK)
	var body struct {
		Values []string `json:"values"`
	}
	decodeJSON(t, resp, &body)
	if strings.Join(body.Values, ",") != "Desarrollo,QA" {
		t.Errorf("unexpected values %v", body.Values)
	}
	if fake.treePath != "/RUNT2PSW/ControlesDeCambio/_apis/wit/workitemtypes/Task/fields/"+azure.FieldSubarea {
		t.Errorf("unexpected azure path %q", fake.treePath)
	}
}

func TestGetClassificationTree_TeamProjectOverride(t *testing.T) {
	base, _, fake := setupCorrectionTaskServer(t, correctionBugJSON)

	resp := get(t, base+"/api/activities/azure-classification-nodes/iterations?team_project=ControlesDeCambio")
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
	if fake.treePath != "/RUNT2PSW/ControlesDeCambio/_apis/wit/classificationnodes/iterations" {
		t.Errorf("expected bug team project in path, got %q", fake.treePath)
	}

	resp = get(t, base+"/api/activities/azure-classification-nodes/areas")
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
	if fake.treePath != "/RUNT2PSW/RUNTPRO/_apis/wit/classificationnodes/areas" {
		t.Errorf("expected default team project in path, got %q", fake.treePath)
	}
}

func TestCreateBugCorrectionTask_ValidationErrors(t *testing.T) {
	base, _, fake := setupCorrectionTaskServer(t, correctionBugJSON)

	cases := map[string]func(map[string]any){
		"title":             func(b map[string]any) { b["title"] = " " },
		"subarea":           func(b map[string]any) { delete(b, "subarea") },
		"original_estimate": func(b map[string]any) { b["original_estimate"] = 0 },
		"iteration_path":    func(b map[string]any) { b["iteration_path"] = "" },
		"assigned_to":       func(b map[string]any) { b["assigned_to"] = "" },
	}
	for field, mutate := range cases {
		t.Run(field, func(t *testing.T) {
			b := validCorrectionBody()
			mutate(b)
			resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/171191/correction-task", b)
			assertStatus(t, resp, http.StatusBadRequest)
			var body map[string]any
			decodeJSON(t, resp, &body)
			if body["code"] != "validation_error" || body["field"] != field {
				t.Errorf("expected validation_error for %s, got %v", field, body)
			}
		})
	}
	if fake.createPath != "" {
		t.Error("no work item must be created on validation errors")
	}
}

func TestCreateBugCorrectionTask_NotABug(t *testing.T) {
	base, _, fake := setupCorrectionTaskServer(t, correctionNotABugJSON)
	resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/171191/correction-task", validCorrectionBody())
	assertStatus(t, resp, http.StatusBadRequest)
	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["code"] != "not_a_bug" {
		t.Errorf("expected not_a_bug, got %v", body)
	}
	if fake.createPath != "" {
		t.Error("no work item must be created for a non-bug")
	}
}

func TestCreateBugCorrectionTask_CreatesInBugProjectWithoutCatalog(t *testing.T) {
	base, st, fake := setupCorrectionTaskServer(t, correctionBugJSON)

	resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/171191/correction-task", validCorrectionBody())
	assertStatus(t, resp, http.StatusOK)
	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["id"] != float64(171400) || body["state"] != "Proposed" {
		t.Errorf("unexpected response %v", body)
	}
	if _, ok := body["azure_activity_id"]; ok {
		t.Errorf("expected no catalog entry, got %v", body)
	}
	if fake.createPath != "/RUNT2PSW/ControlesDeCambio/_apis/wit/workitems/$Task" {
		t.Errorf("expected creation in the bug's team project, got %q", fake.createPath)
	}

	var rel map[string]any
	var subarea any
	for _, op := range fake.createOps {
		switch op["path"] {
		case "/relations/-":
			rel, _ = op["value"].(map[string]any)
		case "/fields/" + azure.FieldSubarea:
			subarea = op["value"]
		}
	}
	if rel == nil || rel["rel"] != "System.LinkTypes.Hierarchy-Reverse" || !strings.HasSuffix(rel["url"].(string), "/_apis/wit/workItems/171191") {
		t.Errorf("expected parent link to the bug, got %v", rel)
	}
	if subarea != "Desarrollo" {
		t.Errorf("expected subarea Desarrollo, got %v", subarea)
	}

	list, err := st.ListAzureActivities(context.Background(), true)
	mustNoErr(t, err)
	for _, a := range list {
		if a.WorkItemID == 171400 {
			t.Errorf("expected no catalog entry for the created task, got %+v", a)
		}
	}
}

func TestCreateBugCorrectionTask_AddsToCatalogWithParent(t *testing.T) {
	base, st, _ := setupCorrectionTaskServer(t, correctionBugJSON)

	b := validCorrectionBody()
	b["add_to_catalog"] = true
	resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/171191/correction-task", b)
	assertStatus(t, resp, http.StatusOK)
	var body map[string]any
	decodeJSON(t, resp, &body)
	if _, ok := body["catalog_error"]; ok {
		t.Fatalf("unexpected catalog_error %v", body)
	}
	actID, ok := body["azure_activity_id"].(float64)
	if !ok {
		t.Fatalf("expected azure_activity_id, got %v", body)
	}

	a, err := st.GetAzureActivity(context.Background(), int64(actID))
	mustNoErr(t, err)
	if a.WorkItemID != 171400 || a.WorkItemType != "Task" || a.Label != "Atención y/o Corrección del defecto 171191" {
		t.Errorf("unexpected catalog entry %+v", a)
	}
	if a.ParentWorkItemID == nil || *a.ParentWorkItemID != 171191 || a.ParentTitle != "Error al radicar" || a.ParentType != "Bug" {
		t.Errorf("expected parent bug on catalog entry, got %+v", a)
	}
}

func TestCreateBugCorrectionTask_CatalogStoresEffectiveEstimate(t *testing.T) {
	base, st, _ := setupCorrectionTaskServer(t, correctionBugJSON)

	b := validCorrectionBody()
	b["add_to_catalog"] = true
	b["original_estimate"] = 6
	resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/171191/correction-task", b)
	assertStatus(t, resp, http.StatusOK)
	var body map[string]any
	decodeJSON(t, resp, &body)
	actID, ok := body["azure_activity_id"].(float64)
	if !ok {
		t.Fatalf("expected azure_activity_id, got %v", body)
	}

	a, err := st.GetAzureActivity(context.Background(), int64(actID))
	mustNoErr(t, err)
	// Validation rejects estimates <= 0, so the effective estimate is the one sent.
	if want := azure.EffectiveOriginalEstimate(6); a.OriginalEstimate != want || want != 6 {
		t.Errorf("catalog OriginalEstimate = %v, want %v", a.OriginalEstimate, want)
	}
}

func TestCreateBugCorrectionTask_CatalogFailureIsNonFatal(t *testing.T) {
	base, _, _ := setupCorrectionTaskServer(t, correctionBugJSON)

	b := validCorrectionBody()
	b["add_to_catalog"] = true
	b["category_id"] = 99999 // does not exist -> catalog add fails
	resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/171191/correction-task", b)
	assertStatus(t, resp, http.StatusOK)
	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["id"] != float64(171400) {
		t.Errorf("expected created id despite catalog failure, got %v", body)
	}
	if _, ok := body["catalog_error"]; !ok {
		t.Errorf("expected catalog_error, got %v", body)
	}
}

func TestGetBugCorrectionTaskDraft_BugReadFailures(t *testing.T) {
	cases := []struct {
		name       string
		azure      int
		wantStatus int
	}{
		{"azure error maps to 502", http.StatusInternalServerError, http.StatusBadGateway},
		{"missing bug maps to 404", http.StatusNotFound, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, _, fake := setupCorrectionTaskServer(t, correctionBugJSON)
			fake.bugStatus = tc.azure
			resp, err := http.Get(base + "/api/azure/bugs/171191/correction-task-draft")
			mustNoErr(t, err)
			defer resp.Body.Close()
			assertStatus(t, resp, tc.wantStatus)
		})
	}
}

func TestGetBugCorrectionTaskDraft_ChildrenReadFailureIsAWarning(t *testing.T) {
	base, _, fake := setupCorrectionTaskServer(t, correctionBugJSON)
	fake.childrenStatus = http.StatusInternalServerError

	resp := get(t, base+"/api/azure/bugs/171191/correction-task-draft")
	assertStatus(t, resp, http.StatusOK)
	var body struct {
		BugID              int               `json:"bug_id"`
		TeamProject        string            `json:"team_project"`
		Existing           []json.RawMessage `json:"existing_correction_tasks"`
		ExistingTasksError string            `json:"existing_tasks_error"`
	}
	decodeJSON(t, resp, &body)
	if body.BugID != 171191 || body.TeamProject != "ControlesDeCambio" {
		t.Errorf("expected bug data despite children failure, got %+v", body)
	}
	if body.Existing == nil || len(body.Existing) != 0 {
		t.Errorf("expected empty existing_correction_tasks array, got %v", body.Existing)
	}
	if body.ExistingTasksError == "" {
		t.Error("expected existing_tasks_error to be set")
	}
}

func TestGetBugCorrectionTaskDraft_OmitsWarningWhenChildrenReadSucceeds(t *testing.T) {
	base, _, _ := setupCorrectionTaskServer(t, correctionBugJSON)
	resp := get(t, base+"/api/azure/bugs/171191/correction-task-draft")
	assertStatus(t, resp, http.StatusOK)
	var body map[string]any
	decodeJSON(t, resp, &body)
	if _, ok := body["existing_tasks_error"]; ok {
		t.Errorf("expected no existing_tasks_error, got %v", body["existing_tasks_error"])
	}
}

func TestCreateBugCorrectionTask_DoesNotDependOnChildrenRead(t *testing.T) {
	base, _, fake := setupCorrectionTaskServer(t, correctionBugJSON)
	fake.childrenStatus = http.StatusInternalServerError

	resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/171191/correction-task", validCorrectionBody())
	assertStatus(t, resp, http.StatusOK)
	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["id"] != float64(171400) {
		t.Errorf("expected task created despite children failure, got %v", body)
	}
	if fake.childrenCalls != 0 {
		t.Errorf("create must not read the bug's children, got %d calls", fake.childrenCalls)
	}
}

func TestCreateBugCorrectionTask_BugReadFailures(t *testing.T) {
	cases := []struct {
		name       string
		azure      int
		wantStatus int
	}{
		{"azure error maps to 502", http.StatusInternalServerError, http.StatusBadGateway},
		{"missing bug maps to 404", http.StatusNotFound, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, _, fake := setupCorrectionTaskServer(t, correctionBugJSON)
			fake.bugStatus = tc.azure
			resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/171191/correction-task", validCorrectionBody())
			defer resp.Body.Close()
			assertStatus(t, resp, tc.wantStatus)
			if fake.createPath != "" {
				t.Error("no work item must be created when the bug cannot be read")
			}
		})
	}
}

func TestCreateBugCorrectionTask_AzureCreateFailureIs502(t *testing.T) {
	base, st, fake := setupCorrectionTaskServer(t, correctionBugJSON)
	fake.createStatus = http.StatusInternalServerError

	b := validCorrectionBody()
	b["add_to_catalog"] = true
	resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/171191/correction-task", b)
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusBadGateway)

	list, err := st.ListAzureActivities(context.Background(), true)
	mustNoErr(t, err)
	for _, a := range list {
		if a.WorkItemID == 171400 {
			t.Errorf("expected no catalog entry after a failed create, got %+v", a)
		}
	}
}

func TestGetSubareaAllowedValues_AzureFailureIs502(t *testing.T) {
	base, _, fake := setupCorrectionTaskServer(t, correctionBugJSON)
	fake.fieldsStatus = http.StatusInternalServerError

	resp, err := http.Get(base + "/api/azure/work-item-fields/subarea/allowed-values?team_project=ControlesDeCambio")
	mustNoErr(t, err)
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusBadGateway)
}
