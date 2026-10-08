package azure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newCorrectionTestClient(serverURL string) *Client {
	return &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "RUNTPRO", User: "Caller"},
		http: &http.Client{Transport: redirectToServer(serverURL)},
	}
}

func validCorrectionInput() BugCorrectionTaskInput {
	return BugCorrectionTaskInput{
		BugID:            171191,
		TeamProject:      "ControlesDeCambio",
		Title:            "Atención y/o Corrección del defecto 171191",
		Description:      "Corregir validación",
		AreaPath:         `ControlesDeCambio\RNET`,
		IterationPath:    `ControlesDeCambio\Sprint 7 Semana 41`,
		AssignedTo:       "dev@runt.com.co",
		Subarea:          "Desarrollo",
		OriginalEstimate: 6,
	}
}

func TestCreateBugCorrectionTask_RequestShapeAndPayload(t *testing.T) {
	var method, path, contentType string
	var ops []patchOp
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, contentType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &ops); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":171400}`)) //nolint:errcheck
	}))
	defer srv.Close()

	got, err := newCorrectionTestClient(srv.URL).CreateBugCorrectionTask(context.Background(), validCorrectionInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 171400 || got.State != "Proposed" {
		t.Errorf("expected {171400 Proposed}, got %+v", got)
	}
	if method != http.MethodPost {
		t.Errorf("expected POST, got %s", method)
	}
	// Created in the BUG's team project, not the configured RUNTPRO.
	if path != "/ORG/ControlesDeCambio/_apis/wit/workitems/$Task" {
		t.Errorf("unexpected path %q", path)
	}
	if contentType != "application/json-patch+json" {
		t.Errorf("expected json-patch content type, got %q", contentType)
	}

	values := opValues(ops)
	want := map[string]any{
		"/fields/System.Title":                               "Atención y/o Corrección del defecto 171191",
		"/fields/System.Description":                         "Corregir validación",
		"/fields/System.AreaPath":                            `ControlesDeCambio\RNET`,
		"/fields/System.IterationPath":                       `ControlesDeCambio\Sprint 7 Semana 41`,
		"/fields/System.AssignedTo":                          "dev@runt.com.co",
		"/fields/System.State":                               "Proposed",
		"/fields/System.Reason":                              "New",
		"/fields/Microsoft.VSTS.Scheduling.OriginalEstimate": float64(6),
		"/fields/Microsoft.VSTS.Scheduling.RemainingWork":    float64(6),
		"/fields/Microsoft.VSTS.CMMI.TaskType":               "Planned",
		"/fields/Microsoft.VSTS.Common.Priority":             float64(2),
		"/fields/" + FieldSubarea:                            "Desarrollo",
	}
	for p, w := range want {
		if g, ok := values[p]; !ok {
			t.Errorf("missing op for path %q", p)
		} else if g != w {
			t.Errorf("path %q: expected %v, got %v", p, w, g)
		}
	}

	rel, ok := values["/relations/-"].(map[string]any)
	if !ok {
		t.Fatalf("missing parent relation op, ops=%+v", ops)
	}
	if rel["rel"] != "System.LinkTypes.Hierarchy-Reverse" {
		t.Errorf("expected Hierarchy-Reverse relation, got %v", rel["rel"])
	}
	if rel["url"] != "https://dev.azure.com/ORG/_apis/wit/workItems/171191" {
		t.Errorf("unexpected relation url %v", rel["url"])
	}
}

func TestCreateBugCorrectionTask_OmitsBlankDescription(t *testing.T) {
	var ops []patchOp
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &ops)  //nolint:errcheck
		w.Write([]byte(`{"id":1}`)) //nolint:errcheck
	}))
	defer srv.Close()

	in := validCorrectionInput()
	in.Description = "   "
	if _, err := newCorrectionTestClient(srv.URL).CreateBugCorrectionTask(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := opValues(ops)["/fields/System.Description"]; ok {
		t.Error("expected no description op when blank")
	}
}

