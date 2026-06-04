package store

import (
	"testing"
)

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func openTestDB(t *testing.T) *Store {
	t.Helper()
	s, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestProject_CRUD verifies CreateProject, GetProject, and ListProjects.
func TestProject_CRUD(t *testing.T) {
	s := openTestDB(t)

	// empty DB — ListProjects must return non-nil empty slice
	projs, err := s.ListProjects()
	mustNoErr(t, err)
	if projs == nil {
		t.Error("ListProjects on empty DB must return non-nil slice")
	}
	if len(projs) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projs))
	}

	// create
	p, err := s.CreateProject("Alpha", "first project", "#ff0000")
	mustNoErr(t, err)
	if p.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if p.Name != "Alpha" {
		t.Errorf("expected name 'Alpha', got %q", p.Name)
	}
	if p.Color != "#ff0000" {
		t.Errorf("expected color '#ff0000', got %q", p.Color)
	}

	// empty color defaults
	p2, err := s.CreateProject("Beta", "", "")
	mustNoErr(t, err)
	if p2.Color != "#1a6faf" {
		t.Errorf("empty color should default to '#1a6faf', got %q", p2.Color)
	}

	// GetProject
	got, err := s.GetProject(p.ID)
	mustNoErr(t, err)
	if got.Name != "Alpha" {
		t.Errorf("GetProject: expected name 'Alpha', got %q", got.Name)
	}

	// ListProjects now has 2
	projs, err = s.ListProjects()
	mustNoErr(t, err)
	if len(projs) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projs))
	}
}

// TestMeeting_CRUD verifies CreateMeeting, GetMeeting, and ListMeetings.
func TestMeeting_CRUD(t *testing.T) {
	s := openTestDB(t)

	// empty DB — nil-safe
	meetings, err := s.ListMeetings(nil)
	mustNoErr(t, err)
	if meetings == nil {
		t.Error("ListMeetings on empty DB must return non-nil slice")
	}
	if len(meetings) != 0 {
		t.Errorf("expected 0 meetings, got %d", len(meetings))
	}

	// create
	m, err := s.CreateMeeting(nil, "file.vtt", "2026-01-15", "Stand-up", "content here", "summary here")
	mustNoErr(t, err)
	if m.ID == 0 {
		t.Error("expected non-zero meeting ID")
	}
	if m.Title != "Stand-up" {
		t.Errorf("expected title 'Stand-up', got %q", m.Title)
	}

	// GetMeeting returns raw_content
	got, err := s.GetMeeting(m.ID)
	mustNoErr(t, err)
	if got.RawContent != "content here" {
		t.Errorf("expected raw_content 'content here', got %q", got.RawContent)
	}

	// ListMeetings
	list, err := s.ListMeetings(nil)
	mustNoErr(t, err)
	if len(list) != 1 {
		t.Errorf("expected 1 meeting, got %d", len(list))
	}
}

// TestTask_CRUD_Filters verifies CreateTask defaults, GetTask joins, and
// ListTasks filters.
func TestTask_CRUD_Filters(t *testing.T) {
	s := openTestDB(t)

	// empty DB — nil-safe
	tasks, err := s.ListTasks(nil, "")
	mustNoErr(t, err)
	if tasks == nil {
		t.Error("ListTasks on empty DB must return non-nil slice")
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks on empty DB, got %d", len(tasks))
	}

	// create — empty status and priority should default
	task, err := s.CreateTask(nil, nil, "Fix bug", "details", "", "", "Alice", "")
	mustNoErr(t, err)
	if task.ID == 0 {
		t.Error("expected non-zero task ID")
	}
	if task.Status != "todo" {
		t.Errorf("expected default status 'todo', got %q", task.Status)
	}
	if task.Priority != "medium" {
		t.Errorf("expected default priority 'medium', got %q", task.Priority)
	}

	// GetTask
	got, err := s.GetTask(task.ID)
	mustNoErr(t, err)
	if got.Title != "Fix bug" {
		t.Errorf("expected title 'Fix bug', got %q", got.Title)
	}

	// ListTasks — no filter
	list, err := s.ListTasks(nil, "")
	mustNoErr(t, err)
	if len(list) != 1 {
		t.Errorf("expected 1 task, got %d", len(list))
	}

	// filter by status — no match
	filtered, err := s.ListTasks(nil, "done")
	mustNoErr(t, err)
	if len(filtered) != 0 {
		t.Errorf("expected 0 tasks with status 'done', got %d", len(filtered))
	}

	// create a project and link a task to it
	proj, err := s.CreateProject("Proj", "", "")
	mustNoErr(t, err)
	task2, err := s.CreateTask(nil, &proj.ID, "Proj task", "", "in_progress", "high", "", "")
	mustNoErr(t, err)

	// filter by project_id
	byProj, err := s.ListTasks(&proj.ID, "")
	mustNoErr(t, err)
	if len(byProj) != 1 {
		t.Errorf("expected 1 task for project, got %d", len(byProj))
	}
	if byProj[0].ID != task2.ID {
		t.Errorf("unexpected task ID in project filter")
	}
	if byProj[0].ProjectName != "Proj" {
		t.Errorf("expected joined ProjectName 'Proj', got %q", byProj[0].ProjectName)
	}
}

