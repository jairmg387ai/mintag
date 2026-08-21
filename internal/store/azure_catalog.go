package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNoDefaultAzureActivity is returned by GetDefaultAzureActivity when the
// catalog has no row with is_default=1 AND is_active=1. Callers use
// errors.Is to distinguish this expected, recoverable state from a genuine
// query failure (e.g. a dropped connection), which is returned as-is.
var ErrNoDefaultAzureActivity = errors.New("no default azure activity is configured")

// ErrAzureActivityNotFound is returned by FindAzureActivityByWorkItemID when
// no catalog entry references the given work item id. Not every work item is
// catalog-registered, so this is an expected, recoverable state — callers
// use errors.Is to distinguish it from a genuine query failure.
var ErrAzureActivityNotFound = errors.New("azure activity not found")

// AzureActivity represents a managed, labeled Azure work item that
// daily_activities records can be assigned to. Exactly one row has
// IsDefault=true at any point in time (enforced by uq_azure_activities_default
// and the transactional clear-then-set in SetDefaultAzureActivity).
//
// Project and CategoryID are the work-item-driven autofill mapping (see
// AzureActivityMapping): selecting this work item during activity
// registration autofills project/category from these fields. Both are
// optional and independent — either, neither, or both may be set.
type AzureActivity struct {
	ID           int64   `json:"id"`
	Org          string  `json:"org"`
	WorkItemID   int     `json:"work_item_id"`
	Label        string  `json:"label"`
	WorkItemType string  `json:"work_item_type"`
	IsActive     bool    `json:"is_active"`
	IsDefault    bool    `json:"is_default"`
	Project      *string `json:"project,omitempty"`
	CategoryID   *int64  `json:"category_id,omitempty"`
}

// AzureActivityMapping is the optional, independent project/category autofill
// mapping carried by an AzureActivity. Project is free text (nil or blank
// after TrimSpace normalizes to NULL); CategoryID, when non-nil, must
// reference an existing row in timelog_categories (existence-only check —
// timelog_categories has no is_active column, categories are hard-deleted via
// RemoveTimelogCategory).
type AzureActivityMapping struct {
	Project    *string
	CategoryID *int64
}

// normalizedProject trims p and returns nil for a nil or blank/whitespace-only
// value, so a blank project is stored as NULL rather than an empty string.
func normalizedProject(p *string) *string {
	if p == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*p)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// validateMapping performs an existence-only check on m.CategoryID against
// timelog_categories. There is no is_active column on that table (categories
// are hard-deleted by RemoveTimelogCategory), so — unlike the azure_activities
// is_active check used elsewhere in this file — a dangling category_id can
// only mean "never existed" or "was deleted since", both surfaced the same
// way. A nil CategoryID is always valid (no mapping).
func (s *Store) validateMapping(ctx context.Context, m AzureActivityMapping) error {
	if m.CategoryID == nil {
		return nil
	}
	var exists int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM timelog_categories WHERE id = ?`, *m.CategoryID,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("timelog category not found: %d", *m.CategoryID)
	}
	return err
}

// ListAzureActivities returns catalog entries ordered by label. When
// includeInactive is false, soft-deleted (is_active=0) rows are excluded.
//
// It opportunistically runs the automatic staleness sweep first (see
// maybeSweepCatalogs in catalog_retention.go) — there is no cron/ticker
// infrastructure in this codebase, so "sweep on read" is how retention gets
// enforced without a background job. The sweep's own error is discarded: a
// failed sweep must never break a plain catalog list read.
func (s *Store) ListAzureActivities(ctx context.Context, includeInactive bool) ([]*AzureActivity, error) {
	_ = s.maybeSweepCatalogs(ctx)

	query := `SELECT id, org, work_item_id, label, COALESCE(work_item_type, ''), is_active, is_default, project, category_id FROM azure_activities`
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
		if err := rows.Scan(&a.ID, &a.Org, &a.WorkItemID, &a.Label, &a.WorkItemType, &a.IsActive, &a.IsDefault, &a.Project, &a.CategoryID); err != nil {
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
func (s *Store) AddAzureActivity(ctx context.Context, org string, workItemID int, label string, workItemType string, m AzureActivityMapping) (*AzureActivity, error) {
	if org == "" {
		return nil, fmt.Errorf("org is required")
	}
	if workItemID <= 0 {
		return nil, fmt.Errorf("work_item_id must be greater than 0, got %d", workItemID)
	}
	if label == "" {
		return nil, fmt.Errorf("label is required")
	}
	if err := s.validateMapping(ctx, m); err != nil {
		return nil, err
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
		`INSERT INTO azure_activities (org, work_item_id, label, work_item_type, is_active, is_default, project, category_id, created_at) VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?)`,
		org, workItemID, label, strings.TrimSpace(workItemType), isDefault, normalizedProject(m.Project), m.CategoryID, time.Now().UTC().Format(time.RFC3339),
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

// UpdateAzureActivity changes the org, label, and mapping of an existing
// entry. work_item_id, is_active, and is_default are left untouched.
// This is a full replace of the mapping: an omitted project/category_id in m
// clears any previously-stored value — callers must resend current values to
// preserve them.
func (s *Store) UpdateAzureActivity(ctx context.Context, id int64, org, label string, workItemType string, m AzureActivityMapping) (*AzureActivity, error) {
	if org == "" {
		return nil, fmt.Errorf("org is required")
	}
	if label == "" {
		return nil, fmt.Errorf("label is required")
	}
	if err := s.validateMapping(ctx, m); err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE azure_activities SET org = ?, label = ?, work_item_type = ?, project = ?, category_id = ? WHERE id = ?`,
		org, label, strings.TrimSpace(workItemType), normalizedProject(m.Project), m.CategoryID, id,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("azure activity not found: %d", id)
	}
	return s.getAzureActivity(ctx, id)
}

