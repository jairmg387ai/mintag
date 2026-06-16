package store

import (
	"context"
	"fmt"

	"github.com/Gentleman-Programming/mintag/internal/azure"
)

// UploadActivities selects all approved activities for the given date, posts
// each to Azure via the provided client, and marks successful entries as
// uploaded. Failures are collected and returned in the result without stopping
// the remaining uploads. If az is nil or disabled, an error is returned
// immediately with no HTTP calls made.
func (s *Store) UploadActivities(ctx context.Context, date string, az *azure.Client) (*UploadResult, error) {
	if az == nil {
		return nil, fmt.Errorf("MINTAG_AZURE_TIMELOG_PAT is not configured")
	}
	if !az.Enabled() {
		return nil, fmt.Errorf("MINTAG_AZURE_TIMELOG_PAT is not configured")
	}

	activities, err := s.ListActivities(ctx, date, "approved")
	if err != nil {
		return nil, fmt.Errorf("upload: list activities: %w", err)
	}

	result := &UploadResult{
		FailedIDs: []int64{},
		Errors:    []string{},
	}

	for _, a := range activities {
		entry := azure.TimeEntry{
			Date:           a.Date,
			Hours:          a.Hours,
			RegistroDiario: a.RegistroDiario,
		}
		if postErr := az.PostTimeEntry(ctx, entry); postErr != nil {
			result.FailedIDs = append(result.FailedIDs, a.ID)
			result.Errors = append(result.Errors, postErr.Error())
			continue
		}
		if markErr := s.MarkUploaded(ctx, a.ID); markErr != nil {
			result.FailedIDs = append(result.FailedIDs, a.ID)
			result.Errors = append(result.Errors, markErr.Error())
			continue
		}
		result.UploadedCount++
	}

	return result, nil
}
