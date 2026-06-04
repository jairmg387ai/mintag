interface StatCardProps {
  label: string
  value: number | string
  color?: string
}

export function StatCard({ label, value, color }: StatCardProps) {
  return (
    <div style={{
      background: 'var(--color-surface)',
      border: '1px solid var(--color-border)',
      borderRadius: 10,
      padding: 16,
    }}>
      <div style={{ fontSize: '0.72em', color: 'var(--color-text3)', textTransform: 'uppercase', letterSpacing: '0.5px', marginBottom: 4 }}>
        {label}
      </div>
      <div style={{ fontSize: '1.8em', fontWeight: 700, color: color ?? 'var(--color-text)' }}>
        {value}
      </div>
    </div>
  )
}
