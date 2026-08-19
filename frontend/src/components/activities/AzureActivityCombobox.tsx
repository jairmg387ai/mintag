import { useEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react'
import type { AzureActivity } from '../../types'
import { filterAzureActivities, findDefaultAzureActivity, formatAzureActivityLabel } from './azureActivity'

interface AzureActivityComboboxProps {
  azureActivities: AzureActivity[] // active-only (server already filters; see D4)
  value: string // '' = use default, else String(id)
  onChange: (value: string) => void // fires only on a committed selection
  inputStyle: CSSProperties
}

interface ComboOption {
  key: string
  value: string
  label: string
}

const DEFAULT_VALUE = ''

export function AzureActivityCombobox({ azureActivities, value, onChange, inputStyle }: AzureActivityComboboxProps) {
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState(0)

  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLUListElement>(null)

  const listboxId = 'azure-activity-combobox-listbox'

  const defaultActivity = findDefaultAzureActivity(azureActivities)
  const defaultLabel = `Usar predeterminada${defaultActivity ? ` (${formatAzureActivityLabel(defaultActivity)})` : ''}`
  const defaultOption: ComboOption = { key: 'default', value: DEFAULT_VALUE, label: defaultLabel }

  const matches = useMemo(() => filterAzureActivities(azureActivities, query), [azureActivities, query])

  const options: ComboOption[] = [
    defaultOption,
    ...matches.map(a => ({ key: String(a.id), value: String(a.id), label: formatAzureActivityLabel(a) })),
  ]

  const assignedActivity = value ? azureActivities.find(a => String(a.id) === value) : undefined
  const selectedLabel = value ? (assignedActivity ? formatAzureActivityLabel(assignedActivity) : `#${value}`) : defaultLabel

  const safeIndex = Math.min(activeIndex, options.length - 1)

  useEffect(() => {
    function handlePointerDown(e: MouseEvent) {
      if (!containerRef.current) return
      if (!containerRef.current.contains(e.target as Node)) {
        setOpen(false)
        setQuery('')
      }
    }
    document.addEventListener('mousedown', handlePointerDown)
    return () => document.removeEventListener('mousedown', handlePointerDown)
  }, [])

  useEffect(() => {
    if (!open || !listRef.current) return
    const activeEl = listRef.current.querySelector<HTMLElement>(`[data-index="${safeIndex}"]`)
    activeEl?.scrollIntoView?.({ block: 'nearest' })
  }, [open, safeIndex])

  function openList() {
    setOpen(true)
    setQuery('')
    const currentIndex = options.findIndex(o => o.value === value)
    setActiveIndex(currentIndex >= 0 ? currentIndex : 0)
  }

  function commit(v: string) {
    onChange(v)
    setOpen(false)
    setQuery('')
    inputRef.current?.focus()
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (!open) {
      if (e.key === 'ArrowDown' || e.key === 'Enter') {
        e.preventDefault()
        openList()
      }
      return
    }

    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActiveIndex(i => Math.min(i + 1, options.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActiveIndex(i => Math.max(i - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const opt = options[safeIndex]
      if (opt) commit(opt.value)
    } else if (e.key === 'Escape') {
      e.preventDefault()
      setOpen(false)
      setQuery('')
    }
  }

  const activeOptionId = open && options[safeIndex] ? `azure-activity-option-${options[safeIndex].key}` : undefined

  return (
    <div ref={containerRef} style={{ position: 'relative' }}>
      <input
        ref={inputRef}
        role="combobox"
        aria-label="Actividad de Azure"
        aria-expanded={open}
        aria-controls={listboxId}
        aria-autocomplete="list"
        aria-activedescendant={activeOptionId}
        autoComplete="off"
        type="text"
        value={open ? query : selectedLabel}
        placeholder={open ? selectedLabel : undefined}
        onFocus={openList}
        onClick={openList}
        onChange={e => {
          setQuery(e.target.value)
          setActiveIndex(0)
        }}
        onKeyDown={handleKeyDown}
        style={inputStyle}
      />
      {open && (
        <ul
          ref={listRef}
          role="listbox"
          id={listboxId}
          style={{
            position: 'absolute',
            top: '100%',
            left: 0,
            right: 0,
            zIndex: 10,
            margin: '4px 0 0',
            padding: 4,
            listStyle: 'none',
            maxHeight: 220,
            overflowY: 'auto',
            background: 'var(--bg-sunken)',
            border: '1px solid var(--border-strong)',
            borderRadius: 'var(--radius-md)',
            boxShadow: 'var(--shadow-xl)',
          }}
        >
          {options.map((opt, index) => (
            <li
              key={opt.key}
              id={`azure-activity-option-${opt.key}`}
              role="option"
              aria-selected={opt.value === value}
              data-index={index}
              onMouseDown={e => {
                e.preventDefault()
                commit(opt.value)
              }}
              onMouseEnter={() => setActiveIndex(index)}
              style={{
                padding: '6px 10px',
                borderRadius: 'var(--radius-sm)',
                cursor: 'pointer',
                background: index === safeIndex ? 'var(--bg-hover, rgba(255,255,255,0.08))' : 'transparent',
                color: 'var(--fg1)',
                font: 'var(--text-body)',
              }}
            >
              {opt.label}
            </li>
          ))}
          {matches.length === 0 && (
            <li
              style={{
                padding: '6px 10px',
                color: 'var(--fg3)',
                font: 'var(--text-sm)',
              }}
            >
              Sin coincidencias
            </li>
          )}
        </ul>
      )}
    </div>
  )
}
