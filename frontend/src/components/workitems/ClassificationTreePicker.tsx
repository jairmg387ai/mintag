import { useEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react'
import { fetchClassificationTree } from '../../api/client'
import { flattenClassificationTree, filterClassificationPaths } from './classificationTree'

interface ClassificationTreePickerProps {
  kind: 'areas' | 'iterations'
  ariaLabel: string
  value: string
  onChange: (path: string) => void
  inputStyle: CSSProperties
}

// A single reusable Area/Iteration path picker (used once per kind). Design
// choice vs. the coworker reference app: instead of a custom expand/collapse
// DOM tree, the fetched tree is flattened once into full paths and reused
// through the same accessible filter+keyboard-listbox pattern already
// implemented by AzureActivityCombobox, so mintag ends up with one combobox
// UI instead of two.
export function ClassificationTreePicker({ kind, ariaLabel, value, onChange, inputStyle }: ClassificationTreePickerProps) {
  const [allPaths, setAllPaths] = useState<string[]>([])
  const [loadError, setLoadError] = useState('')
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState(0)

  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLUListElement>(null)

  const listboxId = `classification-tree-listbox-${kind}`

  useEffect(() => {
    let cancelled = false
    fetchClassificationTree(kind)
      .then(tree => {
        if (!cancelled) setAllPaths(flattenClassificationTree(tree))
      })
      .catch((e: unknown) => {
        if (!cancelled) setLoadError(e instanceof Error ? e.message : 'No se pudo cargar el árbol')
      })
    return () => { cancelled = true }
  }, [kind])

  const matches = useMemo(() => filterClassificationPaths(allPaths, query), [allPaths, query])
  const safeIndex = Math.min(activeIndex, matches.length - 1)

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
    setActiveIndex(Math.max(matches.indexOf(value), 0))
  }

  function commit(path: string) {
    onChange(path)
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
      setActiveIndex(i => Math.min(i + 1, matches.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActiveIndex(i => Math.max(i - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const path = matches[safeIndex]
      if (path) commit(path)
    } else if (e.key === 'Escape') {
      e.preventDefault()
      setOpen(false)
      setQuery('')
    }
  }

  const activeOptionId = open && matches[safeIndex] ? `${listboxId}-option-${safeIndex}` : undefined

  return (
    <div ref={containerRef} style={{ position: 'relative' }}>
      <input
        ref={inputRef}
        role="combobox"
        aria-label={ariaLabel}
        aria-expanded={open}
        aria-controls={listboxId}
        aria-autocomplete="list"
        aria-activedescendant={activeOptionId}
        autoComplete="off"
        type="text"
        value={open ? query : value}
        placeholder={open ? value || 'Buscar...' : 'Seleccionar...'}
        onFocus={openList}
        onClick={openList}
        onChange={e => {
          setQuery(e.target.value)
          setActiveIndex(0)
        }}
        onKeyDown={handleKeyDown}
        style={inputStyle}
      />
      {loadError && (
        <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)', marginTop: 4 }}>
          {loadError}
        </div>
      )}
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
          {matches.map((path, index) => (
            <li
              key={path}
              id={`${listboxId}-option-${index}`}
              role="option"
              aria-selected={path === value}
              data-index={index}
              onMouseDown={e => {
                e.preventDefault()
                commit(path)
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
              {path}
            </li>
          ))}
          {matches.length === 0 && (
            <li style={{ padding: '6px 10px', color: 'var(--fg3)', font: 'var(--text-sm)' }}>
              Sin coincidencias
            </li>
          )}
        </ul>
      )}
    </div>
  )
}
