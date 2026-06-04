import { X, Check } from 'lucide-react'
import { useAppState } from '../../store/AppContext'

export function ToastContainer() {
  const { toasts } = useAppState()

  return (
    <div
      style={{
        position: 'fixed',
        bottom: 24,
        right: 24,
        zIndex: 999,
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
      }}
    >
      {toasts.map(t => (
        <div
          key={t.id}
          style={{
            background: t.isError ? 'var(--block-bg)' : 'var(--bg-surface)',
            border: t.isError
              ? '1px solid var(--block-solid)'
              : '1px solid var(--border-strong)',
            borderRadius: 'var(--radius-lg)',
            padding: '10px 16px',
            fontSize: '0.85em',
            color: t.isError ? 'var(--block-fg)' : 'var(--fg1)',
            boxShadow: 'var(--shadow-lg)',
            animation: 'slideIn 0.2s ease',
            display: 'flex',
            alignItems: 'center',
            gap: 8,
          }}
        >
          {t.isError ? (
            <X size={14} strokeWidth={2} style={{ color: 'var(--block-fg)', flexShrink: 0 }} />
          ) : (
            <Check size={14} strokeWidth={2} style={{ color: 'var(--done-fg)', flexShrink: 0 }} />
          )}
          {t.message}
        </div>
      ))}
      <style>{`
        @keyframes slideIn {
          from { transform: translateX(100%); opacity: 0; }
          to { transform: translateX(0); opacity: 1; }
        }
      `}</style>
    </div>
  )
}
