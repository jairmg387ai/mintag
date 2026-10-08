import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  createBugCorrectionTask,
  fetchClassificationTree,
  fetchSubareaAllowedValues,
  getBugCorrectionTaskDraft,
} from '../../api/client'
import { CreateBugCorrectionTaskModal } from './CreateBugCorrectionTaskModal'
import type { BugCorrectionTaskDraft } from '../../types'

vi.mock('../../api/client', () => ({
  BugEvidenceApiError: class BugEvidenceApiError extends Error {},
  createBugCorrectionTask: vi.fn(),
  fetchClassificationTree: vi.fn(),
  fetchSubareaAllowedValues: vi.fn(),
  getBugCorrectionTaskDraft: vi.fn(),
}))

const draft: BugCorrectionTaskDraft = {
  org: 'RUNT2PSW',
  bug_id: 171191,
  bug_title: 'Error al radicar',
  bug_state: 'Activo',
  team_project: 'ControlesDeCambio',
  area_path: 'ControlesDeCambio\\RNET',
  iteration_path: 'ControlesDeCambio\\Sprint 6',
  assigned_to: { id: 'dev-id', display_name: 'Dev Uno', unique_name: 'dev@runt.com.co' },
  suggested_title: 'Atención y/o Corrección del defecto 171191',
  existing_correction_tasks: [],
}

const areaTree = { name: 'ControlesDeCambio', children: [{ name: 'RNET' }, { name: 'QA' }] }
const iterationTree = { name: 'ControlesDeCambio', children: [{ name: 'Sprint 7 Semana 41' }] }

const catalog = {
  projects: [{ name: 'Mintag', is_active: true }],
  categories: [{ id: 7, name: 'Desarrollo', is_active: true }],
}

interface ModalOpts {
  me?: string
  bugId?: number
}

function renderModal(opts: ModalOpts = {}) {
  const onCreated = vi.fn()
  const onClose = vi.fn()
  const element = (o: ModalOpts) => (
    <CreateBugCorrectionTaskModal
      open
      bugId={o.bugId ?? 171191}
      onClose={onClose}
      onCreated={onCreated}
      catalog={catalog}
      currentUserDisplayName={o.me}
    />
  )
  const { rerender } = render(element(opts))
  const rerenderWith = (next: ModalOpts) => rerender(element({ ...opts, ...next }))
  return { onCreated, onClose, rerenderWith }
}

async function pickPath(user: ReturnType<typeof userEvent.setup>, comboboxName: string, optionName: string) {
  await user.click(screen.getByRole('combobox', { name: comboboxName }))
  await user.click(await screen.findByRole('option', { name: optionName }))
}

async function fillRequired(user: ReturnType<typeof userEvent.setup>) {
  const subarea = await screen.findByRole('combobox', { name: 'Subárea *' })
  await within(subarea).findByRole('option', { name: 'Desarrollo' })
  await user.selectOptions(subarea, 'Desarrollo')
  await user.type(screen.getByLabelText('Estimado en horas *'), '6')
  await pickPath(user, 'Iteración *', 'ControlesDeCambio\\Sprint 7 Semana 41')
}

