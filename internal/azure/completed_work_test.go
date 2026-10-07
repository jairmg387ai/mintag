package azure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestSetCompletedWork_PatchesOnlyCompletedWork(t *testing.T) {
	var method, path string
	var ops []patchOp
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
	if err := c.SetCompletedWork(context.Background(), 171306, 2.5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != http.MethodPatch || path != "/ORG/PROJ/_apis/wit/workitems/171306" {
		t.Errorf("unexpected request %s %s", method, path)
	}
	if len(ops) != 1 || ops[0].Path != "/fields/Microsoft.VSTS.Scheduling.CompletedWork" || ops[0].Value != 2.5 {
		t.Errorf("expected a single CompletedWork=2.5 op, got %+v", ops)
	}
}
