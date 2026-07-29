import type { LucideIcon } from 'lucide-react'

interface StatCardProps {
  icon: LucideIcon
  iconBg: string
  iconFg: string
  value: number | string
  label: string
  delta?: string
  deltaColor?: string
  emphasize?: boolean
  onClick?: () => void
}

export function StatCard({
  icon: Icon,
  iconBg,
  iconFg,
  value,
  label,
  delta,
  deltaColor,
  emphasize,
  onClick,
}: StatCardProps) {
  return (
    <button
      className="card"
      onClick={onClick}
      style={{
        padding: 18,
        textAlign: 'left',
        display: 'block',
        width: '100%',
        cursor: onClick ? 'pointer' : 'default',
        boxShadow: emphasize ? 'var(--shadow-md)' : 'var(--shadow-sm)',
        borderColor: emphasize ? 'var(--rose-200)' : 'var(--border)',
        background: 'var(--bg-surface)',
      }}
    >
      <span
        style={{
          width: 36,
          height: 36,
          borderRadius: 'var(--radius-md)',
          background: iconBg,
          color: iconFg,
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon size={20} strokeWidth={1.75} />
      </span>
      <div
        style={{
          font: 'var(--text-display)',
          letterSpacing: 'var(--tracking-tight)',
          color: emphasize ? 'var(--block-fg)' : 'var(--fg1)',
          margin: '14px 0 2px',
        }}
      >
        {value}
      </div>
      <div style={{ font: 'var(--text-sm)', color: 'var(--fg2)' }}>{label}</div>
      {delta && (
        <span
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 4,
            font: 'var(--text-caption)',
            fontWeight: 600,
            color: deltaColor ?? 'var(--fg3)',
            marginTop: 10,
          }}
        >
          {delta}
        </span>
      )}
    </button>
  )
}
