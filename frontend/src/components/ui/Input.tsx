import React from 'react'

// Omit 'prefix' from HTMLInputElement attributes to avoid type conflict
// (native prefix is string | undefined, ours is ReactNode)
interface InputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'prefix'> {
  prefix?: React.ReactNode
}

export function Input({ prefix, className = '', ...props }: InputProps) {
  if (prefix) {
    return (
      <div className="tb-search" style={{ width: '100%', margin: 0 }}>
        <span className="mt-icon" style={{ pointerEvents: 'none' }}>
          {prefix}
        </span>
        <input className={className} {...props} />
      </div>
    )
  }

  return (
    <input
      className={className}
      style={{
        width: '100%',
        padding: '8px 12px',
        border: '1px solid var(--border-strong)',
        borderRadius: 'var(--radius-md)',
        font: 'var(--text-body)',
        color: 'var(--fg1)',
        background: 'var(--bg-sunken)',
        outline: 'none',
      }}
      {...props}
    />
  )
}
