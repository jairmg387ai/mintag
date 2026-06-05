import { useMemo } from 'react'
import { Upload } from 'lucide-react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { MeetingCard } from './MeetingCard'
import { Button } from '../ui'

export function MeetingsView() {
  const { meetings, projects, activeProject } = useAppState()
  const { setActiveMeetingId, openModal } = useAppActions()

  const filtered = useMemo(() => {
    if (activeProject != null) return meetings.filter(m => m.project_id === activeProject)
    return meetings
  }, [meetings, activeProject])

  function openMeeting(id: number) {
    setActiveMeetingId(id)
    openModal('meeting')
  }

  return (
    <div className="content-pad">
      {/* Action row */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', marginBottom: 20 }}>
        <Button variant="secondary" size="sm" icon={Upload} onClick={() => openModal('import')}>
          Import Meeting
        </Button>
      </div>

      {/* Empty state */}
      {filtered.length === 0 ? (
        <div
          className="card"
          style={{ textAlign: 'center', padding: '48px 24px', color: 'var(--fg3)' }}
        >
          No meetings yet. Import a transcript to get started.
        </div>
      ) : (
        /* Responsive grid */
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
            gap: 16,
          }}
        >
          {filtered.map(m => {
            const proj = projects.find(p => p.id === m.project_id)
            return (
              <MeetingCard
                key={m.id}
                meeting={m}
                projectColor={proj?.color}
                projectName={proj?.name}
                onClick={() => openMeeting(m.id)}
              />
            )
          })}
        </div>
      )}
    </div>
  )
}
