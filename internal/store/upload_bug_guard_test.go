package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/azure"
)

// guardAzureServer fakes every Azure endpoint an upload touches: single work
// item reads ($expand=relations), batch work item reads (ids=...), TimeLog
// POSTs, the TimeLog documents GET, and the CompletedWork PATCH. It records
// how many work item reads were made, which work item ids were posted to
// TimeLog, how often the documents were listed, and every CompletedWork
// value patched. The documents GET returns priorDocs plus one 60-minute-per-
// hour document for every successful POST.
type guardAzureServer struct {
	*httptest.Server
	mu            sync.Mutex
	reads         int
	postedItems   []int
	postedMinutes []int
	priorDocs     []azure.TimeLogDocument
	docListings   int
	failDocs      bool
	failPatch     bool
	completedWork map[int]float64
	patchedFields map[int][]string
}

func newGuardAzureServer(t *testing.T) *guardAzureServer {
	t.Helper()
	rel := func(kind string, id int) string {
		return fmt.Sprintf(`{"rel":"System.LinkTypes.Hierarchy-%s","url":"https://dev.azure.com/ORG/_apis/wit/workItems/%d"}`, kind, id)
	}
	fields := map[int]string{
		171191: `"System.Title":"Login falla","System.WorkItemType":"Bug"`,
		171306: `"System.Title":"Atención y/o Corrección del defecto 171306","System.WorkItemType":"Task","System.AssignedTo":{"displayName":"Me","id":"me-id"}`,
		171323: `"System.Title":"Verificación y validación del defecto 171323","System.WorkItemType":"Task","System.AssignedTo":{"displayName":"Jane Doe","id":"jane-id"}`,
		156263: `"System.Title":"Transversal","System.WorkItemType":"Task"`,
	}
	relations := map[int]string{
		171191: rel("Forward", 171306) + "," + rel("Forward", 171323),
		171306: rel("Reverse", 171191),
		171323: rel("Reverse", 171191),
	}

	g := &guardAzureServer{completedWork: map[int]float64{}, patchedFields: map[int][]string{}}
	g.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		switch {
		case r.Method == http.MethodPost:
			var payload map[string]any
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &payload)
			id, _ := payload["workItemId"].(float64)
			minutes, _ := payload["minutes"].(float64)
			g.postedItems = append(g.postedItems, int(id))
			g.postedMinutes = append(g.postedMinutes, int(minutes))
			w.Write([]byte(`{"id":"doc"}`)) //nolint:errcheck
			return
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/Documents"):
			g.docListings++
			if g.failDocs {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			docs := append([]azure.TimeLogDocument{}, g.priorDocs...)
			for i, id := range g.postedItems {
				docs = append(docs, azure.TimeLogDocument{WorkItemID: id, Minutes: g.postedMinutes[i]})
			}
			json.NewEncoder(w).Encode(docs) //nolint:errcheck
			return
		case r.Method == http.MethodPatch:
			if g.failPatch {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			var id int
			fmt.Sscanf(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], "%d", &id) //nolint:errcheck
			var ops []struct {
				Path  string  `json:"path"`
				Value float64 `json:"value"`
			}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &ops)
			for _, op := range ops {
				g.patchedFields[id] = append(g.patchedFields[id], op.Path)
				g.completedWork[id] = op.Value
			}
			w.Write([]byte(`{}`)) //nolint:errcheck
			return
		}
		g.reads++
		if ids := r.URL.Query().Get("ids"); ids != "" {
			var parts []string
			for _, s := range strings.Split(ids, ",") {
				var id int
				fmt.Sscanf(s, "%d", &id) //nolint:errcheck
				parts = append(parts, fmt.Sprintf(`{"id":%d,"fields":{%s}}`, id, fields[id]))
			}
			w.Write([]byte(`{"value":[` + strings.Join(parts, ",") + `]}`)) //nolint:errcheck
			return
		}
		var id int
		fmt.Sscanf(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], "%d", &id) //nolint:errcheck
		f, ok := fields[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprintf(w, `{"id":%d,"fields":{%s},"relations":[%s]}`, id, f, relations[id])
	}))
	return g
}

