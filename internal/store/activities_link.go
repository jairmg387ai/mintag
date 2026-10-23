package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// validateActivityLinkTx checks the catalog and guard on the same transaction
// that writes the activity. A concurrent catalog change cannot slip between
// validation and the write and leave an unlinked or partially updated row.
func validateActivityLinkTx(ctx context.Context, tx *sql.Tx, azureID int64) error {
	var active bool
	var workItemID int
	var workItemType string
	err := tx.QueryRowContext(ctx,
		`SELECT is_active, work_item_id, COALESCE(work_item_type, '') FROM azure_activities WHERE id = ?`, azureID,
	).Scan(&active, &workItemID, &workItemType)
	if err == sql.ErrNoRows {
		return fmt.Errorf("azure activity not found: %d", azureID)
	}
	if err != nil {
		return err
	}
	if !active {
		return fmt.Errorf("azure activity %d is inactive and cannot be assigned", azureID)
	}
	var guard string
	err = tx.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, settingTimeLogBugGuardEnabled).Scan(&guard)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if (err == sql.ErrNoRows || guard != "0") && strings.EqualFold(strings.TrimSpace(workItemType), "Bug") {
		return fmt.Errorf("azure activity %d points at Bug %d; log hours on the bug's child task assigned to you instead (add it to the catalog)", azureID, workItemID)
	}
	return nil
}

// CreateLinkedActivity inserts the activity and its validated Azure catalog link
// together, or leaves both unchanged if any step fails. This is the MCP path
// when azure_activity_id is supplied; the unlinked CreateActivity stays intact.
func (s *Store) CreateLinkedActivity(ctx context.Context, date string, hours float64, project, category, description, source string, azureID int64, reference string) (*DailyActivity, error) {
	if source == "" {
		source = "manual"
	}
	if err := validateActivity(date, hours, project, category, description, source); err != nil {
		return nil, err
	}
	if err := s.checkMaxHoursPerEntry(ctx, hours); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := validateActivityLinkTx(ctx, tx, azureID); err != nil {
		return nil, err
	}
	var ref any
	if strings.TrimSpace(reference) != "" {
		ref = strings.TrimSpace(reference)
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO daily_activities (date, hours, project, category, registro_diario, source, status, created_at, azure_activity_id, reference_id)
		 VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?, ?)`,
		date, hours, project, category, description, source, time.Now().UTC().Format(time.RFC3339), azureID, ref)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	_ = s.TouchTimelogProjectLastUsed(ctx, project)
	_ = s.TouchAzureActivityLastUsed(ctx, azureID)
	return s.GetActivity(ctx, id)
}

// UpdateLinkedActivity updates editable fields and their validated link as one
// unit. Empty optional fields retain the prior value, matching UpdateActivity
// and the MCP tool's existing date/reference handling.
func (s *Store) UpdateLinkedActivity(ctx context.Context, id int64, hours float64, project, category, description, date, reference string, azureID int64) (*DailyActivity, error) {
	if hours > 0 {
		if err := s.checkMaxHoursPerEntry(ctx, hours); err != nil {
			return nil, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	a := &DailyActivity{}
	err = tx.QueryRowContext(ctx,
		`SELECT date, hours, project, category, registro_diario, status, reference_id FROM daily_activities WHERE id = ?`, id,
	).Scan(&a.Date, &a.Hours, &a.Project, &a.Category, &a.RegistroDiario, &a.Status, &a.ReferenceID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("activity not found: %d", id)
	}
	if err != nil {
		return nil, err
	}
	if a.Status == "uploaded" {
		return nil, fmt.Errorf("activity %d cannot be edited: it has already been uploaded to Azure", id)
	}
	if err := validateActivityLinkTx(ctx, tx, azureID); err != nil {
		return nil, err
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
	if description != "" {
		a.RegistroDiario = description
	}
	if date != "" {
		a.Date = date
	}
	if reference != "" {
		trimmed := strings.TrimSpace(reference)
		a.ReferenceID = &trimmed
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE daily_activities SET date = ?, hours = ?, project = ?, category = ?, registro_diario = ?, reference_id = ?, azure_activity_id = ? WHERE id = ?`,
		a.Date, a.Hours, a.Project, a.Category, a.RegistroDiario, a.ReferenceID, azureID, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if project != "" {
		_ = s.TouchTimelogProjectLastUsed(ctx, a.Project)
	}
	_ = s.TouchAzureActivityLastUsed(ctx, azureID)
	return s.GetActivity(ctx, id)
}
