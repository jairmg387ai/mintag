import { useState, useEffect, useRef, type CSSProperties } from 'react'
import { X } from 'lucide-react'
import type { ActivityCatalog, ActivityValidationSettings, AzureActivity } from '../../types'
import { createActivity, getActivityValidationSettings } from '../../api/client'
import { AzureActivityCombobox } from './AzureActivityCombobox'
import { resolveAutofill } from './activityAutofill'

// Mirrors store.MaxHoursPerActivityEntry (internal/store/activity_validation.go)
// — the cap itself is fixed, not configurable, so this is a literal, not a
// value fetched from the backend. This is a client-side pre-check for
// instant feedback only; the backend enforces the real rule regardless.
const MAX_HOURS_PER_ENTRY = 8

const DAY_NAMES = ['domingo', 'lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábado']

// Computes the day of week from a YYYY-MM-DD string using LOCAL date
// components, NOT `new Date(dateString)` directly — parsing an ISO date
// string that way is interpreted as UTC midnight by some engines and can
// shift the displayed weekday by one day depending on the browser's timezone.
function dayOfWeek(dateStr: string): number {
  const [y, m, d] = dateStr.split('-').map(Number)
  return new Date(y, m - 1, d).getDay()
}

function isWeekendDate(dateStr: string): boolean {
  const dow = dayOfWeek(dateStr)
  return dow === 0 || dow === 6
}

interface NewActivityModalProps {
  open: boolean
  onClose: () => void
  onCreated: () => void
  catalog: ActivityCatalog | null
  defaultDate: string // YYYY-MM-DD
  azureActivities: AzureActivity[]
}

