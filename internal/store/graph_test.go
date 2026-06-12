package store

import (
	"database/sql"
	"testing"
)

// seedGraph builds a small RUNT-shaped graph:
//
//	menu_option:opt1 -> repo:CYRMR (implemented_by)
//	portal:RUNTPRO   -> repo:CYRMR (exposes)
//	repo:CYRMR       -> repo:CYRConsultasVehiculoMS (consumes)
//	repo:CYRConsultasVehiculoMS -> repo:FirmaApiClient (uses_client)
//	repo:CYRMR       -> team_project:ConsultasyReportes (belongs_to)
func seedGraph(t *testing.T, s *Store) {
	t.Helper()
	nodes := []struct {
		kind, key, label, summary string
		attrs                     map[string]any
	}{
		{"portal", "RUNTPRO", "RUNTPRO", "Main back-office portal", nil},
		{"repo", "ConsultasyReportes/CYRMR", "CYRMR", "Frontend MR for vehicle queries", map[string]any{"angular_major": 14}},
		{"repo", "ConsultasyReportes/CYRConsultasVehiculoMS", "CYRConsultasVehiculoMS", "Vehicle query microservice", map[string]any{"tier": "high"}},
		{"repo", "RNF/FirmaApiClient", "FirmaApiClient", "Digital signature client library", nil},
		{"team_project", "ConsultasyReportes", "ConsultasyReportes", "", nil},
		{"menu_option", "RUNTPRO:consulta-vehiculo", "Consulta Vehiculo", "Menu entry for vehicle queries", nil},
	}
	for _, n := range nodes {
		if _, _, err := s.UpsertGraphNode("test", n.kind, n.key, n.label, n.summary, n.attrs); err != nil {
			t.Fatalf("seed node %s: %v", n.key, err)
		}
	}
	edges := []struct {
		relation, sk, skey, tk, tkey string
	}{
		{"exposes", "portal", "RUNTPRO", "repo", "ConsultasyReportes/CYRMR"},
		{"consumes", "repo", "ConsultasyReportes/CYRMR", "repo", "ConsultasyReportes/CYRConsultasVehiculoMS"},
		{"uses_client", "repo", "ConsultasyReportes/CYRConsultasVehiculoMS", "repo", "RNF/FirmaApiClient"},
		{"belongs_to", "repo", "ConsultasyReportes/CYRMR", "team_project", "ConsultasyReportes"},
		{"implemented_by", "menu_option", "RUNTPRO:consulta-vehiculo", "repo", "ConsultasyReportes/CYRMR"},
	}
	for _, e := range edges {
		if _, err := s.UpsertGraphEdge("test", e.relation, e.sk, e.skey, e.tk, e.tkey, nil); err != nil {
			t.Fatalf("seed edge %s: %v", e.relation, err)
		}
	}
}

