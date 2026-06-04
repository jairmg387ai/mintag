import React from 'react'

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  prefix?: React.ReactNode
}

export function Input({ prefix, className = '', ...props }: InputProps) {
  const base = 'bg-surface2 border border-border rounded-[7px] text-text text-[0.88em] outline-none w-full transition-all duration-150 focus:border-blue focus:shadow-[0_0_0_2px_rgba(59,130,246,0.15)]'

  if (prefix) {
    return (
      <div className="relative">
        <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text3 pointer-events-none flex">
          {prefix}
        </span>
        <input className={`${base} pl-8 pr-3 py-2 ${className}`} {...props} />
      </div>
    )
  }

  return <input className={`${base} px-3 py-2 ${className}`} {...props} />
}
