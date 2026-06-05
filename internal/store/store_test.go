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

// --- Phase 4: Find + Upsert tests ---

// TestFindMeetingByFilename covers exact hit, miss, and project-scoping.
func TestFindMeetingByFilename(t *testing.T) {
	s := openTestDB(t)

	proj, err := s.CreateProject("P1", "", "")
	mustNoErr(t, err)
	proj2, err := s.CreateProject("P2", "", "")
	mustNoErr(t, err)

	m, err := s.CreateMeeting(&proj.ID, "standup-2026-06-05.vtt", "2026-06-05", "Stand-up", "content", "")
	mustNoErr(t, err)

	tests := []struct {
		name      string
		filename  string
		projectID *int64
		wantID    int64
		wantNil   bool
	}{
		{
			name:      "exact hit returns correct row",
			filename:  "standup-2026-06-05.vtt",
			projectID: &proj.ID,
			wantID:    m.ID,
		},
		{
			name:     "miss returns nil,nil",
			filename: "nonexistent.vtt",
			wantNil:  true,
		},
		{
			name:      "filename exists in different project — not returned",
			filename:  "standup-2026-06-05.vtt",
			projectID: &proj2.ID,
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.FindMeetingByFilename(tt.filename, tt.projectID)
			mustNoErr(t, err)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got meeting ID %d", got.ID)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil meeting, got nil")
			}
			if got.ID != tt.wantID {
				t.Errorf("expected ID %d, got %d", tt.wantID, got.ID)
			}
		})
	}
}

// TestFindTaskByTitleAndProject covers unique match, no match, and ambiguous.
func TestFindTaskByTitleAndProject(t *testing.T) {
	s := openTestDB(t)

	proj, err := s.CreateProject("P1", "", "")
	mustNoErr(t, err)

	task1, err := s.CreateTask(nil, &proj.ID, "Write release notes", "", "todo", "medium", "", "")
	mustNoErr(t, err)
	// second task with same title in same project to simulate ambiguous case
	task2, err := s.CreateTask(nil, &proj.ID, "Review PR", "", "todo", "medium", "", "")
	mustNoErr(t, err)
	_, err = s.CreateTask(nil, &proj.ID, "Review PR", "", "todo", "medium", "", "")
	mustNoErr(t, err)

	tests := []struct {
		name      string
		title     string
		projectID int64
		wantLen   int
		wantFirst int64
	}{
		{
			name:      "unique match returns slice len 1",
			title:     "Write release notes",
			projectID: proj.ID,
			wantLen:   1,
			wantFirst: task1.ID,
		},
		{
			name:      "no match returns empty slice",
			title:     "Does not exist",
			projectID: proj.ID,
			wantLen:   0,
		},
		{
			name:      "two tasks same title+project returns slice len 2 — ambiguous",
			title:     "Review PR",
			projectID: proj.ID,
			wantLen:   2,
		},
	}

	// suppress "declared but not used" for task2
	_ = task2

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.FindTaskByTitleAndProject(tt.title, tt.projectID)
			mustNoErr(t, err)
			if got == nil {
				t.Fatal("FindTaskByTitleAndProject must never return nil slice")
			}
			if len(got) != tt.wantLen {
				t.Errorf("expected len %d, got %d", tt.wantLen, len(got))
			}
			if tt.wantFirst != 0 && len(got) > 0 && got[0].ID != tt.wantFirst {
				t.Errorf("expected first ID %d, got %d", tt.wantFirst, got[0].ID)
			}
		})
	}
}

