import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  getActivityCatalog,
  listAzureActivities,
  fetchAzureWorkItemStates,
  closeAzureWorkItem,
  recreateAzureWorkItem,
  listAssignedAzureWorkItems,
  addAzureActivity,
  fetchWorkItemEffort,
} from '../../api/client'
import { WorkItemsView } from './WorkItemsView'

vi.mock('../../api/client', () => ({
  getActivityCatalog: vi.fn(),
  listAzureActivities: vi.fn(),
  fetchAzureWorkItemStates: vi.fn(),
  closeAzureWorkItem: vi.fn(),
  recreateAzureWorkItem: vi.fn(),
  listAssignedAzureWorkItems: vi.fn(),
  addAzureActivity: vi.fn(),
  fetchWorkItemEffort: vi.fn(),
}))

// The correction-task modal has its own tests; here only the entry points
// matter, so it is stubbed to expose which bug it was opened for.
vi.mock('./CreateBugCorrectionTaskModal', () => ({
  CreateBugCorrectionTaskModal: ({ open, bugId }: { open: boolean; bugId: number | null }) =>
    open ? <div role="dialog" aria-label="Crear tarea de corrección">bug {bugId}</div> : null,
}))

const pushToast = vi.fn()
const openModal = vi.fn()
const setActiveBugEvidenceId = vi.fn()
const useAppState = vi.fn(() => ({ azureConfig: null as { user_display_name?: string } | null }))
vi.mock('../../store/AppContext', () => ({
  useAppActions: () => ({ pushToast, openModal, setActiveBugEvidenceId }),
  useAppState: () => useAppState(),
}))

const catalog = {
  projects: [{ name: 'Mintag', is_active: true }],
  categories: [{ id: 7, name: 'Desarrollo', is_active: true }],
}

const oneActivity = [
  {
    id: 1,
    org: 'ORG',
    work_item_id: 101,
    label: 'Fix login bug',
    work_item_type: 'Bug',
    is_active: true,
    is_default: true,
    project: 'Mintag',
    category_id: 7,
  },
]

