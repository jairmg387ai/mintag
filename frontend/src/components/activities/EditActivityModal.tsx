import { useState, useEffect, useRef, type CSSProperties } from 'react'
import { X } from 'lucide-react'
import type { DailyActivity, ActivityCatalog, AzureActivity } from '../../types'
import { updateActivity } from '../../api/client'
import { AzureActivityCombobox } from './AzureActivityCombobox'
import { resolveAutofill } from './activityAutofill'

interface EditActivityModalProps {
  activity: DailyActivity | null
  open: boolean
  onClose: () => void
  onSaved: () => void
  catalog: ActivityCatalog | null
  azureActivities: AzureActivity[]
}

const inputStyle: CSSProperties = {
  width: '100%',
  padding: '8px 12px',
  border: '1px solid var(--border-strong)',
  borderRadius: 'var(--radius-md)',
  font: 'var(--text-body)',
  color: 'var(--fg1)',
  background: 'var(--bg-sunken)',
  outline: 'none',
  boxSizing: 'border-box',
}

const textareaStyle: CSSProperties = {
  ...inputStyle,
  resize: 'vertical',
  minHeight: 80,
  fontFamily: 'inherit',
}

function Field({ label, children, error }: { label: string; children: React.ReactNode; error?: string }) {
  return (
    <div style={{ marginBottom: 14 }}>
      <label className="label" style={{ marginBottom: 6, display: 'block' }}>
        {label}
      </label>
      {children}
      {error && (
        <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)', marginTop: 4 }}>
          {error}
        </div>
      )}
    </div>
  )
}

