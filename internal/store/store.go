package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// --- Models ---

type Project struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	CreatedAt   time.Time `json:"created_at"`
}

type Meeting struct {
	ID          int64     `json:"id"`
	ProjectID   *int64    `json:"project_id"`
	Filename    string    `json:"filename"`
	Date        string    `json:"date"`
	Title       string    `json:"title"`
	RawContent  string    `json:"raw_content,omitempty"`
	Summary     string    `json:"summary"`
	TaskCount   int       `json:"task_count,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	RichContent string    `json:"rich_content,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
}

type Task struct {
	ID          int64     `json:"id"`
	MeetingID   *int64    `json:"meeting_id"`
	ProjectID   *int64    `json:"project_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Owner       string    `json:"owner"`
	DueDate     string    `json:"due_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// joined
	ProjectName string `json:"project_name,omitempty"`
	MeetingTitle string `json:"meeting_title,omitempty"`
}

type TaskHistory struct {
	ID              int64     `json:"id"`
	TaskID          int64     `json:"task_id"`
	SourceMeetingID *int64    `json:"source_meeting_id"`
	OldStatus       string    `json:"old_status"`
	NewStatus       string    `json:"new_status"`
	Note            string    `json:"note"`
	Author          string    `json:"author"`
	CreatedAt       time.Time `json:"created_at"`
	// joined
	SourceMeetingTitle string `json:"source_meeting_title,omitempty"`
}

type SearchResult struct {
	Kind    string `json:"kind"` // "task" | "meeting"
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Score   float64 `json:"score"`
}

// --- Open ---

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	return s, s.migrate()
}

// OpenInMemory opens a fresh in-memory SQLite store for testing.
// SetMaxOpenConns(1) is required: without it database/sql may open
// multiple connections, each getting its own empty in-memory database.
func OpenInMemory() (*Store, error) {
	db, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	return s, s.migrate()
}

func (s *Store) Close() error { return s.db.Close() }

