import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { BugEvidence, BugEvidenceFields } from '../../types'
import { BugConflictModal } from './BugConflictModal'

function buildRemote(overrides: Partial<BugEvidence> = {}): BugEvidence {
  return {
    id: 170277,
    rev: 9,
    state: 'Activo',
    team_project: 'ControlesDeCambio',
    title: 'Some bug',
    editable: true,
    fields: {
      causa_raiz: 'Azure root cause',
      causa_raiz_identificada: true,
      solucion_definitiva: 'Azure fix',
      tipo_solucion: 'definitiva',
    },
    ...overrides,
  }
}

function buildDraft(overrides: Partial<BugEvidenceFields> = {}): BugEvidenceFields {
  return {
    causa_raiz: 'Azure root cause',
    causa_raiz_identificada: true,
    solucion_definitiva: 'Azure fix',
    tipo_solucion: 'definitiva',
    ...overrides,
  }
}

describe('BugConflictModal', () => {
  it('shows only the fields that diverged between the draft and remote, not all four unconditionally', () => {
    const draft = buildDraft({ causa_raiz: 'My root cause' })
    const remote = buildRemote()

    render(<BugConflictModal draft={draft} remote={remote} onResolve={vi.fn()} onCancel={vi.fn()} />)

    expect(screen.getByRole('radiogroup', { name: 'Resolver Causa raíz' })).toBeInTheDocument()
    expect(screen.queryByRole('radiogroup', { name: 'Resolver Solución definitiva' })).not.toBeInTheDocument()
    expect(screen.queryByRole('radiogroup', { name: 'Resolver Tipo de solución' })).not.toBeInTheDocument()
    expect(screen.queryByRole('radiogroup', { name: 'Resolver Causa raíz identificada' })).not.toBeInTheDocument()
  })

  it('treats tipo_solucion as ONE atomic field — exactly one resolution group, never two', () => {
    const draft = buildDraft({ tipo_solucion: 'temporal' })
    const remote = buildRemote({ fields: { ...buildDraft(), tipo_solucion: 'definitiva' } })

    render(<BugConflictModal draft={draft} remote={remote} onResolve={vi.fn()} onCancel={vi.fn()} />)

    expect(screen.getAllByRole('radiogroup')).toHaveLength(1)
    const group = screen.getByRole('radiogroup', { name: 'Resolver Tipo de solución' })
    expect(within(group).getAllByRole('radio')).toHaveLength(2)
  })

  it('renders sanitized read-only HTML panes for HTML fields, with no editable controls', () => {
    const draft = buildDraft({ causa_raiz: '<b>mine</b><script>window.__hit = true</script>' })
    const remote = buildRemote({ fields: { ...buildDraft(), causa_raiz: '<i>azure text</i>' } })

    render(<BugConflictModal draft={draft} remote={remote} onResolve={vi.fn()} onCancel={vi.fn()} />)

    expect(screen.getByText('mine')).toBeInTheDocument()
    expect(screen.getByText('azure text')).toBeInTheDocument()
    expect(document.querySelector('script')).not.toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^guardar$/i })).not.toBeInTheDocument()
  })

  it('defaults every divergent field to "keep mine" and calls onResolve with the merged draft after choosing take-Azure on one field', async () => {
    const draft = buildDraft({ causa_raiz: 'My root cause', solucion_definitiva: 'My fix' })
    const remote = buildRemote()
    const onResolve = vi.fn()
    const user = userEvent.setup()

    render(<BugConflictModal draft={draft} remote={remote} onResolve={onResolve} onCancel={vi.fn()} />)

    const causaRaizGroup = screen.getByRole('radiogroup', { name: 'Resolver Causa raíz' })
    await user.click(within(causaRaizGroup).getByRole('radio', { name: /usar de azure/i }))

    await user.click(screen.getByRole('button', { name: /resolver/i }))

    expect(onResolve).toHaveBeenCalledWith({
      causa_raiz: 'Azure root cause',
      causa_raiz_identificada: true,
      solucion_definitiva: 'My fix',
      tipo_solucion: 'definitiva',
    })
  })

  it('calls onCancel without calling onResolve when cancelled', async () => {
    const draft = buildDraft({ causa_raiz: 'My root cause' })
    const remote = buildRemote()
    const onResolve = vi.fn()
    const onCancel = vi.fn()
    const user = userEvent.setup()

    render(<BugConflictModal draft={draft} remote={remote} onResolve={onResolve} onCancel={onCancel} />)

    await user.click(screen.getByRole('button', { name: /cancelar/i }))

    expect(onCancel).toHaveBeenCalled()
    expect(onResolve).not.toHaveBeenCalled()
  })
})
