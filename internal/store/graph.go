package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// The knowledge graph stores heterogeneous nodes (repos, portals, menu
// options, use cases, ...) and typed edges between them, scoped by namespace
// so multiple projects can coexist in one database.
//
// Edge direction convention: source DEPENDS ON target. A portal exposes a
// microfront (portal -> MR), a frontend consumes a backend (MR -> MS), a
// service uses a client library (MS -> ApiClient). Impact analysis walks
// edges in reverse: everything transitively pointing at a node is affected
// when that node changes.

type GraphNode struct {
	ID        int64          `json:"id"`
	Namespace string         `json:"namespace"`
	Kind      string         `json:"kind"`
	Key       string         `json:"key"`
	Label     string         `json:"label"`
	Summary   string         `json:"summary,omitempty"`
	Attrs     map[string]any `json:"attrs,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type GraphNeighbor struct {
	Relation  string         `json:"relation"`
	Direction string         `json:"direction"` // "out" (node depends on neighbor) | "in" (neighbor depends on node)
	EdgeAttrs map[string]any `json:"edge_attrs,omitempty"`
	Node      *GraphNode     `json:"node"`
}

type GraphImpactRow struct {
	Depth int        `json:"depth"`
	Node  *GraphNode `json:"node"`
}

type GraphNodeSearchResult struct {
	Node    *GraphNode `json:"node"`
	Snippet string     `json:"snippet"`
	Score   float64    `json:"score"`
}

type GraphStats struct {
	Namespace  string         `json:"namespace"`
	NodeCount  int            `json:"node_count"`
	EdgeCount  int            `json:"edge_count"`
	NodesByKind map[string]int `json:"nodes_by_kind"`
	EdgesByRelation map[string]int `json:"edges_by_relation"`
}

func (s *Store) migrateGraph() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS graph_nodes (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		namespace  TEXT NOT NULL,
		kind       TEXT NOT NULL,
		key        TEXT NOT NULL,
		label      TEXT NOT NULL,
		summary    TEXT NOT NULL DEFAULT '',
		attrs      TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(namespace, kind, key)
	);

	CREATE TABLE IF NOT EXISTS graph_edges (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		namespace TEXT NOT NULL,
		relation  TEXT NOT NULL,
		source_id INTEGER NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
		target_id INTEGER NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
		attrs     TEXT NOT NULL DEFAULT '{}',
		UNIQUE(namespace, relation, source_id, target_id)
	);

	CREATE INDEX IF NOT EXISTS idx_graph_edges_source ON graph_edges(source_id);
	CREATE INDEX IF NOT EXISTS idx_graph_edges_target ON graph_edges(target_id);

	CREATE VIRTUAL TABLE IF NOT EXISTS graph_nodes_fts USING fts5(
		key,
		label,
		summary,
		content='graph_nodes',
		content_rowid='id',
		tokenize='unicode61'
	);

	CREATE TRIGGER IF NOT EXISTS graph_nodes_ai AFTER INSERT ON graph_nodes BEGIN
		INSERT INTO graph_nodes_fts(rowid, key, label, summary)
		VALUES (new.id, new.key, new.label, new.summary);
	END;

	CREATE TRIGGER IF NOT EXISTS graph_nodes_au AFTER UPDATE ON graph_nodes BEGIN
		INSERT INTO graph_nodes_fts(graph_nodes_fts, rowid, key, label, summary)
		VALUES ('delete', old.id, old.key, old.label, old.summary);
		INSERT INTO graph_nodes_fts(rowid, key, label, summary)
		VALUES (new.id, new.key, new.label, new.summary);
	END;

	CREATE TRIGGER IF NOT EXISTS graph_nodes_ad AFTER DELETE ON graph_nodes BEGIN
		INSERT INTO graph_nodes_fts(graph_nodes_fts, rowid, key, label, summary)
		VALUES ('delete', old.id, old.key, old.label, old.summary);
	END;
	`)
	return err
}