// FindAzureActivityByWorkItemID returns the catalog entry (active or
// inactive) that references the given Azure work item id, or
// ErrAzureActivityNotFound if none does — not every work item is
// catalog-registered, so absence is expected, not an error condition.
func (s *Store) FindAzureActivityByWorkItemID(ctx context.Context, workItemID int) (*AzureActivity, error) {
	a := &AzureActivity{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, org, work_item_id, label, COALESCE(work_item_type, ''), is_active, is_default, project, category_id FROM azure_activities WHERE work_item_id = ?`, workItemID,
	).Scan(&a.ID, &a.Org, &a.WorkItemID, &a.Label, &a.WorkItemType, &a.IsActive, &a.IsDefault, &a.Project, &a.CategoryID)
	if err == sql.ErrNoRows {
		return nil, ErrAzureActivityNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ReassignAzureActivityWorkItem repoints an existing catalog entry at a new
// Azure work item id — used when a Task is recreated (closed + a new one
// created in its place) so the catalog's Project/Category mapping keeps
// pointing at a loggable, open work item instead of the now-closed original.
// Label, mapping, is_active, and is_default are all left untouched.
func (s *Store) ReassignAzureActivityWorkItem(ctx context.Context, id int64, newWorkItemID int) (*AzureActivity, error) {
	if newWorkItemID <= 0 {
		return nil, fmt.Errorf("new work_item_id must be greater than 0, got %d", newWorkItemID)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE azure_activities SET work_item_id = ? WHERE id = ?`, newWorkItemID, id,
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
		`SELECT id, org, work_item_id, label, COALESCE(work_item_type, ''), is_active, is_default, project, category_id FROM azure_activities WHERE is_default = 1 AND is_active = 1`,
	).Scan(&a.ID, &a.Org, &a.WorkItemID, &a.Label, &a.WorkItemType, &a.IsActive, &a.IsDefault, &a.Project, &a.CategoryID)
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

// TouchAzureActivityLastUsed sets last_used_at = now (UTC, RFC3339) for the
// given catalog entry, so it survives SweepStaleBugActivities for another
// full retention window. Callers are the write paths that actually put this
// entry to use — see activities.go's CreateActivity/ApproveActivities and
// SetActivityAzureActivity, which link a daily_activities row to id.
//
// A missing id is not treated as an error (mirrors
// TouchTimelogProjectLastUsed in catalog.go): the UPDATE simply affects zero
// rows, and the caller has no reason to fail an activity write over a
// catalog-touch miss.
func (s *Store) TouchAzureActivityLastUsed(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE azure_activities SET last_used_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

// SweepStaleBugActivities soft-deletes (is_active=0) Bug-type azure_activities
// entries that have gone untouched for more than retentionDays. Scoped to
// work_item_type="Bug" only (case/whitespace-insensitive) — a Task like
// "Vacaciones" is legitimately used rarely and must never be auto-deactivated
// by this sweep. The current default (is_default=1) is always exempt, same as
// DeactivateAzureActivity's manual guard above.
//
// retentionDays <= 0 means "disabled" and returns (0, nil) without querying —
// SetCatalogRetentionDays (catalog_retention.go) represents "disabled" as a
// nil *int, so callers translate that to 0 (or simply skip calling this at
// all) rather than passing a negative sentinel through.
//
// COALESCE(last_used_at, created_at) means an entry that was created but
// never explicitly "used" (linked to a daily_activities row) still ages out
// from its creation date, not forever — both columns are backfilled non-NULL
// by store.go's migrateActivities, so the COALESCE only matters for the
// theoretical case where backfill hasn't run yet.
func (s *Store) SweepStaleBugActivities(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE azure_activities
		 SET is_active = 0
		 WHERE is_active = 1
		   AND is_default = 0
		   AND LOWER(TRIM(COALESCE(work_item_type, ''))) = 'bug'
		   AND COALESCE(last_used_at, created_at) < ?`,
		cutoff,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// GetAzureActivity returns the catalog entry for id, or an error if none
// exists. Public wrapper around getAzureActivity for callers outside this
// package (e.g. internal/server's closed-work-item validation, which needs
// to resolve an azure_activity_id to its WorkItemID before linking).
func (s *Store) GetAzureActivity(ctx context.Context, id int64) (*AzureActivity, error) {
	return s.getAzureActivity(ctx, id)
}

func (s *Store) getAzureActivity(ctx context.Context, id int64) (*AzureActivity, error) {
	a := &AzureActivity{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, org, work_item_id, label, COALESCE(work_item_type, ''), is_active, is_default, project, category_id FROM azure_activities WHERE id = ?`, id,
	).Scan(&a.ID, &a.Org, &a.WorkItemID, &a.Label, &a.WorkItemType, &a.IsActive, &a.IsDefault, &a.Project, &a.CategoryID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("azure activity not found: %d", id)
	}
	return a, err
}
