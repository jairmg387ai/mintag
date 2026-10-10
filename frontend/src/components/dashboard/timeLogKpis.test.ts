import { describe, expect, it } from 'vitest'
import type { AzureActivity, DailyActivity, WorkItemEffort } from '../../types'
import {
  dailyTargetHours,
  expectedBusinessHours,
  filterByRange,
  gapDays,
  groupHours,
  idleWorkItems,
  loadRange,
  overBudgetWorkItems,
  pendingUpload,
  periodRange,
  sumHours,
  toYMD,
} from './timeLogKpis'

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

function wi(id: number, workItemId: number, extra: Partial<AzureActivity> = {}): AzureActivity {
  return {
    id,
    org: 'ORG',
    work_item_id: workItemId,
    label: `WI ${workItemId}`,
    work_item_type: 'Task',
    is_active: true,
    is_default: false,
    ...extra,
  }
}

// Local dates (month is 0-based) so tests do not depend on the runner TZ.
const d = (y: number, m: number, day: number) => new Date(y, m - 1, day)

describe('dailyTargetHours', () => {
  it('is 8h Mon-Thu and 7.5h on Friday', () => {
    expect(dailyTargetHours(1)).toBe(8)
    expect(dailyTargetHours(4)).toBe(8)
    expect(dailyTargetHours(5)).toBe(7.5)
  })
})

describe('periodRange', () => {
  it('week runs from Monday to today', () => {
    const r = periodRange('week', d(2026, 10, 8)) // Thursday
    expect(toYMD(r.from)).toBe('2026-10-05')
    expect(toYMD(r.to)).toBe('2026-10-08')
  })

  it('week starting on Monday is just today', () => {
    const r = periodRange('week', d(2026, 10, 5))
    expect(toYMD(r.from)).toBe('2026-10-05')
    expect(toYMD(r.to)).toBe('2026-10-05')
  })

  it('on Sunday the week starts on the previous Monday', () => {
    const r = periodRange('week', d(2026, 10, 11))
    expect(toYMD(r.from)).toBe('2026-10-05')
  })

  it('month runs from the 1st to today', () => {
    const r = periodRange('month', d(2026, 10, 8))
    expect(toYMD(r.from)).toBe('2026-10-01')
    expect(toYMD(r.to)).toBe('2026-10-08')
  })
})

describe('loadRange', () => {
  it('uses the month start when the week is inside the month', () => {
    expect(toYMD(loadRange(d(2026, 10, 8)).from)).toBe('2026-10-01')
  })

  it('uses the week start when the week crosses the month boundary', () => {
    const r = loadRange(d(2026, 10, 1)) // Thursday; week began Mon 2026-09-28
    expect(toYMD(r.from)).toBe('2026-09-28')
    expect(toYMD(r.to)).toBe('2026-10-01')
  })
})

describe('expectedBusinessHours', () => {
  it('sums Mon-Thu 8h and Fri 7.5h, skipping weekends', () => {
    // Mon 2026-10-05 .. Sun 2026-10-11 = 4*8 + 7.5
    expect(expectedBusinessHours(d(2026, 10, 5), d(2026, 10, 11))).toBe(39.5)
  })
})

describe('filterByRange / sumHours', () => {
  it('keeps only activities inside the inclusive range', () => {
    const list = [act('2026-09-30', 1), act('2026-10-01', 2), act('2026-10-08', 3), act('2026-10-09', 4)]
    const inRange = filterByRange(list, d(2026, 10, 1), d(2026, 10, 8))
    expect(inRange.map(a => a.date)).toEqual(['2026-10-01', '2026-10-08'])
    expect(sumHours(inRange)).toBe(5)
  })
})

describe('groupHours', () => {
  it('sums by key, computes percentage and sorts desc', () => {
    const rows = groupHours(
      [
        act('2026-10-05', 1, { category: 'Reuniones' }),
        act('2026-10-05', 2.5, { category: 'Desarrollo' }),
        act('2026-10-06', 0.5, { category: 'Reuniones' }),
      ],
      'category',
    )
    expect(rows).toEqual([
      { label: 'Desarrollo', hours: 2.5, pct: 62.5 },
      { label: 'Reuniones', hours: 1.5, pct: 37.5 },
    ])
  })

  it('returns an empty list for no activities', () => {
    expect(groupHours([], 'project')).toEqual([])
  })

  it('labels empty keys', () => {
    expect(groupHours([act('2026-10-05', 1, { project: '' })], 'project')[0].label).toBe('(sin asignar)')
  })
})

