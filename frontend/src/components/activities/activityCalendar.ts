import type { DailyActivity } from '../../types'
import { dailyTargetHours, isBusinessDay, round1, toYMD, type DateRange } from '../dashboard/timeLogKpis'

// Pure month-calendar computation for ActivityCalendarView. `today` is passed
// in so tests stay deterministic; dates compare as local YYYY-MM-DD strings
// like DailyActivity.date.

// complete: business day that met its goal (or any day at/over goal).
// short / empty: past business day below its goal / with no hours at all.
// pending: today or a future business day still below its goal — not flagged.
// weekend: Saturday or Sunday (no goal; hours, if any, still count).
export type CalendarDayState = 'complete' | 'short' | 'empty' | 'pending' | 'weekend'

export interface CalendarDay {
  date: string
  dayOfMonth: number
  weekday: number
  isToday: boolean
  activities: DailyActivity[]
  hours: number
  target: number
  state: CalendarDayState
}

export interface CalendarWeek {
  days: CalendarDay[]
  hours: number
  target: number
  incompleteDays: number
}

export interface MonthCalendar {
  weeks: CalendarWeek[]
  hours: number
  target: number
  incompleteDays: number
}

// month is 0-based, matching Date#getMonth.
export function monthRange(year: number, month: number): DateRange {
  return { from: new Date(year, month, 1), to: new Date(year, month + 1, 0) }
}

function dayState(date: string, business: boolean, hours: number, target: number, todayYMD: string): CalendarDayState {
  if (!business) return 'weekend'
  if (hours >= target) return 'complete'
  if (date >= todayYMD) return 'pending'
  return hours === 0 ? 'empty' : 'short'
}

function isIncomplete(d: CalendarDay): boolean {
  return d.state === 'empty' || d.state === 'short'
}

export function buildMonthCalendar(year: number, month: number, activities: DailyActivity[], today: Date): MonthCalendar {
  const byDate = new Map<string, DailyActivity[]>()
  for (const a of activities) {
    const list = byDate.get(a.date)
    if (list) list.push(a)
    else byDate.set(a.date, [a])
  }

  const todayYMD = toYMD(today)
  const { to } = monthRange(year, month)
  const weeks: CalendarWeek[] = []
  let current: CalendarDay[] = []

  for (let dom = 1; dom <= to.getDate(); dom++) {
    const d = new Date(year, month, dom)
    const date = toYMD(d)
    const weekday = d.getDay()
    const business = isBusinessDay(d)
    const dayActivities = byDate.get(date) ?? []
    const hours = round1(dayActivities.reduce((s, a) => s + a.hours, 0))
    const target = business ? dailyTargetHours(weekday) : 0

    // A new week starts every Monday (weeks run Monday to Sunday).
    if (weekday === 1 && current.length > 0) {
      weeks.push(summarizeWeek(current))
      current = []
    }
    current.push({
      date,
      dayOfMonth: dom,
      weekday,
      isToday: date === todayYMD,
      activities: dayActivities,
      hours,
      target,
      state: dayState(date, business, hours, target, todayYMD),
    })
  }
  if (current.length > 0) weeks.push(summarizeWeek(current))

  return {
    weeks,
    hours: round1(weeks.reduce((s, w) => s + w.hours, 0)),
    target: round1(weeks.reduce((s, w) => s + w.target, 0)),
    incompleteDays: weeks.reduce((s, w) => s + w.incompleteDays, 0),
  }
}

function summarizeWeek(days: CalendarDay[]): CalendarWeek {
  return {
    days,
    hours: round1(days.reduce((s, d) => s + d.hours, 0)),
    target: round1(days.reduce((s, d) => s + d.target, 0)),
    incompleteDays: days.filter(isIncomplete).length,
  }
}
