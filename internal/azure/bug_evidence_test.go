package azure

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsBugEvidenceEditableState(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		// Editable states (DSW-PR-017 V2, non-terminal / non-read-only).
		{"En Revisión", true},
		{"En Requisitos", true},
		{"Resuelto", true},
		{"Activo", true},
		{"En Pruebas", true},
		{"Solucionado", true},
		{"Pruebas INT", true},
		{"Pendiente Ventana", true},
		{"Corregido", true},
		{"Devuelto", true},

		// Read-only states.
		{"Registrado", false},
		{"Descartado", false},
		{"Cerrado", false},

		// Unknown/unrecognized state must fail closed.
		{"SomeUnknownState", false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := IsBugEvidenceEditableState(tt.state); got != tt.want {
				t.Errorf("IsBugEvidenceEditableState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestFetchBugEvidence_Success_ParsesEvidenceAndFoldsTipoSolucion(t *testing.T) {
	tests := []struct {
		name             string
		temporal         bool
		definitiva       bool
		wantTipoSolucion TipoSolucion
	}{
		{name: "neither flag set", temporal: false, definitiva: false, wantTipoSolucion: TipoSolucionNone},
		{name: "temporal flag set", temporal: true, definitiva: false, wantTipoSolucion: TipoSolucionTemporal},
		{name: "definitiva flag set", temporal: false, definitiva: true, wantTipoSolucion: TipoSolucionDefinitiva},
		{name: "both flags set, definitiva takes priority", temporal: true, definitiva: true, wantTipoSolucion: TipoSolucionDefinitiva},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path, query string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path = r.URL.Path
				query = r.URL.RawQuery
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{
					"id": 4242,
					"rev": 7,
					"fields": {
						"System.State": "Activo",
						"System.TeamProject": "RUNTPRO",
						"System.Title": "Bug de ejemplo",
						"System.WorkItemType": "Bug",
						"Microsoft.VSTS.CMMI.ProposedFix": "Se identificó un null pointer en el servicio X",
						"Custom.832c1387-0208-47b9-bd6d-500d3a7b8019": true,
						"Custom.818d41f3-03fe-4c91-9d6a-eeedb596ffb7": "Se corrigió la validación de entrada",
						"Custom.Temporal": ` + boolJSON(tt.temporal) + `,
						"Custom.Definitiva": ` + boolJSON(tt.definitiva) + `
					}
				}`)) //nolint:errcheck
			}))
			defer srv.Close()

			c := &Client{
				cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
				http: &http.Client{Transport: redirectToServer(srv.URL)},
			}

			got, err := c.FetchBugEvidence(context.Background(), 4242)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(path, "/_apis/wit/workitems/4242") {
				t.Errorf("expected work item path, got %q", path)
			}
			if !strings.Contains(query, "$expand=all") {
				t.Errorf("expected $expand=all in query (not a fields= allowlist), got %q", query)
			}
			if got.ID != 4242 || got.Rev != 7 {
				t.Errorf("expected ID=4242 Rev=7, got ID=%d Rev=%d", got.ID, got.Rev)
			}
			if got.State != "Activo" {
				t.Errorf("expected State=Activo, got %q", got.State)
			}
			if got.TeamProject != "RUNTPRO" {
				t.Errorf("expected TeamProject=RUNTPRO, got %q", got.TeamProject)
			}
			if got.Title != "Bug de ejemplo" || got.Type != "Bug" {
				t.Errorf("expected Title/Type to be parsed, got %+v", got)
			}
			if got.CausaRaiz != "Se identificó un null pointer en el servicio X" {
				t.Errorf("expected CausaRaiz to be parsed, got %q", got.CausaRaiz)
			}
			if !got.CausaRaizIdentificada {
				t.Errorf("expected CausaRaizIdentificada=true, got %v", got.CausaRaizIdentificada)
			}
			if got.SolucionDefinitiva != "Se corrigió la validación de entrada" {
				t.Errorf("expected SolucionDefinitiva to be parsed, got %q", got.SolucionDefinitiva)
			}
			if got.TipoSolucion != tt.wantTipoSolucion {
				t.Errorf("expected TipoSolucion=%q, got %q", tt.wantTipoSolucion, got.TipoSolucion)
			}
		})
	}
}

func TestFetchBugEvidence_Unauthorized_ReturnsGenericAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"token expired","token":"must-not-leak"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	_, err := c.FetchBugEvidence(context.Background(), 4242)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	// Generic (non-scope-specific): same shape/sanitization as every other
	// fetch method's error handling — no leaked raw body, no bespoke
	// "insufficient scope"/"bug evidence scope" wording.
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to mention status 401, got: %v", err)
	}
	if strings.Contains(err.Error(), "must-not-leak") || strings.Contains(err.Error(), "\"token\"") {
		t.Errorf("error leaked raw response body: %v", err)
	}
}

func boolJSON(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestBuildBugEvidenceOps(t *testing.T) {
	allowlist := map[string]bool{
		"/rev":                                  true,
		"/fields/" + FieldCausaRaiz:             true,
		"/fields/" + FieldCausaRaizIdentificada: true,
		"/fields/" + FieldSolucionDefinitiva:    true,
		"/fields/" + FieldTipoTemporal:          true,
		"/fields/" + FieldTipoDefinitiva:        true,
	}

	t.Run("op[0] is always the test/rev op", func(t *testing.T) {
		causaRaiz := "some root cause"
		ops, err := buildBugEvidenceOps(42, BugEvidenceUpdate{CausaRaiz: &causaRaiz})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ops) == 0 {
			t.Fatal("expected at least one op")
		}
		if ops[0].Op != "test" || ops[0].Path != "/rev" || ops[0].Value != 42 {
			t.Errorf("expected op[0]={test,/rev,42}, got %+v", ops[0])
		}
	})

	t.Run("nil fields produce no ops beyond the test/rev op", func(t *testing.T) {
		causaRaiz := "only this field is dirty"
		ops, err := buildBugEvidenceOps(1, BugEvidenceUpdate{CausaRaiz: &causaRaiz})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ops) != 2 {
			t.Fatalf("expected exactly 2 ops (test/rev + 1 dirty field), got %d: %+v", len(ops), ops)
		}
		if ops[1].Path != "/fields/"+FieldCausaRaiz || ops[1].Value != causaRaiz {
			t.Errorf("expected the single dirty field op, got %+v", ops[1])
		}
	})

	t.Run("TipoSolucion writes both booleans atomically", func(t *testing.T) {
		tests := []struct {
			name           string
			tipo           TipoSolucion
			wantTemporal   any
			wantDefinitiva any
		}{
			{name: "temporal", tipo: TipoSolucionTemporal, wantTemporal: true, wantDefinitiva: false},
			{name: "definitiva", tipo: TipoSolucionDefinitiva, wantTemporal: false, wantDefinitiva: true},
			{name: "empty clears both", tipo: TipoSolucionNone, wantTemporal: false, wantDefinitiva: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tipo := tt.tipo
				ops, err := buildBugEvidenceOps(1, BugEvidenceUpdate{TipoSolucion: &tipo})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				values := opValues(ops)
				temporalVal, temporalOK := values["/fields/"+FieldTipoTemporal]
				definitivaVal, definitivaOK := values["/fields/"+FieldTipoDefinitiva]
				if !temporalOK || !definitivaOK {
					t.Fatalf("expected BOTH Temporal and Definitiva ops present as one atomic pair, got %+v", values)
				}
				if temporalVal != tt.wantTemporal {
					t.Errorf("expected Temporal=%v, got %v", tt.wantTemporal, temporalVal)
				}
				if definitivaVal != tt.wantDefinitiva {
					t.Errorf("expected Definitiva=%v, got %v", tt.wantDefinitiva, definitivaVal)
				}
			})
		}
	})

	t.Run("empty update (all fields nil) returns an error", func(t *testing.T) {
		_, err := buildBugEvidenceOps(1, BugEvidenceUpdate{})
		if err == nil {
			t.Fatal("expected an error for an update with no fields set")
		}
	})

	t.Run("allowlist enforcement: every produced op path across all combinations is one of the 5 constants", func(t *testing.T) {
		causaRaiz := "cr"
		identificada := true
		solucion := "sd"
		tipo := TipoSolucionTemporal

		// BugEvidenceUpdate exposes exactly these 4 fields — there is no way
		// to express a System.State (or any other) write through this type,
		// so exercising every non-empty combination of them proves the
		// allowlist holds for the entire reachable input space, not just one
		// example.
		combos := []BugEvidenceUpdate{
			{CausaRaiz: &causaRaiz},
			{CausaRaizIdentificada: &identificada},
			{SolucionDefinitiva: &solucion},
			{TipoSolucion: &tipo},
			{CausaRaiz: &causaRaiz, CausaRaizIdentificada: &identificada},
			{CausaRaiz: &causaRaiz, SolucionDefinitiva: &solucion, TipoSolucion: &tipo},
			{CausaRaiz: &causaRaiz, CausaRaizIdentificada: &identificada, SolucionDefinitiva: &solucion, TipoSolucion: &tipo},
		}
		for i, u := range combos {
			ops, err := buildBugEvidenceOps(1, u)
			if err != nil {
				t.Fatalf("combo %d: unexpected error: %v", i, err)
			}
			for _, op := range ops {
				if !allowlist[op.Path] {
					t.Errorf("combo %d: produced a non-allowlisted path %q — a System.State (or any other) write must be structurally impossible through this function", i, op.Path)
				}
			}
		}
	})
}

func TestDivergentFields(t *testing.T) {
	t.Run("all match returns an empty update", func(t *testing.T) {
		causaRaiz := "same text"
		identificada := true
		solucion := "same fix"
		tipo := TipoSolucionDefinitiva
		got := &BugEvidence{
			CausaRaiz:             causaRaiz,
			CausaRaizIdentificada: identificada,
			SolucionDefinitiva:    solucion,
			TipoSolucion:          tipo,
		}
		diff := divergentFields(BugEvidenceUpdate{
			CausaRaiz:             &causaRaiz,
			CausaRaizIdentificada: &identificada,
			SolucionDefinitiva:    &solucion,
			TipoSolucion:          &tipo,
		}, got)
		if !isEmptyBugEvidenceUpdate(diff) {
			t.Errorf("expected no divergent fields, got %+v", diff)
		}
	})

	t.Run("a mismatched HTML field is flagged", func(t *testing.T) {
		submitted := "<p>submitted root cause</p>"
		got := &BugEvidence{CausaRaiz: "<p>a different root cause was echoed back</p>"}
		diff := divergentFields(BugEvidenceUpdate{CausaRaiz: &submitted}, got)
		if diff.CausaRaiz == nil || *diff.CausaRaiz != submitted {
			t.Errorf("expected CausaRaiz to be flagged as divergent, got %+v", diff)
		}
		if diff.CausaRaizIdentificada != nil || diff.SolucionDefinitiva != nil || diff.TipoSolucion != nil {
			t.Errorf("expected only CausaRaiz to be flagged, got %+v", diff)
		}
	})

	t.Run("a mismatched TipoSolucion pair is flagged as one unit even if only one boolean differs", func(t *testing.T) {
		submitted := TipoSolucionTemporal
		// got reflects a response where the folded TipoSolucion differs from
		// what was submitted, even though (from the caller's perspective) only
		// one of the two underlying booleans actually diverged.
		got := &BugEvidence{TipoSolucion: TipoSolucionDefinitiva}
		diff := divergentFields(BugEvidenceUpdate{TipoSolucion: &submitted}, got)
		if diff.TipoSolucion == nil || *diff.TipoSolucion != submitted {
			t.Errorf("expected TipoSolucion to be flagged as a single divergent unit, got %+v", diff)
		}
	})

	t.Run("untouched (nil) fields are never reported as divergent", func(t *testing.T) {
		got := &BugEvidence{CausaRaiz: "whatever azure has", SolucionDefinitiva: "whatever else azure has"}
		diff := divergentFields(BugEvidenceUpdate{}, got)
		if !isEmptyBugEvidenceUpdate(diff) {
			t.Errorf("expected no fields flagged when nothing was submitted, got %+v", diff)
		}
	})
}

// bugEvidenceEchoJSON builds a $expand=all-shaped PATCH-echo response body
// for the given field values, matching the wire shape bugEvidenceFields
// expects.
func bugEvidenceEchoJSON(id, rev int, causaRaiz string, identificada bool, solucion string, temporal, definitiva bool) string {
	return `{
		"id": ` + itoa(id) + `,
		"rev": ` + itoa(rev) + `,
		"fields": {
			"System.State": "Activo",
			"System.TeamProject": "RUNTPRO",
			"System.Title": "Bug de ejemplo",
			"System.WorkItemType": "Bug",
			"Microsoft.VSTS.CMMI.ProposedFix": ` + jsonStr(causaRaiz) + `,
			"Custom.832c1387-0208-47b9-bd6d-500d3a7b8019": ` + boolJSON(identificada) + `,
			"Custom.818d41f3-03fe-4c91-9d6a-eeedb596ffb7": ` + jsonStr(solucion) + `,
			"Custom.Temporal": ` + boolJSON(temporal) + `,
			"Custom.Definitiva": ` + boolJSON(definitiva) + `
		}
	}`
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestPatchBugEvidence_ResponseEchoesAll_SinglePatchNoReaffirm(t *testing.T) {
	var patchCount int
	var lastOps []patchOp
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		patchCount++
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &lastOps); err != nil {
			t.Fatalf("unmarshal request ops: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(bugEvidenceEchoJSON(4242, 8, "root cause text", true, "fix text", false, true))) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	causaRaiz := "root cause text"
	identificada := true
	solucion := "fix text"
	tipo := TipoSolucionDefinitiva
	ev, reaffirmed, err := c.PatchBugEvidence(context.Background(), 4242, 7, BugEvidenceUpdate{
		CausaRaiz:             &causaRaiz,
		CausaRaizIdentificada: &identificada,
		SolucionDefinitiva:    &solucion,
		TipoSolucion:          &tipo,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reaffirmed {
		t.Error("expected reaffirmed=false when the response echoes all submitted values")
	}
	if patchCount != 1 {
		t.Fatalf("expected exactly 1 PATCH request, got %d", patchCount)
	}
	if ev.CausaRaiz != causaRaiz || !ev.CausaRaizIdentificada || ev.SolucionDefinitiva != solucion || ev.TipoSolucion != TipoSolucionDefinitiva {
		t.Errorf("expected parsed evidence to reflect the echoed response, got %+v", ev)
	}
	values := opValues(lastOps)
	if got := values["/rev"]; got != float64(7) {
		t.Errorf("expected op[0] test /rev=7, got %v", got)
	}
}

func TestPatchBugEvidence_ResponseDivergesOneField_SecondPatchOnlyThatField(t *testing.T) {
	var patches [][]patchOp
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		body, _ := io.ReadAll(r.Body)
		var ops []patchOp
		if err := json.Unmarshal(body, &ops); err != nil {
			t.Fatalf("unmarshal request ops: %v", err)
		}
		patches = append(patches, ops)
		w.WriteHeader(http.StatusOK)
		if call == 1 {
			// First PATCH echoes a DIFFERENT CausaRaiz than what was submitted
			// (a workflow side effect); every other field matches.
			w.Write([]byte(bugEvidenceEchoJSON(4242, 8, "STALE root cause", true, "fix text", false, true))) //nolint:errcheck
			return
		}
		// Second (reaffirm) PATCH echoes the corrected value.
		w.Write([]byte(bugEvidenceEchoJSON(4242, 9, "root cause text", true, "fix text", false, true))) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	causaRaiz := "root cause text"
	identificada := true
	solucion := "fix text"
	tipo := TipoSolucionDefinitiva
	ev, reaffirmed, err := c.PatchBugEvidence(context.Background(), 4242, 7, BugEvidenceUpdate{
		CausaRaiz:             &causaRaiz,
		CausaRaizIdentificada: &identificada,
		SolucionDefinitiva:    &solucion,
		TipoSolucion:          &tipo,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reaffirmed {
		t.Error("expected reaffirmed=true when a field diverged and needed a second PATCH")
	}
	if len(patches) != 2 {
		t.Fatalf("expected exactly 2 PATCH requests, got %d", len(patches))
	}
	secondValues := opValues(patches[1])
	if _, ok := secondValues["/rev"]; ok {
		t.Error("expected the second PATCH to NOT include the test/rev op (rev already advanced)")
	}
	if len(secondValues) != 1 {
		t.Fatalf("expected the second PATCH to contain exactly 1 op (only the divergent field), got %d: %+v", len(secondValues), secondValues)
	}
	if got := secondValues["/fields/"+FieldCausaRaiz]; got != causaRaiz {
		t.Errorf("expected the second PATCH to re-affirm only CausaRaiz=%q, got %v", causaRaiz, got)
	}
	if ev.CausaRaiz != causaRaiz {
		t.Errorf("expected the final evidence to reflect the reaffirmed value, got %q", ev.CausaRaiz)
	}
}

func TestPatchBugEvidence_StillDivergentAfterSecondPatch_HardError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Every PATCH (first and reaffirm) echoes a value different from what
		// was submitted — the field never actually takes.
		w.Write([]byte(bugEvidenceEchoJSON(4242, 8, "STILL WRONG root cause", false, "", false, false))) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	causaRaiz := "root cause text"
	ev, reaffirmed, err := c.PatchBugEvidence(context.Background(), 4242, 7, BugEvidenceUpdate{CausaRaiz: &causaRaiz})
	if err == nil {
		t.Fatal("expected a hard error when the field is still divergent after the reaffirm PATCH")
	}
	if ev != nil {
		t.Errorf("expected nil evidence on hard error (never report success), got %+v", ev)
	}
	if reaffirmed {
		t.Error("expected reaffirmed=false on hard error")
	}
}

func TestPatchBugEvidence_RevConflict_ReturnsErrRevConflictWithNoFollowupPatch(t *testing.T) {
	var patchCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		patchCount++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"The rev value 7 does not match the current value 9; test operation failed."}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	causaRaiz := "root cause text"
	_, _, err := c.PatchBugEvidence(context.Background(), 4242, 7, BugEvidenceUpdate{CausaRaiz: &causaRaiz})
	if !errors.Is(err, ErrRevConflict) {
		t.Fatalf("expected ErrRevConflict, got %v", err)
	}
	// Exactly one PATCH attempt reached the server: a rejected /rev test op
	// means Azure rejects the whole op list atomically (no partial field
	// writes), and this client must not retry or attempt any corrective
	// follow-up PATCH after a rev conflict.
	if patchCount != 1 {
		t.Fatalf("expected exactly 1 PATCH attempt (no follow-up write after a rev conflict), got %d", patchCount)
	}
}

func TestPatchBugEvidence_PatchForbidden_ReturnsErrInsufficientScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"not authorized"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Client{
		cfg:  Config{Token: "x", AuthMode: AuthModeBearer, Org: "ORG", TeamProject: "PROJ"},
		http: &http.Client{Transport: redirectToServer(srv.URL)},
	}

	causaRaiz := "root cause text"
	_, _, err := c.PatchBugEvidence(context.Background(), 4242, 7, BugEvidenceUpdate{CausaRaiz: &causaRaiz})
	if !errors.Is(err, ErrInsufficientScope) {
		t.Fatalf("expected ErrInsufficientScope, got %v", err)
	}
}
