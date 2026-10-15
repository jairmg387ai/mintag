import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { listActivitiesRange } from '../../api/client'
import type { DailyActivity } from '../../types'
import { ActivityCalendarView } from './ActivityCalendarView'

const openActivitiesOn = vi.fn()

vi.mock('../../api/client', () => ({
  listActivitiesRange: vi.fn(),
}))

vi.mock('../../store/AppContext', () => ({
  useAppActions: () => ({ openActivitiesOn }),
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

function dayRow(label: RegExp) {
  return screen.getByRole('button', { name: label }).closest('tr') as HTMLElement
}

describe('ActivityCalendarView', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 9, 14, 10, 0, 0)) // Wednesday 14 Oct 2026
    openActivitiesOn.mockReset()
    vi.mocked(listActivitiesRange).mockReset()
    vi.mocked(listActivitiesRange).mockResolvedValue([
      act('2026-10-05', 8, { project: 'RUNT', category: 'Soporte' }),
      act('2026-10-06', 6),
      act('2026-10-09', 7.5),
    ])
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads the current month in a single range request', async () => {
    render(<ActivityCalendarView />)
    await waitFor(() => expect(listActivitiesRange).toHaveBeenCalledWith('2026-10-01', '2026-10-31'))
    expect(listActivitiesRange).toHaveBeenCalledTimes(1)
  })

  it('shows each day with its activities, flags days below goal, and subtotals each week', async () => {
    render(<ActivityCalendarView />)
    await screen.findByText('RUNT')

    expect(within(dayRow(/martes, 6 de octubre/i)).getByText('6.0h')).toBeInTheDocument()
    expect(within(dayRow(/miércoles, 7 de octubre/i)).getByText('Sin registros')).toBeInTheDocument()
    expect(within(dayRow(/viernes, 9 de octubre/i)).getByText(/\/ 7\.5h/)).toBeInTheDocument()

    const weekRow = screen.getByText(/Semana del 5 al 11/).closest('tr') as HTMLElement
    expect(within(weekRow).getByText('21.5h')).toBeInTheDocument()
    expect(within(weekRow).getByText('/ 39.5h')).toBeInTheDocument()
    expect(within(weekRow).getByText(/3 días incompletos/)).toBeInTheDocument()
  })

  it('lists the activities of each day as an inner table with the daily list columns', async () => {
    vi.mocked(listActivitiesRange).mockResolvedValue([
      act('2026-10-05', 5, { project: 'RUNT', category: 'Soporte', registro_diario: 'Fix login', status: 'approved' }),
      act('2026-10-05', 3, { project: 'Mintag', category: 'Desarrollo', registro_diario: 'Calendar', status: 'uploaded' }),
    ])
    render(<ActivityCalendarView />)
    await screen.findByText('Fix login')

    const list = within(dayRow(/lunes, 5 de octubre/i)).getByRole('table', { name: /actividades del lunes, 5 de octubre/i })
    const rows = within(list).getAllByRole('row')
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent(/RUNT.*5\.00.*Soporte.*Fix login.*Aprobada/)
    expect(rows[1]).toHaveTextContent(/Mintag.*3\.00.*Desarrollo.*Calendar.*Subida/)
    expect(within(dayRow(/martes, 6 de octubre/i)).queryByRole('table')).not.toBeInTheDocument()
  })

  it('opens the daily view on the clicked day', async () => {
    const user = userEvent.setup()
    render(<ActivityCalendarView />)
    await screen.findByText('RUNT')
    await user.click(screen.getByRole('button', { name: /miércoles, 7 de octubre/i }))
    expect(openActivitiesOn).toHaveBeenCalledWith('2026-10-07')
  })

  it('navigates to the next month', async () => {
    const user = userEvent.setup()
    render(<ActivityCalendarView />)
    await screen.findByText('RUNT')
    await user.click(screen.getByRole('button', { name: 'Mes siguiente' }))
    await waitFor(() => expect(listActivitiesRange).toHaveBeenLastCalledWith('2026-11-01', '2026-11-30'))
  })
})