func marshalAttrs(attrs map[string]any) string {
	if len(attrs) == 0 {
		return "{}"
	}
	b, err := json.Marshal(attrs)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func unmarshalAttrs(raw string) map[string]any {
	if raw == "" || raw == "{}" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

func scanGraphNode(row interface{ Scan(...any) error }) (*GraphNode, error) {
	n := &GraphNode{}
	var attrs string
	err := row.Scan(&n.ID, &n.Namespace, &n.Kind, &n.Key, &n.Label, &n.Summary, &attrs, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, err
	}
	n.Attrs = unmarshalAttrs(attrs)
	return n, nil
}

const graphNodeCols = `id, namespace, kind, key, label, summary, attrs, created_at, updated_at`

// --- Namespaces ---

// GraphNamespaces returns the distinct namespaces present in the graph.
func (s *Store) GraphNamespaces() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT namespace FROM graph_nodes ORDER BY namespace`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var ns string
		if err := rows.Scan(&ns); err != nil {
			return nil, err
		}
		out = append(out, ns)
	}
	return out, rows.Err()
}

// --- Nodes ---

// UpsertGraphNode creates or updates a node identified by (namespace, kind, key).
// On update, label and summary are replaced only when non-empty, and attrs are
// shallow-merged so incremental callers don't wipe existing metadata.
// Returns the node and an action string ("created" | "updated" | "skipped").
func (s *Store) UpsertGraphNode(namespace, kind, key, label, summary string, attrs map[string]any) (*GraphNode, string, error) {
	if namespace == "" || kind == "" || key == "" {
		return nil, "", fmt.Errorf("namespace, kind and key are required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback() //nolint:errcheck

	existing, err := scanGraphNode(tx.QueryRow(
		`SELECT `+graphNodeCols+` FROM graph_nodes WHERE namespace = ? AND kind = ? AND key = ?`,
		namespace, kind, key,
	))
	if err != nil && err != sql.ErrNoRows {
		return nil, "", err
	}

	var action string
	var id int64
	if err == sql.ErrNoRows {
		if label == "" {
			label = key
		}
		res, err := tx.Exec(
			`INSERT INTO graph_nodes (namespace, kind, key, label, summary, attrs) VALUES (?, ?, ?, ?, ?, ?)`,
			namespace, kind, key, label, summary, marshalAttrs(attrs),
		)
		if err != nil {
			return nil, "", err
		}
		id, _ = res.LastInsertId()
		action = "created"
	} else {
		id = existing.ID
		newLabel := existing.Label
		if label != "" {
			newLabel = label
		}
		newSummary := existing.Summary
		if summary != "" {
			newSummary = summary
		}
		// copy before merging: mutating existing.Attrs in place would make
		// the change comparison below compare a map against itself
		merged := make(map[string]any, len(existing.Attrs)+len(attrs))
		for k, v := range existing.Attrs {
			merged[k] = v
		}
		for k, v := range attrs {
			merged[k] = v
		}
		changed := newLabel != existing.Label || newSummary != existing.Summary ||
			marshalAttrs(merged) != marshalAttrs(existing.Attrs)
		if !changed {
			action = "skipped"
		} else {
			_, err := tx.Exec(
				`UPDATE graph_nodes SET label = ?, summary = ?, attrs = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
				newLabel, newSummary, marshalAttrs(merged), id,
			)
			if err != nil {
				return nil, "", err
			}
			action = "updated"
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	n, err := s.GetGraphNode(id)
	return n, action, err
}

func (s *Store) GetGraphNode(id int64) (*GraphNode, error) {
	return scanGraphNode(s.db.QueryRow(
		`SELECT `+graphNodeCols+` FROM graph_nodes WHERE id = ?`, id,
	))
}

// GetGraphNodeByKey looks a node up by its natural key. Namespace may be
// empty, in which case the key must be unambiguous across namespaces.
func (s *Store) GetGraphNodeByKey(namespace, kind, key string) (*GraphNode, error) {
	if namespace != "" {
		return scanGraphNode(s.db.QueryRow(
			`SELECT `+graphNodeCols+` FROM graph_nodes WHERE namespace = ? AND kind = ? AND key = ?`,
			namespace, kind, key,
		))
	}
	rows, err := s.db.Query(
		`SELECT `+graphNodeCols+` FROM graph_nodes WHERE kind = ? AND key = ? LIMIT 2`, kind, key,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found *GraphNode
	count := 0
	for rows.Next() {
		n, err := scanGraphNode(rows)
		if err != nil {
			return nil, err
		}
		found = n
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, sql.ErrNoRows
	}
	if count > 1 {
		return nil, fmt.Errorf("node %s/%s exists in multiple namespaces; specify one", kind, key)
	}
	return found, nil
}

// sanitizeFTSQuery wraps each whitespace-separated term in double quotes so
// user input containing FTS5 operators (-, /, :) cannot break the MATCH
// expression. Terms are AND-ed, matching FTS5's default.
func sanitizeFTSQuery(q string) string {
	fields := strings.Fields(q)
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ReplaceAll(f, `"`, "")
		if f == "" {
			continue
		}
		quoted = append(quoted, `"`+f+`"`)
	}
	return strings.Join(quoted, " ")
}

