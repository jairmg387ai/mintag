package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrMenuOptionNotFound is returned by SetMenuOptionEnabled when id is not
// present in menuOptionsCatalog.
var ErrMenuOptionNotFound = errors.New("menu option not found")

// ErrLastEnabledMenuOption is returned by SetMenuOptionEnabled when the
// caller tries to disable the only currently-enabled option. The sidebar
// must always have at least one reachable section.
var ErrLastEnabledMenuOption = errors.New("cannot disable the last enabled menu option")

// settingMenuOptionsPolicyApplied marks that migrateMenuOptions has already
// run its first-run policy once for this database. Its key deliberately does
// not match the "menu.option." prefix used by per-option overrides (the
// character after "menu.option" here is "s", not "."), so it is never
// mistaken for an override by settingsTx's LIKE query.
const settingMenuOptionsPolicyApplied = "menu.options.policy_applied"

// menuOptionKeyPrefix is the app_settings key prefix for per-option
// enable/disable overrides: "menu.option.<id>".
const menuOptionKeyPrefix = "menu.option."

// MenuOption is a code-defined sidebar section. The catalog order is the
// sidebar display order. Labels here are English; the frontend keeps its own
// localized labels and matches purely by ID.
type MenuOption struct {
	ID             string
	Label          string
	DefaultEnabled bool
}

// MenuOptionStatus is the resolved (catalog + override) state of one menu
// option, safe to serialize directly to JSON for the REST API.
type MenuOptionStatus struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

// menuOptionsCatalog is the fixed set of sidebar sections this install of
// mintag can expose. Adding an option here needs no migration: absent an
// app_settings override, it simply resolves through DefaultEnabled.
var menuOptionsCatalog = []MenuOption{
	{ID: "dashboard", Label: "Dashboard", DefaultEnabled: true},
	{ID: "tasks", Label: "Tasks", DefaultEnabled: true},
	{ID: "meetings", Label: "Meetings", DefaultEnabled: false},
	{ID: "graph", Label: "Knowledge Graph", DefaultEnabled: false},
	{ID: "activities", Label: "Activities", DefaultEnabled: true},
	{ID: "deployment-windows", Label: "Deployment Windows", DefaultEnabled: false},
}

// lookupMenuOption returns the catalog entry for id, if any.
func lookupMenuOption(id string) (MenuOption, bool) {
	for _, m := range menuOptionsCatalog {
		if m.ID == id {
			return m, true
		}
	}
	return MenuOption{}, false
}

// menuOptionKey builds the app_settings key for a per-option override.
func menuOptionKey(id string) string {
	return menuOptionKeyPrefix + id
}

