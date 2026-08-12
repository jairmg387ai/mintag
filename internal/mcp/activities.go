package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// registerActivityTools registers the daily-activity MCP tools: activity_log,
// activity_list, activity_approve, activity_upload, the timelog_projects/
// timelog_categories catalog tools (including catalog_category_set_azure_activity,
// the category→Azure activity default mapping), and the azure_activities
// catalog tools (catalog_azure_activity_add/list/set_default/remove).
func registerActivityTools(s *mcpserver.MCPServer, st *store.Store) {

	// --- activity_log ---
	s.AddTool(mcp.NewTool("activity_log",
		mcp.WithDescription("Record a daily work activity entry. Status starts as 'pending'."),
		mcp.WithString("date", mcp.Description("Date in YYYY-MM-DD format (default: today)")),
		mcp.WithString("hours", mcp.Required(), mcp.Description("Hours worked, e.g. '2.5'")),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name from the catalog")),
		mcp.WithString("category", mcp.Required(), mcp.Description("Activity category from the catalog")),
		mcp.WithString("registro_diario", mcp.Required(), mcp.Description("Work log description (sent as Azure comment)")),
		mcp.WithString("source", mcp.Description("Entry source: 'manual' or 'llm_auto' (default: 'llm_auto')")),
		mcp.WithString("reference_id", mcp.Description("External bug reference: Azure work item ID, Mantis ticket ID, or LuxFlow number")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		date := req.GetString("date", "")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		hoursStr, err := req.RequireString("hours")
		if err != nil {
			return errResult(err)
		}
		hours, err := strconv.ParseFloat(hoursStr, 64)
		if err != nil {
			return errResult(fmt.Errorf("hours must be a number, got %q", hoursStr))
		}

		project, err := req.RequireString("project")
		if err != nil {
			return errResult(err)
		}
		category, err := req.RequireString("category")
		if err != nil {
			return errResult(err)
		}
		registroDiario, err := req.RequireString("registro_diario")
		if err != nil {
			return errResult(err)
		}
		source := req.GetString("source", "llm_auto")

		a, err := st.CreateActivity(ctx, date, hours, project, category, registroDiario, source)
		if err != nil {
			return errResult(err)
		}
		if referenceID := req.GetString("reference_id", ""); referenceID != "" {
			if err := st.SetActivityReferenceID(ctx, a.ID, &referenceID); err != nil {
				return errResult(err)
			}
			a, err = st.GetActivity(ctx, a.ID)
			if err != nil {
				return errResult(err)
			}
		}
		return jsonResult(a, nil)
	})

	// --- activity_list ---
	s.AddTool(mcp.NewTool("activity_list",
		mcp.WithDescription("List daily activity entries, optionally filtered by date and/or status."),
		mcp.WithString("date", mcp.Description("Date filter in YYYY-MM-DD format (default: today)")),
		mcp.WithString("status", mcp.Description("Status filter: pending | approved | uploaded")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		date := req.GetString("date", "")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		status := req.GetString("status", "")

		activities, err := st.ListActivities(ctx, date, status)
		return jsonResult(activities, err)
	})

	// --- activity_approve ---
	s.AddTool(mcp.NewTool("activity_approve",
		mcp.WithDescription("Approve one or more pending activity entries by ID. IDs must be comma-separated."),
		mcp.WithString("ids", mcp.Required(), mcp.Description("Comma-separated activity IDs to approve, e.g. '1,2,3'")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idsStr, err := req.RequireString("ids")
		if err != nil {
			return errResult(err)
		}

		parts := strings.Split(idsStr, ",")
		ids := make([]int64, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			id, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				return errResult(fmt.Errorf("invalid id %q: must be an integer", p))
			}
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return errResult(fmt.Errorf("ids must contain at least one valid integer"))
		}

		approved, err := st.ApproveActivities(ctx, ids)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"approved": approved}, nil)
	})

	// --- activity_update ---
	s.AddTool(mcp.NewTool("activity_update",
		mcp.WithDescription("Update editable fields (hours, project, category, registro_diario) of a pending or approved activity. Activities already uploaded to Azure cannot be edited."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Activity ID to update")),
		mcp.WithString("hours", mcp.Description("New hours value, e.g. '2.5'")),
		mcp.WithString("project", mcp.Description("New project name")),
		mcp.WithString("category", mcp.Description("New activity category")),
		mcp.WithString("registro_diario", mcp.Description("New work log description")),
		mcp.WithString("reference_id", mcp.Description("External bug reference: Azure work item ID, Mantis ticket ID, or LuxFlow number. Leave unchanged if omitted.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("id must be an integer, got %q", idStr))
		}

		var hours float64
		if h := req.GetString("hours", ""); h != "" {
			hours, err = strconv.ParseFloat(h, 64)
			if err != nil {
				return errResult(fmt.Errorf("hours must be a number, got %q", h))
			}
		}

		project := req.GetString("project", "")
		category := req.GetString("category", "")
		registroDiario := req.GetString("registro_diario", "")

		a, err := st.UpdateActivity(ctx, id, hours, project, category, registroDiario)
		if err != nil {
			return errResult(err)
		}
		// Empty means "leave unchanged", consistent with project/category/
		// registro_diario above — this tool doesn't need the REST PATCH's
		// explicit-clear-via-null semantics.
		if referenceID := req.GetString("reference_id", ""); referenceID != "" {
			if err := st.SetActivityReferenceID(ctx, a.ID, &referenceID); err != nil {
				return errResult(err)
			}
			a, err = st.GetActivity(ctx, a.ID)
			if err != nil {
				return errResult(err)
			}
		}
		return jsonResult(a, nil)
	})

	// --- activity_upload ---
	s.AddTool(mcp.NewTool("activity_upload",
		mcp.WithDescription("Upload approved activity entries for a given date to Azure DevOps TimeLog. Requires a configured Azure TimeLog token."),
		mcp.WithString("date", mcp.Description("Date in YYYY-MM-DD format (default: today)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		az, err := st.NewAzureTimeLogClient(ctx)
		if err != nil {
			return errResult(err)
		}
		if az == nil || !az.Enabled() {
			return errResult(fmt.Errorf("Azure TimeLog token is not configured — upload disabled"))
		}

		date := req.GetString("date", "")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		result, err := st.UploadActivities(ctx, date, az)
		return jsonResult(result, err)
	})

	// --- activity_delete ---
	s.AddTool(mcp.NewTool("activity_delete",
		mcp.WithDescription("Delete an activity by ID. Uploaded activities are deleted from Azure TimeLog before the local Mintag row is removed."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Activity ID to delete")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("id must be an integer, got %q", idStr))
		}
		if err := st.DeleteActivityWithAzure(ctx, id, func(ctx context.Context) (store.TimeEntryDeleter, error) {
			return st.NewAzureTimeLogClient(ctx)
		}); err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"deleted": id}, nil)
	})

	// --- catalog_project_add ---
	s.AddTool(mcp.NewTool("catalog_project_add",
		mcp.WithDescription("Add a new project to the TimeLog catalog."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Project name to add")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return errResult(err)
		}
		if err := st.AddTimelogProject(name); err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]string{"added": name}, nil)
	})

	// --- catalog_project_remove ---
	s.AddTool(mcp.NewTool("catalog_project_remove",
		mcp.WithDescription("Remove a project from the TimeLog catalog."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Project name to remove")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return errResult(err)
		}
		if err := st.RemoveTimelogProject(name); err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]string{"removed": name}, nil)
	})

	// --- catalog_category_add ---
	s.AddTool(mcp.NewTool("catalog_category_add",
		mcp.WithDescription("Add a new category to the TimeLog catalog."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Category name to add")),
		mcp.WithString("description", mcp.Description("Optional category description")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return errResult(err)
		}
		description := req.GetString("description", "")
		if err := st.AddTimelogCategory(name, description); err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]string{"added": name}, nil)
	})

	// --- catalog_category_remove ---
	s.AddTool(mcp.NewTool("catalog_category_remove",
		mcp.WithDescription("Remove a category from the TimeLog catalog."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Category name to remove")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return errResult(err)
		}
		if err := st.RemoveTimelogCategory(name); err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]string{"removed": name}, nil)
	})

	// --- catalog_azure_activity_add ---
	s.AddTool(mcp.NewTool("catalog_azure_activity_add",
		mcp.WithDescription("Add a new Azure work-item activity to the catalog. The first activity added automatically becomes the default."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Azure DevOps organization/project key, e.g. 'RUNT2PSW'")),
		mcp.WithString("work_item_id", mcp.Required(), mcp.Description("Azure DevOps work item ID, e.g. '156263'")),
		mcp.WithString("label", mcp.Required(), mcp.Description("Human-readable label for this activity")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		org, err := req.RequireString("org")
		if err != nil {
			return errResult(err)
		}
		workItemIDStr, err := req.RequireString("work_item_id")
		if err != nil {
			return errResult(err)
		}
		workItemID, err := strconv.Atoi(workItemIDStr)
		if err != nil {
			return errResult(fmt.Errorf("work_item_id must be an integer, got %q", workItemIDStr))
		}
		label, err := req.RequireString("label")
		if err != nil {
			return errResult(err)
		}
		a, err := st.AddAzureActivity(ctx, org, workItemID, label)
		return jsonResult(a, err)
	})

	// --- catalog_azure_activity_list ---
	s.AddTool(mcp.NewTool("catalog_azure_activity_list",
		mcp.WithDescription("List Azure work-item activities in the catalog."),
		mcp.WithString("include_inactive", mcp.Description("'true' to include soft-deleted (inactive) activities; default 'false'")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		includeInactive := req.GetString("include_inactive", "") == "true"
		activities, err := st.ListAzureActivities(ctx, includeInactive)
		return jsonResult(activities, err)
	})

	// --- catalog_azure_activity_set_default ---
	s.AddTool(mcp.NewTool("catalog_azure_activity_set_default",
		mcp.WithDescription("Set an Azure activity as the default used when a daily activity has no explicit assignment."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Azure activity ID to promote to default")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("id must be an integer, got %q", idStr))
		}
		if err := st.SetDefaultAzureActivity(ctx, id); err != nil {
			return errResult(err)
		}
		a, err := st.GetDefaultAzureActivity(ctx)
		return jsonResult(a, err)
	})

	// --- catalog_azure_activity_remove ---
	s.AddTool(mcp.NewTool("catalog_azure_activity_remove",
		mcp.WithDescription("Soft-delete (deactivate) an Azure activity from the catalog. The current default cannot be removed; promote another activity to default first."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Azure activity ID to deactivate")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("id must be an integer, got %q", idStr))
		}
		if err := st.DeactivateAzureActivity(ctx, id); err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"deactivated": id}, nil)
	})

	// --- catalog_category_set_azure_activity ---
	s.AddTool(mcp.NewTool("catalog_category_set_azure_activity",
		mcp.WithDescription("Assign or clear a TimeLog category's default Azure activity mapping. Omit or pass an empty azure_activity_id to clear the mapping."),
		mcp.WithString("category_id", mcp.Required(), mcp.Description("TimeLog category ID to update")),
		mcp.WithString("azure_activity_id", mcp.Description("Azure activity ID to map to this category; omit/empty to clear the mapping")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		categoryIDStr, err := req.RequireString("category_id")
		if err != nil {
			return errResult(err)
		}
		categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("category_id must be an integer, got %q", categoryIDStr))
		}

		var azureActivityID *int64
		if azureActivityIDStr := req.GetString("azure_activity_id", ""); azureActivityIDStr != "" {
			id, err := strconv.ParseInt(azureActivityIDStr, 10, 64)
			if err != nil {
				return errResult(fmt.Errorf("azure_activity_id must be an integer, got %q", azureActivityIDStr))
			}
			azureActivityID = &id
		}

		if err := st.SetCategoryAzureActivity(ctx, categoryID, azureActivityID); err != nil {
			return errResult(err)
		}
		categories, err := st.ListTimelogCategories()
		if err != nil {
			return errResult(err)
		}
		for _, c := range categories {
			if c.ID == categoryID {
				return jsonResult(c, nil)
			}
		}
		return errResult(fmt.Errorf("timelog category not found: %d", categoryID))
	})
}
