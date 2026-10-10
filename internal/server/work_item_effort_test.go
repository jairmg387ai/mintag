package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

type effortResponse struct {
	Items []struct {
		ID               int     `json:"id"`
		OriginalEstimate float64 `json:"original_estimate"`
		UploadedHours    float64 `json:"uploaded_hours"`
		LocalHours       float64 `json:"local_hours"`
		Remaining        float64 `json:"remaining"`
		HasEstimate      bool    `json:"has_estimate"`
	} `json:"items"`
}

// newNoCallAzureStub fails the test on any outbound Azure request and counts
// them, proving the effort endpoint is computed from the local DB only.
func newNoCallAzureStub(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		t.Errorf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// seedLocalEffort registers work item 1001 (estimate 24, plus a second
// catalog row with a lower estimate) with 18h uploaded and 2.5h local, and
// work item 1002 (no estimate) with 1h uploaded.
func seedLocalEffort(t *testing.T, st *store.Store) {
	t.Helper()
	ctx := context.Background()
	wi, err := st.AddAzureActivity(ctx, "RUNT2QA", 1001, "Task A", "Task", store.AzureActivityMapping{})
	mustNoErr(t, err)
	_, err = st.AddAzureActivity(ctx, "RUNT2QA", 1001, "Task A alias", "Task", store.AzureActivityMapping{})
	mustNoErr(t, err)
	mustNoErr(t, st.SetAzureActivityEstimate(ctx, 1001, 24))
	wiB, err := st.AddAzureActivity(ctx, "RUNT2QA", 1002, "Task B", "Task", store.AzureActivityMapping{})
	mustNoErr(t, err)

	link := func(hours float64, azureID int64, upload bool) {
		t.Helper()
		a, err := st.CreateActivity(ctx, "2026-06-12", hours, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
		mustNoErr(t, err)
		mustNoErr(t, st.SetActivityAzureActivity(ctx, a.ID, &azureID))
		if upload {
			_, err := st.ApproveActivities(ctx, []int64{a.ID})
			mustNoErr(t, err)
			mustNoErr(t, st.MarkUploaded(ctx, a.ID, "doc-"+strconv.FormatInt(a.ID, 10)))
		}
	}
	link(10, wi.ID, true)
	link(8, wi.ID, true)
	link(2.5, wi.ID, false)
	link(1, wiB.ID, true)
}

func getEffort(t *testing.T, url string) (*http.Response, effortResponse) {
	t.Helper()
	resp, err := http.Get(url)
	mustNoErr(t, err)
	defer resp.Body.Close()
	var body effortResponse
	if resp.StatusCode == http.StatusOK {
		mustNoErr(t, json.NewDecoder(resp.Body).Decode(&body))
	}
	return resp, body
}

func TestGetAzureWorkItemEffort_ComputedFromLocalDBWithoutAzure(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	azureServer, calls := newNoCallAzureStub(t)
	// Azure is deliberately NOT configured: the endpoint must still answer.
	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)
	seedLocalEffort(t, st)

	resp, body := getEffort(t, base+"/api/activities/azure-work-items/effort?ids=1001,1002,1003")
	assertStatus(t, resp, http.StatusOK)
	if len(body.Items) != 3 {
		t.Fatalf("expected 3 items, got %+v", body.Items)
	}
	a, b, c := body.Items[0], body.Items[1], body.Items[2]
	if a.ID != 1001 || a.OriginalEstimate != 24 || a.UploadedHours != 18 || a.LocalHours != 2.5 || a.Remaining != 3.5 || !a.HasEstimate {
		t.Errorf("unexpected item 1001: %+v", a)
	}
	if b.ID != 1002 || b.OriginalEstimate != 0 || b.UploadedHours != 1 || b.LocalHours != 0 || b.Remaining != 0 || b.HasEstimate {
		t.Errorf("unexpected item 1002: %+v", b)
	}
	if c.ID != 1003 || c.OriginalEstimate != 0 || c.UploadedHours != 0 || c.LocalHours != 0 || c.HasEstimate {
		t.Errorf("unexpected uncatalogued item 1003: %+v", c)
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Errorf("expected no outbound Azure calls, got %d", n)
	}
}

func TestGetAzureWorkItemEffort_RemainingNeverNegative(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	azureServer, _ := newNoCallAzureStub(t)
	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)
	ctx := context.Background()
	wi, err := st.AddAzureActivity(ctx, "RUNT2QA", 4001, "Over", "Task", store.AzureActivityMapping{})
	mustNoErr(t, err)
	mustNoErr(t, st.SetAzureActivityEstimate(ctx, 4001, 2))
	act, err := st.CreateActivity(ctx, "2026-06-12", 3, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	mustNoErr(t, err)
	mustNoErr(t, st.SetActivityAzureActivity(ctx, act.ID, &wi.ID))

	resp, body := getEffort(t, base+"/api/activities/azure-work-items/effort?ids=4001")
	assertStatus(t, resp, http.StatusOK)
	if len(body.Items) != 1 || body.Items[0].Remaining != 0 || body.Items[0].LocalHours != 3 || !body.Items[0].HasEstimate {
		t.Errorf("unexpected over-estimate item: %+v", body.Items)
	}
}

func TestGetAzureWorkItemEffort_InvalidIDs(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	azureServer, _ := newNoCallAzureStub(t)
	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)

	tooMany := make([]string, 201)
	for i := range tooMany {
		tooMany[i] = strconv.Itoa(i + 1)
	}
	cases := []struct {
		name  string
		query string
	}{
		{"missing", ""},
		{"empty", "?ids="},
		{"non-numeric", "?ids=1,abc"},
		{"non-positive", "?ids=0"},
		{"over cap", "?ids=" + strings.Join(tooMany, ",")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(base + "/api/activities/azure-work-items/effort" + tc.query)
			mustNoErr(t, err)
			defer resp.Body.Close()
			assertStatus(t, resp, http.StatusBadRequest)
			if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
				t.Errorf("expected JSON error body, got Content-Type %q", ct)
			}
		})
	}
}
