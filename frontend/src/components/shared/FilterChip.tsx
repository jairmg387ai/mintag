interface FilterChipProps {
  label: string
  active: boolean
  onClick: () => void
}

export function FilterChip({ label, active, onClick }: FilterChipProps) {
  return (
    <span
      onClick={onClick}
      className="chip"
      style={{
        cursor: 'pointer',
        userSelect: 'none',
        background: active ? 'var(--brand-subtle)' : 'var(--bg-surface)',
        color: active ? 'var(--brand)' : 'var(--fg2)',
        border: active
          ? '1px solid var(--brand)'
          : '1px solid var(--border-strong)',
        transition: 'background 0.15s ease, color 0.15s ease, border-color 0.15s ease',
      }}
    >
      {label}
    </span>
  )
}
