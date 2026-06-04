interface FilterChipProps {
  label: string
  active: boolean
  onClick: () => void
}

export function FilterChip({ label, active, onClick }: FilterChipProps) {
  return (
    <span
      onClick={onClick}
      className={[
        'inline-flex items-center rounded-full px-3 py-1 text-[0.78em] cursor-pointer select-none whitespace-nowrap transition-all duration-150',
        active
          ? 'bg-blue-bg border border-blue text-blue font-medium'
          : 'bg-surface2 border border-border text-text2 hover:text-text hover:border-border2',
      ].join(' ')}
    >
      {label}
    </span>
  )
}
