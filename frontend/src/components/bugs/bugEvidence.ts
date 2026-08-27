import type { BugEvidenceFields, BugEvidenceUpdate } from '../../types'

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

// resolveConflict (per-field keep-mine/take-Azure resolution for
// BugConflictModal) is deferred to PR5 — see this PR's apply-progress
// judgment-call notes: BugConflictModal doesn't exist yet in this PR, and
// committing to an exact resolution-input/output shape before the modal
// exists to consume it risks designing against an untested UX contract.
