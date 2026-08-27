package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Gentleman-Programming/mintag/internal/azure"
)

// registerBugEvidenceRoutes mounts the four DSW-PR-017 bug-evidence/comment
// endpoints under /api/azure/bugs/{id}. All four are guarded by
// requireLocalRequest — same trust boundary as the other Azure-credential-
// backed routes in this package (see activities.go's azure-config/
// azure-work-items routes).
//
// Routes:
//
//	GET   /api/azure/bugs/{id}/evidence
//	PATCH /api/azure/bugs/{id}/evidence
//	GET   /api/azure/bugs/{id}/comments
//	POST  /api/azure/bugs/{id}/comments
func registerBugEvidenceRoutes(r chi.Router, srv *Server) {
	r.Route("/azure/bugs/{id}", func(r chi.Router) {
		r.With(requireLocalRequest).Get("/evidence", srv.handleGetBugEvidence)
		r.With(requireLocalRequest).Patch("/evidence", srv.handlePatchBugEvidence)
		r.With(requireLocalRequest).Get("/comments", srv.handleListBugComments)
		r.With(requireLocalRequest).Post("/comments", srv.handlePostBugComment)
	})
}

// writeAzureNotConfigured writes the shared 503 body used by every
// Azure-credential-backed route in this package when no token is configured
// (see e.g. handleUploadActivities in activities.go).
func writeAzureNotConfigured(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(map[string]string{"error": "Azure TimeLog token is not configured"}) //nolint:errcheck
}

