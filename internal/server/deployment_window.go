package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerDeploymentWindowRoutes mounts all deployment-window endpoints under /api/deployment-windows.
//
// Routes:
//
//	GET    /api/deployment-windows                               ?state=
//	POST   /api/deployment-windows
//	GET    /api/deployment-windows/{id}
//	PATCH  /api/deployment-windows/{id}/state
//	GET    /api/deployment-windows/{id}/export
//	POST   /api/deployment-windows/{id}/tasks
//	DELETE /api/deployment-windows/{id}/tasks/{task_id}
//	POST   /api/deployment-windows/{id}/repos
//	PATCH  /api/deployment-windows/{id}/repos/{repo_id}
//	DELETE /api/deployment-windows/{id}/repos/{repo_id}
//	POST   /api/deployment-windows/{id}/artifacts
//	PATCH  /api/deployment-windows/{id}/artifacts/{artifact_id}
//	DELETE /api/deployment-windows/{id}/artifacts/{artifact_id}
//	POST   /api/deployment-windows/{id}/test-scenarios
//	PATCH  /api/deployment-windows/{id}/test-scenarios/{scenario_id}
//	DELETE /api/deployment-windows/{id}/test-scenarios/{scenario_id}
//	PATCH  /api/deployment-windows/{id}/test-scenarios/{scenario_id}/sign-off
func registerDeploymentWindowRoutes(r chi.Router, srv *Server) {
	r.Route("/deployment-windows", func(r chi.Router) {
		r.Get("/", srv.handleListDWs)
		r.Post("/", srv.handleCreateDW)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", srv.handleGetDW)
			r.Patch("/state", srv.handleUpdateDWState)
			r.Get("/export", srv.handleExportDW)

			r.Post("/tasks", srv.handleAddDWTask)
			r.Delete("/tasks/{task_id}", srv.handleRemoveDWTask)

			r.Post("/repos", srv.handleAddDWRepo)
			r.Patch("/repos/{repo_id}", srv.handleUpdateDWRepo)
			r.Delete("/repos/{repo_id}", srv.handleRemoveDWRepo)

			r.Post("/artifacts", srv.handleAddDWArtifact)
			r.Patch("/artifacts/{artifact_id}", srv.handleUpdateDWArtifact)
			r.Delete("/artifacts/{artifact_id}", srv.handleRemoveDWArtifact)

			r.Post("/test-scenarios", srv.handleAddDWTestScenario)
			r.Patch("/test-scenarios/{scenario_id}", srv.handleUpdateDWTestScenario)
			r.Delete("/test-scenarios/{scenario_id}", srv.handleRemoveDWTestScenario)
			r.Patch("/test-scenarios/{scenario_id}/sign-off", srv.handleSignOffScenario)
		})
	})
}

// GET /api/deployment-windows?state=
func (srv *Server) handleListDWs(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	dws, err := srv.st.ListDeploymentWindows(state)
	writeJSON(w, dws, err)
}

// POST /api/deployment-windows
func (srv *Server) handleCreateDW(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		CreatedBy   string `json:"created_by"`
		PlannedAt   string `json:"planned_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dw, err := srv.st.CreateDeploymentWindow(body.Title, body.Description, body.CreatedBy, body.PlannedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(dw)
}

// GET /api/deployment-windows/{id}
func (srv *Server) handleGetDW(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	detail, err := srv.st.GetDeploymentWindow(id)
	writeJSON(w, detail, err)
}

// PATCH /api/deployment-windows/{id}/state
func (srv *Server) handleUpdateDWState(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		State         string `json:"state"`
		RejectionNote string `json:"rejection_note"`
		Namespace     string `json:"namespace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dw, err := srv.st.UpdateDeploymentWindowState(id, body.State, body.RejectionNote, body.Namespace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, dw, nil)
}

// GET /api/deployment-windows/{id}/export
func (srv *Server) handleExportDW(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	md, err := srv.st.ExportDeploymentWindowMarkdown(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="ventana-%d.md"`, id))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(md)) //nolint:errcheck
}

// POST /api/deployment-windows/{id}/tasks
func (srv *Server) handleAddDWTask(w http.ResponseWriter, r *http.Request) {
	dwID, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		TaskID int64  `json:"task_id"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := srv.st.AddDWTask(dwID, body.TaskID, body.Note); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]any{"dw_id": dwID, "task_id": body.TaskID, "note": body.Note}, nil)
}

