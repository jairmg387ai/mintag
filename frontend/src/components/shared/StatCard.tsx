import { Card } from '../ui'

interface StatCardProps {
  label: string
  value: number | string
  color?: string
}

export function StatCard({ label, value, color }: StatCardProps) {
  return (
    <Card
      padding
      className="transition-transform duration-150 hover:-translate-y-px hover:shadow-[0_4px_16px_rgba(0,0,0,0.25)]"
      style={color ? { borderTop: `2px solid ${color}` } : undefined}
    >
      <div className="text-[0.7em] text-text3 uppercase tracking-[0.6px] mb-2">{label}</div>
      <div className="text-[2em] font-bold leading-none" style={{ color: color ?? 'var(--color-text)' }}>
        {value}
      </div>
    </Card>
  )
}
