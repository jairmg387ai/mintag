package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// settingTimeLogBugGuardEnabled toggles the rule that TimeLog hours must never
// land on an Azure DevOps Bug: hours for a bug go to one of its child Tasks
// assigned to the caller instead (see UploadActivities and
// azure.Client.CheckTimeLogTarget). Stored with the same "1"/"0" encoding as
// the other activity.validation.* toggles.
const settingTimeLogBugGuardEnabled = "activity.validation.block_bug_work_item"

// TimeLogBugGuardEnabled reports whether the bug guard is on. Unlike the
// ActivityValidationSettings toggles (which default off to stay
// non-breaking), this guard encodes a mandatory team process, so an absent
// key resolves to true (on). Only an explicit "0" turns it off — any other
// stored value, including a corrupted one, keeps the guard on rather than
// silently disabling it.
func (s *Store) TimeLogBugGuardEnabled(ctx context.Context) (bool, error) {
	raw, ok, err := s.setting(ctx, settingTimeLogBugGuardEnabled)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	return raw != "0", nil
}

// SetTimeLogBugGuardEnabled persists the bug guard toggle.
func (s *Store) SetTimeLogBugGuardEnabled(ctx context.Context, enabled bool) error {
	return s.setSettingBool(ctx, settingTimeLogBugGuardEnabled, enabled)
}

// RejectBugAzureActivity is the registration-time half of the TimeLog bug
// guard: with the guard on, it rejects linking an activity to a catalog entry
// whose cached work_item_type is "Bug" (case-insensitive). It reads only the
// local catalog — no Azure call — so an entry whose type was never recorded
// (empty/unknown) is allowed here and left to the authoritative upload-time
// check in UploadActivities. A missing catalog row is not this check's
// concern either (SetActivityAzureActivity's FK validation reports it), so it
// is also allowed.
//
// Exported so handlers can run it before creating a row (see
// handleCreateActivity) instead of leaving an orphan activity behind when the
// link is rejected; SetActivityAzureActivity runs it again for every caller.
func (s *Store) RejectBugAzureActivity(ctx context.Context, azureActivityID int64) error {
	enabled, err := s.TimeLogBugGuardEnabled(ctx)
	if err != nil || !enabled {
		return err
	}
	var workItemID int
	var workItemType string
	err = s.db.QueryRowContext(ctx,
		`SELECT work_item_id, COALESCE(work_item_type, '') FROM azure_activities WHERE id = ?`, azureActivityID,
	).Scan(&workItemID, &workItemType)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(workItemType), "Bug") {
		return nil
	}
	return fmt.Errorf("azure activity %d points at Bug %d; log hours on the bug's child task assigned to you instead (add it to the catalog)", azureActivityID, workItemID)
}
