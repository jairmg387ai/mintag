import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { AzureActivity } from '../../types'
import { AzureActivityCombobox } from './AzureActivityCombobox'

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

const catalog: AzureActivity[] = [
  buildActivity({ id: 1, label: 'Fix login bug', work_item_id: 4521, is_default: true }),
  buildActivity({ id: 2, label: 'Deploy pipeline', work_item_id: 9001 }),
]

function renderCombobox(value = '', azureActivities = catalog) {
  const onChange = vi.fn()
  render(
    <AzureActivityCombobox
      azureActivities={azureActivities}
      value={value}
      onChange={onChange}
      inputStyle={{}}
    />,
  )
  return { onChange }
}

describe('AzureActivityCombobox', () => {
  it('never commits typed query text as a selection', async () => {
    const user = userEvent.setup()
    const { onChange } = renderCombobox()
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })

    await user.click(input)
    await user.type(input, 'login')

    expect(onChange).not.toHaveBeenCalled()
  })

  it('commits the clicked option and closes the list', async () => {
    const user = userEvent.setup()
    const { onChange } = renderCombobox()
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })

    await user.click(input)
    await user.click(screen.getByRole('option', { name: /Deploy pipeline/ }))

    expect(onChange).toHaveBeenCalledWith('2')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('pins the default option first and keeps it visible while filtering', async () => {
    const user = userEvent.setup()
    renderCombobox()
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })

    await user.click(input)
    await user.type(input, 'Deploy')

    const listbox = screen.getByRole('listbox')
    const options = within(listbox).getAllByRole('option')

    expect(options).toHaveLength(2)
    expect(options[0]).toHaveTextContent(/Usar predeterminada/)
    expect(options[1]).toHaveTextContent('Deploy pipeline (#9001)')
  })

  it('selecting the default option clears the assignment', async () => {
    const user = userEvent.setup()
    const { onChange } = renderCombobox('2')
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })

    await user.click(input)
    await user.click(screen.getByRole('option', { name: /Usar predeterminada/ }))

    expect(onChange).toHaveBeenCalledWith('')
  })

  it('closes without changing the value on Escape', async () => {
    const user = userEvent.setup()
    const { onChange } = renderCombobox()
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })

    await user.click(input)
    await user.type(input, 'login')
    await user.keyboard('{Escape}')

    expect(onChange).not.toHaveBeenCalled()
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('closes without changing the value on click outside', async () => {
    const user = userEvent.setup()
    const { onChange } = renderCombobox()
    render(<div data-testid="outside">outside</div>)
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })

    await user.click(input)
    expect(screen.getByRole('listbox')).toBeInTheDocument()

    await user.click(screen.getByTestId('outside'))

    expect(onChange).not.toHaveBeenCalled()
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('commits the arrow-key-highlighted item on Enter', async () => {
    const user = userEvent.setup()
    const { onChange } = renderCombobox()
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })

    await user.click(input)
    await user.keyboard('{ArrowDown}{ArrowDown}{Enter}')

    expect(onChange).toHaveBeenCalledWith('2')
  })

  it('shows the #id fallback label for an assigned value not in the active-only list', () => {
    renderCombobox('99')
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' }) as HTMLInputElement

    expect(input.value).toBe('#99')
  })

  it('shows the full active list plus the default option when focused with an empty query', async () => {
    const user = userEvent.setup()
    renderCombobox()
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })

    await user.click(input)

    const listbox = screen.getByRole('listbox')
    const options = within(listbox).getAllByRole('option')

    expect(options).toHaveLength(3)
    expect(options.map(o => o.textContent)).toEqual([
      expect.stringMatching(/Usar predeterminada/),
      'Fix login bug (#4521)',
      'Deploy pipeline (#9001)',
    ])
  })

  it('shows the parent bug as a tooltip on options and on the selected value', async () => {
    const user = userEvent.setup()
    const withParent = [
      ...catalog,
      buildActivity({ id: 3, label: 'Atención del defecto', work_item_id: 171306, work_item_type: 'Task', parent_work_item_id: 171308, parent_title: 'Login falla', parent_type: 'Bug' }),
    ]
    renderCombobox('3', withParent)
    const input = screen.getByRole('combobox', { name: 'Actividad de Azure' })
    expect(input).toHaveAttribute('title', 'Bug #171308 — Login falla')

    await user.click(input)
    expect(screen.getByRole('option', { name: /Atención del defecto/ })).toHaveAttribute('title', 'Bug #171308 — Login falla')
    expect(screen.getByRole('option', { name: /Deploy pipeline/ })).not.toHaveAttribute('title')
  })
})
