import type { TipoSolucion } from '../../types'

interface TipoSolucionOption {
  value: TipoSolucion
  label: string
}

const OPTIONS: TipoSolucionOption[] = [
  { value: '', label: 'Sin definir' },
  { value: 'temporal', label: 'Temporal' },
  { value: 'definitiva', label: 'Definitiva' },
]

interface TipoSolucionRadioProps {
  value: TipoSolucion
  onChange: (value: TipoSolucion) => void
  disabled?: boolean
}

// Three mutually-exclusive options as ONE control value ('' | 'temporal' |
// 'definitiva') — never two independent checkboxes, which would make an
// illegal both-true state reachable. A normal controlled React component:
// unlike RichTextField, plain radio inputs have no caret to protect.
export function TipoSolucionRadio({ value, onChange, disabled }: TipoSolucionRadioProps) {
  return (
    <div role="radiogroup" aria-label="Tipo de solución" style={{ display: 'flex', gap: 16 }}>
      {OPTIONS.map(option => (
        <label
          key={option.value || 'none'}
          style={{ display: 'flex', alignItems: 'center', gap: 6, font: 'var(--text-body)', color: 'var(--fg1)' }}
        >
          <input
            type="radio"
            name="tipo_solucion"
            value={option.value}
            checked={value === option.value}
            onChange={() => onChange(option.value)}
            disabled={disabled}
          />
          {option.label}
        </label>
      ))}
    </div>
  )
}