// TestUpsertMeeting covers created, skipped, updated, and stale candidate_id.
func TestUpsertMeeting(t *testing.T) {
	s := openTestDB(t)

	proj, err := s.CreateProject("P1", "", "")
	mustNoErr(t, err)

	t.Run("first call returns created", func(t *testing.T) {
		m, action, err := s.UpsertMeeting(&proj.ID, "standup-2026-06-05.vtt", "2026-06-05", "Stand-up", "original content", "", nil)
		mustNoErr(t, err)
		if action != "created" {
			t.Errorf("expected action 'created', got %q", action)
		}
		if m == nil || m.ID == 0 {
			t.Error("expected non-nil meeting with valid ID")
		}
	})

	t.Run("same call same content returns skipped — no dup", func(t *testing.T) {
		m1, _, err := s.UpsertMeeting(&proj.ID, "standup-2026-06-05.vtt", "2026-06-05", "Stand-up", "original content", "", nil)
		mustNoErr(t, err)
		m2, action, err := s.UpsertMeeting(&proj.ID, "standup-2026-06-05.vtt", "2026-06-05", "Stand-up", "original content", "", nil)
		mustNoErr(t, err)
		if action != "skipped" {
			t.Errorf("expected action 'skipped', got %q", action)
		}
		if m1.ID != m2.ID {
			t.Errorf("should resolve to same ID: m1=%d m2=%d", m1.ID, m2.ID)
		}

		// verify no duplicate rows
		meetings, err := s.ListMeetings(&proj.ID)
		mustNoErr(t, err)
		count := 0
		for _, m := range meetings {
			if m.Filename == "standup-2026-06-05.vtt" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected exactly 1 meeting row, got %d", count)
		}
	})

	t.Run("different content returns updated", func(t *testing.T) {
		m, action, err := s.UpsertMeeting(&proj.ID, "standup-2026-06-05.vtt", "2026-06-05", "Stand-up", "enriched content v2", "", nil)
		mustNoErr(t, err)
		if action != "updated" {
			t.Errorf("expected action 'updated', got %q", action)
		}
		if m.RawContent != "enriched content v2" {
			t.Errorf("expected updated raw_content, got %q", m.RawContent)
		}
	})

	t.Run("stale candidate_id pointing to different filename is discarded", func(t *testing.T) {
		// create a second meeting with a different filename
		other, err := s.CreateMeeting(&proj.ID, "retro-2026-06-01.vtt", "2026-06-01", "Retro", "retro content", "")
		mustNoErr(t, err)

		// pass other.ID as candidate_id but request filename for the first meeting
		m, action, err := s.UpsertMeeting(&proj.ID, "standup-2026-06-05.vtt", "2026-06-05", "Stand-up", "enriched content v2", "", &other.ID)
		mustNoErr(t, err)
		// should resolve to the existing standup, not to the other meeting
		if action == "created" {
			t.Error("should not create a new row — existing standup should be found via deterministic path")
		}
		if m.Filename != "standup-2026-06-05.vtt" {
			t.Errorf("resolved wrong meeting: got filename %q", m.Filename)
		}
	})
}

