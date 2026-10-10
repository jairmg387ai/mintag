import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import { fetchWorkItemEffort, listAzureActivities } from '../../api/client'
import type { AzureActivity, DailyActivity } from '../../types'
import { WorkItemAlerts } from './WorkItemAlerts'

vi.mock('../../api/client', () => ({
  listAzureActivities: vi.fn(),
  fetchWorkItemEffort: vi.fn(),
}))

function wi(id: number, workItemId: number, label: string, extra: Partial<AzureActivity> = {}): AzureActivity {
  return {
    id,
    org: 'ORG',
    work_item_id: workItemId,
    label,
    work_item_type: 'Task',
    is_active: true,
    is_default: false,
    ...extra,
  }
}

const weekActivity: DailyActivity = {
  id: 1,
  date: '2026-10-05',
  hours: 2,
  project: 'Mintag',
  category: 'Desarrollo',
  registro_diario: '',
  source: 'manual',
  status: 'pending',
  created_at: '',
  azure_activity_id: 1,
}

const catalog = [
  wi(1, 101, 'Login fix'),
  wi(2, 102, 'Report export'),
  wi(3, 103, 'Old closed task', { last_known_state: 'Closed' }),
  wi(4, 104, 'Payments API'),
]

describe('WorkItemAlerts', () => {
  beforeEach(() => {
    vi.mocked(listAzureActivities).mockReset().mockResolvedValue(catalog)
    vi.mocked(fetchWorkItemEffort).mockReset().mockResolvedValue({
      items: [
        { id: 101, original_estimate: 10, uploaded_hours: 6, local_hours: 2.5, remaining: 1.5, has_estimate: true },
        { id: 102, original_estimate: 10, uploaded_hours: 1, local_hours: 0, remaining: 9, has_estimate: true },
        { id: 104, original_estimate: 4, uploaded_hours: 4, local_hours: 1, remaining: 0, has_estimate: true },
      ],
    })
  })

  it('lists open work items without hours this week and those near or over estimate', async () => {
    render(<WorkItemAlerts weekActivities={[weekActivity]} />)

    const idle = await screen.findByRole('list', { name: 'Work items sin horas esta semana' })
    const idleItems = within(idle).getAllByRole('listitem')
    expect(idleItems.map(li => li.textContent)).toEqual([
      expect.stringContaining('Report export'),
      expect.stringContaining('Payments API'),
    ])

    const over = await screen.findByRole('list', { name: 'Work items con 80% o más del estimado' })
    const overItems = within(over).getAllByRole('listitem')
    expect(overItems).toHaveLength(2)
    expect(overItems[0]).toHaveTextContent('Payments API')
    expect(overItems[0]).toHaveTextContent('usado 5 de 4h (125%)')
    expect(overItems[0]).toHaveAttribute('data-over', 'true')
    expect(overItems[1]).toHaveTextContent('Login fix')
    expect(overItems[1]).toHaveTextContent('usado 8.5 de 10h (85%)')
    expect(overItems[1]).toHaveAttribute('data-over', 'false')

    // Only open work items are asked for effort, de-duplicated.
    expect(fetchWorkItemEffort).toHaveBeenCalledWith([101, 102, 104])
  })

  it('keeps the idle list when the effort call fails', async () => {
    vi.mocked(fetchWorkItemEffort).mockRejectedValue(new Error('boom'))
    render(<WorkItemAlerts weekActivities={[weekActivity]} />)

    expect(await screen.findByRole('list', { name: 'Work items sin horas esta semana' })).toBeInTheDocument()
    expect(await screen.findByText('No se pudo calcular el consumo de los work items.')).toBeInTheDocument()
  })

  it('shows a muted note when the catalog cannot be loaded', async () => {
    vi.mocked(listAzureActivities).mockRejectedValue(new Error('boom'))
    render(<WorkItemAlerts weekActivities={[]} />)

    expect(await screen.findByText('No se pudo cargar el catálogo de work items.')).toBeInTheDocument()
    await waitFor(() => expect(fetchWorkItemEffort).not.toHaveBeenCalled())
    expect(screen.queryByRole('listitem')).toBeNull()
  })
})
