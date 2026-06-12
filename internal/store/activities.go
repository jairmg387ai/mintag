package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"
)

// reDate matches YYYY-MM-DD strictly.
var reDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// DailyActivity represents a single work-log entry recorded by the user.
type DailyActivity struct {
	ID             int64   `json:"id"`
	Date           string  `json:"date"`
	Hours          float64 `json:"hours"`
	Project        string  `json:"project"`
	Category       string  `json:"category"`
	RegistroDiario string  `json:"registro_diario"`
	Source         string  `json:"source"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"created_at"`
	UploadedAt     *string `json:"uploaded_at,omitempty"`
}

// UploadResult summarises the outcome of an UploadActivities call.
type UploadResult struct {
	UploadedCount int      `json:"uploaded_count"`
	FailedIDs     []int64  `json:"failed_ids"`
	Errors        []string `json:"errors"`
}

// validateActivity checks inputs before any DB write.
func validateActivity(date string, hours float64, project, category, registroDiario, source string) error {
	if hours <= 0 {
		return fmt.Errorf("hours must be greater than 0, got %v", hours)
	}
	if !reDate.MatchString(date) {
		return fmt.Errorf("date must be in YYYY-MM-DD format, got %q", date)
	}
	if project == "" {
		return fmt.Errorf("project is required")
	}
	if category == "" {
		return fmt.Errorf("category is required")
	}
	if registroDiario == "" {
		return fmt.Errorf("registro_diario is required")
	}
	if source != "manual" && source != "llm_auto" {
		return fmt.Errorf("source must be 'manual' or 'llm_auto', got %q", source)
	}
	return nil
}

// CreateActivity inserts a new daily activity row with status="pending".
func (s *Store) CreateActivity(ctx context.Context, date string, hours float64, project, category, registroDiario, source string) (*DailyActivity, error) {
	if source == "" {
		source = "manual"
	}
	if err := validateActivity(date, hours, project, category, registroDiario, source); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO daily_activities (date, hours, project, category, registro_diario, source, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'pending', ?)`,
		date, hours, project, category, registroDiario, source, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetActivity(ctx, id)
}

// GetActivity fetches a single activity by ID. Returns an error if not found.
func (s *Store) GetActivity(ctx context.Context, id int64) (*DailyActivity, error) {
	a := &DailyActivity{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, date, hours, project, category, registro_diario, source, status, created_at, uploaded_at
		 FROM daily_activities WHERE id = ?`, id,
	).Scan(&a.ID, &a.Date, &a.Hours, &a.Project, &a.Category, &a.RegistroDiario,
		&a.Source, &a.Status, &a.CreatedAt, &a.UploadedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("activity not found: %d", id)
	}
	return a, err
}

// ListActivities returns activities matching the optional filters, ordered by created_at ASC.
func (s *Store) ListActivities(ctx context.Context, date, status string) ([]*DailyActivity, error) {
	query := `SELECT id, date, hours, project, category, registro_diario, source, status, created_at, uploaded_at
	          FROM daily_activities WHERE 1=1`
	args := []any{}
	if date != "" {
		query += " AND date = ?"
		args = append(args, date)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*DailyActivity, 0)
	for rows.Next() {
		a := &DailyActivity{}
		if err := rows.Scan(&a.ID, &a.Date, &a.Hours, &a.Project, &a.Category, &a.RegistroDiario,
			&a.Source, &a.Status, &a.CreatedAt, &a.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ApproveActivities bulk-transitions the given IDs from pending → approved.
// Only activities with status="pending" are updated. Returns the count of rows
// actually changed. IDs that are not pending are silently skipped.
func (s *Store) ApproveActivities(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var approved int
	for _, id := range ids {
		res, err := s.db.ExecContext(ctx,
			`UPDATE daily_activities SET status = 'approved' WHERE id = ? AND status = 'pending'`, id,
		)
		if err != nil {
			return approved, err
		}
		n, _ := res.RowsAffected()
		approved += int(n)
	}
	return approved, nil
}

// UnapproveActivity transitions a single activity from approved → pending.
func (s *Store) UnapproveActivity(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE daily_activities SET status = 'pending' WHERE id = ? AND status = 'approved'`, id,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		a, err := s.GetActivity(ctx, id)
		if err != nil {
			return err
		}
		return fmt.Errorf("activity %d cannot be unapproved: current status is %q (must be approved)", id, a.Status)
	}
	return nil
}

// UpdateActivity updates editable fields (hours, project, category, registro_diario)
// of a pending activity. Returns an error if the activity is not pending.
func (s *Store) UpdateActivity(ctx context.Context, id int64, hours float64, project, category, registroDiario string) (*DailyActivity, error) {
	a, err := s.GetActivity(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Status != "pending" {
		return nil, fmt.Errorf("activity %d cannot be edited: current status is %q (must be pending)", id, a.Status)
	}
	if hours > 0 {
		a.Hours = hours
	}
	if project != "" {
		a.Project = project
	}
	if category != "" {
		a.Category = category
	}
	if registroDiario != "" {
		a.RegistroDiario = registroDiario
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE daily_activities SET hours = ?, project = ?, category = ?, registro_diario = ? WHERE id = ?`,
		a.Hours, a.Project, a.Category, a.RegistroDiario, id,
	)
	if err != nil {
		return nil, err
	}
	return s.GetActivity(ctx, id)
}

// MarkUploaded transitions a single activity from approved → uploaded and sets
// uploaded_at to the current UTC time. Only the upload orchestration flow may
// call this method.
func (s *Store) MarkUploaded(ctx context.Context, id int64) error {
	a, err := s.GetActivity(ctx, id)
	if err != nil {
		return err
	}
	if a.Status != "approved" {
		return fmt.Errorf("activity %d cannot be marked uploaded: current status is %q (must be approved)", id, a.Status)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx,
		`UPDATE daily_activities SET status = 'uploaded', uploaded_at = ? WHERE id = ?`, now, id,
	)
	return err
}