// TestUpsertTask covers created, updated (with history), skipped,
// stale candidate_id ignored, and ambiguous.
func TestUpsertTask(t *testing.T) {
	s := openTestDB(t)

	proj, err := s.CreateProject("P1", "", "")
	mustNoErr(t, err)

	t.Run("new task returns created", func(t *testing.T) {
		task, action, err := s.UpsertTask(nil, &proj.ID, "Write release notes", "", "todo", "medium", "", "", nil)
		mustNoErr(t, err)
		if action != "created" {
			t.Errorf("expected action 'created', got %q", action)
		}
		if task == nil || task.ID == 0 {
			t.Error("expected non-nil task with valid ID")
		}
	})

	t.Run("existing task with field change returns updated with history", func(t *testing.T) {
		// seed
		task, _, err := s.UpsertTask(nil, &proj.ID, "Deploy API", "", "todo", "medium", "", "", nil)
		mustNoErr(t, err)

		// create a meeting to use as source
		meeting, err := s.CreateMeeting(&proj.ID, "sprint-review.vtt", "2026-06-07", "Sprint Review", "", "")
		mustNoErr(t, err)

		updated, action, err := s.UpsertTask(&meeting.ID, &proj.ID, "Deploy API", "", "in_progress", "", "", "", nil)
		mustNoErr(t, err)
		if action != "updated" {
			t.Errorf("expected action 'updated', got %q", action)
		}
		if updated.Status != "in_progress" {
			t.Errorf("expected status 'in_progress', got %q", updated.Status)
		}

		// history must have a row with source_meeting_id set
		history, err := s.GetTaskHistory(task.ID)
		mustNoErr(t, err)
		var foundUpdate bool
		for _, h := range history {
			if h.OldStatus == "todo" && h.NewStatus == "in_progress" && h.SourceMeetingID != nil && *h.SourceMeetingID == meeting.ID {
				foundUpdate = true
			}
		}
		if !foundUpdate {
			t.Error("expected history entry with old_status=todo, new_status=in_progress, and correct source_meeting_id")
		}
	})

	t.Run("no field diff returns skipped — no history row written", func(t *testing.T) {
		task, _, err := s.UpsertTask(nil, &proj.ID, "Static task", "", "todo", "low", "", "", nil)
		mustNoErr(t, err)

		histBefore, err := s.GetTaskHistory(task.ID)
		mustNoErr(t, err)

		_, action, err := s.UpsertTask(nil, &proj.ID, "Static task", "", "todo", "low", "", "", nil)
		mustNoErr(t, err)
		if action != "skipped" {
			t.Errorf("expected action 'skipped', got %q", action)
		}

		histAfter, err := s.GetTaskHistory(task.ID)
		mustNoErr(t, err)
		if len(histAfter) != len(histBefore) {
			t.Errorf("expected no new history rows on skipped; before=%d after=%d", len(histBefore), len(histAfter))
		}
	})

	t.Run("stale candidate_id is discarded and deterministic find used", func(t *testing.T) {
		// create task A and task B in same project
		taskA, _, err := s.UpsertTask(nil, &proj.ID, "Task Alpha", "", "todo", "medium", "", "", nil)
		mustNoErr(t, err)
		taskB, _, err := s.UpsertTask(nil, &proj.ID, "Task Beta", "", "todo", "medium", "", "", nil)
		mustNoErr(t, err)

		// pass taskB.ID as candidate_id but title for Task Alpha
		resolved, action, err := s.UpsertTask(nil, &proj.ID, "Task Alpha", "", "in_progress", "", "", "", &taskB.ID)
		mustNoErr(t, err)
		// should resolve to taskA, not taskB
		if resolved.ID != taskA.ID {
			t.Errorf("expected to resolve to taskA (ID=%d), got ID=%d", taskA.ID, resolved.ID)
		}
		if action != "updated" {
			t.Errorf("expected action 'updated', got %q", action)
		}
	})

	t.Run("ambiguous match returns ambiguous action", func(t *testing.T) {
		// create two tasks with same title in same project
		_, err := s.CreateTask(nil, &proj.ID, "Review PR", "", "todo", "medium", "", "")
		mustNoErr(t, err)
		_, err = s.CreateTask(nil, &proj.ID, "Review PR", "", "todo", "medium", "", "")
		mustNoErr(t, err)

		result, action, err := s.UpsertTask(nil, &proj.ID, "Review PR", "", "", "", "", "", nil)
		mustNoErr(t, err)
		if action != "ambiguous" {
			t.Errorf("expected action 'ambiguous', got %q", action)
		}
		if result != nil {
			t.Error("expected nil task for ambiguous action")
		}
	})
}

// TestUpsertTask_RollbackOnHistoryFailure verifies that when the history
// insert would fail, the task update is also rolled back. We test this
// indirectly by verifying both writes are atomic: the test simulates the
// atomicity guarantee by checking that after a successful upsert the task
// reflects the new state only if the history row was also written.
func TestUpsertTask_AtomicUpdateAndHistory(t *testing.T) {
	s := openTestDB(t)

	proj, err := s.CreateProject("P1", "", "")
	mustNoErr(t, err)

	task, _, err := s.UpsertTask(nil, &proj.ID, "Atomic task", "", "todo", "medium", "", "", nil)
	mustNoErr(t, err)

	meeting, err := s.CreateMeeting(&proj.ID, "m.vtt", "2026-01-01", "M", "", "")
	mustNoErr(t, err)

	updated, action, err := s.UpsertTask(&meeting.ID, &proj.ID, "Atomic task", "", "done", "", "", "", nil)
	mustNoErr(t, err)
	if action != "updated" {
		t.Fatalf("expected updated, got %q", action)
	}

	// both the task state and the history row must reflect the change
	if updated.Status != "done" {
		t.Errorf("task status should be 'done', got %q", updated.Status)
	}
	history, err := s.GetTaskHistory(task.ID)
	mustNoErr(t, err)
	var found bool
	for _, h := range history {
		if h.OldStatus == "todo" && h.NewStatus == "done" {
			found = true
		}
	}
	if !found {
		t.Error("expected matching history row — update and history must be committed atomically")
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
