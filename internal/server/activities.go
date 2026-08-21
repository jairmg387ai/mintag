package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Gentleman-Programming/mintag/internal/azure"
	"github.com/Gentleman-Programming/mintag/internal/store"
)

func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// reActivityDate validates YYYY-MM-DD query params.
var reActivityDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// registerActivityRoutes mounts all activity endpoints under /api/activities.
//
// Routes:
//
//	GET    /api/activities                           ?date=&status= or ?from=&to=&status=
//	POST   /api/activities
//	GET    /api/activities/azure-config
//	PUT    /api/activities/azure-config
//	DELETE /api/activities/azure-config
//	POST   /api/activities/azure-auth/device/start
//	POST   /api/activities/azure-auth/device/complete
//	GET    /api/activities/azure-work-items/assigned
//	POST   /api/activities/azure-work-items
//	GET    /api/activities/azure-work-items/states?ids=
//	POST   /api/activities/azure-work-items/{id}/close
//	POST   /api/activities/azure-work-items/{id}/recreate
//	GET    /api/activities/azure-classification-nodes/{kind}
//	GET    /api/activities/catalog
//	POST   /api/activities/catalog/projects
//	DELETE /api/activities/catalog/projects/{name}
//	POST   /api/activities/catalog/categories
//	DELETE /api/activities/catalog/categories/{name}
//	PUT    /api/activities/catalog/categories/{id}/description
//	GET    /api/activities/export                    ?from=&to=
//	GET    /api/activities/azure-catalog
//	POST   /api/activities/azure-catalog
//	PATCH  /api/activities/azure-catalog/{id}
//	DELETE /api/activities/azure-catalog/{id}
//	POST   /api/activities/azure-catalog/{id}/default
//	POST   /api/activities/upload                    ?date=
//	PATCH  /api/activities/{id}
//	DELETE /api/activities/{id}
//	GET    /api/settings/catalog-retention
//	PUT    /api/settings/catalog-retention
func registerActivityRoutes(r chi.Router, srv *Server) {
	r.Get("/activities", srv.handleListActivities)
	r.Post("/activities", srv.handleCreateActivity)
	r.With(requireLocalRequest).Get("/activities/azure-config", srv.handleGetAzureTimeLogConfig)
	r.With(requireLocalRequest).Put("/activities/azure-config", srv.handleSaveAzureTimeLogConfig)
	r.With(requireLocalRequest).Delete("/activities/azure-config", srv.handleClearAzureTimeLogConfig)
	r.With(requireLocalRequest).Post("/activities/azure-auth/device/start", srv.handleStartAzureDeviceAuth)
	r.With(requireLocalRequest).Post("/activities/azure-auth/device/complete", srv.handleCompleteAzureDeviceAuth)
	r.With(requireLocalRequest).Get("/activities/azure-work-items/assigned", srv.handleListAssignedAzureWorkItems)
	r.With(requireLocalRequest).Post("/activities/azure-work-items", srv.handleCreateAzureWorkItem)
	r.With(requireLocalRequest).Get("/activities/azure-work-items/states", srv.handleGetAzureWorkItemStates)
	r.With(requireLocalRequest).Post("/activities/azure-work-items/{id}/close", srv.handleCloseAzureWorkItem)
	r.With(requireLocalRequest).Post("/activities/azure-work-items/{id}/recreate", srv.handleRecreateAzureWorkItem)
	r.With(requireLocalRequest).Get("/activities/azure-classification-nodes/{kind}", srv.handleGetAzureClassificationTree)
	r.Get("/activities/catalog", srv.handleActivityCatalog)
	r.Post("/activities/catalog/projects", srv.handleAddCatalogProject)
	r.Delete("/activities/catalog/projects/{name}", srv.handleRemoveCatalogProject)
	r.Post("/activities/catalog/categories", srv.handleAddCatalogCategory)
	r.Delete("/activities/catalog/categories/{name}", srv.handleRemoveCatalogCategory)
	r.Put("/activities/catalog/categories/{id}/description", srv.handleUpdateCatalogCategoryDescription)
	r.Get("/activities/export", srv.handleExportActivities)
	// azure-catalog routes are grouped here, before /activities/{id}, to
	// match the reading order of /activities/catalog and /activities/upload
	// above (chi's radix tree already prioritizes static segments over
	// {id} regardless of registration order, so this is for readability,
	// not a routing requirement).
	r.Get("/activities/azure-catalog", srv.handleListAzureActivities)
	r.With(requireLocalRequest).Post("/activities/azure-catalog", srv.handleAddAzureActivity)
	r.With(requireLocalRequest).Patch("/activities/azure-catalog/{id}", srv.handleUpdateAzureActivity)
	r.With(requireLocalRequest).Delete("/activities/azure-catalog/{id}", srv.handleDeactivateAzureActivity)
	r.With(requireLocalRequest).Post("/activities/azure-catalog/{id}/default", srv.handleSetDefaultAzureActivity)
	r.With(requireLocalRequest).Post("/activities/upload", srv.handleUploadActivities)
	r.Patch("/activities/{id}", srv.handlePatchActivity)
	r.With(requireLocalRequest).Delete("/activities/{id}", srv.handleDeleteActivity)
	r.Get("/settings/catalog-retention", srv.handleGetCatalogRetention)
	r.With(requireLocalRequest).Put("/settings/catalog-retention", srv.handleSetCatalogRetention)
}

