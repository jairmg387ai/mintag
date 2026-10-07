package mcp

import (
	"context"
	"testing"
)

// TestActivityValidationGet_ReportsBugGuardOnByDefault verifies the MCP read
// of the activity validation toggles includes the TimeLog bug guard, on by
// default unlike its three siblings.
func TestActivityValidationGet_ReportsBugGuardOnByDefault(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "activity_validation_get", map[string]any{})
	if out["block_bug_work_item"] != true {
		t.Errorf("expected block_bug_work_item=true, got %#v", out["block_bug_work_item"])
	}
	if out["block_closed_work_item"] != false {
		t.Errorf("expected block_closed_work_item=false, got %#v", out["block_closed_work_item"])
	}
}

// TestActivityValidationSet_UpdatesOnlyProvidedToggles verifies set changes
// only the toggles passed and leaves omitted ones as they were.
func TestActivityValidationSet_UpdatesOnlyProvidedToggles(t *testing.T) {
	s, st := newTestMCPServer(t)

	out := callTool(t, s, "activity_validation_set", map[string]any{"block_bug_work_item": "false"})
	if _, ok := out["error"]; ok {
		t.Fatalf("unexpected error: %#v", out)
	}
	if out["block_bug_work_item"] != false || out["block_closed_work_item"] != false {
		t.Errorf("unexpected settings after set: %#v", out)
	}
	v, err := st.GetActivityValidationSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.BlockBugWorkItem || v.MaxHoursPerEntry || v.WeekendConfirm || v.BlockClosedWorkItem {
		t.Errorf("expected only the bug guard to change (to off), got %+v", v)
	}

	out = callTool(t, s, "activity_validation_set", map[string]any{"block_closed_work_item": "true"})
	if out["block_closed_work_item"] != true || out["block_bug_work_item"] != false {
		t.Errorf("expected omitted block_bug_work_item to stay false, got %#v", out)
	}
}

func TestActivityValidationSet_InvalidBoolReturnsError(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "activity_validation_set", map[string]any{"block_bug_work_item": "maybe"})
	if _, ok := out["error"]; !ok {
		t.Fatalf("expected an error for a non-boolean value, got %#v", out)
	}
}
