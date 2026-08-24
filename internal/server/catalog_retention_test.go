package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/store"
)

// TestListAzureActivities_IncludeInactiveQueryParam verifies GET
// /api/activities/azure-catalog only returns active rows by default, and
// includes deactivated ones when ?include_inactive=true is passed.
func TestListAzureActivities_IncludeInactiveQueryParam(t *testing.T) {
	base, st := newTestServer(t)
	ctx := context.Background()

	added, err := st.AddAzureActivity(ctx, "RUNT2QA", 999201, "To Deactivate", "", store.AzureActivityMapping{})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeactivateAzureActivity(ctx, added.ID); err != nil {
		t.Fatal(err)
	}

	activeResp := get(t, base+"/api/activities/azure-catalog")
	var active []map[string]any
	decodeJSON(t, activeResp, &active)
	for _, a := range active {
		if int64(a["id"].(float64)) == added.ID {
			t.Fatalf("expected deactivated activity %d excluded by default", added.ID)
		}
	}

	allResp := get(t, base+"/api/activities/azure-catalog?include_inactive=true")
	var all []map[string]any
	decodeJSON(t, allResp, &all)
	var found bool
	for _, a := range all {
		if int64(a["id"].(float64)) == added.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected deactivated activity %d present with include_inactive=true", added.ID)
	}
}