// TestTask_UpdateRecordsHistory verifies UpdateTask changes the task and
// records a history entry beyond the creation baseline.
func TestTask_UpdateRecordsHistory(t *testing.T) {
	s := openTestDB(t)

	task, err := s.CreateTask(nil, nil, "Ship it", "", "todo", "high", "Bob", "")
	mustNoErr(t, err)

	updated, err := s.UpdateTask(task.ID, map[string]any{"status": "done"}, "shipped", "Bob", nil)
	mustNoErr(t, err)
	if updated.Status != "done" {
		t.Errorf("expected status 'done' after update, got %q", updated.Status)
	}

	history, err := s.GetTaskHistory(task.ID)
	mustNoErr(t, err)
	if history == nil {
		t.Error("GetTaskHistory must return non-nil slice")
	}
	if len(history) < 2 {
		t.Errorf("expected >= 2 history entries (creation + update), got %d", len(history))
	}

	// find the update entry
	var foundUpdate bool
	for _, h := range history {
		if h.OldStatus == "todo" && h.NewStatus == "done" {
			foundUpdate = true
		}
	}
	if !foundUpdate {
		t.Error("expected a history entry with old_status='todo' and new_status='done'")
	}
}

// TestTask_HistoryBaseline verifies a freshly created task has exactly one
// history row ("Task created") before any updates.
func TestTask_HistoryBaseline(t *testing.T) {
	s := openTestDB(t)

	task, err := s.CreateTask(nil, nil, "Brand new task", "", "todo", "medium", "", "")
	mustNoErr(t, err)

	history, err := s.GetTaskHistory(task.ID)
	mustNoErr(t, err)
	if history == nil {
		t.Error("GetTaskHistory must return non-nil slice")
	}
	if len(history) != 1 {
		t.Errorf("expected exactly 1 history entry for fresh task, got %d", len(history))
	}
	if history[0].Note != "Task created" {
		t.Errorf("expected note 'Task created', got %q", history[0].Note)
	}
}

// TestMigrationIdempotency verifies that calling migrate() multiple times
// on the same DB does not fail and new columns are present.
func TestMigrationIdempotency(t *testing.T) {
	s := openTestDB(t)

	// run migrate again on the same store — addColumnIfMissing must be a no-op
	if err := s.migrate(); err != nil {
		t.Fatalf("second migrate() call failed: %v", err)
	}

	// columns must be readable
	m, err := s.CreateMeeting(nil, "f.vtt", "2026-01-01", "T", "c", "s")
	mustNoErr(t, err)
	got, err := s.GetMeeting(m.ID)
	mustNoErr(t, err)
	if got.RichContent != "" {
		t.Errorf("expected empty rich_content for new meeting, got %q", got.RichContent)
	}
	if got.ContentType != "" {
		t.Errorf("expected empty content_type for new meeting, got %q", got.ContentType)
	}
}

// TestUpdateMeetingRichContent verifies all store-layer scenarios.
func TestUpdateMeetingRichContent(t *testing.T) {
	s := openTestDB(t)

	m, err := s.CreateMeeting(nil, "f.vtt", "2026-01-01", "Title", "raw", "sum")
	mustNoErr(t, err)

	// valid markdown
	updated, err := s.UpdateMeetingRichContent(m.ID, "## Summary\nfoo", "markdown")
	mustNoErr(t, err)
	if updated.RichContent != "## Summary\nfoo" {
		t.Errorf("unexpected rich_content: %q", updated.RichContent)
	}
	if updated.ContentType != "markdown" {
		t.Errorf("unexpected content_type: %q", updated.ContentType)
	}

	// valid html
	updated2, err := s.UpdateMeetingRichContent(m.ID, "<p>ok</p>", "html")
	mustNoErr(t, err)
	if updated2.ContentType != "html" {
		t.Errorf("unexpected content_type: %q", updated2.ContentType)
	}

	// invalid content_type
	_, err = s.UpdateMeetingRichContent(m.ID, "x", "pdf")
	if err == nil {
		t.Error("expected error for invalid content_type")
	}

	// meeting not found
	_, err = s.UpdateMeetingRichContent(99999, "x", "markdown")
	if err == nil {
		t.Error("expected error for missing meeting")
	}
}

// TestFTS_Search verifies that FTS5 search returns results for both
// tasks and meetings containing a unique keyword.
func TestFTS_Search(t *testing.T) {
	s := openTestDB(t)

	// seed with a rare token
	_, err := s.CreateTask(nil, nil, "zephyrquux task", "unique keyword", "todo", "medium", "", "")
	mustNoErr(t, err)
	_, err = s.CreateMeeting(nil, "meeting.txt", "2026-01-01", "zephyrquux meeting", "content", "")
	mustNoErr(t, err)

	results, err := s.Search("zephyrquux", 10)
	mustNoErr(t, err)

	var hasTask, hasMeeting bool
	for _, r := range results {
		switch r.Kind {
		case "task":
			hasTask = true
		case "meeting":
			hasMeeting = true
		}
	}
	if !hasTask {
		t.Error("Search should return a task result for 'zephyrquux'")
	}
	if !hasMeeting {
		t.Error("Search should return a meeting result for 'zephyrquux'")
	}

	// no-match keyword must not panic
	noMatch, err := s.Search("xqzqzqzqzqzq_no_match", 10)
	mustNoErr(t, err)
	if len(noMatch) != 0 {
		t.Errorf("expected 0 results for no-match keyword, got %d", len(noMatch))
	}
}
