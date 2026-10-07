package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestEvaluateTimeLogTarget(t *testing.T) {
	const me = "me-id"
	bug := WorkItemHierarchy{ID: 171191, Title: "Login falla", Type: "Bug", ChildIDs: []int{171323, 171306}}
	bugAsParent := &AssignedWorkItem{ID: 171191, Title: "Login falla", Type: "Bug"}

	tests := []struct {
		name     string
		item     WorkItemHierarchy
		parent   *AssignedWorkItem
		children []AssignedWorkItem
		callerID string
		wantErr  string // empty means allowed
	}{
		{
			name:   "bug lists the single child task assigned to me",
			item:   bug,
			parent: nil,
			children: []AssignedWorkItem{
				{ID: 171323, Title: "Verificación y validación del defecto 171323", Type: "Task", AssignedToID: "other-id"},
				{ID: 171306, Title: "Atención y/o Corrección del defecto 171306", Type: "Task", AssignedToID: me},
			},
			callerID: me,
			wantErr:  "work item 171191 is a Bug; log hours on its child task: #171306 Atención y/o Corrección del defecto 171306",
		},
		{
			name: "bug lists every child task assigned to me sorted by id",
			item: bug,
			children: []AssignedWorkItem{
				{ID: 171323, Title: "Verificación", Type: "Task", AssignedToID: me},
				{ID: 171306, Title: "Atención", Type: "Task", AssignedToID: me},
			},
			callerID: me,
			wantErr:  "work item 171191 is a Bug; log hours on one of its child tasks: #171306 Atención, #171323 Verificación",
		},
		{
			name: "bug with child tasks none assigned to me",
			item: bug,
			children: []AssignedWorkItem{
				{ID: 171323, Title: "Verificación", Type: "Task", AssignedToID: "other-id"},
			},
			callerID: me,
			wantErr:  "work item 171191 is a Bug; log hours on one of its child tasks, but none of them is assigned to you",
		},
		{
			name:     "bug without child tasks",
			item:     WorkItemHierarchy{ID: 5, Type: "Bug"},
			callerID: me,
			wantErr:  "work item 5 is a Bug and has no child tasks; log hours on a child task assigned to you",
		},
		{
			name: "bug ignores non-task children",
			item: bug,
			children: []AssignedWorkItem{
				{ID: 171306, Title: "Sub bug", Type: "Bug", AssignedToID: me},
			},
			callerID: me,
			wantErr:  "work item 171191 is a Bug and has no child tasks; log hours on a child task assigned to you",
		},
		{
			name: "bug with unresolved caller identity lists every child task",
			item: bug,
			children: []AssignedWorkItem{
				{ID: 171306, Title: "Atención", Type: "Task", AssignedToID: "other-id"},
			},
			callerID: "",
			wantErr:  "work item 171191 is a Bug; log hours on its child task: #171306 Atención",
		},
		{
			name:     "bug type match is case-insensitive",
			item:     WorkItemHierarchy{ID: 5, Type: "bug"},
			callerID: me,
			wantErr:  "work item 5 is a Bug and has no child tasks; log hours on a child task assigned to you",
		},
		{
			name:     "bug child task assigned to someone else",
			item:     WorkItemHierarchy{ID: 171323, Type: "Task", ParentID: 171191, AssignedToID: "other-id", AssignedToDisplayName: "Jane Doe"},
			parent:   bugAsParent,
			callerID: me,
			wantErr:  "work item 171323 is a task of Bug 171191 assigned to Jane Doe, not you; log hours only on bug tasks assigned to you",
		},
		{
			name:     "unassigned bug child task",
			item:     WorkItemHierarchy{ID: 171323, Type: "Task", ParentID: 171191},
			parent:   bugAsParent,
			callerID: me,
			wantErr:  "work item 171323 is a task of Bug 171191 assigned to no one (unassigned), not you; log hours only on bug tasks assigned to you",
		},
		{
			name:     "bug child task assigned to me",
			item:     WorkItemHierarchy{ID: 171306, Type: "Task", ParentID: 171191, AssignedToID: me},
			parent:   bugAsParent,
			callerID: me,
		},
		{
			name:     "bug child task with unresolved caller identity is let through",
			item:     WorkItemHierarchy{ID: 171306, Type: "Task", ParentID: 171191, AssignedToID: "other-id"},
			parent:   bugAsParent,
			callerID: "",
		},
		{
			name:     "task under a user story is allowed regardless of assignee",
			item:     WorkItemHierarchy{ID: 9, Type: "Task", ParentID: 8, AssignedToID: "other-id"},
			parent:   &AssignedWorkItem{ID: 8, Type: "User Story"},
			callerID: me,
		},
		{
			name:     "standalone task is allowed",
			item:     WorkItemHierarchy{ID: 156263, Type: "Task"},
			callerID: me,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EvaluateTimeLogTarget(tt.item, tt.parent, tt.children, tt.callerID)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected allowed, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected rejection %q, got nil", tt.wantErr)
			}
			var rejection *TimeLogTargetError
			if !errors.As(err, &rejection) {
				t.Fatalf("expected *TimeLogTargetError, got %T", err)
			}
			if rejection.WorkItemID != tt.item.ID {
				t.Errorf("expected rejection for work item %d, got %d", tt.item.ID, rejection.WorkItemID)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("unexpected message:\n got: %s\nwant: %s", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestFetchWorkItemHierarchy_ParsesFieldsAndRelations(t *testing.T) {
	var path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		query = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":171191,"fields":{
			"System.Title":"Login falla",
			"System.WorkItemType":"Bug",
			"System.State":"Active",
			"System.AssignedTo":{"displayName":"Jane Doe","id":"identity-123"}
		},"relations":[
			{"rel":"System.LinkTypes.Hierarchy-Forward","url":"https://dev.azure.com/ORG/_apis/wit/workItems/171306"},
			{"rel":"System.LinkTypes.Related","url":"https://dev.azure.com/ORG/_apis/wit/workItems/1"},
			{"rel":"System.LinkTypes.Hierarchy-Forward","url":"https://dev.azure.com/ORG/_apis/wit/workItems/171323"},
			{"rel":"System.LinkTypes.Hierarchy-Reverse","url":"https://dev.azure.com/ORG/_apis/wit/workItems/170000"},
			{"rel":"AttachedFile","url":"https://dev.azure.com/ORG/_apis/wit/attachments/abc"}
		]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	h, err := c.FetchWorkItemHierarchy(context.Background(), 171191)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/ORG/_apis/wit/workitems/171191" {
		t.Errorf("unexpected path %q", path)
	}
	if !strings.Contains(query, "$expand=relations") {
		t.Errorf("expected $expand=relations in query, got %s", query)
	}
	if strings.Contains(query, "fields=") {
		t.Errorf("Azure rejects fields together with $expand, got %s", query)
	}
	if h.ID != 171191 || h.Title != "Login falla" || h.Type != "Bug" || h.State != "Active" {
		t.Errorf("unexpected fields: %+v", h)
	}
	if h.AssignedToID != "identity-123" || h.AssignedToDisplayName != "Jane Doe" {
		t.Errorf("unexpected assignee: %+v", h)
	}
	if h.ParentID != 170000 {
		t.Errorf("expected parent 170000, got %d", h.ParentID)
	}
	if fmt.Sprint(h.ChildIDs) != "[171306 171323]" {
		t.Errorf("expected children [171306 171323], got %v", h.ChildIDs)
	}
}

func TestFetchWorkItemHierarchy_NotFoundReturnsNilNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	h, err := c.FetchWorkItemHierarchy(context.Background(), 1)
	if err != nil || h != nil {
		t.Fatalf("expected (nil, nil), got (%+v, %v)", h, err)
	}
}

func TestFetchWorkItemHierarchy_NotEnabled_NoHTTPCall(t *testing.T) {
	callMade := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callMade = true
	}))
	defer srv.Close()

	c := &Client{cfg: Config{Org: "ORG"}, http: &http.Client{Transport: redirectToServer(srv.URL)}}
	if _, err := c.FetchWorkItemHierarchy(context.Background(), 1); err == nil {
		t.Fatal("expected error when token is not configured")
	}
	if callMade {
		t.Error("no HTTP call should be made without a token")
	}
}

