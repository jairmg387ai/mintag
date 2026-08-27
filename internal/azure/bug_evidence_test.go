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