export function EditActivityModal({ activity, open, onClose, onSaved, catalog, azureActivities }: EditActivityModalProps) {
  const pressedOnOverlay = useRef(false)
  const [hours, setHours] = useState('')
  const [project, setProject] = useState('')
  const [category, setCategory] = useState('')
  const [registroDiario, setRegistroDiario] = useState('')
  const [azureActivityId, setAzureActivityId] = useState('')
  // Snapshot of azureActivityId as loaded, so handleSubmit can tell "the
  // user didn't touch this field" (send nothing, don't re-validate an
  // unrelated FK) apart from "the user picked something different"
  // (send the new value, including an explicit null to clear it).
  const [initialAzureActivityId, setInitialAzureActivityId] = useState('')
  const [referenceId, setReferenceId] = useState('')
  const [initialReferenceId, setInitialReferenceId] = useState('')
  // Seeded project/category start untouched (same as a fresh form's seeded
  // defaults) so selecting a different work item can still overwrite them;
  // they become touched only once the user hand-edits them in this session.
  const [projectTouched, setProjectTouched] = useState(false)
  const [categoryTouched, setCategoryTouched] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  useEffect(() => {
    if (open && activity) {
      setHours(String(activity.hours))
      setProject(activity.project)
      setCategory(activity.category)
      setRegistroDiario(activity.registro_diario)
      const loadedAzureActivityId = activity.azure_activity_id != null ? String(activity.azure_activity_id) : ''
      setAzureActivityId(loadedAzureActivityId)
      setInitialAzureActivityId(loadedAzureActivityId)
      const loadedReferenceId = activity.reference_id ?? ''
      setReferenceId(loadedReferenceId)
      setInitialReferenceId(loadedReferenceId)
      setErrors({})
      setSubmitError('')
      setProjectTouched(false)
      setCategoryTouched(false)
    }
  }, [open, activity])

  if (!open || !activity) return null

  // Fires only on an explicit, committed Azure work-item selection — never
  // on mount/open. Fills only the project/category the selected work item
  // actually has mapped, and only while that field hasn't been hand-edited.
  function handleAzureActivityChange(activityId: string) {
    setAzureActivityId(activityId)
    const patch = resolveAutofill({
      activity: azureActivities.find(a => String(a.id) === activityId),
      catalog,
      projectTouched,
      categoryTouched,
    })
    if (patch.project !== undefined) setProject(patch.project)
    if (patch.category !== undefined) setCategory(patch.category)
  }

  function validate(): boolean {
    const errs: Record<string, string> = {}
    const h = parseFloat(hours)
    if (!hours || isNaN(h) || h <= 0) errs.hours = 'Las horas deben ser mayores que 0'
    if (!project.trim()) errs.project = 'El proyecto es obligatorio'
    if (!category.trim()) errs.category = 'La categoría es obligatoria'
    if (!registroDiario.trim()) errs.registroDiario = 'La descripción es obligatoria'
    setErrors(errs)
    return Object.keys(errs).length === 0
  }

  async function handleSubmit() {
    if (!validate() || !activity) return
    setSubmitting(true)
    setSubmitError('')
    try {
      await updateActivity(activity.id, {
        hours: parseFloat(hours),
        project: project.trim(),
        category: category.trim(),
        registro_diario: registroDiario.trim(),
        // Omit the key entirely when the selection didn't change, so a save
        // that only edits e.g. hours never re-touches (and re-validates) an
        // unrelated, possibly since-deactivated Azure activity assignment.
        ...(azureActivityId !== initialAzureActivityId
          ? { azure_activity_id: azureActivityId ? Number(azureActivityId) : null }
          : {}),
        ...(referenceId !== initialReferenceId
          ? { reference_id: referenceId.trim() ? referenceId.trim() : null }
          : {}),
      })
      onSaved()
      onClose()
    } catch (e: unknown) {
      setSubmitError(e instanceof Error ? e.message : 'No se pudo actualizar la actividad')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      onMouseDown={e => { pressedOnOverlay.current = e.target === e.currentTarget }}
      onClick={e => {
        if (pressedOnOverlay.current && e.target === e.currentTarget) onClose()
        pressedOnOverlay.current = false
      }}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(15,23,42,0.5)',
        zIndex: 100,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 20,
      }}
    >
      <div
        onClick={e => e.stopPropagation()}
        className="card"
        style={{
          borderRadius: 'var(--radius-xl)',
          boxShadow: 'var(--shadow-xl)',
          width: '100%',
          maxWidth: 580,
          maxHeight: '90vh',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: '16px 20px',
            borderBottom: '1px solid var(--border)',
            display: 'flex',
            alignItems: 'center',
            gap: 10,
          }}
        >
          <h2 style={{ font: 'var(--text-h3)', color: 'var(--fg1)', flex: 1, margin: 0 }}>
            Editar actividad
          </h2>
          <button
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--fg3)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              padding: 4,
              borderRadius: 'var(--radius-md)',
            }}
            aria-label="Cerrar"
          >
            <X size={18} strokeWidth={1.75} />
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: '20px 22px', overflowY: 'auto', flex: 1 }}>
          <Field label="Horas *" error={errors.hours}>
            <input
              type="number"
              step="0.25"
              min="0.25"
              value={hours}
              onChange={e => setHours(e.target.value)}
              placeholder="0.00"
              style={inputStyle}
            />
          </Field>

          <Field label="Actividad de Azure">
            <AzureActivityCombobox
              azureActivities={azureActivities}
              value={azureActivityId}
              onChange={handleAzureActivityChange}
              inputStyle={inputStyle}
            />
          </Field>

          <Field label="Proyecto *" error={errors.project}>
            {catalog !== null ? (
              <select
                aria-label="Proyecto *"
                style={inputStyle}
                value={project}
                onChange={e => { setProject(e.target.value); setProjectTouched(true) }}
              >
                <option value="">— Seleccionar proyecto —</option>
                {catalog.projects.map(p => (
                  <option key={p} value={p}>{p}</option>
                ))}
                {!catalog.projects.includes(project) && project && (
                  <option value={project}>{project}</option>
                )}
              </select>
            ) : (
              <input
                type="text"
                value={project}
                onChange={e => { setProject(e.target.value); setProjectTouched(true) }}
                placeholder="Nombre del proyecto"
                style={inputStyle}
              />
            )}
          </Field>

          <Field label="Categoría *" error={errors.category}>
            {catalog !== null ? (
              <select
                aria-label="Categoría *"
                style={inputStyle}
                value={category}
                onChange={e => { setCategory(e.target.value); setCategoryTouched(true) }}
              >
                <option value="">— Seleccionar categoría —</option>
                {catalog.categories.map(c => (
                  <option key={c.id} value={c.name}>{c.name}</option>
                ))}
                {!catalog.categories.some(c => c.name === category) && category && (
                  <option value={category}>{category}</option>
                )}
              </select>
            ) : (
              <input
                type="text"
                value={category}
                onChange={e => { setCategory(e.target.value); setCategoryTouched(true) }}
                placeholder="Categoría"
                style={inputStyle}
              />
            )}
          </Field>

          <Field label="Descripción *" error={errors.registroDiario}>
            <textarea
              style={textareaStyle}
              value={registroDiario}
              onChange={e => setRegistroDiario(e.target.value)}
              placeholder="proyecto/categoría/descripción..."
            />
          </Field>

          <Field label="ID Azure / Mantis / LuxFlow">
            <input
              type="text"
              value={referenceId}
              onChange={e => setReferenceId(e.target.value)}
              placeholder="Ej: 156789, MANTIS-1234, LF-2026-045"
              style={inputStyle}
            />
          </Field>

          {submitError && (
            <div
              style={{
                padding: '10px 14px',
                background: 'var(--rose-50)',
                color: 'var(--block-solid)',
                borderRadius: 'var(--radius-md)',
                font: 'var(--text-sm)',
                marginTop: 8,
              }}
            >
              {submitError}
            </div>
          )}
        </div>

        {/* Footer */}
        <div
          style={{
            padding: '14px 22px',
            borderTop: '1px solid var(--border)',
            display: 'flex',
            gap: 8,
            justifyContent: 'flex-end',
          }}
        >
          <button onClick={onClose} className="btn btn-ghost" disabled={submitting}>
            Cancelar
          </button>
          <button onClick={handleSubmit} className="btn btn-primary" disabled={submitting}>
            {submitting ? 'Guardando...' : 'Guardar cambios'}
          </button>
        </div>
      </div>
    </div>
  )
}