func TestUpsertGraphNodeLifecycle(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	n, action, err := s.UpsertGraphNode("test", "repo", "RNF/RNFTransversalMS", "RNFTransversalMS", "Platform hub", map[string]any{"tier": "critical"})
	if err != nil {
		t.Fatal(err)
	}
	if action != "created" {
		t.Fatalf("want created, got %s", action)
	}
	if n.Label != "RNFTransversalMS" || n.Attrs["tier"] != "critical" {
		t.Fatalf("unexpected node: %+v", n)
	}

	// identical content → skipped
	_, action, err = s.UpsertGraphNode("test", "repo", "RNF/RNFTransversalMS", "RNFTransversalMS", "Platform hub", map[string]any{"tier": "critical"})
	if err != nil {
		t.Fatal(err)
	}
	if action != "skipped" {
		t.Fatalf("want skipped, got %s", action)
	}

	// attrs merge: new key added, old key preserved, empty label keeps existing
	n, action, err = s.UpsertGraphNode("test", "repo", "RNF/RNFTransversalMS", "", "", map[string]any{"wave": "wave_0"})
	if err != nil {
		t.Fatal(err)
	}
	if action != "updated" {
		t.Fatalf("want updated, got %s", action)
	}
	if n.Label != "RNFTransversalMS" || n.Attrs["tier"] != "critical" || n.Attrs["wave"] != "wave_0" {
		t.Fatalf("merge failed: %+v", n)
	}

	// empty label on create falls back to key
	n, _, err = s.UpsertGraphNode("test", "repo", "X/NoLabelMS", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if n.Label != "X/NoLabelMS" {
		t.Fatalf("want label fallback to key, got %q", n.Label)
	}
}

func TestGetGraphNodeByKeyAmbiguity(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, _, err := s.UpsertGraphNode("ns1", "repo", "A/B", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.UpsertGraphNode("ns2", "repo", "A/B", "", "", nil); err != nil {
		t.Fatal(err)
	}

	// without namespace: ambiguous → error
	if _, err := s.GetGraphNodeByKey("", "repo", "A/B"); err == nil {
		t.Fatal("want ambiguity error, got nil")
	}
	// with namespace: resolves
	n, err := s.GetGraphNodeByKey("ns2", "repo", "A/B")
	if err != nil || n.Namespace != "ns2" {
		t.Fatalf("want ns2 node, got %+v err=%v", n, err)
	}
	// missing key
	if _, err := s.GetGraphNodeByKey("ns1", "repo", "nope"); err != sql.ErrNoRows {
		t.Fatalf("want ErrNoRows, got %v", err)
	}
}

func TestUpsertGraphEdge(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seedGraph(t, s)

	// duplicate edge with no attrs → skipped
	action, err := s.UpsertGraphEdge("test", "exposes", "portal", "RUNTPRO", "repo", "ConsultasyReportes/CYRMR", nil)
	if err != nil {
		t.Fatal(err)
	}
	if action != "skipped" {
		t.Fatalf("want skipped, got %s", action)
	}

	// new attrs → updated
	action, err = s.UpsertGraphEdge("test", "exposes", "portal", "RUNTPRO", "repo", "ConsultasyReportes/CYRMR", map[string]any{"remote_keys": []string{"consulta-vehiculo"}})
	if err != nil {
		t.Fatal(err)
	}
	if action != "updated" {
		t.Fatalf("want updated, got %s", action)
	}

	// missing endpoint → error
	if _, err := s.UpsertGraphEdge("test", "consumes", "repo", "ghost", "repo", "ConsultasyReportes/CYRMR", nil); err == nil {
		t.Fatal("want error for missing source node")
	}
}

func TestListGraphNodesByKind(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seedGraph(t, s)

	repos, err := s.ListGraphNodesByKind("test", "repo", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 3 {
		t.Fatalf("want 3 repos, got %d", len(repos))
	}
	// ordered by label
	if repos[0].Label != "CYRConsultasVehiculoMS" {
		t.Fatalf("want label-ordered results, got first %q", repos[0].Label)
	}

	portals, err := s.ListGraphNodesByKind("", "portal", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(portals) != 1 || portals[0].Key != "RUNTPRO" {
		t.Fatalf("unexpected portals: %+v", portals)
	}

	none, err := s.ListGraphNodesByKind("other-ns", "repo", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("want empty for unknown namespace, got %d", len(none))
	}
}

func TestSearchGraphNodes(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seedGraph(t, s)

	// single term matches MR, MS and menu summaries
	results, err := s.SearchGraphNodes("test", "vehicle", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 3 {
		t.Fatalf("want >=3 results for 'vehicle', got %d", len(results))
	}

	// multi-term is AND-ed: only the MS summary has both exact tokens
	// (unicode61 does not stem, so "query" must not match "queries")
	results, err = s.SearchGraphNodes("test", "vehicle query", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Node.Key != "ConsultasyReportes/CYRConsultasVehiculoMS" {
		t.Fatalf("want exactly the MS node for 'vehicle query', got %+v", results)
	}

	// kind filter
	results, err = s.SearchGraphNodes("test", "vehicle", "menu_option", 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Node.Kind != "menu_option" {
			t.Fatalf("kind filter leaked: %+v", r.Node)
		}
	}

	// hostile input must not break FTS5 (slashes, dashes, quotes)
	if _, err := s.SearchGraphNodes("test", `RNF/Firma "x" -bad`, "", 10); err != nil {
		t.Fatalf("hostile query broke search: %v", err)
	}
}

func TestGraphNeighbors(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seedGraph(t, s)

	mr, err := s.GetGraphNodeByKey("test", "repo", "ConsultasyReportes/CYRMR")
	if err != nil {
		t.Fatal(err)
	}

	all, err := s.GraphNeighbors(mr.ID, "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	// out: consumes MS, belongs_to TP; in: portal exposes, menu implemented_by
	var outs, ins int
	for _, nb := range all {
		if nb.Direction == "out" {
			outs++
		} else {
			ins++
		}
	}
	if outs != 2 || ins != 2 {
		t.Fatalf("want 2 out / 2 in, got %d out / %d in", outs, ins)
	}

	// relation filter + direction
	cons, err := s.GraphNeighbors(mr.ID, "consumes", "out", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(cons) != 1 || cons[0].Node.Key != "ConsultasyReportes/CYRConsultasVehiculoMS" {
		t.Fatalf("unexpected consumes neighbors: %+v", cons)
	}
}

func TestGraphImpact(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seedGraph(t, s)

	client, err := s.GetGraphNodeByKey("test", "repo", "RNF/FirmaApiClient")
	if err != nil {
		t.Fatal(err)
	}

	rows, err := s.GraphImpact(client.ID, 5, []string{"belongs_to"})
	if err != nil {
		t.Fatal(err)
	}
	// expected chain: MS (d1) <- MR (d2) <- portal + menu (d3)
	got := map[string]int{}
	for _, r := range rows {
		got[r.Node.Key] = r.Depth
	}
	want := map[string]int{
		"ConsultasyReportes/CYRConsultasVehiculoMS": 1,
		"ConsultasyReportes/CYRMR":                  2,
		"RUNTPRO":                                   3,
		"RUNTPRO:consulta-vehiculo":                 3,
	}
	if len(got) != len(want) {
		t.Fatalf("want %d impacted nodes, got %d: %+v", len(want), len(got), got)
	}
	for k, d := range want {
		if got[k] != d {
			t.Fatalf("node %s: want depth %d, got %d", k, d, got[k])
		}
	}

	// team_project must NOT appear (belongs_to excluded, and nothing depends on it otherwise)
	if _, ok := got["ConsultasyReportes"]; ok {
		t.Fatal("belongs_to relation leaked into impact traversal")
	}

	// depth cap: maxDepth=1 → only the MS
	rows, err = s.GraphImpact(client.ID, 1, []string{"belongs_to"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Node.Key != "ConsultasyReportes/CYRConsultasVehiculoMS" {
		t.Fatalf("depth cap failed: %+v", rows)
	}
}

func TestGraphStatsAndNamespaces(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seedGraph(t, s)

	st, err := s.GetGraphStats("test")
	if err != nil {
		t.Fatal(err)
	}
	if st.NodeCount != 6 || st.EdgeCount != 5 {
		t.Fatalf("want 6 nodes / 5 edges, got %d / %d", st.NodeCount, st.EdgeCount)
	}
	if st.NodesByKind["repo"] != 3 || st.EdgesByRelation["exposes"] != 1 {
		t.Fatalf("unexpected breakdown: %+v", st)
	}

	ns, err := s.GraphNamespaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) != 1 || ns[0] != "test" {
		t.Fatalf("want [test], got %v", ns)
	}
}
