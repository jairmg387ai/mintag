package store

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
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
	return s.seedTable("timelog_categories", "seed/timelog_categories.txt")
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

// ListTimelogProjects returns all project names in the catalog, ordered alphabetically.
func (s *Store) ListTimelogProjects() ([]string, error) {
	return s.listCatalog("timelog_projects")
}

// TimelogCategory is a category row from the catalog, including its
// optional mapping to a default Azure activity (see SetCategoryAzureActivity).
type TimelogCategory struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	AzureActivityID *int64 `json:"azure_activity_id,omitempty"`
}

// ListTimelogCategories returns all categories in the catalog, ordered
// alphabetically by name, including each row's optional Azure activity
// mapping.
func (s *Store) ListTimelogCategories() ([]TimelogCategory, error) {
	rows, err := s.db.Query(`SELECT id, name, azure_activity_id FROM timelog_categories ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TimelogCategory, 0)
	for rows.Next() {
		var c TimelogCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.AzureActivityID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCategoryAzureActivity assigns (or clears, when azureActivityID is nil)
// the category's default Azure activity mapping. The column has no real SQL
// foreign key constraint (see addColumnIfMissing("timelog_categories",
// "azure_activity_id", ...) in store.go), so this validates at the
// application level, mirroring SetActivityAzureActivity: a non-nil id must
// reference an existing, active (is_active=1) row in azure_activities.
func (s *Store) SetCategoryAzureActivity(ctx context.Context, categoryID int64, azureActivityID *int64) error {
	if azureActivityID != nil {
		var isActive bool
		err := s.db.QueryRowContext(ctx,
			`SELECT is_active FROM azure_activities WHERE id = ?`, *azureActivityID,
		).Scan(&isActive)
		if err == sql.ErrNoRows {
			return fmt.Errorf("azure activity not found: %d", *azureActivityID)
		}
		if err != nil {
			return err
		}
		if !isActive {
			return fmt.Errorf("azure activity %d is inactive and cannot be assigned", *azureActivityID)
		}
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE timelog_categories SET azure_activity_id = ? WHERE id = ?`, azureActivityID, categoryID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("timelog category not found: %d", categoryID)
	}
	return nil
}

func (s *Store) listCatalog(table string) ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM ` + table + ` ORDER BY name ASC`)
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

// AddTimelogProject adds a new project to the catalog. Duplicate names are silently ignored.
func (s *Store) AddTimelogProject(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO timelog_projects(name) VALUES (?)`, name)
	return err
}

// RemoveTimelogProject deletes a project from the catalog by name.
func (s *Store) RemoveTimelogProject(name string) error {
	_, err := s.db.Exec(`DELETE FROM timelog_projects WHERE name = ?`, name)
	return err
}

// AddTimelogCategory adds a new category to the catalog. Duplicate names are silently ignored.
func (s *Store) AddTimelogCategory(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("category name cannot be empty")
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO timelog_categories(name) VALUES (?)`, name)
	return err
}

// RemoveTimelogCategory deletes a category from the catalog by name.
func (s *Store) RemoveTimelogCategory(name string) error {
	_, err := s.db.Exec(`DELETE FROM timelog_categories WHERE name = ?`, name)
	return err
}
