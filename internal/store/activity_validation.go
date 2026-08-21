package store

import "context"

const (
	settingMaxHoursPerEntry    = "activity.validation.max_hours_per_entry"
	settingWeekendConfirm      = "activity.validation.weekend_confirm"
	settingBlockClosedWorkItem = "activity.validation.block_closed_work_item"
)

// MaxHoursPerActivityEntry is the fixed per-entry hour cap enforced by
// CreateActivity/UpdateActivity when ActivityValidationSettings.MaxHoursPerEntry
// is enabled. The cap itself is not configurable — only whether it is
// enforced is — matching the original request's wording ("configure those
// validations to turn on or off", not "configure the threshold").
const MaxHoursPerActivityEntry = 8.0

// ActivityValidationSettings are the three independently-toggleable activity
// logging guards ported from the coworker's app: a per-entry hour cap, a
// non-business-day confirmation prompt (enforced client-side only — see
// NewActivityModal), and a block on linking hours to a Closed/Cerrado Azure
// work item (enforced server-side — see applyAzureActivityID in
// internal/server/activities.go). All three default to false (off) so this
// feature is entirely non-breaking until a user opts in.
type ActivityValidationSettings struct {
	MaxHoursPerEntry    bool `json:"max_hours_per_entry"`
	WeekendConfirm      bool `json:"weekend_confirm"`
	BlockClosedWorkItem bool `json:"block_closed_work_item"`
}

// GetActivityValidationSettings reads the three toggles. An absent key
// resolves to false (off) — this makes a fresh install, and any install that
// predates this feature, behave exactly as before with no migration needed.
func (s *Store) GetActivityValidationSettings(ctx context.Context) (*ActivityValidationSettings, error) {
	maxHours, err := s.settingBool(ctx, settingMaxHoursPerEntry)
	if err != nil {
		return nil, err
	}
	weekend, err := s.settingBool(ctx, settingWeekendConfirm)
	if err != nil {
		return nil, err
	}
	closedWI, err := s.settingBool(ctx, settingBlockClosedWorkItem)
	if err != nil {
		return nil, err
	}
	return &ActivityValidationSettings{
		MaxHoursPerEntry:    maxHours,
		WeekendConfirm:      weekend,
		BlockClosedWorkItem: closedWI,
	}, nil
}

// SetActivityValidationSettings writes all three toggles in one call — the
// settings form always submits the complete set (see
// ActivityValidationSection.tsx), so there is no partial-update case to
// support here, unlike SetCatalogRetentionDays's independent nil-clears-one
// semantics.
func (s *Store) SetActivityValidationSettings(ctx context.Context, v ActivityValidationSettings) error {
	if err := s.setSettingBool(ctx, settingMaxHoursPerEntry, v.MaxHoursPerEntry); err != nil {
		return err
	}
	if err := s.setSettingBool(ctx, settingWeekendConfirm, v.WeekendConfirm); err != nil {
		return err
	}
	return s.setSettingBool(ctx, settingBlockClosedWorkItem, v.BlockClosedWorkItem)
}

func (s *Store) settingBool(ctx context.Context, key string) (bool, error) {
	raw, ok, err := s.setting(ctx, key)
	if err != nil {
		return false, err
	}
	return ok && raw == "1", nil
}

func (s *Store) setSettingBool(ctx context.Context, key string, value bool) error {
	v := "0"
	if value {
		v = "1"
	}
	return s.setSetting(ctx, key, v)
}