// GET /api/activities?date=YYYY-MM-DD&status=
// GET /api/activities?from=YYYY-MM-DD&to=YYYY-MM-DD&status=
func (srv *Server) handleListActivities(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from != "" || to != "" {
		activities, err := srv.st.ListActivitiesRange(r.Context(), from, to, status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, activities, nil)
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if !reActivityDate.MatchString(date) {
		http.Error(w, "date must be in YYYY-MM-DD format", http.StatusBadRequest)
		return
	}

	activities, err := srv.st.ListActivities(r.Context(), date, status)
	writeJSON(w, activities, err)
}

// POST /api/activities
func (srv *Server) handleCreateActivity(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date            string  `json:"date"`
		Hours           float64 `json:"hours"`
		Project         string  `json:"project"`
		Category        string  `json:"category"`
		RegistroDiario  string  `json:"registro_diario"`
		Source          string  `json:"source"`
		AzureActivityID *int64  `json:"azure_activity_id"`
		ReferenceID     *string `json:"reference_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if body.Date == "" {
		body.Date = time.Now().Format("2006-01-02")
	}
	if body.Source == "" {
		body.Source = "manual"
	}

	ctx := r.Context()
	a, err := srv.st.CreateActivity(ctx, body.Date, body.Hours, body.Project, body.Category, body.RegistroDiario, body.Source)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	// On create there's no prior state to preserve, so "field present in
	// JSON" and "pointer non-nil" agree — no need for raw-body presence
	// detection here (contrast with handlePatchActivity below).
	if a, err = srv.applyAzureActivityID(w, ctx, a, body.AzureActivityID, body.AzureActivityID != nil); err != nil {
		return // response already written by applyAzureActivityID
	}
	// Same reasoning as applyAzureActivityID above: on create there's no
	// prior state, so "field present in JSON" and "pointer non-nil" agree.
	if a, err = srv.applyReferenceID(w, ctx, a, body.ReferenceID, body.ReferenceID != nil); err != nil {
		return // response already written by applyReferenceID
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(a)
}

// applyAzureActivityID sets or clears the activity's azure_activity_id FK
// and reloads the row, so callers see the field populated in the response.
// provided distinguishes "the caller wants this field touched" (azureActivityID
// itself may still be nil, meaning "clear it") from "leave it as-is" — callers
// must compute this themselves, since a bare *int64 can't tell "omitted from
// the request" and "explicitly set to null" apart. Shared by
// handleCreateActivity and handlePatchActivity's default case. On error, it
// writes the HTTP response itself (422 for a rejected FK — e.g. missing or
// inactive azure activity — 500 for a reload failure) and returns a non-nil
// error so the caller can short-circuit without writing a second response.
func (srv *Server) applyAzureActivityID(w http.ResponseWriter, ctx context.Context, a *store.DailyActivity, azureActivityID *int64, provided bool) (*store.DailyActivity, error) {
	if !provided {
		return a, nil
	}
	if err := srv.st.SetActivityAzureActivity(ctx, a.ID, azureActivityID); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return nil, err
	}
	a, err := srv.st.GetActivity(ctx, a.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}
	return a, nil
}

// applyReferenceID sets or clears the activity's reference_id (an external
// Azure work item / Mantis ticket / LuxFlow reference) and reloads the row,
// mirroring applyAzureActivityID above. provided distinguishes "the caller
// wants this field touched" from "leave it as-is" — see applyAzureActivityID's
// doc comment for why a bare *string can't carry that distinction on its
// own. Shared by handleCreateActivity and handlePatchActivity's default
// case. On error, it writes the HTTP response itself (422 — though
// SetActivityReferenceID has no FK/format validation to fail, so this path
// is unlikely in practice; kept for consistency with applyAzureActivityID —
// 500 for a reload failure) and returns a non-nil error so the caller can
// short-circuit without writing a second response.
func (srv *Server) applyReferenceID(w http.ResponseWriter, ctx context.Context, a *store.DailyActivity, referenceID *string, provided bool) (*store.DailyActivity, error) {
	if !provided {
		return a, nil
	}
	if err := srv.st.SetActivityReferenceID(ctx, a.ID, referenceID); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return nil, err
	}
	a, err := srv.st.GetActivity(ctx, a.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}
	return a, nil
}

// PATCH /api/activities/{id}
// Body is action-discriminated:
//
//	{"action":"approve"}   → ApproveActivities
//	{"action":"unapprove"} → UnapproveActivity
//	{other fields}         → update pending or approved activity
func (srv *Server) handlePatchActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var body struct {
		Action          string  `json:"action"`
		Hours           float64 `json:"hours"`
		Project         string  `json:"project"`
		Category        string  `json:"category"`
		RegistroDiario  string  `json:"registro_diario"`
		AzureActivityID *int64  `json:"azure_activity_id"`
		ReferenceID     *string `json:"reference_id"`
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// A bare *int64 can't distinguish "azure_activity_id omitted" (leave
	// unchanged) from "azure_activity_id explicitly null" (clear it back to
	// the default) — both decode to a nil pointer. Check the raw JSON keys
	// instead so a PATCH that only edits e.g. hours never re-touches (and
	// re-validates) an unrelated, possibly since-deactivated FK.
	var rawFields map[string]json.RawMessage
	_ = json.Unmarshal(bodyBytes, &rawFields) // already validated above; re-parse can't fail
	_, azureActivityIDProvided := rawFields["azure_activity_id"]
	_, referenceIDProvided := rawFields["reference_id"]

	ctx := r.Context()
	switch body.Action {
	case "approve":
		n, err := srv.st.ApproveActivities(ctx, []int64{id})
		if err != nil {
			status := http.StatusUnprocessableEntity
			if isNotFound(err) {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, map[string]any{"approved": n}, nil)

	case "unapprove":
		if err := srv.st.UnapproveActivity(ctx, id); err != nil {
			status := http.StatusUnprocessableEntity
			if isNotFound(err) {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
		}
		a, err := srv.st.GetActivity(ctx, id)
		writeJSON(w, a, err)

	default:
		if body.Action != "" {
			http.Error(w, "unknown action: "+body.Action, http.StatusBadRequest)
			return
		}
		a, err := srv.st.UpdateActivity(ctx, id, body.Hours, body.Project, body.Category, body.RegistroDiario)
		if err != nil {
			status := http.StatusUnprocessableEntity
			if isNotFound(err) {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
		}
		if a, err = srv.applyAzureActivityID(w, ctx, a, body.AzureActivityID, azureActivityIDProvided); err != nil {
			return // response already written by applyAzureActivityID
		}
		if a, err = srv.applyReferenceID(w, ctx, a, body.ReferenceID, referenceIDProvided); err != nil {
			return // response already written by applyReferenceID
		}
		writeJSON(w, a, nil)
	}
}

// POST /api/activities/upload?date=YYYY-MM-DD
func (srv *Server) handleUploadActivities(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if !reActivityDate.MatchString(date) {
		http.Error(w, "date must be in YYYY-MM-DD format", http.StatusBadRequest)
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
		json.NewEncoder(w).Encode(map[string]string{"error": "Azure TimeLog token is not configured"})
		return
	}

	result, err := srv.st.UploadActivities(r.Context(), date, az)
	writeJSON(w, result, err)
}

// GET /api/activities/azure-config
func (srv *Server) handleGetAzureTimeLogConfig(w http.ResponseWriter, r *http.Request) {
	status, err := srv.st.AzureTimeLogConfigStatus(r.Context())
	writeJSON(w, status, err)
}

// PUT /api/activities/azure-config
func (srv *Server) handleSaveAzureTimeLogConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		AuthMode string `json:"auth_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	status, err := srv.st.SaveAzureTimeLogConfig(r.Context(), body.Token, body.AuthMode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	status = srv.resolveAzureIdentity(r.Context(), status)
	writeJSON(w, status, nil)
}

// resolveAzureIdentity fetches the Azure identity behind the currently
// configured token and persists it, so uploads stop attributing to whatever
// hardcoded/env fallback identity happened to be set. Best-effort: a failure
// here (e.g. token valid for TimeLog but the org denies connectiondata) must
// not fail the credential save — it just leaves uploads blocked until
// FetchIdentity succeeds, which PostTimeEntry already enforces.
func (srv *Server) resolveAzureIdentity(ctx context.Context, status *store.AzureTimeLogConfigStatus) *store.AzureTimeLogConfigStatus {
	client, err := srv.newAzureTimeLogClient(ctx)
	if err != nil {
		return status
	}
	userID, displayName, err := client.FetchIdentity(ctx)
	if err != nil {
		return status
	}
	if err := srv.st.SaveAzureIdentity(ctx, userID, displayName); err != nil {
		return status
	}
	if refreshed, err := srv.st.AzureTimeLogConfigStatus(ctx); err == nil {
		return refreshed
	}
	return status
}

// DELETE /api/activities/azure-config
func (srv *Server) handleClearAzureTimeLogConfig(w http.ResponseWriter, r *http.Request) {
	status, err := srv.st.ClearAzureTimeLogConfig(r.Context())
	writeJSON(w, status, err)
}

// POST /api/activities/azure-auth/device/start
func (srv *Server) handleStartAzureDeviceAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Tenant   string `json:"tenant"`
		ClientID string `json:"client_id"`
		Scope    string `json:"scope"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	cfg := srv.st.AzureOAuthConfig(r.Context())
	if strings.TrimSpace(body.Tenant) != "" {
		cfg.Tenant = strings.TrimSpace(body.Tenant)
	}
	if strings.TrimSpace(body.ClientID) != "" {
		cfg.ClientID = strings.TrimSpace(body.ClientID)
	}
	if strings.TrimSpace(body.Scope) != "" {
		cfg.Scope = strings.TrimSpace(body.Scope)
	}
	client := srv.newAzureOAuthClient(r.Context(), cfg)
	device, err := client.StartDeviceCode(r.Context())
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	if err := srv.st.SaveAzureOAuthConfig(r.Context(), client.Config()); err != nil {
		writeJSON(w, nil, err)
		return
	}
	writeJSON(w, map[string]any{
		"device_code":      device.DeviceCode,
		"user_code":        device.UserCode,
		"verification_uri": device.VerificationURI,
		"expires_in":       device.ExpiresIn,
		"interval":         device.Interval,
		"message":          device.Message,
	}, nil)
}

// POST /api/activities/azure-auth/device/complete
func (srv *Server) handleCompleteAzureDeviceAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.DeviceCode) == "" {
		http.Error(w, "device_code is required", http.StatusBadRequest)
		return
	}
	cfg := srv.st.AzureOAuthConfig(r.Context())
	client := srv.newAzureOAuthClient(r.Context(), cfg)
	token, status, err := client.PollDeviceCode(r.Context(), body.DeviceCode)
	if errors.Is(err, azure.ErrAuthorizationPending) {
		writeJSON(w, map[string]string{"status": azure.DeviceAuthStatusPending}, nil)
		return
	}
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	if status != azure.DeviceAuthStatusComplete {
		writeJSON(w, map[string]string{"status": status}, nil)
		return
	}
	if err := srv.st.SaveAzureOAuthTokens(r.Context(), token, client.Config()); err != nil {
		writeJSON(w, nil, err)
		return
	}
	srv.resolveAzureIdentity(r.Context(), nil)
	writeJSON(w, map[string]string{"status": azure.DeviceAuthStatusComplete}, nil)
}

// GET /api/activities/azure-work-items/assigned
// Lists open work items assigned to the identity behind the configured
// Azure credential, so they can be picked into the Azure activity catalog
// instead of typed in by hand.
func (srv *Server) handleListAssignedAzureWorkItems(w http.ResponseWriter, r *http.Request) {
	az, err := srv.newAzureTimeLogClient(r.Context())
	if err != nil {
		writeJSON(w, nil, err)
		return
	}
	if az == nil || !az.Enabled() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Azure TimeLog token is not configured"})
		return
	}
	items, err := az.FetchAssignedWorkItems(r.Context())
	if err != nil {
		http.Error(w, sanitizePublicError(err), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"org": az.Config().Org, "items": items}, nil)
}

func sanitizePublicError(err error) string {
	msg := strings.TrimSpace(err.Error())
	msg = strings.ReplaceAll(msg, "\r", " ")
	msg = strings.ReplaceAll(msg, "\n", " ")
	if len(msg) > 300 {
		msg = msg[:300] + "..."
	}
	return msg
}

// catalogProjectEntry is the /api/activities/catalog "projects" element
// shape. It replaced a bare []string so the frontend can tell active from
// inactive projects apart when ?include_inactive=true is requested — see
// handleActivityCatalog below. This is a JSON response shape change: any
// existing frontend code expecting `projects: string[]` needs updating to
// `projects: {name, is_active}[]`.
type catalogProjectEntry struct {
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// GET /api/activities/catalog?include_inactive=true
func (srv *Server) handleActivityCatalog(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	ctx := r.Context()

	names, err := srv.st.ListTimelogProjects(ctx, includeInactive)
	if err != nil {
		writeJSON(w, nil, err)
		return
	}

	projects := make([]catalogProjectEntry, len(names))
	if includeInactive {
		// A second, active-only query is the simplest way to tell which of
		// the (possibly larger) includeInactive set are still active,
		// without widening ListTimelogProjects' return shape beyond []string.
		activeNames, err := srv.st.ListTimelogProjects(ctx, false)
		if err != nil {
			writeJSON(w, nil, err)
			return
		}
		active := make(map[string]bool, len(activeNames))
		for _, n := range activeNames {
			active[n] = true
		}
		for i, n := range names {
			projects[i] = catalogProjectEntry{Name: n, IsActive: active[n]}
		}
	} else {
		for i, n := range names {
			projects[i] = catalogProjectEntry{Name: n, IsActive: true}
		}
	}

	categories, err := srv.st.ListTimelogCategories()
	if err != nil {
		writeJSON(w, nil, err)
		return
	}

	writeJSON(w, map[string]any{
		"projects":   projects,
		"categories": categories,
	}, nil)
}

// PUT /api/activities/catalog/categories/{id}/description
// Body: {"description": "..."}
func (srv *Server) handleUpdateCatalogCategoryDescription(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c, err := srv.st.UpdateTimelogCategoryDescription(r.Context(), id, body.Description)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, c, nil)
}

// DELETE /api/activities/{id}
func (srv *Server) handleDeleteActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := srv.st.DeleteActivityWithAzure(r.Context(), id, func(ctx context.Context) (store.TimeEntryDeleter, error) {
		return srv.newAzureTimeLogClient(ctx)
	}); err != nil {
		status := http.StatusUnprocessableEntity
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/activities/catalog/projects
func (srv *Server) handleAddCatalogProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := srv.st.AddTimelogProject(body.Name); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]string{"name": body.Name}, nil)
}

// DELETE /api/activities/catalog/projects/{name}
// Soft-deletes (is_active=0) rather than hard-deleting, so historical
// daily_activities.project references stay intact — see
// DeactivateTimelogProject's doc comment.
func (srv *Server) handleRemoveCatalogProject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := srv.st.DeactivateTimelogProject(r.Context(), name); err != nil {
		status := http.StatusInternalServerError
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/activities/catalog/categories
func (srv *Server) handleAddCatalogCategory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := srv.st.AddTimelogCategory(body.Name, body.Description); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]string{"name": body.Name}, nil)
}

// DELETE /api/activities/catalog/categories/{name}
func (srv *Server) handleRemoveCatalogCategory(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := srv.st.RemoveTimelogCategory(name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/activities/azure-catalog?include_inactive=true
func (srv *Server) handleListAzureActivities(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	activities, err := srv.st.ListAzureActivities(r.Context(), includeInactive)
	writeJSON(w, activities, err)
}

// POST /api/activities/azure-catalog
func (srv *Server) handleAddAzureActivity(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Org          string  `json:"org"`
		WorkItemID   int     `json:"work_item_id"`
		Label        string  `json:"label"`
		WorkItemType string  `json:"work_item_type"`
		Project      *string `json:"project"`
		CategoryID   *int64  `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mapping := store.AzureActivityMapping{Project: body.Project, CategoryID: body.CategoryID}
	a, err := srv.st.AddAzureActivity(r.Context(), body.Org, body.WorkItemID, body.Label, body.WorkItemType, mapping)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(a)
}

