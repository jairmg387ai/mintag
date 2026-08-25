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
}))

const pushToast = vi.fn()
vi.mock('../../store/AppContext', () => ({
  useAppActions: () => ({ pushToast }),
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
    vi.mocked(getActivityCatalog).mockReset()
    vi.mocked(listAzureActivities).mockReset()
    vi.mocked(fetchAzureWorkItemStates).mockReset()
    vi.mocked(closeAzureWorkItem).mockReset()
    vi.mocked(recreateAzureWorkItem).mockReset()
    vi.mocked(listAssignedAzureWorkItems).mockReset()
    vi.mocked(addAzureActivity).mockReset()
    vi.mocked(getActivityCatalog).mockResolvedValue(catalog)
  })

  it('renders catalog rows from listAzureActivities', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)

    render(<WorkItemsView />)

    expect(await screen.findByText('101')).toBeInTheDocument()
    expect(screen.getByText('Fix login bug')).toBeInTheDocument()
    expect(screen.getByText('Mintag')).toBeInTheDocument()
    expect(screen.getByText('Desarrollo')).toBeInTheDocument()
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

  it('renders an Azure DevOps link per row only after a states refresh has resolved org/team_project', async () => {
    vi.mocked(listAzureActivities).mockResolvedValue(oneActivity)
    vi.mocked(fetchAzureWorkItemStates).mockResolvedValue({
      org: 'ORG', team_project: 'RUNTPRO',
      items: [{ id: 101, title: 'Fix login bug', type: 'Bug', state: 'Active' }],
    })
    const user = userEvent.setup()
    render(<WorkItemsView />)
    await screen.findByText('101')

    expect(screen.queryByTitle(/abrir en azure devops/i)).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /refrescar estados/i }))

    const link = await screen.findByTitle(/abrir en azure devops/i)
    expect(link).toHaveAttribute('href', 'https://dev.azure.com/ORG/RUNTPRO/_workitems/edit/101')
    expect(link).toHaveAttribute('target', '_blank')
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
})
