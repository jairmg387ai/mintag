package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// reDate matches YYYY-MM-DD strictly.
var reDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// DailyActivity represents a single work-log entry recorded by the user.
type DailyActivity struct {
	ID              int64   `json:"id"`
	Date            string  `json:"date"`
	Hours           float64 `json:"hours"`
	Project         string  `json:"project"`
	Category        string  `json:"category"`
	RegistroDiario  string  `json:"registro_diario"`
	Source          string  `json:"source"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"created_at"`
	UploadedAt      *string `json:"uploaded_at,omitempty"`
	AzureDocumentID *string `json:"azure_document_id,omitempty"`
	AzureActivityID *int64  `json:"azure_activity_id,omitempty"`
	ReferenceID     *string `json:"reference_id,omitempty"`
}

// UploadResult summarises the outcome of an UploadActivities call.
type UploadResult struct {
	UploadedCount    int              `json:"uploaded_count"`
	FailedIDs        []int64          `json:"failed_ids"`
	Errors           []string         `json:"errors"`
	AzureDocumentIDs map[int64]string `json:"azure_document_ids,omitempty"`
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
	// Best-effort: logging an activity always refreshes the project catalog
	// entry's last_used_at (see TouchTimelogProjectLastUsed's doc comment for
	// why a project not present in the catalog is not an error here). A
	// touch failure must never fail the activity that was already created.
	_ = s.TouchTimelogProjectLastUsed(ctx, project)
	return s.GetActivity(ctx, id)
}

