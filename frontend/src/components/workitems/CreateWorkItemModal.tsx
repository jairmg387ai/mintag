import { useState, useEffect, useRef, type CSSProperties } from 'react'
import { X } from 'lucide-react'
import type { ActivityCatalog, CreatedWorkItemResponse } from '../../types'
import { createAzureWorkItem } from '../../api/client'
import { ClassificationTreePicker } from './ClassificationTreePicker'

interface CreateWorkItemModalProps {
  open: boolean
  onClose: () => void
  onCreated: (result: CreatedWorkItemResponse) => void
  catalog?: ActivityCatalog | null
}

const DEFAULT_ESTIMATE = '24'

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
  ...selectStyle,
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

export function CreateWorkItemModal({ open, onClose, onCreated, catalog = null }: CreateWorkItemModalProps) {
  const pressedOnOverlay = useRef(false)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [areaPath, setAreaPath] = useState('')
  const [iterationPath, setIterationPath] = useState('')
  const [estimate, setEstimate] = useState(DEFAULT_ESTIMATE)
  const [project, setProject] = useState('')
  const [category, setCategory] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  useEffect(() => {
    if (open) {
      setTitle('')
      setDescription('')
      setAreaPath('')
      setIterationPath('')
      setEstimate(DEFAULT_ESTIMATE)
      setProject('')
      setCategory('')
      setErrors({})
      setSubmitError('')
    }
  }, [open])

  if (!open) return null

  function validate(): boolean {
    const errs: Record<string, string> = {}
    if (!title.trim()) errs.title = 'El título es obligatorio'
    if (!areaPath) errs.areaPath = 'El área es obligatoria'
    if (!iterationPath) errs.iterationPath = 'La iteración es obligatoria'
    setErrors(errs)
    return Object.keys(errs).length === 0
  }

  async function handleSubmit() {
    if (!validate()) return
    setSubmitting(true)
    setSubmitError('')
    try {
      const estimateValue = parseFloat(estimate)
      const categoryId = category ? catalog?.categories.find(c => c.name === category)?.id : undefined
      const result = await createAzureWorkItem({
        title: title.trim(),
        ...(description.trim() ? { description: description.trim() } : {}),
        area_path: areaPath,
        iteration_path: iterationPath,
        ...(!isNaN(estimateValue) && estimateValue > 0 ? { original_estimate: estimateValue } : {}),
        ...(project ? { project } : {}),
        ...(categoryId !== undefined ? { category_id: categoryId } : {}),
      })
      onCreated(result)
      onClose()
    } catch (e: unknown) {
      setSubmitError(e instanceof Error ? e.message : 'No se pudo crear el work item')
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
            Nuevo Work Item
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

        <div style={{ padding: '20px 22px', overflowY: 'auto', flex: 1 }}>
          <Field label="Título *" error={errors.title}>
            <input
              type="text"
              value={title}
              onChange={e => setTitle(e.target.value)}
              placeholder="Título del work item"
              style={selectStyle}
            />
          </Field>

          <Field label="Descripción">
            <textarea
              style={textareaStyle}
              value={description}
              onChange={e => setDescription(e.target.value)}
              placeholder="Descripción (opcional)..."
            />
          </Field>

          <Field label="Área *" error={errors.areaPath}>
            <ClassificationTreePicker
              kind="areas"
              ariaLabel="Área *"
              value={areaPath}
              onChange={setAreaPath}
              inputStyle={selectStyle}
            />
          </Field>

          <Field label="Iteración *" error={errors.iterationPath}>
            <ClassificationTreePicker
              kind="iterations"
              ariaLabel="Iteración *"
              value={iterationPath}
              onChange={setIterationPath}
              inputStyle={selectStyle}
            />
          </Field>

          <Field label="Estimación (horas)">
            <input
              type="number"
              step="1"
              min="0"
              value={estimate}
              onChange={e => setEstimate(e.target.value)}
              placeholder={DEFAULT_ESTIMATE}
              style={selectStyle}
            />
          </Field>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Field label="Proyecto">
              {catalog !== null ? (
                <select
                  aria-label="Proyecto"
                  style={selectStyle}
                  value={project}
                  onChange={e => setProject(e.target.value)}
                >
                  <option value="">— Sin registrar en catálogo —</option>
                  {catalog.projects.map(p => (
                    <option key={p.name} value={p.name}>{p.name}</option>
                  ))}
                </select>
              ) : (
                <input
                  type="text"
                  value={project}
                  onChange={e => setProject(e.target.value)}
                  placeholder="Nombre del proyecto (opcional)"
                  style={selectStyle}
                />
              )}
            </Field>

            <Field label="Categoría">
              {catalog !== null ? (
                <select
                  aria-label="Categoría"
                  style={selectStyle}
                  value={category}
                  onChange={e => setCategory(e.target.value)}
                >
                  <option value="">— Sin registrar en catálogo —</option>
                  {catalog.categories.map(c => (
                    <option key={c.id} value={c.name}>{c.name}</option>
                  ))}
                </select>
              ) : (
                <input
                  type="text"
                  value={category}
                  onChange={e => setCategory(e.target.value)}
                  placeholder="Categoría (opcional)"
                  style={selectStyle}
                />
              )}
            </Field>
          </div>

          {(project || category) && (
            <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', marginTop: -6, marginBottom: 14 }}>
              Se registrará este work item en el catálogo de actividades de Azure.
            </div>
          )}

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
            {submitting ? 'Creando...' : 'Crear work item'}
          </button>
        </div>
      </div>
    </div>
  )
}
