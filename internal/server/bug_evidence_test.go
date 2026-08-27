package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Gentleman-Programming/mintag/internal/azure"
)

// bugEvidenceFixtureJSON builds a $expand=all-shaped work item response body
// for the given bug-evidence field values, matching what FetchBugEvidence
// expects to parse (see internal/azure/bug_evidence.go's bugEvidenceFields).
func bugEvidenceFixtureJSON(id, rev int, workItemType, state, teamProject, causaRaiz string, identificada bool, solucion string, temporal, definitiva bool) string {
	b, _ := json.Marshal(map[string]any{
		"id":  id,
		"rev": rev,
		"fields": map[string]any{
			"System.State":                   state,
			"System.TeamProject":             teamProject,
			"System.Title":                   "Bug de prueba",
			"System.WorkItemType":            workItemType,
			azure.FieldCausaRaiz:             causaRaiz,
			azure.FieldCausaRaizIdentificada: identificada,
			azure.FieldSolucionDefinitiva:    solucion,
			azure.FieldTipoTemporal:          temporal,
			azure.FieldTipoDefinitiva:        definitiva,
		},
	})
	return string(b)
}

// azureConnectionDataOK is the shared connectiondata fixture every
// newTestServerWithAzureRedirect test needs to satisfy resolveAzureIdentity.
func azureConnectionDataOK(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"authenticatedUser":{"id":"id","providerDisplayName":"Name"}}`)) //nolint:errcheck
}

func setupBugEvidenceTestServer(t *testing.T, azureHandler http.HandlerFunc) (base string) {
	t.Helper()
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")

	azureServer := httptest.NewServer(azureHandler)
	t.Cleanup(azureServer.Close)

	base, _ = newTestServerWithAzureRedirect(t, azureServer.URL)
	putResp := doJSON(t, http.MethodPut, base+"/api/activities/azure-config", map[string]any{"token": "db-token", "auth_mode": "bearer"})
	assertStatus(t, putResp, http.StatusOK)
	return base
}

// --- requireLocalRequest guard ---

func TestBugEvidenceRoutesRequireLocalRequest(t *testing.T) {
	_, st := newTestServer(t)
	h := New(st).Handler()

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/azure/bugs/4242/evidence"},
		{http.MethodPatch, "/api/azure/bugs/4242/evidence"},
		{http.MethodGet, "/api/azure/bugs/4242/comments"},
		{http.MethodPost, "/api/azure/bugs/4242/comments"},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, "http://127.0.0.1"+rt.path, strings.NewReader(`{}`))
			req.Host = "evil.example"
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403 for non-local host, got %d", rec.Code)
			}
		})
	}
}

// --- GET /api/azure/bugs/{id}/evidence ---

func TestGetBugEvidence_Success(t *testing.T) {
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 7, "Bug", "Activo", "RUNTPRO", "root cause", true, "fix", false, true))) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	resp := get(t, base+"/api/azure/bugs/4242/evidence")
	var body struct {
		ID          int    `json:"id"`
		Rev         int    `json:"rev"`
		State       string `json:"state"`
		TeamProject string `json:"team_project"`
		Title       string `json:"title"`
		Editable    bool   `json:"editable"`
		Fields      struct {
			CausaRaiz             string `json:"causa_raiz"`
			CausaRaizIdentificada bool   `json:"causa_raiz_identificada"`
			SolucionDefinitiva    string `json:"solucion_definitiva"`
			TipoSolucion          string `json:"tipo_solucion"`
		} `json:"fields"`
	}
	decodeJSON(t, resp, &body)
	if body.ID != 4242 || body.Rev != 7 || body.State != "Activo" {
		t.Errorf("unexpected response: %+v", body)
	}
	if !body.Editable {
		t.Error("expected editable=true for state Activo")
	}
	if body.TeamProject != "RUNTPRO" || body.Title != "Bug de prueba" {
		t.Errorf("expected team_project/title to be parsed, got %+v", body)
	}
	if body.Fields.CausaRaiz != "root cause" || !body.Fields.CausaRaizIdentificada || body.Fields.SolucionDefinitiva != "fix" {
		t.Errorf("unexpected fields: %+v", body.Fields)
	}
	if body.Fields.TipoSolucion != "definitiva" {
		t.Errorf("expected tipo_solucion=definitiva, got %q", body.Fields.TipoSolucion)
	}
}

func TestGetBugEvidence_ReadOnlyStateReportsNotEditable(t *testing.T) {
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 3, "Bug", "Cerrado", "RUNTPRO", "cr", true, "sd", false, true))) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	resp := get(t, base+"/api/azure/bugs/4242/evidence")
	var body struct {
		Editable bool `json:"editable"`
	}
	decodeJSON(t, resp, &body)
	if body.Editable {
		t.Error("expected editable=false for state Cerrado")
	}
}

func TestGetBugEvidence_NotABug_Returns400(t *testing.T) {
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 1, "Task", "Active", "RUNTPRO", "", false, "", false, false))) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	resp, err := http.Get(base + "/api/azure/bugs/4242/evidence")
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusBadRequest)
	var body struct {
		Code string `json:"code"`
	}
	decodeJSON(t, resp, &body)
	if body.Code != "not_a_bug" {
		t.Errorf("expected code=not_a_bug, got %q", body.Code)
	}
}

func TestGetBugEvidence_NotFound_Returns404(t *testing.T) {
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "connectiondata") {
			azureConnectionDataOK(w)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	resp, err := http.Get(base + "/api/azure/bugs/9999/evidence")
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestGetBugEvidence_ServerErrorMentioning404InBody_Returns502NotMisclassifiedAs404(t *testing.T) {
	// A genuine 500 whose error body happens to echo unrelated text
	// containing "status 404" (e.g. an upstream dependency's own error)
	// must not be misclassified as "work item not found".
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "connectiondata") {
			azureConnectionDataOK(w)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"upstream dependency reported status 404 for an unrelated resource"}`)) //nolint:errcheck
	})

	resp, err := http.Get(base + "/api/azure/bugs/4242/evidence")
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusBadGateway)
}

