package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/Gentleman-Programming/mintag/internal/azure"
	"github.com/Gentleman-Programming/mintag/internal/store"
)

// registerActivityTools registers the four daily-activity MCP tools:
// activity_log, activity_list, activity_approve, activity_upload.
func registerActivityTools(s *mcpserver.MCPServer, st *store.Store, az *azure.Client) {

	// --- activity_log ---
	s.AddTool(mcp.NewTool("activity_log",
		mcp.WithDescription("Record a daily work activity entry. Status starts as 'pending'."),
		mcp.WithString("date", mcp.Description("Date in YYYY-MM-DD format (default: today)")),
		mcp.WithString("hours", mcp.Required(), mcp.Description("Hours worked, e.g. '2.5'")),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name from the catalog")),
		mcp.WithString("category", mcp.Required(), mcp.Description("Activity category from the catalog")),
		mcp.WithString("registro_diario", mcp.Required(), mcp.Description("Work log description (sent as Azure comment)")),
		mcp.WithString("source", mcp.Description("Entry source: 'manual' or 'llm_auto' (default: 'llm_auto')")),
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
		return jsonResult(a, err)
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

	// --- activity_upload ---
	s.AddTool(mcp.NewTool("activity_upload",
		mcp.WithDescription("Upload approved activity entries for a given date to Azure DevOps TimeLog. Requires MINTAG_AZURE_PAT environment variable."),
		mcp.WithString("date", mcp.Description("Date in YYYY-MM-DD format (default: today)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if az == nil || !az.Enabled() {
			return errResult(fmt.Errorf("MINTAG_AZURE_PAT not set — upload disabled"))
		}

		date := req.GetString("date", "")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		result, err := st.UploadActivities(ctx, date, az)
		return jsonResult(result, err)
	})
}
