import React from 'react'
import { SearchBar } from './SearchBar'

interface TopBarProps {
  title: string
  children?: React.ReactNode
}

export function TopBar({ title, children }: TopBarProps) {
  return (
    <header className="topbar">
      <div className="crumb">{title}</div>
      <SearchBar />
      {children}
    </header>
  )
}
