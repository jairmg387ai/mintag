import React from 'react'

interface BadgeProps {
  children: React.ReactNode
  className?: string
}

export function Badge({ children, className = '' }: BadgeProps) {
  return (
    <span className={`inline-flex items-center bg-surface2 ring-1 ring-border rounded-full px-2.5 py-0.5 text-[0.72em] text-text2 whitespace-nowrap ${className}`}>
      {children}
    </span>
  )
}
