package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestListMenuOptions_ReturnsResolvedCatalog verifies GET /api/menu-options
// returns 200 with the full catalog shape (id/label/enabled), resolved
// through DefaultEnabled fallback on a fresh in-memory store (no overrides).
func TestListMenuOptions_ReturnsResolvedCatalog(t *testing.T) {
	base, _ := newTestServer(t)

	resp := get(t, base+"/api/menu-options")
	var options []map[string]any
	decodeJSON(t, resp, &options)

	if len(options) == 0 {
		t.Fatal("expected non-empty menu options catalog")
	}

	byID := map[string]map[string]any{}
	for _, o := range options {
		id, _ := o["id"].(string)
		byID[id] = o
	}

	for _, id := range []string{"dashboard", "tasks", "activities"} {
		o, ok := byID[id]
		if !ok {
			t.Fatalf("expected catalog entry %q, got %#v", id, options)
		}
		if o["enabled"] != true {
			t.Errorf("expected %q enabled=true by default, got %#v", id, o["enabled"])
		}
		if _, ok := o["label"]; !ok {
			t.Errorf("expected %q to have a label field, got %#v", id, o)
		}
	}

	for _, id := range []string{"meetings", "graph", "deployment-windows"} {
		o, ok := byID[id]
		if !ok {
			t.Fatalf("expected catalog entry %q, got %#v", id, options)
		}
		if o["enabled"] != false {
			t.Errorf("expected %q enabled=false by default, got %#v", id, o["enabled"])
		}
	}
}

// TestPatchMenuOption_TogglesEnabled verifies PATCH /api/menu-options/{id}
// disables and re-enables a non-critical option, returning 200 with the
// updated resolved status each time.
func TestPatchMenuOption_TogglesEnabled(t *testing.T) {
	base, _ := newTestServer(t)

	disableResp := doJSON(t, http.MethodPatch, base+"/api/menu-options/tasks", map[string]any{
		"enabled": false,
	})
	assertStatus(t, disableResp, http.StatusOK)
	var disabled map[string]any
	decodeJSON(t, disableResp, &disabled)
	if disabled["enabled"] != false {
		t.Fatalf("expected enabled=false after disable, got %#v", disabled)
	}
	if disabled["id"] != "tasks" {
		t.Fatalf("expected id=tasks, got %#v", disabled)
	}

	enableResp := doJSON(t, http.MethodPatch, base+"/api/menu-options/tasks", map[string]any{
		"enabled": true,
	})
	assertStatus(t, enableResp, http.StatusOK)
	var enabled map[string]any
	decodeJSON(t, enableResp, &enabled)
	if enabled["enabled"] != true {
		t.Fatalf("expected enabled=true after re-enable, got %#v", enabled)
	}
}

// TestPatchMenuOption_MissingEnabledReturns400 verifies a malformed/absent
// "enabled" field is rejected with 400, distinguishable from 404/422.
func TestPatchMenuOption_MissingEnabledReturns400(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPatch, base+"/api/menu-options/tasks", map[string]any{})
	assertStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}

// TestPatchMenuOption_MalformedJSONReturns400 verifies a body that fails to
// decode at all is also a 400, not a 500.
func TestPatchMenuOption_MalformedJSONReturns400(t *testing.T) {
	base, _ := newTestServer(t)

	req, err := http.NewRequest(http.MethodPatch, base+"/api/menu-options/tasks", strings.NewReader(`{not valid json`))
	mustNoErr(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}

// TestPatchMenuOption_UnknownIDReturns404 verifies an id absent from the
// code-defined catalog is rejected with 404, not 422 or 500.
func TestPatchMenuOption_UnknownIDReturns404(t *testing.T) {
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPatch, base+"/api/menu-options/does-not-exist", map[string]any{
		"enabled": false,
	})
	assertStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()
}

// TestPatchMenuOption_LastEnabledReturns422 verifies the sidebar can never
// be left with zero enabled options: disabling down to a single enabled
// option, then attempting to disable that one, returns 422 and leaves state
// unchanged.
func TestPatchMenuOption_LastEnabledReturns422(t *testing.T) {
	base, _ := newTestServer(t)

	// Default catalog on a fresh store: dashboard, tasks, activities enabled.
	// Disable tasks and activities, leaving only dashboard enabled.
	for _, id := range []string{"tasks", "activities"} {
		resp := doJSON(t, http.MethodPatch, base+"/api/menu-options/"+id, map[string]any{
			"enabled": false,
		})
		assertStatus(t, resp, http.StatusOK)
		resp.Body.Close()
	}

	resp := doJSON(t, http.MethodPatch, base+"/api/menu-options/dashboard", map[string]any{
		"enabled": false,
	})
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	resp.Body.Close()

	// State must remain unchanged: dashboard is still the sole enabled option.
	listResp := get(t, base+"/api/menu-options")
	var options []map[string]any
	decodeJSON(t, listResp, &options)
	for _, o := range options {
		if o["id"] == "dashboard" && o["enabled"] != true {
			t.Fatalf("expected dashboard to remain enabled after rejected disable, got %#v", o)
		}
	}
}

// TestPatchMenuOption_RequiresLocalRequest verifies the mutating PATCH route
// is gated the same way as other mutating routes (e.g. azure-catalog):
// non-local Host is rejected with 403 before the store is ever touched.
func TestPatchMenuOption_RequiresLocalRequest(t *testing.T) {
	_, st := newTestServer(t)
	h := New(st).Handler()

	req := httptest.NewRequest(http.MethodPatch, "http://127.0.0.1/api/menu-options/tasks", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "evil.example"
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for non-local host, got %d", rec.Code)
	}
}
