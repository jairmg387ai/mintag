package store

import (
	"testing"
)

type tableUsageSeed struct {
	file, repo string
	tables     []string
	viaEdge    bool // link repo through an in_repo edge (else only attrs.repo)
}

// seedTableGraph builds a RUNT2-shaped table-usage graph in namespace ns.
func seedTableGraph(t *testing.T, s *Store, ns string, tables []string, usages []tableUsageSeed) {
	t.Helper()
	for _, tb := range tables {
		if _, _, err := s.UpsertGraphNode(ns, "table", tb, tb, "", nil); err != nil {
			t.Fatalf("seed table %s: %v", tb, err)
		}
	}
	for _, u := range usages {
		if _, _, err := s.UpsertGraphNode(ns, "repo", u.repo, u.repo, "", nil); err != nil {
			t.Fatalf("seed repo %s: %v", u.repo, err)
		}
		if _, _, err := s.UpsertGraphNode(ns, "source_file", u.file, u.file, "", map[string]any{"repo": u.repo}); err != nil {
			t.Fatalf("seed file %s: %v", u.file, err)
		}
		if u.viaEdge {
			if _, err := s.UpsertGraphEdge(ns, "in_repo", "source_file", u.file, "repo", u.repo, nil); err != nil {
				t.Fatalf("seed in_repo %s: %v", u.file, err)
			}
		}
		for _, tb := range u.tables {
			attrs := map[string]any{"evidence": []any{
				map[string]any{"path": u.file, "line": 42, "type": "sql_from", "text": "FROM " + tb},
			}}
			if _, err := s.UpsertGraphEdge(ns, "uses_table", "source_file", u.file, "table", tb, attrs); err != nil {
				t.Fatalf("seed uses_table %s->%s: %v", u.file, tb, err)
			}
		}
	}
}

func openTableStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	seedTableGraph(t, s, "runt2",
		[]string{"RUNTPROD.PA_DEPARTAME", "RUNTPROD.PA_DEPARTAMENTO_HIST", "RUNTPROD.PAXDEPARTAME", "PA_D"},
		[]tableUsageSeed{
			{"src/b/Dao.java", "BlockChain/BCAdministracionMS", []string{"RUNTPROD.PA_DEPARTAME", "PA_D"}, true},
			{"src/a/Entity.java", "BlockChain/BCAdministracionMS", []string{"RUNTPROD.PA_DEPARTAME"}, true},
			{"src/c/Repo.java", "Other/OtherMS", []string{"RUNTPROD.PA_DEPARTAME", "RUNTPROD.PAXDEPARTAME"}, false},
			{"src/d/Hist.java", "Other/OtherMS", []string{"RUNTPROD.PA_DEPARTAMENTO_HIST"}, true},
		})
	return s
}

