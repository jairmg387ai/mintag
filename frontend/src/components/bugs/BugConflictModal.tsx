import { useState } from 'react'
import type { BugEvidence, BugEvidenceFields } from '../../types'
import { SafeHtml } from '../shared/SafeHtml'
import { Button } from '../ui/Button'
import {
  TIPO_SOLUCION_LABELS,
  divergentBugEvidenceFields,
  resolveConflict,
  type BugEvidenceFieldKey,
  type ConflictResolutionChoice,
  type ConflictResolutionChoices,
} from './bugEvidence'

const FIELD_LABELS: Record<BugEvidenceFieldKey, string> = {
  causa_raiz: 'Causa raíz',
  causa_raiz_identificada: 'Causa raíz identificada',
  solucion_definitiva: 'Solución definitiva',
  tipo_solucion: 'Tipo de solución',
}

// HTML_FIELDS are rendered as two sanitized, read-only SafeHtml panes side
// by side — no live diffing, no rich-text editing inside the modal. The
// user picks a whole side per field; they never hand-merge HTML content.
const HTML_FIELDS = new Set<BugEvidenceFieldKey>(['causa_raiz', 'solucion_definitiva'])

function displayValue(key: BugEvidenceFieldKey, fields: BugEvidenceFields): string {
  if (key === 'causa_raiz_identificada') return fields.causa_raiz_identificada ? 'Sí' : 'No'
  if (key === 'tipo_solucion') return TIPO_SOLUCION_LABELS[fields.tipo_solucion]
  return String(fields[key])
}

interface BugConflictModalProps {
  draft: BugEvidenceFields
  remote: BugEvidence
  onResolve: (resolved: BugEvidenceFields) => void
  onCancel: () => void
}

// BugConflictModal is shown when BugEvidencePanel's save hits a rev_conflict:
// another change landed on this Bug in Azure DevOps since the panel opened.
// Only fields that actually diverged between the user's draft and the
// freshly re-fetched remote evidence are shown — never all four fields
// unconditionally. tipo_solucion is resolved as ONE atomic choice (never two
// independently-resolvable booleans), per the design's explicit invariant.
// No audit trail of which side was picked is kept (spec requirement).
export function BugConflictModal({ draft, remote, onResolve, onCancel }: BugConflictModalProps) {
  const divergent = divergentBugEvidenceFields(draft, remote.fields)
  const [choices, setChoices] = useState<ConflictResolutionChoices>({})

  function setChoice(key: BugEvidenceFieldKey, choice: ConflictResolutionChoice) {
    setChoices(prev => ({ ...prev, [key]: choice }))
  }

  function handleResolve() {
    onResolve(resolveConflict(draft, remote.fields, choices))
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <p style={{ font: 'var(--text-body)', color: 'var(--fg2)', margin: 0 }}>
        Alguien más modificó este bug en Azure DevOps mientras lo editabas. Elige, para cada campo
        que cambió, si quieres mantener tu versión o usar la de Azure.
      </p>

      {divergent.map(key => {
        const choice = choices[key] ?? 'mine'
        return (
          <div key={key} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <div className="label">{FIELD_LABELS[key]}</div>

            <div
              role="radiogroup"
              aria-label={`Resolver ${FIELD_LABELS[key]}`}
              style={{ display: 'flex', gap: 16 }}
            >
              <label style={{ display: 'flex', alignItems: 'center', gap: 6, font: 'var(--text-body)', color: 'var(--fg1)' }}>
                <input
                  type="radio"
                  name={`resolve-${key}`}
                  checked={choice === 'mine'}
                  onChange={() => setChoice(key, 'mine')}
                />
                Mantener mío
              </label>
              <label style={{ display: 'flex', alignItems: 'center', gap: 6, font: 'var(--text-body)', color: 'var(--fg1)' }}>
                <input
                  type="radio"
                  name={`resolve-${key}`}
                  checked={choice === 'azure'}
                  onChange={() => setChoice(key, 'azure')}
                />
                Usar de Azure
              </label>
            </div>

            <div style={{ display: 'flex', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', marginBottom: 4 }}>Mío</div>
                {HTML_FIELDS.has(key) ? (
                  <SafeHtml html={draft[key] as string} />
                ) : (
                  <div style={{ color: 'var(--fg2)' }}>{displayValue(key, draft)}</div>
                )}
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', marginBottom: 4 }}>Azure</div>
                {HTML_FIELDS.has(key) ? (
                  <SafeHtml html={remote.fields[key] as string} />
                ) : (
                  <div style={{ color: 'var(--fg2)' }}>{displayValue(key, remote.fields)}</div>
                )}
              </div>
            </div>
          </div>
        )
      })}

      <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
        <Button variant="ghost" onClick={onCancel}>Cancelar</Button>
        <Button variant="primary" onClick={handleResolve}>Resolver</Button>
      </div>
    </div>
  )
}
