import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import type { DailyActivity } from '../../types'
import { HoursBreakdown } from './HoursBreakdown'

function act(hours: number, project: string, category: string): DailyActivity {
  return {
    id: Math.floor(Math.random() * 1e9),
    date: '2026-10-05',
    hours,
    project,
    category,
    registro_diario: '',
    source: 'manual',
    status: 'pending',
    created_at: '',
  }
}

describe('HoursBreakdown', () => {
  it('lists categories and projects sorted by hours with percentage', () => {
    render(
      <HoursBreakdown
        activities={[
          act(1, 'Mintag', 'Reuniones'),
          act(3, 'RUNT', 'Desarrollo'),
          act(1, 'RUNT', 'Reuniones'),
        ]}
      />,
    )

    const cats = within(screen.getByRole('list', { name: 'Horas por categoría' })).getAllByRole('listitem')
    expect(cats).toHaveLength(2)
    expect(cats[0]).toHaveTextContent('Desarrollo')
    expect(cats[0]).toHaveTextContent('3.0h')
    expect(cats[0]).toHaveTextContent('60%')
    expect(cats[1]).toHaveTextContent('Reuniones')
    expect(cats[1]).toHaveTextContent('2.0h')
    expect(cats[1]).toHaveTextContent('40%')

    const projects = within(screen.getByRole('list', { name: 'Horas por proyecto' })).getAllByRole('listitem')
    expect(projects.map(li => li.textContent)).toEqual([
      expect.stringContaining('RUNT'),
      expect.stringContaining('Mintag'),
    ])
  })

  it('shows the empty state when there are no hours', () => {
    render(<HoursBreakdown activities={[]} />)
    expect(screen.getAllByText('Sin horas registradas en este período')).toHaveLength(2)
    expect(screen.queryByRole('listitem')).toBeNull()
  })
})
