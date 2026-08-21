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
	{ID: "work-items", Label: "Work Items", DefaultEnabled: false},
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

// migrateMenuOptions applies the sidebar menu options first-run policy
// exactly once per database, gated by settingMenuOptionsPolicyApplied.
// preExisting reports whether the SQLite file at the configured path already
// existed on disk before the current Open() call (computed via os.Stat in
// store.go, before sql.Open() creates the connection/file). This is
// provably exhaustive — it does not enumerate which tables have rows, only
// whether this file predates the current process — so it needs no
// maintenance as new independently-writable data surfaces (tables,
// app_settings keys) are added:
//   - preExisting == false (this Open() call is the one creating the file)
//     writes only the marker, so ListMenuOptions resolves entirely through
//     DefaultEnabled;
//   - preExisting == true (the file predates this process, i.e. mintag has
//     been run against it before, regardless of what any table contains)
//     writes an explicit menu.option.<id>=1 override for every catalog
//     entry before the marker, so nothing an existing user relied on
//     disappears from the sidebar.
//
// KNOWN LIMITATION (accepted, not fixed): preExisting is derived from a
// plain os.Stat check at the top of Open() (see store.go), taken *before*
// sql.Open() causes modernc.org/sqlite to physically create the file on
// first connection (the first CREATE TABLE Exec inside migrate()). Those two
// moments are not atomic. If two mintag processes race to Open() the same
// not-yet-existing MINTAG_DB path at nearly the same instant — e.g. `mintag
// serve` and `mintag mcp` both defaulting to the same path, launched together
// — one process's migrate() can create the file before the other process's
// os.Stat runs, so the second process observes preExisting=true for what is
// genuinely a brand-new database.
//
// This is accepted rather than fixed because: (1) mintag is a single-user
// local app, so the window only matters for a self-inflicted simultaneous
// launch of two mintag processes against a database that doesn't exist yet;
// (2) the outcome even when the race is lost is still safe-direction-only —
// the losing process's fresh install ends up with all six catalog options
// enabled instead of the curated 3-option default, which is extra visible
// sidebar options, never lost or hidden data (the same safe-direction
// trade-off already accepted throughout this design, see preExisting==true
// above); (3) it is narrow — the window is only open between the os.Stat
// call and the first CREATE TABLE statement of the losing process's own
// migrate(), not the whole app lifetime.
//
// Closing this properly would require replacing the bare os.Stat with an
// OS-level exclusive-create/lock pattern around Open() — e.g. attempting an
// O_CREATE|O_EXCL sentinel file (or the SQLite file itself) to atomically
// detect "I created this" vs. "this already existed", or an advisory
// filesystem lock held across the stat-then-migrate window. That has been
// deliberately deferred rather than implemented; revisit only if mintag
// grows a real multi-process-on-first-launch use case.
func (s *Store) migrateMenuOptions(preExisting bool) error {
	ctx := context.Background()

	applied, ok, err := s.setting(ctx, settingMenuOptionsPolicyApplied)
	if err != nil {
		return err
	}
	if ok && applied == "1" {
		return nil
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
