import { useState, useEffect, useRef, type CSSProperties } from 'react'
import { X } from 'lucide-react'
import type { ActivityCatalog, AzureActivity } from '../../types'
import { createActivity } from '../../api/client'
import { findDefaultAzureActivity } from './azureActivity'

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
  const [descriptionTouched, setDescriptionTouched] = useState(false)
  const [azureActivityId, setAzureActivityId] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  // Reset form when modal opens
  useEffect(() => {
    if (open) {
      setDate(defaultDate)
      setHours('')
      setProject(catalog?.projects[0] ?? '')
      setCategory(catalog?.categories[0] ?? '')
      setDescription('')
      setDescriptionTouched(false)
      setAzureActivityId('')
      setErrors({})
      setSubmitError('')
    }
  }, [open, defaultDate, catalog])

  // Auto-prefill description while pristine
  useEffect(() => {
    if (!descriptionTouched && project && category) {
      setDescription(`${project}/${category}/`)
    }
  }, [project, category, descriptionTouched])

  if (!open) return null

  const defaultAzureActivity = findDefaultAzureActivity(azureActivities)

  const emptyCatalog =
    catalog !== null &&
    catalog.projects.length === 0 &&
    catalog.categories.length === 0

  function validate(): boolean {
    const errs: Record<string, string> = {}
    const h = parseFloat(hours)
    if (!hours || isNaN(h) || h <= 0) errs.hours = 'Hours must be greater than 0'
    if (!project.trim()) errs.project = 'Project is required'
    if (!category.trim()) errs.category = 'Category is required'

    // The actual description part is what comes after "{project}/{category}/"
    const prefix = `${project}/${category}/`
    const descPart = description.startsWith(prefix)
      ? description.slice(prefix.length).trim()
      : description.trim()
    if (!descPart) errs.description = 'Description is required'

    setErrors(errs)
    return Object.keys(errs).length === 0
  }

  async function handleSubmit() {
    if (!validate()) return
    setSubmitting(true)
    setSubmitError('')

    const h = parseFloat(hours)
    const prefix = `${project}/${category}/`
    const descPart = (description.startsWith(prefix)
      ? description.slice(prefix.length)
      : description).trim()
    const registro_diario = `${project}/${category}/${descPart}`

    try {
      await createActivity({
        date,
        hours: h,
        project,
        category,
        registro_diario,
        source: 'manual',
        ...(azureActivityId ? { azure_activity_id: Number(azureActivityId) } : {}),
      })
      onCreated()
      onClose()
    } catch (e: unknown) {
      setSubmitError(e instanceof Error ? e.message : 'Error creating activity')
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
            aria-label="Close"
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
              No projects or categories available
            </div>
          )}

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Field label="Date *" error={errors.date}>
              <input
                type="date"
                value={date}
                onChange={e => setDate(e.target.value)}
                style={selectStyle}
              />
            </Field>
            <Field label="Hours *" error={errors.hours}>
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

          <Field label="Project *" error={errors.project}>
            {catalog !== null ? (
              <select
                style={selectStyle}
                value={project}
                onChange={e => setProject(e.target.value)}
                disabled={emptyCatalog}
              >
                {catalog.projects.length === 0 ? (
                  <option value="">— no projects —</option>
                ) : (
                  catalog.projects.map(p => (
                    <option key={p} value={p}>{p}</option>
                  ))
                )}
              </select>
            ) : (
              <input
                type="text"
                value={project}
                onChange={e => setProject(e.target.value)}
                placeholder="Project name"
                style={selectStyle}
              />
            )}
          </Field>

          <Field label="Category *" error={errors.category}>
            {catalog !== null ? (
              <select
                style={selectStyle}
                value={category}
                onChange={e => setCategory(e.target.value)}
                disabled={emptyCatalog}
              >
                {catalog.categories.length === 0 ? (
                  <option value="">— no categories —</option>
                ) : (
                  catalog.categories.map(c => (
                    <option key={c} value={c}>{c}</option>
                  ))
                )}
              </select>
            ) : (
              <input
                type="text"
                value={category}
                onChange={e => setCategory(e.target.value)}
                placeholder="Category"
                style={selectStyle}
              />
            )}
          </Field>

          <Field label="Description *" error={errors.description}>
            <textarea
              style={textareaStyle}
              value={description}
              onChange={e => {
                setDescription(e.target.value)
                setDescriptionTouched(true)
              }}
              placeholder={`${project || 'project'}/${category || 'category'}/description...`}
            />
          </Field>

          <Field label="Azure Activity">
            <select
              style={selectStyle}
              value={azureActivityId}
              onChange={e => setAzureActivityId(e.target.value)}
            >
              <option value="">
                Use default{defaultAzureActivity ? ` (${defaultAzureActivity.label})` : ''}
              </option>
              {azureActivities.map(a => (
                <option key={a.id} value={a.id}>
                  {a.label}{a.is_default ? ' (default)' : ''}
                </option>
              ))}
            </select>
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
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            className="btn btn-primary"
            disabled={submitting || emptyCatalog}
          >
            {submitting ? 'Creating...' : 'Create Activity'}
          </button>
        </div>
      </div>
    </div>
  )
}
