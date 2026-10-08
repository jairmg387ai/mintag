package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

type effortResponse struct {
	Org   string `json:"org"`
	Items []struct {
		ID               int     `json:"id"`
		OriginalEstimate float64 `json:"original_estimate"`
		UploadedHours    float64 `json:"uploaded_hours"`
		LocalHours       float64 `json:"local_hours"`
		Remaining        float64 `json:"remaining"`
		HasEstimate      bool    `json:"has_estimate"`
	} `json:"items"`
	TimelogError string `json:"timelog_error"`
}

func newEffortAzureStub(t *testing.T, timelogStatus int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"authenticatedUser":{"id":"route-user-id","providerDisplayName":"Route User"}}`))
		case strings.Contains(r.URL.Path, "TimeLogData/Documents"):
			w.WriteHeader(timelogStatus)
			if timelogStatus == http.StatusOK {
				// 1001: 600+480 minutes = 18h; 1002: 60 minutes = 1h.
				_, _ = w.Write([]byte(`[{"workItemId":1001,"minutes":600},{"workItemId":1001,"minutes":480},{"workItemId":1002,"minutes":60}]`))
			}
		case strings.Contains(r.URL.Path, "/_apis/wit/workitems"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"value":[
				{"id":1001,"fields":{"Microsoft.VSTS.Scheduling.OriginalEstimate":24}},
				{"id":1002,"fields":{}}
			]}`))
		default:
			t.Errorf("unexpected azure path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func seedLocalEffort(t *testing.T, st *store.Store) {
	t.Helper()
	ctx := context.Background()
	wi, err := st.AddAzureActivity(ctx, "RUNT2QA", 1001, "Task A", "Task", store.AzureActivityMapping{})
	mustNoErr(t, err)
	a, err := st.CreateActivity(ctx, "2026-06-12", 2.5, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	mustNoErr(t, err)
	mustNoErr(t, st.SetActivityAzureActivity(ctx, a.ID, &wi.ID))
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

func configureAzureToken(t *testing.T, base string) {
	t.Helper()
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)
}

func TestGetAzureWorkItemEffort_CombinesEstimateTimeLogAndLocal(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	azureServer := newEffortAzureStub(t, http.StatusOK)
	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)

	unconfigured, err := http.Get(base + "/api/activities/azure-work-items/effort?ids=1001")
	mustNoErr(t, err)
	assertStatus(t, unconfigured, http.StatusServiceUnavailable)

	configureAzureToken(t, base)
	seedLocalEffort(t, st)

	resp, body := getEffort(t, base+"/api/activities/azure-work-items/effort?ids=1001,1002")
	assertStatus(t, resp, http.StatusOK)
	if body.TimelogError != "" {
		t.Errorf("unexpected timelog_error: %q", body.TimelogError)
	}
	if body.Org == "" {
		t.Errorf("expected org in response")
	}
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 items, got %+v", body.Items)
	}
	a, b := body.Items[0], body.Items[1]
	if a.ID != 1001 || a.OriginalEstimate != 24 || a.UploadedHours != 18 || a.LocalHours != 2.5 || a.Remaining != 3.5 || !a.HasEstimate {
		t.Errorf("unexpected item 1001: %+v", a)
	}
	if b.ID != 1002 || b.OriginalEstimate != 0 || b.UploadedHours != 1 || b.LocalHours != 0 || b.Remaining != 0 || b.HasEstimate {
		t.Errorf("unexpected item 1002: %+v", b)
	}
}

func TestGetAzureWorkItemEffort_TimeLogFailureIsBestEffort(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	azureServer := newEffortAzureStub(t, http.StatusInternalServerError)
	base, st := newTestServerWithAzureRedirect(t, azureServer.URL)
	configureAzureToken(t, base)
	seedLocalEffort(t, st)

	resp, body := getEffort(t, base+"/api/activities/azure-work-items/effort?ids=1001")
	assertStatus(t, resp, http.StatusOK)
	if body.TimelogError == "" {
		t.Errorf("expected timelog_error to be set")
	}
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %+v", body.Items)
	}
	it := body.Items[0]
	if it.UploadedHours != 0 || it.LocalHours != 2.5 || it.Remaining != 21.5 || !it.HasEstimate {
		t.Errorf("unexpected item: %+v", it)
	}
}

func TestGetAzureWorkItemEffort_InvalidIDs(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	azureServer := newEffortAzureStub(t, http.StatusOK)
	base, _ := newTestServerWithAzureRedirect(t, azureServer.URL)
	configureAzureToken(t, base)

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
