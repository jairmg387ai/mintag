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
	writeJSON(w, map[string]any{"org": az.Config().Org, "items": items}, nil)
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
