import React from 'react'

interface TopBarProps {
  title: string
  children?: React.ReactNode
}

export function TopBar({ title, children }: TopBarProps) {
  return (
    <div style={{
      padding: '16px 28px',
      borderBottom: '1px solid var(--color-border)',
      display: 'flex',
      alignItems: 'center',
      gap: 12,
      background: 'var(--color-surface)',
      position: 'sticky',
      top: 0,
      zIndex: 10,
    }}>
      <h1 style={{ fontSize: '1.1em', fontWeight: 600 }}>{title}</h1>
      <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
        {children}
      </div>
    </div>
  )
}
