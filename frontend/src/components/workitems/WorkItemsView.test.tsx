import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { getActivityCatalog, listAzureActivities, fetchAzureWorkItemStates, closeAzureWorkItem, recreateAzureWorkItem } from '../../api/client'
import { WorkItemsView } from './WorkItemsView'

vi.mock('../../api/client', () => ({
  getActivityCatalog: vi.fn(),
  listAzureActivities: vi.fn(),
  fetchAzureWorkItemStates: vi.fn(),
  closeAzureWorkItem: vi.fn(),
  recreateAzureWorkItem: vi.fn(),
}))

const pushToast = vi.fn()
vi.mock('../../store/AppContext', () => ({
  useAppActions: () => ({ pushToast }),
}))

const catalog = {
  projects: [{ name: 'Mintag', is_active: true }],
  categories: [{ id: 7, name: 'Desarrollo' }],
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
})
