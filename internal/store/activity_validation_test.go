package store

import (
	"context"
	"testing"
)

// TestGetActivityValidationSettings_DefaultsToAllOff verifies a fresh store
// has all three validations disabled — this feature must be entirely
// non-breaking until a user opts in.
func TestGetActivityValidationSettings_DefaultsToAllOff(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	v, err := s.GetActivityValidationSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.MaxHoursPerEntry || v.WeekendConfirm || v.BlockClosedWorkItem {
		t.Errorf("expected all three settings to default to false, got %+v", v)
	}
}

// TestSetActivityValidationSettings_RoundTrips verifies a configured value
// reads back exactly, for every combination of the three booleans.
func TestSetActivityValidationSettings_RoundTrips(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	cases := []ActivityValidationSettings{
		{MaxHoursPerEntry: true, WeekendConfirm: false, BlockClosedWorkItem: false},
		{MaxHoursPerEntry: false, WeekendConfirm: true, BlockClosedWorkItem: false},
		{MaxHoursPerEntry: false, WeekendConfirm: false, BlockClosedWorkItem: true},
		{MaxHoursPerEntry: true, WeekendConfirm: true, BlockClosedWorkItem: true},
		{MaxHoursPerEntry: false, WeekendConfirm: false, BlockClosedWorkItem: false},
	}
	for _, want := range cases {
		if err := s.SetActivityValidationSettings(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetActivityValidationSettings(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if *got != want {
			t.Errorf("expected %+v, got %+v", want, *got)
		}
	}
}
