package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTimeLogHours(t *testing.T) {
	docs := []TimeLogDocument{
		{WorkItemID: 7, Minutes: 90},
		{WorkItemID: 8, Minutes: 600},
		{WorkItemID: 7, Minutes: 20},
	}
	tests := []struct {
		name string
		id   int
		want float64
	}{
		{name: "sums and rounds to 2 decimals", id: 7, want: 1.83},
		{name: "single document", id: 8, want: 10},
		{name: "no documents", id: 9, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TimeLogHours(docs, tt.id); got != tt.want {
				t.Errorf("TimeLogHours(%d) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestSetEffortFromTimeLogTotal(t *testing.T) {
	const completed = "/fields/Microsoft.VSTS.Scheduling.CompletedWork"
	const remaining = "/fields/Microsoft.VSTS.Scheduling.RemainingWork"
	tests := []struct {
		name     string
		estimate float64
		want     map[string]float64 // patched path -> value
	}{
		{name: "estimate above total sets remaining", estimate: 4, want: map[string]float64{completed: 2.5, remaining: 1.5}},
		{name: "estimate below total floors remaining at zero", estimate: 2, want: map[string]float64{completed: 2.5, remaining: 0}},
		{name: "no estimate leaves remaining untouched", estimate: 0, want: map[string]float64{completed: 2.5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var method, path string
			var ops []struct {
				Path  string  `json:"path"`
				Value float64 `json:"value"`
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method, path = r.Method, r.URL.Path
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &ops)
				w.Write([]byte(`{"id":171306}`)) //nolint:errcheck
			}))
			defer srv.Close()
			c := &Client{
				cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ"},
				http: &http.Client{Transport: redirectToServer(srv.URL)},
			}
			if err := c.SetEffortFromTimeLogTotal(context.Background(), 171306, 2.5, tt.estimate); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if method != http.MethodPatch || path != "/ORG/PROJ/_apis/wit/workitems/171306" {
				t.Errorf("unexpected request %s %s", method, path)
			}
			got := map[string]float64{}
			for _, op := range ops {
				got[op.Path] = op.Value
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("patched %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchOriginalEstimates(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`{"value":[
			{"id":1,"fields":{"Microsoft.VSTS.Scheduling.OriginalEstimate":8}},
			{"id":2,"fields":{}}
		]}`)) //nolint:errcheck
	}))
	defer srv.Close()
	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}
	got, err := c.FetchOriginalEstimates(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "ids=1,2") || !strings.Contains(query, "Microsoft.VSTS.Scheduling.OriginalEstimate") {
		t.Errorf("unexpected query %s", query)
	}
	if fmt.Sprint(got) != "map[1:8 2:0]" {
		t.Errorf("unexpected estimates %v", got)
	}
}
