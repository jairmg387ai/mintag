import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
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

function renderModal(opts: { me?: string } = {}) {
  const onCreated = vi.fn()
  const onClose = vi.fn()
  render(
    <CreateBugCorrectionTaskModal
      open
      bugId={171191}
      onClose={onClose}
      onCreated={onCreated}
      catalog={catalog}
      currentUserDisplayName={opts.me}
    />,
  )
  return { onCreated, onClose }
}

async function fillRequired(user: ReturnType<typeof userEvent.setup>) {
  await user.selectOptions(await screen.findByRole('combobox', { name: 'Subárea *' }), 'Desarrollo')
  await user.type(screen.getByLabelText('Estimado en horas *'), '6')
  await user.selectOptions(screen.getByRole('combobox', { name: 'Iteración *' }), 'ControlesDeCambio\\Sprint 7 Semana 41')
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
    await user.selectOptions(screen.getByRole('combobox', { name: 'Iteración *' }), 'ControlesDeCambio\\Sprint 7 Semana 41')
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
})
