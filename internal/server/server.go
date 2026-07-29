package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Gentleman-Programming/mintag/internal/azure"
	"github.com/Gentleman-Programming/mintag/internal/parser"
	"github.com/Gentleman-Programming/mintag/internal/store"
	"github.com/Gentleman-Programming/mintag/internal/web"
)

type Server struct {
	st                    *store.Store
	newAzureTimeLogClient func(context.Context) (*azure.Client, error)
	newAzureOAuthClient   func(context.Context, azure.OAuthConfig) *azure.DeviceAuthClient
}

func New(st *store.Store) *Server {
	return &Server{
		st:                    st,
		newAzureTimeLogClient: st.NewAzureTimeLogClient,
		newAzureOAuthClient: func(_ context.Context, cfg azure.OAuthConfig) *azure.DeviceAuthClient {
			return azure.NewDeviceAuthClient(cfg)
		},
	}
}

func (srv *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(localCORSMiddleware)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/stats", srv.handleStats)
		r.Get("/search", srv.handleSearch)

		r.Route("/projects", func(r chi.Router) {
			r.Get("/", srv.handleListProjects)
			r.Post("/", srv.handleCreateProject)
		})

		r.Route("/meetings", func(r chi.Router) {
			r.Get("/", srv.handleListMeetings)
			r.Post("/import", srv.handleImportMeeting)
			r.Get("/{id}", srv.handleGetMeeting)
			r.Put("/{id}/rich-content", srv.handleSetMeetingRichContent)
			r.Put("/{id}/summary", srv.handleSetMeetingSummary)
		})

		r.Route("/tasks", func(r chi.Router) {
			r.Get("/", srv.handleListTasks)
			r.Post("/", srv.handleCreateTask)
			r.Put("/{id}", srv.handleUpdateTask)
			r.Get("/{id}", srv.handleGetTask)
			r.Get("/{id}/history", srv.handleTaskHistory)
		})

		srv.registerGraphRoutes(r)
		registerActivityRoutes(r, srv)
		registerDeploymentWindowRoutes(r, srv)
		registerMenuOptionRoutes(r, srv)
	})

	// SPA — serve embedded UI for everything else
	r.Handle("/*", web.Handler())

	return r
}

// --- Stats ---

func (srv *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := srv.st.GetStats()
	writeJSON(w, stats, err)
}

// --- Search ---

func (srv *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, []any{}, nil)
		return
	}
	results, err := srv.st.Search(q, 30)
	writeJSON(w, results, err)
}

// --- Projects ---

func (srv *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := srv.st.ListProjects()
	writeJSON(w, projects, err)
}

func (srv *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, err := srv.st.CreateProject(body.Name, body.Description, body.Color)
	writeJSON(w, p, err)
}

// --- Meetings ---

func (srv *Server) handleListMeetings(w http.ResponseWriter, r *http.Request) {
	var projectID *int64
	if v := r.URL.Query().Get("project_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			projectID = &id
		}
	}
	meetings, err := srv.st.ListMeetings(projectID)
	writeJSON(w, meetings, err)
}

func (srv *Server) handleImportMeeting(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path      string `json:"path"`
		ProjectID *int64 `json:"project_id"`
		Summary   string `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	raw, err := os.ReadFile(body.Path)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot read file: %v", err), http.StatusBadRequest)
		return
	}
	pm := parser.Parse(body.Path, string(raw))
	m, err := srv.st.CreateMeeting(body.ProjectID, body.Path, pm.Date, pm.Title, pm.Content, body.Summary)
	writeJSON(w, m, err)
}

func (srv *Server) handleGetMeeting(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m, err := srv.st.GetMeeting(id)
	writeJSON(w, m, err)
}

func (srv *Server) handleSetMeetingRichContent(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Content     string `json:"content"`
		ContentType string `json:"content_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.ContentType != "markdown" && body.ContentType != "html" {
		http.Error(w, "invalid content_type", http.StatusBadRequest)
		return
	}
	m, err := srv.st.UpdateMeetingRichContent(id, body.Content, body.ContentType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, m, nil)
}

func (srv *Server) handleSetMeetingSummary(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m, err := srv.st.UpdateMeetingSummary(id, body.Summary)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, m, nil)
}

// --- Tasks ---

func (srv *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	var projectID *int64
	if v := r.URL.Query().Get("project_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			projectID = &id
		}
	}
	status := r.URL.Query().Get("status")
	tasks, err := srv.st.ListTasks(projectID, status)
	writeJSON(w, tasks, err)
}

func (srv *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Owner       string `json:"owner"`
		Status      string `json:"status"`
		Priority    string `json:"priority"`
		DueDate     string `json:"due_date"`
		ProjectID   *int64 `json:"project_id"`
		MeetingID   *int64 `json:"meeting_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	t, err := srv.st.CreateTask(body.MeetingID, body.ProjectID, body.Title, body.Description,
		body.Status, body.Priority, body.Owner, body.DueDate)
	writeJSON(w, t, err)
}

func (srv *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	t, err := srv.st.GetTask(id)
	writeJSON(w, t, err)
}

func (srv *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Status          string `json:"status"`
		Owner           string `json:"owner"`
		Priority        string `json:"priority"`
		DueDate         string `json:"due_date"`
		Description     string `json:"description"`
		Note            string `json:"note"`
		Author          string `json:"author"`
		SourceMeetingID *int64 `json:"source_meeting_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	updates := map[string]any{}
	if body.Status != "" {
		updates["status"] = body.Status
	}
	if body.Owner != "" {
		updates["owner"] = body.Owner
	}
	if body.Priority != "" {
		updates["priority"] = body.Priority
	}
	if body.DueDate != "" {
		updates["due_date"] = body.DueDate
	}
	if body.Description != "" {
		updates["description"] = body.Description
	}
	t, err := srv.st.UpdateTask(id, updates, body.Note, body.Author, body.SourceMeetingID)
	writeJSON(w, t, err)
}

func (srv *Server) handleTaskHistory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	history, err := srv.st.GetTaskHistory(id)
	writeJSON(w, history, err)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, v any, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func localCORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isLocalOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			if origin != "" && !isLocalOrigin(origin) {
				http.Error(w, "origin is not allowed", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requireLocalRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && !isLocalOrigin(origin) {
			http.Error(w, "origin is not allowed", http.StatusForbidden)
			return
		}
		if !isLocalHost(r.Host) {
			http.Error(w, "host is not allowed", http.StatusForbidden)
			return
		}
		if r.RemoteAddr != "" {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err == nil && !isLoopbackHost(host) {
				http.Error(w, "remote address is not allowed", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLocalOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return isLocalHost(u.Host)
}

func isLocalHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return host == "localhost" || isLoopbackHost(host)
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func pathID(r *http.Request, param string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, param), 10, 64)
}
