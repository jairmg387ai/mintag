package store

import (
	"context"
	"fmt"
	"testing"
)

// TestTimeLogBugGuardEnabled_DefaultsOnAndToggles verifies the guard is ON for
// a fresh database (no row in app_settings) and that the setter round-trips
// both values.
func TestTimeLogBugGuardEnabled_DefaultsOnAndToggles(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	enabled, err := s.TimeLogBugGuardEnabled(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Fatal("expected guard to be enabled by default on a fresh database")
	}

	for _, want := range []bool{false, true, false} {
		if err := s.SetTimeLogBugGuardEnabled(ctx, want); err != nil {
			t.Fatalf("set guard=%v: %v", want, err)
		}
		got, err := s.TimeLogBugGuardEnabled(ctx)
		if err != nil {
			t.Fatalf("get guard after set=%v: %v", want, err)
		}
		if got != want {
			t.Errorf("expected guard=%v after set, got %v", want, got)
		}
	}
}

// TestTimeLogBugGuardEnabled_UnparseableValueFallsBackToOn verifies that a
// corrupted stored value never silently disables the guard.
func TestTimeLogBugGuardEnabled_UnparseableValueFallsBackToOn(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	if err := s.setSetting(ctx, settingTimeLogBugGuardEnabled, "garbage"); err != nil {
		t.Fatal(err)
	}
	enabled, err := s.TimeLogBugGuardEnabled(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Error("expected unparseable stored value to fall back to enabled")
	}
}

func TestSetActivityAzureActivity_BugGuard(t *testing.T) {
	tests := []struct {
		name         string
		workItemType string
		guardOff     bool
		wantErr      string // empty means the link is accepted
	}{
		{name: "bug rejected", workItemType: "Bug", wantErr: "azure activity %d points at Bug 171191; log hours on the bug's child task assigned to you instead (add it to the catalog)"},
		{name: "bug type is case-insensitive", workItemType: " bug ", wantErr: "azure activity %d points at Bug 171191; log hours on the bug's child task assigned to you instead (add it to the catalog)"},
		{name: "task allowed", workItemType: "Task"},
		{name: "unknown type allowed", workItemType: ""},
		{name: "guard off allows bug", workItemType: "Bug", guardOff: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := OpenInMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			ctx := context.Background()
			if tt.guardOff {
				if err := s.SetTimeLogBugGuardEnabled(ctx, false); err != nil {
					t.Fatal(err)
				}
			}
			entry, err := s.AddAzureActivity(ctx, "ORG", 171191, "Login falla", tt.workItemType, AzureActivityMapping{})
			if err != nil {
				t.Fatal(err)
			}
			a, err := s.CreateActivity(ctx, "2026-06-12", 1, "RNCEA", "Actividades de arquitectura, diseño y código", "Trabajo", "manual")
			if err != nil {
				t.Fatal(err)
			}

			err = s.SetActivityAzureActivity(ctx, a.ID, &entry.ID)
			got, getErr := s.GetActivity(ctx, a.ID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected link accepted, got %v", err)
				}
				if got.AzureActivityID == nil || *got.AzureActivityID != entry.ID {
					t.Errorf("expected azure_activity_id=%d, got %v", entry.ID, got.AzureActivityID)
				}
				return
			}
			if err == nil || err.Error() != fmt.Sprintf(tt.wantErr, entry.ID) {
				t.Fatalf("unexpected error:\n got: %v\nwant: %s", err, fmt.Sprintf(tt.wantErr, entry.ID))
			}
			if got.AzureActivityID != nil {
				t.Errorf("rejected link must not be persisted, got %v", *got.AzureActivityID)
			}
		})
	}
}

// TestActivityValidationSettings_BlockBugWorkItemDefaultsOnAndMapsToGuard
// verifies the bug guard is exposed as the fourth ActivityValidationSettings
// toggle: on by default (unlike its three siblings) and backed by the same
// setting TimeLogBugGuardEnabled reads.
func TestActivityValidationSettings_BlockBugWorkItemDefaultsOnAndMapsToGuard(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	v, err := s.GetActivityValidationSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !v.BlockBugWorkItem {
		t.Fatalf("expected block_bug_work_item to default on, got %+v", v)
	}

	v.BlockBugWorkItem = false
	if err := s.SetActivityValidationSettings(ctx, *v); err != nil {
		t.Fatal(err)
	}
	enabled, err := s.TimeLogBugGuardEnabled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Error("expected the guard to be off after saving block_bug_work_item=false")
	}
}
