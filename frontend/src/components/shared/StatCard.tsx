interface StatCardProps {
  label: string
  value: number | string
  color?: string
}

export function StatCard({ label, value, color }: StatCardProps) {
  return (
    <div
      className="stat-card"
      style={color ? { borderTopColor: color } : undefined}
    >
      <div style={{ fontSize: '0.7em', color: 'var(--color-text3)', textTransform: 'uppercase', letterSpacing: '0.6px', marginBottom: 8 }}>
        {label}
      </div>
      <div style={{ fontSize: '2em', fontWeight: 700, color: color ?? 'var(--color-text)', lineHeight: 1 }}>
        {value}
      </div>
    </div>
  )
}
