import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { getActivityCatalog, listAzureActivities, fetchAzureWorkItemStates } from '../../api/client'
import { WorkItemsView } from './WorkItemsView'

vi.mock('../../api/client', () => ({
  getActivityCatalog: vi.fn(),
  listAzureActivities: vi.fn(),
  fetchAzureWorkItemStates: vi.fn(),
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
})
