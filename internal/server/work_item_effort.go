package server

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/Gentleman-Programming/mintag/internal/azure"
)

// maxEffortWorkItemIDs caps how many work items one effort request may ask
// for — one Azure workitems batch.
const maxEffortWorkItemIDs = 200

// workItemEffort is one work item's effort breakdown: the Azure original
// estimate, hours already in TimeLog, hours still local to Mintag
// (pending + approved), and what is left of the estimate.
type workItemEffort struct {
	ID               int     `json:"id"`
	OriginalEstimate float64 `json:"original_estimate"`
	UploadedHours    float64 `json:"uploaded_hours"`
	LocalHours       float64 `json:"local_hours"`
	Remaining        float64 `json:"remaining"`
	HasEstimate      bool    `json:"has_estimate"`
}

// GET /api/activities/azure-work-items/effort?ids=1,2,3
// Read-only: combines the Azure OriginalEstimate, one TimeLog documents
// snapshot (best-effort — a failure is reported in timelog_error with
// uploaded_hours 0) and local not-yet-uploaded hours per work item.
func (srv *Server) handleGetAzureWorkItemEffort(w http.ResponseWriter, r *http.Request) {
	ids, err := parseEffortWorkItemIDs(r.URL.Query().Get("ids"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_ids", map[string]any{"error": err.Error()})
		return
	}

	az := srv.azureClientOrUnavailable(w, r)
	if az == nil {
		return
	}

	estimates, err := az.FetchWorkItemEstimates(r.Context(), ids)
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}

	local, err := srv.st.LocalUnuploadedHoursByWorkItem(r.Context(), ids)
	if err != nil {
		writeJSON(w, nil, err)
		return
	}

	timelogError := ""
	docs, err := az.FetchTimeLogDocuments(r.Context())
	if err != nil {
		timelogError = sanitizePublicError(err)
		docs = nil
	}

	items := make([]workItemEffort, 0, len(ids))
	for _, id := range ids {
		item := workItemEffort{
			ID:               id,
			OriginalEstimate: estimates[id],
			UploadedHours:    azure.TimeLogHours(docs, id),
			LocalHours:       local[id],
		}
		if item.OriginalEstimate > 0 {
			item.HasEstimate = true
			item.Remaining = math.Max(0, math.Round((item.OriginalEstimate-item.UploadedHours-item.LocalHours)*100)/100)
		}
		items = append(items, item)
	}

	writeJSON(w, map[string]any{"org": az.Config().Org, "items": items, "timelog_error": timelogError}, nil)
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
