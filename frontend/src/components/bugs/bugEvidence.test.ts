import { describe, expect, it } from 'vitest'
import type { BugEvidenceFields } from '../../types'
import {
  EDITABLE_STATES,
  isBugEvidenceEditableState,
  buildBugEvidenceUpdate,
  canSetCausaRaizIdentificada,
} from './bugEvidence'

function buildFields(overrides: Partial<BugEvidenceFields> = {}): BugEvidenceFields {
  return {
    causa_raiz: '',
    causa_raiz_identificada: false,
    solucion_definitiva: '',
    tipo_solucion: '',
    ...overrides,
  }
}

describe('EDITABLE_STATES', () => {
  it('mirrors the exact DSW-PR-017 V2 10-state editable allowlist (internal/azure/bug_evidence.go)', () => {
    expect([...EDITABLE_STATES].sort()).toEqual(
      [
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
      ].sort(),
    )
  })
})

describe('isBugEvidenceEditableState', () => {
  it('returns true for every editable state', () => {
    for (const state of EDITABLE_STATES) {
      expect(isBugEvidenceEditableState(state)).toBe(true)
    }
  })

  it('fails closed (returns false) for the three read-only terminal states', () => {
    expect(isBugEvidenceEditableState('Registrado')).toBe(false)
    expect(isBugEvidenceEditableState('Descartado')).toBe(false)
    expect(isBugEvidenceEditableState('Cerrado')).toBe(false)
  })

  it('fails closed for an unrecognized state string', () => {
    expect(isBugEvidenceEditableState('Some Unknown State')).toBe(false)
  })
})

describe('buildBugEvidenceUpdate', () => {
  it('returns an empty update when nothing changed', () => {
    const original = buildFields({ causa_raiz: 'root cause' })
    const draft = buildFields({ causa_raiz: 'root cause' })

    expect(buildBugEvidenceUpdate(original, draft)).toEqual({})
  })

  it('includes only the fields that actually changed (dirty-only)', () => {
    const original = buildFields({ causa_raiz: 'old', solucion_definitiva: 'old fix' })
    const draft = buildFields({ causa_raiz: 'new', solucion_definitiva: 'old fix' })

    expect(buildBugEvidenceUpdate(original, draft)).toEqual({ causa_raiz: 'new' })
  })

  it('includes all four fields when all changed', () => {
    const original = buildFields()
    const draft = buildFields({
      causa_raiz: 'root',
      causa_raiz_identificada: true,
      solucion_definitiva: 'fix',
      tipo_solucion: 'definitiva',
    })

    expect(buildBugEvidenceUpdate(original, draft)).toEqual({
      causa_raiz: 'root',
      causa_raiz_identificada: true,
      solucion_definitiva: 'fix',
      tipo_solucion: 'definitiva',
    })
  })

  it('treats tipo_solucion as ONE control value, not two independent booleans', () => {
    const original = buildFields({ tipo_solucion: 'temporal' })
    const draft = buildFields({ tipo_solucion: 'definitiva' })

    const update = buildBugEvidenceUpdate(original, draft)

    expect(update).toEqual({ tipo_solucion: 'definitiva' })
    expect(update).not.toHaveProperty('causa_raiz')
  })

  it('does not mark tipo_solucion dirty when unchanged, even at the non-empty default', () => {
    const original = buildFields({ tipo_solucion: 'temporal' })
    const draft = buildFields({ tipo_solucion: 'temporal', causa_raiz: 'root' })

    const update = buildBugEvidenceUpdate(original, draft)

    expect(update).toEqual({ causa_raiz: 'root' })
  })
})

describe('canSetCausaRaizIdentificada', () => {
  it('returns false when causa raiz is empty', () => {
    expect(canSetCausaRaizIdentificada('')).toBe(false)
  })

  it('returns false when causa raiz is whitespace-only', () => {
    expect(canSetCausaRaizIdentificada('   \n  ')).toBe(false)
  })

  it('returns true once causa raiz has non-empty content', () => {
    expect(canSetCausaRaizIdentificada('Root cause found in logs')).toBe(true)
  })
})
