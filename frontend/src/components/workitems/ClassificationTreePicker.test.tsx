import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { fetchClassificationTree } from '../../api/client'
import { ClassificationTreePicker } from './ClassificationTreePicker'

vi.mock('../../api/client', () => ({
  fetchClassificationTree: vi.fn(),
}))

const trees: Record<string, { name: string; children: { name: string }[] }> = {
  ControlesDeCambio: { name: 'ControlesDeCambio', children: [{ name: 'RNET' }] },
  OtroProyecto: { name: 'OtroProyecto', children: [{ name: 'QA' }] },
  default: { name: 'RUNTPRO', children: [{ name: 'Core' }] },
}

function renderPicker(teamProject?: string) {
  const onChange = vi.fn()
  const utils = render(
    <ClassificationTreePicker kind="areas" ariaLabel="Área" value="" onChange={onChange} inputStyle={{}} teamProject={teamProject} />,
  )
  return { ...utils, onChange }
}

describe('ClassificationTreePicker', () => {
  beforeEach(() => {
    vi.mocked(fetchClassificationTree).mockReset().mockImplementation((_kind, teamProject) =>
      Promise.resolve(trees[teamProject ?? 'default']),
    )
  })

  it('loads the configured team project tree when no teamProject is given', async () => {
    renderPicker()
    await waitFor(() => expect(fetchClassificationTree).toHaveBeenCalledTimes(1))
    expect(vi.mocked(fetchClassificationTree).mock.calls[0]).toEqual(['areas'])
  })

  it('loads the tree for the given teamProject and reloads when it changes', async () => {
    const user = userEvent.setup()
    const { rerender, onChange } = renderPicker('ControlesDeCambio')
    await waitFor(() => expect(fetchClassificationTree).toHaveBeenCalledWith('areas', 'ControlesDeCambio'))

    await user.click(screen.getByRole('combobox', { name: 'Área' }))
    expect(await screen.findByRole('option', { name: 'ControlesDeCambio\\RNET' })).toBeInTheDocument()

    rerender(
      <ClassificationTreePicker kind="areas" ariaLabel="Área" value="" onChange={onChange} inputStyle={{}} teamProject="OtroProyecto" />,
    )
    await waitFor(() => expect(fetchClassificationTree).toHaveBeenCalledWith('areas', 'OtroProyecto'))
    expect(await screen.findByRole('option', { name: 'OtroProyecto\\QA' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'ControlesDeCambio\\RNET' })).not.toBeInTheDocument()
  })
})
