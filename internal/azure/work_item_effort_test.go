package azure

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestFetchWorkItemEstimates_ParsesEstimatesAndMissingField(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/_apis/wit/workitems") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		query = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"count":2,"value":[
			{"id":101,"fields":{"System.Id":101,"Microsoft.VSTS.Scheduling.OriginalEstimate":24}},
			{"id":202,"fields":{"System.Id":202}}
		]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	got, err := c.FetchWorkItemEstimates(context.Background(), []int{101, 202})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "ids=101,202") || !strings.Contains(query, "Microsoft.VSTS.Scheduling.OriginalEstimate") {
		t.Errorf("unexpected query: %s", query)
	}
	if got[101] != 24 {
		t.Errorf("expected estimate 24 for 101, got %v", got[101])
	}
	if v, ok := got[202]; !ok || v != 0 {
		t.Errorf("expected 202 present with estimate 0, got %v (present=%v)", v, ok)
	}
}

func TestFetchWorkItemEstimates_BatchesAt200(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		ids := strings.Split(r.URL.Query().Get("ids"), ",")
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = fmt.Sprintf(`{"id":%s,"fields":{"Microsoft.VSTS.Scheduling.OriginalEstimate":1}}`, id)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"value":[` + strings.Join(parts, ",") + `]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	ids := make([]int, 250)
	for i := range ids {
		ids[i] = i + 1
	}
	got, err := c.FetchWorkItemEstimates(context.Background(), ids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 batched calls, got %d", calls)
	}
	if len(got) != 250 {
		t.Errorf("expected 250 estimates, got %d", len(got))
	}
}

func TestFetchWorkItemEstimates_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	if _, err := c.FetchWorkItemEstimates(context.Background(), []int{1}); err == nil {
		t.Fatal("expected error on 401")
	}
}

func TestFetchWorkItemEstimates_EmptyIDs(t *testing.T) {
	c := &Client{cfg: Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"}, http: http.DefaultClient}
	got, err := c.FetchWorkItemEstimates(context.Background(), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("expected empty map and no error, got %v, %v", got, err)
	}
}