// SearchGraphNodes runs a full-text search over node keys, labels and
// summaries. kind and namespace are optional filters.
func (s *Store) SearchGraphNodes(namespace, query, kind string, limit int) ([]*GraphNodeSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	match := sanitizeFTSQuery(query)
	if match == "" {
		return []*GraphNodeSearchResult{}, nil
	}
	q := `
		SELECT n.id, n.namespace, n.kind, n.key, n.label, n.summary, n.attrs, n.created_at, n.updated_at,
		       snippet(graph_nodes_fts, 2, '<mark>', '</mark>', '...', 20),
		       bm25(graph_nodes_fts)
		FROM graph_nodes_fts
		JOIN graph_nodes n ON n.id = graph_nodes_fts.rowid
		WHERE graph_nodes_fts MATCH ?`
	args := []any{match}
	if namespace != "" {
		q += ` AND n.namespace = ?`
		args = append(args, namespace)
	}
	if kind != "" {
		q += ` AND n.kind = ?`
		args = append(args, kind)
	}
	q += ` ORDER BY bm25(graph_nodes_fts) LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*GraphNodeSearchResult, 0)
	for rows.Next() {
		n := &GraphNode{}
		r := &GraphNodeSearchResult{Node: n}
		var attrs string
		if err := rows.Scan(&n.ID, &n.Namespace, &n.Kind, &n.Key, &n.Label, &n.Summary, &attrs,
			&n.CreatedAt, &n.UpdatedAt, &r.Snippet, &r.Score); err != nil {
			return nil, err
		}
		n.Attrs = unmarshalAttrs(attrs)
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Edges ---

// UpsertGraphEdge creates or updates an edge between two nodes resolved by
// their natural keys. Edge attrs are replaced on update. Both nodes must
// already exist. Returns ("created" | "updated" | "skipped").
func (s *Store) UpsertGraphEdge(namespace, relation, sourceKind, sourceKey, targetKind, targetKey string, attrs map[string]any) (string, error) {
	if namespace == "" || relation == "" {
		return "", fmt.Errorf("namespace and relation are required")
	}
	src, err := s.GetGraphNodeByKey(namespace, sourceKind, sourceKey)
	if err != nil {
		return "", fmt.Errorf("source node %s/%s not found: %w", sourceKind, sourceKey, err)
	}
	dst, err := s.GetGraphNodeByKey(namespace, targetKind, targetKey)
	if err != nil {
		return "", fmt.Errorf("target node %s/%s not found: %w", targetKind, targetKey, err)
	}

	var existingID int64
	var existingAttrs string
	err = s.db.QueryRow(
		`SELECT id, attrs FROM graph_edges WHERE namespace = ? AND relation = ? AND source_id = ? AND target_id = ?`,
		namespace, relation, src.ID, dst.ID,
	).Scan(&existingID, &existingAttrs)
	if err == sql.ErrNoRows {
		_, err := s.db.Exec(
			`INSERT INTO graph_edges (namespace, relation, source_id, target_id, attrs) VALUES (?, ?, ?, ?, ?)`,
			namespace, relation, src.ID, dst.ID, marshalAttrs(attrs),
		)
		if err != nil {
			return "", err
		}
		return "created", nil
	}
	if err != nil {
		return "", err
	}
	newAttrs := marshalAttrs(attrs)
	if newAttrs == existingAttrs || len(attrs) == 0 {
		return "skipped", nil
	}
	if _, err := s.db.Exec(`UPDATE graph_edges SET attrs = ? WHERE id = ?`, newAttrs, existingID); err != nil {
		return "", err
	}
	return "updated", nil
}

// GraphNeighbors returns the 1-hop neighborhood of a node. direction is
// "out", "in" or "" (both). relation optionally filters by edge type.
func (s *Store) GraphNeighbors(nodeID int64, relation, direction string, limit int) ([]*GraphNeighbor, error) {
	if limit <= 0 {
		limit = 200
	}
	out := make([]*GraphNeighbor, 0)

	fetch := func(dir string) error {
		var q string
		if dir == "out" {
			q = `SELECT e.relation, e.attrs, n.id, n.namespace, n.kind, n.key, n.label, n.summary, n.attrs, n.created_at, n.updated_at
			     FROM graph_edges e JOIN graph_nodes n ON n.id = e.target_id WHERE e.source_id = ?`
		} else {
			q = `SELECT e.relation, e.attrs, n.id, n.namespace, n.kind, n.key, n.label, n.summary, n.attrs, n.created_at, n.updated_at
			     FROM graph_edges e JOIN graph_nodes n ON n.id = e.source_id WHERE e.target_id = ?`
		}
		args := []any{nodeID}
		if relation != "" {
			q += ` AND e.relation = ?`
			args = append(args, relation)
		}
		q += ` ORDER BY e.relation, n.kind, n.label LIMIT ?`
		args = append(args, limit)

		rows, err := s.db.Query(q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			n := &GraphNode{}
			nb := &GraphNeighbor{Direction: dir, Node: n}
			var edgeAttrs, nodeAttrs string
			if err := rows.Scan(&nb.Relation, &edgeAttrs, &n.ID, &n.Namespace, &n.Kind, &n.Key,
				&n.Label, &n.Summary, &nodeAttrs, &n.CreatedAt, &n.UpdatedAt); err != nil {
				return err
			}
			nb.EdgeAttrs = unmarshalAttrs(edgeAttrs)
			n.Attrs = unmarshalAttrs(nodeAttrs)
			out = append(out, nb)
		}
		return rows.Err()
	}

	if direction == "" || direction == "out" {
		if err := fetch("out"); err != nil {
			return nil, err
		}
	}
	if direction == "" || direction == "in" {
		if err := fetch("in"); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// GraphImpact returns every node that transitively depends on the given node
// (reverse reachability over edges), with the minimum dependency depth.
// Edges whose relation is in excludeRelations are not traversed; pass
// {"belongs_to"} to keep organizational grouping out of impact results.
func (s *Store) GraphImpact(nodeID int64, maxDepth int, excludeRelations []string) ([]*GraphImpactRow, error) {
	if maxDepth <= 0 || maxDepth > 10 {
		maxDepth = 5
	}
	exclude := "(''"
	args := []any{}
	for _, r := range excludeRelations {
		exclude += ",?"
		args = append(args, r)
	}
	exclude += ")"

	q := `
		WITH RECURSIVE dependents(id, depth) AS (
			SELECT e.source_id, 1 FROM graph_edges e
			 WHERE e.target_id = ? AND e.relation NOT IN ` + exclude + `
			UNION
			SELECT e.source_id, d.depth + 1 FROM graph_edges e
			  JOIN dependents d ON e.target_id = d.id
			 WHERE d.depth < ? AND e.relation NOT IN ` + exclude + `
		)
		SELECT MIN(d.depth), n.id, n.namespace, n.kind, n.key, n.label, n.summary, n.attrs, n.created_at, n.updated_at
		FROM dependents d JOIN graph_nodes n ON n.id = d.id
		WHERE n.id != ?
		GROUP BY n.id
		ORDER BY MIN(d.depth), n.kind, n.label`

	fullArgs := []any{nodeID}
	fullArgs = append(fullArgs, args...)
	fullArgs = append(fullArgs, maxDepth)
	fullArgs = append(fullArgs, args...)
	fullArgs = append(fullArgs, nodeID)

	rows, err := s.db.Query(q, fullArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*GraphImpactRow, 0)
	for rows.Next() {
		n := &GraphNode{}
		r := &GraphImpactRow{Node: n}
		var attrs string
		if err := rows.Scan(&r.Depth, &n.ID, &n.Namespace, &n.Kind, &n.Key, &n.Label, &n.Summary,
			&attrs, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.Attrs = unmarshalAttrs(attrs)
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Stats ---

func (s *Store) GetGraphStats(namespace string) (*GraphStats, error) {
	st := &GraphStats{
		Namespace:       namespace,
		NodesByKind:     map[string]int{},
		EdgesByRelation: map[string]int{},
	}
	nodeQ := `SELECT kind, COUNT(*) FROM graph_nodes`
	edgeQ := `SELECT relation, COUNT(*) FROM graph_edges`
	var args []any
	if namespace != "" {
		nodeQ += ` WHERE namespace = ?`
		edgeQ += ` WHERE namespace = ?`
		args = []any{namespace}
	}
	rows, err := s.db.Query(nodeQ+` GROUP BY kind`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var c int
		if err := rows.Scan(&kind, &c); err != nil {
			return nil, err
		}
		st.NodesByKind[kind] = c
		st.NodeCount += c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows2, err := s.db.Query(edgeQ+` GROUP BY relation`, args...)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var rel string
		var c int
		if err := rows2.Scan(&rel, &c); err != nil {
			return nil, err
		}
		st.EdgesByRelation[rel] = c
		st.EdgeCount += c
	}
	return st, rows2.Err()
}