func TestGetBugEvidence_NotConfigured_Returns503(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	base, _ := newTestServer(t)

	resp, err := http.Get(base + "/api/azure/bugs/4242/evidence")
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusServiceUnavailable)
}

// --- PATCH /api/azure/bugs/{id}/evidence ---

func TestPatchBugEvidence_Success(t *testing.T) {
	var patchCount int
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 7, "Bug", "Activo", "RUNTPRO", "old root cause", false, "", false, false))) //nolint:errcheck
		case r.Method == http.MethodPatch:
			patchCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 8, "Bug", "Activo", "RUNTPRO", "new root cause", true, "the fix", false, true))) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	patchResp := doJSON(t, http.MethodPatch, base+"/api/azure/bugs/4242/evidence", map[string]any{
		"rev": 7,
		"fields": map[string]any{
			"causa_raiz":              "new root cause",
			"causa_raiz_identificada": true,
			"solucion_definitiva":     "the fix",
			"tipo_solucion":           "definitiva",
		},
	})
	assertStatus(t, patchResp, http.StatusOK)

	var body struct {
		Rev        int  `json:"rev"`
		Reaffirmed bool `json:"reaffirmed"`
		Fields     struct {
			CausaRaiz string `json:"causa_raiz"`
		} `json:"fields"`
	}
	decodeJSON(t, patchResp, &body)
	if body.Rev != 8 || body.Reaffirmed {
		t.Errorf("unexpected response: %+v", body)
	}
	if body.Fields.CausaRaiz != "new root cause" {
		t.Errorf("expected updated causa_raiz, got %q", body.Fields.CausaRaiz)
	}
	if patchCount != 1 {
		t.Errorf("expected exactly 1 PATCH call, got %d", patchCount)
	}
}

