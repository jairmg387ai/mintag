package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrNoDefaultAzureActivity is returned by GetDefaultAzureActivity when the
// catalog has no row with is_default=1 AND is_active=1. Callers use
// errors.Is to distinguish this expected, recoverable state from a genuine
// query failure (e.g. a dropped connection), which is returned as-is.
var ErrNoDefaultAzureActivity = errors.New("no default azure activity is configured")

// AzureActivity represents a managed, labeled Azure work item that
// daily_activities records can be assigned to. Exactly one row has
// IsDefault=true at any point in time (enforced by uq_azure_activities_default
// and the transactional clear-then-set in SetDefaultAzureActivity).
type AzureActivity struct {
	ID           int64  `json:"id"`
	Org          string `json:"org"`
	WorkItemID   int    `json:"work_item_id"`
	Label        string `json:"label"`
	WorkItemType string `json:"work_item_type"`
	IsActive     bool   `json:"is_active"`
	IsDefault    bool   `json:"is_default"`
}

// ListAzureActivities returns catalog entries ordered by label. When
// includeInactive is false, soft-deleted (is_active=0) rows are excluded.
func (s *Store) ListAzureActivities(ctx context.Context, includeInactive bool) ([]*AzureActivity, error) {
	query := `SELECT id, org, work_item_id, label, COALESCE(work_item_type, ''), is_active, is_default FROM azure_activities`
	if !includeInactive {
		query += ` WHERE is_active = 1`
	}
	query += ` ORDER BY label ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*AzureActivity, 0)
	for rows.Next() {
		a := &AzureActivity{}
		if err := rows.Scan(&a.ID, &a.Org, &a.WorkItemID, &a.Label, &a.WorkItemType, &a.IsActive, &a.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AddAzureActivity inserts a new catalog entry. If it is the first row in the
// table, it automatically becomes the default so the catalog always has
// exactly one default once non-empty.
//
// The count+insert is wrapped in a single transaction so two concurrent
// inserts into an empty catalog cannot both observe count==0 and both race
// to become the default — the transactional lock serializes them, so the
// second insert deterministically observes count==1 and never trips the
// uq_azure_activities_default partial unique index.
func (s *Store) AddAzureActivity(ctx context.Context, org string, workItemID int, label string, workItemType string) (*AzureActivity, error) {
	if org == "" {
		return nil, fmt.Errorf("org is required")
	}
	if workItemID <= 0 {
		return nil, fmt.Errorf("work_item_id must be greater than 0, got %d", workItemID)
	}
	if label == "" {
		return nil, fmt.Errorf("label is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM azure_activities`).Scan(&existing); err != nil {
		return nil, err
	}
	isDefault := existing == 0

	res, err := tx.ExecContext(ctx,
		`INSERT INTO azure_activities (org, work_item_id, label, work_item_type, is_active, is_default) VALUES (?, ?, ?, ?, 1, ?)`,
		org, workItemID, label, strings.TrimSpace(workItemType), isDefault,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.getAzureActivity(ctx, id)
}

// UpdateAzureActivity changes the org and label of an existing entry.
// work_item_id, is_active, and is_default are left untouched.
func (s *Store) UpdateAzureActivity(ctx context.Context, id int64, org, label string, workItemType string) (*AzureActivity, error) {
	if org == "" {
		return nil, fmt.Errorf("org is required")
	}
	if label == "" {
		return nil, fmt.Errorf("label is required")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE azure_activities SET org = ?, label = ?, work_item_type = ? WHERE id = ?`, org, label, strings.TrimSpace(workItemType), id,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("azure activity not found: %d", id)
	}
	return s.getAzureActivity(ctx, id)
}

// GetDefaultAzureActivity returns the single catalog entry with is_default=1.
// The is_active=1 filter is defense in depth: the invariant is that a default
// is always active, but this guarantees an inactive default is never returned
// even if that invariant were ever violated.
func (s *Store) GetDefaultAzureActivity(ctx context.Context) (*AzureActivity, error) {
	a := &AzureActivity{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, org, work_item_id, label, COALESCE(work_item_type, ''), is_active, is_default FROM azure_activities WHERE is_default = 1 AND is_active = 1`,
	).Scan(&a.ID, &a.Org, &a.WorkItemID, &a.Label, &a.WorkItemType, &a.IsActive, &a.IsDefault)
	if err == sql.ErrNoRows {
		return nil, ErrNoDefaultAzureActivity
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// SetDefaultAzureActivity promotes id to the sole default, clearing any
// previous default in the same transaction so no interim state can ever have
// zero or two defaults. It rejects missing or inactive targets.
func (s *Store) SetDefaultAzureActivity(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var isActive bool
	err = tx.QueryRowContext(ctx, `SELECT is_active FROM azure_activities WHERE id = ?`, id).Scan(&isActive)
	if err == sql.ErrNoRows {
		return fmt.Errorf("azure activity not found: %d", id)
	}
	if err != nil {
		return err
	}
	if !isActive {
		return fmt.Errorf("azure activity %d is inactive and cannot be set as default", id)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE azure_activities SET is_default = 0 WHERE is_default = 1`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE azure_activities SET is_default = 1 WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// DeactivateAzureActivity soft-deletes an entry (is_active=0), preserving any
// historical daily_activities.azure_activity_id references. If id is the
// current default, the operation is rejected — the caller must promote
// another activity to default first (no auto-reassignment).
//
// The read-check-write is wrapped in a single transaction (same pattern as
// SetDefaultAzureActivity) so a concurrent SetDefaultAzureActivity call
// cannot promote this row to default in between the check and the write,
// which would otherwise leave is_default=1, is_active=0 — a state the
// invariant says must never exist.
func (s *Store) DeactivateAzureActivity(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var isDefault bool
	err = tx.QueryRowContext(ctx, `SELECT is_default FROM azure_activities WHERE id = ?`, id).Scan(&isDefault)
	if err == sql.ErrNoRows {
		return fmt.Errorf("azure activity not found: %d", id)
	}
	if err != nil {
		return err
	}
	if isDefault {
		return fmt.Errorf("cannot deactivate the default activity; promote another activity to default first")
	}

	if _, err := tx.ExecContext(ctx, `UPDATE azure_activities SET is_active = 0 WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) getAzureActivity(ctx context.Context, id int64) (*AzureActivity, error) {
	a := &AzureActivity{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, org, work_item_id, label, COALESCE(work_item_type, ''), is_active, is_default FROM azure_activities WHERE id = ?`, id,
	).Scan(&a.ID, &a.Org, &a.WorkItemID, &a.Label, &a.WorkItemType, &a.IsActive, &a.IsDefault)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("azure activity not found: %d", id)
	}
	return a, err
}
