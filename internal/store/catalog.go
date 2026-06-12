package store

import (
	"bufio"
	"bytes"
	"embed"
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
