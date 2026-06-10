package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// seedGraphForServer inserts a small graph identical to the one in
// internal/store/graph_test.go so server tests have predictable data.
//
// Topology:
//
//	menu_option:RUNTPRO:consulta-vehiculo --implemented_by--> repo:ConsultasyReportes/CYRMR
//	portal:RUNTPRO                        --exposes----------> repo:ConsultasyReportes/CYRMR
//	repo:ConsultasyReportes/CYRMR         --consumes---------> repo:ConsultasyReportes/CYRConsultasVehiculoMS
//	repo:ConsultasyReportes/CYRConsultasVehiculoMS --uses_client--> repo:RNF/FirmaApiClient
//	repo:ConsultasyReportes/CYRMR         --belongs_to-------> team_project:ConsultasyReportes
func seedGraphForServer(t *testing.T, st *store.Store) {
	t.Helper()
	nodes := []struct {
		kind, key, label, summary string
	}{
		{"portal", "RUNTPRO", "RUNTPRO", "Main back-office portal"},
		{"repo", "ConsultasyReportes/CYRMR", "CYRMR", "Frontend MR for vehicle queries"},
		{"repo", "ConsultasyReportes/CYRConsultasVehiculoMS", "CYRConsultasVehiculoMS", "Vehicle query microservice"},
		{"repo", "RNF/FirmaApiClient", "FirmaApiClient", "Digital signature client library"},
		{"team_project", "ConsultasyReportes", "ConsultasyReportes", ""},
		{"menu_option", "RUNTPRO:consulta-vehiculo", "Consulta Vehiculo", "Menu entry for vehicle queries"},
	}
	for _, n := range nodes {
		if _, _, err := st.UpsertGraphNode("test", n.kind, n.key, n.label, n.summary, nil); err != nil {
			t.Fatalf("seed node %s: %v", n.key, err)
		}
	}
	edges := []struct{ rel, sk, skey, tk, tkey string }{
		{"exposes", "portal", "RUNTPRO", "repo", "ConsultasyReportes/CYRMR"},
		{"consumes", "repo", "ConsultasyReportes/CYRMR", "repo", "ConsultasyReportes/CYRConsultasVehiculoMS"},
		{"uses_client", "repo", "ConsultasyReportes/CYRConsultasVehiculoMS", "repo", "RNF/FirmaApiClient"},
		{"belongs_to", "repo", "ConsultasyReportes/CYRMR", "team_project", "ConsultasyReportes"},
		{"implemented_by", "menu_option", "RUNTPRO:consulta-vehiculo", "repo", "ConsultasyReportes/CYRMR"},
	}
	for _, e := range edges {
		if _, err := st.UpsertGraphEdge("test", e.rel, e.sk, e.skey, e.tk, e.tkey, nil); err != nil {
			t.Fatalf("seed edge %s: %v", e.rel, err)
		}
	}
}

// --- search ---

func TestGraphSearch_ReturnsResults(t *testing.T) {
	base, st := newTestServer(t)
	seedGraphForServer(t, st)

	resp := get(t, base+"/api/graph/search?q=vehicle&namespace=test")
	var results []map[string]any
	decodeJSON(t, resp, &results)

	if len(results) < 2 {
		t.Fatalf("want >= 2 results for 'vehicle', got %d", len(results))
	}
}

func TestGraphSearch_EmptyQuery_ReturnsEmptyArray(t *testing.T) {
	base, _ := newTestServer(t)

	resp := get(t, base+"/api/graph/search")
	var results []any
	decodeJSON(t, resp, &results)

	if results == nil {
		t.Fatal("expected non-null (but possibly empty) array")
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty query, got %d", len(results))
	}
}

func TestGraphSearch_KindFilter(t *testing.T) {
	base, st := newTestServer(t)
	seedGraphForServer(t, st)

	resp := get(t, base+"/api/graph/search?q=vehicle&namespace=test&kind=repo")
	var results []map[string]any
	decodeJSON(t, resp, &results)

	for _, r := range results {
		node := r["node"].(map[string]any)
		if node["kind"] != "repo" {
			t.Fatalf("kind filter leaked: got kind=%v", node["kind"])
		}
	}
}

// --- node by key ---