describe('CreateBugCorrectionTaskModal', () => {
  beforeEach(() => {
    vi.mocked(getBugCorrectionTaskDraft).mockReset().mockResolvedValue(draft)
    vi.mocked(fetchSubareaAllowedValues).mockReset().mockResolvedValue(['Arquitectura', 'Desarrollo', 'QA'])
    vi.mocked(fetchClassificationTree).mockReset().mockImplementation(kind =>
      Promise.resolve(kind === 'areas' ? areaTree : iterationTree),
    )
    vi.mocked(createBugCorrectionTask).mockReset().mockResolvedValue({ id: 171400, state: 'Proposed', azure_activity_id: 3 })
  })

  it('prefills title, area and assignee from the bug draft and loads trees for the bug team project', async () => {
    renderModal()

    expect(await screen.findByDisplayValue('Atención y/o Corrección del defecto 171191')).toBeInTheDocument()
    expect(screen.getByText(/#171191 — Error al radicar/)).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Área' })).toHaveValue('ControlesDeCambio\\RNET')
    expect(screen.getByLabelText('Asignado a *')).toHaveValue('dev@runt.com.co')
    await waitFor(() => expect(fetchClassificationTree).toHaveBeenCalledWith('iterations', 'ControlesDeCambio'))
    expect(fetchClassificationTree).toHaveBeenCalledWith('areas', 'ControlesDeCambio')
    expect(fetchSubareaAllowedValues).toHaveBeenCalledWith('ControlesDeCambio')
  })

  it('does not preselect a subárea and keeps submit disabled until required fields are valid', async () => {
    const user = userEvent.setup()
    renderModal()

    const subarea = await screen.findByRole('combobox', { name: 'Subárea *' })
    await screen.findByRole('option', { name: 'Desarrollo' })
    expect(subarea).toHaveValue('')
    expect(screen.getByRole('option', { name: 'Selecciona la subárea' })).toBeInTheDocument()

    const submit = screen.getByRole('button', { name: 'Crear tarea de corrección' })
    expect(submit).toBeDisabled()

    await user.selectOptions(subarea, 'Desarrollo')
    expect(submit).toBeDisabled()
    await user.type(screen.getByLabelText('Estimado en horas *'), '0')
    expect(submit).toBeDisabled()
    await user.clear(screen.getByLabelText('Estimado en horas *'))
    await user.type(screen.getByLabelText('Estimado en horas *'), '6')
    expect(submit).toBeDisabled()
    await pickPath(user, 'Iteración *', 'ControlesDeCambio\\Sprint 7 Semana 41')
    expect(submit).toBeEnabled()
  })

  it('warns about existing correction tasks but still allows creating', async () => {
    vi.mocked(getBugCorrectionTaskDraft).mockResolvedValue({
      ...draft,
      existing_correction_tasks: [{ id: 171306, title: 'Atención y/o Corrección del defecto 171306', state: 'Proposed' }],
    })
    const user = userEvent.setup()
    renderModal()

    expect(await screen.findByText(/ya tiene tareas de corrección/i)).toBeInTheDocument()
    expect(screen.getByText(/#171306/)).toBeInTheDocument()
    await fillRequired(user)
    expect(screen.getByRole('button', { name: 'Crear tarea de corrección' })).toBeEnabled()
  })

  it('submits the expected payload and reports the created task', async () => {
    const user = userEvent.setup()
    const { onCreated } = renderModal({ me: 'Dev Uno' })

    await fillRequired(user)
    // Catalog defaults to checked when the assignee is the current user.
    expect(screen.getByRole('checkbox', { name: 'Agregar al catálogo' })).toBeChecked()
    await user.selectOptions(screen.getByRole('combobox', { name: 'Proyecto' }), 'Mintag')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Categoría' }), 'Desarrollo')
    await user.type(screen.getByPlaceholderText('Descripción (opcional)...'), 'Corregir validación')
    await user.click(screen.getByRole('button', { name: 'Crear tarea de corrección' }))

    await waitFor(() => expect(createBugCorrectionTask).toHaveBeenCalledTimes(1))
    expect(createBugCorrectionTask).toHaveBeenCalledWith(171191, {
      title: 'Atención y/o Corrección del defecto 171191',
      description: 'Corregir validación',
      subarea: 'Desarrollo',
      original_estimate: 6,
      area_path: 'ControlesDeCambio\\RNET',
      iteration_path: 'ControlesDeCambio\\Sprint 7 Semana 41',
      assigned_to: 'dev@runt.com.co',
      add_to_catalog: true,
      project: 'Mintag',
      category_id: 7,
    })
    expect(onCreated).toHaveBeenCalledWith({ id: 171400, state: 'Proposed', azure_activity_id: 3 })
    expect(await screen.findByText(/Tarea #171400 creada/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /#171400/ })).toHaveAttribute(
      'href',
      'https://dev.azure.com/RUNT2PSW/_workitems/edit/171400',
    )
  })

  it('leaves the catalog unchecked when the assignee is not the current user', async () => {
    const user = userEvent.setup()
    renderModal({ me: 'Otra Persona' })

    await fillRequired(user)
    expect(screen.getByRole('checkbox', { name: 'Agregar al catálogo' })).not.toBeChecked()
    await user.click(screen.getByRole('button', { name: 'Crear tarea de corrección' }))
    await waitFor(() => expect(createBugCorrectionTask).toHaveBeenCalled())
    expect(vi.mocked(createBugCorrectionTask).mock.calls[0][1]).toMatchObject({ add_to_catalog: false })
    expect(vi.mocked(createBugCorrectionTask).mock.calls[0][1]).not.toHaveProperty('project')
  })

  it('uses tree pickers for iteration and area loaded from the bug team project', async () => {
    const user = userEvent.setup()
    renderModal()

    const area = await screen.findByRole('combobox', { name: 'Área' })
    expect(area).toHaveAttribute('aria-autocomplete', 'list')
    expect(area).toHaveValue('ControlesDeCambio\\RNET')
    await pickPath(user, 'Área', 'ControlesDeCambio\\QA')
    expect(screen.getByRole('combobox', { name: 'Área' })).toHaveValue('ControlesDeCambio\\QA')

    expect(screen.getByRole('combobox', { name: 'Iteración *' })).toHaveAttribute('aria-autocomplete', 'list')
    expect(fetchClassificationTree).toHaveBeenCalledWith('iterations', 'ControlesDeCambio')
    expect(fetchClassificationTree).toHaveBeenCalledWith('areas', 'ControlesDeCambio')
  })

  it('submits a custom title typed by the user', async () => {
    const user = userEvent.setup()
    renderModal()

    const title = await screen.findByDisplayValue('Atención y/o Corrección del defecto 171191')
    await user.clear(title)
    await user.type(title, 'Corrección manual del radicado')
    await fillRequired(user)
    await user.click(screen.getByRole('button', { name: 'Crear tarea de corrección' }))

    await waitFor(() => expect(createBugCorrectionTask).toHaveBeenCalledTimes(1))
    expect(vi.mocked(createBugCorrectionTask).mock.calls[0][1]).toMatchObject({ title: 'Corrección manual del radicado' })
  })

  it('keeps user edits when the connected identity arrives after opening', async () => {
    const user = userEvent.setup()
    const { rerenderWith } = renderModal()

    const title = await screen.findByDisplayValue('Atención y/o Corrección del defecto 171191')
    await user.clear(title)
    await user.type(title, 'Título propio')
    await pickPath(user, 'Área', 'ControlesDeCambio\\QA')
    await user.clear(screen.getByLabelText('Asignado a *'))
    await user.type(screen.getByLabelText('Asignado a *'), 'otro@runt.com.co')
    const checkbox = screen.getByRole('checkbox', { name: 'Agregar al catálogo' })
    expect(checkbox).not.toBeChecked()
    await user.click(checkbox)
    await user.click(checkbox)
    expect(checkbox).not.toBeChecked()

    rerenderWith({ me: 'Dev Uno' })

    expect(screen.getByLabelText('Título *')).toHaveValue('Título propio')
    expect(screen.getByRole('combobox', { name: 'Área' })).toHaveValue('ControlesDeCambio\\QA')
    expect(screen.getByLabelText('Asignado a *')).toHaveValue('otro@runt.com.co')
    // The user's explicit choice wins over the identity-derived default.
    expect(screen.getByRole('checkbox', { name: 'Agregar al catálogo' })).not.toBeChecked()

    await fillRequired(user)
    expect(screen.getByLabelText('Título *')).toHaveValue('Título propio')
    expect(getBugCorrectionTaskDraft).toHaveBeenCalledTimes(1)
    expect(fetchSubareaAllowedValues).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'Crear tarea de corrección' }))
    await waitFor(() => expect(createBugCorrectionTask).toHaveBeenCalledTimes(1))
    expect(vi.mocked(createBugCorrectionTask).mock.calls[0][1]).toMatchObject({
      title: 'Título propio',
      area_path: 'ControlesDeCambio\\QA',
      assigned_to: 'otro@runt.com.co',
      add_to_catalog: false,
    })
  })

  it('derives the catalog default from an identity that arrives late when the user has not toggled it', async () => {
    const { rerenderWith } = renderModal()
    await screen.findByDisplayValue('Atención y/o Corrección del defecto 171191')
    expect(screen.getByRole('checkbox', { name: 'Agregar al catálogo' })).not.toBeChecked()

    rerenderWith({ me: 'Dev Uno' })

    expect(screen.getByRole('checkbox', { name: 'Agregar al catálogo' })).toBeChecked()
    await waitFor(() => expect(fetchSubareaAllowedValues).toHaveBeenCalledTimes(1))
    expect(getBugCorrectionTaskDraft).toHaveBeenCalledTimes(1)
  })

  it('clears a previous load error once a draft loads successfully', async () => {
    vi.mocked(getBugCorrectionTaskDraft).mockRejectedValueOnce(new Error('bug no disponible'))
    const { rerenderWith } = renderModal({ bugId: 1 })
    expect(await screen.findByText('bug no disponible')).toBeInTheDocument()

    rerenderWith({ bugId: 171191 })

    expect(await screen.findByDisplayValue('Atención y/o Corrección del defecto 171191')).toBeInTheDocument()
    expect(screen.queryByText('bug no disponible')).not.toBeInTheDocument()
  })
})
