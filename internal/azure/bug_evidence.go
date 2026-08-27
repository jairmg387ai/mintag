package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// divergentFields returns the subset of u's non-nil fields whose submitted
// value does not match the corresponding field in got (typically the
// just-echoed PatchBugEvidence response, parsed into a BugEvidence).
// TipoSolucion is compared as one unit: if u.TipoSolucion is set, the whole
// field is reported divergent whenever got's folded TipoSolucion differs,
// regardless of which one of the two underlying booleans actually caused
// the mismatch. Fields u never touched (nil) are never reported.
func divergentFields(u BugEvidenceUpdate, got *BugEvidence) BugEvidenceUpdate {
	var out BugEvidenceUpdate
	if u.CausaRaiz != nil && *u.CausaRaiz != got.CausaRaiz {
		out.CausaRaiz = u.CausaRaiz
	}
	if u.CausaRaizIdentificada != nil && *u.CausaRaizIdentificada != got.CausaRaizIdentificada {
		out.CausaRaizIdentificada = u.CausaRaizIdentificada
	}
	if u.SolucionDefinitiva != nil && *u.SolucionDefinitiva != got.SolucionDefinitiva {
		out.SolucionDefinitiva = u.SolucionDefinitiva
	}
	if u.TipoSolucion != nil && *u.TipoSolucion != got.TipoSolucion {
		out.TipoSolucion = u.TipoSolucion
	}
	return out
}

// bugEvidenceEnvelope is the wire shape of a work item response carrying the
// bug-evidence fields this package cares about — shared by both a GET
// $expand=all response (FetchBugEvidence) and a PATCH echo response
// (PatchBugEvidence), so the JSON struct/decode logic exists in exactly one
// place.
type bugEvidenceEnvelope struct {
	ID     int               `json:"id"`
	Rev    int               `json:"rev"`
	Fields bugEvidenceFields `json:"fields"`
}

// parseBugEvidenceResponse decodes a work item response body (GET or PATCH
// echo) into a BugEvidence, folding the two TipoSolucion booleans via
// tipoSolucionFromFlags.
func parseBugEvidenceResponse(body []byte) (*BugEvidence, error) {
	var parsed bugEvidenceEnvelope
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("azure: decode bug evidence response: %w", err)
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
		return nil, fmt.Errorf("azure: unexpected fetch bug evidence status %d%s: %w",
			resp.StatusCode, sanitizedResponseMessage(respBody), &FetchBugEvidenceStatusError{StatusCode: resp.StatusCode})
	}
	if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
		return nil, fmt.Errorf("azure: Azure returned HTML/sign-in response; token may be expired or auth mode invalid")
	}

	return parseBugEvidenceResponse(respBody)
}

// FetchBugEvidenceStatusError carries the raw HTTP status code from a failed
// FetchBugEvidence call so callers can classify the failure (e.g. "was this a
// 404") via errors.As, instead of substring-matching the error text — which
// otherwise echoes untrusted Azure response-body content via
// sanitizedResponseMessage and could coincidentally contain a misleading
// status-like substring.
type FetchBugEvidenceStatusError struct {
	StatusCode int
}

func (e *FetchBugEvidenceStatusError) Error() string {
	return fmt.Sprintf("azure: fetch bug evidence returned status %d", e.StatusCode)
}

// ErrRevConflict is returned by PatchBugEvidence when Azure rejects the
// leading "/rev" test op — the work item changed since the caller last read
// it (expectedRev is stale). No field write is applied when this happens;
// Azure's json-patch test op rejects the entire op list atomically.
//
// ASSUMPTION (flagged as a risk, not verified against a real Bug work item):
// Azure DevOps does not publicly document a stable, distinguishing error
// code for a rejected json-patch "test" operation. This is detected via
// isRevConflictResponse: an HTTP 400 whose body mentions both "rev" and a
// mismatch/test-failure marker. If empirical testing against a real Bug
// shows a different shape, isRevConflictResponse is the single place to
// correct.
var ErrRevConflict = errors.New("azure: bug evidence rev conflict — the work item changed since it was last read")

