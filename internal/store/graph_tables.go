package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Table-usage queries over the knowledge graph. An external importer loads
// database tables as `table` nodes, code files as `source_file` nodes and
// repos as `repo` nodes, linked by:
//
//	source_file -uses_table-> table   (attrs.evidence: [{path,line,type,text}])
//	source_file -in_repo->    repo
//
// SearchTableUsage answers "which repos/files read or write table X?".

const (
	tableSearchDefaultLimit    = 20
	tableSearchMaxLimit        = 100
	tableSearchDefaultMaxFiles = 50
	tableSearchMaxMaxFiles     = 200
)

// TableEvidence is one code location where a file references a table.
type TableEvidence struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Type string `json:"type"`
	Text string `json:"text"`
}

type TableUsageFile struct {
	Path     string          `json:"path"`
	Evidence []TableEvidence `json:"evidence"`
}

type TableUsageRepo struct {
	Repo  string            `json:"repo"`
	Files []*TableUsageFile `json:"files"`
}

type TableUsage struct {
	Namespace      string            `json:"namespace"`
	Key            string            `json:"key"`
	Label          string            `json:"label"`
	FileCount      int               `json:"file_count"`
	RepoCount      int               `json:"repo_count"`
	FilesTruncated bool              `json:"files_truncated"`
	Repos          []*TableUsageRepo `json:"repos"`
}

type TableUsageResult struct {
	Query     string        `json:"query"`
	Namespace string        `json:"namespace,omitempty"`
	Repo      string        `json:"repo,omitempty"`
	Tables    []*TableUsage `json:"tables"`
	// Truncated is true when the table limit or any per-table file cap cut results.
	Truncated       bool `json:"truncated"`
	TablesTruncated bool `json:"tables_truncated"`
}

// escapeLike escapes LIKE wildcards so user input matches literally with ESCAPE '\'.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

type tableUsageRow struct {
	repo, path string
	evidence   []TableEvidence
}

// SearchTableUsage finds table nodes whose key contains query
// (case-insensitive, literal) and lists the source files using each one,
// grouped by repo. namespace and repo are optional exact filters; with a repo
// filter, tables unused in that repo are omitted. limit caps tables
// (default 20, max 100) and maxFiles caps files per table (default 50,
// max 200); counts always cover every usage.
func (s *Store) SearchTableUsage(namespace, query, repo string, limit, maxFiles int) (*TableUsageResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	repo = strings.TrimSpace(repo)
	if limit <= 0 {
		limit = tableSearchDefaultLimit
	}
	if limit > tableSearchMaxLimit {
		limit = tableSearchMaxLimit
	}
	if maxFiles <= 0 {
		maxFiles = tableSearchDefaultMaxFiles
	}
	if maxFiles > tableSearchMaxMaxFiles {
		maxFiles = tableSearchMaxMaxFiles
	}

	q := `SELECT ` + graphNodeCols + ` FROM graph_nodes
		WHERE kind = 'table' AND UPPER(key) LIKE ? ESCAPE '\'`
	args := []any{"%" + escapeLike(strings.ToUpper(query)) + "%"}
	if namespace != "" {
		q += ` AND namespace = ?`
		args = append(args, namespace)
	}
	q += ` ORDER BY CASE WHEN UPPER(key) = ? THEN 0 ELSE 1 END, key, namespace`
	args = append(args, strings.ToUpper(query))

	// collect candidates before running per-table queries so no result set
	// stays open while issuing nested statements
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	candidates := make([]*GraphNode, 0)
	for rows.Next() {
		n, err := scanGraphNode(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	res := &TableUsageResult{Query: query, Namespace: namespace, Repo: repo, Tables: make([]*TableUsage, 0)}
	for _, node := range candidates {
		usages, err := s.tableUsages(node.ID)
		if err != nil {
			return nil, err
		}
		if repo != "" {
			filtered := usages[:0]
			for _, u := range usages {
				if u.repo == repo {
					filtered = append(filtered, u)
				}
			}
			usages = filtered
			if len(usages) == 0 {
				continue
			}
		}
		if len(res.Tables) == limit {
			res.TablesTruncated = true
			res.Truncated = true
			break
		}
		tb := buildTableUsage(node, usages, maxFiles)
		if tb.FilesTruncated {
			res.Truncated = true
		}
		res.Tables = append(res.Tables, tb)
	}
	return res, nil
}

// tableUsages loads every uses_table edge pointing at a table, resolving the
// file's repo through its in_repo edge with attrs.repo as fallback.
func (s *Store) tableUsages(tableID int64) ([]tableUsageRow, error) {
	rows, err := s.db.Query(`
		SELECT sf.key, e.attrs,
		       COALESCE(
		           (SELECT r.key FROM graph_edges ir JOIN graph_nodes r ON r.id = ir.target_id
		             WHERE ir.source_id = sf.id AND ir.relation = 'in_repo' AND r.kind = 'repo'
		             ORDER BY r.key LIMIT 1),
		           CAST(json_extract(CASE WHEN json_valid(sf.attrs) THEN sf.attrs ELSE '{}' END, '$.repo') AS TEXT),
		           '')
		FROM graph_edges e JOIN graph_nodes sf ON sf.id = e.source_id
		WHERE e.target_id = ? AND e.relation = 'uses_table' AND sf.kind = 'source_file'`, tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]tableUsageRow, 0)
	for rows.Next() {
		var u tableUsageRow
		var attrs string
		if err := rows.Scan(&u.path, &attrs, &u.repo); err != nil {
			return nil, err
		}
		u.evidence = parseTableEvidence(attrs)
		out = append(out, u)
	}
	return out, rows.Err()
}

// parseTableEvidence decodes attrs.evidence leniently: malformed entries are
// skipped rather than failing the whole search.
func parseTableEvidence(raw string) []TableEvidence {
	out := make([]TableEvidence, 0)
	attrs := unmarshalAttrs(raw)
	list, ok := attrs["evidence"].([]any)
	if !ok {
		return out
	}
	for _, item := range list {
		b, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var ev TableEvidence
		if err := json.Unmarshal(b, &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	return out
}

func buildTableUsage(node *GraphNode, usages []tableUsageRow, maxFiles int) *TableUsage {
	sort.Slice(usages, func(i, j int) bool {
		if usages[i].repo != usages[j].repo {
			return usages[i].repo < usages[j].repo
		}
		return usages[i].path < usages[j].path
	})
	tb := &TableUsage{
		Namespace: node.Namespace,
		Key:       node.Key,
		Label:     node.Label,
		FileCount: len(usages),
		Repos:     make([]*TableUsageRepo, 0),
	}
	repoSeen := map[string]bool{}
	for _, u := range usages {
		repoSeen[u.repo] = true
	}
	tb.RepoCount = len(repoSeen)
	if len(usages) > maxFiles {
		usages = usages[:maxFiles]
		tb.FilesTruncated = true
	}
	var cur *TableUsageRepo
	for _, u := range usages {
		if cur == nil || cur.Repo != u.repo {
			cur = &TableUsageRepo{Repo: u.repo, Files: make([]*TableUsageFile, 0)}
			tb.Repos = append(tb.Repos, cur)
		}
		cur.Files = append(cur.Files, &TableUsageFile{Path: u.path, Evidence: u.evidence})
	}
	return tb
}
