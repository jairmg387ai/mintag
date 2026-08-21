package store

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

const (
	settingBugRetentionDays     = "catalog.retention.bug_days"
	settingProjectRetentionDays = "catalog.retention.project_days"
	settingCatalogLastSweptAt   = "catalog.retention.last_swept_at"
)

// catalogSweepInterval bounds how often maybeSweepCatalogs actually runs
// SweepStaleCatalogEntries — ListAzureActivities and ListTimelogProjects call
// it on every read, but there is no cron/ticker infrastructure in this
// codebase (see CLAUDE.md), so the sweep itself is throttled to once per
// interval via settingCatalogLastSweptAt rather than running on every single
// list call.
const catalogSweepInterval = time.Hour

// GetCatalogRetentionDays reads the configured bug/project retention windows
// (in days). A nil result for either means that catalog's automatic
// deactivation is unset/disabled.
func (s *Store) GetCatalogRetentionDays(ctx context.Context) (bugDays *int, projectDays *int, err error) {
	bugDays, err = s.getRetentionSetting(ctx, settingBugRetentionDays)
	if err != nil {
		return nil, nil, err
	}
	projectDays, err = s.getRetentionSetting(ctx, settingProjectRetentionDays)
	if err != nil {
		return nil, nil, err
	}
	return bugDays, projectDays, nil
}

func (s *Store) getRetentionSetting(ctx context.Context, key string) (*int, error) {
	raw, ok, err := s.setting(ctx, key)
	if err != nil {
		return nil, err
	}
	if !ok || raw == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("stored retention setting %q is not an integer: %q", key, raw)
	}
	return &n, nil
}

// SetCatalogRetentionDays configures the automatic catalog staleness sweep.
// A nil pointer clears (disables) that catalog's retention; a non-nil
// pointer must be > 0 — 0 or negative is rejected rather than silently
// treated as "disabled" (use nil for that).
func (s *Store) SetCatalogRetentionDays(ctx context.Context, bugDays, projectDays *int) error {
	if err := s.setRetentionSetting(ctx, settingBugRetentionDays, bugDays); err != nil {
		return err
	}
	return s.setRetentionSetting(ctx, settingProjectRetentionDays, projectDays)
}

func (s *Store) setRetentionSetting(ctx context.Context, key string, days *int) error {
	if days == nil {
		return s.deleteSettings(ctx, key)
	}
	if *days <= 0 {
		return fmt.Errorf("retention days must be greater than 0, got %d", *days)
	}
	return s.setSetting(ctx, key, strconv.Itoa(*days))
}

// SweepStaleCatalogEntries deactivates catalog entries older than the
// configured retention windows: Bug-type azure_activities via
// SweepStaleBugActivities, and timelog_projects via
// SweepStaleTimelogProjects. An unset (nil) window is a no-op for that
// catalog — both unset is a full no-op except for the last-swept timestamp.
//
// settingCatalogLastSweptAt is always updated, even when both windows are
// unset, so maybeSweepCatalogs' hourly gate advances regardless of whether
// retention is actually configured (otherwise every single catalog read
// while retention is disabled would re-check settings on every call).
func (s *Store) SweepStaleCatalogEntries(ctx context.Context) (bugDeactivated, projectDeactivated int64, err error) {
	bugDays, projectDays, err := s.GetCatalogRetentionDays(ctx)
	if err != nil {
		return 0, 0, err
	}
	if bugDays != nil {
		bugDeactivated, err = s.SweepStaleBugActivities(ctx, *bugDays)
		if err != nil {
			return bugDeactivated, projectDeactivated, err
		}
	}
	if projectDays != nil {
		projectDeactivated, err = s.SweepStaleTimelogProjects(ctx, *projectDays)
		if err != nil {
			return bugDeactivated, projectDeactivated, err
		}
	}
	if err := s.setSetting(ctx, settingCatalogLastSweptAt, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return bugDeactivated, projectDeactivated, err
	}
	return bugDeactivated, projectDeactivated, nil
}

// maybeSweepCatalogs runs SweepStaleCatalogEntries when
// settingCatalogLastSweptAt is unset or older than catalogSweepInterval,
// otherwise it is a no-op. Called from ListAzureActivities and
// ListTimelogProjects so retention gets enforced on read without any
// cron/ticker infrastructure (none exists in this codebase).
//
// The sweep's own error is swallowed into this function's return so a caller
// that discards it (both current callers do, via `_ = s.maybeSweepCatalogs(ctx)`)
// never has a failed sweep break a plain catalog list read. A genuine failure
// reading settingCatalogLastSweptAt itself is still returned, in case a
// future caller wants to distinguish "sweep declined to run due to a read
// error" from "sweep ran and failed".
func (s *Store) maybeSweepCatalogs(ctx context.Context) error {
	last, ok, err := s.setting(ctx, settingCatalogLastSweptAt)
	if err != nil {
		return err
	}
	if ok && last != "" {
		if t, perr := time.Parse(time.RFC3339, last); perr == nil && time.Now().UTC().Sub(t) < catalogSweepInterval {
			return nil
		}
	}
	_, _, sweepErr := s.SweepStaleCatalogEntries(ctx)
	return sweepErr
}
