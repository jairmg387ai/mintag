import { useAppState, useAppActions } from '../../store/AppContext'
import { FilterChip } from '../shared/FilterChip'

const FILTERS = [
  { value: 'all', label: 'All' },
  { value: 'todo', label: 'To Do' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'blocked', label: 'Blocked' },
  { value: 'done', label: 'Done' },
]

export function FilterBar() {
  const { filterStatus } = useAppState()
  const { setFilterStatus } = useAppActions()

  function toggle(value: string) {
    setFilterStatus(filterStatus === value ? 'all' : value)
  }

  return (
    <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 16 }}>
      {FILTERS.map(f => (
        <FilterChip
          key={f.value}
          label={f.label}
          active={filterStatus === f.value}
          onClick={() => toggle(f.value)}
        />
      ))}
    </div>
  )
}