// DELETE /api/deployment-windows/{id}/tasks/{task_id}
func (srv *Server) handleRemoveDWTask(w http.ResponseWriter, r *http.Request) {
	dwID, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	taskID, err := pathID(r, "task_id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := srv.st.RemoveDWTask(dwID, taskID); err != nil {
		status := http.StatusUnprocessableEntity
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/deployment-windows/{id}/repos
func (srv *Server) handleAddDWRepo(w http.ResponseWriter, r *http.Request) {
	dwID, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		GraphNodeKey string `json:"graph_node_key"`
		Version      string `json:"version"`
		Notes        string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	repo, err := srv.st.AddDWRepo(dwID, body.GraphNodeKey, body.Version, body.Notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(repo)
}

// PATCH /api/deployment-windows/{id}/repos/{repo_id}
func (srv *Server) handleUpdateDWRepo(w http.ResponseWriter, r *http.Request) {
	repoID, err := pathID(r, "repo_id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Version string `json:"version"`
		Notes   string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	repo, err := srv.st.UpdateDWRepo(repoID, body.Version, body.Notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, repo, nil)
}

// DELETE /api/deployment-windows/{id}/repos/{repo_id}
func (srv *Server) handleRemoveDWRepo(w http.ResponseWriter, r *http.Request) {
	repoID, err := pathID(r, "repo_id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := srv.st.RemoveDWRepo(repoID); err != nil {
		status := http.StatusUnprocessableEntity
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/deployment-windows/{id}/artifacts
func (srv *Server) handleAddDWArtifact(w http.ResponseWriter, r *http.Request) {
	dwID, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Kind    string `json:"kind"`
		Name    string `json:"name"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	artifact, err := srv.st.AddDWArtifact(dwID, body.Kind, body.Name, body.Path, body.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(artifact)
}

// PATCH /api/deployment-windows/{id}/artifacts/{artifact_id}
func (srv *Server) handleUpdateDWArtifact(w http.ResponseWriter, r *http.Request) {
	artifactID, err := pathID(r, "artifact_id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Kind    string `json:"kind"`
		Name    string `json:"name"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	artifact, err := srv.st.UpdateDWArtifact(artifactID, body.Kind, body.Name, body.Path, body.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, artifact, nil)
}

// DELETE /api/deployment-windows/{id}/artifacts/{artifact_id}
func (srv *Server) handleRemoveDWArtifact(w http.ResponseWriter, r *http.Request) {
	artifactID, err := pathID(r, "artifact_id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := srv.st.RemoveDWArtifact(artifactID); err != nil {
		status := http.StatusUnprocessableEntity
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/deployment-windows/{id}/test-scenarios
func (srv *Server) handleAddDWTestScenario(w http.ResponseWriter, r *http.Request) {
	dwID, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Expected    string `json:"expected"`
		SortOrder   int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sc, err := srv.st.AddDWTestScenario(dwID, body.Title, body.Description, body.Expected, body.SortOrder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sc)
}

// PATCH /api/deployment-windows/{id}/test-scenarios/{scenario_id}
func (srv *Server) handleUpdateDWTestScenario(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := pathID(r, "scenario_id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Expected    string `json:"expected"`
		SortOrder   int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sc, err := srv.st.UpdateDWTestScenario(scenarioID, body.Title, body.Description, body.Expected, body.SortOrder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, sc, nil)
}

// DELETE /api/deployment-windows/{id}/test-scenarios/{scenario_id}
func (srv *Server) handleRemoveDWTestScenario(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := pathID(r, "scenario_id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := srv.st.RemoveDWTestScenario(scenarioID); err != nil {
		status := http.StatusUnprocessableEntity
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PATCH /api/deployment-windows/{id}/test-scenarios/{scenario_id}/sign-off
func (srv *Server) handleSignOffScenario(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := pathID(r, "scenario_id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		Result      string `json:"result"`
		SignedOffBy string `json:"signed_off_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sc, err := srv.st.SignOffTestScenario(scenarioID, body.Result, body.SignedOffBy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, sc, nil)
}
