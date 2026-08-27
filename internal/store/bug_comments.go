package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// BeginBugCommentUpload starts (or resumes) a local idempotency row for one
// outbound Azure Bug comment, mirroring MarkUploaded's approved->uploaded
// idempotency pattern (see upload.go) for comment posting instead of TimeLog
// entries.
//
//   - A fresh idempotency_key inserts a new 'pending' row and returns its id,
//     alreadyPosted=false, existingAzureCommentID=0.
//   - Replaying a key whose row already reached status='posted' short-circuits:
//     no insert happens (idempotency_key is UNIQUE, so a duplicate insert
//     would fail anyway), and the caller gets back the ALREADY-POSTED result
//     (alreadyPosted=true, the stored azure_comment_id) without contacting
//     Azure again.
//   - Replaying a key whose row is still 'pending' or 'failed' reuses that
//     same row (alreadyPosted=false, existingAzureCommentID=0) instead of
//     erroring or inserting a duplicate, so a caller can retry the exact same
//     upload attempt (call AddBugComment again, then MarkBugCommentPosted/
//     MarkBugCommentFailed on this same id).
func (s *Store) BeginBugCommentUpload(ctx context.Context, workItemID int, idempotencyKey, body string) (int64, bool, int64, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return 0, false, 0, fmt.Errorf("bug comment upload: idempotency_key is required")
	}

	var (
		existingID      int64
		existingStatus  string
		existingAzureID sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, status, azure_comment_id FROM azure_bug_comment_uploads WHERE idempotency_key = ?`, idempotencyKey,
	).Scan(&existingID, &existingStatus, &existingAzureID)
	switch {
	case err == nil:
		if existingStatus == "posted" {
			return existingID, true, existingAzureID.Int64, nil
		}
		// pending or failed: the same idempotency_key was submitted before Azure
		// confirmed success — reuse the same row (the column is UNIQUE, so a
		// second insert is not an option) so the caller retries this exact
		// upload attempt.
		return existingID, false, 0, nil
	case errors.Is(err, sql.ErrNoRows):
		// fall through to insert a fresh row
	default:
		return 0, false, 0, fmt.Errorf("bug comment upload: lookup idempotency key: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO azure_bug_comment_uploads (work_item_id, idempotency_key, body, status, created_at) VALUES (?, ?, ?, 'pending', ?)`,
		workItemID, idempotencyKey, body, now,
	)
	if err != nil {
		return 0, false, 0, fmt.Errorf("bug comment upload: insert: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, false, 0, fmt.Errorf("bug comment upload: get inserted id: %w", err)
	}
	return id, false, 0, nil
}

// MarkBugCommentPosted transitions a bug-comment upload row to status=
// 'posted', stamping posted_at and the confirmed Azure comment id. A 2xx
// response without a confirmed non-zero Azure comment id must never reach
// this call — see the design's mirroring of MarkUploaded's idempotency
// contract in upload.go.
func (s *Store) MarkBugCommentPosted(ctx context.Context, id int64, azureCommentID int64) error {
	if azureCommentID == 0 {
		return fmt.Errorf("bug comment upload %d cannot be marked posted: azure comment id is required", id)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE azure_bug_comment_uploads SET status = 'posted', posted_at = ?, azure_comment_id = ? WHERE id = ?`,
		now, azureCommentID, id,
	)
	if err != nil {
		return fmt.Errorf("bug comment upload %d: mark posted: %w", id, err)
	}
	return rowsAffectedOrNotFound(res, id, "bug comment upload")
}

// MarkBugCommentFailed transitions a bug-comment upload row to status=
// 'failed' and records the error. The row is never deleted on failure — it
// stays queryable for diagnostics/retry, mirroring how a failed TimeLog
// upload leaves the source activity row alone (see UploadActivities).
func (s *Store) MarkBugCommentFailed(ctx context.Context, id int64, errMsg string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE azure_bug_comment_uploads SET status = 'failed', last_error = ? WHERE id = ?`,
		errMsg, id,
	)
	if err != nil {
		return fmt.Errorf("bug comment upload %d: mark failed: %w", id, err)
	}
	return rowsAffectedOrNotFound(res, id, "bug comment upload")
}

// rowsAffectedOrNotFound turns a zero-rows-affected UPDATE result into a
// "not found" error, shared by MarkBugCommentPosted/MarkBugCommentFailed so
// an unknown id is reported instead of silently succeeding.
func rowsAffectedOrNotFound(res sql.Result, id int64, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s %d: %w", what, id, err)
	}
	if n == 0 {
		return fmt.Errorf("%s %d not found", what, id)
	}
	return nil
}