func TestGraphNodeByKey_Found(t *testing.T) {
	base, st := newTestServer(t)
	seedGraphForServer(t, st)

	resp := get(t, base+"/api/graph/nodes?namespace=test&kind=portal&key=RUNTPRO")
	var node map[string]any
	decodeJSON(t, resp, &node)

	if node["key"] != "RUNTPRO" {
		t.Fatalf("want key=RUNTPRO, got %v", node["key"])
	}
	if node["kind"] != "portal" {
		t.Fatalf("want kind=portal, got %v", node["kind"])
	}
}

func TestGraphNodeByKey_NotFound_Returns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp, err := http.Get(base + "/api/graph/nodes?namespace=test&kind=repo&key=ghost")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGraphNodeByKey_AmbiguousNamespace_Returns400(t *testing.T) {
	base, st := newTestServer(t)
	for _, ns := range []string{"ns-a", "ns-b"} {
		if _, _, err := st.UpsertGraphNode(ns, "repo", "Shared/Repo", "Shared Repo", "", nil); err != nil {
			t.Fatalf("seed %s: %v", ns, err)
		}
	}

	resp, err := http.Get(base + "/api/graph/nodes?kind=repo&key=Shared%2FRepo")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for ambiguous key across namespaces, got %d", resp.StatusCode)
	}
}

func TestGraphNodeByKey_MissingParams_Returns400(t *testing.T) {
	base, _ := newTestServer(t)

	resp, err := http.Get(base + "/api/graph/nodes?namespace=test")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing kind/key, got %d", resp.StatusCode)
	}
}

// --- node by ID ---

func TestGraphNodeByID_ReturnsNodeAndNeighbors(t *testing.T) {
	base, st := newTestServer(t)
	seedGraphForServer(t, st)

	// Look up CYRMR's ID via the key endpoint first.
	nodeResp := get(t, base+"/api/graph/nodes?namespace=test&kind=repo&key=ConsultasyReportes/CYRMR")
	var node map[string]any
	decodeJSON(t, nodeResp, &node)
	nodeID := int(node["id"].(float64))

	detailResp := get(t, fmt.Sprintf("%s/api/graph/nodes/%d", base, nodeID))
	var detail map[string]any
	decodeJSON(t, detailResp, &detail)

	if detail["node"] == nil {
		t.Fatal("expected 'node' field in response")
	}
	if detail["relations"] == nil {
		t.Fatal("expected 'relations' field in response")
	}
	count := detail["neighbor_count"].(float64)
	if count < 4 {
		// CYRMR has: out(consumes, belongs_to) in(exposes, implemented_by)
		t.Fatalf("expected >= 4 neighbors for CYRMR, got %.0f", count)
	}
}

func TestGraphNodeByID_NotFound_Returns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp, err := http.Get(base + "/api/graph/nodes/99999")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- neighbors ---

func TestGraphNeighbors_DirectionFilter(t *testing.T) {
	base, st := newTestServer(t)
	seedGraphForServer(t, st)

	nodeResp := get(t, base+"/api/graph/nodes?namespace=test&kind=repo&key=ConsultasyReportes/CYRMR")
	var node map[string]any
	decodeJSON(t, nodeResp, &node)
	nodeID := int(node["id"].(float64))

	// outbound only
	resp := get(t, fmt.Sprintf("%s/api/graph/nodes/%d/neighbors?direction=out", base, nodeID))
	var result map[string]any
	decodeJSON(t, resp, &result)

	nbs := result["neighbors"].([]any)
	for _, nb := range nbs {
		m := nb.(map[string]any)
		if m["direction"] != "out" {
			t.Fatalf("direction filter leaked: got direction=%v", m["direction"])
		}
	}
}

func TestGraphNeighbors_UnknownNode_Returns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp, err := http.Get(base + "/api/graph/nodes/99999/neighbors")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- impact ---

