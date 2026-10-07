package store

import "context"

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
