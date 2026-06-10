package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// registerGraphTools exposes the knowledge graph over MCP. The graph holds
// heterogeneous nodes (repos, portals, menu options, use cases, ...) linked
// by typed edges, scoped by namespace. Edge convention: source depends on
// target, so impact analysis walks edges in reverse.
func registerGraphTools(s *mcpserver.MCPServer, st *store.Store) {

	// resolveNS picks the namespace: explicit param wins; otherwise, if the
	// graph contains exactly one namespace, use it. Empty result means
	// "all namespaces" (fine for reads, rejected for writes).
	resolveNS := func(req mcp.CallToolRequest) string {
		if ns := req.GetString("namespace", ""); ns != "" {
			return ns
		}
		all, err := st.GraphNamespaces()
		if err == nil && len(all) == 1 {
			return all[0]
		}
		return ""
	}

	// resolveNode finds a node by kind+key, with a search fallback so LLMs
	// can pass approximate keys (e.g. "RNFTransversalMS" instead of
	// "RNF/RNFTransversalMS").
	resolveNode := func(ns, kind, key string) (*store.GraphNode, error) {
		n, err := st.GetGraphNodeByKey(ns, kind, key)
		if err == nil {
			return n, nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
		results, serr := st.SearchGraphNodes(ns, key, kind, 2)
		if serr != nil || len(results) == 0 {
			return nil, fmt.Errorf("node not found: %s/%s", kind, key)
		}
		if len(results) > 1 {
			return nil, fmt.Errorf("node %s/%s is ambiguous: did you mean %q or %q? Use graph_search to disambiguate",
				kind, key, results[0].Node.Key, results[1].Node.Key)
		}
		return results[0].Node, nil
	}

	// --- graph_search ---
	s.AddTool(mcp.NewTool("graph_search",
		mcp.WithDescription("Full-text search over knowledge graph nodes (repos, portals, menu options, use cases, api clients, team projects). Returns matching nodes with kind, key and attrs. Use this first to find exact node keys."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search terms (AND-ed, no stemming)")),
		mcp.WithString("kind", mcp.Description("Filter by node kind: repo | portal | menu_option | use_case | team_project")),
		mcp.WithString("namespace", mcp.Description("Graph namespace (auto-detected when only one exists)")),
		mcp.WithString("limit", mcp.Description("Max results, default 20")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q, err := req.RequireString("query")
		if err != nil {
			return errResult(err)
		}
		results, err := st.SearchGraphNodes(resolveNS(req), q, req.GetString("kind", ""), req.GetInt("limit", 20))
		return jsonResult(results, err)
	})

	// --- graph_node ---
	s.AddTool(mcp.NewTool("graph_node",
		mcp.WithDescription("Get a node's full context: its attributes plus all 1-hop relations grouped by relation type and direction. 'out' relations are dependencies of this node; 'in' relations are its dependents."),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Node kind: repo | portal | menu_option | use_case | team_project")),
		mcp.WithString("key", mcp.Required(), mcp.Description("Node key, e.g. 'RNF/RNFTransversalMS' or 'RUNTPRO'. Approximate keys are resolved via search when unambiguous.")),
		mcp.WithString("namespace", mcp.Description("Graph namespace (auto-detected when only one exists)")),
		mcp.WithString("neighbor_limit", mcp.Description("Max neighbors per direction, default 100")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kind, err := req.RequireString("kind")
		if err != nil {
			return errResult(err)
		}
		key, err := req.RequireString("key")
		if err != nil {
			return errResult(err)
		}
		node, err := resolveNode(resolveNS(req), kind, key)
		if err != nil {
			return errResult(err)
		}
		limit := req.GetInt("neighbor_limit", 100)
		neighbors, err := st.GraphNeighbors(node.ID, "", "", limit)
		if err != nil {
			return errResult(err)
		}

		grouped := map[string][]*store.GraphNeighbor{}
		for _, nb := range neighbors {
			k := nb.Direction + ":" + nb.Relation
			grouped[k] = append(grouped[k], nb)
		}
		return jsonResult(map[string]any{
			"node":           node,
			"relations":      grouped,
			"neighbor_count": len(neighbors),
			"hint":           "out = this node depends on them; in = they depend on this node. Use graph_impact for transitive dependents.",
		}, nil)
	})

	// --- graph_neighbors ---
	s.AddTool(mcp.NewTool("graph_neighbors",
		mcp.WithDescription("List direct neighbors of a node, optionally filtered by relation and direction. Lighter than graph_node when you only need one relation type."),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Node kind")),
		mcp.WithString("key", mcp.Required(), mcp.Description("Node key")),
		mcp.WithString("relation", mcp.Description("Filter: exposes | consumes | uses_client | implemented_by | implements | belongs_to")),
		mcp.WithString("direction", mcp.Description("'out' (dependencies) | 'in' (dependents) | omit for both")),
		mcp.WithString("namespace", mcp.Description("Graph namespace (auto-detected when only one exists)")),
		mcp.WithString("limit", mcp.Description("Max results per direction, default 200")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kind, err := req.RequireString("kind")
		if err != nil {
			return errResult(err)
		}
		key, err := req.RequireString("key")
		if err != nil {
			return errResult(err)
		}
		node, err := resolveNode(resolveNS(req), kind, key)
		if err != nil {
			return errResult(err)
		}
		neighbors, err := st.GraphNeighbors(node.ID,
			req.GetString("relation", ""), req.GetString("direction", ""), req.GetInt("limit", 200))
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"node": node, "neighbors": neighbors, "count": len(neighbors)}, nil)
	})

	// --- graph_impact ---
	s.AddTool(mcp.NewTool("graph_impact",
		mcp.WithDescription("Blast-radius analysis: every node that transitively depends on the given node, with dependency depth. Use to answer 'what breaks if X changes?'. Organizational belongs_to edges are excluded from traversal."),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Node kind")),
		mcp.WithString("key", mcp.Required(), mcp.Description("Node key, e.g. 'RNF/FirmaApiClient'")),
		mcp.WithString("max_depth", mcp.Description("Traversal depth cap, default 5, max 10")),
		mcp.WithString("limit", mcp.Description("Max listed nodes, default 100 (counts always cover everything)")),
		mcp.WithString("kind_filter", mcp.Description("Only list impacted nodes of this kind, e.g. 'portal'")),
		mcp.WithString("namespace", mcp.Description("Graph namespace (auto-detected when only one exists)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kind, err := req.RequireString("kind")
		if err != nil {
			return errResult(err)
		}
		key, err := req.RequireString("key")
		if err != nil {
			return errResult(err)
		}
		node, err := resolveNode(resolveNS(req), kind, key)
		if err != nil {
			return errResult(err)
		}
		rows, err := st.GraphImpact(node.ID, req.GetInt("max_depth", 5), []string{"belongs_to"})
		if err != nil {
			return errResult(err)
		}

		// hub nodes can have thousands of transitive dependents: return slim
		// rows (no summary/attrs) and cap the list, counts stay exact
		limit := req.GetInt("limit", 100)
		kindFilter := req.GetString("kind_filter", "")
		byKind := map[string]int{}
		type slimRow struct {
			Depth int    `json:"depth"`
			Kind  string `json:"kind"`
			Key   string `json:"key"`
			Label string `json:"label"`
		}
		listed := make([]slimRow, 0, limit)
		for _, r := range rows {
			byKind[r.Node.Kind]++
			if kindFilter != "" && r.Node.Kind != kindFilter {
				continue
			}
			if len(listed) < limit {
				listed = append(listed, slimRow{r.Depth, r.Node.Kind, r.Node.Key, r.Node.Label})
			}
		}
		return jsonResult(map[string]any{
			"node":             map[string]any{"kind": node.Kind, "key": node.Key, "label": node.Label},
			"impacted_count":   len(rows),
			"impacted_by_kind": byKind,
			"impacted":         listed,
			"truncated":        len(rows) > len(listed),
			"hint":             "Use kind_filter and limit to page through results; use graph_node on specific results for detail.",
		}, nil)
	})

	// --- graph_upsert_node ---
	s.AddTool(mcp.NewTool("graph_upsert_node",
		mcp.WithDescription("Create or update a knowledge graph node. Dedup-safe by (namespace, kind, key). On update, attrs are shallow-merged and empty label/summary keep existing values."),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Node kind, e.g. repo | portal | menu_option | use_case | team_project | note")),
		mcp.WithString("key", mcp.Required(), mcp.Description("Stable natural key, e.g. 'RNF/RNFTransversalMS'")),
		mcp.WithString("label", mcp.Description("Human-readable name (defaults to key on create)")),
		mcp.WithString("summary", mcp.Description("Searchable description — this is what FTS matches on, write it rich")),
		mcp.WithString("attrs", mcp.Description("JSON object with arbitrary metadata, e.g. {\"tier\":\"critical\"}")),
		mcp.WithString("namespace", mcp.Description("Graph namespace (auto-detected when only one exists; required otherwise)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kind, err := req.RequireString("kind")
		if err != nil {
			return errResult(err)
		}
		key, err := req.RequireString("key")
		if err != nil {
			return errResult(err)
		}
		ns := resolveNS(req)
		if ns == "" {
			return errResult(fmt.Errorf("namespace is required (none or multiple exist in the graph)"))
		}
		var attrs map[string]any
		if raw := req.GetString("attrs", ""); raw != "" {
			if err := json.Unmarshal([]byte(raw), &attrs); err != nil {
				return errResult(fmt.Errorf("attrs must be a JSON object: %w", err))
			}
		}
		n, action, err := st.UpsertGraphNode(ns, kind, key, req.GetString("label", ""), req.GetString("summary", ""), attrs)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"action": action, "node": n}, nil)
	})

	// --- graph_upsert_edge ---
	s.AddTool(mcp.NewTool("graph_upsert_edge",
		mcp.WithDescription("Create or update a typed edge between two existing nodes. Direction convention: SOURCE DEPENDS ON TARGET (portal exposes MR, MR consumes MS, MS uses_client ApiClient). Dedup-safe."),
		mcp.WithString("relation", mcp.Required(), mcp.Description("Edge type: exposes | consumes | uses_client | implemented_by | implements | belongs_to | relates_to")),
		mcp.WithString("source_kind", mcp.Required(), mcp.Description("Source node kind (the dependent)")),
		mcp.WithString("source_key", mcp.Required(), mcp.Description("Source node key")),
		mcp.WithString("target_kind", mcp.Required(), mcp.Description("Target node kind (the dependency)")),
		mcp.WithString("target_key", mcp.Required(), mcp.Description("Target node key")),
		mcp.WithString("attrs", mcp.Description("JSON object with edge metadata, e.g. {\"remote_key\":\"consulta-vehiculo\"}")),
		mcp.WithString("namespace", mcp.Description("Graph namespace (auto-detected when only one exists; required otherwise)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		relation, err := req.RequireString("relation")
		if err != nil {
			return errResult(err)
		}
		sk, err := req.RequireString("source_kind")
		if err != nil {
			return errResult(err)
		}
		skey, err := req.RequireString("source_key")
		if err != nil {
			return errResult(err)
		}
		tk, err := req.RequireString("target_kind")
		if err != nil {
			return errResult(err)
		}
		tkey, err := req.RequireString("target_key")
		if err != nil {
			return errResult(err)
		}
		ns := resolveNS(req)
		if ns == "" {
			return errResult(fmt.Errorf("namespace is required (none or multiple exist in the graph)"))
		}
		var attrs map[string]any
		if raw := req.GetString("attrs", ""); raw != "" {
			if err := json.Unmarshal([]byte(raw), &attrs); err != nil {
				return errResult(fmt.Errorf("attrs must be a JSON object: %w", err))
			}
		}
		action, err := st.UpsertGraphEdge(ns, relation, sk, skey, tk, tkey, attrs)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"action": action}, nil)
	})

	// --- graph_stats ---
	s.AddTool(mcp.NewTool("graph_stats",
		mcp.WithDescription("Knowledge graph overview: node counts by kind and edge counts by relation. Call this first to discover what the graph contains."),
		mcp.WithString("namespace", mcp.Description("Graph namespace (omit for all)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stats, err := st.GetGraphStats(req.GetString("namespace", ""))
		if err != nil {
			return errResult(err)
		}
		namespaces, _ := st.GraphNamespaces()
		return jsonResult(map[string]any{"stats": stats, "namespaces": namespaces}, nil)
	})
}
