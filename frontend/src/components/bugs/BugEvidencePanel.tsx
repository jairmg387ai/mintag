import { useEffect, useRef, useState } from 'react'
import { fetchBugEvidence, patchBugEvidence, BugEvidenceApiError } from '../../api/client'
import type { BugEvidence, BugEvidenceFields, TipoSolucion } from '../../types'
import { SafeHtml } from '../shared/SafeHtml'
import { Button } from '../ui/Button'
import { TIPO_SOLUCION_LABELS, buildBugEvidenceUpdate, canSetCausaRaizIdentificada } from './bugEvidence'
import { BugConflictModal } from './BugConflictModal'
import { RichTextField, type RichTextFieldHandle } from './RichTextField'
import { TipoSolucionRadio } from './TipoSolucionRadio'

interface BugEvidencePanelProps {
  bugId: number
}

// BugEvidencePanel is the DSW-PR-017 V2 evidence container for a single
// Azure Bug work item. It trusts the GET response's server-computed
// `editable` boolean directly (rather than re-deriving it from
// EDITABLE_STATES client-side) to decide read-only vs editable rendering —
// the server already re-checks state on every write, so this is simply the
// UX reflection of that same boundary. Not wrapped in this project's Modal
// component here: PR5's WorkItemsView (C14) is the entry point that opens
// this panel inside a Modal.
export function BugEvidencePanel({ bugId }: BugEvidencePanelProps) {
  const [evidence, setEvidence] = useState<BugEvidence | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [saveError, setSaveError] = useState('')
  const [saving, setSaving] = useState(false)
  // Non-null exactly while a rev_conflict is being resolved. Holds the
  // draft that failed to save plus the freshly re-fetched remote evidence
  // (new rev) — see handleSave's rev_conflict branch and
  // handleResolveConflict below.
  const [conflict, setConflict] = useState<{ draft: BugEvidenceFields; remote: BugEvidence } | null>(null)

  // Controlled draft state for the two non-rich-text controls. The two
  // rich-text fields (causa_raiz, solucion_definitiva) are intentionally
  // uncontrolled — see RichTextField's doc comment — and are only read
  // through refs at save time.
  const [causaRaizIdentificada, setCausaRaizIdentificada] = useState(false)
  const [tipoSolucion, setTipoSolucion] = useState<TipoSolucion>('')
  // Live emptiness signal for causa_raiz, fed by RichTextField's onTextChange
  // (reads through the ref, never writes back to the DOM — does not turn
  // the field controlled). Used only to gate the causa_raiz_identificada
  // checkbox per the root-cause-identified invariant.
  const [causaRaizText, setCausaRaizText] = useState('')

  const causaRaizRef = useRef<RichTextFieldHandle>(null)
  const solucionDefinitivaRef = useRef<RichTextFieldHandle>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setLoadError('')
    fetchBugEvidence(bugId)
      .then(ev => {
        if (cancelled) return
        setEvidence(ev)
        setCausaRaizIdentificada(ev.fields.causa_raiz_identificada)
        setTipoSolucion(ev.fields.tipo_solucion)
        setCausaRaizText(ev.fields.causa_raiz)
      })
      .catch((e: unknown) => {
        if (cancelled) return
        setLoadError(e instanceof Error ? e.message : 'No se pudo cargar la evidencia del bug.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [bugId])

  // classifySaveError maps a rejected patchBugEvidence call to the message
  // shown in saveError. rev_conflict is deliberately NOT handled here — its
  // caller opens BugConflictModal instead of setting a static message.
  function classifySaveError(e: unknown): string {
    if (e instanceof BugEvidenceApiError && e.code === 'root_cause_required') {
      return 'No se puede marcar "Causa raíz identificada" sin registrar la causa raíz.'
    }
    if (e instanceof BugEvidenceApiError && e.code === 'insufficient_scope') {
      return 'Las credenciales configuradas no tienen permiso para escribir en este bug.'
    }
    if (e instanceof BugEvidenceApiError && e.code === 'state_not_editable') {
      return 'El bug cambió de estado y ya no admite edición.'
    }
    return e instanceof Error ? e.message : 'No se pudo guardar la evidencia.'
  }

  async function handleSave() {
    if (!evidence) return
    const draft: BugEvidenceFields = {
      causa_raiz: causaRaizRef.current?.getValue() ?? evidence.fields.causa_raiz,
      causa_raiz_identificada: causaRaizIdentificada,
      solucion_definitiva: solucionDefinitivaRef.current?.getValue() ?? evidence.fields.solucion_definitiva,
      tipo_solucion: tipoSolucion,
    }

    if (draft.causa_raiz_identificada && !canSetCausaRaizIdentificada(draft.causa_raiz)) {
      // Defensive: the checkbox is disabled while causa_raiz is empty, but
      // this still guards the edge case where causa_raiz_identificada was
      // already true from the loaded evidence and the user then clears
      // causa_raiz's content without unchecking it.
      setSaveError('No se puede marcar "Causa raíz identificada" sin registrar la causa raíz.')
      return
    }

    const update = buildBugEvidenceUpdate(evidence.fields, draft)
    if (Object.keys(update).length === 0) return

    setSaving(true)
    setSaveError('')
    try {
      const result = await patchBugEvidence(evidence.id, evidence.rev, update)
      setEvidence({ ...evidence, rev: result.rev, fields: result.fields })
      setCausaRaizIdentificada(result.fields.causa_raiz_identificada)
      setTipoSolucion(result.fields.tipo_solucion)
      setCausaRaizText(result.fields.causa_raiz)
    } catch (e: unknown) {
      if (e instanceof BugEvidenceApiError && e.code === 'rev_conflict') {
        // Re-GET the Bug (builds the fresh remote snapshot the modal diffs
        // against) and open BugConflictModal instead of silently overwriting
        // or blocking the user with a plain error.
        try {
          const remote = await fetchBugEvidence(evidence.id)
          setConflict({ draft, remote })
        } catch (fetchErr: unknown) {
          setSaveError(fetchErr instanceof Error ? fetchErr.message : 'No se pudo cargar la evidencia actual de Azure DevOps.')
        }
      } else {
        setSaveError(classifySaveError(e))
      }
    } finally {
      setSaving(false)
    }
  }

  // handleResolveConflict retries the save with the conflict's remote rev
  // (the caller-side retry the design calls for). If it hits another
  // rev_conflict, it is reported as a plain error rather than reopening the
  // modal a second time — a second consecutive conflict is out of this PR's
  // scope.
  async function handleResolveConflict(resolved: BugEvidenceFields) {
    if (!conflict) return
    const { remote } = conflict
    setConflict(null)

    const update = buildBugEvidenceUpdate(remote.fields, resolved)
    if (Object.keys(update).length === 0) {
      setEvidence(remote)
      setCausaRaizIdentificada(remote.fields.causa_raiz_identificada)
      setTipoSolucion(remote.fields.tipo_solucion)
      setCausaRaizText(remote.fields.causa_raiz)
      return
    }

    setSaving(true)
    setSaveError('')
    try {
      const result = await patchBugEvidence(remote.id, remote.rev, update)
      const nextEvidence = { ...remote, rev: result.rev, fields: result.fields }
      setEvidence(nextEvidence)
      setCausaRaizIdentificada(result.fields.causa_raiz_identificada)
      setTipoSolucion(result.fields.tipo_solucion)
      setCausaRaizText(result.fields.causa_raiz)
    } catch (e: unknown) {
      setSaveError(classifySaveError(e))
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div style={{ padding: 12, color: 'var(--fg3)' }}>Cargando evidencia del bug...</div>
  }

  if (loadError || !evidence) {
    return <div style={{ padding: 12, color: 'var(--block-solid)' }}>{loadError || 'Bug no encontrado.'}</div>
  }

  const fieldKeyBase = `${evidence.id}:${evidence.rev}`

  if (!evidence.editable) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div>
          <div className="label" style={{ marginBottom: 6 }}>Causa raíz</div>
          <SafeHtml html={evidence.fields.causa_raiz} />
        </div>
        <div>
          <div className="label" style={{ marginBottom: 6 }}>Causa raíz identificada</div>
          <div style={{ color: 'var(--fg2)' }}>{evidence.fields.causa_raiz_identificada ? 'Sí' : 'No'}</div>
        </div>
        <div>
          <div className="label" style={{ marginBottom: 6 }}>Solución definitiva</div>
          <SafeHtml html={evidence.fields.solucion_definitiva} />
        </div>
        <div>
          <div className="label" style={{ marginBottom: 6 }}>Tipo de solución</div>
          <div style={{ color: 'var(--fg2)' }}>{TIPO_SOLUCION_LABELS[evidence.fields.tipo_solucion]}</div>
        </div>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div>
        <div className="label" style={{ marginBottom: 6 }}>Causa raíz</div>
        <RichTextField
          key={`${fieldKeyBase}:causa_raiz`}
          ref={causaRaizRef}
          initialHtml={evidence.fields.causa_raiz}
          ariaLabel="Causa raíz"
          onTextChange={setCausaRaizText}
        />
      </div>

      <label style={{ display: 'flex', alignItems: 'center', gap: 8, font: 'var(--text-body)', color: 'var(--fg1)' }}>
        <input
          type="checkbox"
          checked={causaRaizIdentificada}
          onChange={e => setCausaRaizIdentificada(e.target.checked)}
          disabled={!canSetCausaRaizIdentificada(causaRaizText)}
        />
        Causa raíz identificada
      </label>

      <div>
        <div className="label" style={{ marginBottom: 6 }}>Solución definitiva</div>
        <RichTextField
          key={`${fieldKeyBase}:solucion_definitiva`}
          ref={solucionDefinitivaRef}
          initialHtml={evidence.fields.solucion_definitiva}
          ariaLabel="Solución definitiva"
        />
      </div>

      <div>
        <div className="label" style={{ marginBottom: 6 }}>Tipo de solución</div>
        <TipoSolucionRadio value={tipoSolucion} onChange={setTipoSolucion} />
      </div>

      {saveError && <div style={{ color: 'var(--block-solid)', font: 'var(--text-sm)' }}>{saveError}</div>}

      <div>
        <Button variant="primary" onClick={handleSave} disabled={saving}>
          {saving ? 'Guardando...' : 'Guardar'}
        </Button>
      </div>

      {conflict && (
        <BugConflictModal
          draft={conflict.draft}
          remote={conflict.remote}
          onResolve={handleResolveConflict}
          onCancel={() => setConflict(null)}
        />
      )}
    </div>
  )
}
