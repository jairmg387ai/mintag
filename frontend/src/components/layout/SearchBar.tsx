import { useEffect, useRef, useState } from 'react'
import { Search } from 'lucide-react'
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
    <div ref={wrapperRef} className="tb-search">
      <span className="mt-icon"><Search size={16} strokeWidth={1.75} /></span>
      <input
        type="text"
        placeholder="Search tasks & meetings…"
        value={query}
        onChange={e => setQuery(e.target.value)}
      />
      {loading && (
        <span style={{ position: 'absolute', right: 10, top: '50%', transform: 'translateY(-50%)', fontSize: '0.7em', color: 'var(--fg3)' }}>…</span>
      )}
      {open && (
        <div style={{
          position: 'absolute',
          top: 'calc(100% + 6px)',
          right: 0,
          minWidth: 320,
          background: 'var(--bg-surface)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--radius-lg)',
          boxShadow: 'var(--shadow-lg)',
          zIndex: 20,
          maxHeight: 360,
          overflowY: 'auto',
        }}>
          {results.length === 0 ? (
            <div style={{ padding: '12px 16px', fontSize: '0.82em', color: 'var(--fg3)' }}>
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
      <div style={{ fontSize: '0.72em', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.5px', color: 'var(--fg3)', padding: '8px 16px 4px' }}>
        {label}
      </div>
      {items.map(r => (
        <div
          key={`${r.kind}-${r.id}`}
          onClick={() => onSelect(r)}
          style={{
            padding: '8px 16px',
            cursor: 'pointer',
            borderTop: '1px solid var(--border)',
          }}
          onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-hover)')}
          onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
        >
          <div style={{ fontSize: '0.85em', fontWeight: 500, color: 'var(--fg1)', marginBottom: 2 }}>{r.title}</div>
          {r.snippet && (
            <div
              style={{ fontSize: '0.78em', color: 'var(--fg3)', lineHeight: 1.5 }}
              dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(r.snippet, { ALLOWED_TAGS: ['mark'] }) }}
            />
          )}
        </div>
      ))}
    </div>
  )
}
