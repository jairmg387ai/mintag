import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { fetchBugEvidence, patchBugEvidence, listBugComments, addBugComment } from '../../api/client'
import type { BugEvidence } from '../../types'
import { BugEvidencePanel } from './BugEvidencePanel'

vi.mock('../../api/client', () => ({
  fetchBugEvidence: vi.fn(),
  patchBugEvidence: vi.fn(),
  // BugEvidencePanel renders BugCommentTimeline (C12), which calls these —
  // mocked here so this file's tests (which only exercise the evidence
  // fields) don't depend on the comment timeline's own behavior.
  listBugComments: vi.fn(),
  addBugComment: vi.fn(),
}))

function buildEvidence(overrides: Partial<BugEvidence> = {}): BugEvidence {
  return {
    id: 170277,
    rev: 5,
    state: 'Cerrado',
    team_project: 'ControlesDeCambio',
    title: 'Some bug',
    editable: false,
    fields: {
      causa_raiz: 'Root cause text',
      causa_raiz_identificada: true,
      solucion_definitiva: 'Fix text',
      tipo_solucion: 'definitiva',
    },
    ...overrides,
  }
}

describe('BugEvidencePanel', () => {
  beforeEach(() => {
    vi.mocked(fetchBugEvidence).mockReset()
    vi.mocked(patchBugEvidence).mockReset()
    vi.mocked(listBugComments).mockReset().mockResolvedValue([])
    vi.mocked(addBugComment).mockReset()
  })

  it('renders read-only when the bug is not editable (no editable inputs in the DOM)', async () => {
    vi.mocked(fetchBugEvidence).mockResolvedValue(buildEvidence({ editable: false }))

    render(<BugEvidencePanel bugId={170277} />)

    await waitFor(() => expect(screen.getByText('Root cause text')).toBeInTheDocument())

    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('radio')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /guardar/i })).not.toBeInTheDocument()
  })

  it('blocks checking "causa raíz identificada" while causa raíz is empty (root-cause-identified invariant)', async () => {
    vi.mocked(fetchBugEvidence).mockResolvedValue(
      buildEvidence({
        editable: true,
        fields: {
          causa_raiz: '',
          causa_raiz_identificada: false,
          solucion_definitiva: '',
          tipo_solucion: '',
        },
      }),
    )

    render(<BugEvidencePanel bugId={170277} />)

    const checkbox = await screen.findByRole('checkbox', { name: /causa raíz identificada/i })

    expect(checkbox).toBeDisabled()
  })

  it('enables "causa raíz identificada" once causa raíz has non-empty content', async () => {
    vi.mocked(fetchBugEvidence).mockResolvedValue(
      buildEvidence({
        editable: true,
        fields: {
          causa_raiz: 'Root cause found in logs',
          causa_raiz_identificada: false,
          solucion_definitiva: '',
          tipo_solucion: '',
        },
      }),
    )

    render(<BugEvidencePanel bugId={170277} />)

    const checkbox = await screen.findByRole('checkbox', { name: /causa raíz identificada/i })

    expect(checkbox).toBeEnabled()
  })
})
