import { describe, expect, it } from 'vitest'
import type { DailyActivity } from '../../types'
import { buildMonthCalendar, monthRange } from './activityCalendar'

function act(date: string, hours: number): DailyActivity {
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
  }
}

// October 2026: the 1st is a Thursday, the 31st a Saturday.
const TODAY = new Date(2026, 9, 14) // Wednesday 14 Oct 2026

function day(cal: ReturnType<typeof buildMonthCalendar>, ymd: string) {
  const found = cal.weeks.flatMap(w => w.days).find(d => d.date === ymd)
  if (!found) throw new Error(`day ${ymd} not in calendar`)
  return found
}

describe('monthRange', () => {
  it('covers the first through the last day of the month', () => {
    const { from, to } = monthRange(2026, 1)
    expect(from).toEqual(new Date(2026, 1, 1))
    expect(to).toEqual(new Date(2026, 1, 28))
  })
})

describe('buildMonthCalendar', () => {
  it('lists every day of the month, top to bottom, grouped into Monday-start weeks', () => {
    const cal = buildMonthCalendar(2026, 9, [], TODAY)
    const days = cal.weeks.flatMap(w => w.days)
    expect(days).toHaveLength(31)
    expect(days[0].date).toBe('2026-10-01')
    expect(days[30].date).toBe('2026-10-31')
    // Partial first week (Thu 1 – Sun 4), then Monday-start weeks.
    expect(cal.weeks[0].days.map(d => d.dayOfMonth)).toEqual([1, 2, 3, 4])
    expect(cal.weeks[1].days[0].date).toBe('2026-10-05')
    expect(cal.weeks[1].days).toHaveLength(7)
    expect(cal.weeks).toHaveLength(5)
  })

  it('uses an 8h goal Monday to Thursday and 7.5h on Friday, none on weekends', () => {
    const cal = buildMonthCalendar(2026, 9, [], TODAY)
    expect(day(cal, '2026-10-05').target).toBe(8) // Monday
    expect(day(cal, '2026-10-08').target).toBe(8) // Thursday
    expect(day(cal, '2026-10-09').target).toBe(7.5) // Friday
    expect(day(cal, '2026-10-10').target).toBe(0) // Saturday
    expect(day(cal, '2026-10-10').state).toBe('weekend')
  })

  it('flags past business days with no hours as empty and below the goal as short', () => {
    const cal = buildMonthCalendar(2026, 9, [
      act('2026-10-05', 8),
      act('2026-10-06', 4), act('2026-10-06', 2),
      act('2026-10-09', 7.5),
      act('2026-10-08', 7),
    ], TODAY)
    expect(day(cal, '2026-10-05').state).toBe('complete')
    expect(day(cal, '2026-10-06').hours).toBe(6)
    expect(day(cal, '2026-10-06').state).toBe('short')
    expect(day(cal, '2026-10-07').state).toBe('empty')
    expect(day(cal, '2026-10-08').state).toBe('short')
    expect(day(cal, '2026-10-09').state).toBe('complete') // 7.5h meets Friday's goal
  })

  it('does not flag today or future days that are still below the goal', () => {
    const cal = buildMonthCalendar(2026, 9, [act('2026-10-14', 3)], TODAY)
    expect(day(cal, '2026-10-14').state).toBe('pending')
    expect(day(cal, '2026-10-14').isToday).toBe(true)
    expect(day(cal, '2026-10-20').state).toBe('pending')
  })

  it('sums hours, goal and incomplete days per week and for the month', () => {
    const cal = buildMonthCalendar(2026, 9, [
      act('2026-10-05', 8),
      act('2026-10-06', 6),
      act('2026-10-09', 7.5),
      act('2026-10-11', 2), // Sunday hours still count toward the week total
      act('2026-09-30', 8), // outside the month: ignored
    ], TODAY)
    const week = cal.weeks[1] // Mon 5 – Sun 11
    expect(week.hours).toBe(23.5)
    expect(week.target).toBe(39.5)
    expect(week.incompleteDays).toBe(3) // Tue short, Wed + Thu empty
    expect(cal.hours).toBe(23.5)
    expect(cal.target).toBe(8 * 17 + 7.5 * 5) // 17 Mon-Thu + 5 Fridays in Oct 2026
  })
})