// queryerContext is satisfied by both *sql.DB and *sql.Tx, letting
// settingsTx run identically inside or outside an explicit transaction.
type queryerContext interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// execerContext is satisfied by both *sql.DB and *sql.Tx.
type execerContext interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// settingsTx reads every app_settings row whose key starts with prefix,
// using q (a *sql.DB for read-only callers like ListMenuOptions, or an
// in-flight *sql.Tx for read-then-write callers like SetMenuOptionEnabled).
// This is purely additive: it does not touch the existing setting()/
// setSetting() helpers in azure_config.go.
func settingsTx(ctx context.Context, q queryerContext, prefix string) (map[string]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT key, value FROM app_settings WHERE key LIKE ?`, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// setSettingTx upserts a single app_settings row using ex (a *sql.DB or an
// in-flight *sql.Tx). updated_at is formatted the same way setSetting (in
// azure_config.go) formats it, so every app_settings row shares one
// timestamp format regardless of which helper wrote it.
func setSettingTx(ctx context.Context, ex execerContext, key, value string) error {
	_, err := ex.ExecContext(ctx,
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// resolveMenuOptionEnabled applies a single override value ("1"/"0") over a
// catalog entry's DefaultEnabled. Any other stored value (garbage, manual
// edit, etc.) is ignored and falls back to DefaultEnabled.
func resolveMenuOptionEnabled(defaultEnabled bool, override string, hasOverride bool) bool {
	if !hasOverride {
		return defaultEnabled
	}
	switch override {
	case "1":
		return true
	case "0":
		return false
	default:
		return defaultEnabled
	}
}

// ListMenuOptions returns every catalog entry with its resolved Enabled
// state: an app_settings override wins when present, else DefaultEnabled.
func (s *Store) ListMenuOptions(ctx context.Context) ([]*MenuOptionStatus, error) {
	overrides, err := settingsTx(ctx, s.db, menuOptionKeyPrefix)
	if err != nil {
		return nil, err
	}
	out := make([]*MenuOptionStatus, 0, len(menuOptionsCatalog))
	for _, m := range menuOptionsCatalog {
		v, ok := overrides[menuOptionKey(m.ID)]
		out = append(out, &MenuOptionStatus{
			ID:      m.ID,
			Label:   m.Label,
			Enabled: resolveMenuOptionEnabled(m.DefaultEnabled, v, ok),
		})
	}
	return out, nil
}

// SetMenuOptionEnabled toggles a single menu option. The whole read-check-
// write is one transaction (mirroring SetDefaultAzureActivity), so a
// concurrent call can never observe or leave a state with zero enabled
// options: Open() configures busy_timeout + txlock=immediate, so SQLite
// serializes writers instead of letting two callers both see enabledCount==1.
func (s *Store) SetMenuOptionEnabled(ctx context.Context, id string, enabled bool) (*MenuOptionStatus, error) {
	opt, ok := lookupMenuOption(id)
	if !ok {
		return nil, ErrMenuOptionNotFound
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	overrides, err := settingsTx(ctx, tx, menuOptionKeyPrefix)
	if err != nil {
		return nil, err
	}

	effective := make(map[string]bool, len(menuOptionsCatalog))
	enabledCount := 0
	for _, m := range menuOptionsCatalog {
		v, ok := overrides[menuOptionKey(m.ID)]
		e := resolveMenuOptionEnabled(m.DefaultEnabled, v, ok)
		effective[m.ID] = e
		if e {
			enabledCount++
		}
	}

	current := effective[id]
	if current == enabled {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &MenuOptionStatus{ID: opt.ID, Label: opt.Label, Enabled: current}, nil
	}

	if !enabled && enabledCount == 1 {
		return nil, ErrLastEnabledMenuOption
	}

	value := "0"
	if enabled {
		value = "1"
	}
	if err := setSettingTx(ctx, tx, menuOptionKey(id), value); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &MenuOptionStatus{ID: opt.ID, Label: opt.Label, Enabled: enabled}, nil
}

// hasPreExistingData reports whether this database already has any project,
// task, meeting, daily activity, azure activity catalog, deployment window,
// or graph node rows — used by migrateMenuOptions to distinguish a genuinely
// fresh install from an upgrade of an existing one. All of projects, tasks,
// meetings, daily_activities, deployment_windows, and graph_nodes are
// independently reachable via MCP without ever touching projects/tasks/
// meetings, so each must count toward "pre-existing".
//
// azure_activities is special-cased: migrateActivities (which runs before
// migrateMenuOptions in migrate()) unconditionally seeds one "Default" row
// into azure_activities the first time the table is empty, so every
// database — fresh or not — always has at least 1 row there by the time
// this runs. Only rows beyond that always-present seed indicate the catalog
// was actually used.
func (s *Store) hasPreExistingData() (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM projects) +
			(SELECT COUNT(*) FROM tasks) +
			(SELECT COUNT(*) FROM meetings) +
			(SELECT COUNT(*) FROM daily_activities) +
			(SELECT CASE WHEN COUNT(*) > 1 THEN COUNT(*) - 1 ELSE 0 END FROM azure_activities) +
			(SELECT COUNT(*) FROM deployment_windows) +
			(SELECT COUNT(*) FROM graph_nodes)
	`).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// migrateMenuOptions applies the sidebar menu options first-run policy
// exactly once per database, gated by settingMenuOptionsPolicyApplied:
//   - a fresh DB (no projects/tasks/meetings) writes only the marker, so
//     ListMenuOptions resolves entirely through DefaultEnabled;
//   - a non-fresh DB (upgrade of an existing install) writes an explicit
//     menu.option.<id>=1 override for every catalog entry before the marker,
//     so nothing an existing user relied on disappears from the sidebar.
func (s *Store) migrateMenuOptions() error {
	ctx := context.Background()

	applied, ok, err := s.setting(ctx, settingMenuOptionsPolicyApplied)
	if err != nil {
		return err
	}
	if ok && applied == "1" {
		return nil
	}

	preExisting, err := s.hasPreExistingData()
	if err != nil {
		return err
	}
	if preExisting {
		for _, m := range menuOptionsCatalog {
			if err := s.setSetting(ctx, menuOptionKey(m.ID), "1"); err != nil {
				return err
			}
		}
	}

	return s.setSetting(ctx, settingMenuOptionsPolicyApplied, "1")
}
