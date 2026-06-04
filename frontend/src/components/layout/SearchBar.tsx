import { useEffect, useRef, useState } from 'react'
import DOMPurify from 'dompurify'
import { search } from '../../api/client'
import { useAppActions } from '../../store/AppContext'
import type { SearchResult } from '../../types'

export function SearchBar() {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchResult[]>([])
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const wrapperRef = useRef<HTMLDivElement>(null)
  const { setEditingTaskId, setActiveMeetingId, openModal } = useAppActions()

  useEffect(() => {
    const trimmed = query.trim()
    if (!trimmed) {
      setResults([])
      setOpen(false)
      return
    }
    const timer = setTimeout(() => {
      setLoading(true)
      search(trimmed)
        .then(r => {
          setResults(Array.isArray(r) ? r : [])
          setOpen(true)
        })
        .catch(() => setResults([]))
        .finally(() => setLoading(false))
    }, 300)
    return () => clearTimeout(timer)
  }, [query])

  useEffect(() => {
    function handleOutside(e: MouseEvent) {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    function handleKeydown(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', handleOutside)
    document.addEventListener('keydown', handleKeydown)
    return () => {
      document.removeEventListener('mousedown', handleOutside)
      document.removeEventListener('keydown', handleKeydown)
    }
  }, [])

  function handleSelect(r: SearchResult) {
    setOpen(false)
    setQuery('')
    if (r.kind === 'meeting') {
      setActiveMeetingId(r.id)
      openModal('meeting')
    } else {
      setEditingTaskId(r.id)
      openModal('task')
    }
  }

  const meetings = results.filter(r => r.kind === 'meeting')
  const taskResults = results.filter(r => r.kind === 'task')

  return (
    <div ref={wrapperRef} style={{ position: 'relative' }}>
      <input
        type="text"
        placeholder="Search meetings & tasks…"
        value={query}
        onChange={e => setQuery(e.target.value)}
        style={{
          background: 'var(--color-surface2)',
          border: '1px solid var(--color-border)',
          borderRadius: 7,
          color: 'var(--color-text)',
          fontSize: '0.85em',
          padding: '6px 12px',
          width: 220,
          outline: 'none',
        }}
      />
      {loading && (
        <span style={{ position: 'absolute', right: 10, top: '50%', transform: 'translateY(-50%)', fontSize: '0.7em', color: 'var(--color-text3)' }}>…</span>
      )}
      {open && (
        <div style={{
          position: 'absolute',
          top: 'calc(100% + 6px)',
          right: 0,
          minWidth: 320,
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 8,
          boxShadow: '0 8px 24px rgba(0,0,0,0.3)',
          zIndex: 20,
          maxHeight: 360,
          overflowY: 'auto',
        }}>
          {results.length === 0 ? (
            <div style={{ padding: '12px 16px', fontSize: '0.82em', color: 'var(--color-text3)' }}>
              No results for "{query}"
            </div>
          ) : (
            <>
              {meetings.length > 0 && (
                <ResultGroup label="Meetings" items={meetings} onSelect={handleSelect} />
              )}
              {taskResults.length > 0 && (
                <ResultGroup label="Tasks" items={taskResults} onSelect={handleSelect} />
              )}
            </>
          )}
        </div>
      )}
    </div>
  )
}

function ResultGroup({ label, items, onSelect }: { label: string; items: SearchResult[]; onSelect: (r: SearchResult) => void }) {
  return (
    <div>
      <div style={{ fontSize: '0.72em', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.5px', color: 'var(--color-text3)', padding: '8px 16px 4px' }}>
        {label}
      </div>
      {items.map(r => (
        <div
          key={`${r.kind}-${r.id}`}
          onClick={() => onSelect(r)}
          style={{
            padding: '8px 16px',
            cursor: 'pointer',
            borderTop: '1px solid var(--color-border)',
          }}
          onMouseEnter={e => (e.currentTarget.style.background = 'var(--color-surface2)')}
          onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
        >
          <div style={{ fontSize: '0.85em', fontWeight: 500, color: 'var(--color-text)', marginBottom: 2 }}>{r.title}</div>
          {r.snippet && (
            <div
              style={{ fontSize: '0.78em', color: 'var(--color-text3)', lineHeight: 1.5 }}
              dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(r.snippet, { ALLOWED_TAGS: ['mark'] }) }}
            />
          )}
        </div>
      ))}
    </div>
  )
}
