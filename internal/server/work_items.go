package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Gentleman-Programming/mintag/internal/azure"
)

// POST /api/activities/azure-work-items
// Creates a new Azure DevOps Task work item (Proposed) and immediately
// attempts to activate it (Active/Accepted). If activation fails, the
// response still reports the created id/state=Proposed plus
// activation_error — the work item exists in Azure either way, so this is
// never a 5xx: only a failed *creation* is.
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
		writeJSON(w, map[string]any{
			"id":               created.ID,
			"state":            created.State,
			"activation_error": sanitizePublicError(activationErr.Err),
		}, nil)
		return
	}
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"id": created.ID, "state": created.State}, nil)
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
