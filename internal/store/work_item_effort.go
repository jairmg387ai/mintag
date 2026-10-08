package store

import (
	"context"
	"math"
	"strings"
)

// WorkItemHours is the hours logged in Mintag against one Azure work item,
// split by whether they already reached TimeLog.
type WorkItemHours struct {
	// Uploaded is the sum of activities with status 'uploaded'.
	Uploaded float64
	// Local is the sum of activities still local to Mintag (pending or
	// approved, i.e. not yet in TimeLog).
	Local float64
}

// workItemIDArgs builds the "?,?,..." placeholder list and args for an
// IN (...) clause over work item ids.
func workItemIDArgs(ids []int) (string, []any) {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	return strings.Join(placeholders, ","), args
}

// WorkItemLoggedHours sums, in one query, the hours of daily activities
// linked to each requested work item through daily_activities.azure_activity_id
// -> azure_activities.id, split into uploaded vs local (pending/approved).
// Activities with no azure_activity_id are ignored. Only work items with at
// least one such activity appear in the result; totals are rounded to 2
// decimals.
func (s *Store) WorkItemLoggedHours(ctx context.Context, ids []int) (map[int]WorkItemHours, error) {
	out := make(map[int]WorkItemHours, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	in, args := workItemIDArgs(ids)

	rows, err := s.db.QueryContext(ctx, `
		SELECT aa.work_item_id,
		       COALESCE(SUM(CASE WHEN da.status = 'uploaded' THEN da.hours END), 0),
		       COALESCE(SUM(CASE WHEN da.status IN ('pending', 'approved') THEN da.hours END), 0)
		FROM daily_activities da
		JOIN azure_activities aa ON aa.id = da.azure_activity_id
		WHERE da.status IN ('uploaded', 'pending', 'approved')
		  AND aa.work_item_id IN (`+in+`)
		GROUP BY aa.work_item_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var workItemID int
		var uploaded, local float64
		if err := rows.Scan(&workItemID, &uploaded, &local); err != nil {
			return nil, err
		}
		out[workItemID] = WorkItemHours{
			Uploaded: math.Round(uploaded*100) / 100,
			Local:    math.Round(local*100) / 100,
		}
	}
	return out, rows.Err()
}

// AzureActivityEstimates returns the locally cached OriginalEstimate for each
// requested work item that has at least one catalog entry. When several
// entries reference the same work item, the highest estimate wins — the
// estimate is a property of the work item, so a lower value can only be a
// stale row. Uncatalogued work items are absent from the result.
func (s *Store) AzureActivityEstimates(ctx context.Context, ids []int) (map[int]float64, error) {
	out := make(map[int]float64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	in, args := workItemIDArgs(ids)

	rows, err := s.db.QueryContext(ctx, `
		SELECT work_item_id, MAX(original_estimate)
		FROM azure_activities
		WHERE work_item_id IN (`+in+`)
		GROUP BY work_item_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var workItemID int
		var estimate float64
		if err := rows.Scan(&workItemID, &estimate); err != nil {
			return nil, err
		}
		out[workItemID] = estimate
	}
	return out, rows.Err()
}
