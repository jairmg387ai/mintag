import type { Period } from './timeLogKpis'

const OPTIONS: { value: Period; label: string }[] = [
  { value: 'week', label: 'Semana actual' },
  { value: 'month', label: 'Mes actual' },
]

interface PeriodSelectorProps {
  value: Period
  onChange: (p: Period) => void
}

export function PeriodSelector({ value, onChange }: PeriodSelectorProps) {
  return (
    <div role="group" aria-label="Período" style={{ display: 'inline-flex', gap: 4 }}>
      {OPTIONS.map(o => (
        <button
          key={o.value}
          type="button"
          aria-pressed={value === o.value}
          className={`btn btn-sm ${value === o.value ? 'btn-primary' : 'btn-ghost'}`}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}
