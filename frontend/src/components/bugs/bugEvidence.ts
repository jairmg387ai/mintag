import type { BugEvidenceFields, BugEvidenceUpdate, TipoSolucion } from '../../types'

// EDITABLE_STATES mirrors editableBugEvidenceStates in
// internal/azure/bug_evidence.go exactly (same 10-state DSW-PR-017 V2
// allowlist). This mirror exists for UX purposes only — precedent for
// mirroring a Go state list client-side is WorkItemsView.tsx's
// isClosedAzureState. UNLIKE that helper, BugEvidencePanel's actual
// editable/read-only gate trusts the GET response's server-computed
// `editable` boolean directly rather than re-deriving it from this list:
// the server is the real boundary (re-checked on every write in
// handlePatchBugEvidence), and re-deriving here would mean keeping an
// accent/case-sensitive Spanish string list in sync in two languages for a
// value the server already tells us. This export exists so the mirror is
// itself testable/reviewable independent of that panel-level judgment call.
export const EDITABLE_STATES: readonly string[] = [
  'En Revisión',
  'En Requisitos',
  'Resuelto',
  'Activo',
  'En Pruebas',
  'Solucionado',
  'Pruebas INT',
  'Pendiente Ventana',
  'Corregido',
  'Devuelto',
]

const editableStateSet = new Set(EDITABLE_STATES)

// isBugEvidenceEditableState mirrors azure.IsBugEvidenceEditableState (Go):
// exact match against EDITABLE_STATES, fails closed (false) for anything not
// explicitly listed, including the three read-only terminal states and any
// unrecognized state string.
export function isBugEvidenceEditableState(state: string): boolean {
  return editableStateSet.has(state)
}

// buildBugEvidenceUpdate computes the minimal BugEvidenceUpdate (dirty-only)
// between an original baseline and a draft. tipo_solucion is compared as ONE
// control value (never as two independent booleans) — dirtiness is decided
// at the control level, matching the design's explicit exception to
// "PATCH only dirty fields".
export function buildBugEvidenceUpdate(
  original: BugEvidenceFields,
  draft: BugEvidenceFields,
): BugEvidenceUpdate {
  const update: BugEvidenceUpdate = {}
  if (draft.causa_raiz !== original.causa_raiz) {
    update.causa_raiz = draft.causa_raiz
  }
  if (draft.causa_raiz_identificada !== original.causa_raiz_identificada) {
    update.causa_raiz_identificada = draft.causa_raiz_identificada
  }
  if (draft.solucion_definitiva !== original.solucion_definitiva) {
    update.solucion_definitiva = draft.solucion_definitiva
  }
  if (draft.tipo_solucion !== original.tipo_solucion) {
    update.tipo_solucion = draft.tipo_solucion
  }
  return update
}

// canSetCausaRaizIdentificada enforces the Root-Cause-Identified Invariant
// client-side: "Causa raíz identificada" may only be set to true once
// "Causa raíz" has non-empty, non-whitespace-only content. The server
// re-checks the same invariant (root_cause_required, 422) — this is UX only,
// not the trust boundary.
export function canSetCausaRaizIdentificada(causaRaiz: string): boolean {
  return causaRaiz.trim().length > 0
}

// TIPO_SOLUCION_LABELS is shared by BugEvidencePanel's read-only view and
// BugConflictModal's diff panes, so the three-state radio's display strings
// live in exactly one place.
export const TIPO_SOLUCION_LABELS: Record<TipoSolucion, string> = {
  '': 'Sin definir',
  temporal: 'Temporal',
  definitiva: 'Definitiva',
}

// One of the four BugEvidenceFields keys, used to key per-field conflict
// resolution. tipo_solucion is always ONE key here — never split into its
// two underlying Azure booleans — mirroring buildBugEvidenceUpdate's
// control-level dirtiness above.
export type BugEvidenceFieldKey = keyof BugEvidenceFields

const BUG_EVIDENCE_FIELD_KEYS: readonly BugEvidenceFieldKey[] = [
  'causa_raiz',
  'causa_raiz_identificada',
  'solucion_definitiva',
  'tipo_solucion',
]

// divergentBugEvidenceFields returns ONLY the field keys whose value differs
// between draft (the user's in-progress edit) and remote (the freshly
// re-fetched Azure evidence after a rev_conflict) — BugConflictModal renders
// exactly these, never all four unconditionally.
export function divergentBugEvidenceFields(
  draft: BugEvidenceFields,
  remote: BugEvidenceFields,
): BugEvidenceFieldKey[] {
  return BUG_EVIDENCE_FIELD_KEYS.filter(key => draft[key] !== remote[key])
}

// 'mine' keeps the user's draft value for that field; 'azure' takes remote's
// value. No audit trail of the choice is kept anywhere (explicit spec
// requirement) — this is a pure, one-shot merge, not a stored decision.
export type ConflictResolutionChoice = 'mine' | 'azure'
export type ConflictResolutionChoices = Partial<Record<BugEvidenceFieldKey, ConflictResolutionChoice>>

// resolveConflict merges draft and remote per the caller's per-field
// choices. An unset/omitted choice defaults to 'mine' — the user's own draft
// wins unless they explicitly opt into Azure's value for that field.
// tipo_solucion is resolved as ONE atomic value: a 'mine'/'azure' choice on
// it picks the whole three-state control, never a mixed temporal/definitiva
// result assembled from two independent booleans.
export function resolveConflict(
  draft: BugEvidenceFields,
  remote: BugEvidenceFields,
  choices: ConflictResolutionChoices,
): BugEvidenceFields {
  const resolved = { ...draft }
  for (const key of BUG_EVIDENCE_FIELD_KEYS) {
    if (choices[key] === 'azure') {
      // Each field is assigned independently, but tipo_solucion is a single
      // string-valued key ('' | 'temporal' | 'definitiva') in
      // BugEvidenceFields already, so assigning it here can never produce a
      // state that mixes mine/azure within the pair of underlying booleans.
      ;(resolved as Record<BugEvidenceFieldKey, unknown>)[key] = remote[key]
    }
  }
  return resolved
}