func TestGraphImpact_SlimPayload(t *testing.T) {
	base, st := newTestServer(t)
	seedGraphForServer(t, st)

	// FirmaApiClient is the deepest node; everything in the chain depends on it.
	nodeResp := get(t, base+"/api/graph/nodes?namespace=test&kind=repo&key=RNF/FirmaApiClient")
	var node map[string]any
	decodeJSON(t, nodeResp, &node)
	nodeID := int(node["id"].(float64))

	resp := get(t, fmt.Sprintf("%s/api/graph/nodes/%d/impact", base, nodeID))
	var result map[string]any
	decodeJSON(t, resp, &result)

	if result["node"] == nil {
		t.Fatal("expected 'node' field in impact response")
	}
	count := result["impacted_count"].(float64)
	if count < 4 {
		// chain: CYRConsultasVehiculoMS(d1) -> CYRMR(d2) -> RUNTPRO+menu(d3)
		t.Fatalf("expected >= 4 impacted nodes, got %.0f", count)
	}

	// slim row format: must have depth/kind/key/label, must NOT have attrs/summary
	listed := result["impacted"].([]any)
	if len(listed) == 0 {
		t.Fatal("expected non-empty impacted list")
	}
	first := listed[0].(map[string]any)
	for _, required := range []string{"depth", "kind", "key", "label"} {
		if first[required] == nil {
			t.Fatalf("slim row missing field %q", required)
		}
	}
	if _, hasAttrs := first["attrs"]; hasAttrs {
		t.Fatal("slim row must not contain attrs")
	}
}

func TestGraphImpact_KindFilter(t *testing.T) {
	base, st := newTestServer(t)
	seedGraphForServer(t, st)

	nodeResp := get(t, base+"/api/graph/nodes?namespace=test&kind=repo&key=RNF/FirmaApiClient")
	var node map[string]any
	decodeJSON(t, nodeResp, &node)
	nodeID := int(node["id"].(float64))

	resp := get(t, fmt.Sprintf("%s/api/graph/nodes/%d/impact?kind_filter=portal", base, nodeID))
	var result map[string]any
	decodeJSON(t, resp, &result)

	listed := result["impacted"].([]any)
	for _, item := range listed {
		row := item.(map[string]any)
		if row["kind"] != "portal" {
			t.Fatalf("kind_filter leaked: got kind=%v", row["kind"])
		}
	}
	// impacted_count must still reflect total (not filtered) count
	total := result["impacted_count"].(float64)
	if total < float64(len(listed)) {
		t.Fatalf("impacted_count %v is less than listed %d — filtering must not reduce count", total, len(listed))
	}
}

func TestGraphImpact_UnknownNode_Returns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp, err := http.Get(base + "/api/graph/nodes/99999/impact")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- stats ---

func TestGraphStats_ReturnsCountsAndNamespaces(t *testing.T) {
	base, st := newTestServer(t)
	seedGraphForServer(t, st)

	resp := get(t, base+"/api/graph/stats?namespace=test")
	var result map[string]any
	decodeJSON(t, resp, &result)

	stats := result["stats"].(map[string]any)
	if stats["node_count"].(float64) != 6 {
		t.Fatalf("want 6 nodes, got %v", stats["node_count"])
	}
	if stats["edge_count"].(float64) != 5 {
		t.Fatalf("want 5 edges, got %v", stats["edge_count"])
	}

	nsList := result["namespaces"].([]any)
	if len(nsList) != 1 || nsList[0] != "test" {
		t.Fatalf("want ['test'] namespaces, got %v", nsList)
	}
}

func TestGraphStats_Empty_ReturnsZeroes(t *testing.T) {
	base, _ := newTestServer(t)

	resp := get(t, base+"/api/graph/stats")
	var result map[string]any
	decodeJSON(t, resp, &result)

	if result["stats"] == nil {
		t.Fatal("expected 'stats' field")
	}
	if result["namespaces"] == nil {
		t.Fatal("expected 'namespaces' field")
	}
}

// --- namespaces ---

func TestGraphNamespaces(t *testing.T) {
	base, st := newTestServer(t)
	seedGraphForServer(t, st)

	resp := get(t, base+"/api/graph/namespaces")
	var ns []any
	decodeJSON(t, resp, &ns)

	if len(ns) != 1 || ns[0] != "test" {
		t.Fatalf("want ['test'], got %v", ns)
	}
}

func TestGraphNamespaces_Empty_ReturnsEmptyArray(t *testing.T) {
	base, _ := newTestServer(t)

	resp := get(t, base+"/api/graph/namespaces")
	var ns []any
	decodeJSON(t, resp, &ns)

	if ns == nil {
		t.Fatal("expected non-null (but possibly empty) array")
	}
}
