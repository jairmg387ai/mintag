package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

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

// bugEvidenceFields is the wire shape of the fields this client needs from a
// Bug work item's $expand=all response. GUID-suffixed custom refnames break
// Azure's fields= allowlist approach (confirmed against a real Bug work
// item), so FetchBugEvidence always requests the full item instead and
// picks out only these fields here.
type bugEvidenceFields struct {
	State                 string `json:"System.State"`
	TeamProject           string `json:"System.TeamProject"`
	Title                 string `json:"System.Title"`
	Type                  string `json:"System.WorkItemType"`
	CausaRaiz             string `json:"Microsoft.VSTS.CMMI.ProposedFix"`
	CausaRaizIdentificada bool   `json:"Custom.832c1387-0208-47b9-bd6d-500d3a7b8019"`
	SolucionDefinitiva    string `json:"Custom.818d41f3-03fe-4c91-9d6a-eeedb596ffb7"`
	Temporal              bool   `json:"Custom.Temporal"`
	Definitiva            bool   `json:"Custom.Definitiva"`
}

// tipoSolucionFromFlags derives TipoSolucion from the two boolean custom
// fields. Definitiva takes priority if a fixture ever has both flags true
// simultaneously — that combination is a pre-existing data anomaly to
// tolerate (not validate against) in this read-path method.
func tipoSolucionFromFlags(temporal, definitiva bool) TipoSolucion {
	switch {
	case definitiva:
		return TipoSolucionDefinitiva
	case temporal:
		return TipoSolucionTemporal
	default:
		return TipoSolucionNone
	}
}

// tipoSolucionFlags is the write-side inverse of tipoSolucionFromFlags: it
// derives the two boolean custom fields to send to Azure for a given
// TipoSolucion. TipoSolucionNone (or any other/empty value) clears both.
func tipoSolucionFlags(t TipoSolucion) (temporal, definitiva bool) {
	switch t {
	case TipoSolucionTemporal:
		return true, false
	case TipoSolucionDefinitiva:
		return false, true
	default:
		return false, false
	}
}

// BugEvidenceUpdate is a per-field write intent for PatchBugEvidence. A nil
// pointer means the field is untouched — untouched fields are never sent to
// Azure. These 4 exposed fields are the only fields this type can ever
// express; there is no path to any other Azure field (in particular
// System.State) through it.
type BugEvidenceUpdate struct {
	CausaRaiz             *string
	CausaRaizIdentificada *bool
	SolucionDefinitiva    *string
	TipoSolucion          *TipoSolucion
}

// isEmptyBugEvidenceUpdate reports whether every field of u is untouched
// (nil).
func isEmptyBugEvidenceUpdate(u BugEvidenceUpdate) bool {
	return u.CausaRaiz == nil && u.CausaRaizIdentificada == nil &&
		u.SolucionDefinitiva == nil && u.TipoSolucion == nil
}

// bugEvidenceFieldOps builds one json-patch op per non-nil field of u. This
// is the single place where an Azure field path is ever emitted for a bug
// evidence write, and it only ever uses the 5 field constants declared
// above — there is no code path that can produce any other path (in
// particular, no System.State path exists here or anywhere else in this
// file), so a bug-state write through this mechanism is structurally
// unreachable, not merely a review convention.
//
// TipoSolucion is the one deliberate exception to "one field = one op": it
// always emits BOTH FieldTipoTemporal and FieldTipoDefinitiva together, as
// one atomic pair, so the illegal "both true" state can never be reached by
// a caller that only sets one of them.
func bugEvidenceFieldOps(u BugEvidenceUpdate) []patchOp {
	var ops []patchOp
	if u.CausaRaiz != nil {
		ops = append(ops, patchOp{Op: "add", Path: "/fields/" + FieldCausaRaiz, Value: *u.CausaRaiz})
	}
	if u.CausaRaizIdentificada != nil {
		ops = append(ops, patchOp{Op: "add", Path: "/fields/" + FieldCausaRaizIdentificada, Value: *u.CausaRaizIdentificada})
	}
	if u.SolucionDefinitiva != nil {
		ops = append(ops, patchOp{Op: "add", Path: "/fields/" + FieldSolucionDefinitiva, Value: *u.SolucionDefinitiva})
	}
	if u.TipoSolucion != nil {
		temporal, definitiva := tipoSolucionFlags(*u.TipoSolucion)
		ops = append(ops,
			patchOp{Op: "add", Path: "/fields/" + FieldTipoTemporal, Value: temporal},
			patchOp{Op: "add", Path: "/fields/" + FieldTipoDefinitiva, Value: definitiva},
		)
	}
	return ops
}

// buildBugEvidenceOps is a pure function building the full json-patch op
// list for a bug-evidence write: op[0] is always the "/rev" test op
// (optimistic concurrency against expectedRev), followed by one op per
// dirty field from u (see bugEvidenceFieldOps). Returns an error if u has no
// fields set — there is nothing meaningful to PATCH.
func buildBugEvidenceOps(expectedRev int, u BugEvidenceUpdate) ([]patchOp, error) {
	fieldOps := bugEvidenceFieldOps(u)
	if len(fieldOps) == 0 {
		return nil, fmt.Errorf("azure: bug evidence update has no fields set")
	}
	ops := make([]patchOp, 0, len(fieldOps)+1)
	ops = append(ops, patchOp{Op: "test", Path: "/rev", Value: expectedRev})
	ops = append(ops, fieldOps...)
	return ops, nil
}

// FetchBugEvidence resolves the DSW-PR-017 V2 root-cause/solution evidence
// fields for a single Bug work item by id, org-scoped (same id-uniqueness
// rationale as FetchWorkItemFull). Unlike FetchWorkItemFull, this always
// requests $expand=all instead of an explicit fields= allowlist, because the
// GUID-suffixed custom refnames it needs are not safe to allowlist.
func (c *Client) FetchBugEvidence(ctx context.Context, id int) (*BugEvidence, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Azure TimeLog token is not configured")
	}

	url := fmt.Sprintf(
		"https://dev.azure.com/%s/_apis/wit/workitems/%d?$expand=all&api-version=7.1-preview.3",
		c.cfg.Org, id,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: build fetch bug evidence request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: fetch bug evidence http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure: unexpected fetch bug evidence status %d%s", resp.StatusCode, sanitizedResponseMessage(respBody))
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return nil, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}

	var parsed struct {
		ID     int               `json:"id"`
		Rev    int               `json:"rev"`
		Fields bugEvidenceFields `json:"fields"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("azure: decode fetch bug evidence response: %w", err)
	}

	return &BugEvidence{
		ID:                    parsed.ID,
		Rev:                   parsed.Rev,
		State:                 parsed.Fields.State,
		TeamProject:           parsed.Fields.TeamProject,
		Title:                 parsed.Fields.Title,
		Type:                  parsed.Fields.Type,
		CausaRaiz:             parsed.Fields.CausaRaiz,
		CausaRaizIdentificada: parsed.Fields.CausaRaizIdentificada,
		SolucionDefinitiva:    parsed.Fields.SolucionDefinitiva,
		TipoSolucion:          tipoSolucionFromFlags(parsed.Fields.Temporal, parsed.Fields.Definitiva),
	}, nil
}