func TestPatchBugEvidence_StateNotEditable_Returns409WithoutPatching(t *testing.T) {
	patchCalled := false
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 3, "Bug", "Cerrado", "RUNTPRO", "cr", true, "sd", false, true))) //nolint:errcheck
		case r.Method == http.MethodPatch:
			patchCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	patchResp := doJSON(t, http.MethodPatch, base+"/api/azure/bugs/4242/evidence", map[string]any{
		"rev":    3,
		"fields": map[string]any{"causa_raiz": "attempted edit"},
	})
	assertStatus(t, patchResp, http.StatusConflict)
	var body struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	decodeJSON(t, patchResp, &body)
	if body.Code != "state_not_editable" || body.State != "Cerrado" {
		t.Errorf("unexpected error body: %+v", body)
	}
	if patchCalled {
		t.Error("expected no PATCH call — server must re-check editable state before writing, regardless of client gating")
	}
}

func TestPatchBugEvidence_RootCauseRequired_Returns422WithoutPatching(t *testing.T) {
	patchCalled := false
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			// current causa_raiz is empty, and the PATCH body below does not
			// set it either — so the effective causa_raiz stays empty.
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 5, "Bug", "Activo", "RUNTPRO", "", false, "", false, false))) //nolint:errcheck
		case r.Method == http.MethodPatch:
			patchCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	patchResp := doJSON(t, http.MethodPatch, base+"/api/azure/bugs/4242/evidence", map[string]any{
		"rev":    5,
		"fields": map[string]any{"causa_raiz_identificada": true},
	})
	assertStatus(t, patchResp, http.StatusUnprocessableEntity)
	var body struct {
		Code string `json:"code"`
	}
	decodeJSON(t, patchResp, &body)
	if body.Code != "root_cause_required" {
		t.Errorf("expected code=root_cause_required, got %+v", body)
	}
	if patchCalled {
		t.Error("expected no PATCH call — the root-cause invariant must block before contacting Azure")
	}
}

func TestPatchBugEvidence_RevConflict_Returns409(t *testing.T) {
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 9, "Bug", "Activo", "RUNTPRO", "someone else's edit", false, "", false, false))) //nolint:errcheck
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message":"The rev value 7 does not match the current value 9; test operation failed."}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	patchResp := doJSON(t, http.MethodPatch, base+"/api/azure/bugs/4242/evidence", map[string]any{
		"rev":    7,
		"fields": map[string]any{"causa_raiz": "my local edit"},
	})
	assertStatus(t, patchResp, http.StatusConflict)
	var body struct {
		Code   string         `json:"code"`
		Remote map[string]any `json:"remote"`
	}
	decodeJSON(t, patchResp, &body)
	if body.Code != "rev_conflict" {
		t.Errorf("expected code=rev_conflict, got %+v", body)
	}
	if body.Remote == nil {
		t.Error("expected a non-nil remote snapshot in the conflict response")
	}
}

func TestPatchBugEvidence_InsufficientScope_Returns403(t *testing.T) {
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 4, "Bug", "Activo", "RUNTPRO", "cr", false, "", false, false))) //nolint:errcheck
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"not authorized"}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	patchResp := doJSON(t, http.MethodPatch, base+"/api/azure/bugs/4242/evidence", map[string]any{
		"rev":    4,
		"fields": map[string]any{"causa_raiz": "edit"},
	})
	assertStatus(t, patchResp, http.StatusForbidden)
	var body struct {
		Code string `json:"code"`
	}
	decodeJSON(t, patchResp, &body)
	if body.Code != "insufficient_scope" {
		t.Errorf("expected code=insufficient_scope, got %+v", body)
	}
}

func TestPatchBugEvidence_NotConfigured_Returns503(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPatch, base+"/api/azure/bugs/4242/evidence", map[string]any{
		"rev":    1,
		"fields": map[string]any{"causa_raiz": "x"},
	})
	assertStatus(t, resp, http.StatusServiceUnavailable)
}

// --- GET /api/azure/bugs/{id}/comments ---

func TestListBugComments_Success(t *testing.T) {
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"totalCount":1,"comments":[{"id":10,"text":"<p>hi</p>","createdBy":{"displayName":"Alice"},"createdDate":"2026-08-01T10:00:00Z"}]}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 1, "Bug", "Activo", "BUGS-PROJECT", "cr", false, "", false, false))) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	resp := get(t, base+"/api/azure/bugs/4242/comments")
	var comments []struct {
		ID          int64  `json:"id"`
		Text        string `json:"text"`
		CreatedBy   string `json:"created_by"`
		CreatedDate string `json:"created_date"`
	}
	decodeJSON(t, resp, &comments)
	if len(comments) != 1 || comments[0].ID != 10 || comments[0].CreatedBy != "Alice" {
		t.Errorf("unexpected comments: %+v", comments)
	}
}

