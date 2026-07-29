package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// registerMenuOptionRoutes mounts the configurable sidebar menu options
// endpoints under /api/menu-options.
//
// Routes:
//
//	GET   /api/menu-options
//	PATCH /api/menu-options/{id}
func registerMenuOptionRoutes(r chi.Router, srv *Server) {
	r.Get("/menu-options", srv.handleListMenuOptions)
	r.With(requireLocalRequest).Patch("/menu-options/{id}", srv.handleSetMenuOptionEnabled)
}

// GET /api/menu-options
func (srv *Server) handleListMenuOptions(w http.ResponseWriter, r *http.Request) {
	options, err := srv.st.ListMenuOptions(r.Context())
	writeJSON(w, options, err)
}

// PATCH /api/menu-options/{id}
// Body: {"enabled": true|false}
func (srv *Server) handleSetMenuOptionEnabled(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Enabled == nil {
		http.Error(w, "enabled is required", http.StatusBadRequest)
		return
	}

	opt, err := srv.st.SetMenuOptionEnabled(r.Context(), id, *body.Enabled)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrMenuOptionNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, store.ErrLastEnabledMenuOption):
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, opt, nil)
}
