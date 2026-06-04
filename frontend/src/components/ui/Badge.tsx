import React from 'react'

interface BadgeProps {
  children: React.ReactNode
  className?: string
}

export function Badge({ children, className = '' }: BadgeProps) {
  return (
    <span className={['chip', className].filter(Boolean).join(' ')}>
      {children}
    </span>
  )
}
