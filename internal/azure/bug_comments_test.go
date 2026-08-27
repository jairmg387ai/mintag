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

func TestListBugComments_ParsesCommentsListResponse(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"totalCount": 2,
			"comments": [
				{"id": 10, "text": "<p>first comment</p>", "createdBy": {"displayName": "Alice"}, "createdDate": "2026-08-01T10:00:00Z"},
				{"id": 11, "text": "<p>second comment</p>", "createdBy": {"displayName": "Bob"}, "createdDate": "2026-08-02T11:00:00Z"}
			]
		}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "DEFAULT-PROJECT"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	comments, err := c.ListBugComments(context.Background(), 4242, "RUNTPRO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(path, "RUNTPRO") {
		t.Errorf("expected request path to contain the given teamProject RUNTPRO, got %q", path)
	}
	if !strings.Contains(path, "/_apis/wit/workitems/4242/comments") {
		t.Errorf("expected request path to target the work item's comments endpoint, got %q", path)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].ID != 10 || comments[0].Text != "<p>first comment</p>" || comments[0].CreatedBy != "Alice" || comments[0].CreatedDate != "2026-08-01T10:00:00Z" {
		t.Errorf("expected first comment to be fully parsed, got %+v", comments[0])
	}
	if comments[1].ID != 11 || comments[1].CreatedBy != "Bob" {
		t.Errorf("expected second comment to be fully parsed, got %+v", comments[1])
	}
}

func TestListBugComments_EmptyList_ReturnsEmptySlice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"totalCount": 0, "comments": []}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	comments, err := c.ListBugComments(context.Background(), 4242, "RUNTPRO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected an empty slice, got %+v", comments)
	}
}

func TestAddBugComment_PostsToGivenTeamProject_NotClientConfiguredDefault(t *testing.T) {
	var path, method string
	var requestBody struct {
		Text string `json:"text"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &requestBody); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 77, "text": "<p>new comment</p>", "createdBy": {"displayName": "Carol"}, "createdDate": "2026-08-03T12:00:00Z"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	// The client's configured default TeamProject is deliberately different
	// from the project passed to AddBugComment, so the assertion below proves
	// the request actually used the explicit parameter, not c.cfg.TeamProject.
	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "DEFAULT-PROJECT"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	comment, err := c.AddBugComment(context.Background(), 4242, "BUGS-ACTUAL-PROJECT", "<p>new comment</p>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != http.MethodPost {
		t.Errorf("expected POST, got %s", method)
	}
	if !strings.Contains(path, "BUGS-ACTUAL-PROJECT") {
		t.Errorf("expected request path to use the explicit teamProject param BUGS-ACTUAL-PROJECT, got %q", path)
	}
	if strings.Contains(path, "DEFAULT-PROJECT") {
		t.Errorf("expected request path to NOT use the client's configured default TeamProject, got %q", path)
	}
	if requestBody.Text != "<p>new comment</p>" {
		t.Errorf("expected request body text to be sent, got %q", requestBody.Text)
	}
	if comment.ID != 77 || comment.CreatedBy != "Carol" {
		t.Errorf("expected the created comment to be parsed from the response, got %+v", comment)
	}
}

func TestAddBugComment_Forbidden_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"not authorized"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	_, err := c.AddBugComment(context.Background(), 4242, "RUNTPRO", "text")
	if err == nil {
		t.Fatal("expected an error for a 403 response")
	}
}
