import { useMemo } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { MeetingCard } from './MeetingCard'
import { Button } from '../ui'
import { Upload } from 'lucide-react'

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
    <div style={{ padding: '24px 28px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', marginBottom: 16 }}>
        <Button variant="secondary" onClick={() => openModal('import')}>
          <Upload size={14} strokeWidth={1.75} />
          Import Meeting
        </Button>
      </div>

      {filtered.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '48px 24px', color: 'var(--fg3)' }}>No meetings imported</div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
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