describe('WorkItemsView', () => {
  beforeEach(() => {
    pushToast.mockReset()
    openModal.mockReset()
    setActiveBugEvidenceId.mockReset()
    vi.mocked(getActivityCatalog).mockReset()
    vi.mocked(listAzureActivities).mockReset()
    vi.mocked(fetchAzureWorkItemStates).mockReset()
    vi.mocked(closeAzureWorkItem).mockReset()
    vi.mocked(recreateAzureWorkItem).mockReset()
    vi.mocked(listAssignedAzureWorkItems).mockReset()
    vi.mocked(addAzureActivity).mockReset()
    vi.mocked(fetchWorkItemEffort).mockReset()
    vi.mocked(fetchWorkItemEffort).mockResolvedValue({ org: 'ORG', items: [] })
    vi.mocked(getActivityCatalog).mockResolvedValue(catalog)
    useAppState.mockReset().mockReturnValue({ azureConfig: null })
  })

  it('renders catalog rows from listAzureActivities', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)

    render(<WorkItemsView />)

    expect(await screen.findByText('101')).toBeInTheDocument()
    expect(screen.getByText('Fix login bug')).toBeInTheDocument()
    expect(screen.getByText('Mintag')).toBeInTheDocument()
    expect(screen.getByText('Desarrollo')).toBeInTheDocument()
  })

  it('renders the HORAS column from the effort of the current page work items', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)
    vi.mocked(fetchWorkItemEffort).mockResolvedValue({
      org: 'ORG',
      items: [{ id: 101, original_estimate: 24, uploaded_hours: 18, local_hours: 2.5, remaining: 3.5, has_estimate: true }],
    })

    render(<WorkItemsView />)

    const row = (await screen.findByText('Fix login bug')).closest('tr') as HTMLElement
    expect(screen.getByRole('columnheader', { name: 'HORAS' })).toBeInTheDocument()
    await waitFor(() =>
      expect(within(row).getByTestId('work-item-effort')).toHaveTextContent(
        'Estimado 24h · Registrado 18h (+2.5h sin subir) · Quedan 3.5h',
      ),
    )
    expect(fetchWorkItemEffort).toHaveBeenCalledWith([101])
  })

  it('shows a dash and a non-blocking notice when the effort call fails', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)
    vi.mocked(fetchWorkItemEffort).mockRejectedValue(new Error('azure not configured'))

    render(<WorkItemsView />)

    expect(await screen.findByText(/no se pudieron cargar las horas/i)).toBeInTheDocument()
    const row = screen.getByText('Fix login bug').closest('tr') as HTMLElement
    expect(within(row).queryByTestId('work-item-effort')).not.toBeInTheDocument()
    expect(screen.getByText('101')).toBeInTheDocument()
  })

  it('shows a notice when the TimeLog snapshot could not be read', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)
    vi.mocked(fetchWorkItemEffort).mockResolvedValue({
      org: 'ORG',
      items: [{ id: 101, original_estimate: 24, uploaded_hours: 0, local_hours: 0, remaining: 24, has_estimate: true }],
      timelog_error: 'timelog down',
    })

    render(<WorkItemsView />)

    expect(await screen.findByText(/timelog down/i)).toBeInTheDocument()
  })

  it('shows an empty state when the catalog has no entries', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([])

    render(<WorkItemsView />)

    expect(await screen.findByText(/no hay work items registrados/i)).toBeInTheDocument()
  })

  it('refreshes live state on demand and renders the resulting badges', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([
      ...oneActivity,
      { ...oneActivity[0], id: 2, work_item_id: 202, label: 'Add export button', work_item_type: 'Task', category_id: null, project: null, is_default: false },
    ])
    vi.mocked(fetchAzureWorkItemStates).mockResolvedValue({
      org: 'ORG',
      items: [
        { id: 101, title: 'Fix login bug', type: 'Bug', state: 'Active' },
        { id: 202, title: 'Add export button', type: 'Task', state: 'Closed' },
      ],
    })
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('button', { name: /refrescar estados/i }))

    expect(fetchAzureWorkItemStates).toHaveBeenCalledWith([101, 202])
    expect(await screen.findByText('Active')).toBeInTheDocument()
    expect(screen.getByText('Closed')).toBeInTheDocument()
  })

  it('shows a toast and keeps the table intact when the states fetch fails', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)
    vi.mocked(fetchAzureWorkItemStates).mockRejectedValue(new Error('azure: token is not configured'))
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('button', { name: /refrescar estados/i }))

    await waitFor(() => expect(pushToast).toHaveBeenCalledWith(expect.stringContaining('azure: token is not configured'), true))
    expect(screen.getByText('101')).toBeInTheDocument()
  })

  it('hides Close/Recreate actions for a Bug-type row but shows them for a Task-type row', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([
      { ...oneActivity[0], id: 1, work_item_id: 101, work_item_type: 'Bug' },
      { ...oneActivity[0], id: 2, work_item_id: 202, work_item_type: 'Task' },
    ])

    render(<WorkItemsView />)
    await screen.findByText('101')

    const rows = screen.getAllByRole('row')
    const bugRow = rows.find(r => r.textContent?.includes('101'))!
    const taskRow = rows.find(r => r.textContent?.includes('202'))!

    expect(within(bugRow).queryByRole('button', { name: /cerrar/i })).not.toBeInTheDocument()
    expect(within(bugRow).queryByRole('button', { name: /recrear/i })).not.toBeInTheDocument()
    expect(within(taskRow).getByRole('button', { name: /cerrar/i })).toBeInTheDocument()
    expect(within(taskRow).getByRole('button', { name: /recrear/i })).toBeInTheDocument()
  })

  it('shows the "Evidencia DSW-PR-017" action only on Bug rows, opening the panel in the shared Modal', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([
      { ...oneActivity[0], id: 1, work_item_id: 101, work_item_type: 'Bug' },
      { ...oneActivity[0], id: 2, work_item_id: 202, work_item_type: 'Task' },
    ])
    const user = userEvent.setup()

    render(<WorkItemsView />)
    await screen.findByText('101')

    const rows = screen.getAllByRole('row')
    const bugRow = rows.find(r => r.textContent?.includes('101'))!
    const taskRow = rows.find(r => r.textContent?.includes('202'))!

    expect(within(taskRow).queryByRole('button', { name: /evidencia dsw-pr-017/i })).not.toBeInTheDocument()
    const evidenceButton = within(bugRow).getByRole('button', { name: /evidencia dsw-pr-017/i })

    await user.click(evidenceButton)

    expect(setActiveBugEvidenceId).toHaveBeenCalledWith(101)
    expect(openModal).toHaveBeenCalledWith('bug-evidence')
  })

  it('offers "Crear tarea de corrección" only on Bug rows and opens the form for that bug', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([
      { ...oneActivity[0], id: 1, work_item_id: 101, work_item_type: 'Bug' },
      { ...oneActivity[0], id: 2, work_item_id: 202, work_item_type: 'Task' },
    ])
    const user = userEvent.setup()

    render(<WorkItemsView />)
    await screen.findByText('101')

    const rows = screen.getAllByRole('row')
    const bugRow = rows.find(r => r.textContent?.includes('101'))!
    const taskRow = rows.find(r => r.textContent?.includes('202'))!
    expect(within(taskRow).queryByRole('button', { name: /crear tarea de corrección/i })).not.toBeInTheDocument()

    await user.click(within(bugRow).getByRole('button', { name: /crear tarea de corrección/i }))

    expect(screen.getByRole('dialog', { name: 'Crear tarea de corrección' })).toHaveTextContent('bug 101')
  })

  it('offers "Crear tarea de corrección" on assigned-but-uncatalogued Bug items', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([])
    vi.mocked(listAssignedAzureWorkItems).mockResolvedValue({
      org: 'ORG',
      items: [
        { id: 171191, title: 'Assigned bug', type: 'Bug', state: 'Active' },
        { id: 505, title: 'New assigned task', type: 'Task', state: 'New' },
      ],
    })
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText(/no hay work items registrados/i)

    await user.click(screen.getByRole('button', { name: /sincronizar asignados/i }))
    const pendingSection = screen.getByText('Asignados en Azure sin catalogar').closest('.card') as HTMLElement
    await within(pendingSection).findByText('Assigned bug')

    const buttons = within(pendingSection).getAllByRole('button', { name: /crear tarea de corrección/i })
    expect(buttons).toHaveLength(1)
    await user.click(buttons[0])

    expect(screen.getByRole('dialog', { name: 'Crear tarea de corrección' })).toHaveTextContent('bug 171191')
  })

  it('closes a work item after confirmation and shows the synced hours', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([{ ...oneActivity[0], work_item_type: 'Task' }])
    vi.mocked(closeAzureWorkItem).mockResolvedValue({ state: 'Closed', hours_synced: 3 })
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const user = userEvent.setup()

    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('button', { name: /cerrar/i }))

    expect(confirmSpy).toHaveBeenCalled()
    expect(closeAzureWorkItem).toHaveBeenCalledWith(101)
    await waitFor(() => expect(pushToast).toHaveBeenCalledWith(expect.stringContaining('3'), false))
    expect(await screen.findByText('Closed')).toBeInTheDocument()

    confirmSpy.mockRestore()
  })

  it('does not call closeAzureWorkItem when the confirmation is cancelled', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([{ ...oneActivity[0], work_item_type: 'Task' }])
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const user = userEvent.setup()

    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('button', { name: /cerrar/i }))

    expect(closeAzureWorkItem).not.toHaveBeenCalled()

    confirmSpy.mockRestore()
  })

  it('blocks Cerrar/Recrear with a toast (no confirm, no API call) when the last known assignee does not match the connected identity', async () => {
    useAppState.mockReturnValue({ azureConfig: { user_display_name: 'Jane Doe' } })
    vi.mocked(listAzureActivities).mockResolvedValue([
      { ...oneActivity[0], work_item_type: 'Task', last_known_assigned_to: 'Someone Else' },
    ])
    const confirmSpy = vi.spyOn(window, 'confirm')
    const user = userEvent.setup()

    render(<WorkItemsView />)
    await screen.findByText('101')

    const closeBtn = screen.getByRole('button', { name: /cerrar/i })
    expect(closeBtn).not.toBeDisabled()

    await user.click(closeBtn)

    expect(confirmSpy).not.toHaveBeenCalled()
    expect(closeAzureWorkItem).not.toHaveBeenCalled()
    expect(pushToast).toHaveBeenCalledWith(expect.stringContaining('Someone Else'), true)

    await user.click(screen.getByRole('button', { name: /recrear/i }))
    expect(recreateAzureWorkItem).not.toHaveBeenCalled()

    confirmSpy.mockRestore()
  })

  it('keeps Cerrar/Recrear enabled when the last known assignee matches the connected identity', async () => {
    useAppState.mockReturnValue({ azureConfig: { user_display_name: 'Jane Doe' } })
    vi.mocked(listAzureActivities).mockResolvedValue([
      { ...oneActivity[0], work_item_type: 'Task', last_known_assigned_to: 'Jane Doe' },
    ])

    render(<WorkItemsView />)
    await screen.findByText('101')

    expect(screen.getByRole('button', { name: /cerrar/i })).not.toBeDisabled()
    expect(screen.getByRole('button', { name: /recrear/i })).not.toBeDisabled()
  })

  it('keeps Cerrar/Recrear enabled when the assignee is unknown (no refresh has run yet)', async () => {
    useAppState.mockReturnValue({ azureConfig: { user_display_name: 'Jane Doe' } })
    vi.mocked(listAzureActivities).mockResolvedValue([{ ...oneActivity[0], work_item_type: 'Task' }])

    render(<WorkItemsView />)
    await screen.findByText('101')

    expect(screen.getByRole('button', { name: /cerrar/i })).not.toBeDisabled()
    expect(screen.getByRole('button', { name: /recrear/i })).not.toBeDisabled()
  })

  it('recreates a work item after confirmation, refreshing the catalog to show the new id', async () => {
    vi.mocked(listAzureActivities)
      .mockResolvedValueOnce([{ ...oneActivity[0], work_item_type: 'Task' }])
      .mockResolvedValueOnce([{ ...oneActivity[0], work_item_id: 909, work_item_type: 'Task' }])
    vi.mocked(recreateAzureWorkItem).mockResolvedValue({
      id: 909, state: 'Active', hours_synced: 1, catalog_reassigned: true, azure_activity_id: 1,
    })
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const user = userEvent.setup()

    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('button', { name: /recrear/i }))

    expect(recreateAzureWorkItem).toHaveBeenCalledWith(101)
    expect(await screen.findByText('909')).toBeInTheDocument()
    expect(listAzureActivities).toHaveBeenCalledTimes(2)

    confirmSpy.mockRestore()
  })

  it('renders an Azure DevOps link per row on load, without a states refresh', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)
    render(<WorkItemsView />)
    await screen.findByText('101')

    const link = screen.getByTitle(/abrir en azure devops/i)
    expect(link).toHaveAttribute('href', 'https://dev.azure.com/ORG/_workitems/edit/101')
    expect(link).toHaveAttribute('target', '_blank')
    expect(fetchAzureWorkItemStates).not.toHaveBeenCalled()
  })

  it('shows the parent bug under the task label, linked to Azure', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([
      {
        ...oneActivity[0], work_item_id: 171306, label: 'Atención y/o Corrección del defecto 171191', work_item_type: 'Task',
        parent_work_item_id: 171191, parent_title: 'Login falla', parent_type: 'Bug',
      },
    ])
    render(<WorkItemsView />)
    await screen.findByText('171306')

    const parentLink = screen.getByRole('link', { name: /#171191 — Login falla/ })
    expect(parentLink).toHaveAttribute('href', 'https://dev.azure.com/ORG/_workitems/edit/171191')
    expect(parentLink).toHaveAttribute('target', '_blank')
    expect(parentLink).toHaveAttribute('rel', 'noreferrer')
    expect(within(parentLink).getByLabelText('Bug')).toBeInTheDocument()
  })

  it('opens the parent bug evidence panel from a bug child task row', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([
      {
        ...oneActivity[0], work_item_id: 171306, label: 'Atención y/o Corrección del defecto 171191', work_item_type: 'Task',
        parent_work_item_id: 171191, parent_title: 'Login falla', parent_type: 'Bug',
      },
    ])
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('171306')

    await user.click(screen.getByRole('button', { name: /evidencia del bug #171191/i }))

    expect(setActiveBugEvidenceId).toHaveBeenCalledWith(171191)
    expect(openModal).toHaveBeenCalledWith('bug-evidence')
  })

  it('does not offer bug evidence for a non-bug parent', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([
      { ...oneActivity[0], work_item_type: 'Task', parent_work_item_id: 900, parent_title: 'Historia', parent_type: 'User Story' },
    ])
    render(<WorkItemsView />)
    await screen.findByText('101')

    expect(screen.queryByRole('button', { name: /evidencia del bug/i })).not.toBeInTheDocument()
  })

  it('shows a non-bug parent with a plain type prefix instead of the bug icon', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([
      { ...oneActivity[0], work_item_type: 'Task', parent_work_item_id: 900, parent_title: 'Historia', parent_type: 'User Story' },
    ])
    render(<WorkItemsView />)
    await screen.findByText('101')

    const parentLink = screen.getByRole('link', { name: /User Story #900 — Historia/ })
    expect(within(parentLink).queryByLabelText('Bug')).not.toBeInTheDocument()
  })

  it('assigned rows show an Azure link and the parent bug, and adding persists the parent', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)
    vi.mocked(listAssignedAzureWorkItems).mockResolvedValue({
      org: 'ORG',
      items: [
        { id: 171306, title: 'Atención y/o Corrección del defecto 171191', type: 'Task', state: 'Active', parent_id: 171191, parent_title: 'Login falla', parent_type: 'Bug' },
      ],
    })
    vi.mocked(addAzureActivity).mockResolvedValue({
      id: 2, org: 'ORG', work_item_id: 171306, label: 'x', work_item_type: 'Task', is_active: true, is_default: false,
    })
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('button', { name: /sincronizar asignados/i }))
    const pendingSection = screen.getByText('Asignados en Azure sin catalogar').closest('.card') as HTMLElement
    await within(pendingSection).findByText('Atención y/o Corrección del defecto 171191')

    expect(within(pendingSection).getByTitle(/abrir en azure devops/i)).toHaveAttribute('href', 'https://dev.azure.com/ORG/_workitems/edit/171306')
    const parentLink = within(pendingSection).getByRole('link', { name: /#171191 — Login falla/ })
    expect(parentLink).toHaveAttribute('href', 'https://dev.azure.com/ORG/_workitems/edit/171191')

    await user.click(within(pendingSection).getByRole('button', { name: /agregar/i }))
    expect(addAzureActivity).toHaveBeenCalledWith({
      org: 'ORG', work_item_id: 171306, label: 'Atención y/o Corrección del defecto 171191', work_item_type: 'Task',
      parent_work_item_id: 171191, parent_title: 'Login falla', parent_type: 'Bug',
    })
  })

  it('filters hide bugs, tasks, and closed rows independently, leaving unknown-state rows visible', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([
      { ...oneActivity[0], id: 1, work_item_id: 101, work_item_type: 'Bug', label: 'Bug row' },
      { ...oneActivity[0], id: 2, work_item_id: 202, work_item_type: 'Task', label: 'Task row' },
      { ...oneActivity[0], id: 3, work_item_id: 303, work_item_type: 'Task', label: 'Closed task row' },
    ])
    vi.mocked(fetchAzureWorkItemStates).mockResolvedValue({
      org: 'ORG', team_project: 'RUNTPRO',
      items: [
        { id: 202, title: 'Task row', type: 'Task', state: 'Active' },
        { id: 303, title: 'Closed task row', type: 'Task', state: 'Closed' },
        // 101 deliberately absent — unknown state, must never be hidden by "Ocultar cerrados".
      ],
    })
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('Bug row')
    await user.click(screen.getByRole('button', { name: /refrescar estados/i }))
    await screen.findByText('Closed')

    await user.click(screen.getByRole('checkbox', { name: /ocultar bugs/i }))
    expect(screen.queryByText('Bug row')).not.toBeInTheDocument()
    expect(screen.getByText('Task row')).toBeInTheDocument()
    await user.click(screen.getByRole('checkbox', { name: /ocultar bugs/i }))

    await user.click(screen.getByRole('checkbox', { name: /ocultar tasks/i }))
    expect(screen.getByText('Bug row')).toBeInTheDocument()
    expect(screen.queryByText('Task row')).not.toBeInTheDocument()
    expect(screen.queryByText('Closed task row')).not.toBeInTheDocument()
    await user.click(screen.getByRole('checkbox', { name: /ocultar tasks/i }))

    await user.click(screen.getByRole('checkbox', { name: /ocultar cerrados/i }))
    expect(screen.getByText('Bug row')).toBeInTheDocument()
    expect(screen.getByText('Task row')).toBeInTheDocument()
    expect(screen.queryByText('Closed task row')).not.toBeInTheDocument()
  })

  it('shows a distinct message when filters hide every row, versus an empty catalog', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue([{ ...oneActivity[0], work_item_type: 'Bug' }])
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('checkbox', { name: /ocultar bugs/i }))

    expect(await screen.findByText(/ningún work item coincide con los filtros/i)).toBeInTheDocument()
    expect(screen.queryByText(/no hay work items registrados en el catálogo todavía/i)).not.toBeInTheDocument()
  })

  it('sync-assigned lists assigned-but-not-catalogued items, excluding already-catalogued ones', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity) // work_item_id 101 already catalogued
    vi.mocked(listAssignedAzureWorkItems).mockResolvedValue({
      org: 'ORG',
      items: [
        { id: 101, title: 'Fix login bug', type: 'Bug', state: 'Active' },
        { id: 505, title: 'New assigned task', type: 'Task', state: 'New' },
      ],
    })
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('button', { name: /sincronizar asignados/i }))

    const pendingSection = screen.getByText('Asignados en Azure sin catalogar').closest('.card') as HTMLElement
    expect(await within(pendingSection).findByText('New assigned task')).toBeInTheDocument()
    expect(within(pendingSection).queryByText('Fix login bug')).not.toBeInTheDocument()
  })

  it('sync-assigned shows an all-caught-up message when nothing is pending', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)
    vi.mocked(listAssignedAzureWorkItems).mockResolvedValue({
      org: 'ORG',
      items: [{ id: 101, title: 'Fix login bug', type: 'Bug', state: 'Active' }],
    })
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('button', { name: /sincronizar asignados/i }))

    expect(await screen.findByText(/todo al día/i)).toBeInTheDocument()
  })

  it('adding a pending assigned item calls addAzureActivity with the right payload and removes it from the pending list', async () => {
    vi.mocked(listAzureActivities)
      .mockResolvedValueOnce(oneActivity)
      .mockResolvedValueOnce([...oneActivity, { ...oneActivity[0], id: 2, work_item_id: 505, label: 'New assigned task', work_item_type: 'Task' }])
    vi.mocked(listAssignedAzureWorkItems).mockResolvedValue({
      org: 'ORG',
      items: [
        { id: 101, title: 'Fix login bug', type: 'Bug', state: 'Active' },
        { id: 505, title: 'New assigned task', type: 'Task', state: 'New' },
      ],
    })
    vi.mocked(addAzureActivity).mockResolvedValue({
      id: 2, org: 'ORG', work_item_id: 505, label: 'New assigned task', work_item_type: 'Task', is_active: true, is_default: false,
    })
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('button', { name: /sincronizar asignados/i }))
    const pendingSection = screen.getByText('Asignados en Azure sin catalogar').closest('.card') as HTMLElement
    await within(pendingSection).findByText('New assigned task')

    await user.click(within(pendingSection).getByRole('button', { name: /agregar/i }))

    expect(addAzureActivity).toHaveBeenCalledWith({ org: 'ORG', work_item_id: 505, label: 'New assigned task', work_item_type: 'Task' })
    // The item leaves the pending section (and, via the refreshed catalog
    // fetch, shows up in the main table instead — see the second
    // listAzureActivities mock above).
    await waitFor(() => expect(within(pendingSection).queryByText('New assigned task')).not.toBeInTheDocument())
  })

  it('keeps the assigned search box after adding the only filtered match, so the rest of the list can be recovered', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)
    vi.mocked(listAssignedAzureWorkItems).mockResolvedValue({
      org: 'ORG',
      items: [
        { id: 505, title: 'New assigned task', type: 'Task', state: 'New' },
        { id: 606, title: 'Another pending item', type: 'Bug', state: 'New' },
      ],
    })
    vi.mocked(addAzureActivity).mockResolvedValue({
      id: 2, org: 'ORG', work_item_id: 505, label: 'New assigned task', work_item_type: 'Task', is_active: true, is_default: false,
    })
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('101')

    await user.click(screen.getByRole('button', { name: /sincronizar asignados/i }))
    const pendingSection = screen.getByText('Asignados en Azure sin catalogar').closest('.card') as HTMLElement
    const search = await within(pendingSection).findByRole('searchbox', { name: /buscar work items asignados/i })
    await user.type(search, '505')
    await user.click(within(pendingSection).getByRole('button', { name: /agregar/i }))

    await waitFor(() => expect(within(pendingSection).queryByText('New assigned task')).not.toBeInTheDocument())
    const searchAfter = within(pendingSection).getByRole('searchbox', { name: /buscar work items asignados/i })
    await user.clear(searchAfter)
    expect(await within(pendingSection).findByText('Another pending item')).toBeInTheDocument()
  })
})
