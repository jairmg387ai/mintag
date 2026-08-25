import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  listActivities,
  getActivityCatalog,
  getAzureTimeLogConfig,
  listAzureActivities,
} from '../../api/client'
import { ActivitiesView } from './ActivitiesView'

// This file covers only the day-complete auto-advance wiring added to
// ActivitiesView (see NewActivityModal's onCreated handler) — not full
// coverage of the view. NewActivityModal's own behavior is already covered
// by NewActivityModal.test.tsx, so it's stubbed here to a single button that
// fires onCreated on click, keeping this test focused on ActivitiesView's
// own date-advance logic rather than re-driving the real create form.
vi.mock('./NewActivityModal', () => ({
  NewActivityModal: (props: { open: boolean; onCreated: () => void }) =>
    props.open ? <button onClick={props.onCreated}>MockCreateActivity</button> : null,
}))
vi.mock('./EditActivityModal', () => ({ EditActivityModal: () => null }))
vi.mock('./CatalogManagementModal', () => ({ CatalogManagementModal: () => null }))
vi.mock('./ActivityDetailModal', () => ({ ActivityDetailModal: () => null }))
vi.mock('./ExportActivitiesModal', () => ({ ExportActivitiesModal: () => null }))

vi.mock('../../api/client', () => ({
  listActivities: vi.fn(),
  approveActivity: vi.fn(),
  unapproveActivity: vi.fn(),
  uploadActivities: vi.fn(),
  getActivityCatalog: vi.fn(),
  deleteActivity: vi.fn(),
  getAzureTimeLogConfig: vi.fn(),
  saveAzureTimeLogConfig: vi.fn(),
  clearAzureTimeLogConfig: vi.fn(),
  startAzureDeviceAuth: vi.fn(),
  completeAzureDeviceAuth: vi.fn(),
  listAzureActivities: vi.fn(),
}))

vi.mock('../../store/AppContext', () => ({
  useAppActions: () => ({ setAzureConfig: vi.fn() }),
}))

const catalog = { projects: [{ name: 'Mintag', is_active: true }], categories: [{ id: 1, name: 'Dev', is_active: true }] }
const azureConfig = { configured: false, auth_mode: 'bearer' as const, source: 'env' }

let nextActivityId = 1
function activity(hours: number, dateStr: string) {
  return {
    id: nextActivityId++, date: dateStr, hours, project: 'Mintag', category: 'Dev',
    registro_diario: 'x', source: 'manual' as const, status: 'pending' as const, created_at: `${dateStr}T00:00:00Z`,
  }
}

function toYMD(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${dd}`
}

// Mirrors ActivitiesView's own nextBusinessDay (not exported) so this test
// doesn't need to pin "today" via fake timers — computed the same way,
// independently, against whatever the real current date is.
function expectedNextBusinessDay(from: Date): string {
  const next = new Date(from)
  next.setDate(next.getDate() + 1)
  while (next.getDay() === 0 || next.getDay() === 6) {
    next.setDate(next.getDate() + 1)
  }
  return toYMD(next)
}

describe('ActivitiesView day-complete auto-advance', () => {
  beforeEach(() => {
    vi.mocked(listActivities).mockReset()
    vi.mocked(getActivityCatalog).mockReset().mockResolvedValue(catalog)
    vi.mocked(getAzureTimeLogConfig).mockReset().mockResolvedValue(azureConfig)
    vi.mocked(listAzureActivities).mockReset().mockResolvedValue([])
  })

  it('advances to the next business day when a created activity brings the day total to 8h', async () => {
    const today = toYMD(new Date())
    const nextDay = expectedNextBusinessDay(new Date())

    vi.mocked(listActivities)
      .mockResolvedValueOnce([activity(2, today)]) // initial load
      .mockResolvedValueOnce([activity(2, today), activity(6, today)]) // refetch after "creation": total 8h
      .mockResolvedValueOnce([]) // load for the advanced date

    const user = userEvent.setup()
    render(<ActivitiesView />)

    await user.click(await screen.findByRole('button', { name: /nueva actividad/i }))
    await user.click(screen.getByRole('button', { name: /mockcreateactivity/i }))

    await waitFor(() => expect(listActivities).toHaveBeenLastCalledWith(nextDay))
  })

  it('does not advance the date when the day total stays under 8h', async () => {
    const today = toYMD(new Date())

    vi.mocked(listActivities)
      .mockResolvedValueOnce([activity(2, today)]) // initial load
      .mockResolvedValueOnce([activity(2, today), activity(3, today)]) // refetch after "creation": total 5h

    const user = userEvent.setup()
    render(<ActivitiesView />)

    await user.click(await screen.findByRole('button', { name: /nueva actividad/i }))
    await user.click(screen.getByRole('button', { name: /mockcreateactivity/i }))

    await waitFor(() => expect(listActivities).toHaveBeenCalledTimes(2))
    expect(listActivities).toHaveBeenLastCalledWith(today)
  })
})
