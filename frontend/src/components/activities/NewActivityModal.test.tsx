import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ActivityCatalog, ActivityValidationSettings, AzureActivity } from '../../types'
import { createActivity, getActivityValidationSettings } from '../../api/client'
import { NewActivityModal } from './NewActivityModal'

vi.mock('../../api/client', () => ({
  createActivity: vi.fn(),
  getActivityValidationSettings: vi.fn(),
}))

const ALL_VALIDATIONS_OFF: ActivityValidationSettings = {
  max_hours_per_entry: false,
  weekend_confirm: false,
  block_closed_work_item: false,
}

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

function renderModal(defaultDate = '2026-08-19') {
  const onCreated = vi.fn()
  const onClose = vi.fn()
  render(
    <NewActivityModal
      open
      onClose={onClose}
      onCreated={onCreated}
      catalog={catalog}
      defaultDate={defaultDate}
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
    vi.mocked(getActivityValidationSettings).mockReset()
    vi.mocked(getActivityValidationSettings).mockResolvedValue(ALL_VALIDATIONS_OFF)
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
    vi.mocked(getActivityValidationSettings).mockReset()
    vi.mocked(getActivityValidationSettings).mockResolvedValue(ALL_VALIDATIONS_OFF)
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

describe('NewActivityModal configurable validations', () => {
  beforeEach(() => {
    vi.mocked(createActivity).mockReset()
    vi.mocked(createActivity).mockResolvedValue({} as never)
    vi.mocked(getActivityValidationSettings).mockReset()
  })

  it('blocks hours over 8 client-side when max_hours_per_entry is enabled', async () => {
    vi.mocked(getActivityValidationSettings).mockResolvedValue({ ...ALL_VALIDATIONS_OFF, max_hours_per_entry: true })
    const user = userEvent.setup()
    renderModal()
    await waitFor(() => expect(getActivityValidationSettings).toHaveBeenCalled())

    await user.type(screen.getByPlaceholderText('0.00'), '9')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Proyecto *' }), 'Project A')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Categoría *' }), 'Development')
    await user.type(screen.getByPlaceholderText('Descripción de la actividad...'), 'Trabajo de prueba')
    await user.click(screen.getByRole('button', { name: 'Crear actividad' }))

    await waitFor(() => expect(screen.getByText('No se permiten más de 8 horas en un solo registro')).toBeInTheDocument())
    expect(createActivity).not.toHaveBeenCalled()
  })

  it('allows hours over 8 through to the API when max_hours_per_entry is disabled', async () => {
    vi.mocked(getActivityValidationSettings).mockResolvedValue(ALL_VALIDATIONS_OFF)
    const user = userEvent.setup()
    renderModal()
    await waitFor(() => expect(getActivityValidationSettings).toHaveBeenCalled())

    await user.type(screen.getByPlaceholderText('0.00'), '9')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Proyecto *' }), 'Project A')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Categoría *' }), 'Development')
    await user.type(screen.getByPlaceholderText('Descripción de la actividad...'), 'Trabajo de prueba')
    await user.click(screen.getByRole('button', { name: 'Crear actividad' }))

    await waitFor(() => expect(createActivity).toHaveBeenCalledWith(expect.objectContaining({ hours: 9 })))
  })

  it('confirms before submitting on a weekend date when weekend_confirm is enabled, and aborts on cancel', async () => {
    vi.mocked(getActivityValidationSettings).mockResolvedValue({ ...ALL_VALIDATIONS_OFF, weekend_confirm: true })
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const user = userEvent.setup()
    renderModal('2026-08-22') // Saturday
    await waitFor(() => expect(getActivityValidationSettings).toHaveBeenCalled())

    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: 'Crear actividad' }))

    await waitFor(() => expect(confirmSpy).toHaveBeenCalled())
    expect(createActivity).not.toHaveBeenCalled()
    confirmSpy.mockRestore()
  })

  it('proceeds after confirming a weekend date', async () => {
    vi.mocked(getActivityValidationSettings).mockResolvedValue({ ...ALL_VALIDATIONS_OFF, weekend_confirm: true })
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const user = userEvent.setup()
    renderModal('2026-08-23') // Sunday
    await waitFor(() => expect(getActivityValidationSettings).toHaveBeenCalled())

    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: 'Crear actividad' }))

    await waitFor(() => expect(createActivity).toHaveBeenCalled())
    confirmSpy.mockRestore()
  })

  it('never prompts for a weekday date, regardless of weekend_confirm', async () => {
    vi.mocked(getActivityValidationSettings).mockResolvedValue({ ...ALL_VALIDATIONS_OFF, weekend_confirm: true })
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const user = userEvent.setup()
    renderModal('2026-08-19') // Wednesday
    await waitFor(() => expect(getActivityValidationSettings).toHaveBeenCalled())

    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: 'Crear actividad' }))

    await waitFor(() => expect(createActivity).toHaveBeenCalled())
    expect(confirmSpy).not.toHaveBeenCalled()
    confirmSpy.mockRestore()
  })
})
