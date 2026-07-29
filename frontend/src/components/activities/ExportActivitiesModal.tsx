import { useState, useEffect, useRef, type CSSProperties } from 'react'
import { X } from 'lucide-react'
import { exportActivities } from '../../api/client'

interface ExportActivitiesModalProps {
  open: boolean
  onClose: () => void
  defaultDate: string // YYYY-MM-DD, used to seed both fields on open
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

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 14 }}>
      <label className="label" style={{ marginBottom: 6, display: 'block' }}>
        {label}
      </label>
      {children}
    </div>
  )
}

// ExportActivitiesModal — D1 overlay dismissal applied from day one (design
// "ExportActivitiesModal.tsx (new) — D1 overlay pattern from day one"): a
// same-target mousedown+click latch, so a drag-select that starts inside the
// card and releases outside it does not close the modal, while a direct
// overlay click and the X/Cancel buttons always do.
export function ExportActivitiesModal({ open, onClose, defaultDate }: ExportActivitiesModalProps) {
  const [from, setFrom] = useState(defaultDate)
  const [to, setTo] = useState(defaultDate)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const pressedOnOverlay = useRef(false)

  useEffect(() => {
    if (open) {
      setFrom(defaultDate)
      setTo(defaultDate)
      setSubmitting(false)
      setError('')
    }
  }, [open, defaultDate])

  if (!open) return null

  async function handleSubmit() {
    if (!from || !to) {
      setError('Selecciona ambas fechas')
      return
    }
    if (from > to) {
      setError('La fecha inicial no puede ser posterior a la final')
      return
    }
    setError('')
    setSubmitting(true)
    try {
      await exportActivities(from, to)
      onClose()
    } catch {
      setError('No se pudo generar el archivo')
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
          maxWidth: 420,
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
            Exportar actividades
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
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Field label="Desde">
              <input
                type="date"
                value={from}
                onChange={e => setFrom(e.target.value)}
                style={inputStyle}
              />
            </Field>
            <Field label="Hasta">
              <input
                type="date"
                value={to}
                onChange={e => setTo(e.target.value)}
                style={inputStyle}
              />
            </Field>
          </div>

          {error && (
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
              {error}
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
            disabled={submitting}
          >
            {submitting ? 'Exportando...' : 'Exportar'}
          </button>
        </div>
      </div>
    </div>
  )
}
