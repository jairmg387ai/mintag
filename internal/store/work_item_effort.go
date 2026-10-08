package store

import (
	"context"
	"math"
	"strings"
)

// LocalUnuploadedHoursByWorkItem sums the hours of daily activities that are
// still local to Mintag (status pending or approved, i.e. not yet in TimeLog)
// grouped by the Azure work item they are linked to through
// daily_activities.azure_activity_id -> azure_activities.id. Activities with
// no azure_activity_id are ignored. Only work items with at least one such
// activity appear in the result; totals are rounded to 2 decimals.
func (s *Store) LocalUnuploadedHoursByWorkItem(ctx context.Context, ids []int) (map[int]float64, error) {
	out := make(map[int]float64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT aa.work_item_id, SUM(da.hours)
		FROM daily_activities da
		JOIN azure_activities aa ON aa.id = da.azure_activity_id
		WHERE da.status IN ('pending', 'approved')
		  AND aa.work_item_id IN (`+strings.Join(placeholders, ",")+`)
		GROUP BY aa.work_item_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var workItemID int
		var hours float64
		if err := rows.Scan(&workItemID, &hours); err != nil {
			return nil, err
		}
		out[workItemID] = math.Round(hours*100) / 100
	}
	return out, rows.Err()
}
