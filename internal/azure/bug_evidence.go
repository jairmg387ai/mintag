package azure

// Field reference names for the DSW-PR-017 V2 bug-evidence fields. The two
// custom-GUID-suffixed refnames (FieldCausaRaizIdentificada,
// FieldSolucionDefinitiva) come from the shared Bug work item type
// definition and are not guessable from their display name — they must
// match exactly what Azure DevOps returns in $expand=all.
const (
	FieldCausaRaiz             = "Microsoft.VSTS.CMMI.ProposedFix"
	FieldCausaRaizIdentificada = "Custom.832c1387-0208-47b9-bd6d-500d3a7b8019"
	FieldSolucionDefinitiva    = "Custom.818d41f3-03fe-4c91-9d6a-eeedb596ffb7"
	FieldTipoTemporal          = "Custom.Temporal"
	FieldTipoDefinitiva        = "Custom.Definitiva"
)

// TipoSolucion is the kind of fix registered against a bug, derived from the
// two boolean custom fields (FieldTipoTemporal/FieldTipoDefinitiva).
type TipoSolucion string

const (
	TipoSolucionNone       TipoSolucion = ""
	TipoSolucionTemporal   TipoSolucion = "temporal"
	TipoSolucionDefinitiva TipoSolucion = "definitiva"
)

// BugEvidence is the read-only projection of a Bug work item's root-cause
// and solution evidence fields (DSW-PR-017 V2), as fetched by
// FetchBugEvidence.
type BugEvidence struct {
	ID          int
	Rev         int
	State       string
	TeamProject string
	Title       string
	Type        string

	CausaRaiz             string
	CausaRaizIdentificada bool
	SolucionDefinitiva    string
	TipoSolucion          TipoSolucion
}

// editableBugEvidenceStates is the exact DSW-PR-017 V2 allowlist of states in
// which bug-evidence fields may still be edited. Anything not in this set —
// including the read-only terminal/registration states and any unrecognized
// state string — is treated as read-only (fail closed).
var editableBugEvidenceStates = map[string]bool{
	"En Revisión":       true,
	"En Requisitos":     true,
	"Resuelto":          true,
	"Activo":            true,
	"En Pruebas":        true,
	"Solucionado":       true,
	"Pruebas INT":       true,
	"Pendiente Ventana": true,
	"Corregido":         true,
	"Devuelto":          true,
}

// IsBugEvidenceEditableState reports whether bug-evidence fields may be
// edited while a bug is in the given state. Exact case/accent match against
// the DSW-PR-017 V2 state list; any state not explicitly listed as editable
// (including read-only states like Registrado/Descartado/Cerrado, and any
// unrecognized state string) returns false.
func IsBugEvidenceEditableState(state string) bool {
	return editableBugEvidenceStates[state]
}
