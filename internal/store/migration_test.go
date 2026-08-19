package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenMigratesExistingActivitiesWithoutDataLoss(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mintag.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE daily_activities (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			date            TEXT NOT NULL,
			hours           REAL NOT NULL,
			project         TEXT NOT NULL,
			category        TEXT NOT NULL,
			registro_diario TEXT NOT NULL,
			source          TEXT NOT NULL DEFAULT 'manual',
			status          TEXT NOT NULL DEFAULT 'pending',
			created_at      TEXT NOT NULL,
			uploaded_at     TEXT
		);
		INSERT INTO daily_activities (date, hours, project, category, registro_diario, source, status, created_at)
		VALUES ('2026-06-12', 1.5, 'RNCEA', 'Development', 'Existing row before migration', 'manual', 'approved', '2026-06-12T10:00:00Z');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var count int
	var status string
	if err := s.db.QueryRow(`SELECT COUNT(*), MAX(status) FROM daily_activities`).Scan(&count, &status); err != nil {
		t.Fatal(err)
	}
	if count != 1 || status != "approved" {
		t.Fatalf("expected existing approved row to survive migration, got count=%d status=%q", count, status)
	}

	if !columnExists(t, s.db, "daily_activities", "azure_document_id") {
		t.Fatal("expected daily_activities.azure_document_id column to exist")
	}
	if !columnExists(t, s.db, "daily_activities", "reference_id") {
		t.Fatal("expected daily_activities.reference_id column to exist")
	}
	var azureDocumentID sql.NullString
	if err := s.db.QueryRow(`SELECT azure_document_id FROM daily_activities LIMIT 1`).Scan(&azureDocumentID); err != nil {
		t.Fatal(err)
	}
	if azureDocumentID.Valid {
		t.Fatalf("expected migrated azure_document_id to be NULL, got %q", azureDocumentID.String)
	}

	if !tableExists(t, s.db, "app_settings") {
		t.Fatal("expected app_settings table to exist")
	}
}

// TestMigrationIdempotency_AzureActivitiesProjectAndCategoryID verifies the
// nullable azure_activities.project and azure_activities.category_id columns
// are created by migrate() and that re-running migrate() on the same store is
// a no-op (addColumnIfMissing idempotency), matching the daily_activities
// precedent. This replaces the removed
// TestMigrationIdempotency_TimelogCategoriesAzureActivityID: the mapping
// direction moved from timelog_categories onto azure_activities (see
// design D1/D2 — the category→work-item column is orphaned, not migrated).
func TestMigrationIdempotency_AzureActivitiesProjectAndCategoryID(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if !columnExists(t, s.db, "azure_activities", "project") {
		t.Fatal("expected azure_activities.project column to exist after Open")
	}
	if !columnExists(t, s.db, "azure_activities", "category_id") {
		t.Fatal("expected azure_activities.category_id column to exist after Open")
	}

	// run migrate again on the same store — addColumnIfMissing must be a no-op
	if err := s.migrate(); err != nil {
		t.Fatalf("second migrate() call failed: %v", err)
	}
	if !columnExists(t, s.db, "azure_activities", "project") {
		t.Fatal("expected azure_activities.project column to still exist after re-migrate")
	}
	if !columnExists(t, s.db, "azure_activities", "category_id") {
		t.Fatal("expected azure_activities.category_id column to still exist after re-migrate")
	}
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return name == table
}
