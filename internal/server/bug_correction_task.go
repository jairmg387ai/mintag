package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Gentleman-Programming/mintag/internal/azure"
	"github.com/Gentleman-Programming/mintag/internal/store"
)

// registerBugCorrectionTaskRoutes wires the bug correction-task form routes.
// They are registered as flat routes (not inside registerBugEvidenceRoutes'
// r.Route("/azure/bugs/{id}")) — chi resolves the static suffixes before
// that subrouter's catch-all.
func registerBugCorrectionTaskRoutes(r chi.Router, srv *Server) {
	r.With(requireLocalRequest).Get("/azure/bugs/{id}/correction-task-draft", srv.handleGetBugCorrectionTaskDraft)
	r.With(requireLocalRequest).Post("/azure/bugs/{id}/correction-task", srv.handleCreateBugCorrectionTask)
	r.With(requireLocalRequest).Get("/azure/work-item-fields/subarea/allowed-values", srv.handleGetSubareaAllowedValues)
}

// azureClientOrUnavailable resolves the Azure client, writing the error or
// the shared 503 body and returning nil when it can't be used.
func (srv *Server) azureClientOrUnavailable(w http.ResponseWriter, r *http.Request) *azure.Client {
	az, err := srv.newAzureTimeLogClient(r.Context())
	if err != nil {
		writeJSON(w, nil, err)
		return nil
	}
	if az == nil || !az.Enabled() {
		writeAzureNotConfigured(w)
		return nil
	}
	return az
}

// fetchBugDraftOrError runs fetch (the full draft, or the bug-only read)
// and writes 404 / 502 / 400 not_a_bug as appropriate, returning nil when
// the handler must stop.
func fetchBugDraftOrError(w http.ResponseWriter, id int, fetch func() (*azure.BugCorrectionTaskDraft, error)) *azure.BugCorrectionTaskDraft {
	d, err := fetch()
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return nil
	}
	if d == nil {
		http.Error(w, fmt.Sprintf("work item %d not found", id), http.StatusNotFound)
		return nil
	}
	if !d.IsBug() {
		writeAPIError(w, http.StatusBadRequest, "not_a_bug", nil)
		return nil
	}
	return d
}

// GET /api/azure/bugs/{id}/correction-task-draft
// Prefill for the correction-task form: the bug's team project, area,
// iteration and assignee, the suggested title, and any correction Tasks the
// bug already has (so the UI can warn about duplicates).
func (srv *Server) handleGetBugCorrectionTaskDraft(w http.ResponseWriter, r *http.Request) {
	id, ok := azureWorkItemID(w, r)
	if !ok {
		return
	}
	az := srv.azureClientOrUnavailable(w, r)
	if az == nil {
		return
	}
	d := fetchBugDraftOrError(w, id, func() (*azure.BugCorrectionTaskDraft, error) {
		return az.FetchBugCorrectionTaskDraft(r.Context(), id)
	})
	if d == nil {
		return
	}
	existing := d.ExistingCorrectionTasks
	if existing == nil {
		existing = []azure.CorrectionTaskSummary{}
	}
	resp := map[string]any{
		"org":                       az.Config().Org,
		"bug_id":                    d.BugID,
		"bug_title":                 d.BugTitle,
		"bug_state":                 d.State,
		"team_project":              d.TeamProject,
		"area_path":                 d.AreaPath,
		"iteration_path":            d.IterationPath,
		"assigned_to":               d.AssignedTo,
		"suggested_title":           d.SuggestedTitle,
		"existing_correction_tasks": existing,
	}
	// The duplicate lookup is best-effort: a failed children read is a
	// warning for the UI, not a failed draft.
	if d.ExistingTasksError != "" {
		resp["existing_tasks_error"] = sanitizePublicError(errors.New(d.ExistingTasksError))
	}
	writeJSON(w, resp, nil)
}

// GET /api/azure/work-item-fields/subarea/allowed-values?team_project=X
func (srv *Server) handleGetSubareaAllowedValues(w http.ResponseWriter, r *http.Request) {
	project := strings.TrimSpace(r.URL.Query().Get("team_project"))
	if project == "" {
		writeAPIError(w, http.StatusBadRequest, "validation_error", map[string]any{"field": "team_project", "message": "team_project is required"})
		return
	}
	az := srv.azureClientOrUnavailable(w, r)
	if az == nil {
		return
	}
	values, err := az.FetchTaskFieldAllowedValues(r.Context(), project, azure.FieldSubarea)
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"field": azure.FieldSubarea, "values": values}, nil)
}

