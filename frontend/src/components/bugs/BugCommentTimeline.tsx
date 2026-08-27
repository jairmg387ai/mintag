import { useEffect, useState } from 'react'
import { listBugComments, addBugComment } from '../../api/client'
import type { BugComment } from '../../types'
import { SafeHtmlNoImages } from '../shared/SafeHtml'
import { Button } from '../ui/Button'

const POLL_INTERVAL_MS = 30_000

interface BugCommentTimelineProps {
  bugId: number
  // Hides the composer while the Bug is not in an editable state — mirrors
  // BugEvidencePanel's own read-only rendering, which asserts no editable
  // control (textbox/checkbox/radio/button) exists in the DOM when
  // editable=false. The timeline itself always keeps polling/rendering
  // regardless: viewing comments is never gated by state. The server
  // re-checks state_not_editable on POST independently; this is UX only.
  // Defaults to true so callers that don't pass it (and existing tests)
  // keep the composer visible.
  editable?: boolean
}

// BugCommentTimeline displays the Bug's Azure comment timeline and a
// composer to post new ones. It polls independently of everything else on
// the page (own state slice, own effect) — this component never receives,
// reads, or calls any evidence-draft setter, which is what makes the spec's
// "Comment Timeline Silent Refresh" requirement true by construction: this
// poll keeps running and silently updating the timeline even while
// BugEvidencePanel's BugConflictModal is open, because there is no code path
// here that could reach into that unrelated state.
export function BugCommentTimeline({ bugId, editable = true }: BugCommentTimelineProps) {
  const [comments, setComments] = useState<BugComment[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [draft, setDraft] = useState('')
  const [posting, setPosting] = useState(false)
  const [postError, setPostError] = useState('')

  useEffect(() => {
    let cancelled = false

    async function refresh(isInitial: boolean) {
      if (isInitial) setLoading(true)
      try {
        const list = await listBugComments(bugId)
        if (cancelled) return
        setComments(list)
        setLoadError('')
      } catch (e: unknown) {
        if (cancelled) return
        setLoadError(e instanceof Error ? e.message : 'No se pudo cargar la línea de tiempo de comentarios.')
      } finally {
        if (isInitial && !cancelled) setLoading(false)
      }
    }

    refresh(true)
    const interval = setInterval(() => refresh(false), POLL_INTERVAL_MS)

    return () => {
      cancelled = true
      clearInterval(interval)
    }
  }, [bugId])

  async function handlePost() {
    const text = draft.trim()
    if (!text) return
    setPosting(true)
    setPostError('')
    try {
      const idempotencyKey = crypto.randomUUID()
      const comment = await addBugComment(bugId, idempotencyKey, text)
      setComments(prev => [...prev, comment])
      setDraft('')
    } catch (e: unknown) {
      setPostError(e instanceof Error ? e.message : 'No se pudo publicar el comentario.')
    } finally {
      setPosting(false)
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div className="label">Comentarios</div>

      {loading ? (
        <div style={{ color: 'var(--fg3)', font: 'var(--text-body)' }}>Cargando comentarios...</div>
      ) : loadError ? (
        <div style={{ color: 'var(--block-solid)', font: 'var(--text-sm)' }}>{loadError}</div>
      ) : comments.length === 0 ? (
        <div style={{ color: 'var(--fg3)', font: 'var(--text-body)' }}>Todavía no hay comentarios.</div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {comments.map(c => (
            <div
              key={c.id}
              style={{
                padding: '8px 10px',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--border)',
                background: 'var(--bg-sunken)',
              }}
            >
              <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', marginBottom: 4 }}>
                {c.created_by} — {c.created_date}
              </div>
              <SafeHtmlNoImages html={c.text} />
            </div>
          ))}
        </div>
      )}

      {editable && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <textarea
            aria-label="Nuevo comentario"
            value={draft}
            onChange={e => setDraft(e.target.value)}
            placeholder="Agregar un comentario..."
            disabled={posting}
            style={{
              minHeight: 64,
              padding: '8px 12px',
              border: '1px solid var(--border-strong)',
              borderRadius: 'var(--radius-md)',
              font: 'var(--text-body)',
              color: 'var(--fg1)',
              background: 'var(--bg-sunken)',
              outline: 'none',
              resize: 'vertical',
            }}
          />
          {postError && <div style={{ color: 'var(--block-solid)', font: 'var(--text-sm)' }}>{postError}</div>}
          <div>
            <Button variant="secondary" onClick={handlePost} disabled={posting || draft.trim().length === 0}>
              {posting ? 'Enviando...' : 'Enviar'}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
