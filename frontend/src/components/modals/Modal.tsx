import React, { useRef } from 'react'
import { X } from 'lucide-react'
import { useAppActions } from '../../store/AppContext'
import { Button } from '../ui/Button'

interface ModalProps {
  title: string
  children: React.ReactNode
  footer?: React.ReactNode
  maxWidth?: number
}

export function Modal({ title, children, footer, maxWidth = 640 }: ModalProps) {
  const { closeModal } = useAppActions()
  const pressedOnOverlay = useRef(false)

  return (
    <div
      onMouseDown={e => { pressedOnOverlay.current = e.target === e.currentTarget }}
      onClick={e => {
        if (pressedOnOverlay.current && e.target === e.currentTarget) closeModal()
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