// TestActivityCatalog_ProjectsShapeAndIncludeInactive verifies the
// /api/activities/catalog "projects" field is a {name, is_active}[] and
// respects ?include_inactive.
func TestActivityCatalog_ProjectsShapeAndIncludeInactive(t *testing.T) {
	base, st := newTestServer(t)
	ctx := context.Background()

	if err := st.AddTimelogProject("Active Proj"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddTimelogProject("Inactive Proj"); err != nil {
		t.Fatal(err)
	}
	if err := st.DeactivateTimelogProject(ctx, "Inactive Proj"); err != nil {
		t.Fatal(err)
	}

	type catalogResp struct {
		Projects []struct {
			Name     string `json:"name"`
			IsActive bool   `json:"is_active"`
		} `json:"projects"`
	}

	defaultResp := get(t, base+"/api/activities/catalog")
	var defaultCatalog catalogResp
	decodeJSON(t, defaultResp, &defaultCatalog)
	for _, p := range defaultCatalog.Projects {
		if p.Name == "Inactive Proj" {
			t.Fatal("expected Inactive Proj excluded from the default (active-only) listing")
		}
		if !p.IsActive {
			t.Fatalf("expected every project in the default listing to have is_active=true, got %+v", p)
		}
	}

	allResp := get(t, base+"/api/activities/catalog?include_inactive=true")
	var allCatalog catalogResp
	decodeJSON(t, allResp, &allCatalog)
	var sawActive, sawInactive bool
	for _, p := range allCatalog.Projects {
		if p.Name == "Active Proj" {
			sawActive = true
			if !p.IsActive {
				t.Errorf("expected Active Proj to have is_active=true, got %+v", p)
			}
		}
		if p.Name == "Inactive Proj" {
			sawInactive = true
			if p.IsActive {
				t.Errorf("expected Inactive Proj to have is_active=false, got %+v", p)
			}
		}
	}
	if !sawActive || !sawInactive {
		t.Fatalf("expected both projects present with include_inactive=true, got %+v", allCatalog.Projects)
	}
}

// TestRemoveCatalogProject_DeactivatesAndReturns404WhenUnknown verifies
// DELETE /api/activities/catalog/projects/{name} soft-deletes instead of
// hard-deleting, and 404s for an unknown name.
func TestRemoveCatalogProject_DeactivatesAndReturns404WhenUnknown(t *testing.T) {
	base, st := newTestServer(t)
	ctx := context.Background()

	if err := st.AddTimelogProject("Removable Proj"); err != nil {
		t.Fatal(err)
	}

	resp := doJSON(t, http.MethodDelete, base+"/api/activities/catalog/projects/Removable%20Proj", nil)
	assertStatus(t, resp, http.StatusNoContent)
	resp.Body.Close()

	// Soft-deleted, not hard-deleted: still visible with include_inactive.
	names, err := st.ListTimelogProjects(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !containsName(names, "Removable Proj") {
		t.Fatalf("expected Removable Proj to still exist (soft-deleted), got %v", names)
	}
	activeNames, err := st.ListTimelogProjects(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if containsName(activeNames, "Removable Proj") {
		t.Fatalf("expected Removable Proj excluded from the active listing after delete, got %v", activeNames)
	}

	notFoundResp := doJSON(t, http.MethodDelete, base+"/api/activities/catalog/projects/Never%20Existed", nil)
	assertStatus(t, notFoundResp, http.StatusNotFound)
	notFoundResp.Body.Close()
}

// TestRemoveCatalogProject_NameWithParensAndSpaces guards against a real
// regression: a name with parentheses alongside percent-encoded spaces (e.g.
// "MT 10 (Escuelas ZD)") makes Go's net/url populate r.URL.RawPath, which chi
// prefers over r.URL.Path for route matching — so chi.URLParam hands back the
// still percent-encoded segment unless the handler unescapes it itself. A
// plain single-word-with-space name (as in the test above) never triggers
// this, since RawPath stays empty for it — hence the separate case.
func TestRemoveCatalogProject_NameWithParensAndSpaces(t *testing.T) {
	base, st := newTestServer(t)
	ctx := context.Background()

	const name = "MT 10 (Escuelas ZD)"
	if err := st.AddTimelogProject(name); err != nil {
		t.Fatal(err)
	}

	resp := doJSON(t, http.MethodDelete, base+"/api/activities/catalog/projects/MT%2010%20(Escuelas%20ZD)", nil)
	assertStatus(t, resp, http.StatusNoContent)
	resp.Body.Close()

	activeNames, err := st.ListTimelogProjects(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if containsName(activeNames, name) {
		t.Fatalf("expected %q excluded from the active listing after delete, got %v", name, activeNames)
	}
}

func containsName(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}

// TestCatalogRetentionSettings_GetDefaultsToNull verifies a fresh store
// reports both retention windows as null (disabled).
func TestCatalogRetentionSettings_GetDefaultsToNull(t *testing.T) {
	base, _ := newTestServer(t)

	resp := get(t, base+"/api/settings/catalog-retention")
	var body struct {
		BugRetentionDays     *int `json:"bug_retention_days"`
		ProjectRetentionDays *int `json:"project_retention_days"`
	}
	decodeJSON(t, resp, &body)
	if body.BugRetentionDays != nil {
		t.Errorf("expected bug_retention_days=null, got %v", *body.BugRetentionDays)
	}
	if body.ProjectRetentionDays != nil {
		t.Errorf("expected project_retention_days=null, got %v", *body.ProjectRetentionDays)
	}
}

// TestCatalogRetentionSettings_PutRoundTrips verifies PUT persists and GET
// reads back the configured values.
func TestCatalogRetentionSettings_PutRoundTrips(t *testing.T) {
	base, _ := newTestServer(t)

	putResp := doJSON(t, http.MethodPut, base+"/api/settings/catalog-retention", map[string]any{
		"bug_retention_days":     14,
		"project_retention_days": 60,
	})
	assertStatus(t, putResp, http.StatusOK)
	var putBody struct {
		BugRetentionDays     *int `json:"bug_retention_days"`
		ProjectRetentionDays *int `json:"project_retention_days"`
	}
	decodeJSON(t, putResp, &putBody)
	if putBody.BugRetentionDays == nil || *putBody.BugRetentionDays != 14 {
		t.Errorf("expected bug_retention_days=14, got %v", putBody.BugRetentionDays)
	}
	if putBody.ProjectRetentionDays == nil || *putBody.ProjectRetentionDays != 60 {
		t.Errorf("expected project_retention_days=60, got %v", putBody.ProjectRetentionDays)
	}

	getResp := get(t, base+"/api/settings/catalog-retention")
	var getBody struct {
		BugRetentionDays     *int `json:"bug_retention_days"`
		ProjectRetentionDays *int `json:"project_retention_days"`
	}
	decodeJSON(t, getResp, &getBody)
	if getBody.BugRetentionDays == nil || *getBody.BugRetentionDays != 14 {
		t.Errorf("expected persisted bug_retention_days=14, got %v", getBody.BugRetentionDays)
	}
	if getBody.ProjectRetentionDays == nil || *getBody.ProjectRetentionDays != 60 {
		t.Errorf("expected persisted project_retention_days=60, got %v", getBody.ProjectRetentionDays)
	}
}

// TestCatalogRetentionSettings_RejectsNonPositive verifies a 0/negative
// value is rejected with 422 (use null to disable, not 0).
func TestCatalogRetentionSettings_RejectsNonPositive(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPut, base+"/api/settings/catalog-retention", map[string]any{
		"bug_retention_days": 0,
	})
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	resp.Body.Close()
}
