package store

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

// --- Catalog listing (task 1.1) ---

func TestListMenuOptions(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string // key -> value written directly to app_settings
		wantID    string
		want      bool
	}{
		{
			name:      "no override falls back to DefaultEnabled=true",
			overrides: nil,
			wantID:    "dashboard",
			want:      true,
		},
		{
			name:      "no override falls back to DefaultEnabled=false",
			overrides: nil,
			wantID:    "meetings",
			want:      false,
		},
		{
			name:      "override disables a default-enabled option",
			overrides: map[string]string{"menu.option.dashboard": "0"},
			wantID:    "dashboard",
			want:      false,
		},
		{
			name:      "override enables a default-disabled option",
			overrides: map[string]string{"menu.option.meetings": "1"},
			wantID:    "meetings",
			want:      true,
		},
		{
			name:      "unknown override value is ignored, falls back to DefaultEnabled",
			overrides: map[string]string{"menu.option.dashboard": "banana"},
			wantID:    "dashboard",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := OpenInMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()

			ctx := context.Background()
			for k, v := range tt.overrides {
				if err := s.setSetting(ctx, k, v); err != nil {
					t.Fatal(err)
				}
			}

			opts, err := s.ListMenuOptions(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(opts) != len(menuOptionsCatalog) {
				t.Fatalf("expected %d catalog entries, got %d", len(menuOptionsCatalog), len(opts))
			}

			var found bool
			for _, o := range opts {
				if o.ID == tt.wantID {
					found = true
					if o.Enabled != tt.want {
						t.Errorf("option %q: expected enabled=%v, got %v", tt.wantID, tt.want, o.Enabled)
					}
				}
			}
			if !found {
				t.Fatalf("option %q not found in ListMenuOptions result", tt.wantID)
			}
		})
	}
}

// --- Toggle persistence and invariants (task 1.3) ---

func TestSetMenuOptionEnabled(t *testing.T) {
	t.Run("unknown id is rejected without writing any row", func(t *testing.T) {
		s, err := OpenInMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		ctx := context.Background()

		_, err = s.SetMenuOptionEnabled(ctx, "does-not-exist", false)
		if !errors.Is(err, ErrMenuOptionNotFound) {
			t.Fatalf("expected ErrMenuOptionNotFound, got %v", err)
		}

		var count int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM app_settings WHERE key = ?`, "menu.option.does-not-exist").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("expected no app_settings row for an unknown id, got %d", count)
		}
	})

	t.Run("valid toggle succeeds and persists", func(t *testing.T) {
		s, err := OpenInMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		ctx := context.Background()

		status, err := s.SetMenuOptionEnabled(ctx, "meetings", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !status.Enabled {
			t.Fatal("expected returned status to report meetings as enabled")
		}

		value, ok, err := s.setting(ctx, "menu.option.meetings")
		if err != nil {
			t.Fatal(err)
		}
		if !ok || value != "1" {
			t.Fatalf("expected menu.option.meetings=1 to persist, got ok=%v value=%q", ok, value)
		}
	})

	t.Run("re-enable a previously disabled option", func(t *testing.T) {
		s, err := OpenInMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		ctx := context.Background()

		if _, err := s.SetMenuOptionEnabled(ctx, "tasks", false); err != nil {
			t.Fatal(err)
		}
		status, err := s.SetMenuOptionEnabled(ctx, "tasks", true)
		if err != nil {
			t.Fatalf("unexpected error re-enabling: %v", err)
		}
		if !status.Enabled {
			t.Fatal("expected tasks to be re-enabled")
		}
	})

	t.Run("no-op toggle to the current state does not error", func(t *testing.T) {
		s, err := OpenInMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		ctx := context.Background()

		// dashboard is DefaultEnabled=true with no override yet.
		status, err := s.SetMenuOptionEnabled(ctx, "dashboard", true)
		if err != nil {
			t.Fatalf("unexpected error on no-op toggle: %v", err)
		}
		if !status.Enabled {
			t.Fatal("expected dashboard to remain enabled")
		}
	})

	t.Run("disabling the last enabled option is rejected and state is unchanged", func(t *testing.T) {
		s, err := OpenInMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		ctx := context.Background()

		// Default catalog starts with exactly 3 enabled: dashboard, tasks, activities.
		if _, err := s.SetMenuOptionEnabled(ctx, "dashboard", false); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SetMenuOptionEnabled(ctx, "tasks", false); err != nil {
			t.Fatal(err)
		}

		_, err = s.SetMenuOptionEnabled(ctx, "activities", false)
		if !errors.Is(err, ErrLastEnabledMenuOption) {
			t.Fatalf("expected ErrLastEnabledMenuOption, got %v", err)
		}

		opts, err := s.ListMenuOptions(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range opts {
			if o.ID == "activities" && !o.Enabled {
				t.Fatal("expected activities to remain enabled after a rejected disable")
			}
		}
	})
}

// TestSetMenuOptionEnabled_ConcurrentDisable exercises the last-enabled
// invariant under real concurrent writers. It uses a file-backed DB via
// Open() in t.TempDir() rather than OpenInMemory() (which pools a single
// connection and would serialize the goroutines before they ever reach
// SQLite, masking any real transaction race).
func TestSetMenuOptionEnabled_ConcurrentDisable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mintag.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	// Fresh DB: migrateMenuOptions leaves the catalog fallback in place, so
	// exactly 3 options are enabled by default: dashboard, tasks, activities.
	targets := []string{"dashboard", "tasks", "activities"}

	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	start := make(chan struct{})
	for i, id := range targets {
		wg.Add(1)
		go func(idx int, optionID string) {
			defer wg.Done()
			<-start
			_, errs[idx] = s.SetMenuOptionEnabled(ctx, optionID, false)
		}(i, id)
	}
	close(start)
	wg.Wait()

	var succeeded, rejected int
	for i, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrLastEnabledMenuOption):
			rejected++
		default:
			t.Fatalf("goroutine %d: unexpected error (busy_timeout should have absorbed contention): %v", i, err)
		}
	}
	if succeeded != 2 {
		t.Fatalf("expected exactly 2 successful concurrent disables, got %d", succeeded)
	}
	if rejected != 1 {
		t.Fatalf("expected exactly 1 rejection with ErrLastEnabledMenuOption, got %d", rejected)
	}

	opts, err := s.ListMenuOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	enabledCount := 0
	for _, o := range opts {
		if o.Enabled {
			enabledCount++
		}
	}
	if enabledCount != 1 {
		t.Fatalf("invariant violated: expected exactly 1 enabled option after concurrent disables, got %d", enabledCount)
	}
}

// --- First-run migration policy (task 2.1) ---

func TestMigrateMenuOptions_FreshDatabaseUsesFallbackOnly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mintag.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM app_settings WHERE key LIKE 'menu.option.%'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no menu.option.* rows written on a fresh DB, got %d", count)
	}

	marker, ok, err := s.setting(ctx, "menu.options.policy_applied")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || marker != "1" {
		t.Fatalf("expected policy_applied marker to be set to \"1\", got ok=%v value=%q", ok, marker)
	}

	opts, err := s.ListMenuOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantEnabled := map[string]bool{
		"dashboard":          true,
		"tasks":              true,
		"meetings":           false,
		"graph":              false,
		"activities":         true,
		"deployment-windows": false,
	}
	for _, o := range opts {
		if o.Enabled != wantEnabled[o.ID] {
			t.Errorf("option %q: expected enabled=%v (from DefaultEnabled fallback), got %v", o.ID, wantEnabled[o.ID], o.Enabled)
		}
	}
}

func TestMigrateMenuOptions_ExistingDatabaseEnablesAllExplicitly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mintag.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	// Simulate an install that predates this feature: data already exists,
	// but the policy marker has not been applied yet.
	if _, err := s.CreateProject("Existing Project", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM app_settings WHERE key = ?`, "menu.options.policy_applied"); err != nil {
		t.Fatal(err)
	}

	if err := s.migrateMenuOptions(); err != nil {
		t.Fatal(err)
	}

	opts, err := s.ListMenuOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) != len(menuOptionsCatalog) {
		t.Fatalf("expected %d options, got %d", len(menuOptionsCatalog), len(opts))
	}
	for _, o := range opts {
		if !o.Enabled {
			t.Errorf("expected option %q to be explicitly enabled after non-fresh migration, got disabled", o.ID)
		}
	}

	marker, ok, err := s.setting(ctx, "menu.options.policy_applied")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || marker != "1" {
		t.Fatalf("expected policy_applied marker to be set after migration, got ok=%v value=%q", ok, marker)
	}
}

