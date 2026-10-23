package mcp

import (
	"context"
	"fmt"
	"github.com/Gentleman-Programming/mintag/internal/store"
	"strings"
	"testing"
)

func TestActivityLinkMCP(t *testing.T) {
	s, st := newTestMCPServer(t)
	ctx := context.Background()
	task, err := st.AddAzureActivity(ctx, "org", 771, "Task", "Task", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	otherTask, err := st.AddAzureActivity(ctx, "org", 774, "Other task", "Task", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	bug, err := st.AddAzureActivity(ctx, "org", 772, "Bug", "Bug", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	inactive, err := st.AddAzureActivity(ctx, "org", 773, "Inactive", "Task", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeactivateAzureActivity(ctx, inactive.ID); err != nil {
		t.Fatal(err)
	}
	base := map[string]any{"date": "2025-01-06", "hours": "1", "project": "Alpha", "category": "Dev", "registro_diario": "Work"}
	copyArgs := func() map[string]any {
		a := map[string]any{}
		for k, v := range base {
			a[k] = v
		}
		return a
	}
	for _, tc := range []struct {
		name string
		id   string
		want string
	}{
		{"missing", "999999", "azure activity not found: 999999"},
		{"inactive", fmt.Sprint(inactive.ID), fmt.Sprintf("azure activity %d is inactive and cannot be assigned", inactive.ID)},
		{"bug", fmt.Sprint(bug.ID), "Bug"},
		{"malformed", "bad", "azure_activity_id must be an integer"},
		{"blank", "", "azure_activity_id must be an integer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := copyArgs()
			args["azure_activity_id"] = tc.id
			args["reference_id"] = "BUG-42"
			before := len(callToolList(t, s, "activity_list", map[string]any{"date": "2025-01-06"}))
			out := callTool(t, s, "activity_log", args)
			if !strings.Contains(fmt.Sprint(out["error"]), tc.want) {
				t.Fatalf("error = %#v, want %q", out, tc.want)
			}
			after := len(callToolList(t, s, "activity_list", map[string]any{"date": "2025-01-06"}))
			if after != before {
				t.Fatalf("rejected log changed count: %d to %d", before, after)
			}
		})
	}
	args := copyArgs()
	args["azure_activity_id"] = fmt.Sprint(task.ID)
	args["reference_id"] = "BUG-42"
	created := callTool(t, s, "activity_log", args)
	if created["azure_activity_id"] != float64(task.ID) || created["reference_id"] != "BUG-42" {
		t.Fatalf("linked log: %#v", created)
	}
	id := fmt.Sprint(int64(created["id"].(float64)))
	for _, tc := range []struct {
		name string
		id   string
		want string
	}{
		{"missing", "999999", "azure activity not found: 999999"},
		{"inactive", fmt.Sprint(inactive.ID), fmt.Sprintf("azure activity %d is inactive and cannot be assigned", inactive.ID)},
		{"bug", fmt.Sprint(bug.ID), "Bug"},
		{"malformed", "bad", "azure_activity_id must be an integer"},
		{"blank", "", "azure_activity_id must be an integer"},
	} {
		t.Run("update_"+tc.name, func(t *testing.T) {
			out := callTool(t, s, "activity_update", map[string]any{"id": id, "hours": "2", "date": "2025-01-07", "reference_id": "changed", "azure_activity_id": tc.id})
			if !strings.Contains(fmt.Sprint(out["error"]), tc.want) {
				t.Fatalf("error = %#v, want %q", out, tc.want)
			}
			rows := callToolList(t, s, "activity_list", map[string]any{"date": "2025-01-06"})
			if len(rows) != 1 || rows[0]["hours"] != float64(1) || rows[0]["reference_id"] != "BUG-42" || rows[0]["azure_activity_id"] != float64(task.ID) {
				t.Fatalf("partial update: %#v", rows)
			}
			if len(callToolList(t, s, "activity_list", map[string]any{"date": "2025-01-07"})) != 0 {
				t.Fatal("date changed")
			}
		})
	}
	updated := callTool(t, s, "activity_update", map[string]any{"id": id, "azure_activity_id": fmt.Sprint(otherTask.ID), "reference_id": "FREE-TEXT"})
	if updated["azure_activity_id"] != float64(otherTask.ID) || updated["reference_id"] != "FREE-TEXT" {
		t.Fatalf("update: %#v", updated)
	}
	plain := callTool(t, s, "activity_log", func() map[string]any { a := copyArgs(); a["reference_id"] = "MANTIS-9"; return a }())
	if plain["reference_id"] != "MANTIS-9" {
		t.Fatalf("plain reference: %#v", plain)
	}
	withoutLink := callTool(t, s, "activity_update", map[string]any{"id": id, "hours": "3"})
	if withoutLink["azure_activity_id"] != float64(otherTask.ID) || withoutLink["reference_id"] != "FREE-TEXT" {
		t.Fatalf("unrelated update changed link or reference: %#v", withoutLink)
	}
	if _, ok := plain["azure_activity_id"]; ok {
		t.Fatalf("unexpected link: %#v", plain)
	}
}
