import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { fetchWorkItemEffort, listActivitiesRange, listAzureActivities } from '../../api/client'
import type { DailyActivity } from '../../types'
import { Dashboard } from './Dashboard'

vi.mock('../../api/client', () => ({
  listActivitiesRange: vi.fn(),
  listAzureActivities: vi.fn(),
  fetchWorkItemEffort: vi.fn(),
}))

vi.mock('../../store/AppContext', () => ({
  useAppState: () => ({ stats: null, tasks: [], meetings: [] }),
  useAppActions: () => ({
    setEditingTaskId: vi.fn(),
    openModal: vi.fn(),
    setActiveMeetingId: vi.fn(),
    setView: vi.fn(),
  }),
}))

function act(date: string, hours: number, extra: Partial<DailyActivity> = {}): DailyActivity {
  return {
    id: Math.floor(Math.random() * 1e9),
    date,
    hours,
    project: 'Mintag',
    category: 'Desarrollo',
    registro_diario: '',
    source: 'manual',
    status: 'pending',
    created_at: '',
    ...extra,
  }
}

function card(label: RegExp) {
  return screen.getByText(label).closest('button') as HTMLElement
}

describe('Dashboard time-log period', () => {
  beforeEach(() => {
    // Wednesday 2026-10-07; week starts Mon 2026-10-05, month on 2026-10-01.
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 9, 7, 10, 0, 0))
    try { localStorage.clear() } catch { /* ignore */ }
    vi.mocked(listActivitiesRange).mockReset().mockResolvedValue([
      act('2026-10-01', 4, { status: 'approved' }),
      act('2026-10-05', 8, { category: 'Reuniones' }),
      act('2026-10-06', 3),
    ])
    vi.mocked(listAzureActivities).mockReset().mockResolvedValue([])
    vi.mocked(fetchWorkItemEffort).mockReset().mockResolvedValue({ items: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('defaults to the current week and switches to the month', async () => {
    render(<Dashboard />)

    expect(listActivitiesRange).toHaveBeenCalledWith('2026-10-01', '2026-10-07')
    expect(await screen.findByText('11h')).toBeInTheDocument()
    expect(card(/Horas registradas \(semana\)/)).toHaveTextContent('11h')
    expect(card(/Meta de la semana/)).toHaveTextContent('24h')

    // Tue 2026-10-06 logged 3h < 8h; Mon is complete, today excluded.
    expect(card(/Días por debajo de la meta/)).toHaveTextContent('1')

    await userEvent.click(screen.getByRole('button', { name: 'Mes actual' }))

    expect(card(/Horas registradas \(mes\)/)).toHaveTextContent('15h')
    expect(card(/Meta del mes/)).toHaveTextContent('39.5h')
    expect(screen.getByRole('button', { name: 'Mes actual' })).toHaveAttribute('aria-pressed', 'true')

    const projects = within(screen.getByRole('list', { name: 'Horas por proyecto' })).getAllByRole('listitem')
    expect(projects[0]).toHaveTextContent('15.0h')
  })

  it('shows approved hours waiting for upload', async () => {
    render(<Dashboard />)
    await screen.findByText('11h')
    expect(card(/Pendiente de subir/)).toHaveTextContent('4h')
    expect(card(/Pendiente de subir/)).toHaveTextContent('1 aprobada')
  })

  it('remembers the selected period', async () => {
    try { localStorage.setItem('mintag.dashboard.period', 'month') } catch { /* ignore */ }
    render(<Dashboard />)
    expect(await screen.findByText(/Horas registradas \(mes\)/)).toBeInTheDocument()
  })
})
