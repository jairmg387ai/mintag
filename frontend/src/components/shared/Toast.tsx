import { useAppState } from '../../store/AppContext'

export function ToastContainer() {
  const { toasts } = useAppState()

  return (
    <div style={{
      position: 'fixed',
      bottom: 24,
      right: 24,
      zIndex: 999,
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
    }}>
      {toasts.map(t => (
        <div
          key={t.id}
          style={{
            background: t.isError ? 'rgba(239,68,68,0.12)' : 'var(--color-surface2)',
            border: t.isError ? '1px solid var(--color-red)' : '1px solid var(--color-border2)',
            borderRadius: 10,
            padding: '10px 16px',
            fontSize: '0.85em',
            color: 'var(--color-text)',
            boxShadow: '0 4px 24px rgba(0,0,0,.4)',
            animation: 'slideIn 0.2s ease',
            display: 'flex',
            alignItems: 'center',
            gap: 8,
          }}
        >
          {t.isError && <span style={{ color: 'var(--color-red)' }}>✕</span>}
          {!t.isError && <span style={{ color: 'var(--color-green)' }}>✓</span>}
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