func TestListBugComments_NotConfigured_Returns503(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	base, _ := newTestServer(t)

	resp, err := http.Get(base + "/api/azure/bugs/4242/comments")
	mustNoErr(t, err)
	assertStatus(t, resp, http.StatusServiceUnavailable)
}

// --- POST /api/azure/bugs/{id}/comments ---

func TestPostBugComment_Success_AndIdempotentReplay(t *testing.T) {
	var postCommentCount int
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/comments"):
			postCommentCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":77,"text":"hello","createdBy":{"displayName":"Carol"},"createdDate":"2026-08-03T12:00:00Z"}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 1, "Bug", "Activo", "RUNTPRO", "cr", false, "", false, false))) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	firstResp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/4242/comments", map[string]any{
		"idempotency_key": "k1",
		"text":            "hello",
	})
	assertStatus(t, firstResp, http.StatusCreated)
	var firstBody struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, firstResp, &firstBody)
	if firstBody.ID != 77 {
		t.Fatalf("expected comment id=77, got %d", firstBody.ID)
	}

	secondResp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/4242/comments", map[string]any{
		"idempotency_key": "k1",
		"text":            "hello",
	})
	assertStatus(t, secondResp, http.StatusOK)
	var secondBody struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, secondResp, &secondBody)
	if secondBody.ID != 77 {
		t.Errorf("expected the replayed response to carry the same comment id=77, got %d", secondBody.ID)
	}
	if postCommentCount != 1 {
		t.Errorf("expected exactly 1 Azure POST comment call (idempotent replay must not call Azure again), got %d", postCommentCount)
	}
}

func TestPostBugComment_StateNotEditable_Returns409WithoutPostingToAzure(t *testing.T) {
	postCalled := false
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/comments"):
			postCalled = true
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 1, "Bug", "Cerrado", "RUNTPRO", "cr", true, "sd", false, true))) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/4242/comments", map[string]any{
		"idempotency_key": "k-blocked",
		"text":            "should not post",
	})
	assertStatus(t, resp, http.StatusConflict)
	var body struct {
		Code string `json:"code"`
	}
	decodeJSON(t, resp, &body)
	if body.Code != "state_not_editable" {
		t.Errorf("expected code=state_not_editable, got %+v", body)
	}
	if postCalled {
		t.Error("expected no Azure comment POST for a read-only state")
	}
}

func TestPostBugComment_InsufficientScope_Returns403(t *testing.T) {
	base := setupBugEvidenceTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "connectiondata"):
			azureConnectionDataOK(w)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/comments"):
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"not authorized"}`)) //nolint:errcheck
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_apis/wit/workitems/4242"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(bugEvidenceFixtureJSON(4242, 1, "Bug", "Activo", "RUNTPRO", "cr", false, "", false, false))) //nolint:errcheck
		default:
			t.Fatalf("unexpected azure request: %s %s", r.Method, r.URL.Path)
		}
	})

	resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/4242/comments", map[string]any{
		"idempotency_key": "k-forbidden",
		"text":            "text",
	})
	assertStatus(t, resp, http.StatusForbidden)
	var body struct {
		Code string `json:"code"`
	}
	decodeJSON(t, resp, &body)
	if body.Code != "insufficient_scope" {
		t.Errorf("expected code=insufficient_scope, got %+v", body)
	}
}

func TestPostBugComment_NotConfigured_Returns503(t *testing.T) {
	t.Setenv("MINTAG_AZURE_TIMELOG_TOKEN", "")
	t.Setenv("MINTAG_AZURE_TIMELOG_PAT", "")
	base, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, base+"/api/azure/bugs/4242/comments", map[string]any{
		"idempotency_key": "k",
		"text":            "t",
	})
	assertStatus(t, resp, http.StatusServiceUnavailable)
}
