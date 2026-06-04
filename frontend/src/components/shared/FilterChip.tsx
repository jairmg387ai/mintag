interface FilterChipProps {
  label: string
  active: boolean
  onClick: () => void
}

export function FilterChip({ label, active, onClick }: FilterChipProps) {
  return (
    <span
      onClick={onClick}
      style={{
        background: active ? 'rgba(59,130,246,0.12)' : 'var(--color-surface2)',
        border: `1px solid ${active ? 'var(--color-blue)' : 'var(--color-border)'}`,
        borderRadius: 20,
        padding: '4px 12px',
        fontSize: '0.78em',
        cursor: 'pointer',
        color: active ? 'var(--color-blue)' : 'var(--color-text2)',
        transition: 'all 0.15s',
        whiteSpace: 'nowrap',
        userSelect: 'none',
      }}
    >
      {label}
    </span>
  )
}
