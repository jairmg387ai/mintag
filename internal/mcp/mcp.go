package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/Gentleman-Programming/mintag/internal/parser"
	"github.com/Gentleman-Programming/mintag/internal/store"
)

func Serve(st *store.Store) error {
	s := mcpserver.NewMCPServer("mintag", "1.0.0",
		mcpserver.WithToolCapabilities(true),
	)
	registerTools(s, st)
	return mcpserver.ServeStdio(s)
}

func registerTools(s *mcpserver.MCPServer, st *store.Store) {

	// --- project_create ---
	s.AddTool(mcp.NewTool("project_create",
		mcp.WithDescription("Create a new project to group meetings and tasks"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Project name")),
		mcp.WithString("description", mcp.Description("Short description")),
		mcp.WithString("color", mcp.Description("Hex color e.g. #1a6faf")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return errResult(err)
		}
		desc := req.GetString("description", "")
		color := req.GetString("color", "")
		p, err := st.CreateProject(name, desc, color)
		return jsonResult(p, err)
	})

	// --- project_list ---
	s.AddTool(mcp.NewTool("project_list",
		mcp.WithDescription("List all projects"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ps, err := st.ListProjects()
		return jsonResult(ps, err)
	})

	// --- meeting_import ---
	s.AddTool(mcp.NewTool("meeting_import",
		mcp.WithDescription("Import a .vtt or .txt meeting transcription from disk. Automatically parses speakers and timestamps."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the .vtt or .txt file")),
		mcp.WithString("project_id", mcp.Description("Project ID (optional)")),
		mcp.WithString("summary", mcp.Description("Brief summary of the meeting (optional)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := req.RequireString("path")
		if err != nil {
			return errResult(err)
		}
		summary := req.GetString("summary", "")

		raw, err := os.ReadFile(path)
		if err != nil {
			return errResult(fmt.Errorf("cannot read file: %w", err))
		}
		pm := parser.Parse(path, string(raw))

		projectID, _ := parsePtrID(req, "project_id")

		m, err := st.CreateMeeting(projectID, path, pm.Date, pm.Title, pm.Content, summary)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{
			"meeting":       m,
			"content_chars": len(pm.Content),
			"hint":          "Use task_create with this meeting_id to record tasks identified in the transcription",
		}, nil)
	})

	// --- meeting_search ---
	s.AddTool(mcp.NewTool("meeting_search",
		mcp.WithDescription("Full-text search across all imported meeting transcriptions (FTS5)"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search terms")),
		mcp.WithString("limit", mcp.Description("Max results, default 10")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q, err := req.RequireString("query")
		if err != nil {
			return errResult(err)
		}
		limit := req.GetInt("limit", 10)
		results, err := st.Search(q, limit)
		var meetings []*store.SearchResult
		for _, r := range results {
			if r.Kind == "meeting" {
				meetings = append(meetings, r)
			}
		}
		return jsonResult(meetings, err)
	})

	// --- task_create ---
	s.AddTool(mcp.NewTool("task_create",
		mcp.WithDescription("Create a task identified from a meeting or manually"),
		mcp.WithString("title", mcp.Required(), mcp.Description("Short task title")),
		mcp.WithString("description", mcp.Description("Full description / context")),
		mcp.WithString("owner", mcp.Description("Responsible person")),
		mcp.WithString("project_id", mcp.Description("Project ID")),
		mcp.WithString("meeting_id", mcp.Description("Origin meeting ID")),
		mcp.WithString("priority", mcp.Description("low | medium | high | critical")),
		mcp.WithString("status", mcp.Description("todo | in_progress | blocked | done")),
		mcp.WithString("due_date", mcp.Description("Due date YYYY-MM-DD")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, err := req.RequireString("title")
		if err != nil {
			return errResult(err)
		}
		desc := req.GetString("description", "")
		owner := req.GetString("owner", "")
		priority := req.GetString("priority", "medium")
		status := req.GetString("status", "todo")
		dueDate := req.GetString("due_date", "")

		projectID, _ := parsePtrID(req, "project_id")
		meetingID, _ := parsePtrID(req, "meeting_id")

		t, err := st.CreateTask(meetingID, projectID, title, desc, status, priority, owner, dueDate)
		return jsonResult(t, err)
	})

	// --- task_update ---
	s.AddTool(mcp.NewTool("task_update",
		mcp.WithDescription("Update a task status, owner, or any field. Every change is recorded in history."),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task ID")),
		mcp.WithString("status", mcp.Description("todo | in_progress | blocked | done | cancelled")),
		mcp.WithString("owner", mcp.Description("New responsible person")),
		mcp.WithString("priority", mcp.Description("low | medium | high | critical")),
		mcp.WithString("due_date", mcp.Description("New due date YYYY-MM-DD")),
		mcp.WithString("description", mcp.Description("Updated description")),
		mcp.WithString("note", mcp.Description("Reason for this update")),
		mcp.WithString("author", mcp.Description("Who is making this update")),
		mcp.WithString("source_meeting_id", mcp.Description("Meeting ID that triggered this update")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskIDStr, err := req.RequireString("task_id")
		if err != nil {
			return errResult(err)
		}
		taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid task_id: %s", taskIDStr))
		}

		updates := map[string]any{}
		for _, f := range []string{"status", "owner", "priority", "due_date", "description"} {
			if v := req.GetString(f, ""); v != "" {
				updates[f] = v
			}
		}

		note := req.GetString("note", "")
		author := req.GetString("author", "")

		sourceMeetingID, _ := parsePtrID(req, "source_meeting_id")

		t, err := st.UpdateTask(taskID, updates, note, author, sourceMeetingID)
		return jsonResult(t, err)
	})

	// --- task_search ---
	s.AddTool(mcp.NewTool("task_search",
		mcp.WithDescription("Full-text search across all tasks (title, description, owner)"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search terms")),
		mcp.WithString("limit", mcp.Description("Max results, default 20")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q, err := req.RequireString("query")
		if err != nil {
			return errResult(err)
		}
		limit := req.GetInt("limit", 20)
		results, err := st.Search(q, limit)
		var tasks []*store.SearchResult
		for _, r := range results {
			if r.Kind == "task" {
				tasks = append(tasks, r)
			}
		}
		return jsonResult(tasks, err)
	})

	// --- task_history ---
	s.AddTool(mcp.NewTool("task_history",
		mcp.WithDescription("Get the full change history of a task across all meetings"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskIDStr, err := req.RequireString("task_id")
		if err != nil {
			return errResult(err)
		}
		taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid task_id"))
		}
		task, err := st.GetTask(taskID)
		if err != nil {
			return errResult(err)
		}
		history, err := st.GetTaskHistory(taskID)
		return jsonResult(map[string]any{"task": task, "history": history}, err)
	})

	// --- meeting_set_rich_content ---
	s.AddTool(mcp.NewTool("meeting_set_rich_content",
		mcp.WithDescription("Set rich content (Markdown or HTML) on a meeting record"),
		mcp.WithString("meeting_id", mcp.Required(), mcp.Description("Meeting ID")),
		mcp.WithString("content", mcp.Required(), mcp.Description("The rich content to store")),
		mcp.WithString("content_type", mcp.Required(), mcp.Description("markdown | html")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		meetingIDStr, err := req.RequireString("meeting_id")
		if err != nil {
			return errResult(err)
		}
		meetingID, err := strconv.ParseInt(meetingIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid meeting_id: %s", meetingIDStr))
		}
		content, err := req.RequireString("content")
		if err != nil {
			return errResult(err)
		}
		contentType, err := req.RequireString("content_type")
		if err != nil {
			return errResult(err)
		}
		if contentType != "markdown" && contentType != "html" {
			return errResult(fmt.Errorf("invalid content_type: must be markdown or html"))
		}
		m, err := st.UpdateMeetingRichContent(meetingID, content, contentType)
		return jsonResult(m, err)
	})

	// --- tasks_by_project ---
	s.AddTool(mcp.NewTool("tasks_by_project",
		mcp.WithDescription("List all tasks for a project, optionally filtered by status"),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
		mcp.WithString("status", mcp.Description("Filter: todo | in_progress | blocked | done")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectIDStr, err := req.RequireString("project_id")
		if err != nil {
			return errResult(err)
		}
		projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
		if err != nil {
			return errResult(fmt.Errorf("invalid project_id"))
		}
		status := req.GetString("status", "")
		tasks, err := st.ListTasks(&projectID, status)

		summary := map[string]int{"todo": 0, "in_progress": 0, "blocked": 0, "done": 0}
		for _, t := range tasks {
			summary[t.Status]++
		}
		return jsonResult(map[string]any{
			"tasks":   tasks,
			"summary": summary,
			"total":   len(tasks),
		}, err)
	})

	// --- meeting_find_or_create ---
	s.AddTool(mcp.NewTool("meeting_find_or_create",
		mcp.WithDescription("Find an existing meeting by filename or create it. Dedup-safe: never creates duplicates. Returns {action: created|updated|skipped, id, filename, meeting}."),
		mcp.WithString("filename", mcp.Required(), mcp.Description("Canonical filename of the transcript (e.g. standup-2026-06-05.vtt)")),
		mcp.WithString("title", mcp.Description("Meeting title (used on create)")),
		mcp.WithString("date", mcp.Description("ISO date YYYY-MM-DD (used on create)")),
		mcp.WithString("content", mcp.Description("Transcript body (used on create or update if differs)")),
		mcp.WithString("summary", mcp.Description("Meeting summary (used on create or update if differs)")),
		mcp.WithString("project_id", mcp.Description("Project ID (optional)")),
		mcp.WithString("candidate_id", mcp.Description("Advisory hint: ID from a previous meeting_search result")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filename, err := req.RequireString("filename")
		if err != nil {
			return errResult(err)
		}
		title := req.GetString("title", "")
		date := req.GetString("date", "")
		content := req.GetString("content", "")
		summary := req.GetString("summary", "")

		projectID, err := parsePtrID(req, "project_id")
		if err != nil {
			return errResult(fmt.Errorf("invalid project_id: %w", err))
		}
		candidateID, err := parsePtrID(req, "candidate_id")
		if err != nil {
			return errResult(fmt.Errorf("invalid candidate_id: %w", err))
		}

		m, action, err := st.UpsertMeeting(projectID, filename, date, title, content, summary, candidateID)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{
			"action":   action,
			"id":       m.ID,
			"filename": m.Filename,
			"meeting":  m,
		}, nil)
	})

	// --- task_upsert ---
	s.AddTool(mcp.NewTool("task_upsert",
		mcp.WithDescription("Find an existing task by title+project or create it. Dedup-safe. Returns {action: created|updated|skipped|ambiguous, id, title, task}. On ambiguous, surface candidates to user — do NOT retry without a candidate_id."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Task title — primary match key")),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID (required for deterministic match)")),
		mcp.WithString("source_meeting_id", mcp.Description("Meeting that triggered this upsert (recorded in task history)")),
		mcp.WithString("status", mcp.Description("todo | in_progress | blocked | done | cancelled")),
		mcp.WithString("priority", mcp.Description("low | medium | high | critical")),
		mcp.WithString("owner", mcp.Description("Responsible person")),
		mcp.WithString("description", mcp.Description("Task description / context")),
		mcp.WithString("due_date", mcp.Description("Due date YYYY-MM-DD")),
		mcp.WithString("candidate_id", mcp.Description("Advisory hint: ID from a previous task_search result")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, err := req.RequireString("title")
		if err != nil {
			return errResult(err)
		}
		description := req.GetString("description", "")
		status := req.GetString("status", "")
		priority := req.GetString("priority", "")
		owner := req.GetString("owner", "")
		dueDate := req.GetString("due_date", "")

		projectID, err := parsePtrID(req, "project_id")
		if err != nil {
			return errResult(fmt.Errorf("invalid project_id: %w", err))
		}
		sourceMeetingID, err := parsePtrID(req, "source_meeting_id")
		if err != nil {
			return errResult(fmt.Errorf("invalid source_meeting_id: %w", err))
		}
		candidateID, err := parsePtrID(req, "candidate_id")
		if err != nil {
			return errResult(fmt.Errorf("invalid candidate_id: %w", err))
		}

		t, action, err := st.UpsertTask(sourceMeetingID, projectID, title, description, status, priority, owner, dueDate, candidateID)
		if err != nil {
			return errResult(err)
		}

		if action == "ambiguous" {
			// fetch candidates so the caller can surface them
			var candidateIDs []int64
			if projectID != nil {
				candidates, ferr := st.FindTaskByTitleAndProject(title, *projectID)
				if ferr == nil {
					for _, c := range candidates {
						candidateIDs = append(candidateIDs, c.ID)
					}
				}
			}
			return jsonResult(map[string]any{
				"action":     "ambiguous",
				"candidates": candidateIDs,
			}, nil)
		}

		return jsonResult(map[string]any{
			"action": action,
			"id":     t.ID,
			"title":  t.Title,
			"task":   t,
		}, nil)
	})
}

// --- helpers ---

func jsonResult(v any, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return errResult(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return errResult(err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

func errResult(err error) (*mcp.CallToolResult, error) {
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
	}
	return mcp.NewToolResultText(fmt.Sprintf(`{"error":"%s"}`, strings.ReplaceAll(msg, `"`, `'`))), nil
}

// parsePtrID reads an optional string parameter by key and parses it as
// int64. Returns (nil, nil) when the parameter is absent or empty.
// Returns (nil, err) on a non-numeric value.
func parsePtrID(req mcp.CallToolRequest, key string) (*int64, error) {
	v := req.GetString(key, "")
	if v == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, err
	}
	return &id, nil
}
