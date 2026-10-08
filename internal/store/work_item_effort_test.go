package store

import (
	"context"
	"testing"
)

func TestLocalUnuploadedHoursByWorkItem(t *testing.T) {
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

	// Work item A: 1.5 pending + 2 approved + 4 uploaded (excluded).
	newActivity(1.5, &wiA.ID)
	approved := newActivity(2, &wiA.ID)
	uploaded := newActivity(4, &wiA.ID)
	if _, err := s.ApproveActivities(ctx, []int64{approved.ID, uploaded.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkUploaded(ctx, uploaded.ID, "doc-1"); err != nil {
		t.Fatal(err)
	}
	// Work item B: 0.5 pending.
	newActivity(0.5, &wiB.ID)
	// Unlinked activity is ignored.
	newActivity(8, nil)

	got, err := s.LocalUnuploadedHoursByWorkItem(ctx, []int{1001, 1002, 1003})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[1001] != 3.5 {
		t.Errorf("work item 1001: want 3.5, got %v", got[1001])
	}
	if got[1002] != 0.5 {
		t.Errorf("work item 1002: want 0.5, got %v", got[1002])
	}
	if _, ok := got[1003]; ok {
		t.Errorf("work item 1003 should be absent, got %v", got[1003])
	}

	// Restricting ids filters the result.
	only, err := s.LocalUnuploadedHoursByWorkItem(ctx, []int{1002})
	if err != nil {
		t.Fatal(err)
	}
	if len(only) != 1 || only[1002] != 0.5 {
		t.Errorf("filtered result: want {1002:0.5}, got %v", only)
	}

	empty, err := s.LocalUnuploadedHoursByWorkItem(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("empty ids: want empty map, got %v, %v", empty, err)
	}
}
