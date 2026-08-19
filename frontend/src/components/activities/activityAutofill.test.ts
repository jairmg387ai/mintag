import { describe, expect, it } from 'vitest'
import { resolveAutofill } from './activityAutofill'
import type { ActivityCatalog, AzureActivity } from '../../types'

const catalog: ActivityCatalog = {
  projects: ['Alpha', 'Beta'],
  categories: [
    { id: 1, name: 'Bug' },
    { id: 2, name: 'Feature' },
  ],
}

function workItem(overrides: Partial<AzureActivity> = {}): AzureActivity {
  return {
    id: 10,
    org: 'acme',
    work_item_id: 123,
    label: 'Fix crash',
    work_item_type: 'Bug',
    is_active: true,
    is_default: false,
    project: null,
    category_id: null,
    ...overrides,
  }
}

describe('resolveAutofill', () => {
  it('returns an empty patch when no work item is selected', () => {
    const result = resolveAutofill({
      activity: undefined,
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({})
  })

  it('fills both project and category for a fully-mapped work item', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Alpha', category_id: 1 }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: 'Alpha', category: 'Bug' })
  })

  it('fills only project for a project-only-mapped work item, leaving category untouched', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Alpha', category_id: null }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: 'Alpha' })
    expect(result.category).toBeUndefined()
  })

  it('fills only category for a category-only-mapped work item, leaving project untouched', () => {
    const result = resolveAutofill({
      activity: workItem({ project: null, category_id: 2 }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ category: 'Feature' })
    expect(result.project).toBeUndefined()
  })

  it('returns an empty patch for a fully-unmapped work item', () => {
    const result = resolveAutofill({
      activity: workItem({ project: null, category_id: null }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({})
  })

  it('skips project when the project field was manually touched, even if mapped', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Alpha', category_id: 1 }),
      catalog,
      projectTouched: true,
      categoryTouched: false,
    })
    expect(result).toEqual({ category: 'Bug' })
    expect(result.project).toBeUndefined()
  })

  it('skips category when the category field was manually touched, even if mapped', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Alpha', category_id: 1 }),
      catalog,
      projectTouched: false,
      categoryTouched: true,
    })
    expect(result).toEqual({ project: 'Alpha' })
    expect(result.category).toBeUndefined()
  })

  it('returns an empty patch when both fields are touched', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Alpha', category_id: 1 }),
      catalog,
      projectTouched: true,
      categoryTouched: true,
    })
    expect(result).toEqual({})
  })

  it('treats a blank project string on the work item as unset', () => {
    const result = resolveAutofill({
      activity: workItem({ project: '   ', category_id: 1 }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ category: 'Bug' })
    expect(result.project).toBeUndefined()
  })

  it('skips project when the mapped project is absent from the loaded catalog', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Gamma', category_id: 1 }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ category: 'Bug' })
    expect(result.project).toBeUndefined()
  })

  it('skips category when the mapped category_id is dangling (hard-deleted category)', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Alpha', category_id: 999 }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: 'Alpha' })
    expect(result.category).toBeUndefined()
  })

  it('emits the raw project string as-is in free-text mode (catalog === null), skipping membership checks', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Gamma', category_id: null }),
      catalog: null,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: 'Gamma' })
  })

  it('skips category in free-text mode (catalog === null) because there is no id-to-name source', () => {
    const result = resolveAutofill({
      activity: workItem({ project: null, category_id: 1 }),
      catalog: null,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({})
    expect(result.category).toBeUndefined()
  })
})
