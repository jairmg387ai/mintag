import { useState, useEffect, useMemo } from 'react'
import { ClipboardList, Circle, CircleDot, CircleAlert, CircleCheck, CalendarDays } from 'lucide-react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { StatCard } from '../shared/StatCard'
import { StatusBadge } from '../shared/StatusBadge'
import { PriorityDot } from '../shared/PriorityDot'
import { Avatar } from '../shared/Avatar'
import { useDebounce } from '../../hooks/useDebounce'
import { search } from '../../api/client'
import type { SearchResult, Status, Priority } from '../../types'
import { TopBar } from '../layout/TopBar'
import { Card, CardHeader, Badge, Input } from '../ui'

function fmt(dt: string) {
  if (!dt) return '—'
  try { return new Date(dt).toLocaleString('en-US', { day: '2-digit', month: 'short', year: 'numeric' }) }
  catch { return dt }
}

const SearchIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
  </svg>
)

export function Dashboard() {
  const { stats, tasks, meetings } = useAppState()
  const { setEditingTaskId, openModal, setActiveMeetingId } = useAppActions()
  const [query, setQuery] = useState('')
  const [searchResults, setSearchResults] = useState<SearchResult[]>([])
  const debouncedQuery = useDebounce(query, 400)

  useEffect(() => {
    if (!debouncedQuery || debouncedQuery.length < 2) { setSearchResults([]); return }
    search(debouncedQuery).then(setSearchResults).catch(() => setSearchResults([]))
  }, [debouncedQuery])

  const activeTasks = useMemo(
    () => tasks.filter(t => t.status === 'blocked' || t.status === 'in_progress').slice(0, 8),
    [tasks]
  )
  const recentMeetings = useMemo(
    () => [...meetings].sort((a, b) => b.id - a.id).slice(0, 5),
    [meetings]
  )

  function openTask(id: number) { setEditingTaskId(id); openModal('task') }
  function openMeeting(id: number) { setActiveMeetingId(id); openModal('meeting') }

  return (
    <>
      <TopBar title="Dashboard">
        <div className="relative flex-1 max-w-[380px]">
          <Input
            prefix={<SearchIcon />}
            type="text"
            placeholder="Search tasks and meetings..."
            value={query}
            onChange={e => setQuery(e.target.value)}
          />
        </div>
      </TopBar>

      <div className="p-7">
        {/* Stats */}
        <div className="grid gap-3.5 mb-6" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))' }}>
          <StatCard icon={ClipboardList} iconBg="var(--indigo-50)"   iconFg="var(--indigo-700)"  value={stats?.total_tasks ?? 0}       label="Total Tasks"  onClick={() => {}} />
          <StatCard icon={Circle}        iconBg="var(--slate-100)"   iconFg="var(--slate-600)"   value={stats?.todo_tasks ?? 0}        label="To Do"        onClick={() => {}} />
          <StatCard icon={CircleDot}     iconBg="var(--amber-50)"    iconFg="var(--amber-700)"   value={stats?.in_progress_tasks ?? 0} label="In Progress"  onClick={() => {}} />
          <StatCard icon={CircleAlert}   iconBg="var(--rose-50)"     iconFg="var(--rose-700)"    value={stats?.blocked_tasks ?? 0}     label="Blocked"      onClick={() => {}} emphasize={!!stats?.blocked_tasks} />
          <StatCard icon={CircleCheck}   iconBg="var(--emerald-50)"  iconFg="var(--emerald-700)" value={stats?.done_tasks ?? 0}        label="Done"         onClick={() => {}} />
          <StatCard icon={CalendarDays}  iconBg="var(--indigo-50)"   iconFg="var(--indigo-700)"  value={stats?.total_meetings ?? 0}    label="Meetings"     onClick={() => {}} />
        </div>

        {/* Search results */}
        {searchResults.length > 0 && (
          <Card className="mb-5">
            <CardHeader>Search results: "{query}"</CardHeader>
            <div className="p-2">
              {searchResults.map(r => (
                <div
                  key={`${r.kind}-${r.id}`}
                  onClick={() => r.kind === 'task' ? openTask(r.id) : openMeeting(r.id)}
                  className="row-clickable flex flex-col gap-1 px-3 py-2.5"
                >
                  <div className="flex items-center gap-2">
                    <Badge>{r.kind === 'task' ? 'Task' : 'Meeting'}</Badge>
                    <strong className="text-[0.88em]">{r.title}</strong>
                  </div>
                  <div className="text-[0.78em] text-text3 pl-0.5">{r.snippet}</div>
                </div>
              ))}
            </div>
          </Card>
        )}

        {/* Content grid */}
        <div className="grid grid-cols-2 gap-[18px]">
          {/* Active tasks */}
          <Card>
            <CardHeader icon={
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10"/><polyline points="12,6 12,12 16,14"/>
              </svg>
            }>
              Blocked / In Progress
            </CardHeader>
            {activeTasks.length === 0 ? (
              <div className="text-center py-12 text-text3">No active tasks</div>
            ) : (
              activeTasks.map(t => (
                <div
                  key={t.id}
                  onClick={() => openTask(t.id)}
                  className="row-clickable grid items-center gap-3 px-4 py-3"
                  style={{ gridTemplateColumns: '28px 1fr 110px 90px' }}
                >
                  <PriorityDot priority={t.priority as Priority} />
                  <div>
                    <div className="text-[0.9em] font-medium truncate">{t.title}</div>
                    <div className="text-[0.78em] text-text3 truncate">{t.project_name ?? ''}</div>
                  </div>
                  <StatusBadge status={t.status as Status} />
                  {t.owner ? <Avatar name={t.owner} /> : <span className="text-text3 text-[0.78em]">—</span>}
                </div>
              ))
            )}
          </Card>

          {/* Recent meetings */}
          <Card>
            <CardHeader icon={
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/>
              </svg>
            }>
              Recent Meetings
            </CardHeader>
            <div className="px-4">
              {recentMeetings.length === 0 ? (
                <div className="text-center py-9 text-text3">No meetings</div>
              ) : (
                recentMeetings.map(m => (
                  <div
                    key={m.id}
                    onClick={() => openMeeting(m.id)}
                    className="row-clickable flex items-center gap-2.5 px-0 py-3"
                  >
                    <span className="text-[0.72em] text-text3 min-w-[80px]">{fmt(m.date) || '—'}</span>
                    <span className="text-[0.85em] flex-1 truncate">{m.title}</span>
                    <Badge>{m.task_count ?? 0} tasks</Badge>
                  </div>
                ))
              )}
            </div>
          </Card>
        </div>
      </div>
    </>
  )
}