type bugCorrectionTaskBody struct {
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Subarea          string  `json:"subarea"`
	OriginalEstimate float64 `json:"original_estimate"`
	AreaPath         string  `json:"area_path"`
	IterationPath    string  `json:"iteration_path"`
	AssignedTo       string  `json:"assigned_to"`
	AddToCatalog     bool    `json:"add_to_catalog"`
	Project          *string `json:"project"`
	CategoryID       *int64  `json:"category_id"`
}

// firstInvalidField returns the first missing/invalid required field name,
// or "" when the body is valid. area_path is optional here: it falls back
// to the bug's own area.
func (b bugCorrectionTaskBody) firstInvalidField() (string, string) {
	switch {
	case strings.TrimSpace(b.Title) == "":
		return "title", "title is required"
	case strings.TrimSpace(b.Subarea) == "":
		return "subarea", "subarea is required"
	case b.OriginalEstimate <= 0:
		return "original_estimate", "original_estimate must be greater than 0"
	case strings.TrimSpace(b.IterationPath) == "":
		return "iteration_path", "iteration_path is required"
	case strings.TrimSpace(b.AssignedTo) == "":
		return "assigned_to", "assigned_to is required"
	}
	return "", ""
}

// POST /api/azure/bugs/{id}/correction-task
// Creates the bug's correction Task in the bug's own team project, linked to
// the bug as parent, and leaves it Proposed (no activation — matching what
// CMMI does). {id} is re-validated as a Bug server-side. With add_to_catalog
// the new Task is also registered in the Azure activity catalog with its
// parent bug; a catalog failure is reported as catalog_error, never as a
// lost work item.
func (srv *Server) handleCreateBugCorrectionTask(w http.ResponseWriter, r *http.Request) {
	id, ok := azureWorkItemID(w, r)
	if !ok {
		return
	}
	var body bugCorrectionTaskBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_body", map[string]any{"message": err.Error()})
		return
	}
	if field, msg := body.firstInvalidField(); field != "" {
		writeAPIError(w, http.StatusBadRequest, "validation_error", map[string]any{"field": field, "message": msg})
		return
	}

	az := srv.azureClientOrUnavailable(w, r)
	if az == nil {
		return
	}
	// Creating depends only on the bug itself (type, team project, area),
	// never on the duplicate lookup.
	bug := fetchBugDraftOrError(w, id, func() (*azure.BugCorrectionTaskDraft, error) {
		return az.FetchBugForCorrectionTask(r.Context(), id)
	})
	if bug == nil {
		return
	}

	areaPath := strings.TrimSpace(body.AreaPath)
	if areaPath == "" {
		areaPath = bug.AreaPath
	}
	title := strings.TrimSpace(body.Title)
	created, err := az.CreateBugCorrectionTask(r.Context(), azure.BugCorrectionTaskInput{
		BugID:            bug.BugID,
		TeamProject:      bug.TeamProject,
		Title:            title,
		Description:      body.Description,
		AreaPath:         areaPath,
		IterationPath:    body.IterationPath,
		AssignedTo:       body.AssignedTo,
		Subarea:          body.Subarea,
		OriginalEstimate: body.OriginalEstimate,
	})
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}

	resp := map[string]any{"id": created.ID, "state": created.State}
	if body.AddToCatalog {
		srv.catalogBugCorrectionTask(r, az, created.ID, title, bug, body, resp)
	}
	writeJSON(w, resp, nil)
}

// catalogBugCorrectionTask adds the created Task to the Azure activity
// catalog (type Task) and stores its parent bug, mutating resp with
// azure_activity_id or catalog_error. Same best-effort contract as
// registerAzureWorkItemCatalogEntry.
func (srv *Server) catalogBugCorrectionTask(r *http.Request, az *azure.Client, workItemID int, title string, bug *azure.BugCorrectionTaskDraft, body bugCorrectionTaskBody, resp map[string]any) {
	ctx := r.Context()
	a, err := srv.st.AddAzureActivity(ctx, az.Config().Org, workItemID, title, "Task", store.AzureActivityMapping{
		Project:    body.Project,
		CategoryID: body.CategoryID,
	})
	if err != nil {
		resp["catalog_error"] = sanitizePublicError(err)
		return
	}
	resp["azure_activity_id"] = a.ID
	// Best-effort: the entry exists; a failed parent or estimate write only
	// means they show up after the next states refresh instead.
	_ = srv.st.SyncAzureActivityParent(ctx, workItemID, bug.BugID, bug.BugTitle, bug.Type)
	_ = srv.st.SetAzureActivityEstimate(ctx, workItemID, body.OriginalEstimate)
}
