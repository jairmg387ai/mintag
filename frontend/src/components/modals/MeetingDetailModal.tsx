import { useEffect, useState } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { Modal, BtnGhost } from './Modal'
import { StatusBadge } from '../shared/StatusBadge'
import { PriorityDot } from '../shared/PriorityDot'
import { Avatar } from '../shared/Avatar'
import { getMeeting } from '../../api/client'
import type { Meeting, Task, Status, Priority } from '../../types'
import { useToast } from '../../hooks/useToast'

export function MeetingDetailModal() {
  const { activeMeetingId, tasks, projects } = useAppState()
  const { closeModal, setEditingTaskId, openModal } = useAppActions()
  const { pushToast } = useToast()

  const [meeting, setMeeting] = useState<Meeting | null>(null)
  const [showTranscript, setShowTranscript] = useState(false)

  useEffect(() => {
    if (!activeMeetingId) return
    setShowTranscript(false)
    getMeeting(activeMeetingId).then(setMeeting).catch(e => {
      pushToast(e instanceof Error ? e.message : 'Error', true)
      closeModal()
    })
  }, [activeMeetingId])

  if (!meeting) return null

  const meetingTasks: Task[] = tasks.filter(t => t.meeting_id === meeting.id)
  const project = projects.find(p => p.id === meeting.project_id)
  const hasTranscript = (meeting.raw_content ?? '').trim().length > 0

  function openTask(id: number) {
    closeModal()
    setTimeout(() => {
      setEditingTaskId(id)
      openModal('task')
    }, 50)
  }

  return (
    <Modal
      title={meeting.title}
      maxWidth={720}
      footer={<BtnGhost onClick={closeModal}>Close</BtnGhost>}
    >
      {/* Meta grid */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginBottom: 16 }}>
        <MetaItem label="Date" value={meeting.date || '—'} />
        <MetaItem label="Project" value={project?.name ?? '—'} />
        <div style={{ gridColumn: '1 / -1' }}>
          <MetaItem label="File" value={meeting.filename || '—'} small />
        </div>
      </div>

      {/* Summary */}
      <div style={{ background: 'rgba(59,130,246,0.12)', borderLeft: '3px solid var(--color-blue)', borderRadius: '0 8px 8px 0', padding: '12px 14px', marginBottom: 16, fontSize: '0.88em', color: 'var(--color-text2)', lineHeight: 1.6 }}>
        {meeting.summary || <span style={{ color: 'var(--color-text3)' }}>No summary available</span>}
      </div>

      {/* Tasks */}
      <div style={{ fontSize: '0.8em', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.5px', color: 'var(--color-text3)', marginBottom: 12 }}>
        Associated Tasks ({meetingTasks.length})
      </div>
      {meetingTasks.length === 0 ? (
        <div style={{ fontSize: '0.82em', color: 'var(--color-text3)', marginBottom: 12 }}>No associated tasks</div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginBottom: 16 }}>
          {meetingTasks.map(t => (
            <div
              key={t.id}
              onClick={() => openTask(t.id)}
              style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 12px', background: 'var(--color-surface2)', borderRadius: 7, fontSize: '0.85em', cursor: 'pointer' }}
            >
              <PriorityDot priority={t.priority as Priority} />
              <div style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.title}</div>
              <StatusBadge status={t.status as Status} />
              {t.owner && <Avatar name={t.owner} />}
            </div>
          ))}
        </div>
      )}

      {/* Transcript toggle */}
      {hasTranscript && (
        <div style={{ marginTop: 16 }}>
          <div style={{ fontSize: '0.8em', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.5px', color: 'var(--color-text3)', marginBottom: 8 }}>Transcript</div>
          <button
            onClick={() => setShowTranscript(v => !v)}
            style={{ background: 'none', border: '1px solid var(--color-border)', borderRadius: 7, color: 'var(--color-text2)', cursor: 'pointer', padding: '6px 14px', fontSize: '0.8em', width: '100%', textAlign: 'left', marginBottom: 8 }}
          >
            {showTranscript ? '▲ Hide transcript' : '▼ Show transcript'}
          </button>
          {showTranscript && (
            <div style={{ fontSize: '0.78em', color: 'var(--color-text3)', lineHeight: 1.7, whiteSpace: 'pre-wrap', background: 'var(--color-surface2)', borderRadius: 7, padding: 12, maxHeight: 400, overflowY: 'auto' }}>
              {meeting.raw_content}
            </div>
          )}
        </div>
      )}
    </Modal>
  )
}

function MetaItem({ label, value, small }: { label: string; value: string; small?: boolean }) {
  return (
    <div style={{ background: 'var(--color-surface2)', borderRadius: 8, padding: 12 }}>
      <div style={{ fontSize: '0.72em', textTransform: 'uppercase', letterSpacing: '0.5px', color: 'var(--color-text3)', marginBottom: 3 }}>{label}</div>
      <div style={{ fontWeight: 600, fontSize: small ? '0.78em' : '0.88em', color: 'var(--color-text)', wordBreak: 'break-all' }}>{value}</div>
    </div>
  )
}