describe('gapDays', () => {
  it('lists business days before today under the daily target, Friday at 7.5h', () => {
    const list = [
      act('2026-10-05', 8), // Mon ok
      act('2026-10-06', 6), // Tue short
      act('2026-10-08', 4), // Thu short
      act('2026-10-09', 7.5), // Fri ok at 7.5h
      act('2026-10-12', 3), // Mon = today, excluded
    ]
    expect(gapDays(list, d(2026, 10, 5), d(2026, 10, 12))).toEqual([
      { date: '2026-10-06', hours: 6, target: 8 },
      { date: '2026-10-07', hours: 0, target: 8 },
      { date: '2026-10-08', hours: 4, target: 8 },
    ])
  })

  it('flags a Friday below 7.5h', () => {
    expect(gapDays([act('2026-10-09', 7)], d(2026, 10, 9), d(2026, 10, 10))).toEqual([
      { date: '2026-10-09', hours: 7, target: 7.5 },
    ])
  })

  it('is empty when the period starts today', () => {
    expect(gapDays([], d(2026, 10, 5), d(2026, 10, 5))).toEqual([])
  })
})

describe('pendingUpload', () => {
  it('sums approved and unapproved hours, ignoring uploaded', () => {
    expect(
      pendingUpload([
        act('2026-10-05', 2, { status: 'approved' }),
        act('2026-10-06', 1.5, { status: 'approved' }),
        act('2026-10-06', 3, { status: 'pending' }),
        act('2026-10-06', 8, { status: 'uploaded' }),
      ]),
    ).toEqual({ approvedHours: 3.5, approvedCount: 2, pendingHours: 3 })
  })
})

describe('idleWorkItems', () => {
  it('returns active, non-closed catalog items with no linked activity', () => {
    const catalog = [
      wi(1, 101),
      wi(2, 102),
      wi(3, 103, { is_active: false }),
      wi(4, 104, { last_known_state: 'Closed' }),
      wi(5, 105, { last_known_state: 'Cerrado' }),
    ]
    const week = [act('2026-10-05', 2, { azure_activity_id: 1 })]
    expect(idleWorkItems(catalog, week).map(w => w.id)).toEqual([2])
  })

  it('groups catalog rows by work item id', () => {
    const catalog = [
      wi(1, 101),
      wi(2, 101), // sibling row of 101 received the hours
      wi(3, 102),
      wi(4, 102), // 102 has no hours on any row: listed once
      wi(5, 103, { is_active: false }),
      wi(6, 103), // hours went to the inactive sibling row
    ]
    const week = [
      act('2026-10-05', 2, { azure_activity_id: 2 }),
      act('2026-10-06', 1, { azure_activity_id: 5 }),
    ]
    expect(idleWorkItems(catalog, week).map(w => w.work_item_id)).toEqual([102])
  })
})

describe('overBudgetWorkItems', () => {
  it('returns items at >= 80% of estimate sorted by ratio desc', () => {
    const catalog = [wi(1, 101), wi(2, 102), wi(3, 103), wi(4, 104), wi(5, 104)]
    const effort: WorkItemEffort[] = [
      { id: 101, original_estimate: 10, uploaded_hours: 6, local_hours: 2, remaining: 2, has_estimate: true },
      { id: 102, original_estimate: 10, uploaded_hours: 5, local_hours: 2, remaining: 3, has_estimate: true },
      { id: 103, original_estimate: 0, uploaded_hours: 50, local_hours: 0, remaining: 0, has_estimate: false },
      { id: 104, original_estimate: 4, uploaded_hours: 4, local_hours: 1, remaining: 0, has_estimate: true },
    ]
    const rows = overBudgetWorkItems(catalog, effort)
    expect(rows.map(r => [r.workItemId, r.used, r.estimate, r.pct])).toEqual([
      [104, 5, 4, 125],
      [101, 8, 10, 80],
    ])
    expect(rows[0].label).toBe('WI 104')
  })
})