func TestCreateBugCorrectionTask_ValidatesRequiredFieldsWithoutHTTPCall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer srv.Close()

	cases := map[string]func(*BugCorrectionTaskInput){
		"bug id":       func(in *BugCorrectionTaskInput) { in.BugID = 0 },
		"team project": func(in *BugCorrectionTaskInput) { in.TeamProject = " " },
		"title":        func(in *BugCorrectionTaskInput) { in.Title = "" },
		"area":         func(in *BugCorrectionTaskInput) { in.AreaPath = "" },
		"iteration":    func(in *BugCorrectionTaskInput) { in.IterationPath = "" },
		"assignee":     func(in *BugCorrectionTaskInput) { in.AssignedTo = "" },
		"subarea":      func(in *BugCorrectionTaskInput) { in.Subarea = "" },
		"estimate":     func(in *BugCorrectionTaskInput) { in.OriginalEstimate = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := validCorrectionInput()
			mutate(&in)
			if _, err := newCorrectionTestClient(srv.URL).CreateBugCorrectionTask(context.Background(), in); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if called {
		t.Error("no HTTP call expected for invalid input")
	}
}

func TestCreateBugCorrectionTask_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"TF401320: Rule Error for field Subárea"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	_, err := newCorrectionTestClient(srv.URL).CreateBugCorrectionTask(context.Background(), validCorrectionInput())
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestFetchTaskFieldAllowedValues(t *testing.T) {
	var path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"referenceName":"` + FieldSubarea + `","allowedValues":["Arquitectura","Desarrollo","QA"]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	got, err := newCorrectionTestClient(srv.URL).FetchTaskFieldAllowedValues(context.Background(), "ControlesDeCambio", FieldSubarea)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(got, ",") != "Arquitectura,Desarrollo,QA" {
		t.Errorf("unexpected values %v", got)
	}
	if path != "/ORG/ControlesDeCambio/_apis/wit/workitemtypes/Task/fields/"+FieldSubarea {
		t.Errorf("unexpected path %q", path)
	}
	if !strings.Contains(query, "$expand=allowedValues") {
		t.Errorf("expected $expand=allowedValues, got %q", query)
	}
}

func TestFetchTaskFieldAllowedValues_RequiresTeamProject(t *testing.T) {
	c := newCorrectionTestClient("http://unused")
	if _, err := c.FetchTaskFieldAllowedValues(context.Background(), " ", FieldSubarea); err == nil {
		t.Fatal("expected error for blank team project")
	}
}

func TestFetchClassificationTreeForProject_UsesGivenProject(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(`{"name":"ControlesDeCambio","children":[{"name":"Sprint 7"}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	node, err := newCorrectionTestClient(srv.URL).FetchClassificationTreeForProject(context.Background(), "ControlesDeCambio", "iterations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Name != "ControlesDeCambio" || len(node.Children) != 1 {
		t.Errorf("unexpected node %+v", node)
	}
	if path != "/ORG/ControlesDeCambio/_apis/wit/classificationnodes/iterations" {
		t.Errorf("unexpected path %q", path)
	}
}

func TestFetchClassificationTree_DefaultsToConfiguredProject(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(`{"name":"RUNTPRO"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	if _, err := newCorrectionTestClient(srv.URL).FetchClassificationTree(context.Background(), "areas"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/ORG/RUNTPRO/_apis/wit/classificationnodes/areas" {
		t.Errorf("unexpected path %q", path)
	}
}

const bugWithChildrenJSON = `{
  "id": 171191,
  "fields": {
    "System.Title": "Error al radicar",
    "System.WorkItemType": "Bug",
    "System.State": "Activo",
    "System.TeamProject": "ControlesDeCambio",
    "System.AreaPath": "ControlesDeCambio\\RNET",
    "System.IterationPath": "ControlesDeCambio\\Sprint 7 Semana 41",
    "System.AssignedTo": {"displayName": "Dev Uno", "uniqueName": "dev@runt.com.co", "id": "dev-id"}
  },
  "relations": [
    {"rel": "System.LinkTypes.Hierarchy-Forward", "url": "https://dev.azure.com/ORG/_apis/wit/workItems/171306"},
    {"rel": "System.LinkTypes.Hierarchy-Forward", "url": "https://dev.azure.com/ORG/_apis/wit/workItems/171307"},
    {"rel": "System.LinkTypes.Hierarchy-Forward", "url": "https://dev.azure.com/ORG/_apis/wit/workItems/171308"},
    {"rel": "AttachedFile", "url": "https://dev.azure.com/ORG/_apis/wit/attachments/abc"}
  ]
}`

const bugChildrenDetailsJSON = `{"value":[
  {"id":171306,"fields":{"System.Title":"Atención y/o Corrección del defecto 171306","System.WorkItemType":"Task","System.State":"Proposed"}},
  {"id":171307,"fields":{"System.Title":"Verificación y validación del defecto 171191","System.WorkItemType":"Task","System.State":"Proposed"}},
  {"id":171308,"fields":{"System.Title":"Atención y/o Corrección del defecto 171191","System.WorkItemType":"Bug","System.State":"Active"}}
]}`

func TestFetchBugCorrectionTaskDraft_BugWithExistingCorrectionTask(t *testing.T) {
	var detailIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/ORG/_apis/wit/workitems/171191":
			if !strings.Contains(r.URL.RawQuery, "$expand=relations") {
				t.Errorf("expected $expand=relations, got %q", r.URL.RawQuery)
			}
			w.Write([]byte(bugWithChildrenJSON)) //nolint:errcheck
		case r.URL.Path == "/ORG/_apis/wit/workitems":
			detailIDs = r.URL.Query().Get("ids")
			w.Write([]byte(bugChildrenDetailsJSON)) //nolint:errcheck
		default:
			t.Fatalf("unexpected request %s", r.URL.String())
		}
	}))
	defer srv.Close()

	d, err := newCorrectionTestClient(srv.URL).FetchBugCorrectionTaskDraft(context.Background(), 171191)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d == nil {
		t.Fatal("expected draft")
	}
	if detailIDs != "171306,171307,171308" {
		t.Errorf("expected children details read, got ids=%q", detailIDs)
	}
	if d.BugID != 171191 || d.BugTitle != "Error al radicar" || d.Type != "Bug" {
		t.Errorf("unexpected bug fields %+v", d)
	}
	if d.TeamProject != "ControlesDeCambio" || d.AreaPath != `ControlesDeCambio\RNET` || d.IterationPath != `ControlesDeCambio\Sprint 7 Semana 41` {
		t.Errorf("unexpected classification fields %+v", d)
	}
	if d.AssignedTo.DisplayName != "Dev Uno" || d.AssignedTo.UniqueName != "dev@runt.com.co" || d.AssignedTo.ID != "dev-id" {
		t.Errorf("unexpected assignee %+v", d.AssignedTo)
	}
	if d.SuggestedTitle != "Atención y/o Corrección del defecto 171191" {
		t.Errorf("unexpected suggested title %q", d.SuggestedTitle)
	}
	// Only Task children whose title starts with the correction prefix count.
	if len(d.ExistingCorrectionTasks) != 1 || d.ExistingCorrectionTasks[0].ID != 171306 {
		t.Fatalf("expected only 171306 as existing correction task, got %+v", d.ExistingCorrectionTasks)
	}
	if d.ExistingCorrectionTasks[0].State != "Proposed" {
		t.Errorf("unexpected state %+v", d.ExistingCorrectionTasks[0])
	}
}