// --- Schema ---

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS projects (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		color       TEXT NOT NULL DEFAULT '#1a6faf',
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS meetings (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id  INTEGER REFERENCES projects(id),
		filename    TEXT NOT NULL,
		date        TEXT NOT NULL DEFAULT '',
		title       TEXT NOT NULL,
		raw_content TEXT NOT NULL DEFAULT '',
		summary     TEXT NOT NULL DEFAULT '',
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		meeting_id  INTEGER REFERENCES meetings(id),
		project_id  INTEGER REFERENCES projects(id),
		title       TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'todo',
		priority    TEXT NOT NULL DEFAULT 'medium',
		owner       TEXT NOT NULL DEFAULT '',
		due_date    TEXT NOT NULL DEFAULT '',
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS task_history (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id           INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		source_meeting_id INTEGER REFERENCES meetings(id),
		old_status        TEXT NOT NULL DEFAULT '',
		new_status        TEXT NOT NULL DEFAULT '',
		note              TEXT NOT NULL DEFAULT '',
		author            TEXT NOT NULL DEFAULT '',
		created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS meetings_fts USING fts5(
		title,
		raw_content,
		summary,
		content='meetings',
		content_rowid='id',
		tokenize='unicode61'
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS tasks_fts USING fts5(
		title,
		description,
		owner,
		content='tasks',
		content_rowid='id',
		tokenize='unicode61'
	);

	CREATE TRIGGER IF NOT EXISTS meetings_ai AFTER INSERT ON meetings BEGIN
		INSERT INTO meetings_fts(rowid, title, raw_content, summary)
		VALUES (new.id, new.title, new.raw_content, new.summary);
	END;

	CREATE TRIGGER IF NOT EXISTS meetings_au AFTER UPDATE ON meetings BEGIN
		INSERT INTO meetings_fts(meetings_fts, rowid, title, raw_content, summary)
		VALUES ('delete', old.id, old.title, old.raw_content, old.summary);
		INSERT INTO meetings_fts(rowid, title, raw_content, summary)
		VALUES (new.id, new.title, new.raw_content, new.summary);
	END;

	CREATE TRIGGER IF NOT EXISTS tasks_ai AFTER INSERT ON tasks BEGIN
		INSERT INTO tasks_fts(rowid, title, description, owner)
		VALUES (new.id, new.title, new.description, new.owner);
	END;

	CREATE TRIGGER IF NOT EXISTS tasks_au AFTER UPDATE ON tasks BEGIN
		INSERT INTO tasks_fts(tasks_fts, rowid, title, description, owner)
		VALUES ('delete', old.id, old.title, old.description, old.owner);
		INSERT INTO tasks_fts(rowid, title, description, owner)
		VALUES (new.id, new.title, new.description, new.owner);
	END;
	`)
	if err != nil {
		return err
	}
	if err := s.addColumnIfMissing("meetings", "rich_content", "rich_content TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return s.addColumnIfMissing("meetings", "content_type", "content_type TEXT NOT NULL DEFAULT ''")
}

func (s *Store) addColumnIfMissing(table, col, ddl string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == col {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + ddl)
	return err
}

// --- Projects ---

func (s *Store) CreateProject(name, description, color string) (*Project, error) {
	if color == "" {
		color = "#1a6faf"
	}
	res, err := s.db.Exec(
		`INSERT INTO projects (name, description, color) VALUES (?, ?, ?)`,
		name, description, color,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetProject(id)
}

func (s *Store) GetProject(id int64) (*Project, error) {
	p := &Project{}
	err := s.db.QueryRow(
		`SELECT id, name, description, color, created_at FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Color, &p.CreatedAt)
	return p, err
}

func (s *Store) ListProjects() ([]*Project, error) {
	rows, err := s.db.Query(`SELECT id, name, description, color, created_at FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*Project, 0)
	for rows.Next() {
		p := &Project{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Color, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// --- Meetings ---

func (s *Store) CreateMeeting(projectID *int64, filename, date, title, rawContent, summary string) (*Meeting, error) {
	res, err := s.db.Exec(
		`INSERT INTO meetings (project_id, filename, date, title, raw_content, summary) VALUES (?, ?, ?, ?, ?, ?)`,
		projectID, filename, date, title, rawContent, summary,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetMeeting(id)
}

func (s *Store) GetMeeting(id int64) (*Meeting, error) {
	m := &Meeting{}
	err := s.db.QueryRow(
		`SELECT id, project_id, filename, date, title, raw_content, summary, created_at, rich_content, content_type FROM meetings WHERE id = ?`, id,
	).Scan(&m.ID, &m.ProjectID, &m.Filename, &m.Date, &m.Title, &m.RawContent, &m.Summary, &m.CreatedAt, &m.RichContent, &m.ContentType)
	return m, err
}

func (s *Store) ListMeetings(projectID *int64) ([]*Meeting, error) {
	query := `
		SELECT m.id, m.project_id, m.filename, m.date, m.title, m.summary, m.created_at,
		       COUNT(t.id) as task_count
		FROM meetings m
		LEFT JOIN tasks t ON t.meeting_id = m.id
	`
	args := []any{}
	if projectID != nil {
		query += " WHERE m.project_id = ?"
		args = append(args, *projectID)
	}
	query += " GROUP BY m.id ORDER BY m.date DESC, m.created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*Meeting, 0)
	for rows.Next() {
		m := &Meeting{}
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Filename, &m.Date, &m.Title, &m.Summary, &m.CreatedAt, &m.TaskCount); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) UpdateMeetingRichContent(id int64, content, contentType string) (*Meeting, error) {
	if contentType != "markdown" && contentType != "html" {
		return nil, fmt.Errorf("invalid content_type: must be markdown or html")
	}
	res, err := s.db.Exec(
		`UPDATE meetings SET rich_content = ?, content_type = ? WHERE id = ?`,
		content, contentType, id,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("meeting not found: %d", id)
	}
	return s.GetMeeting(id)
}

// --- Tasks ---

func (s *Store) CreateTask(meetingID, projectID *int64, title, description, status, priority, owner, dueDate string) (*Task, error) {
	if status == "" {
		status = "todo"
	}
	if priority == "" {
		priority = "medium"
	}
	res, err := s.db.Exec(
		`INSERT INTO tasks (meeting_id, project_id, title, description, status, priority, owner, due_date)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		meetingID, projectID, title, description, status, priority, owner, dueDate,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	// record creation in history
	s.db.Exec(`INSERT INTO task_history (task_id, new_status, note, author) VALUES (?, ?, ?, ?)`,
		id, status, "Task created", owner)
	return s.GetTask(id)
}

func (s *Store) GetTask(id int64) (*Task, error) {
	t := &Task{}
	err := s.db.QueryRow(`
		SELECT t.id, t.meeting_id, t.project_id, t.title, t.description, t.status,
		       t.priority, t.owner, t.due_date, t.created_at, t.updated_at,
		       COALESCE(p.name,''), COALESCE(m.title,'')
		FROM tasks t
		LEFT JOIN projects p ON p.id = t.project_id
		LEFT JOIN meetings m ON m.id = t.meeting_id
		WHERE t.id = ?`, id,
	).Scan(&t.ID, &t.MeetingID, &t.ProjectID, &t.Title, &t.Description, &t.Status,
		&t.Priority, &t.Owner, &t.DueDate, &t.CreatedAt, &t.UpdatedAt,
		&t.ProjectName, &t.MeetingTitle)
	return t, err
}

func (s *Store) ListTasks(projectID *int64, status string) ([]*Task, error) {
	query := `
		SELECT t.id, t.meeting_id, t.project_id, t.title, t.description, t.status,
		       t.priority, t.owner, t.due_date, t.created_at, t.updated_at,
		       COALESCE(p.name,''), COALESCE(m.title,'')
		FROM tasks t
		LEFT JOIN projects p ON p.id = t.project_id
		LEFT JOIN meetings m ON m.id = t.meeting_id
		WHERE 1=1
	`
	args := []any{}
	if projectID != nil {
		query += " AND t.project_id = ?"
		args = append(args, *projectID)
	}
	if status != "" {
		query += " AND t.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY CASE t.priority WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END, t.updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*Task, 0)
	for rows.Next() {
		t := &Task{}
		if err := rows.Scan(&t.ID, &t.MeetingID, &t.ProjectID, &t.Title, &t.Description, &t.Status,
			&t.Priority, &t.Owner, &t.DueDate, &t.CreatedAt, &t.UpdatedAt,
			&t.ProjectName, &t.MeetingTitle); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) UpdateTask(id int64, updates map[string]any, note, author string, sourceMeetingID *int64) (*Task, error) {
	old, err := s.GetTask(id)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	setClauses := []string{}
	args := []any{}
	for k, v := range updates {
		switch k {
		case "title", "description", "status", "priority", "owner", "due_date", "project_id":
			setClauses = append(setClauses, k+" = ?")
			args = append(args, v)
		}
	}
	if len(setClauses) == 0 && note == "" {
		return old, nil
	}

	if len(setClauses) > 0 {
		setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
		query := "UPDATE tasks SET "
		for i, c := range setClauses {
			if i > 0 {
				query += ", "
			}
			query += c
		}
		query += " WHERE id = ?"
		args = append(args, id)
		if _, err := s.db.Exec(query, args...); err != nil {
			return nil, err
		}
	}

	newStatus := old.Status
	if v, ok := updates["status"]; ok {
		newStatus = fmt.Sprintf("%v", v)
	}

	_, err = s.db.Exec(
		`INSERT INTO task_history (task_id, source_meeting_id, old_status, new_status, note, author) VALUES (?, ?, ?, ?, ?, ?)`,
		id, sourceMeetingID, old.Status, newStatus, note, author,
	)
	return s.GetTask(id)
}

func (s *Store) GetTaskHistory(taskID int64) ([]*TaskHistory, error) {
	rows, err := s.db.Query(`
		SELECT h.id, h.task_id, h.source_meeting_id, h.old_status, h.new_status,
		       h.note, h.author, h.created_at, COALESCE(m.title,'')
		FROM task_history h
		LEFT JOIN meetings m ON m.id = h.source_meeting_id
		WHERE h.task_id = ?
		ORDER BY h.created_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*TaskHistory, 0)
	for rows.Next() {
		h := &TaskHistory{}
		if err := rows.Scan(&h.ID, &h.TaskID, &h.SourceMeetingID, &h.OldStatus, &h.NewStatus,
			&h.Note, &h.Author, &h.CreatedAt, &h.SourceMeetingTitle); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// --- Search ---

func (s *Store) Search(query string, limit int) ([]*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	results := make([]*SearchResult, 0)

	// search tasks
	rows, err := s.db.Query(`
		SELECT t.id, t.title, snippet(tasks_fts, 1, '<mark>', '</mark>', '...', 20),
		       bm25(tasks_fts)
		FROM tasks_fts
		JOIN tasks t ON t.id = tasks_fts.rowid
		WHERE tasks_fts MATCH ?
		ORDER BY bm25(tasks_fts)
		LIMIT ?`, query, limit)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			r := &SearchResult{Kind: "task"}
			rows.Scan(&r.ID, &r.Title, &r.Snippet, &r.Score)
			results = append(results, r)
		}
	}

	// search meetings
	rows2, err := s.db.Query(`
		SELECT m.id, m.title, snippet(meetings_fts, 1, '<mark>', '</mark>', '...', 20),
		       bm25(meetings_fts)
		FROM meetings_fts
		JOIN meetings m ON m.id = meetings_fts.rowid
		WHERE meetings_fts MATCH ?
		ORDER BY bm25(meetings_fts)
		LIMIT ?`, query, limit)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			r := &SearchResult{Kind: "meeting"}
			rows2.Scan(&r.ID, &r.Title, &r.Snippet, &r.Score)
			results = append(results, r)
		}
	}

	return results, nil
}

// --- Stats ---

type Stats struct {
	TotalTasks    int `json:"total_tasks"`
	TodoTasks     int `json:"todo_tasks"`
	InProgressTasks int `json:"in_progress_tasks"`
	BlockedTasks  int `json:"blocked_tasks"`
	DoneTasks     int `json:"done_tasks"`
	TotalMeetings int `json:"total_meetings"`
	TotalProjects int `json:"total_projects"`
}

func (s *Store) GetStats() (*Stats, error) {
	st := &Stats{}
	s.db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&st.TotalTasks)
	s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status='todo'`).Scan(&st.TodoTasks)
	s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status='in_progress'`).Scan(&st.InProgressTasks)
	s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status='blocked'`).Scan(&st.BlockedTasks)
	s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status='done'`).Scan(&st.DoneTasks)
	s.db.QueryRow(`SELECT COUNT(*) FROM meetings`).Scan(&st.TotalMeetings)
	s.db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&st.TotalProjects)
	return st, nil
}
