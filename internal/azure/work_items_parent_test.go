package azure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newParentTestClient(serverURL string) *Client {
	return &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(serverURL)},
	}
}

func TestAttachWorkItemParents_ResolvesParentFromRelations(t *testing.T) {
	batch := map[int]string{
		171306: `{"id":171306,"fields":{"System.Title":"Atención y/o Corrección del defecto 171191","System.WorkItemType":"Task"},"relations":[
			{"rel":"System.LinkTypes.Related","url":"` + relURL(999) + `"},
			{"rel":"System.LinkTypes.Hierarchy-Reverse","url":"` + relURL(171191) + `"}]}`,
		156263: `{"id":156263,"fields":{"System.Title":"Transversal","System.WorkItemType":"Task"}}`,
		171191: `{"id":171191,"fields":{"System.Title":"Login falla","System.WorkItemType":"Bug"}}`,
	}
	srv, calls := fakeWorkItemServer(t, nil, batch)
	defer srv.Close()

	items := []AssignedWorkItem{{ID: 171306, Type: "Task"}, {ID: 156263, Type: "Task", ParentID: 5, ParentTitle: "stale", ParentType: "Bug"}}
	if err := newParentTestClient(srv.URL).AttachWorkItemParents(context.Background(), items); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if items[0].ParentID != 171191 || items[0].ParentTitle != "Login falla" || items[0].ParentType != "Bug" {
		t.Errorf("expected parent Bug 171191 'Login falla', got %+v", items[0])
	}
	if items[1].ParentID != 0 || items[1].ParentTitle != "" || items[1].ParentType != "" {
		t.Errorf("expected parent cleared for an item without parent, got %+v", items[1])
	}
	if len(*calls) != 2 {
		t.Fatalf("expected 2 batch calls (relations + parents), got %d: %v", len(*calls), *calls)
	}
	first := (*calls)[0]
	if !strings.Contains(first, "$expand=relations") || strings.Contains(first, "fields=") {
		t.Errorf("relations call must use $expand=relations without fields, got %s", first)
	}
	if !strings.Contains(first, "ids=171306,156263") {
		t.Errorf("relations call must batch every item id, got %s", first)
	}
	if !strings.Contains((*calls)[1], "ids=171191") {
		t.Errorf("parent call must read the distinct parent ids, got %s", (*calls)[1])
	}
}

func TestAttachWorkItemParents_NoParentsSkipsSecondCall(t *testing.T) {
	batch := map[int]string{
		156263: `{"id":156263,"fields":{"System.Title":"Transversal","System.WorkItemType":"Task"}}`,
	}
	srv, calls := fakeWorkItemServer(t, nil, batch)
	defer srv.Close()

	items := []AssignedWorkItem{{ID: 156263}}
	if err := newParentTestClient(srv.URL).AttachWorkItemParents(context.Background(), items); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Errorf("expected a single relations call, got %d: %v", len(*calls), *calls)
	}
}

func TestAttachWorkItemParents_EmptyItemsMakesNoCall(t *testing.T) {
	srv, calls := fakeWorkItemServer(t, nil, nil)
	defer srv.Close()
	if err := newParentTestClient(srv.URL).AttachWorkItemParents(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("expected no HTTP call, got %v", *calls)
	}
}

func TestAttachWorkItemParents_RelationsFailureReturnsErrorAndLeavesItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	items := []AssignedWorkItem{{ID: 1, ParentID: 7, ParentTitle: "kept"}}
	if err := newParentTestClient(srv.URL).AttachWorkItemParents(context.Background(), items); err == nil {
		t.Fatal("expected an error when the relations read fails")
	}
	if items[0].ParentID != 7 || items[0].ParentTitle != "kept" {
		t.Errorf("items must be left untouched on failure, got %+v", items[0])
	}
}
