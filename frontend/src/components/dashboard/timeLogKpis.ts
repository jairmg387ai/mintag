import type { AzureActivity, DailyActivity, WorkItemEffort } from '../../types'

// Pure time-log computations for the Dashboard. Every function takes `today`
// (or explicit dates) so tests stay deterministic. Dates are local, compared
// as YYYY-MM-DD strings like DailyActivity.date.

export type Period = 'week' | 'month'

export interface DateRange {
  from: Date
  to: Date
}

// Weekly schedule: Mon-Thu 8h/day, Fri 7.5h/day.
export function dailyTargetHours(day: number): number {
  return day === 5 ? 7.5 : 8
}

export function isBusinessDay(d: Date): boolean {
  const day = d.getDay()
  return day !== 0 && day !== 6
}

export function toYMD(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${dd}`
}

export function round1(n: number): number {
  return Math.round(n * 10) / 10
}

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate())
}

// weekStart returns the Monday of `today`'s week (Sunday belongs to the week
// that started the previous Monday).
export function weekStart(today: Date): Date {
  const start = startOfDay(today)
  const offset = (start.getDay() + 6) % 7
  start.setDate(start.getDate() - offset)
  return start
}

export function periodRange(period: Period, today: Date): DateRange {
  const to = startOfDay(today)
  const from = period === 'week' ? weekStart(today) : new Date(today.getFullYear(), today.getMonth(), 1)
  return { from, to }
}

// loadRange covers both the current week and the current month, so one fetch
// serves either period (the week can start in the previous month).
export function loadRange(today: Date): DateRange {
  const week = periodRange('week', today)
  const month = periodRange('month', today)
  return { from: week.from < month.from ? week.from : month.from, to: month.to }
}

// expectedBusinessHours sums the daily target across Mon-Fri dates in
// [from, to] inclusive. No holiday calendar exists in Mintag, so this is a
// deliberate approximation — it overcounts expected hours around holidays.
export function expectedBusinessHours(from: Date, to: Date): number {
  let hours = 0
  const cur = startOfDay(from)
  const end = startOfDay(to)
  while (cur <= end) {
    if (isBusinessDay(cur)) hours += dailyTargetHours(cur.getDay())
    cur.setDate(cur.getDate() + 1)
  }
  return hours
}

export function filterByRange(activities: DailyActivity[], from: Date, to: Date): DailyActivity[] {
  const lo = toYMD(from)
  const hi = toYMD(to)
  return activities.filter(a => a.date >= lo && a.date <= hi)
}

export function sumHours(activities: DailyActivity[]): number {
  return round1(activities.reduce((sum, a) => sum + a.hours, 0))
}

export interface HoursGroup {
  label: string
  hours: number
  pct: number
}

// groupHours sums hours by project or category; pct is the share of the total
// (1 decimal). Sorted by hours desc, then label.
export function groupHours(activities: DailyActivity[], key: 'project' | 'category'): HoursGroup[] {
  const sums = new Map<string, number>()
  let total = 0
  for (const a of activities) {
    const label = a[key]?.trim() || '(sin asignar)'
    sums.set(label, (sums.get(label) ?? 0) + a.hours)
    total += a.hours
  }
  return Array.from(sums, ([label, hours]) => ({
    label,
    hours: round1(hours),
    pct: total > 0 ? round1((hours / total) * 100) : 0,
  })).sort((a, b) => b.hours - a.hours || a.label.localeCompare(b.label))
}

export interface GapDay {
  date: string
  hours: number
  target: number
}

// gapDays lists business days from `from` up to and including yesterday whose
// logged hours are below that day's target. Today is excluded: it is not over.
export function gapDays(activities: DailyActivity[], from: Date, today: Date): GapDay[] {
  const byDate = new Map<string, number>()
  for (const a of activities) byDate.set(a.date, (byDate.get(a.date) ?? 0) + a.hours)

  const gaps: GapDay[] = []
  const cur = startOfDay(from)
  const end = startOfDay(today)
  while (cur < end) {
    if (isBusinessDay(cur)) {
      const date = toYMD(cur)
      const hours = round1(byDate.get(date) ?? 0)
      const target = dailyTargetHours(cur.getDay())
      if (hours < target) gaps.push({ date, hours, target })
    }
    cur.setDate(cur.getDate() + 1)
  }
  return gaps
}

export interface PendingUpload {
  approvedHours: number
  approvedCount: number
  pendingHours: number
}

export function pendingUpload(activities: DailyActivity[]): PendingUpload {
  const approved = activities.filter(a => a.status === 'approved')
  return {
    approvedHours: sumHours(approved),
    approvedCount: approved.length,
    pendingHours: sumHours(activities.filter(a => a.status === 'pending')),
  }
}

// Mirrors isClosedAzureState in WorkItemsView (azure.IsClosedState in Go).
export function isClosedAzureState(state: string | undefined): boolean {
  return state === 'Closed' || state === 'Cerrado'
}

export function openWorkItems(catalog: AzureActivity[]): AzureActivity[] {
  return catalog.filter(w => w.is_active && !isClosedAzureState(w.last_known_state))
}

// idleWorkItems returns open catalog entries with no activity linked to them
// (activity.azure_activity_id -> catalog id) among `activities`.
export function idleWorkItems(catalog: AzureActivity[], activities: DailyActivity[]): AzureActivity[] {
  const used = new Set(activities.map(a => a.azure_activity_id).filter((id): id is number => id != null))
  return openWorkItems(catalog).filter(w => !used.has(w.id))
}

export interface OverBudgetItem {
  workItemId: number
  label: string
  used: number
  estimate: number
  pct: number
}

// overBudgetWorkItems returns open work items whose used hours (uploaded +
// local) reach `threshold` of the estimate, sorted by usage desc. Catalog rows
// sharing a work item id are reported once.
export function overBudgetWorkItems(
  catalog: AzureActivity[],
  effort: WorkItemEffort[],
  threshold = 0.8,
): OverBudgetItem[] {
  const labels = new Map<number, string>()
  for (const w of openWorkItems(catalog)) {
    if (!labels.has(w.work_item_id)) labels.set(w.work_item_id, w.label)
  }
  const rows: OverBudgetItem[] = []
  const seen = new Set<number>()
  for (const e of effort) {
    if (seen.has(e.id) || !labels.has(e.id) || !e.has_estimate || e.original_estimate <= 0) continue
    seen.add(e.id)
    const used = round1(e.uploaded_hours + e.local_hours)
    const ratio = used / e.original_estimate
    if (ratio < threshold) continue
    rows.push({
      workItemId: e.id,
      label: labels.get(e.id) ?? `#${e.id}`,
      used,
      estimate: e.original_estimate,
      pct: Math.round(ratio * 100),
    })
  }
  return rows.sort((a, b) => b.pct - a.pct || a.workItemId - b.workItemId)
}
