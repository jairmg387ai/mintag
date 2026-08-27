import { describe, expect, it, vi, afterEach } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { listBugComments, addBugComment } from '../../api/client'
import type { BugComment } from '../../types'
import { BugCommentTimeline } from './BugCommentTimeline'

vi.mock('../../api/client', () => ({
  listBugComments: vi.fn(),
  addBugComment: vi.fn(),
}))

function buildComment(overrides: Partial<BugComment> = {}): BugComment {
  return {
    id: 1,
    text: 'Primer comentario',
    created_by: 'Ana',
    created_date: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

// Harness mirrors the shape of the invariant BugEvidencePanel relies on: an
// evidence-draft setter and a "conflict modal open" flag both live in a
// SIBLING component, entirely outside BugCommentTimeline's own state/props.
// BugCommentTimeline has no way to reach them — this test proves the poll
// never mutates them, which is the "silent refresh while a conflict modal is
// open" requirement made observable.
function Harness({ bugId }: { bugId: number }) {
  const [draftTouchCount, setDraftTouchCount] = useState(0)
  return (
    <div>
      <div data-testid="draft-touch-count">{draftTouchCount}</div>
      <button onClick={() => setDraftTouchCount(c => c + 1)}>mutate draft (never called by the poll)</button>
      <div data-testid="conflict-modal-open">Conflict modal is open</div>
      <BugCommentTimeline bugId={bugId} />
    </div>
  )
}

describe('BugCommentTimeline', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.mocked(listBugComments).mockReset()
    vi.mocked(addBugComment).mockReset()
  })

  it('polls and silently updates the timeline while a conflict modal is (simulated) open, without touching evidence draft state', async () => {
    vi.useFakeTimers()
    vi.mocked(listBugComments)
      .mockResolvedValueOnce([buildComment({ id: 1, text: 'Primer comentario' })])
      .mockResolvedValueOnce([
        buildComment({ id: 1, text: 'Primer comentario' }),
        buildComment({ id: 2, text: 'Comentario nuevo llegado por poll' }),
      ])

    render(<Harness bugId={170277} />)

    // Fake timers make RTL's own waitFor/findBy* hang (their internal polling
    // uses the same faked setTimeout) — flush the initial fetch's
    // microtasks directly instead.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(screen.getByText('Primer comentario')).toBeInTheDocument()
    expect(screen.getByTestId('conflict-modal-open')).toBeInTheDocument()
    expect(screen.getByTestId('draft-touch-count')).toHaveTextContent('0')

    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000)
    })

    expect(screen.getByText('Comentario nuevo llegado por poll')).toBeInTheDocument()
    // The sibling state the poll must never touch stays exactly as it was.
    expect(screen.getByTestId('conflict-modal-open')).toBeInTheDocument()
    expect(screen.getByTestId('draft-touch-count')).toHaveTextContent('0')
  })

  it('renders comment bodies via SafeHtmlNoImages (images stripped from rendered comments)', async () => {
    vi.mocked(listBugComments).mockResolvedValue([
      buildComment({ text: '<p>hola mundo</p><img src="x.png" alt="x" />' }),
    ])

    render(<BugCommentTimeline bugId={170277} />)

    await screen.findByText('hola mundo')
    expect(document.querySelector('img')).not.toBeInTheDocument()
  })

  it('posts a new comment with a generated idempotency key and clears the composer on success', async () => {
    vi.mocked(listBugComments).mockResolvedValue([])
    vi.mocked(addBugComment).mockResolvedValue(buildComment({ id: 9, text: 'Nuevo comentario' }))
    const user = userEvent.setup()

    render(<BugCommentTimeline bugId={170277} />)
    const textarea = await screen.findByRole('textbox', { name: /nuevo comentario/i })

    await user.type(textarea, 'Nuevo comentario')
    await user.click(screen.getByRole('button', { name: /enviar/i }))

    await waitFor(() => expect(addBugComment).toHaveBeenCalledTimes(1))
    const [calledBugId, idempotencyKey, text] = vi.mocked(addBugComment).mock.calls[0]
    expect(calledBugId).toBe(170277)
    expect(typeof idempotencyKey).toBe('string')
    expect(idempotencyKey.length).toBeGreaterThan(0)
    expect(text).toBe('Nuevo comentario')

    await waitFor(() => expect(textarea).toHaveValue(''))
    expect(await screen.findByText('Nuevo comentario')).toBeInTheDocument()
  })

  it('does not clear the composer or post twice when the submit is disabled while empty', async () => {
    vi.mocked(listBugComments).mockResolvedValue([])
    const user = userEvent.setup()

    render(<BugCommentTimeline bugId={170277} />)
    await screen.findByRole('textbox', { name: /nuevo comentario/i })

    expect(screen.getByRole('button', { name: /enviar/i })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: /enviar/i }))
    expect(addBugComment).not.toHaveBeenCalled()
  })

  it('hides the composer entirely (no textbox, no button) when editable=false, while still showing comments', async () => {
    vi.mocked(listBugComments).mockResolvedValue([buildComment({ text: 'Solo lectura' })])

    render(<BugCommentTimeline bugId={170277} editable={false} />)

    await screen.findByText('Solo lectura')
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /enviar/i })).not.toBeInTheDocument()
  })
})
