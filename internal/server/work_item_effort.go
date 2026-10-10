package server

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// maxEffortWorkItemIDs caps how many work items one effort request may ask
// for.
const maxEffortWorkItemIDs = 200

// workItemEffort is one work item's effort breakdown, computed entirely from
// the Mintag DB: the cached Azure original estimate, hours already uploaded
// to TimeLog, hours still local to Mintag (pending + approved), and what is
// left of the estimate.
type workItemEffort struct {
	ID               int     `json:"id"`
	OriginalEstimate float64 `json:"original_estimate"`
	UploadedHours    float64 `json:"uploaded_hours"`
	LocalHours       float64 `json:"local_hours"`
	Remaining        float64 `json:"remaining"`
	HasEstimate      bool    `json:"has_estimate"`
}

// GET /api/activities/azure-work-items/effort?ids=1,2,3
// Read-only and local: the user logs all their time from Mintag (Azure's
// Completed/Remaining fields are not trustworthy), so effort comes from the
// DB alone — no Azure or TimeLog call, and Azure need not be configured. The
// estimate is the one cached on the catalog (refreshed on work item creation
// and on the states refresh); uncatalogued work items report no estimate.
func (srv *Server) handleGetAzureWorkItemEffort(w http.ResponseWriter, r *http.Request) {
	ids, err := parseEffortWorkItemIDs(r.URL.Query().Get("ids"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_ids", map[string]any{"error": err.Error()})
		return
	}

	estimates, err := srv.st.AzureActivityEstimates(r.Context(), ids)
	if err != nil {
		writeJSON(w, nil, err)
		return
	}
	logged, err := srv.st.WorkItemLoggedHours(r.Context(), ids)
	if err != nil {
		writeJSON(w, nil, err)
		return
	}

	items := make([]workItemEffort, 0, len(ids))
	for _, id := range ids {
		item := workItemEffort{
			ID:               id,
			OriginalEstimate: estimates[id],
			UploadedHours:    logged[id].Uploaded,
			LocalHours:       logged[id].Local,
		}
		if item.OriginalEstimate > 0 {
			item.HasEstimate = true
			item.Remaining = math.Max(0, math.Round((item.OriginalEstimate-item.UploadedHours-item.LocalHours)*100)/100)
		}
		items = append(items, item)
	}

	writeJSON(w, map[string]any{"items": items}, nil)
}

// parseEffortWorkItemIDs parses a comma-separated list of positive work item
// ids, dropping duplicates while keeping first-seen order. It rejects an
// empty list, any non-positive or non-numeric id, and more than
// maxEffortWorkItemIDs distinct ids.
func parseEffortWorkItemIDs(raw string) ([]int, error) {
	seen := map[int]bool{}
	var ids []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid work item id %q", part)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids is required")
	}
	if len(ids) > maxEffortWorkItemIDs {
		return nil, fmt.Errorf("at most %d work item ids are allowed, got %d", maxEffortWorkItemIDs, len(ids))
	}
	return ids, nil
}