// PATCH /api/activities/azure-catalog/{id}
// Despite the verb, this is a full replace, not a partial update: org and
// label are both required (see UpdateAzureActivity), so a caller renaming
// only one field must still send the other's current value. The same
// full-replace contract applies to project/category_id: an omitted field
// clears any previously-stored mapping value (see UpdateAzureActivity's doc
// comment) — callers must resend current values to preserve them.
func (srv *Server) handleUpdateAzureActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Org          string  `json:"org"`
		Label        string  `json:"label"`
		WorkItemType string  `json:"work_item_type"`
		Project      *string `json:"project"`
		CategoryID   *int64  `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mapping := store.AzureActivityMapping{Project: body.Project, CategoryID: body.CategoryID}
	a, err := srv.st.UpdateAzureActivity(r.Context(), id, body.Org, body.Label, body.WorkItemType, mapping)
	if err != nil {
		// "azure activity not found" (row missing) -> 404. Any other error,
		// including validateMapping's "timelog category not found" (an
		// invalid category_id, not a missing azure activity), maps to 422 —
		// both messages contain "not found", so the generic isNotFound()
		// substring check used elsewhere in this file cannot distinguish
		// them and would otherwise mis-route a rejected mapping as a 404.
		status := http.StatusUnprocessableEntity
		if strings.Contains(err.Error(), "azure activity not found") {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, a, nil)
}

// DELETE /api/activities/azure-catalog/{id}
func (srv *Server) handleDeactivateAzureActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := srv.st.DeactivateAzureActivity(r.Context(), id); err != nil {
		status := http.StatusUnprocessableEntity
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/activities/azure-catalog/{id}/default
func (srv *Server) handleSetDefaultAzureActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if err := srv.st.SetDefaultAzureActivity(ctx, id); err != nil {
		status := http.StatusUnprocessableEntity
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	a, err := srv.st.GetDefaultAzureActivity(ctx)
	writeJSON(w, a, err)
}

// catalogRetentionResponse is the shared response shape for both the GET and
// PUT catalog-retention endpoints. A nil field means that catalog's
// automatic staleness deactivation is unset/disabled.
type catalogRetentionResponse struct {
	BugRetentionDays     *int `json:"bug_retention_days"`
	ProjectRetentionDays *int `json:"project_retention_days"`
}

// GET /api/settings/catalog-retention
func (srv *Server) handleGetCatalogRetention(w http.ResponseWriter, r *http.Request) {
	bugDays, projectDays, err := srv.st.GetCatalogRetentionDays(r.Context())
	writeJSON(w, catalogRetentionResponse{BugRetentionDays: bugDays, ProjectRetentionDays: projectDays}, err)
}

// PUT /api/settings/catalog-retention
// Body: {"bug_retention_days": <int|null>, "project_retention_days": <int|null>}
// A null (or omitted) field disables automatic deactivation for that
// catalog; a positive integer configures its retention window in days.
func (srv *Server) handleSetCatalogRetention(w http.ResponseWriter, r *http.Request) {
	var body catalogRetentionResponse
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if err := srv.st.SetCatalogRetentionDays(ctx, body.BugRetentionDays, body.ProjectRetentionDays); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	bugDays, projectDays, err := srv.st.GetCatalogRetentionDays(ctx)
	writeJSON(w, catalogRetentionResponse{BugRetentionDays: bugDays, ProjectRetentionDays: projectDays}, err)
}
