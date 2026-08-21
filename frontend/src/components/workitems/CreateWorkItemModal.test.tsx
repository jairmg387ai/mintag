import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createAzureWorkItem, fetchClassificationTree } from '../../api/client'
import { CreateWorkItemModal } from './CreateWorkItemModal'

vi.mock('../../api/client', () => ({
  createAzureWorkItem: vi.fn(),
  fetchClassificationTree: vi.fn(),
}))

const areaTree = { name: 'RUNTPRO', children: [{ name: 'RNET' }] }
const iterationTree = { name: 'RUNTPRO', children: [{ name: 'Sprint 1' }] }

const catalog = {
  projects: [{ name: 'Mintag', is_active: true }],
  categories: [{ id: 7, name: 'Desarrollo' }],
}

function renderModal(catalogProp: typeof catalog | null = null) {
  const onCreated = vi.fn()
  const onClose = vi.fn()
  render(<CreateWorkItemModal open onClose={onClose} onCreated={onCreated} catalog={catalogProp} />)
  return { onCreated, onClose }
}

async function pickPath(user: ReturnType<typeof userEvent.setup>, comboboxName: string, optionName: string) {
  const input = screen.getByRole('combobox', { name: comboboxName })
  await user.click(input)
  await user.click(await screen.findByRole('option', { name: optionName }))
}

describe('CreateWorkItemModal', () => {
  beforeEach(() => {
    vi.mocked(createAzureWorkItem).mockReset()
    vi.mocked(fetchClassificationTree).mockReset()
    vi.mocked(fetchClassificationTree).mockImplementation(kind =>
      Promise.resolve(kind === 'areas' ? areaTree : iterationTree),
    )
    vi.mocked(createAzureWorkItem).mockResolvedValue({ id: 4242, state: 'Active' })
  })

  it('requires title, area, and iteration before submitting', async () => {
    const user = userEvent.setup()
    renderModal()

    await user.click(screen.getByRole('button', { name: 'Crear work item' }))

    expect(screen.getByText('El título es obligatorio')).toBeInTheDocument()
    expect(screen.getByText('El área es obligatoria')).toBeInTheDocument()
    expect(screen.getByText('La iteración es obligatoria')).toBeInTheDocument()
    expect(createAzureWorkItem).not.toHaveBeenCalled()
  })

  it('submits the selected area/iteration paths and default estimate', async () => {
    const user = userEvent.setup()
    const { onCreated, onClose } = renderModal()

    await user.type(screen.getByPlaceholderText('Título del work item'), 'Fix the thing')
    await pickPath(user, 'Área *', 'RUNTPRO\\RNET')
    await pickPath(user, 'Iteración *', 'RUNTPRO\\Sprint 1')

    await user.click(screen.getByRole('button', { name: 'Crear work item' }))

    await waitFor(() => expect(createAzureWorkItem).toHaveBeenCalledWith({
      title: 'Fix the thing',
      area_path: 'RUNTPRO\\RNET',
      iteration_path: 'RUNTPRO\\Sprint 1',
      original_estimate: 24,
    }))
    expect(onCreated).toHaveBeenCalledWith({ id: 4242, state: 'Active' })
    expect(onClose).toHaveBeenCalled()
  })

  it('omits description when left blank and includes it when provided', async () => {
    const user = userEvent.setup()
    renderModal()

    await user.type(screen.getByPlaceholderText('Título del work item'), 'T')
    await user.type(screen.getByPlaceholderText('Descripción (opcional)...'), '  some notes  ')
    await pickPath(user, 'Área *', 'RUNTPRO\\RNET')
    await pickPath(user, 'Iteración *', 'RUNTPRO\\Sprint 1')

    await user.click(screen.getByRole('button', { name: 'Crear work item' }))

    await waitFor(() => expect(createAzureWorkItem).toHaveBeenCalledWith(
      expect.objectContaining({ description: 'some notes' }),
    ))
  })

  it('includes project and resolved category_id when both are selected from the catalog', async () => {
    const user = userEvent.setup()
    renderModal(catalog)

    await user.type(screen.getByPlaceholderText('Título del work item'), 'Fix the thing')
    await pickPath(user, 'Área *', 'RUNTPRO\\RNET')
    await pickPath(user, 'Iteración *', 'RUNTPRO\\Sprint 1')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Proyecto' }), 'Mintag')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Categoría' }), 'Desarrollo')

    await user.click(screen.getByRole('button', { name: 'Crear work item' }))

    await waitFor(() => expect(createAzureWorkItem).toHaveBeenCalledWith(
      expect.objectContaining({ project: 'Mintag', category_id: 7 }),
    ))
  })

  it('omits project and category_id entirely when left unselected', async () => {
    const user = userEvent.setup()
    renderModal(catalog)

    await user.type(screen.getByPlaceholderText('Título del work item'), 'Fix the thing')
    await pickPath(user, 'Área *', 'RUNTPRO\\RNET')
    await pickPath(user, 'Iteración *', 'RUNTPRO\\Sprint 1')

    await user.click(screen.getByRole('button', { name: 'Crear work item' }))

    await waitFor(() => expect(createAzureWorkItem).toHaveBeenCalled())
    const payload = vi.mocked(createAzureWorkItem).mock.calls[0][0]
    expect(payload).not.toHaveProperty('project')
    expect(payload).not.toHaveProperty('category_id')
  })

  it('shows the create error without closing the modal', async () => {
    vi.mocked(createAzureWorkItem).mockRejectedValue(new Error('area path is required'))
    const user = userEvent.setup()
    const { onClose } = renderModal()

    await user.type(screen.getByPlaceholderText('Título del work item'), 'T')
    await pickPath(user, 'Área *', 'RUNTPRO\\RNET')
    await pickPath(user, 'Iteración *', 'RUNTPRO\\Sprint 1')
    await user.click(screen.getByRole('button', { name: 'Crear work item' }))

    expect(await screen.findByText('area path is required')).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })
})