// fakeWorkItemServer serves single-item ($expand=relations) and batch
// (ids=...) work item reads from an in-memory map, recording every request
// path+query so tests can assert which calls were made.
func fakeWorkItemServer(t *testing.T, single map[int]string, batch map[int]string) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()
		if ids := r.URL.Query().Get("ids"); ids != "" {
			var parts []string
			for _, s := range strings.Split(ids, ",") {
				var id int
				fmt.Sscanf(s, "%d", &id) //nolint:errcheck
				if body, ok := batch[id]; ok {
					parts = append(parts, body)
				}
			}
			w.Write([]byte(`{"value":[` + strings.Join(parts, ",") + `]}`)) //nolint:errcheck
			return
		}
		var id int
		fmt.Sscanf(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], "%d", &id) //nolint:errcheck
		body, ok := single[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(body)) //nolint:errcheck
	}))
	return srv, &calls
}

func relURL(id int) string {
	return fmt.Sprintf("https://dev.azure.com/ORG/_apis/wit/workItems/%d", id)
}

func TestCheckTimeLogTarget(t *testing.T) {
	single := map[int]string{
		171191: `{"id":171191,"fields":{"System.Title":"Login falla","System.WorkItemType":"Bug"},"relations":[
			{"rel":"System.LinkTypes.Hierarchy-Forward","url":"` + relURL(171306) + `"},
			{"rel":"System.LinkTypes.Hierarchy-Forward","url":"` + relURL(171323) + `"}]}`,
		171306: `{"id":171306,"fields":{"System.Title":"Atención","System.WorkItemType":"Task","System.AssignedTo":{"displayName":"Me","id":"me-id"}},"relations":[
			{"rel":"System.LinkTypes.Hierarchy-Reverse","url":"` + relURL(171191) + `"}]}`,
		171323: `{"id":171323,"fields":{"System.Title":"Verificación","System.WorkItemType":"Task","System.AssignedTo":{"displayName":"Jane Doe","id":"jane-id"}},"relations":[
			{"rel":"System.LinkTypes.Hierarchy-Reverse","url":"` + relURL(171191) + `"}]}`,
		156263: `{"id":156263,"fields":{"System.Title":"Transversal","System.WorkItemType":"Task"}}`,
	}
	batch := map[int]string{
		171191: `{"id":171191,"fields":{"System.Title":"Login falla","System.WorkItemType":"Bug"}}`,
		171306: `{"id":171306,"fields":{"System.Title":"Atención","System.WorkItemType":"Task","System.AssignedTo":{"displayName":"Me","id":"me-id"}}}`,
		171323: `{"id":171323,"fields":{"System.Title":"Verificación","System.WorkItemType":"Task","System.AssignedTo":{"displayName":"Jane Doe","id":"jane-id"}}}`,
	}

	tests := []struct {
		name      string
		id        int
		wantErr   string
		wantCalls int
	}{
		{name: "bug rejected with my child task", id: 171191, wantErr: "work item 171191 is a Bug; log hours on its child task: #171306 Atención", wantCalls: 2},
		{name: "bug child assigned to me allowed", id: 171306, wantCalls: 2},
		{name: "bug child assigned to someone else rejected", id: 171323, wantErr: "work item 171323 is a task of Bug 171191 assigned to Jane Doe, not you; log hours only on bug tasks assigned to you", wantCalls: 2},
		{name: "standalone task allowed with a single call", id: 156263, wantCalls: 1},
		{name: "missing work item rejected", id: 42, wantErr: "work item 42 was not found in Azure DevOps", wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, calls := fakeWorkItemServer(t, single, batch)
			defer srv.Close()
			c := &Client{
				cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", UserID: "me-id"},
				http: &http.Client{Transport: redirectToServer(srv.URL)},
			}
			err := c.CheckTimeLogTarget(context.Background(), tt.id)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("expected allowed, got %v", err)
			}
			if tt.wantErr != "" {
				var rejection *TimeLogTargetError
				if !errors.As(err, &rejection) {
					t.Fatalf("expected *TimeLogTargetError, got %v", err)
				}
				if err.Error() != tt.wantErr {
					t.Errorf("unexpected message:\n got: %s\nwant: %s", err.Error(), tt.wantErr)
				}
			}
			if len(*calls) != tt.wantCalls {
				t.Errorf("expected %d HTTP calls, got %d: %v", tt.wantCalls, len(*calls), *calls)
			}
		})
	}
}

func TestCheckTimeLogTarget_TransportErrorIsNotARejection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", UserID: "me-id"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	err := c.CheckTimeLogTarget(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error on 500")
	}
	var rejection *TimeLogTargetError
	if errors.As(err, &rejection) {
		t.Errorf("a transport failure must not be reported as a rule rejection: %v", err)
	}
}
