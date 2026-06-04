import React from 'react'
import type { LucideIcon } from 'lucide-react'

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
  size?: 'sm' | 'md'
  icon?: LucideIcon
  iconRight?: LucideIcon
}

export function Button({
  variant = 'secondary',
  size = 'md',
  icon: Icon,
  iconRight: IconRight,
  className = '',
  children,
  ...props
}: ButtonProps) {
  const variantClass = ['primary', 'secondary', 'ghost', 'danger'].includes(variant ?? '')
    ? `btn-${variant}`
    : 'btn-secondary'

  const sizeClass = size === 'sm' ? 'btn-sm' : ''

  const classes = ['btn', variantClass, sizeClass, className].filter(Boolean).join(' ')

  const iconSize = size === 'sm' ? 15 : 16

  return (
    <button className={classes} {...props}>
      {Icon && <Icon size={iconSize} strokeWidth={1.75} />}
      {children}
      {IconRight && <IconRight size={iconSize} strokeWidth={1.75} />}
    </button>
  )
}
