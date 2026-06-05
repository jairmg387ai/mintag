import React from 'react'
import { X } from 'lucide-react'
import { useAppActions } from '../../store/AppContext'
import { Button } from '../ui/Button'
import { Input as UiInput } from '../ui/Input'

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
          maxWidth,
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
          <h2
            style={{
              font: 'var(--text-h3)',
              color: 'var(--fg1)',
              flex: 1,
              margin: 0,
            }}
          >
            {title}
          </h2>
          <Button variant="ghost" size="sm" icon={X} onClick={closeModal} aria-label="Close" />
        </div>

        {/* Body */}
        <div style={{ padding: '20px 22px', overflowY: 'auto', flex: 1 }}>
          {children}
        </div>

        {/* Footer */}
        {footer && (
          <div
            style={{
              padding: '14px 22px',
              borderTop: '1px solid var(--border)',
              display: 'flex',
              gap: 8,
              justifyContent: 'flex-end',
            }}
          >
            {footer}
          </div>
        )}
      </div>
    </div>
  )
}

// Field label wrapper — used by all modals
export function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 14 }}>
      <label
        className="label"
        style={{ marginBottom: 6, display: 'block' }}
      >
        {label}
      </label>
      {children}
    </div>
  )
}

// Input — re-exports the design-system Input from ui/
export function Input(props: Omit<React.InputHTMLAttributes<HTMLInputElement>, 'prefix'>) {
  return <UiInput {...props} />
}

// Textarea styled with same tokens as Input
export function Textarea(props: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      {...props}
      style={{
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
        ...props.style,
      }}
    />
  )
}

// Select styled with same tokens as Input
export function Select(props: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      {...props}
      style={{
        width: '100%',
        padding: '8px 12px',
        border: '1px solid var(--border-strong)',
        borderRadius: 'var(--radius-md)',
        font: 'var(--text-body)',
        color: 'var(--fg1)',
        background: 'var(--bg-sunken)',
        outline: 'none',
        ...props.style,
      }}
    />
  )
}

// BtnPrimary — legacy alias; Slice 3e modals still use this
export function BtnPrimary(props: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      {...props}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        padding: '7px 14px',
        borderRadius: 'var(--radius-md)',
        border: 'none',
        cursor: 'pointer',
        font: 'var(--text-h4)',
        background: 'var(--brand)',
        color: '#fff',
        ...props.style,
      }}
    >
      {props.children}
    </button>
  )
}

// BtnGhost — legacy alias; Slice 3e modals still use this
export function BtnGhost(props: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      {...props}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        padding: '7px 14px',
        borderRadius: 'var(--radius-md)',
        cursor: 'pointer',
        font: 'var(--text-h4)',
        background: 'none',
        color: 'var(--fg2)',
        border: '1px solid var(--border)',
        ...props.style,
      }}
    >
      {props.children}
    </button>
  )
}
