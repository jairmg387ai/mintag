import { useMemo } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { TopBar } from '../layout/TopBar'
import { MeetingCard } from './MeetingCard'

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
    <>
      <TopBar title="Meetings">
        <button
          onClick={() => openModal('import')}
          style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '4px 10px', borderRadius: 7, border: 'none', cursor: 'pointer', fontSize: '0.78em', fontWeight: 500, background: 'var(--color-blue)', color: '#fff' }}
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
            <polyline points="17,8 12,3 7,8"/>
            <line x1="12" y1="3" x2="12" y2="15"/>
          </svg>
          Import Meeting
        </button>
      </TopBar>

      <div style={{ padding: '24px 28px' }}>
        {filtered.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '48px 24px', color: 'var(--color-text3)' }}>No meetings imported</div>
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
    </>
  )
}
