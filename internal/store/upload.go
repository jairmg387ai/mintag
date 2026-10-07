package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/Gentleman-Programming/mintag/internal/azure"
)

// UploadActivities selects all approved activities for the given date, posts
// each to Azure via the provided client, and marks successful entries as
// uploaded. Failures are collected and returned in the result without stopping
// the remaining uploads. If az is nil or disabled, an error is returned
// immediately with no HTTP calls made.
//
// With the TimeLog bug guard on (the default — see TimeLogBugGuardEnabled),
// each resolved work item is first checked with azure.Client.CheckTimeLogTarget
// and a rejected row fails without being posted.
func (s *Store) UploadActivities(ctx context.Context, date string, az *azure.Client) (*UploadResult, error) {
	if az == nil {
		return nil, fmt.Errorf("Azure TimeLog token is not configured")
	}
	if !az.Enabled() {
		return nil, fmt.Errorf("Azure TimeLog token is not configured")
	}

	activities, err := s.ListActivities(ctx, date, "approved")
	if err != nil {
		return nil, fmt.Errorf("upload: list activities: %w", err)
	}

	// Resolve the default activity and the full id->work_item_id map once per
	// batch (not per entry) so every entry in this upload sees a consistent
	// snapshot. includeInactive=true so a soft-deleted activity that a
	// historical row still references can still resolve.
	//
	// ErrNoDefaultAzureActivity is an expected, recoverable state (per-entry
	// resolution below fails only the rows that actually need the default);
	// any other error is a genuine query failure and aborts the whole batch,
	// same as the ListActivities error handling above.
	defaultActivity, err := s.GetDefaultAzureActivity(ctx)
	if err != nil && !errors.Is(err, ErrNoDefaultAzureActivity) {
		return nil, fmt.Errorf("upload: get default azure activity: %w", err)
	}
	allActivities, err := s.ListAzureActivities(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("upload: list azure activities: %w", err)
	}
	workItemByAzureActivityID := make(map[int64]int, len(allActivities))
	for _, aa := range allActivities {
		workItemByAzureActivityID[aa.ID] = aa.WorkItemID
	}

	// The bug guard (see TimeLogBugGuardEnabled) is read once per batch like
	// the catalog snapshot above. With it off, no extra Azure call is made.
	bugGuard, err := s.TimeLogBugGuardEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("upload: read bug guard setting: %w", err)
	}
	// targetCheckByWorkItem caches CheckTimeLogTarget's verdict per work item
	// for this upload only: many rows usually share a work item, and the
	// hierarchy can change between uploads, so it is never persisted.
	targetCheckByWorkItem := map[int]error{}

	result := &UploadResult{
		FailedIDs:        []int64{},
		Errors:           []string{},
		AzureDocumentIDs: map[int64]string{},
	}

	for _, a := range activities {
		workItemID, resolveErr := resolveAzureWorkItemID(a.AzureActivityID, workItemByAzureActivityID, defaultActivity)
		if resolveErr != nil {
			result.FailedIDs = append(result.FailedIDs, a.ID)
			result.Errors = append(result.Errors, resolveErr.Error())
			continue
		}
		if bugGuard {
			checkErr, checked := targetCheckByWorkItem[workItemID]
			if !checked {
				checkErr = az.CheckTimeLogTarget(ctx, workItemID)
				targetCheckByWorkItem[workItemID] = checkErr
			}
			// Both a rule rejection and a failure to read Azure fail only this
			// row, which stays "approved" for a retry — same partial-failure
			// semantics as a failed PostTimeEntry below. Failing closed on a
			// read error is deliberate: posting without the check is exactly
			// what the guard exists to prevent.
			if checkErr != nil {
				result.FailedIDs = append(result.FailedIDs, a.ID)
				result.Errors = append(result.Errors, checkErr.Error())
				continue
			}
		}

		entry := azure.TimeEntry{
			Date:           a.Date,
			Hours:          a.Hours,
			RegistroDiario: a.RegistroDiario,
			WorkItemID:     workItemID,
		}
		azureDocumentID, postErr := az.PostTimeEntry(ctx, entry)
		if postErr != nil {
			result.FailedIDs = append(result.FailedIDs, a.ID)
			result.Errors = append(result.Errors, postErr.Error())
			continue
		}
		if markErr := s.MarkUploaded(ctx, a.ID, azureDocumentID); markErr != nil {
			result.FailedIDs = append(result.FailedIDs, a.ID)
			result.Errors = append(result.Errors, markErr.Error())
			continue
		}
		result.AzureDocumentIDs[a.ID] = azureDocumentID
		result.UploadedCount++
	}

	return result, nil
}

// resolveAzureWorkItemID picks the Azure work item id for a single activity
// row using a batch-wide snapshot (workItemByAzureActivityID, defaultActivity)
// resolved once before the upload loop:
//   - a non-nil azureActivityID present in the snapshot wins (explicit
//     per-record assignment, including soft-deleted activities so historical
//     assignments still resolve);
//   - otherwise it falls back to the current default activity;
//   - if neither resolves (missing FK target, or no default configured), it
//     returns an error so the caller can record the row as failed and leave
//     it in "approved" status for a retry, matching existing partial-failure
//     semantics.
func resolveAzureWorkItemID(azureActivityID *int64, workItemByAzureActivityID map[int64]int, defaultActivity *AzureActivity) (int, error) {
	if azureActivityID != nil {
		if workItemID, ok := workItemByAzureActivityID[*azureActivityID]; ok {
			return workItemID, nil
		}
		if defaultActivity == nil {
			return 0, fmt.Errorf("azure activity %d not found and no default azure activity is configured", *azureActivityID)
		}
		return defaultActivity.WorkItemID, nil
	}
	if defaultActivity == nil {
		return 0, fmt.Errorf("no default azure activity is configured")
	}
	return defaultActivity.WorkItemID, nil
}
