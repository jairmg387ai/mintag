package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Gentleman-Programming/mintag/internal/azure"
	"github.com/Gentleman-Programming/mintag/internal/store"
)

// POST /api/activities/azure-work-items
// Creates a new Azure DevOps Task work item (Proposed) and immediately
// attempts to activate it (Active/Accepted). If activation fails, the
// response still reports the created id/state=Proposed plus
// activation_error — the work item exists in Azure either way, so this is
// never a 5xx: only a failed *creation* is.
//
// When project and/or category_id are supplied, the created work item is
// also registered into the azure_activities catalog (same mapping used by
// the Activities autofill system), so it is immediately available for
// activity logging without a separate manual catalog step. That
// registration is best-effort and independent of activation: a failure is
// reported as catalog_error, never as a lost work item — see
// activation_error's identical partial-success treatment above.
func (srv *Server) handleCreateAzureWorkItem(w http.ResponseWriter, r *http.Request) {
	az, err := srv.newAzureTimeLogClient(r.Context())
	if err != nil {
		writeJSON(w, nil, err)
		return
	}
	if az == nil || !az.Enabled() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Azure TimeLog token is not configured"}) //nolint:errcheck
		return
	}

	var body struct {
		Title            string  `json:"title"`
		Description      string  `json:"description"`
		AreaPath         string  `json:"area_path"`
		IterationPath    string  `json:"iteration_path"`
		OriginalEstimate float64 `json:"original_estimate"`
		Project          *string `json:"project"`
		CategoryID       *int64  `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Title) == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.AreaPath) == "" {
		http.Error(w, "area_path is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.IterationPath) == "" {
		http.Error(w, "iteration_path is required", http.StatusBadRequest)
		return
	}

	created, err := az.CreateAndActivateWorkItem(r.Context(), azure.CreateWorkItemInput{
		Title:            body.Title,
		Description:      body.Description,
		AreaPath:         body.AreaPath,
		IterationPath:    body.IterationPath,
		OriginalEstimate: body.OriginalEstimate,
	})

	var activationErr *azure.ActivationError
	if errors.As(err, &activationErr) {
		resp := map[string]any{
			"id":               created.ID,
			"state":            created.State,
			"activation_error": sanitizePublicError(activationErr.Err),
		}
		srv.registerAzureWorkItemCatalogEntry(r.Context(), az, created, body.Title, body.Project, body.CategoryID, resp)
		writeJSON(w, resp, nil)
		return
	}
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}

	resp := map[string]any{"id": created.ID, "state": created.State}
	srv.registerAzureWorkItemCatalogEntry(r.Context(), az, created, body.Title, body.Project, body.CategoryID, resp)
	writeJSON(w, resp, nil)
}

// registerAzureWorkItemCatalogEntry adds the just-created work item to the
// azure_activities catalog when project and/or categoryID were supplied,
// mutating resp in place with azure_activity_id on success or catalog_error
// on failure. A nil project and nil categoryID mean the caller did not ask
// for catalog registration at all — resp is left untouched in that case.
func (srv *Server) registerAzureWorkItemCatalogEntry(ctx context.Context, az *azure.Client, created azure.CreatedWorkItem, title string, project *string, categoryID *int64, resp map[string]any) {
	if project == nil && categoryID == nil {
		return
	}
	a, err := srv.st.AddAzureActivity(ctx, az.Config().Org, created.ID, strings.TrimSpace(title), "Task", store.AzureActivityMapping{
		Project:    project,
		CategoryID: categoryID,
	})
	if err != nil {
		resp["catalog_error"] = sanitizePublicError(err)
		return
	}
	resp["azure_activity_id"] = a.ID
}

// GET /api/activities/azure-work-items/states?ids=1,2,3
// Resolves current title/type/state for an arbitrary set of work item ids
// (e.g. the local Azure activity catalog) — a manual, opt-in refresh, not
// something the frontend polls automatically.
func (srv *Server) handleGetAzureWorkItemStates(w http.ResponseWriter, r *http.Request) {
	az, err := srv.newAzureTimeLogClient(r.Context())
	if err != nil {
		writeJSON(w, nil, err)
		return
	}
	if az == nil || !az.Enabled() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Azure TimeLog token is not configured"}) //nolint:errcheck
		return
	}

	raw := strings.TrimSpace(r.URL.Query().Get("ids"))
	var ids []int
	if raw != "" {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, convErr := strconv.Atoi(part)
			if convErr != nil {
				http.Error(w, fmt.Sprintf("invalid work item id %q", part), http.StatusBadRequest)
				return
			}
			ids = append(ids, id)
		}
	}

	items, err := az.FetchWorkItemsByIDs(r.Context(), ids)
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	// Persist each item's live state/type/assignee into the local catalog so
	// the portal has a "last known" value to show on load without requiring
	// this manual refresh every time — see SyncAzureActivityLiveState's doc
	// comment. Best-effort: a write failure here must never turn an
	// otherwise-successful Azure fetch into an error response.
	for _, item := range items {
		_ = srv.st.SyncAzureActivityLiveState(r.Context(), item.ID, item.State, item.Type, item.AssignedToDisplayName)
	}
	writeJSON(w, map[string]any{"org": az.Config().Org, "team_project": az.Config().TeamProject, "items": items}, nil)
}

// azureWorkItemID parses the {id} path param shared by the close/recreate
// routes, writing a 400 and returning ok=false on a malformed value.
func azureWorkItemID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, fmt.Sprintf("invalid work item id %q", chi.URLParam(r, "id")), http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// ensureAssignedToCaller enforces that only the person a Task is assigned to
// in Azure may Close or Recreate it. This is not redundant with Azure's own
// permission model: tech leads register time against shared "transversal"
// catalog entries (meetings, PTO, permits) that a whole team logs hours
// against, and a dev may likewise be handed a TL-shared task for the same
// reason — everyone with TimeLog access to the org can otherwise close
// anyone's task through mintag. Compares full.AssignedToID (not the display
// name, which is not guaranteed unique) against az.Config().UserID.
//
// A caller whose identity hasn't been resolved (UserID empty — see
// FetchIdentity/SaveAzureIdentity) cannot be checked and is let through:
// falling closed here would break every PAT-based setup that never
// configured MINTAG_AZURE_USER_ID, for a check that's a UX guard rail against
// mistaken clicks, not a security boundary — anyone with a valid credential
// can call the Azure API directly regardless of what mintag enforces.
func ensureAssignedToCaller(az *azure.Client, full *azure.WorkItemFull) error {
	callerID := strings.TrimSpace(az.Config().UserID)
	if callerID == "" {
		return nil
	}
	if full.AssignedToID == callerID {
		return nil
	}
	assignee := full.AssignedToDisplayName
	if assignee == "" {
		assignee = "no one (unassigned)"
	}
	return fmt.Errorf("work item %d is assigned to %s, not you — only the assignee can close or recreate it", full.ID, assignee)
}

// POST /api/activities/azure-work-items/{id}/close
// Closes a Task work item, syncing Completed/Remaining Work with hours
// actually logged in TimeLog first. Already-closed work items are a no-op
// (already_closed:true), not an error. The effort sync is best-effort: a
// failure there does not block the close — the state transition is the
// point of this action, and a stale Completed/Remaining field is a lesser
// problem than the work item staying open — so it is surfaced as a non-fatal
// effort_sync_error while the close still proceeds. A failure to close
// itself (the primary mutation) IS a hard failure, since nothing else
// succeeded worth reporting.
func (srv *Server) handleCloseAzureWorkItem(w http.ResponseWriter, r *http.Request) {
	id, ok := azureWorkItemID(w, r)
	if !ok {
		return
	}

	az, err := srv.newAzureTimeLogClient(r.Context())
	if err != nil {
		writeJSON(w, nil, err)
		return
	}
	if az == nil || !az.Enabled() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Azure TimeLog token is not configured"}) //nolint:errcheck
		return
	}

	full, err := az.FetchWorkItemFull(r.Context(), id)
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	if full == nil {
		http.Error(w, fmt.Sprintf("work item %d not found", id), http.StatusNotFound)
		return
	}
	if err := ensureAssignedToCaller(az, full); err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusForbidden)
		return
	}

	if azure.IsClosedState(full.State) {
		writeJSON(w, map[string]any{"state": full.State, "hours_synced": 0, "already_closed": true}, nil)
		return
	}

	resp := map[string]any{}
	hoursSynced, syncErr := az.SyncEffortFromTimeLog(r.Context(), id, full.OriginalEstimate)
	if syncErr != nil {
		resp["effort_sync_error"] = sanitizePublicError(syncErr)
	}
	if err := az.CloseWorkItem(r.Context(), id, full.State); err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	resp["state"] = "Closed"
	resp["hours_synced"] = hoursSynced
	writeJSON(w, resp, nil)
}

// POST /api/activities/azure-work-items/{id}/recreate
// Closes the given Task (same effort-sync-then-close as handleCloseAzureWorkItem)
// and creates a new one with the same Title/Description/AreaPath/IterationPath/
// OriginalEstimate. If the old work item was registered in the azure_activities
// catalog, that entry's work_item_id is repointed at the new work item so the
// Activities autofill mapping (Project/Category) keeps working — the catalog
// otherwise silently ends up pointing at a closed, unloggable work item.
//
// Closing the old work item is NOT best-effort: if it fails, the whole
// operation aborts before creating anything, because leaving both the old
// and a brand-new work item open would double the outstanding effort for
// the same task. Effort sync and catalog reassignment, like the create
// endpoint's activation, are both best-effort and surfaced as non-fatal
// *_error fields once the new work item safely exists.
func (srv *Server) handleRecreateAzureWorkItem(w http.ResponseWriter, r *http.Request) {
	id, ok := azureWorkItemID(w, r)
	if !ok {
		return
	}

	az, err := srv.newAzureTimeLogClient(r.Context())
	if err != nil {
		writeJSON(w, nil, err)
		return
	}
	if az == nil || !az.Enabled() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Azure TimeLog token is not configured"}) //nolint:errcheck
		return
	}

	old, err := az.FetchWorkItemFull(r.Context(), id)
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	if old == nil {
		http.Error(w, fmt.Sprintf("work item %d not found", id), http.StatusNotFound)
		return
	}
	if err := ensureAssignedToCaller(az, old); err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusForbidden)
		return
	}

	resp := map[string]any{}
	hoursSynced, syncErr := az.SyncEffortFromTimeLog(r.Context(), id, old.OriginalEstimate)
	if syncErr != nil {
		resp["effort_sync_error"] = sanitizePublicError(syncErr)
	}
	resp["hours_synced"] = hoursSynced

	if err := az.CloseWorkItem(r.Context(), id, old.State); err != nil {
		http.Error(w, fmt.Sprintf("could not close work item %d, no new work item was created: %s", id, sanitizePublicError(err)), http.StatusBadGateway)
		return
	}

	created, err := az.CreateAndActivateWorkItem(r.Context(), azure.CreateWorkItemInput{
		Title:            old.Title,
		Description:      old.Description,
		AreaPath:         old.AreaPath,
		IterationPath:    old.IterationPath,
		OriginalEstimate: old.OriginalEstimate,
	})

	var activationErr *azure.ActivationError
	if errors.As(err, &activationErr) {
		resp["id"] = created.ID
		resp["state"] = created.State
		resp["activation_error"] = sanitizePublicError(activationErr.Err)
	} else if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	} else {
		resp["id"] = created.ID
		resp["state"] = created.State
	}

	srv.reassignAzureWorkItemCatalogEntry(r.Context(), id, created.ID, resp)
	writeJSON(w, resp, nil)
}

// reassignAzureWorkItemCatalogEntry repoints the catalog entry (if any)
// referencing oldWorkItemID at newWorkItemID, mutating resp in place with
// catalog_reassigned/azure_activity_id on success, or catalog_error on
// failure. Absence of a catalog entry for oldWorkItemID is not an error —
// not every work item is catalog-registered — and resp reports
// catalog_reassigned:false in that case.
func (srv *Server) reassignAzureWorkItemCatalogEntry(ctx context.Context, oldWorkItemID, newWorkItemID int, resp map[string]any) {
	existing, err := srv.st.FindAzureActivityByWorkItemID(ctx, oldWorkItemID)
	if errors.Is(err, store.ErrAzureActivityNotFound) {
		resp["catalog_reassigned"] = false
		return
	}
	if err != nil {
		resp["catalog_reassigned"] = false
		resp["catalog_error"] = sanitizePublicError(err)
		return
	}
	reassigned, err := srv.st.ReassignAzureActivityWorkItem(ctx, existing.ID, newWorkItemID)
	if err != nil {
		resp["catalog_reassigned"] = false
		resp["catalog_error"] = sanitizePublicError(err)
		return
	}
	resp["catalog_reassigned"] = true
	resp["azure_activity_id"] = reassigned.ID
}

// GET /api/activities/azure-classification-nodes/{kind}
// Returns the full Area or Iteration path tree for the configured team
// project (kind must be "areas" or "iterations"), so the frontend can offer
// a searchable picker instead of a hand-typed path.
func (srv *Server) handleGetAzureClassificationTree(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	if kind != "areas" && kind != "iterations" {
		http.Error(w, `kind must be "areas" or "iterations"`, http.StatusBadRequest)
		return
	}

	az, err := srv.newAzureTimeLogClient(r.Context())
	if err != nil {
		writeJSON(w, nil, err)
		return
	}
	if az == nil || !az.Enabled() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Azure TimeLog token is not configured"}) //nolint:errcheck
		return
	}

	tree, err := az.FetchClassificationTree(r.Context(), kind)
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	writeJSON(w, tree, nil)
}