func tableKeys(r *TableUsageResult) []string {
	out := make([]string, 0, len(r.Tables))
	for _, tb := range r.Tables {
		out = append(out, tb.Key)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSearchTableUsageEmptyQuery(t *testing.T) {
	s := openTableStore(t)
	if _, err := s.SearchTableUsage("", "   ", "", 0, 0); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSearchTableUsageCaseInsensitiveSubstring(t *testing.T) {
	s := openTableStore(t)
	r, err := s.SearchTableUsage("", "departame", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"RUNTPROD.PAXDEPARTAME", "RUNTPROD.PA_DEPARTAME", "RUNTPROD.PA_DEPARTAMENTO_HIST"}
	if got := tableKeys(r); !equalStrings(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
	if r.Truncated {
		t.Fatal("unexpected truncation")
	}
}

func TestSearchTableUsageEscapesUnderscore(t *testing.T) {
	s := openTableStore(t)
	r, err := s.SearchTableUsage("", "pa_d", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// exact match "PA_D" ranks first; PAXDEPARTAME must not match.
	want := []string{"PA_D", "RUNTPROD.PA_DEPARTAME", "RUNTPROD.PA_DEPARTAMENTO_HIST"}
	if got := tableKeys(r); !equalStrings(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
}

func TestSearchTableUsageExactMatchFirst(t *testing.T) {
	s := openTableStore(t)
	r, err := s.SearchTableUsage("", "runtprod.pa_departame", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"RUNTPROD.PA_DEPARTAME", "RUNTPROD.PA_DEPARTAMENTO_HIST"}
	if got := tableKeys(r); !equalStrings(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
}

func TestSearchTableUsageGrouping(t *testing.T) {
	s := openTableStore(t)
	r, err := s.SearchTableUsage("runt2", "RUNTPROD.PA_DEPARTAME", "", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tables) != 1 {
		t.Fatalf("tables = %v", tableKeys(r))
	}
	if !r.Truncated {
		t.Fatal("expected table-limit truncation (2 matches, limit 1)")
	}
	tb := r.Tables[0]
	if tb.Namespace != "runt2" || tb.Label != "RUNTPROD.PA_DEPARTAME" {
		t.Fatalf("table meta = %+v", tb)
	}
	if tb.FileCount != 3 || tb.RepoCount != 2 || tb.FilesTruncated {
		t.Fatalf("counts file=%d repo=%d trunc=%v", tb.FileCount, tb.RepoCount, tb.FilesTruncated)
	}
	if len(tb.Repos) != 2 || tb.Repos[0].Repo != "BlockChain/BCAdministracionMS" || tb.Repos[1].Repo != "Other/OtherMS" {
		t.Fatalf("repos = %+v", tb.Repos)
	}
	bc := tb.Repos[0]
	if len(bc.Files) != 2 || bc.Files[0].Path != "src/a/Entity.java" || bc.Files[1].Path != "src/b/Dao.java" {
		t.Fatalf("bc files = %+v", bc.Files)
	}
	ev := bc.Files[0].Evidence
	if len(ev) != 1 || ev[0].Path != "src/a/Entity.java" || ev[0].Line != 42 || ev[0].Type != "sql_from" || ev[0].Text != "FROM RUNTPROD.PA_DEPARTAME" {
		t.Fatalf("evidence = %+v", ev)
	}
	// repo resolved from attrs.repo fallback when no in_repo edge exists
	if len(tb.Repos[1].Files) != 1 || tb.Repos[1].Files[0].Path != "src/c/Repo.java" {
		t.Fatalf("other files = %+v", tb.Repos[1].Files)
	}
}

func TestSearchTableUsageRepoFilter(t *testing.T) {
	s := openTableStore(t)
	r, err := s.SearchTableUsage("", "DEPARTAME", "Other/OtherMS", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"RUNTPROD.PAXDEPARTAME", "RUNTPROD.PA_DEPARTAME", "RUNTPROD.PA_DEPARTAMENTO_HIST"}
	if got := tableKeys(r); !equalStrings(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
	tb := r.Tables[1] // RUNTPROD.PA_DEPARTAME is used in both repos
	if tb.FileCount != 1 || tb.RepoCount != 1 || len(tb.Repos) != 1 || tb.Repos[0].Repo != "Other/OtherMS" {
		t.Fatalf("filtered table = %+v", tb)
	}

	r, err = s.SearchTableUsage("", "PA_D", "Other/OtherMS", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// PA_D is only used in BlockChain, so it is omitted under this filter.
	want = []string{"RUNTPROD.PA_DEPARTAME", "RUNTPROD.PA_DEPARTAMENTO_HIST"}
	if got := tableKeys(r); !equalStrings(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
}

func TestSearchTableUsageMaxFilesTruncation(t *testing.T) {
	s := openTableStore(t)
	r, err := s.SearchTableUsage("", "RUNTPROD.PA_DEPARTAME", "", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	tb := r.Tables[0]
	if !tb.FilesTruncated || !r.Truncated {
		t.Fatalf("expected files truncation, table=%v result=%v", tb.FilesTruncated, r.Truncated)
	}
	if tb.FileCount != 3 || tb.RepoCount != 2 {
		t.Fatalf("counts must cover all usages: file=%d repo=%d", tb.FileCount, tb.RepoCount)
	}
	listed := 0
	for _, rp := range tb.Repos {
		listed += len(rp.Files)
	}
	if listed != 2 {
		t.Fatalf("listed files = %d, want 2", listed)
	}
}

func TestSearchTableUsageNamespaceFilter(t *testing.T) {
	s := openTableStore(t)
	seedTableGraph(t, s, "other", []string{"RUNTPROD.PA_DEPARTAME"}, nil)

	r, err := s.SearchTableUsage("", "RUNTPROD.PA_DEPARTAME", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tables) != 3 {
		t.Fatalf("all namespaces: tables = %v", tableKeys(r))
	}
	r, err = s.SearchTableUsage("other", "RUNTPROD.PA_DEPARTAME", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tables) != 1 || r.Tables[0].Namespace != "other" || r.Tables[0].FileCount != 0 {
		t.Fatalf("namespace filter: %+v", r.Tables)
	}
}
