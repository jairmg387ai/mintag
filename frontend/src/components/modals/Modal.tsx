import React from 'react'
import { useAppActions } from '../../store/AppContext'

interface ModalProps {
  title: string
  children: React.ReactNode
  footer?: React.ReactNode
  maxWidth?: number
}

export function Modal({ title, children, footer, maxWidth = 640 }: ModalProps) {
  const { closeModal } = useAppActions()

  return (
    <div
      onClick={closeModal}
      style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.65)', zIndex: 100, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 20 }}
    >
      <div
        onClick={e => e.stopPropagation()}
        style={{ background: 'var(--color-surface)', border: '1px solid var(--color-border2)', borderRadius: 14, width: '100%', maxWidth, maxHeight: '90vh', display: 'flex', flexDirection: 'column', boxShadow: '0 4px 24px rgba(0,0,0,.4)' }}
      >
        {/* Header */}
        <div style={{ padding: '18px 22px', borderBottom: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', gap: 10 }}>
          <h2 style={{ fontSize: '1em', fontWeight: 600, flex: 1 }}>{title}</h2>
          <button
            onClick={closeModal}
            style={{ background: 'none', border: 'none', color: 'var(--color-text3)', cursor: 'pointer', padding: 4, borderRadius: 6, display: 'flex' }}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: '20px 22px', overflowY: 'auto', flex: 1 }}>
          {children}
        </div>

        {/* Footer */}
        {footer && (
          <div style={{ padding: '14px 22px', borderTop: '1px solid var(--color-border)', display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            {footer}
          </div>
        )}
      </div>
    </div>
  )
}

// Reusable form helpers
export function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 14 }}>
      <label style={{ fontSize: '0.8em', color: 'var(--color-text2)', marginBottom: 4, display: 'block', fontWeight: 500 }}>{label}</label>
      {children}
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  background: 'var(--color-surface2)',
  border: '1px solid var(--color-border)',
  borderRadius: 7,
  color: 'var(--color-text)',
  padding: '8px 12px',
  fontSize: '0.88em',
  width: '100%',
  outline: 'none',
  fontFamily: 'inherit',
}

export function Input(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} style={{ ...inputStyle, ...props.style }} />
}

export function Textarea(props: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} style={{ ...inputStyle, resize: 'vertical', minHeight: 80, ...props.style }} />
}

export function Select(props: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return <select {...props} style={{ ...inputStyle, ...props.style }} />
}

export function BtnPrimary(props: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button {...props} style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '7px 14px', borderRadius: 7, border: 'none', cursor: 'pointer', fontSize: '0.85em', fontWeight: 500, background: 'var(--color-blue)', color: '#fff', ...props.style }}>
      {props.children}
    </button>
  )
}

export function BtnGhost(props: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button {...props} style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '7px 14px', borderRadius: 7, cursor: 'pointer', fontSize: '0.85em', fontWeight: 500, background: 'var(--color-surface2)', color: 'var(--color-text2)', border: '1px solid var(--color-border)', ...props.style }}>
      {props.children}
    </button>
  )
}