// TestMigrateMenuOptions_ExistingDatabaseViaGraphNodeOnly guards against
// hasPreExistingData only counting projects/tasks/meetings: an install that
// exclusively used the knowledge graph (or activities/deployment windows,
// all reachable independently via MCP) must still be classified as a
// pre-existing install, not a fresh one.
func TestMigrateMenuOptions_ExistingDatabaseViaGraphNodeOnly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mintag.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	// Simulate an install that predates this feature and only ever touched
	// the knowledge graph: projects/tasks/meetings stay empty, but
	// graph_nodes has data.
	if _, _, err := s.UpsertGraphNode("default", "repo", "example/repo", "Example Repo", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM app_settings WHERE key = ?`, "menu.options.policy_applied"); err != nil {
		t.Fatal(err)
	}

	if err := s.migrateMenuOptions(); err != nil {
		t.Fatal(err)
	}

	opts, err := s.ListMenuOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) != len(menuOptionsCatalog) {
		t.Fatalf("expected %d options, got %d", len(menuOptionsCatalog), len(opts))
	}
	for _, o := range opts {
		if !o.Enabled {
			t.Errorf("expected option %q to be explicitly enabled (graph_nodes-only install), got disabled", o.ID)
		}
	}

	marker, ok, err := s.setting(ctx, "menu.options.policy_applied")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || marker != "1" {
		t.Fatalf("expected policy_applied marker to be set after migration, got ok=%v value=%q", ok, marker)
	}
}

func TestMigrateMenuOptions_RerunIsNoop(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mintag.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	// Explicitly disable one option so a clobbering rerun would be detectable.
	if _, err := s.SetMenuOptionEnabled(ctx, "tasks", false); err != nil {
		t.Fatal(err)
	}

	if err := s.migrate(); err != nil {
		t.Fatal(err)
	}

	opts, err := s.ListMenuOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range opts {
		if o.ID == "tasks" && o.Enabled {
			t.Fatal("expected rerunning migrate() to preserve the disabled tasks option, but it was re-enabled")
		}
	}
}

func TestMigrateMenuOptions_PolicyPersistsAfterDataDeleted(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mintag.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	// First Open() ran migrate() against a fresh (empty) DB: fallback-only,
	// no menu.option.* rows, marker set.
	p, err := s.CreateProject("Temp Project", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, p.ID); err != nil {
		t.Fatal(err)
	}

	// Rerunning migrate() after data was added then removed must not
	// re-evaluate the policy — it already applied on the first Open().
	if err := s.migrate(); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM app_settings WHERE key LIKE 'menu.option.%'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected policy to remain fallback-only after data was added then removed, got %d menu.option.* rows", count)
	}
	_ = ctx
}
