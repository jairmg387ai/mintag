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
  it('blanks both fields when no work item is selected', () => {
    const result = resolveAutofill({
      activity: undefined,
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: '', category: '' })
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

  it('fills project and blanks category for a project-only-mapped work item', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Alpha', category_id: null }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: 'Alpha', category: '' })
  })

  it('fills category and blanks project for a category-only-mapped work item', () => {
    const result = resolveAutofill({
      activity: workItem({ project: null, category_id: 2 }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: '', category: 'Feature' })
  })

  it('blanks both fields for a fully-unmapped work item', () => {
    const result = resolveAutofill({
      activity: workItem({ project: null, category_id: null }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: '', category: '' })
  })

  it('blanks a field that was previously autofilled once the user switches to a work item that no longer maps it', () => {
    // Regression: the caller applies each call's patch directly to state, so
    // this simulates the exact sequence a user triggers by picking a
    // fully-mapped work item and then switching to an unmapped one — the
    // second call must clear the first call's autofill, not leave it stale.
    const firstPick = resolveAutofill({
      activity: workItem({ project: 'Alpha', category_id: 1 }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(firstPick).toEqual({ project: 'Alpha', category: 'Bug' })

    const secondPick = resolveAutofill({
      activity: workItem({ project: null, category_id: null }),
      catalog,
      projectTouched: false, // never hand-edited — only ever set by the first pick's autofill
      categoryTouched: false,
    })
    expect(secondPick).toEqual({ project: '', category: '' })
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

  it('treats a blank project string on the work item as unset and blanks the field', () => {
    const result = resolveAutofill({
      activity: workItem({ project: '   ', category_id: 1 }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: '', category: 'Bug' })
  })

  it('blanks project when the mapped project is absent from the loaded catalog', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Gamma', category_id: 1 }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: '', category: 'Bug' })
  })

  it('blanks category when the mapped category_id is dangling (hard-deleted category)', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Alpha', category_id: 999 }),
      catalog,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: 'Alpha', category: '' })
  })

  it('emits the raw project string as-is in free-text mode (catalog === null), skipping membership checks', () => {
    const result = resolveAutofill({
      activity: workItem({ project: 'Gamma', category_id: null }),
      catalog: null,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: 'Gamma', category: '' })
  })

  it('blanks category in free-text mode (catalog === null) because there is no id-to-name source', () => {
    const result = resolveAutofill({
      activity: workItem({ project: null, category_id: 1 }),
      catalog: null,
      projectTouched: false,
      categoryTouched: false,
    })
    expect(result).toEqual({ project: '', category: '' })
  })
})
