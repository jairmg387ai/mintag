package server

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// registerGraphRoutes mounts all read-only graph endpoints under /api/graph.
//
// Routes:
//
//	GET /api/graph/search         ?q=&namespace=&kind=&limit=
//	GET /api/graph/nodes/{id}     full node + neighbors grouped by relation
//	GET /api/graph/nodes          ?namespace=&kind=&key= (lookup by natural key; key omitted lists by kind)
//	GET /api/graph/nodes/{id}/neighbors  ?relation=&direction=&limit=
//	GET /api/graph/nodes/{id}/impact     ?max_depth=&limit=&kind_filter=
//	GET /api/graph/stats          ?namespace=
//	GET /api/graph/namespaces
func (srv *Server) registerGraphRoutes(r chi.Router) {
	r.Route("/graph", func(r chi.Router) {
		r.Get("/search", srv.handleGraphSearch)
		r.Get("/nodes", srv.handleGraphNodeByKey)
		r.Get("/nodes/{id}", srv.handleGraphNodeByID)
		r.Get("/nodes/{id}/neighbors", srv.handleGraphNeighbors)
		r.Get("/nodes/{id}/impact", srv.handleGraphImpact)
		r.Get("/stats", srv.handleGraphStats)
		r.Get("/namespaces", srv.handleGraphNamespaces)
	})
}

// --- helpers ---

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// --- handlers ---

// GET /api/graph/search?q=&namespace=&kind=&limit=
func (srv *Server) handleGraphSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, []any{}, nil)
		return
	}
	namespace := r.URL.Query().Get("namespace")
	kind := r.URL.Query().Get("kind")
	limit := queryInt(r, "limit", 20)

	results, err := srv.st.SearchGraphNodes(namespace, q, kind, limit)
	writeJSON(w, results, err)
}

// GET /api/graph/nodes?namespace=&kind=&key=
// Looks up a single node by its natural key. When key is omitted, lists all
// nodes of the given kind (tree explorer roots).
func (srv *Server) handleGraphNodeByKey(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	kind := r.URL.Query().Get("kind")
	key := r.URL.Query().Get("key")

	if kind == "" {
		http.Error(w, "kind is required", http.StatusBadRequest)
		return
	}

	if key == "" {
		limit := queryInt(r, "limit", 100)
		nodes, err := srv.st.ListGraphNodesByKind(namespace, kind, limit)
		writeJSON(w, map[string]any{"nodes": nodes, "count": len(nodes)}, err)
		return
	}

	node, err := srv.st.GetGraphNodeByKey(namespace, kind, key)
	if err == sql.ErrNoRows {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	if err != nil {
		// Ambiguous natural key (multiple namespaces) is client-fixable:
		// retry with an explicit namespace.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, node, nil)
}

// GET /api/graph/nodes/{id}
// Returns the node plus its 1-hop neighbors, grouped by direction:relation.
func (srv *Server) handleGraphNodeByID(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node, err := srv.st.GetGraphNode(id)
	if err == sql.ErrNoRows {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	limit := queryInt(r, "neighbor_limit", 100)
	neighbors, err := srv.st.GraphNeighbors(node.ID, "", "", limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// group by "direction:relation" matching the MCP tool convention
	type groupKey = string
	grouped := map[groupKey]any{}
	for _, nb := range neighbors {
		k := nb.Direction + ":" + nb.Relation
		existing, ok := grouped[k]
		if !ok {
			grouped[k] = []any{nb}
		} else {
			grouped[k] = append(existing.([]any), nb)
		}
	}

	writeJSON(w, map[string]any{
		"node":           node,
		"relations":      grouped,
		"neighbor_count": len(neighbors),
	}, nil)
}

// GET /api/graph/nodes/{id}/neighbors?relation=&direction=&limit=
func (srv *Server) handleGraphNeighbors(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Verify node exists; return 404 rather than empty list for unknown IDs.
	if _, err := srv.st.GetGraphNode(id); err == sql.ErrNoRows {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	relation := r.URL.Query().Get("relation")
	direction := r.URL.Query().Get("direction")
	limit := queryInt(r, "limit", 200)

	neighbors, err := srv.st.GraphNeighbors(id, relation, direction, limit)
	writeJSON(w, map[string]any{"neighbors": neighbors, "count": len(neighbors)}, err)
}

// GET /api/graph/nodes/{id}/impact?max_depth=&limit=&kind_filter=
// Returns a slim payload (no summary/attrs) identical to the MCP tool.
func (srv *Server) handleGraphImpact(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node, err := srv.st.GetGraphNode(id)
	if err == sql.ErrNoRows {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	maxDepth := queryInt(r, "max_depth", 5)
	limit := queryInt(r, "limit", 100)
	kindFilter := r.URL.Query().Get("kind_filter")

	rows, err := srv.st.GraphImpact(node.ID, maxDepth, []string{"belongs_to"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type slimRow struct {
		Depth int    `json:"depth"`
		Kind  string `json:"kind"`
		Key   string `json:"key"`
		Label string `json:"label"`
	}

	byKind := map[string]int{}
	listed := make([]slimRow, 0, limit)
	for _, row := range rows {
		byKind[row.Node.Kind]++
		if kindFilter != "" && row.Node.Kind != kindFilter {
			continue
		}
		if len(listed) < limit {
			listed = append(listed, slimRow{row.Depth, row.Node.Kind, row.Node.Key, row.Node.Label})
		}
	}

	writeJSON(w, map[string]any{
		"node":             map[string]any{"id": node.ID, "kind": node.Kind, "key": node.Key, "label": node.Label},
		"impacted_count":   len(rows),
		"impacted_by_kind": byKind,
		"impacted":         listed,
		"truncated":        len(rows) > len(listed),
	}, nil)
}

// GET /api/graph/stats?namespace=
func (srv *Server) handleGraphStats(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")

	// Filter by comma-separated namespaces only if explicitly given; otherwise
	// treat empty namespace as "all".
	stats, err := srv.st.GetGraphStats(namespace)
	if err != nil {
		writeJSON(w, nil, err)
		return
	}
	namespaces, _ := srv.st.GraphNamespaces()

	// Normalise nil slices to empty arrays for clean JSON.
	if namespaces == nil {
		namespaces = []string{}
	}

	writeJSON(w, map[string]any{
		"stats":      stats,
		"namespaces": namespaces,
	}, nil)
}

// GET /api/graph/namespaces
func (srv *Server) handleGraphNamespaces(w http.ResponseWriter, r *http.Request) {
	ns, err := srv.st.GraphNamespaces()
	if ns == nil {
		ns = []string{}
	}
	writeJSON(w, ns, err)
}