func (g *guardAzureServer) client() *azure.Client {
	c := azure.NewClient(azure.Config{
		Token: "test-pat", AuthMode: azure.AuthModeBasic, Org: "ORG", WorkItemID: 1, User: "Me", UserID: "me-id", EntryType: "T",
	})
	c.SetHTTPClient(&http.Client{Transport: uploadRedirectToServer(g.URL)})
	return c
}

// approvedActivityOn creates one approved activity pointing at workItemID.
// The seeded default catalog entry is work item 156263, so that id reuses it
// (azure_activity_id stays nil); any other id gets its own catalog row.
func approvedActivityOn(t *testing.T, s *Store, workItemID int) int64 {
	t.Helper()
	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if workItemID != 156263 {
		aa, err := s.AddAzureActivity(ctx, "ORG", workItemID, fmt.Sprintf("WI %d", workItemID), "", AzureActivityMapping{})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetActivityAzureActivity(ctx, a.ID, &aa.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ApproveActivities(ctx, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}
	return a.ID
}

func TestUploadActivities_BugGuard(t *testing.T) {
	tests := []struct {
		name       string
		workItemID int
		guardOff   bool
		wantErr    string // empty means uploaded
		wantReads  int
	}{
		{
			name:       "bug rejected listing my child task",
			workItemID: 171191,
			wantErr:    "work item 171191 is a Bug; log hours on its child task: #171306 Atención y/o Corrección del defecto 171306",
			wantReads:  2,
		},
		{
			name:       "bug child task assigned to someone else rejected",
			workItemID: 171323,
			wantErr:    "work item 171323 is a task of Bug 171191 assigned to Jane Doe, not you; log hours only on bug tasks assigned to you",
			wantReads:  2,
		},
		{name: "bug child task assigned to me uploaded", workItemID: 171306, wantReads: 2},
		{name: "standalone task uploaded", workItemID: 156263, wantReads: 1},
		{name: "guard off uploads a bug without reading Azure", workItemID: 171191, guardOff: true, wantReads: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := OpenInMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			ctx := context.Background()
			if tt.guardOff {
				if err := s.SetTimeLogBugGuardEnabled(ctx, false); err != nil {
					t.Fatal(err)
				}
			}
			id := approvedActivityOn(t, s, tt.workItemID)

			srv := newGuardAzureServer(t)
			defer srv.Close()
			result, err := s.UploadActivities(ctx, "2026-06-12", srv.client())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, err := s.GetActivity(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantErr == "" {
				if result.UploadedCount != 1 || got.Status != "uploaded" {
					t.Fatalf("expected upload, got count=%d status=%q errors=%v", result.UploadedCount, got.Status, result.Errors)
				}
				if len(srv.postedItems) != 1 || srv.postedItems[0] != tt.workItemID {
					t.Errorf("expected one TimeLog post to %d, got %v", tt.workItemID, srv.postedItems)
				}
			} else {
				if len(result.FailedIDs) != 1 || result.FailedIDs[0] != id {
					t.Fatalf("expected activity %d to fail, got %v", id, result.FailedIDs)
				}
				if len(result.Errors) != 1 || result.Errors[0] != tt.wantErr {
					t.Errorf("unexpected errors:\n got: %v\nwant: %s", result.Errors, tt.wantErr)
				}
				if got.Status != "approved" {
					t.Errorf("rejected activity must stay approved for retry, got %q", got.Status)
				}
				if len(srv.postedItems) != 0 {
					t.Errorf("rejected activity must not reach TimeLog, posted %v", srv.postedItems)
				}
			}
			if srv.reads != tt.wantReads {
				t.Errorf("expected %d work item reads, got %d", tt.wantReads, srv.reads)
			}
		})
	}
}

// TestUploadActivities_BugGuardCachesPerWorkItem verifies two rows on the
// same work item cost one hierarchy check, not one per row.
func TestUploadActivities_BugGuardCachesPerWorkItem(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	first := approvedActivityOn(t, s, 171306)
	second, err := s.CreateActivity(ctx, "2026-06-12", 2.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	firstRow, err := s.GetActivity(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetActivityAzureActivity(ctx, second.ID, firstRow.AzureActivityID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveActivities(ctx, []int64{second.ID}); err != nil {
		t.Fatal(err)
	}

	srv := newGuardAzureServer(t)
	defer srv.Close()
	result, err := s.UploadActivities(ctx, "2026-06-12", srv.client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UploadedCount != 2 {
		t.Fatalf("expected 2 uploads, got %d (errors: %v)", result.UploadedCount, result.Errors)
	}
	if srv.reads != 2 {
		t.Errorf("expected the hierarchy check to run once (2 reads), got %d reads", srv.reads)
	}
}

// TestUploadActivities_SyncsCompletedWork verifies that after a batch posts,
// each work item with at least one successful post gets CompletedWork set to
// its TimeLog total (prior documents + this batch), from a single documents
// listing, without touching RemainingWork; a rejected row's work item is not
// synced.
func TestUploadActivities_SyncsCompletedWork(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	approvedActivityOn(t, s, 171306)
	approvedActivityOn(t, s, 156263)
	approvedActivityOn(t, s, 171191) // Bug: rejected by the guard

	srv := newGuardAzureServer(t)
	defer srv.Close()
	srv.priorDocs = []azure.TimeLogDocument{{WorkItemID: 171306, Minutes: 90}}

	result, err := s.UploadActivities(ctx, "2026-06-12", srv.client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UploadedCount != 2 || len(result.FailedIDs) != 1 {
		t.Fatalf("expected 2 uploads and 1 rejection, got %d / %v", result.UploadedCount, result.FailedIDs)
	}
	if len(result.EffortSyncErrors) != 0 {
		t.Errorf("expected no effort sync errors, got %v", result.EffortSyncErrors)
	}
	if srv.docListings != 1 {
		t.Errorf("expected TimeLog documents to be listed once per batch, got %d", srv.docListings)
	}
	want := map[int]float64{171306: 2.5, 156263: 1}
	if fmt.Sprint(srv.completedWork) != fmt.Sprint(want) {
		t.Errorf("expected CompletedWork %v, got %v", want, srv.completedWork)
	}
	for id, paths := range srv.patchedFields {
		if len(paths) != 1 || paths[0] != "/fields/Microsoft.VSTS.Scheduling.CompletedWork" {
			t.Errorf("work item %d: expected only CompletedWork to be patched, got %v", id, paths)
		}
	}
}

// TestUploadActivities_EffortSyncFailureIsNonFatal verifies a failed
// documents listing or PATCH is reported in EffortSyncErrors while the
// upload itself still succeeds and the row is marked uploaded.
func TestUploadActivities_EffortSyncFailureIsNonFatal(t *testing.T) {
	tests := []struct {
		name      string
		failDocs  bool
		failPatch bool
		wantErr   string
	}{
		{name: "documents listing fails", failDocs: true, wantErr: "completed work sync: azure: unexpected fetch time log documents status 500"},
		{name: "patch fails", failPatch: true, wantErr: "completed work sync for work item 171306: azure: unexpected patch work item status 403"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := OpenInMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			ctx := context.Background()
			id := approvedActivityOn(t, s, 171306)

			srv := newGuardAzureServer(t)
			defer srv.Close()
			srv.failDocs, srv.failPatch = tt.failDocs, tt.failPatch

			result, err := s.UploadActivities(ctx, "2026-06-12", srv.client())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.UploadedCount != 1 || len(result.FailedIDs) != 0 || len(result.Errors) != 0 {
				t.Fatalf("expected the upload to succeed, got %+v", result)
			}
			if len(result.EffortSyncErrors) != 1 || result.EffortSyncErrors[0] != tt.wantErr {
				t.Errorf("unexpected effort sync errors:\n got: %v\nwant: [%s]", result.EffortSyncErrors, tt.wantErr)
			}
			got, err := s.GetActivity(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != "uploaded" {
				t.Errorf("a sync failure must not undo the upload, got status %q", got.Status)
			}
		})
	}
}
