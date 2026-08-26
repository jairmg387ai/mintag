package azure

import "testing"

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
