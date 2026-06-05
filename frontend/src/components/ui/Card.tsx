import React from 'react'

interface CardProps {
  children: React.ReactNode
  className?: string
  style?: React.CSSProperties
  padding?: boolean
}

export const Card = React.forwardRef<HTMLDivElement, CardProps>(
  ({ children, className = '', style, padding = false }, ref) => (
    <div
      ref={ref}
      className={['card', padding ? 'p-5' : '', className].filter(Boolean).join(' ')}
      style={style}
    >
      {children}
    </div>
  )
)
Card.displayName = 'Card'

interface CardHeaderProps {
  children: React.ReactNode
  icon?: React.ReactNode
  right?: React.ReactNode
  size?: 'sm' | 'md'
  className?: string
  style?: React.CSSProperties
}

export function CardHeader({ children, icon, right, size = 'md', className = '', style }: CardHeaderProps) {
  if (size === 'sm') {
    return (
      <div
        className={['flex items-center justify-between', 'px-[14px] py-2.5', 'border-b', 'text-[0.76em] font-semibold uppercase tracking-[0.5px]', className].filter(Boolean).join(' ')}
        style={{ borderColor: 'var(--border)', ...style }}
      >
        <span>{children}</span>
        {right}
      </div>
    )
  }
  return (
    <div
      className={['flex items-center gap-2.5', 'px-[18px] py-3.5', 'border-b', className].filter(Boolean).join(' ')}
      style={{ borderColor: 'var(--border)', ...style }}
    >
      {icon && <span className="shrink-0" style={{ color: 'var(--fg2)' }}>{icon}</span>}
      <h2 className="text-[0.95em] font-semibold" style={{ color: 'var(--fg1)', margin: 0 }}>{children}</h2>
      {right && <span className="ml-auto">{right}</span>}
    </div>
  )
}
