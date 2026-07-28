package store

import (
	"testing"
)

func TestSeedCatalogs_Idempotent(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Seed once manually.
	if err := s.seedCatalogs(); err != nil {
		t.Fatalf("first seed: %v", err)
	}

	projects, err := s.ListTimelogProjects()
	if err != nil {
		t.Fatal(err)
	}
	initialCount := len(projects)
	if initialCount == 0 {
		t.Fatal("expected projects after seeding, got 0")
	}

	// Seed again — row count must be unchanged.
	if err := s.seedCatalogs(); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	projects2, err := s.ListTimelogProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects2) != initialCount {
		t.Errorf("expected %d projects after re-seed, got %d", initialCount, len(projects2))
	}

	categories, err := s.ListTimelogCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) == 0 {
		t.Fatal("expected categories after seeding, got 0")
	}
}

func TestListTimelogProjects_OrderedAlphabetically(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.seedCatalogs(); err != nil {
		t.Fatal(err)
	}

	projects, err := s.ListTimelogProjects()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(projects); i++ {
		if projects[i] < projects[i-1] {
			t.Errorf("projects not ordered at index %d: %q before %q", i, projects[i-1], projects[i])
		}
	}
}

func TestListTimelogCategories_OrderedAlphabetically(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.seedCatalogs(); err != nil {
		t.Fatal(err)
	}

	cats, err := s.ListTimelogCategories()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(cats); i++ {
		if cats[i].Name < cats[i-1].Name {
			t.Errorf("categories not ordered at index %d: %q before %q", i, cats[i-1].Name, cats[i].Name)
		}
	}
}
