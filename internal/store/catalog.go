package store

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"fmt"
	"strings"
	"time"
)

//go:embed seed/timelog_projects.txt seed/timelog_categories.txt
var catalogFS embed.FS

// seedCatalogs populates timelog_projects and timelog_categories from the
// embedded seed files using INSERT OR IGNORE, so it is safe to call on every
// Open() — already-present rows are silently skipped.
func (s *Store) seedCatalogs() error {
	if err := s.seedTable("timelog_projects", "seed/timelog_projects.txt"); err != nil {
		return err
	}
	return s.seedCategoriesTable("seed/timelog_categories.txt")
}

func (s *Store) seedTable(table, file string) error {
	data, err := catalogFS.ReadFile(file)
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if _, err := s.db.Exec(
			`INSERT OR IGNORE INTO `+table+`(name) VALUES (?)`, line,
		); err != nil {
			return err
		}
	}
	return sc.Err()
}

// seedCategoriesTable seeds timelog_categories from a "Name|Description"
// per-line file (description may be empty). Like seedTable, this uses
// INSERT OR IGNORE so it is safe to call on every Open() — already-present
// rows (matched by name) are never touched or removed, only new inserts
// happen.
func (s *Store) seedCategoriesTable(file string) error {
	data, err := catalogFS.ReadFile(file)
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		name, description, _ := strings.Cut(line, "|")
		name = strings.TrimSpace(name)
		description = strings.TrimSpace(description)
		if name == "" {
			continue
		}
		if _, err := s.db.Exec(
			`INSERT OR IGNORE INTO timelog_categories(name, description) VALUES (?, ?)`, name, description,
		); err != nil {
			return err
		}
	}
	return sc.Err()
}

// ListTimelogProjects returns project names in the catalog, ordered
// alphabetically. Soft-deleted (is_active=0) entries are excluded unless
// includeInactive is true.
//
// It opportunistically runs the automatic staleness sweep first (see
// maybeSweepCatalogs in catalog_retention.go) — there is no cron/ticker
// infrastructure in this codebase, so "sweep on read" is how retention gets
// enforced without a background job. The sweep's own error is discarded: a
// failed sweep must never break a plain catalog list read.
func (s *Store) ListTimelogProjects(ctx context.Context, includeInactive bool) ([]string, error) {
	_ = s.maybeSweepCatalogs(ctx)

	query := `SELECT name FROM timelog_projects`
	if !includeInactive {
		query += ` WHERE is_active = 1`
	}
	query += ` ORDER BY name ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// TimelogCategory is a category row from the catalog.
//
// The former azure_activity_id mapping field (category -> default Azure
// activity) was removed: the mapping direction inverted onto
// azure_activities (project + category_id, see AzureActivityMapping in
// azure_catalog.go) so there is exactly one editable mapping direction.
type TimelogCategory struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListTimelogCategories returns all categories in the catalog, ordered
// alphabetically by name.
func (s *Store) ListTimelogCategories() ([]TimelogCategory, error) {
	rows, err := s.db.Query(`SELECT id, name, COALESCE(description, '') FROM timelog_categories ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TimelogCategory, 0)
	for rows.Next() {
		var c TimelogCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Description); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddTimelogProject adds a new project to the catalog. Duplicate names are silently ignored.
func (s *Store) AddTimelogProject(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO timelog_projects(name, created_at) VALUES (?, ?)`,
		name, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// DeactivateTimelogProject soft-deletes a project from the catalog by name
// (is_active=0), preserving any historical daily_activities.project string
// references — project is plain free TEXT, not a FK (see this file's package
// doc / TimelogCategory comment), so nothing needs to be re-pointed. Replaces
// the former RemoveTimelogProject hard delete. Returns a "not found"-style
// error when name isn't in the catalog (RowsAffected==0), mirroring
// DeactivateAzureActivity's not-found style in azure_catalog.go.
func (s *Store) DeactivateTimelogProject(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE timelog_projects SET is_active = 0 WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("timelog project not found: %q", name)
	}
	return nil
}

// TouchTimelogProjectLastUsed sets last_used_at = now (UTC, RFC3339) for the
// catalog entry matching name, so it survives SweepStaleTimelogProjects for
// another full retention window. daily_activities.project is plain free TEXT
// (not a FK to timelog_projects.name), so an activity may legitimately
// reference a project name that was never added to the catalog — that's not
// an error, it simply touches zero rows.
func (s *Store) TouchTimelogProjectLastUsed(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE timelog_projects SET last_used_at = ? WHERE name = ?`,
		time.Now().UTC().Format(time.RFC3339), name,
	)
	return err
}

// SweepStaleTimelogProjects soft-deletes (is_active=0) timelog_projects
// entries that have gone untouched for more than retentionDays. Unlike
// SweepStaleBugActivities in azure_catalog.go, timelog_projects has neither
// an is_default nor a work_item_type concept, so there is no equivalent
// exemption guard here.
//
// retentionDays <= 0 means "disabled" and returns (0, nil) without querying.
func (s *Store) SweepStaleTimelogProjects(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE timelog_projects
		 SET is_active = 0
		 WHERE is_active = 1
		   AND COALESCE(last_used_at, created_at) < ?`,
		cutoff,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// AddTimelogCategory adds a new category to the catalog. Duplicate names are silently ignored.
func (s *Store) AddTimelogCategory(name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("category name cannot be empty")
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO timelog_categories(name, description) VALUES (?, ?)`, name, strings.TrimSpace(description))
	return err
}

// RemoveTimelogCategory deletes a category from the catalog by name.
func (s *Store) RemoveTimelogCategory(name string) error {
	_, err := s.db.Exec(`DELETE FROM timelog_categories WHERE name = ?`, name)
	return err
}

// UpdateTimelogCategoryDescription updates an existing category's
// description and returns the updated row. RowsAffected==0 means the id
// doesn't exist.
func (s *Store) UpdateTimelogCategoryDescription(ctx context.Context, id int64, description string) (*TimelogCategory, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE timelog_categories SET description = ? WHERE id = ?`, strings.TrimSpace(description), id,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("timelog category not found: %d", id)
	}

	var c TimelogCategory
	err = s.db.QueryRowContext(ctx,
		`SELECT id, name, COALESCE(description, '') FROM timelog_categories WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.Description)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