const selectStyle: CSSProperties = {
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
  width: '100%',
  padding: '8px 12px',
  border: '1px solid var(--border-strong)',
  borderRadius: 'var(--radius-md)',
  font: 'var(--text-body)',
  color: 'var(--fg1)',
  background: 'var(--bg-sunken)',
  outline: 'none',
  resize: 'vertical',
  minHeight: 80,
  fontFamily: 'inherit',
  boxSizing: 'border-box',
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

export function NewActivityModal({ open, onClose, onCreated, catalog, defaultDate, azureActivities }: NewActivityModalProps) {
  const pressedOnOverlay = useRef(false)
  const [date, setDate] = useState(defaultDate)
  const [hours, setHours] = useState('')
  const [project, setProject] = useState('')
  const [category, setCategory] = useState('')
  const [description, setDescription] = useState('')
  const [azureActivityId, setAzureActivityId] = useState('')
  const [referenceId, setReferenceId] = useState('')
  // Tracks whether the user has explicitly edited project/category by hand
  // in this session, so a later Azure work-item selection knows which
  // fields it's still allowed to autofill (see resolveAutofill).
  const [projectTouched, setProjectTouched] = useState(false)
  const [categoryTouched, setCategoryTouched] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')
  const [validationSettings, setValidationSettings] = useState<ActivityValidationSettings | null>(null)

  // Fetched once on mount, not tied to `open` — these toggles don't change
  // while the modal is open, and a failed fetch just leaves both client-side
  // checks off (fail open, matching the backend's own fail-open posture for
  // the closed-work-item check).
  useEffect(() => {
    getActivityValidationSettings().then(setValidationSettings).catch(() => setValidationSettings(null))
  }, [])

  // Reset form when modal opens
  useEffect(() => {
    if (open) {
      setDate(defaultDate)
      setHours('')
      // Left blank (not pre-seeded to the catalog's first entry) so that an
      // Azure work item without a mapped project/category leaves these
      // fields empty and the required-field validation catches it, instead
      // of silently submitting whatever happened to be first in the list.
      setProject('')
      setCategory('')
      setDescription('')
      setAzureActivityId('')
      setReferenceId('')
      setErrors({})
      setSubmitError('')
      setProjectTouched(false)
      setCategoryTouched(false)
    }
  }, [open, defaultDate, catalog])

  if (!open) return null

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

  const emptyCatalog =
    catalog !== null &&
    catalog.projects.length === 0 &&
    catalog.categories.length === 0

  function validate(): boolean {
    const errs: Record<string, string> = {}
    const h = parseFloat(hours)
    if (!hours || isNaN(h) || h <= 0) {
      errs.hours = 'Las horas deben ser mayores que 0'
    } else if (validationSettings?.max_hours_per_entry && h > MAX_HOURS_PER_ENTRY) {
      errs.hours = `No se permiten más de ${MAX_HOURS_PER_ENTRY} horas en un solo registro`
    }
    if (!project.trim()) errs.project = 'El proyecto es obligatorio'
    if (!category.trim()) errs.category = 'La categoría es obligatoria'
    if (!description.trim()) errs.description = 'La descripción es obligatoria'

    setErrors(errs)
    return Object.keys(errs).length === 0
  }

  async function handleSubmit() {
    if (!validate()) return

    if (validationSettings?.weekend_confirm && isWeekendDate(date)) {
      const confirmed = window.confirm(
        `Estás registrando horas para el ${date} (${DAY_NAMES[dayOfWeek(date)]}), un día no laboral.\n¿Deseas continuar?`
      )
      if (!confirmed) return
    }

    setSubmitting(true)
    setSubmitError('')

    const h = parseFloat(hours)
    const registro_diario = description.trim()

    try {
      await createActivity({
        date,
        hours: h,
        project,
        category,
        registro_diario,
        source: 'manual',
        ...(azureActivityId ? { azure_activity_id: Number(azureActivityId) } : {}),
        ...(referenceId.trim() ? { reference_id: referenceId.trim() } : {}),
      })
      onCreated()
      onClose()
    } catch (e: unknown) {
      setSubmitError(e instanceof Error ? e.message : 'No se pudo crear la actividad')
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
            Nueva Actividad
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
          {emptyCatalog && (
            <div
              style={{
                padding: '10px 14px',
                background: 'var(--amber-50)',
                color: 'var(--amber-700)',
                borderRadius: 'var(--radius-md)',
                font: 'var(--text-sm)',
                marginBottom: 14,
              }}
            >
              No hay proyectos ni categorías disponibles
            </div>
          )}

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Field label="Fecha *" error={errors.date}>
              <input
                type="date"
                value={date}
                onChange={e => setDate(e.target.value)}
                style={selectStyle}
              />
            </Field>
            <Field label="Horas *" error={errors.hours}>
              <input
                type="number"
                step="0.25"
                min="0.25"
                value={hours}
                onChange={e => setHours(e.target.value)}
                placeholder="0.00"
                style={selectStyle}
              />
            </Field>
          </div>

          <Field label="Actividad de Azure">
            <AzureActivityCombobox
              azureActivities={azureActivities}
              value={azureActivityId}
              onChange={handleAzureActivityChange}
              inputStyle={selectStyle}
            />
          </Field>

          <Field label="Proyecto *" error={errors.project}>
            {catalog !== null ? (
              <select
                aria-label="Proyecto *"
                style={selectStyle}
                value={project}
                onChange={e => { setProject(e.target.value); setProjectTouched(true) }}
                disabled={emptyCatalog}
              >
                {catalog.projects.length === 0 ? (
                  <option value="">— sin proyectos —</option>
                ) : (
                  <>
                    <option value="">— Seleccionar proyecto —</option>
                    {catalog.projects.map(p => (
                      <option key={p.name} value={p.name}>{p.name}</option>
                    ))}
                  </>
                )}
              </select>
            ) : (
              <input
                type="text"
                value={project}
                onChange={e => { setProject(e.target.value); setProjectTouched(true) }}
                placeholder="Nombre del proyecto"
                style={selectStyle}
              />
            )}
          </Field>

          <Field label="Categoría *" error={errors.category}>
            {catalog !== null ? (
              <select
                aria-label="Categoría *"
                style={selectStyle}
                value={category}
                onChange={e => { setCategory(e.target.value); setCategoryTouched(true) }}
                disabled={emptyCatalog}
              >
                {catalog.categories.length === 0 ? (
                  <option value="">— sin categorías —</option>
                ) : (
                  <>
                    <option value="">— Seleccionar categoría —</option>
                    {catalog.categories.map(c => (
                      <option key={c.id} value={c.name}>{c.name}</option>
                    ))}
                  </>
                )}
              </select>
            ) : (
              <input
                type="text"
                value={category}
                onChange={e => { setCategory(e.target.value); setCategoryTouched(true) }}
                placeholder="Categoría"
                style={selectStyle}
              />
            )}
          </Field>

          <Field label="Descripción *" error={errors.description}>
            <textarea
              style={textareaStyle}
              value={description}
              onChange={e => setDescription(e.target.value)}
              placeholder="Descripción de la actividad..."
            />
          </Field>

          <Field label="ID Azure / Mantis / LuxFlow">
            <input
              type="text"
              value={referenceId}
              onChange={e => setReferenceId(e.target.value)}
              placeholder="Ej: 156789, MANTIS-1234, LF-2026-045"
              style={selectStyle}
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
          <button
            onClick={onClose}
            className="btn btn-ghost"
            disabled={submitting}
          >
            Cancelar
          </button>
          <button
            onClick={handleSubmit}
            className="btn btn-primary"
            disabled={submitting || emptyCatalog}
          >
            {submitting ? 'Creando...' : 'Crear actividad'}
          </button>
        </div>
      </div>
    </div>
  )
}
