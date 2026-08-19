import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ActivityCatalog, AzureActivity, DailyActivity } from '../../types'
import { updateActivity } from '../../api/client'
import { EditActivityModal } from './EditActivityModal'

vi.mock('../../api/client', () => ({
  updateActivity: vi.fn(),
}))

function buildActivity(overrides: Partial<AzureActivity> = {}): AzureActivity {
  return {
    id: 1,
    org: 'my-org',
    work_item_id: 4521,
    label: 'Fix login bug',
    work_item_type: 'Bug',
    is_active: true,
    is_default: true,
    ...overrides,
  }
}

const azureActivities: AzureActivity[] = [
  buildActivity({ id: 1, label: 'Fix login bug', work_item_id: 4521, is_default: true }),
  buildActivity({ id: 2, label: 'Deploy pipeline', work_item_id: 9001, is_default: false }),
]

const catalog: ActivityCatalog = {
  projects: ['Project A'],
  categories: [{ id: 1, name: 'Development' }],
}

const activity: DailyActivity = {
  id: 42,
  date: '2026-08-19',
  hours: 2,
  project: 'Project A',
  category: 'Development',
  registro_diario: 'Trabajo previo',
  source: 'manual',
  status: 'pending',
  created_at: '2026-08-19T00:00:00Z',
  azure_activity_id: 1,
}

function renderModal() {
  const onSaved = vi.fn()
  const onClose = vi.fn()
  render(
    <EditActivityModal
      activity={activity}
      open
      onClose={onClose}
      onSaved={onSaved}
      catalog={catalog}
      azureActivities={azureActivities}
    />,
  )
  return { onSaved, onClose }
}

describe('EditActivityModal Azure activity picker', () => {
  beforeEach(() => {
    vi.mocked(updateActivity).mockReset()
    vi.mocked(updateActivity).mockResolvedValue({} as never)
  })

  it('omits azure_activity_id when the selection is untouched', async () => {
    const user = userEvent.setup()
    renderModal()

    await user.click(screen.getByRole('button', { name: 'Guardar cambios' }))

    const call = vi.mocked(updateActivity).mock.calls[0][1]
    expect(call).not.toHaveProperty('azure_activity_id')
  })

  it('sends the new work item id when the selection changes', async () => {
    const user = userEvent.setup()
    renderModal()

    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })
    await user.click(input)
    await user.click(screen.getByRole('option', { name: /Deploy pipeline/ }))

    await user.click(screen.getByRole('button', { name: 'Guardar cambios' }))

    const call = vi.mocked(updateActivity).mock.calls[0][1]
    expect(call).toMatchObject({ azure_activity_id: 2 })
  })
})
