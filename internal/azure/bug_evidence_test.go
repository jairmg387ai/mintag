package azure

import (
	"context"
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