func TestFetchBugCorrectionTaskDraft_NotABugSkipsChildrenRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ORG/_apis/wit/workitems/55" {
			t.Fatalf("unexpected request %s", r.URL.String())
		}
		w.Write([]byte(`{"id":55,"fields":{"System.Title":"T","System.WorkItemType":"Task","System.TeamProject":"RUNTPRO"},"relations":[{"rel":"System.LinkTypes.Hierarchy-Forward","url":"https://x/_apis/wit/workItems/56"}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	d, err := newCorrectionTestClient(srv.URL).FetchBugCorrectionTaskDraft(context.Background(), 55)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d == nil || d.IsBug() {
		t.Fatalf("expected non-bug draft, got %+v", d)
	}
	if len(d.ExistingCorrectionTasks) != 0 {
		t.Errorf("expected no children for non-bug, got %+v", d.ExistingCorrectionTasks)
	}
}

func TestFetchBugCorrectionTaskDraft_NotFoundReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d, err := newCorrectionTestClient(srv.URL).FetchBugCorrectionTaskDraft(context.Background(), 9)
	if err != nil || d != nil {
		t.Fatalf("expected (nil, nil), got (%+v, %v)", d, err)
	}
}

func TestIsCorrectionTaskTitle(t *testing.T) {
	cases := map[string]bool{
		"Atención y/o Corrección del defecto 1": true,
		"  atención y/o corrección del defecto": true,
		"Verificación y validación":             false,
		"":                                      false,
	}
	for title, want := range cases {
		if got := IsCorrectionTaskTitle(title); got != want {
			t.Errorf("IsCorrectionTaskTitle(%q) = %v, want %v", title, got, want)
		}
	}
}
