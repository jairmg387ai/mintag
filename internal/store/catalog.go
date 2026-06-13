package store

import (
	"bufio"
	"bytes"
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

// ListTimelogCategories returns all category names in the catalog, ordered alphabetically.
func (s *Store) ListTimelogCategories() ([]string, error) {
	return s.listCatalog("timelog_categories")
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