// ErrInsufficientScope is returned by PatchBugEvidence when the write is
// rejected as an auth/scope failure (401/403, or the HTML sign-in page
// pattern) on a PATCH — as opposed to a generic transport error. Because
// PatchBugEvidence is only ever called after a caller has already
// successfully read the same Bug (to obtain expectedRev), this rejection is
// necessarily write-specific, not a blanket credential failure.
var ErrInsufficientScope = errors.New("azure: azure token lacks scope to write bug evidence fields")

// isRevConflictResponse heuristically detects a rejected "/rev" test op in a
// failed PATCH response body. See ErrRevConflict's doc comment for the
// assumption this encodes.
func isRevConflictResponse(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest {
		return false
	}
	lower := strings.ToLower(string(body))
	if !strings.Contains(lower, "rev") {
		return false
	}
	return strings.Contains(lower, "does not match") ||
		strings.Contains(lower, "test operation") ||
		strings.Contains(lower, "testfailed")
}

// classifyBugEvidencePatchError maps a failed patchWorkItemWithResponse call
// to ErrRevConflict or ErrInsufficientScope when the failure's status/body
// matches one of those known shapes, or returns err unchanged (wrapped
// transport/marshal errors, or a generic non-2xx status) otherwise.
func classifyBugEvidencePatchError(err error) error {
	var statusErr *patchStatusError
	if !errors.As(err, &statusErr) {
		return err
	}
	if isRevConflictResponse(statusErr.StatusCode, statusErr.Body) {
		return ErrRevConflict
	}
	if statusErr.IsHTML || statusErr.StatusCode == http.StatusUnauthorized || statusErr.StatusCode == http.StatusForbidden {
		return ErrInsufficientScope
	}
	return err
}

// PatchBugEvidence writes the dirty fields in u to Bug id, guarded by an
// optimistic-concurrency test against expectedRev (see buildBugEvidenceOps).
// After the PATCH, it parses Azure's echoed fields and compares them against
// what was submitted (divergentFields): if everything matches, it returns
// immediately with reaffirmed=false. If some subset diverged — a documented
// Azure workflow side effect on some fields — it issues a second PATCH
// containing ONLY that divergent subset (no test/rev op: rev already
// advanced from the first PATCH, so there is no stable value left to test
// against) and returns reaffirmed=true on success. If the field is STILL
// divergent after that second PATCH, this is a hard error — a caller must
// never treat it as success, and no third PATCH is attempted.
func (c *Client) PatchBugEvidence(ctx context.Context, id, expectedRev int, u BugEvidenceUpdate) (*BugEvidence, bool, error) {
	if !c.Enabled() {
		return nil, false, fmt.Errorf("Azure TimeLog token is not configured")
	}

	ops, err := buildBugEvidenceOps(expectedRev, u)
	if err != nil {
		return nil, false, err
	}

	body, err := c.patchWorkItemWithResponse(ctx, c.cfg.TeamProject, id, ops)
	if err != nil {
		return nil, false, classifyBugEvidencePatchError(err)
	}

	got, err := parseBugEvidenceResponse(body)
	if err != nil {
		return nil, false, err
	}

	divergent := divergentFields(u, got)
	if isEmptyBugEvidenceUpdate(divergent) {
		return got, false, nil
	}

	reaffirmOps := bugEvidenceFieldOps(divergent)
	reaffirmBody, err := c.patchWorkItemWithResponse(ctx, c.cfg.TeamProject, id, reaffirmOps)
	if err != nil {
		return nil, false, classifyBugEvidencePatchError(err)
	}

	reaffirmed, err := parseBugEvidenceResponse(reaffirmBody)
	if err != nil {
		return nil, false, err
	}

	stillDivergent := divergentFields(divergent, reaffirmed)
	if !isEmptyBugEvidenceUpdate(stillDivergent) {
		return nil, false, fmt.Errorf("azure: bug evidence still divergent after reaffirm PATCH for work item %d", id)
	}

	return reaffirmed, true, nil
}