// writeAPIError writes a structured {"code": ...} error body (plus any extra
// fields) — the shape all bug-evidence/comment error responses use, as
// opposed to the plain-text http.Error used by most of the rest of this
// package. A distinct code lets the frontend branch on the exact failure
// reason (rev_conflict, state_not_editable, root_cause_required,
// insufficient_scope, not_a_bug) instead of parsing an error string.
func writeAPIError(w http.ResponseWriter, status int, code string, extra map[string]any) {
	body := map[string]any{"code": code}
	for k, v := range extra {
		body[k] = v
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

// isAzureNotFoundStatus reports whether err is a FetchBugEvidence failure
// whose underlying HTTP status was 404. FetchBugEvidence (unlike
// FetchWorkItemFull) does not special-case 404 into a (nil, nil) return — it
// always returns a wrapped azure.FetchBugEvidenceStatusError, which this
// checks via errors.As rather than matching error text: the error text
// echoes untrusted Azure response-body content (sanitizedResponseMessage)
// and could otherwise coincidentally contain a misleading status substring.
func isAzureNotFoundStatus(err error) bool {
	var statusErr *azure.FetchBugEvidenceStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound
}

// bugEvidenceFieldsResponse projects a *azure.BugEvidence's 4 evidence fields
// into the REST "fields" object shape shared by the GET and PATCH evidence
// responses.
func bugEvidenceFieldsResponse(ev *azure.BugEvidence) map[string]any {
	return map[string]any{
		"causa_raiz":              ev.CausaRaiz,
		"causa_raiz_identificada": ev.CausaRaizIdentificada,
		"solucion_definitiva":     ev.SolucionDefinitiva,
		"tipo_solucion":           ev.TipoSolucion,
	}
}

// changedBugEvidenceFields returns the subset of u's dirty (non-nil) fields
// whose submitted value does not match remote — used to build the
// rev_conflict 409's "changed_fields", so BugConflictModal only shows fields
// that actually diverged from what the client submitted, not every field the
// client happened to touch. This intentionally duplicates (rather than
// exports) internal/azure's unexported divergentFields: that function lives
// in the already-reviewed PR2 bug_evidence.go and this is a small,
// server-local read of already-exported BugEvidence/BugEvidenceUpdate
// fields.
func changedBugEvidenceFields(u azure.BugEvidenceUpdate, remote *azure.BugEvidence) map[string]any {
	changed := map[string]any{}
	if u.CausaRaiz != nil && *u.CausaRaiz != remote.CausaRaiz {
		changed["causa_raiz"] = remote.CausaRaiz
	}
	if u.CausaRaizIdentificada != nil && *u.CausaRaizIdentificada != remote.CausaRaizIdentificada {
		changed["causa_raiz_identificada"] = remote.CausaRaizIdentificada
	}
	if u.SolucionDefinitiva != nil && *u.SolucionDefinitiva != remote.SolucionDefinitiva {
		changed["solucion_definitiva"] = remote.SolucionDefinitiva
	}
	if u.TipoSolucion != nil && *u.TipoSolucion != remote.TipoSolucion {
		changed["tipo_solucion"] = remote.TipoSolucion
	}
	return changed
}

// GET /api/azure/bugs/{id}/evidence
func (srv *Server) handleGetBugEvidence(w http.ResponseWriter, r *http.Request) {
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
		writeAzureNotConfigured(w)
		return
	}

	ev, err := az.FetchBugEvidence(r.Context(), id)
	if err != nil {
		if isAzureNotFoundStatus(err) {
			http.Error(w, fmt.Sprintf("work item %d not found", id), http.StatusNotFound)
			return
		}
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	if ev.Type != "Bug" {
		writeAPIError(w, http.StatusBadRequest, "not_a_bug", nil)
		return
	}

	writeJSON(w, map[string]any{
		"id":           ev.ID,
		"rev":          ev.Rev,
		"state":        ev.State,
		"team_project": ev.TeamProject,
		"title":        ev.Title,
		"editable":     azure.IsBugEvidenceEditableState(ev.State),
		"fields":       bugEvidenceFieldsResponse(ev),
	}, nil)
}

// bugEvidencePatchBody is the PATCH /evidence request body: rev is the
// client's captured expected revision, and fields carries ONLY the fields
// the caller actually changed (nil = untouched, never sent to Azure) —
// mirrors azure.BugEvidenceUpdate exactly.
type bugEvidencePatchBody struct {
	Rev    int `json:"rev"`
	Fields struct {
		CausaRaiz             *string             `json:"causa_raiz"`
		CausaRaizIdentificada *bool               `json:"causa_raiz_identificada"`
		SolucionDefinitiva    *string             `json:"solucion_definitiva"`
		TipoSolucion          *azure.TipoSolucion `json:"tipo_solucion"`
	} `json:"fields"`
}

// PATCH /api/azure/bugs/{id}/evidence
// Client-side gating (editable state, root-cause invariant) is UX only —
// this handler re-checks both against a fresh server-side fetch before ever
// calling PatchBugEvidence, per the design's explicit trust boundary.
func (srv *Server) handlePatchBugEvidence(w http.ResponseWriter, r *http.Request) {
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
		writeAzureNotConfigured(w)
		return
	}

	var body bugEvidencePatchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	current, err := az.FetchBugEvidence(r.Context(), id)
	if err != nil {
		if isAzureNotFoundStatus(err) {
			http.Error(w, fmt.Sprintf("work item %d not found", id), http.StatusNotFound)
			return
		}
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	if current.Type != "Bug" {
		writeAPIError(w, http.StatusBadRequest, "not_a_bug", nil)
		return
	}
	if !azure.IsBugEvidenceEditableState(current.State) {
		writeAPIError(w, http.StatusConflict, "state_not_editable", map[string]any{"state": current.State})
		return
	}

	update := azure.BugEvidenceUpdate{
		CausaRaiz:             body.Fields.CausaRaiz,
		CausaRaizIdentificada: body.Fields.CausaRaizIdentificada,
		SolucionDefinitiva:    body.Fields.SolucionDefinitiva,
		TipoSolucion:          body.Fields.TipoSolucion,
	}

	// Root-cause-identified invariant, re-checked server-side: block
	// causa_raiz_identificada=true while the EFFECTIVE causa_raiz (the
	// submitted value if dirty, otherwise Azure's current value) is empty.
	if update.CausaRaizIdentificada != nil && *update.CausaRaizIdentificada {
		effectiveCausaRaiz := current.CausaRaiz
		if update.CausaRaiz != nil {
			effectiveCausaRaiz = *update.CausaRaiz
		}
		if strings.TrimSpace(effectiveCausaRaiz) == "" {
			writeAPIError(w, http.StatusUnprocessableEntity, "root_cause_required", nil)
			return
		}
	}

	ev, reaffirmed, err := az.PatchBugEvidence(r.Context(), id, body.Rev, update)
	if err != nil {
		switch {
		case errors.Is(err, azure.ErrRevConflict):
			remote, fetchErr := az.FetchBugEvidence(r.Context(), id)
			if fetchErr != nil {
				http.Error(w, sanitizePublicError(fetchErr), http.StatusBadGateway)
				return
			}
			writeAPIError(w, http.StatusConflict, "rev_conflict", map[string]any{
				"remote":         bugEvidenceFieldsResponse(remote),
				"changed_fields": changedBugEvidenceFields(update, remote),
			})
		case errors.Is(err, azure.ErrInsufficientScope):
			writeAPIError(w, http.StatusForbidden, "insufficient_scope", nil)
		default:
			http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		}
		return
	}

	writeJSON(w, map[string]any{
		"rev":        ev.Rev,
		"fields":     bugEvidenceFieldsResponse(ev),
		"reaffirmed": reaffirmed,
	}, nil)
}

// bugCommentResponse is the shared wire shape for one comment across both
// GET (list) and POST (create) responses.
type bugCommentResponse struct {
	ID          int64  `json:"id"`
	Text        string `json:"text"`
	CreatedBy   string `json:"created_by"`
	CreatedDate string `json:"created_date"`
}

// GET /api/azure/bugs/{id}/comments
// Resolves the Bug's own System.TeamProject via a fresh FetchBugEvidence
// call (simplest correct option — see the design's note that a per-request
// re-fetch here is acceptable rather than trying to cache/share it with the
// evidence panel's own fetch).
func (srv *Server) handleListBugComments(w http.ResponseWriter, r *http.Request) {
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
		writeAzureNotConfigured(w)
		return
	}

	ev, err := az.FetchBugEvidence(r.Context(), id)
	if err != nil {
		if isAzureNotFoundStatus(err) {
			http.Error(w, fmt.Sprintf("work item %d not found", id), http.StatusNotFound)
			return
		}
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}

	comments, err := az.ListBugComments(r.Context(), id, ev.TeamProject)
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}

	out := make([]bugCommentResponse, len(comments))
	for i, c := range comments {
		out[i] = bugCommentResponse{ID: c.ID, Text: c.Text, CreatedBy: c.CreatedBy, CreatedDate: c.CreatedDate}
	}
	writeJSON(w, out, nil)
}

