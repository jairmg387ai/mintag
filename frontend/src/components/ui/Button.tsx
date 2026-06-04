import React from 'react'

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'ghost'
  size?: 'sm' | 'md'
}

export function Button({ variant = 'ghost', size = 'md', className = '', children, ...props }: ButtonProps) {
  const base = 'inline-flex items-center gap-1.5 rounded-[7px] border-0 cursor-pointer font-medium transition-all duration-100 active:scale-[0.96] disabled:opacity-50 disabled:cursor-not-allowed'

  const variants = {
    primary: 'bg-blue text-white hover:bg-blue-dim',
    ghost:   'bg-surface2 text-text2 ring-1 ring-border hover:bg-surface3 hover:text-text',
  }

  const sizes = {
    sm: 'px-2 py-1 text-[0.75em]',
    md: 'px-2.5 py-[5px] text-[0.78em]',
  }

  return (
    <button className={`${base} ${variants[variant]} ${sizes[size]} ${className}`} {...props}>
      {children}
    </button>
  )
}
