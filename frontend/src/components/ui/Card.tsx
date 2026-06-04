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
      className={`bg-surface border border-border rounded-[10px] overflow-hidden ${padding ? 'p-5' : ''} ${className}`}
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
        className={`px-3.5 py-2.5 border-b border-border flex items-center justify-between text-[0.76em] font-semibold uppercase tracking-[0.5px] ${className}`}
        style={style}
      >
        <span>{children}</span>
        {right}
      </div>
    )
  }
  return (
    <div
      className={`px-[18px] py-3.5 border-b border-border flex items-center gap-2.5 ${className}`}
      style={style}
    >
      {icon && <span className="shrink-0 text-text2">{icon}</span>}
      <h2 className="text-[0.95em] font-semibold">{children}</h2>
    </div>
  )
}
