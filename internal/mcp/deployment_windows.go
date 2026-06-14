package mcp

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// registerDeploymentWindowTools registers all 17 deployment-window MCP tools.
func registerDeploymentWindowTools(s *mcpserver.MCPServer, st *store.Store) {

	// --- dw_create ---
	s.AddTool(mcp.NewTool("dw_create",
		mcp.WithDescription("Create a new deployment window in draft state."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Short title for the deployment window")),
		mcp.WithString("description", mcp.Description("Full description / context")),
		mcp.WithString("created_by", mcp.Description("Author or team creating this window")),
		mcp.WithString("planned_at", mcp.Description("Planned deployment date/time (ISO 8601 or free text)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, err := req.RequireString("title")
		if err != nil {
			return errResult(err)
		}
		description := req.GetString("description", "")
		createdBy := req.GetString("created_by", "")
		plannedAt := req.GetString("planned_at", "")
		dw, err := st.CreateDeploymentWindow(title, description, createdBy, plannedAt)
		return jsonResult(dw, err)
	})

	// --- dw_list ---
	s.AddTool(mcp.NewTool("dw_list",
		mcp.WithDescription("List deployment windows, optionally filtered by state."),
		mcp.WithString("state", mcp.Description("Filter: draft | submitted | approved | deployed (omit for all)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state := req.GetString("state", "")
		dws, err := st.ListDeploymentWindows(state)
		return jsonResult(dws, err)
	})

	// --- dw_get ---
	s.AddTool(mcp.NewTool("dw_get",
		mcp.WithDescription("Get a deployment window with all child collections (tasks, repos, artifacts, test scenarios)."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Deployment window ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid id: %s", idStr))
		}
		detail, err := st.GetDeploymentWindow(id)
		return jsonResult(detail, err)
	})

	// --- dw_update_state ---
	s.AddTool(mcp.NewTool("dw_update_state",
		mcp.WithDescription("Transition a deployment window to a new state. Drives the state machine and syncs graph edges on submit."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Deployment window ID")),
		mcp.WithString("state", mcp.Required(), mcp.Description("Target state: submitted | approved | draft | deployed")),
		mcp.WithString("rejection_note", mcp.Description("Required when returning submitted→draft; explains the rejection")),
		mcp.WithString("namespace", mcp.Description("Graph namespace for edge creation (auto-detected if omitted)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid id: %s", idStr))
		}
		state, err := req.RequireString("state")
		if err != nil {
			return errResult(err)
		}
		rejectionNote := req.GetString("rejection_note", "")
		namespace := req.GetString("namespace", "")
		dw, err := st.UpdateDeploymentWindowState(id, state, rejectionNote, namespace)
		return jsonResult(dw, err)
	})

	// --- dw_add_task ---
	s.AddTool(mcp.NewTool("dw_add_task",
		mcp.WithDescription("Attach an existing task to a deployment window (draft only)."),
		mcp.WithString("dw_id", mcp.Required(), mcp.Description("Deployment window ID")),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task ID to attach")),
		mcp.WithString("note", mcp.Description("Optional note about why this task is included")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dwIDStr, err := req.RequireString("dw_id")
		if err != nil {
			return errResult(err)
		}
		dwID, err := strconv.ParseInt(dwIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid dw_id: %s", dwIDStr))
		}
		taskIDStr, err := req.RequireString("task_id")
		if err != nil {
			return errResult(err)
		}
		taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid task_id: %s", taskIDStr))
		}
		note := req.GetString("note", "")
		err = st.AddDWTask(dwID, taskID, note)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"dw_id": dwID, "task_id": taskID, "note": note}, nil)
	})

	// --- dw_remove_task ---
	s.AddTool(mcp.NewTool("dw_remove_task",
		mcp.WithDescription("Remove a task from a deployment window (draft only)."),
		mcp.WithString("dw_id", mcp.Required(), mcp.Description("Deployment window ID")),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task ID to remove")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dwIDStr, err := req.RequireString("dw_id")
		if err != nil {
			return errResult(err)
		}
		dwID, err := strconv.ParseInt(dwIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid dw_id: %s", dwIDStr))
		}
		taskIDStr, err := req.RequireString("task_id")
		if err != nil {
			return errResult(err)
		}
		taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid task_id: %s", taskIDStr))
		}
		err = st.RemoveDWTask(dwID, taskID)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"removed": true, "dw_id": dwID, "task_id": taskID}, nil)
	})

	// --- dw_add_repo ---
	s.AddTool(mcp.NewTool("dw_add_repo",
		mcp.WithDescription("Add a repository version reference to a deployment window (draft only)."),
		mcp.WithString("dw_id", mcp.Required(), mcp.Description("Deployment window ID")),
		mcp.WithString("graph_node_key", mcp.Required(), mcp.Description("Graph node key of the repo (e.g. 'my-repo')")),
		mcp.WithString("version", mcp.Required(), mcp.Description("Version or tag to deploy (e.g. '1.4.2' or 'abc1234')")),
		mcp.WithString("notes", mcp.Description("Deployment notes for this repo")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dwIDStr, err := req.RequireString("dw_id")
		if err != nil {
			return errResult(err)
		}
		dwID, err := strconv.ParseInt(dwIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid dw_id: %s", dwIDStr))
		}
		graphNodeKey, err := req.RequireString("graph_node_key")
		if err != nil {
			return errResult(err)
		}
		version, err := req.RequireString("version")
		if err != nil {
			return errResult(err)
		}
		notes := req.GetString("notes", "")
		repo, err := st.AddDWRepo(dwID, graphNodeKey, version, notes)
		return jsonResult(repo, err)
	})

	// --- dw_update_repo ---
	s.AddTool(mcp.NewTool("dw_update_repo",
		mcp.WithDescription("Update the version and notes of a repo reference (draft only)."),
		mcp.WithString("id", mcp.Required(), mcp.Description("DW repo row ID")),
		mcp.WithString("version", mcp.Required(), mcp.Description("New version or tag")),
		mcp.WithString("notes", mcp.Description("Updated deployment notes")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid id: %s", idStr))
		}
		version, err := req.RequireString("version")
		if err != nil {
			return errResult(err)
		}
		notes := req.GetString("notes", "")
		repo, err := st.UpdateDWRepo(id, version, notes)
		return jsonResult(repo, err)
	})

	// --- dw_remove_repo ---
	s.AddTool(mcp.NewTool("dw_remove_repo",
		mcp.WithDescription("Remove a repository reference from a deployment window (draft only)."),
		mcp.WithString("id", mcp.Required(), mcp.Description("DW repo row ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid id: %s", idStr))
		}
		err = st.RemoveDWRepo(id)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"removed": true, "id": id}, nil)
	})

	// --- dw_add_artifact ---
	s.AddTool(mcp.NewTool("dw_add_artifact",
		mcp.WithDescription("Add an artifact (db_script, config, blob, other) to a deployment window (draft only)."),
		mcp.WithString("dw_id", mcp.Required(), mcp.Description("Deployment window ID")),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Artifact kind: db_script | blob | config | other")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Artifact name")),
		mcp.WithString("path", mcp.Description("File path (informational)")),
		mcp.WithString("content", mcp.Description("Artifact content (inline text)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dwIDStr, err := req.RequireString("dw_id")
		if err != nil {
			return errResult(err)
		}
		dwID, err := strconv.ParseInt(dwIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid dw_id: %s", dwIDStr))
		}
		kind, err := req.RequireString("kind")
		if err != nil {
			return errResult(err)
		}
		name, err := req.RequireString("name")
		if err != nil {
			return errResult(err)
		}
		path := req.GetString("path", "")
		content := req.GetString("content", "")
		artifact, err := st.AddDWArtifact(dwID, kind, name, path, content)
		return jsonResult(artifact, err)
	})

	// --- dw_update_artifact ---
	s.AddTool(mcp.NewTool("dw_update_artifact",
		mcp.WithDescription("Update an artifact's kind, name, path, or content (draft only)."),
		mcp.WithString("id", mcp.Required(), mcp.Description("DW artifact row ID")),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Artifact kind: db_script | blob | config | other")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Artifact name")),
		mcp.WithString("path", mcp.Description("File path (informational)")),
		mcp.WithString("content", mcp.Description("Artifact content (inline text)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid id: %s", idStr))
		}
		kind, err := req.RequireString("kind")
		if err != nil {
			return errResult(err)
		}
		name, err := req.RequireString("name")
		if err != nil {
			return errResult(err)
		}
		path := req.GetString("path", "")
		content := req.GetString("content", "")
		artifact, err := st.UpdateDWArtifact(id, kind, name, path, content)
		return jsonResult(artifact, err)
	})

	// --- dw_remove_artifact ---
	s.AddTool(mcp.NewTool("dw_remove_artifact",
		mcp.WithDescription("Remove an artifact from a deployment window (draft only)."),
		mcp.WithString("id", mcp.Required(), mcp.Description("DW artifact row ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid id: %s", idStr))
		}
		err = st.RemoveDWArtifact(id)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"removed": true, "id": id}, nil)
	})

	// --- dw_add_test_scenario ---
	s.AddTool(mcp.NewTool("dw_add_test_scenario",
		mcp.WithDescription("Add a QA test scenario to a deployment window (allowed in any non-deployed state)."),
		mcp.WithString("dw_id", mcp.Required(), mcp.Description("Deployment window ID")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Short scenario title")),
		mcp.WithString("description", mcp.Description("Detailed scenario description")),
		mcp.WithString("expected", mcp.Description("Expected outcome")),
		mcp.WithString("sort_order", mcp.Description("Display order integer (default 0)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dwIDStr, err := req.RequireString("dw_id")
		if err != nil {
			return errResult(err)
		}
		dwID, err := strconv.ParseInt(dwIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid dw_id: %s", dwIDStr))
		}
		title, err := req.RequireString("title")
		if err != nil {
			return errResult(err)
		}
		description := req.GetString("description", "")
		expected := req.GetString("expected", "")
		sortOrder := 0
		if s := req.GetString("sort_order", ""); s != "" {
			sortOrder, _ = strconv.Atoi(s)
		}
		sc, err := st.AddDWTestScenario(dwID, title, description, expected, sortOrder)
		return jsonResult(sc, err)
	})

	// --- dw_update_test_scenario ---
	s.AddTool(mcp.NewTool("dw_update_test_scenario",
		mcp.WithDescription("Update a test scenario's title, description, expected outcome, or sort order."),
		mcp.WithString("id", mcp.Required(), mcp.Description("DW test scenario row ID")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Updated title")),
		mcp.WithString("description", mcp.Description("Updated description")),
		mcp.WithString("expected", mcp.Description("Updated expected outcome")),
		mcp.WithString("sort_order", mcp.Description("Updated display order integer")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid id: %s", idStr))
		}
		title, err := req.RequireString("title")
		if err != nil {
			return errResult(err)
		}
		description := req.GetString("description", "")
		expected := req.GetString("expected", "")
		sortOrder := 0
		if s := req.GetString("sort_order", ""); s != "" {
			sortOrder, _ = strconv.Atoi(s)
		}
		sc, err := st.UpdateDWTestScenario(id, title, description, expected, sortOrder)
		return jsonResult(sc, err)
	})

	// --- dw_remove_test_scenario ---
	s.AddTool(mcp.NewTool("dw_remove_test_scenario",
		mcp.WithDescription("Remove a test scenario from a deployment window (not allowed once deployed)."),
		mcp.WithString("id", mcp.Required(), mcp.Description("DW test scenario row ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid id: %s", idStr))
		}
		err = st.RemoveDWTestScenario(id)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"removed": true, "id": id}, nil)
	})

	// --- dw_sign_off_scenario ---
	s.AddTool(mcp.NewTool("dw_sign_off_scenario",
		mcp.WithDescription("Record an auditor's sign-off (pass or fail) on a test scenario."),
		mcp.WithString("id", mcp.Required(), mcp.Description("DW test scenario row ID")),
		mcp.WithString("result", mcp.Required(), mcp.Description("Sign-off result: pass | fail")),
		mcp.WithString("signed_off_by", mcp.Required(), mcp.Description("Name or identifier of the auditor")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid id: %s", idStr))
		}
		result, err := req.RequireString("result")
		if err != nil {
			return errResult(err)
		}
		signedOffBy, err := req.RequireString("signed_off_by")
		if err != nil {
			return errResult(err)
		}
		sc, err := st.SignOffTestScenario(id, result, signedOffBy)
		return jsonResult(sc, err)
	})

	// --- dw_export_markdown ---
	s.AddTool(mcp.NewTool("dw_export_markdown",
		mcp.WithDescription("Export a deployment window as a full Spanish-language markdown document for handoff to the oversight body (interventoría)."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Deployment window ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idStr, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid id: %s", idStr))
		}
		md, err := st.ExportDeploymentWindowMarkdown(id)
		if err != nil {
			return errResult(err)
		}
		return mcp.NewToolResultText(md), nil
	})
}
