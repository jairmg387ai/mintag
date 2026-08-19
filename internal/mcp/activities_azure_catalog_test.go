package mcp

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// newTestMCPServer builds an in-memory store and a fully-registered MCP
// server for exercising activity catalog tools directly through their
// registered ToolHandlerFunc, mirroring the REST package's newTestServer
// helper (internal/server/server_test.go). This is the first test file for
// package mcp — there is no existing local test helper to reuse.
func newTestMCPServer(t *testing.T) (*mcpserver.MCPServer, *store.Store) {
	t.Helper()
	st, err := store.OpenInMemory()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	s := mcpserver.NewMCPServer("mintag-test", "test")
	registerActivityTools(s, st)
	return s, st
}

// callToolText invokes a registered tool's handler directly (bypassing the
// stdio transport) and returns its raw JSON text result.
func callToolText(t *testing.T, s *mcpserver.MCPServer, name string, args map[string]any) string {
	t.Helper()
	tool := s.GetTool(name)
	if tool == nil {
		t.Fatalf("tool %q is not registered", name)
	}
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: name, Arguments: args}}
	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("tool %q handler error: %v", name, err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("tool %q returned no content", name)
	}
	text, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("tool %q returned non-text content: %#v", name, res.Content[0])
	}
	return text.Text
}

// callTool decodes a tool's JSON object result into a map.
func callTool(t *testing.T, s *mcpserver.MCPServer, name string, args map[string]any) map[string]any {
	t.Helper()
	var out map[string]any
	text := callToolText(t, s, name, args)
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("tool %q returned invalid JSON object: %v — %s", name, err, text)
	}
	return out
}

// callToolList decodes a tool's JSON array result into a slice of maps.
func callToolList(t *testing.T, s *mcpserver.MCPServer, name string, args map[string]any) []map[string]any {
	t.Helper()
	var out []map[string]any
	text := callToolText(t, s, name, args)
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("tool %q returned invalid JSON array: %v — %s", name, err, text)
	}
	return out
}

// seedTimelogCategory adds a category and returns its assigned id, resolved
// through ListTimelogCategories since AddTimelogCategory does not return one.
func seedTimelogCategory(t *testing.T, st *store.Store, name string) int64 {
	t.Helper()
	if err := st.AddTimelogCategory(name, ""); err != nil {
		t.Fatalf("seed category %q: %v", name, err)
	}
	categories, err := st.ListTimelogCategories()
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	for _, c := range categories {
		if c.Name == name {
			return c.ID
		}
	}
	t.Fatalf("seeded category %q not found after add", name)
	return 0
}

// TestCatalogAzureActivityAdd_WithProjectAndCategoryID verifies the add tool
// threads project/category_id through to the store and returns them.
func TestCatalogAzureActivityAdd_WithProjectAndCategoryID(t *testing.T) {
	s, st := newTestMCPServer(t)
	categoryID := seedTimelogCategory(t, st, "Bug Triage")

	out := callTool(t, s, "catalog_azure_activity_add", map[string]any{
		"org":          "RUNT2QA",
		"work_item_id": "555111",
		"label":        "Mapped work item",
		"project":      "Alpha",
		"category_id":  strconv.FormatInt(categoryID, 10),
	})

	if out["project"] != "Alpha" {
		t.Fatalf("expected project=Alpha, got %#v", out["project"])
	}
	gotCategoryID, ok := out["category_id"].(float64)
	if !ok || int64(gotCategoryID) != categoryID {
		t.Fatalf("expected category_id=%d, got %#v", categoryID, out["category_id"])
	}
}

// TestCatalogAzureActivityAdd_WithoutMapping_OmitsFields verifies that
// omitting project/category_id keeps them unset (omitempty), matching the
// store's nil-mapping contract.
func TestCatalogAzureActivityAdd_WithoutMapping_OmitsFields(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_azure_activity_add", map[string]any{
		"org":          "RUNT2QA",
		"work_item_id": "555222",
		"label":        "Unmapped work item",
	})

	if _, ok := out["project"]; ok {
		t.Fatalf("expected no project field when unset, got %#v", out["project"])
	}
	if _, ok := out["category_id"]; ok {
		t.Fatalf("expected no category_id field when unset, got %#v", out["category_id"])
	}
}

// TestCatalogAzureActivityAdd_UnknownCategoryIDReturnsError verifies an
// unknown category_id surfaces validateMapping's rejection as a tool error,
// not a silently-dropped mapping.
func TestCatalogAzureActivityAdd_UnknownCategoryIDReturnsError(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_azure_activity_add", map[string]any{
		"org":          "RUNT2QA",
		"work_item_id": "555333",
		"label":        "Dangling category",
		"category_id":  "999999",
	})

	errMsg, ok := out["error"].(string)
	if !ok || errMsg == "" {
		t.Fatalf("expected an error result for unknown category_id, got %#v", out)
	}
	if !strings.Contains(errMsg, "timelog category not found") {
		t.Fatalf("expected 'timelog category not found' in error, got %q", errMsg)
	}
}

// TestCatalogAzureActivityAdd_NonNumericCategoryIDReturnsError verifies a
// malformed category_id is rejected rather than silently ignored.
func TestCatalogAzureActivityAdd_NonNumericCategoryIDReturnsError(t *testing.T) {
	s, _ := newTestMCPServer(t)

	out := callTool(t, s, "catalog_azure_activity_add", map[string]any{
		"org":          "RUNT2QA",
		"work_item_id": "555444",
		"label":        "Bad category id",
		"category_id":  "not-a-number",
	})

	if _, ok := out["error"]; !ok {
		t.Fatalf("expected an error result for non-numeric category_id, got %#v", out)
	}
}

// TestCatalogAzureActivityList_IncludesProjectAndCategoryID verifies the
// serialized AzureActivity fields ride along in the list tool's output
// without any handler code change (struct-level json tags already carry
// them — this locks that contract for the MCP surface specifically).
func TestCatalogAzureActivityList_IncludesProjectAndCategoryID(t *testing.T) {
	s, st := newTestMCPServer(t)
	categoryID := seedTimelogCategory(t, st, "Ops")

	callTool(t, s, "catalog_azure_activity_add", map[string]any{
		"org":          "RUNT2QA",
		"work_item_id": "555555",
		"label":        "Listed mapping",
		"project":      "Beta",
		"category_id":  strconv.FormatInt(categoryID, 10),
	})

	list := callToolList(t, s, "catalog_azure_activity_list", map[string]any{})
	var found map[string]any
	for _, item := range list {
		if item["label"] == "Listed mapping" {
			found = item
		}
	}
	if found == nil {
		t.Fatalf("expected added activity in list output, got %#v", list)
	}
	if found["project"] != "Beta" {
		t.Fatalf("expected project=Beta in list output, got %#v", found["project"])
	}
	gotCategoryID, ok := found["category_id"].(float64)
	if !ok || int64(gotCategoryID) != categoryID {
		t.Fatalf("expected category_id=%d in list output, got %#v", categoryID, found["category_id"])
	}
}
