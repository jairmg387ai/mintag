package store

import (
	"context"
	"strings"
	"testing"
)

func TestCreateActivity_ValidInput(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1.5, "RNCEA", "Actividades de arquitectura, diseño y código", "Revisé PRs del módulo de vehículos", "manual")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if a.Status != "pending" {
		t.Errorf("expected status=pending, got %q", a.Status)
	}
	if a.CreatedAt == "" {
		t.Error("expected non-empty created_at")
	}
	if a.UploadedAt != nil {
		t.Error("expected uploaded_at to be nil on creation")
	}
}

func TestCreateActivity_DefaultSource(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Source != "manual" {
		t.Errorf("expected source=manual, got %q", a.Source)
	}
}

func TestCreateActivity_InvalidHours(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	tests := []struct {
		name  string
		hours float64
	}{
		{"zero", 0},
		{"negative", -2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.CreateActivity(ctx, "2026-06-12", tc.hours, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
			if err == nil {
				t.Error("expected error for invalid hours, got nil")
			}
		})
	}
}

func TestCreateActivity_InvalidDate(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	_, err = s.CreateActivity(ctx, "12/06/2026", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err == nil {
		t.Error("expected error for invalid date format")
	}
}

func TestCreateActivity_EmptyRequiredFields(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	tests := []struct {
		name           string
		project        string
		category       string
		registroDiario string
	}{
		{"empty project", "", "Actividades de arquitectura, diseño y código", "Trabajo"},
		{"empty category", "RNCEA", "", "Trabajo"},
		{"empty registroDiario", "RNCEA", "Actividades de arquitectura, diseño y código", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.CreateActivity(ctx, "2026-06-12", 1.0, tc.project, tc.category, tc.registroDiario, "manual")
			if err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestListActivities_FilterByDate(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if _, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo A", "manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateActivity(ctx, "2026-06-13", 2.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo B", "manual"); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListActivities(ctx, "2026-06-12", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 result, got %d", len(list))
	}
	if list[0].Date != "2026-06-12" {
		t.Errorf("unexpected date: %q", list[0].Date)
	}
}

func TestListActivities_FilterByStatus(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a1, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo A", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateActivity(ctx, "2026-06-12", 2.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo B", "manual"); err != nil {
		t.Fatal(err)
	}

	// Approve the first one.
	if _, err := s.ApproveActivities(ctx, []int64{a1.ID}); err != nil {
		t.Fatal(err)
	}

	approved, err := s.ListActivities(ctx, "", "approved")
	if err != nil {
		t.Fatal(err)
	}
	if len(approved) != 1 {
		t.Fatalf("expected 1 approved, got %d", len(approved))
	}
	if approved[0].Status != "approved" {
		t.Errorf("expected status=approved, got %q", approved[0].Status)
	}
}

func TestListActivities_NoResults(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	list, err := s.ListActivities(ctx, "2026-01-01", "")
	if err != nil {
		t.Fatal(err)
	}
	if list == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(list) != 0 {
		t.Errorf("expected 0 results, got %d", len(list))
	}
}

func TestGetActivity_NotFound(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	_, err = s.GetActivity(ctx, 9999)
	if err == nil {
		t.Error("expected error for non-existent id")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestApproveActivities_PendingToApproved(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}

	count, err := s.ApproveActivities(ctx, []int64{a.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}

	updated, err := s.GetActivity(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "approved" {
		t.Errorf("expected status=approved, got %q", updated.Status)
	}
}

func TestApproveActivities_AlreadyApproved_ReturnsError(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveActivities(ctx, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}
	// Try to approve again — should fail.
	_, err = s.ApproveActivities(ctx, []int64{a.ID})
	if err == nil {
		t.Error("expected error when approving already-approved activity")
	}
}

func TestUnapproveActivity_ApprovedToPending(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveActivities(ctx, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.UnapproveActivity(ctx, a.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := s.GetActivity(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "pending" {
		t.Errorf("expected status=pending, got %q", updated.Status)
	}
}

func TestMarkUploaded_ApprovedToUploaded(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveActivities(ctx, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkUploaded(ctx, a.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := s.GetActivity(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "uploaded" {
		t.Errorf("expected status=uploaded, got %q", updated.Status)
	}
	if updated.UploadedAt == nil {
		t.Error("expected uploaded_at to be set")
	}
}

func TestMarkUploaded_PendingRejected(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	// Attempt to mark uploaded without approval — must fail.
	err = s.MarkUploaded(ctx, a.ID)
	if err == nil {
		t.Error("expected error when marking pending activity as uploaded")
	}
}