// POST /api/azure/bugs/{id}/comments
// Re-checks editable state server-side (fresh fetch) before ever posting.
// Idempotency is enforced via store.BeginBugCommentUpload: a replayed
// idempotency_key whose row already reached status=posted short-circuits
// with the stored result, without a second Azure POST.
func (srv *Server) handlePostBugComment(w http.ResponseWriter, r *http.Request) {
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
		writeAzureNotConfigured(w)
		return
	}

	var body struct {
		IdempotencyKey string `json:"idempotency_key"`
		Text           string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.IdempotencyKey) == "" {
		http.Error(w, "idempotency_key is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}

	ev, err := az.FetchBugEvidence(r.Context(), id)
	if err != nil {
		if isAzureNotFoundStatus(err) {
			http.Error(w, fmt.Sprintf("work item %d not found", id), http.StatusNotFound)
			return
		}
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	if !azure.IsBugEvidenceEditableState(ev.State) {
		writeAPIError(w, http.StatusConflict, "state_not_editable", map[string]any{"state": ev.State})
		return
	}

	ctx := r.Context()
	uploadID, alreadyPosted, existingAzureCommentID, err := srv.st.BeginBugCommentUpload(ctx, id, body.IdempotencyKey, body.Text)
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusUnprocessableEntity)
		return
	}
	if alreadyPosted {
		writeJSON(w, bugCommentResponse{ID: existingAzureCommentID, Text: body.Text}, nil)
		return
	}

	comment, err := az.AddBugComment(ctx, id, ev.TeamProject, body.Text)
	if err != nil {
		_ = srv.st.MarkBugCommentFailed(ctx, uploadID, sanitizePublicError(err))
		if errors.Is(err, azure.ErrInsufficientScope) {
			writeAPIError(w, http.StatusForbidden, "insufficient_scope", nil)
			return
		}
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	if comment.ID == 0 {
		_ = srv.st.MarkBugCommentFailed(ctx, uploadID, "azure response missing comment id")
		http.Error(w, "azure response missing comment id", http.StatusBadGateway)
		return
	}
	if err := srv.st.MarkBugCommentPosted(ctx, uploadID, comment.ID); err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bugCommentResponse{ //nolint:errcheck
		ID:          comment.ID,
		Text:        comment.Text,
		CreatedBy:   comment.CreatedBy,
		CreatedDate: comment.CreatedDate,
	})
}
