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
	// Try to approve again — already approved, should be skipped (0 rows changed, no error).
	n, err := s.ApproveActivities(ctx, []int64{a.ID})
	if err != nil {
		t.Errorf("expected no error when approving already-approved activity, got: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 approved (skipped), got %d", n)
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

func TestUpdateActivity_PendingSucceeds(t *testing.T) {
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

	updated, err := s.UpdateActivity(ctx, a.ID, 2.5, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo actualizado")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Hours != 2.5 {
		t.Errorf("expected hours=2.5, got %v", updated.Hours)
	}
	if updated.RegistroDiario != "Trabajo actualizado" {
		t.Errorf("expected updated registro_diario, got %q", updated.RegistroDiario)
	}
}

func TestUpdateActivity_ApprovedSucceeds(t *testing.T) {
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

	updated, err := s.UpdateActivity(ctx, a.ID, 3.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo aprobado editado")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != "approved" {
		t.Errorf("expected status to remain approved, got %q", updated.Status)
	}
	if updated.Hours != 3.0 {
		t.Errorf("expected hours=3.0, got %v", updated.Hours)
	}
}

func TestUpdateActivity_UploadedRejected(t *testing.T) {
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
	if err := s.MarkUploaded(ctx, a.ID, "azure-doc-123"); err != nil {
		t.Fatal(err)
	}

	_, err = s.UpdateActivity(ctx, a.ID, 5.0, "RNCEA", "Actividades de arquitectura, diseño y código", "Intento de edición")
	if err == nil {
		t.Fatal("expected error when editing an uploaded activity")
	}
	if !strings.Contains(err.Error(), "already been uploaded") {
		t.Errorf("expected 'already been uploaded' in error, got: %v", err)
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
	if err := s.MarkUploaded(ctx, a.ID, "azure-doc-123"); err != nil {
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
	if updated.AzureDocumentID == nil || *updated.AzureDocumentID != "azure-doc-123" {
		t.Fatalf("expected azure_document_id to be stored, got %v", updated.AzureDocumentID)
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
	err = s.MarkUploaded(ctx, a.ID, "azure-doc-123")
	if err == nil {
		t.Error("expected error when marking pending activity as uploaded")
	}
}

func TestMarkUploaded_WhitespaceDocumentIDRejected(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveActivities(ctx, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}

	err = s.MarkUploaded(ctx, a.ID, "   ")
	if err == nil {
		t.Fatal("expected whitespace azure document id to be rejected")
	}
	updated, err := s.GetActivity(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "approved" {
		t.Fatalf("expected activity to remain approved, got %q", updated.Status)
	}
}

func TestDeleteActivityWithAzure_PendingDeletesLocallyWithoutAzure(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteActivityWithAzure(ctx, a.ID, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := s.GetActivity(ctx, a.ID); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected local row to be deleted, got %v", err)
	}
}

func TestDeleteActivityWithAzure_UploadedSuccessDeletesRemoteBeforeLocal(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a := createUploadedActivity(t, s, ctx, "azure-doc-123")
	deleter := &fakeTimeEntryDeleter{}

	if err := s.DeleteActivityWithAzure(ctx, a.ID, func(context.Context) (TimeEntryDeleter, error) {
		return deleter, nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleter.documentID != "azure-doc-123" {
		t.Fatalf("expected remote delete for azure-doc-123, got %q", deleter.documentID)
	}
	if _, err := s.GetActivity(ctx, a.ID); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected local row to be deleted, got %v", err)
	}
}

func TestDeleteActivityWithAzure_UploadedRemoteFailurePreservesLocalRow(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a := createUploadedActivity(t, s, ctx, "azure-doc-123")
	deleter := &fakeTimeEntryDeleter{err: context.DeadlineExceeded}

	if err := s.DeleteActivityWithAzure(ctx, a.ID, func(context.Context) (TimeEntryDeleter, error) {
		return deleter, nil
	}); err == nil || !strings.Contains(err.Error(), "delete uploaded activity") {
		t.Fatalf("expected remote delete error, got %v", err)
	}
	updated, err := s.GetActivity(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "uploaded" {
		t.Fatalf("expected local row to remain uploaded, got %q", updated.Status)
	}
}

func TestDeleteActivityWithAzure_UploadedMissingDocumentIDFailsAndPreservesLocalRow(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveActivities(ctx, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE daily_activities SET status = 'uploaded' WHERE id = ?`, a.ID); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteActivityWithAzure(ctx, a.ID, func(context.Context) (TimeEntryDeleter, error) {
		return &fakeTimeEntryDeleter{}, nil
	}); err == nil || !strings.Contains(err.Error(), "azure document id is required") {
		t.Fatalf("expected missing document id error, got %v", err)
	}
	updated, err := s.GetActivity(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "uploaded" {
		t.Fatalf("expected local row to remain uploaded, got %q", updated.Status)
	}
}

type fakeTimeEntryDeleter struct {
	documentID string
	err        error
}

func (f *fakeTimeEntryDeleter) DeleteTimeEntry(_ context.Context, documentID string) error {
	f.documentID = documentID
	return f.err
}

func createUploadedActivity(t *testing.T, s *Store, ctx context.Context, documentID string) *DailyActivity {
	t.Helper()
	a, err := s.CreateActivity(ctx, "2026-06-12", 1, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveActivities(ctx, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkUploaded(ctx, a.ID, documentID); err != nil {
		t.Fatal(err)
	}
	return a
}
