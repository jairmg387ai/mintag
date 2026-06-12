import { useState, useEffect, type CSSProperties } from 'react'
import { X } from 'lucide-react'
import type { DailyActivity, ActivityCatalog } from '../../types'
import { updateActivity } from '../../api/client'

interface EditActivityModalProps {
  activity: DailyActivity | null
  open: boolean
  onClose: () => void
  onSaved: () => void
  catalog: ActivityCatalog | null
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

export function EditActivityModal({ activity, open, onClose, onSaved, catalog }: EditActivityModalProps) {
  const [hours, setHours] = useState('')
  const [project, setProject] = useState('')
  const [category, setCategory] = useState('')
  const [registroDiario, setRegistroDiario] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  useEffect(() => {
    if (open && activity) {
      setHours(String(activity.hours))
      setProject(activity.project)
      setCategory(activity.category)
      setRegistroDiario(activity.registro_diario)
      setErrors({})
      setSubmitError('')
    }
  }, [open, activity])

  if (!open || !activity) return null

  function validate(): boolean {
    const errs: Record<string, string> = {}
    const h = parseFloat(hours)
    if (!hours || isNaN(h) || h <= 0) errs.hours = 'Hours must be greater than 0'
    if (!project.trim()) errs.project = 'Project is required'
    if (!category.trim()) errs.category = 'Category is required'
    if (!registroDiario.trim()) errs.registroDiario = 'Description is required'
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
      })
      onSaved()
      onClose()
    } catch (e: unknown) {
      setSubmitError(e instanceof Error ? e.message : 'Error updating activity')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      onClick={onClose}
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
            Edit Activity
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
          <Field label="Hours *" error={errors.hours}>
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

          <Field label="Project *" error={errors.project}>
            {catalog !== null ? (
              <select
                style={inputStyle}
                value={project}
                onChange={e => setProject(e.target.value)}
              >
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
                onChange={e => setProject(e.target.value)}
                placeholder="Project name"
                style={inputStyle}
              />
            )}
          </Field>

          <Field label="Category *" error={errors.category}>
            {catalog !== null ? (
              <select
                style={inputStyle}
                value={category}
                onChange={e => setCategory(e.target.value)}
              >
                {catalog.categories.map(c => (
                  <option key={c} value={c}>{c}</option>
                ))}
                {!catalog.categories.includes(category) && category && (
                  <option value={category}>{category}</option>
                )}
              </select>
            ) : (
              <input
                type="text"
                value={category}
                onChange={e => setCategory(e.target.value)}
                placeholder="Category"
                style={inputStyle}
              />
            )}
          </Field>

          <Field label="Description *" error={errors.registroDiario}>
            <textarea
              style={textareaStyle}
              value={registroDiario}
              onChange={e => setRegistroDiario(e.target.value)}
              placeholder="project/category/description..."
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
            Cancel
          </button>
          <button onClick={handleSubmit} className="btn btn-primary" disabled={submitting}>
            {submitting ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </div>
    </div>
  )
}
