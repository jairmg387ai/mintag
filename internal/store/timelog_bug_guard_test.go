package store

import (
	"context"
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
