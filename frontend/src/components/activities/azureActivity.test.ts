import { describe, expect, it } from 'vitest'
import type { AzureActivity } from '../../types'
import { azureActivityMatches, filterAzureActivities } from './azureActivity'

function buildActivity(overrides: Partial<AzureActivity> = {}): AzureActivity {
  return {
    id: 1,
    org: 'my-org',
    work_item_id: 4521,
    label: 'Fix login bug',
    work_item_type: 'Bug',
    is_active: true,
    is_default: false,
    ...overrides,
  }
}

describe('azureActivityMatches', () => {
  it('matches on a case-insensitive label substring', () => {
    const activity = buildActivity({ label: 'Fix login bug', work_item_id: 4521 })

    expect(azureActivityMatches(activity, 'login')).toBe(true)
    expect(azureActivityMatches(activity, 'LOGIN')).toBe(true)
  })

  it('matches on the numeric work item id', () => {
    const activity = buildActivity({ label: 'Fix login bug', work_item_id: 4521 })

    expect(azureActivityMatches(activity, '4521')).toBe(true)
  })

  it('does not match a query absent from label and id', () => {
    const activity = buildActivity({ label: 'Fix login bug', work_item_id: 4521 })

    expect(azureActivityMatches(activity, 'deploy pipeline')).toBe(false)
  })

  it('excludes inactive items even when the query matches', () => {
    const activity = buildActivity({ label: 'Fix login bug', work_item_id: 4521, is_active: false })

    expect(azureActivityMatches(activity, 'login')).toBe(false)
  })
})

describe('filterAzureActivities', () => {
  const catalog: AzureActivity[] = [
    buildActivity({ id: 1, label: 'Fix login bug', work_item_id: 4521, is_active: true }),
    buildActivity({ id: 2, label: 'Deploy pipeline', work_item_id: 9001, is_active: true }),
    buildActivity({ id: 3, label: 'Legacy login cleanup', work_item_id: 4522, is_active: false }),
  ]

  it('returns only active items matching a label substring', () => {
    const result = filterAzureActivities(catalog, 'login')

    expect(result).toHaveLength(1)
    expect(result[0].id).toBe(1)
  })

  it('returns only active items matching a numeric id substring', () => {
    const result = filterAzureActivities(catalog, '4521')

    expect(result).toHaveLength(1)
    expect(result[0].id).toBe(1)
  })

  it('returns an empty result when nothing matches', () => {
    const result = filterAzureActivities(catalog, 'nonexistent query')

    expect(result).toEqual([])
  })

  it('excludes inactive items from an empty-query full listing', () => {
    const result = filterAzureActivities(catalog, '')

    expect(result).toHaveLength(2)
    expect(result.map(a => a.id)).toEqual([1, 2])
  })
})
