package server

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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
//	GET  /api/activities             ?date=&status=
//	POST /api/activities
//	GET  /api/activities/catalog
//	POST /api/activities/upload      ?date=
//	PATCH /api/activities/{id}
func registerActivityRoutes(r chi.Router, srv *Server) {
	r.Get("/activities", srv.handleListActivities)
	r.Post("/activities", srv.handleCreateActivity)
	r.Get("/activities/catalog", srv.handleActivityCatalog)
	r.Post("/activities/upload", srv.handleUploadActivities)
	r.Patch("/activities/{id}", srv.handlePatchActivity)
}

// GET /api/activities?date=YYYY-MM-DD&status=
func (srv *Server) handleListActivities(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if !reActivityDate.MatchString(date) {
		http.Error(w, "date must be in YYYY-MM-DD format", http.StatusBadRequest)
		return
	}

	status := r.URL.Query().Get("status")
	activities, err := srv.st.ListActivities(r.Context(), date, status)
	writeJSON(w, activities, err)
}

// POST /api/activities
func (srv *Server) handleCreateActivity(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date           string  `json:"date"`
		Hours          float64 `json:"hours"`
		Project        string  `json:"project"`
		Category       string  `json:"category"`
		RegistroDiario string  `json:"registro_diario"`
		Source         string  `json:"source"`
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

	a, err := srv.st.CreateActivity(r.Context(), body.Date, body.Hours, body.Project, body.Category, body.RegistroDiario, body.Source)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(a)
}

// PATCH /api/activities/{id}
// Body is action-discriminated:
//
//	{"action":"approve"}   → ApproveActivities
//	{"action":"unapprove"} → UnapproveActivity
//	{other fields}         → update pending activity
func (srv *Server) handlePatchActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var body struct {
		Action         string  `json:"action"`
		Hours          float64 `json:"hours"`
		Project        string  `json:"project"`
		Category       string  `json:"category"`
		RegistroDiario string  `json:"registro_diario"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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
		a, err := srv.st.UpdateActivity(ctx, id, body.Hours, body.Project, body.Category, body.RegistroDiario)
		if err != nil {
			status := http.StatusUnprocessableEntity
			if isNotFound(err) {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
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

	if srv.az == nil || !srv.az.Enabled() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "MINTAG_AZURE_PAT not set"})
		return
	}

	result, err := srv.st.UploadActivities(r.Context(), date, srv.az)
	writeJSON(w, result, err)
}

// GET /api/activities/catalog
func (srv *Server) handleActivityCatalog(w http.ResponseWriter, r *http.Request) {
	projects, err := srv.st.ListTimelogProjects()
	if err != nil {
		writeJSON(w, nil, err)
		return
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
