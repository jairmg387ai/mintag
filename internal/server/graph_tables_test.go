package server

import (
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

func seedTableUsageForServer(t *testing.T, st *store.Store) {
	t.Helper()
	for _, n := range []struct{ kind, key string }{
		{"repo", "Team/AlphaMS"},
		{"repo", "Team/BetaMS"},
		{"table", "RUNTPROD.PA_DEPARTAME"},
		{"source_file", "Team/AlphaMS/src/DepartamentoEntity.java"},
		{"source_file", "Team/BetaMS/src/DepartamentoDao.java"},
	} {
		if _, _, err := st.UpsertGraphNode("runt2", n.kind, n.key, n.key, "", nil); err != nil {
			t.Fatalf("seed node %s: %v", n.key, err)
		}
	}
	for _, e := range []struct{ rel, sk, skey, tk, tkey string }{
		{"in_repo", "source_file", "Team/AlphaMS/src/DepartamentoEntity.java", "repo", "Team/AlphaMS"},
		{"in_repo", "source_file", "Team/BetaMS/src/DepartamentoDao.java", "repo", "Team/BetaMS"},
		{"uses_table", "source_file", "Team/AlphaMS/src/DepartamentoEntity.java", "table", "RUNTPROD.PA_DEPARTAME"},
		{"uses_table", "source_file", "Team/BetaMS/src/DepartamentoDao.java", "table", "RUNTPROD.PA_DEPARTAME"},
	} {
		if _, err := st.UpsertGraphEdge("runt2", e.rel, e.sk, e.skey, e.tk, e.tkey, nil); err != nil {
			t.Fatalf("seed edge %s: %v", e.rel, err)
		}
	}
}

func TestGraphTables_ReturnsUsageGroupedByRepo(t *testing.T) {
	base, st := newTestServer(t)
	seedTableUsageForServer(t, st)

	resp := get(t, base+"/api/graph/tables?q=pa_departame&namespace=runt2")
	var result store.TableUsageResult
	decodeJSON(t, resp, &result)

	if len(result.Tables) != 1 || result.Tables[0].Key != "RUNTPROD.PA_DEPARTAME" {
		t.Fatalf("want table RUNTPROD.PA_DEPARTAME, got %+v", result.Tables)
	}
	if result.Tables[0].RepoCount != 2 || result.Tables[0].FileCount != 2 {
		t.Fatalf("want 2 repos / 2 files, got %d / %d", result.Tables[0].RepoCount, result.Tables[0].FileCount)
	}
}

func TestGraphTables_RepoFilterAndFileCap(t *testing.T) {
	base, st := newTestServer(t)
	seedTableUsageForServer(t, st)

	resp := get(t, base+"/api/graph/tables?q=PA_DEPARTAME&repo=Team/BetaMS")
	var result store.TableUsageResult
	decodeJSON(t, resp, &result)
	if len(result.Tables) != 1 || len(result.Tables[0].Repos) != 1 || result.Tables[0].Repos[0].Repo != "Team/BetaMS" {
		t.Fatalf("repo filter not applied: %+v", result.Tables)
	}

	resp = get(t, base+"/api/graph/tables?q=PA_DEPARTAME&max_files=1")
	decodeJSON(t, resp, &result)
	if len(result.Tables) != 1 || !result.Tables[0].FilesTruncated {
		t.Fatalf("expected files_truncated with max_files=1, got %+v", result.Tables)
	}
}

func TestGraphTables_EmptyQuery_ReturnsEmptyTables(t *testing.T) {
	base, _ := newTestServer(t)

	resp := get(t, base+"/api/graph/tables")
	var result map[string]any
	decodeJSON(t, resp, &result)
	tables, ok := result["tables"].([]any)
	if !ok || len(tables) != 0 {
		t.Fatalf("expected empty tables array, got %v", result["tables"])
	}
}
