import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ActivityCatalog, AzureActivity } from '../../types'
import { createActivity } from '../../api/client'
import { NewActivityModal } from './NewActivityModal'

vi.mock('../../api/client', () => ({
  createActivity: vi.fn(),
}))

function buildActivity(overrides: Partial<AzureActivity> = {}): AzureActivity {
  return {
    id: 1,
    org: 'my-org',
    work_item_id: 4521,
    label: 'Fix login bug',
    work_item_type: 'Bug',
    is_active: true,
    is_default: false,
    ...overrides,
  }
}

const azureActivities: AzureActivity[] = [
  buildActivity({ id: 1, label: 'Fix login bug', work_item_id: 4521, is_default: true }),
  buildActivity({ id: 2, label: 'Deploy pipeline', work_item_id: 9001 }),
  buildActivity({
    id: 3,
    label: 'Mapped work item',
    work_item_id: 7777,
    project: 'Project A',
    category_id: 1,
  }),
]

const catalog: ActivityCatalog = {
  projects: [{ name: 'Project A', is_active: true }],
  categories: [{ id: 1, name: 'Development' }],
}

function renderModal() {
  const onCreated = vi.fn()
  const onClose = vi.fn()
  render(
    <NewActivityModal
      open
      onClose={onClose}
      onCreated={onCreated}
      catalog={catalog}
      defaultDate="2026-08-19"
      azureActivities={azureActivities}
    />,
  )
  return { onCreated, onClose }
}

async function fillRequiredFields(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByPlaceholderText('0.00'), '2')
  // Project/category are left blank on open (not pre-seeded to the
  // catalog's first entry) so an unmapped Azure work item forces an
  // explicit choice — these tests' work items have no project/category
  // mapping, so both must be picked by hand to pass validation.
  await user.selectOptions(screen.getByRole('combobox', { name: 'Proyecto *' }), 'Project A')
  await user.selectOptions(screen.getByRole('combobox', { name: 'Categoría *' }), 'Development')
  await user.type(screen.getByPlaceholderText('Descripción de la actividad...'), 'Trabajo de prueba')
}

describe('NewActivityModal Azure activity picker', () => {
  beforeEach(() => {
    vi.mocked(createActivity).mockReset()
    vi.mocked(createActivity).mockResolvedValue({} as never)
  })

  it('filters the candidate list as the user types', async () => {
    const user = userEvent.setup()
    renderModal()
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })

    await user.click(input)
    await user.type(input, 'Deploy')

    const listbox = screen.getByRole('listbox')
    expect(within(listbox).getAllByRole('option')).toHaveLength(2) // pinned default + Deploy pipeline
    expect(within(listbox).getByRole('option', { name: /Deploy pipeline/ })).toBeInTheDocument()
    expect(within(listbox).queryByRole('option', { name: 'Fix login bug (#4521)' })).not.toBeInTheDocument()
  })

  it('submits the selected work item id', async () => {
    const user = userEvent.setup()
    renderModal()
    await fillRequiredFields(user)

    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })
    await user.click(input)
    await user.click(screen.getByRole('option', { name: /Deploy pipeline/ }))

    await user.click(screen.getByRole('button', { name: 'Crear actividad' }))

    expect(createActivity).toHaveBeenCalledWith(
      expect.objectContaining({ azure_activity_id: 2 }),
    )
  })

  it('omits azure_activity_id when the default option is used', async () => {
    const user = userEvent.setup()
    renderModal()
    await fillRequiredFields(user)

    await user.click(screen.getByRole('button', { name: 'Crear actividad' }))

    const call = vi.mocked(createActivity).mock.calls[0][0]
    expect(call).not.toHaveProperty('azure_activity_id')
  })
})

describe('NewActivityModal project/category autofill', () => {
  beforeEach(() => {
    vi.mocked(createActivity).mockReset()
    vi.mocked(createActivity).mockResolvedValue({} as never)
  })

  it('leaves project and category blank on open, blocking submission until a mapped work item is picked', async () => {
    const user = userEvent.setup()
    renderModal()

    expect(screen.getByRole('combobox', { name: 'Proyecto *' })).toHaveValue('')
    expect(screen.getByRole('combobox', { name: 'Categoría *' })).toHaveValue('')

    await user.type(screen.getByPlaceholderText('0.00'), '2')
    await user.type(screen.getByPlaceholderText('Descripción de la actividad...'), 'Trabajo de prueba')
    await user.click(screen.getByRole('button', { name: 'Crear actividad' }))

    expect(createActivity).not.toHaveBeenCalled()
    expect(screen.getByText('El proyecto es obligatorio')).toBeInTheDocument()
    expect(screen.getByText('La categoría es obligatoria')).toBeInTheDocument()
  })

  it('autofills project and category from a fully mapped work item, allowing submission without manual selection', async () => {
    const user = userEvent.setup()
    renderModal()

    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })
    await user.click(input)
    await user.click(screen.getByRole('option', { name: /Mapped work item/ }))

    expect(screen.getByRole('combobox', { name: 'Proyecto *' })).toHaveValue('Project A')
    expect(screen.getByRole('combobox', { name: 'Categoría *' })).toHaveValue('Development')

    await user.type(screen.getByPlaceholderText('0.00'), '2')
    await user.type(screen.getByPlaceholderText('Descripción de la actividad...'), 'Trabajo de prueba')
    await user.click(screen.getByRole('button', { name: 'Crear actividad' }))

    expect(createActivity).toHaveBeenCalledWith(
      expect.objectContaining({ project: 'Project A', category: 'Development' }),
    )
  })

  it('blanks a previously autofilled project/category when switching to a work item that maps neither', async () => {
    const user = userEvent.setup()
    renderModal()

    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })
    await user.click(input)
    await user.click(screen.getByRole('option', { name: /Mapped work item/ }))

    expect(screen.getByRole('combobox', { name: 'Proyecto *' })).toHaveValue('Project A')
    expect(screen.getByRole('combobox', { name: 'Categoría *' })).toHaveValue('Development')

    await user.click(input)
    // Exact match: the pinned "Usar predeterminada (...)" option embeds this
    // same default item's label as substring, so a partial/regex match here
    // would ambiguously hit both options.
    await user.click(screen.getByRole('option', { name: 'Fix login bug (#4521)' }))

    expect(screen.getByRole('combobox', { name: 'Proyecto *' })).toHaveValue('')
    expect(screen.getByRole('combobox', { name: 'Categoría *' })).toHaveValue('')
  })
})
