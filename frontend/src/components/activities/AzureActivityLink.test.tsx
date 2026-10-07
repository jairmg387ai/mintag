import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { AzureActivity } from '../../types'
import { AzureActivityReference } from './AzureActivityLink'

function buildActivity(overrides: Partial<AzureActivity> = {}): AzureActivity {
  return {
    id: 1,
    org: 'my-org',
    work_item_id: 171306,
    label: 'Atención del defecto',
    work_item_type: 'Task',
    is_active: true,
    is_default: true,
    ...overrides,
  }
}

describe('AzureActivityReference', () => {
  it('adds the parent bug to the link tooltip when known', () => {
    const activity = buildActivity({ parent_work_item_id: 171308, parent_title: 'Login falla', parent_type: 'Bug' })
    render(<AzureActivityReference azureActivityId={1} azureActivities={[activity]} />)

    const link = screen.getByRole('link')
    expect(link).toHaveAttribute('title', 'Abrir work item de Azure #171306\nBug #171308 — Login falla')
  })

  it('keeps the plain tooltip when there is no parent', () => {
    render(<AzureActivityReference azureActivityId={1} azureActivities={[buildActivity()]} />)

    expect(screen.getByRole('link')).toHaveAttribute('title', 'Abrir work item de Azure #171306')
  })
})