// GetActivity fetches a single activity by ID. Returns an error if not found.
func (s *Store) GetActivity(ctx context.Context, id int64) (*DailyActivity, error) {
	a := &DailyActivity{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, date, hours, project, category, registro_diario, source, status, created_at, uploaded_at, azure_document_id, azure_activity_id, reference_id
		 FROM daily_activities WHERE id = ?`, id,
	).Scan(&a.ID, &a.Date, &a.Hours, &a.Project, &a.Category, &a.RegistroDiario,
		&a.Source, &a.Status, &a.CreatedAt, &a.UploadedAt, &a.AzureDocumentID, &a.AzureActivityID, &a.ReferenceID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("activity not found: %d", id)
	}
	return a, err
}

// ListActivities returns activities matching the optional filters, ordered by created_at ASC.
func (s *Store) ListActivities(ctx context.Context, date, status string) ([]*DailyActivity, error) {
	query := `SELECT id, date, hours, project, category, registro_diario, source, status, created_at, uploaded_at, azure_document_id, azure_activity_id, reference_id
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

	return s.queryActivities(ctx, query, args...)
}

// ListActivitiesRange returns activities whose date falls within [from, to]
// inclusive, optionally filtered by status, ordered by date ASC then
// created_at ASC. Both from and to are required and must be in YYYY-MM-DD
// format; from must not be later than to (both checks are validation
// errors, not empty results — see the activity-export spec's "Invalid range
// is rejected" scenario).
func (s *Store) ListActivitiesRange(ctx context.Context, from, to, status string) ([]*DailyActivity, error) {
	if !reDate.MatchString(from) {
		return nil, fmt.Errorf("from must be in YYYY-MM-DD format, got %q", from)
	}
	if !reDate.MatchString(to) {
		return nil, fmt.Errorf("to must be in YYYY-MM-DD format, got %q", to)
	}
	// ISO 8601 (YYYY-MM-DD) dates compare correctly as plain strings.
	if from > to {
		return nil, fmt.Errorf("from (%q) must not be later than to (%q)", from, to)
	}

	query := `SELECT id, date, hours, project, category, registro_diario, source, status, created_at, uploaded_at, azure_document_id, azure_activity_id, reference_id
	          FROM daily_activities WHERE date >= ? AND date <= ?`
	args := []any{from, to}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY date ASC, created_at ASC"

	return s.queryActivities(ctx, query, args...)
}

// queryActivities executes a SELECT against daily_activities and scans every
// row into a DailyActivity. Shared by ListActivities and ListActivitiesRange
// so the column list and scan order stay in exactly one place.
func (s *Store) queryActivities(ctx context.Context, query string, args ...any) ([]*DailyActivity, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*DailyActivity, 0)
	for rows.Next() {
		a := &DailyActivity{}
		if err := rows.Scan(&a.ID, &a.Date, &a.Hours, &a.Project, &a.Category, &a.RegistroDiario,
			&a.Source, &a.Status, &a.CreatedAt, &a.UploadedAt, &a.AzureDocumentID, &a.AzureActivityID, &a.ReferenceID); err != nil {
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
		if n == 0 {
			continue
		}
		approved += int(n)
		// Best-effort: approving confirms the activity's project and (if
		// linked) Azure work item are genuinely in use, so both catalog
		// entries' last_used_at get refreshed the same as at creation time
		// (see CreateActivity above). GetActivity can only fail here if the
		// row vanished between the UPDATE and this read, which would be a
		// real anomaly — not worth failing an approval that already
		// committed, so it's swallowed like the touch calls themselves.
		if a, gerr := s.GetActivity(ctx, id); gerr == nil {
			_ = s.TouchTimelogProjectLastUsed(ctx, a.Project)
			if a.AzureActivityID != nil {
				_ = s.TouchAzureActivityLastUsed(ctx, *a.AzureActivityID)
			}
		}
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
// of a pending or approved activity. Returns an error if the activity has
// already been uploaded to Azure.
func (s *Store) UpdateActivity(ctx context.Context, id int64, hours float64, project, category, registroDiario string) (*DailyActivity, error) {
	a, err := s.GetActivity(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Status == "uploaded" {
		return nil, fmt.Errorf("activity %d cannot be edited: it has already been uploaded to Azure", id)
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
	if project != "" {
		// Best-effort, same as CreateActivity: editing an activity onto a
		// new project name means that project is now in use, so refresh its
		// catalog last_used_at. An unchanged (empty) project isn't touched
		// here — ApproveActivities already refreshes it independently.
		_ = s.TouchTimelogProjectLastUsed(ctx, a.Project)
	}
	return s.GetActivity(ctx, id)
}

// TimeEntryDeleter is the Azure-side boundary needed to delete uploaded
// activities. It keeps store deletion orchestration testable without HTTP code.
type TimeEntryDeleter interface {
	DeleteTimeEntry(ctx context.Context, documentID string) error
}

// NewTimeEntryDeleter creates the Azure-side delete client lazily. Delete flows
// pass a factory so pending/approved rows never require Azure configuration.
type NewTimeEntryDeleter func(context.Context) (TimeEntryDeleter, error)

// DeleteActivity removes a pending or approved activity. Uploaded activities must
// be deleted through DeleteActivityWithAzure so the remote document is removed
// before the local row.
func (s *Store) DeleteActivity(ctx context.Context, id int64) error {
	a, err := s.GetActivity(ctx, id)
	if err != nil {
		return err
	}
	if a.Status == "uploaded" {
		return fmt.Errorf("activity %d cannot be deleted: already uploaded", id)
	}
	return s.deleteActivityRow(ctx, id)
}

// DeleteActivityWithAzure deletes an activity safely. Pending/approved rows are
// local-only; uploaded rows are deleted from Azure first and the local row is
// preserved unless that remote delete succeeds or is already gone.
func (s *Store) DeleteActivityWithAzure(ctx context.Context, id int64, newDeleter NewTimeEntryDeleter) error {
	a, err := s.GetActivity(ctx, id)
	if err != nil {
		return err
	}
	if a.Status != "uploaded" {
		return s.deleteActivityRow(ctx, id)
	}
	if a.AzureDocumentID == nil || strings.TrimSpace(*a.AzureDocumentID) == "" {
		return fmt.Errorf("activity %d cannot be deleted: azure document id is required", id)
	}
	if newDeleter == nil {
		return fmt.Errorf("Azure TimeLog token is not configured")
	}
	deleter, err := newDeleter(ctx)
	if err != nil {
		return err
	}
	if deleter == nil {
		return fmt.Errorf("Azure TimeLog token is not configured")
	}
	if err := deleter.DeleteTimeEntry(ctx, *a.AzureDocumentID); err != nil {
		return fmt.Errorf("delete uploaded activity %d from Azure: %w", id, err)
	}
	return s.deleteActivityRow(ctx, id)
}

func (s *Store) deleteActivityRow(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM daily_activities WHERE id = ?`, id)
	return err
}

// MarkUploaded transitions a single activity from approved → uploaded and sets
// uploaded_at to the current UTC time. Only the upload orchestration flow may
// call this method.
func (s *Store) MarkUploaded(ctx context.Context, id int64, azureDocumentID string) error {
	azureDocumentID = strings.TrimSpace(azureDocumentID)
	a, err := s.GetActivity(ctx, id)
	if err != nil {
		return err
	}
	if a.Status != "approved" {
		return fmt.Errorf("activity %d cannot be marked uploaded: current status is %q (must be approved)", id, a.Status)
	}
	if azureDocumentID == "" {
		return fmt.Errorf("activity %d cannot be marked uploaded: azure document id is required", id)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx,
		`UPDATE daily_activities SET status = 'uploaded', uploaded_at = ?, azure_document_id = ? WHERE id = ?`, now, azureDocumentID, id,
	)
	return err
}

// SetActivityAzureActivity assigns (or clears, when azureActivityID is nil)
// the azure_activity_id FK on a daily activity. Kept separate from
// CreateActivity/UpdateActivity so their existing positional signatures
// (used across REST, MCP, and tests) do not need to widen.
//
// The column has no real SQL foreign key constraint (see
// addColumnIfMissing("daily_activities", "azure_activity_id", ...) in
// store.go), so this validates at the application level: when
// azureActivityID is non-nil, it must reference an existing, active
// (is_active=1) row in azure_activities.
func (s *Store) SetActivityAzureActivity(ctx context.Context, id int64, azureActivityID *int64) error {
	if azureActivityID != nil {
		var isActive bool
		err := s.db.QueryRowContext(ctx,
			`SELECT is_active FROM azure_activities WHERE id = ?`, *azureActivityID,
		).Scan(&isActive)
		if err == sql.ErrNoRows {
			return fmt.Errorf("azure activity not found: %d", *azureActivityID)
		}
		if err != nil {
			return err
		}
		if !isActive {
			return fmt.Errorf("azure activity %d is inactive and cannot be assigned", *azureActivityID)
		}
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE daily_activities SET azure_activity_id = ? WHERE id = ?`, azureActivityID, id,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("activity not found: %d", id)
	}
	if azureActivityID != nil {
		// Best-effort: linking an activity to a work item confirms it is
		// genuinely in use, so refresh the catalog entry's last_used_at
		// (see TouchAzureActivityLastUsed's doc comment). Never touched when
		// azureActivityID is nil (clearing the link back to "use the
		// default") — that's not a "use" of any specific catalog entry.
		_ = s.TouchAzureActivityLastUsed(ctx, *azureActivityID)
	}
	return nil
}

// SetActivityReferenceID assigns (or clears, when referenceID is nil or
// blank) the reference_id column on a daily activity — a freeform external
// bug reference (Azure work item ID, Mantis ticket ID, or LuxFlow number).
// Kept separate from CreateActivity/UpdateActivity for the same reason as
// SetActivityAzureActivity above: PATCH must distinguish "the field was
// omitted from the request" (leave it as-is) from "the field was explicitly
// cleared" (set it back to NULL), and a bare *string parameter on its own
// can't carry that distinction — callers compute "provided" separately and
// only call this method when the field was actually present in the request.
//
// Unlike SetActivityAzureActivity, there is no catalog to validate against:
// reference_id is plain text (Azure work item IDs, Mantis ticket IDs, and
// LuxFlow numbers all use different formats), so no format validation is
// applied here.
func (s *Store) SetActivityReferenceID(ctx context.Context, id int64, referenceID *string) error {
	var value any
	if referenceID != nil {
		trimmed := strings.TrimSpace(*referenceID)
		if trimmed != "" {
			value = trimmed
		}
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE daily_activities SET reference_id = ? WHERE id = ?`, value, id,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("activity not found: %d", id)
	}
	return nil
}
