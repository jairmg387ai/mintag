import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { WorkItemEffort as WorkItemEffortData } from '../../types'
import { WorkItemEffort } from './WorkItemEffort'

function effort(overrides: Partial<WorkItemEffortData> = {}): WorkItemEffortData {
  return {
    id: 1,
    original_estimate: 24,
    uploaded_hours: 18,
    local_hours: 2.5,
    remaining: 3.5,
    has_estimate: true,
    ...overrides,
  }
}

describe('WorkItemEffort', () => {
  it('renders estimate, registered, pending upload and remaining hours', () => {
    render(<WorkItemEffort effort={effort()} />)

    expect(screen.getByTestId('work-item-effort')).toHaveTextContent(
      'Estimado 24h · Registrado 18h (+2.5h sin subir) · Quedan 3.5h',
    )
  })

  it('omits the pending-upload part when there are no local hours', () => {
    render(<WorkItemEffort effort={effort({ local_hours: 0, remaining: 6 })} />)

    expect(screen.getByTestId('work-item-effort')).toHaveTextContent('Estimado 24h · Registrado 18h · Quedan 6h')
  })

  it('renders the no-estimate variant', () => {
    render(<WorkItemEffort effort={effort({ has_estimate: false, original_estimate: 0, local_hours: 0, remaining: 0 })} />)

    const el = screen.getByTestId('work-item-effort')
    expect(el).toHaveTextContent('Sin estimado · Registrado 18h')
    expect(el).not.toHaveTextContent('Quedan')
  })

  it('flags remaining hours as a warning when under 20% of the estimate', () => {
    render(<WorkItemEffort effort={effort()} />)

    expect(screen.getByText('Quedan 3.5h')).toHaveAttribute('data-warning', 'true')
  })

  it('does not flag remaining hours when there is plenty left', () => {
    render(<WorkItemEffort effort={effort({ uploaded_hours: 4, local_hours: 0, remaining: 20 })} />)

    expect(screen.getByText('Quedan 20h')).toHaveAttribute('data-warning', 'false')
  })

  it('warns when the entered hours exceed the remaining hours', () => {
    render(<WorkItemEffort effort={effort()} enteredHours={4} />)

    expect(screen.getByText('Estas horas superan lo que queda en el work item (quedan 3.5h).')).toBeInTheDocument()
  })

  it('does not warn when the entered hours fit, or when there is no estimate', () => {
    const { rerender } = render(<WorkItemEffort effort={effort()} enteredHours={3.5} />)
    expect(screen.queryByText(/superan lo que queda/i)).not.toBeInTheDocument()

    rerender(<WorkItemEffort effort={effort({ has_estimate: false, remaining: 0 })} enteredHours={4} />)
    expect(screen.queryByText(/superan lo que queda/i)).not.toBeInTheDocument()
  })
})
