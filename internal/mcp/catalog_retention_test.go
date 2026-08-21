package mcp

import (
	"context"
	"strings"
	"testing"
)

// TestCatalogProjectRemove_DeactivatesRatherThanHardDeletes verifies the
// catalog_project_remove tool now soft-deletes (is_active=0) instead of hard
// deleting, so historical daily_activities.project references survive.
func TestCatalogProjectRemove_DeactivatesRatherThanHardDeletes(t *testing.T) {
	s, st := newTestMCPServer(t)

	if err := st.AddTimelogProject("Removable"); err != nil {
		t.Fatal(err)
	}

	out := callTool(t, s, "catalog_project_remove", map[string]any{"name": "Removable"})
	if _, ok := out["error"]; ok {
		t.Fatalf("expected no error removing an existing project, got %#v", out)
	}

	names, err := st.ListTimelogProjects(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(names, "Removable") {
		t.Fatalf("expected Removable to still exist (soft-deleted), got %v", names)
	}
	activeNames, err := st.ListTimelogProjects(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if contains(activeNames, "Removable") {
		t.Fatalf("expected Removable excluded from the active listing, got %v", activeNames)
	}
}

// TestCatalogProjectRemove_UnknownNameReturnsError verifies removing a name
// that was never added surfaces DeactivateTimelogProject's not-found error.
func TestCatalogProjectRemove_UnknownNameReturnsError(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_project_remove", map[string]any{"name": "Never Added"})
	errMsg, ok := out["error"].(string)
	if !ok || errMsg == "" {
		t.Fatalf("expected an error result for unknown project name, got %#v", out)
	}
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' in error, got %q", errMsg)
	}
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

// TestCatalogRetentionGet_DefaultsToNull verifies a fresh store reports both
// retention windows as unset/null.
func TestCatalogRetentionGet_DefaultsToNull(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_retention_get", map[string]any{})
	if _, ok := out["bug_retention_days"]; ok && out["bug_retention_days"] != nil {
		t.Errorf("expected bug_retention_days=null, got %#v", out["bug_retention_days"])
	}
	if _, ok := out["project_retention_days"]; ok && out["project_retention_days"] != nil {
		t.Errorf("expected project_retention_days=null, got %#v", out["project_retention_days"])
	}
}

// TestCatalogRetentionSet_RoundTripsThroughGet verifies set persists both
// values and get reads them back.
func TestCatalogRetentionSet_RoundTripsThroughGet(t *testing.T) {
	s, _ := newTestMCPServer(t)

	setOut := callTool(t, s, "catalog_retention_set", map[string]any{
		"bug_retention_days":     "21",
		"project_retention_days": "45",
	})
	if setOut["bug_retention_days"] != float64(21) {
		t.Fatalf("expected bug_retention_days=21, got %#v", setOut["bug_retention_days"])
	}
	if setOut["project_retention_days"] != float64(45) {
		t.Fatalf("expected project_retention_days=45, got %#v", setOut["project_retention_days"])
	}

	getOut := callTool(t, s, "catalog_retention_get", map[string]any{})
	if getOut["bug_retention_days"] != float64(21) {
		t.Fatalf("expected persisted bug_retention_days=21, got %#v", getOut["bug_retention_days"])
	}
	if getOut["project_retention_days"] != float64(45) {
		t.Fatalf("expected persisted project_retention_days=45, got %#v", getOut["project_retention_days"])
	}
}

// TestCatalogRetentionSet_OmittedValueDisablesThatCatalog verifies omitting
// a field clears (disables) that catalog's retention.
func TestCatalogRetentionSet_OmittedValueDisablesThatCatalog(t *testing.T) {
	s, _ := newTestMCPServer(t)

	callTool(t, s, "catalog_retention_set", map[string]any{
		"bug_retention_days":     "21",
		"project_retention_days": "45",
	})
	out := callTool(t, s, "catalog_retention_set", map[string]any{
		"project_retention_days": "45",
	})
	if out["bug_retention_days"] != nil {
		t.Fatalf("expected bug_retention_days cleared to null, got %#v", out["bug_retention_days"])
	}
}

// TestCatalogRetentionSet_NonNumericValueReturnsError verifies a malformed
// days value is rejected rather than silently ignored.
func TestCatalogRetentionSet_NonNumericValueReturnsError(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_retention_set", map[string]any{
		"bug_retention_days": "not-a-number",
	})
	if _, ok := out["error"]; !ok {
		t.Fatalf("expected an error result for non-numeric bug_retention_days, got %#v", out)
	}
}

// TestCatalogRetentionSet_RejectsNonPositive verifies 0/negative values are
// rejected — nil/omitted is the only way to disable.
func TestCatalogRetentionSet_RejectsNonPositive(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_retention_set", map[string]any{
		"bug_retention_days": "0",
	})
	if _, ok := out["error"]; !ok {
		t.Fatalf("expected an error result for bug_retention_days=0, got %#v", out)
	}
}
