package store

import (
	"context"
	"testing"
)

func TestWorkItemLoggedHours_SplitsUploadedAndLocal(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	wiA, err := s.AddAzureActivity(ctx, "RUNT2QA", 1001, "Task A", "Task", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	wiB, err := s.AddAzureActivity(ctx, "RUNT2QA", 1002, "Task B", "Task", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}

	newActivity := func(hours float64, azureID *int64) *DailyActivity {
		t.Helper()
		a, err := s.CreateActivity(ctx, "2026-06-12", hours, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
		if err != nil {
			t.Fatal(err)
		}
		if azureID != nil {
			if err := s.SetActivityAzureActivity(ctx, a.ID, azureID); err != nil {
				t.Fatal(err)
			}
		}
		return a
	}

	// Work item A: 1.5 pending + 2 approved (local) + 4 uploaded.
	newActivity(1.5, &wiA.ID)
	approved := newActivity(2, &wiA.ID)
	uploaded := newActivity(4, &wiA.ID)
	if _, err := s.ApproveActivities(ctx, []int64{approved.ID, uploaded.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkUploaded(ctx, uploaded.ID, "doc-1"); err != nil {
		t.Fatal(err)
	}
	// Work item B: 0.5 pending only.
	newActivity(0.5, &wiB.ID)
	// Unlinked activity is ignored.
	newActivity(8, nil)

	got, err := s.WorkItemLoggedHours(ctx, []int{1001, 1002, 1003})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := (WorkItemHours{Uploaded: 4, Local: 3.5}); got[1001] != want {
		t.Errorf("work item 1001: want %+v, got %+v", want, got[1001])
	}
	if want := (WorkItemHours{Uploaded: 0, Local: 0.5}); got[1002] != want {
		t.Errorf("work item 1002: want %+v, got %+v", want, got[1002])
	}
	if _, ok := got[1003]; ok {
		t.Errorf("work item 1003 should be absent, got %+v", got[1003])
	}

	// Restricting ids filters the result.
	only, err := s.WorkItemLoggedHours(ctx, []int{1002})
	if err != nil {
		t.Fatal(err)
	}
	if len(only) != 1 || only[1002].Local != 0.5 {
		t.Errorf("filtered result: want {1002:{Local:0.5}}, got %+v", only)
	}

	empty, err := s.WorkItemLoggedHours(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("empty ids: want empty map, got %v, %v", empty, err)
	}
}

func TestAzureActivityEstimates_MaxAcrossRows(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	// Two catalog rows for the same work item (e.g. different labels) — the
	// estimate is a work item property, so the max wins.
	first, err := s.AddAzureActivity(ctx, "RUNT2QA", 2001, "Task A", "Task", AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddAzureActivity(ctx, "RUNT2QA", 2001, "Task A (alias)", "Task", AzureActivityMapping{}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAzureActivityEstimate(ctx, 2001, 16); err != nil {
		t.Fatal(err)
	}
	// Lower the first row only, directly, to prove MAX is applied.
	if _, err := s.db.ExecContext(ctx, `UPDATE azure_activities SET original_estimate = 4 WHERE id = ?`, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddAzureActivity(ctx, "RUNT2QA", 2002, "Task B", "Task", AzureActivityMapping{}); err != nil {
		t.Fatal(err)
	}

	got, err := s.AzureActivityEstimates(ctx, []int{2001, 2002, 2003})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[2001] != 16 {
		t.Errorf("work item 2001: want 16, got %v", got[2001])
	}
	if v, ok := got[2002]; !ok || v != 0 {
		t.Errorf("work item 2002: want present with 0, got %v (present=%v)", v, ok)
	}
	if _, ok := got[2003]; ok {
		t.Errorf("work item 2003 is not catalogued and should be absent")
	}

	empty, err := s.AzureActivityEstimates(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("empty ids: want empty map, got %v, %v", empty, err)
	}
}
