import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeHighlight from 'rehype-highlight'
import DOMPurify from 'dompurify'
import 'highlight.js/styles/github-dark.css'
import { useAppState, useAppActions } from '../../store/AppContext'
import { Modal } from './Modal'
import { Button } from '../ui/Button'
import { StatusBadge } from '../shared/StatusBadge'
import { PriorityTag } from '../shared/PriorityTag'
import { Avatar } from '../shared/Avatar'
import { Pencil } from 'lucide-react'
import { getMeeting, updateMeetingSummary } from '../../api/client'
import type { Meeting, Task, Status, Priority } from '../../types'
import { useToast } from '../../hooks/useToast'

type ActiveTab = 'summary' | 'transcript' | 'rich'

export function MeetingDetailModal() {
  const { activeMeetingId, tasks, projects } = useAppState()
  const { closeModal, setEditingTaskId, openModal } = useAppActions()
  const { pushToast } = useToast()

  const [meeting, setMeeting] = useState<Meeting | null>(null)
  const [activeTab, setActiveTab] = useState<ActiveTab>('summary')
  const [editSummary, setEditSummary] = useState(false)
  const [summaryDraft, setSummaryDraft] = useState('')

  useEffect(() => {
    if (!activeMeetingId) return
    setActiveTab('summary')
    setEditSummary(false)
    getMeeting(activeMeetingId).then(m => {
      setMeeting(m)
      setSummaryDraft(m.summary ?? '')
    }).catch(e => {
      pushToast(e instanceof Error ? e.message : 'Error', true)
      closeModal()
    })
  }, [activeMeetingId])

  useEffect(() => {
    if (meeting) {
      setSummaryDraft(meeting.summary ?? '')
    }
  }, [meeting])

  if (!meeting) return null

  const meetingTasks: Task[] = tasks.filter(t => t.meeting_id === meeting.id)
  const project = projects.find(p => p.id === meeting.project_id)
  const hasTranscript = (meeting.raw_content ?? '').trim().length > 0
  const hasRich = meeting.content_type === 'markdown' || meeting.content_type === 'html'

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
      footer={<Button variant="ghost" onClick={closeModal}>Close</Button>}
    >
      {/* Meta grid */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: 12,
          marginBottom: 16,
        }}
      >
        <MetaItem label="Date" value={meeting.date || '—'} />
        <MetaItem label="Project" value={project?.name ?? '—'} />
        <div style={{ gridColumn: '1 / -1' }}>
          <MetaItem label="File" value={meeting.filename || '—'} small />
        </div>
      </div>

      {/* Tab row */}
      <div
        style={{
          display: 'flex',
          gap: 4,
          marginBottom: 16,
          borderBottom: '1px solid var(--border)',
          paddingBottom: 0,
        }}
      >
        <TabButton
          label="Summary"
          active={activeTab === 'summary'}
          onClick={() => setActiveTab('summary')}
        />
        {hasTranscript && (
          <TabButton
            label="Transcript"
            active={activeTab === 'transcript'}
            onClick={() => setActiveTab('transcript')}
          />
        )}
        {hasRich && (
          <TabButton
            label="Rich Content"
            active={activeTab === 'rich'}
            onClick={() => setActiveTab('rich')}
          />
        )}
      </div>

      {/* Summary tab */}
      {activeTab === 'summary' && (
        <>
          {/* Summary block */}
          <div
            style={{
              background: 'var(--bg-sunken)',
              borderLeft: '3px solid var(--brand)',
              borderRadius: '0 var(--radius-md) var(--radius-md) 0',
              padding: '12px 14px',
              marginBottom: 16,
              font: 'var(--text-body)',
              color: 'var(--fg2)',
              lineHeight: 1.6,
            }}
          >
            {editSummary ? (
              <div>
                <textarea
                  value={summaryDraft}
                  onChange={e => setSummaryDraft(e.target.value)}
                  style={{
                    width: '100%',
                    padding: '8px 12px',
                    border: '1px solid var(--border-strong)',
                    borderRadius: 'var(--radius-md)',
                    font: 'var(--text-body)',
                    color: 'var(--fg1)',
                    background: 'var(--bg-sunken)',
                    outline: 'none',
                    resize: 'vertical',
                    minHeight: 120,
                    fontFamily: 'inherit',
                    boxSizing: 'border-box',
                    marginBottom: 8,
                  }}
                />
                <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => { setEditSummary(false); setSummaryDraft(meeting.summary ?? '') }}
                  >
                    Cancel
                  </button>
                  <button
                    className="btn btn-primary btn-sm"
                    onClick={async () => {
                      try {
                        const updated = await updateMeetingSummary(meeting.id, summaryDraft)
                        setMeeting(updated)
                        setEditSummary(false)
                      } catch (e: unknown) {
                        pushToast(e instanceof Error ? e.message : 'Error saving summary', true)
                      }
                    }}
                  >
                    Save
                  </button>
                </div>
              </div>
            ) : (
              <div style={{ position: 'relative' }}>
                <button
                  onClick={() => setEditSummary(true)}
                  style={{
                    position: 'absolute',
                    top: 0,
                    right: 0,
                    background: 'none',
                    border: 'none',
                    color: 'var(--fg3)',
                    cursor: 'pointer',
                    padding: 4,
                    borderRadius: 'var(--radius-md)',
                    display: 'flex',
                    alignItems: 'center',
                  }}
                  title="Edit summary"
                  aria-label="Edit summary"
                >
                  <Pencil size={14} strokeWidth={1.75} />
                </button>
                {meeting.summary ? (
                  <div className="prose prose-invert max-w-none" style={{ paddingRight: 24 }}>
                    <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeHighlight]}>
                      {meeting.summary}
                    </ReactMarkdown>
                  </div>
                ) : (
                  <span style={{ color: 'var(--fg3)' }}>No summary available — click the pencil to add one</span>
                )}
              </div>
            )}
          </div>

          {/* Action items section */}
          <div className="label" style={{ marginBottom: 12 }}>
            Associated Tasks ({meetingTasks.length})
          </div>
          {meetingTasks.length === 0 ? (
            <div style={{ font: 'var(--text-sm)', color: 'var(--fg3)', marginBottom: 12 }}>
              No associated tasks
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginBottom: 16 }}>
              {meetingTasks.map(t => (
                <button
                  key={t.id}
                  onClick={() => openTask(t.id)}
                  className="card"
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 10,
                    padding: '10px 14px',
                    font: 'var(--text-body)',
                    cursor: 'pointer',
                    width: '100%',
                    textAlign: 'left',
                    border: 'none',
                    background: 'var(--bg-surface)',
                  }}
                  onMouseEnter={e => {
                    e.currentTarget.style.background = 'var(--bg-hover)'
                  }}
                  onMouseLeave={e => {
                    e.currentTarget.style.background = 'var(--bg-surface)'
                  }}
                >
                  <PriorityTag priority={t.priority as Priority} />
                  <div
                    style={{
                      flex: 1,
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      color: 'var(--fg1)',
                    }}
                  >
                    {t.title}
                  </div>
                  <StatusBadge status={t.status as Status} />
                  {t.owner && <Avatar name={t.owner} />}
                </button>
              ))}
            </div>
          )}
        </>
      )}

      {/* Transcript tab */}
      {activeTab === 'transcript' && hasTranscript && (
        <div
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: 'var(--text-sm)',
            color: 'var(--fg2)',
            lineHeight: 1.7,
            whiteSpace: 'pre-wrap',
            background: 'var(--bg-sunken)',
            borderRadius: 'var(--radius-md)',
            padding: '12px 14px',
            maxHeight: 400,
            overflowY: 'auto',
          }}
        >
          {meeting.raw_content}
        </div>
      )}

      {/* Rich Content tab */}
      {activeTab === 'rich' && hasRich && (
        <div style={{ maxHeight: 480, overflowY: 'auto' }}>
          {meeting.content_type === 'markdown' ? (
            <div
              className="prose prose-invert max-w-none"
              style={{ font: 'var(--text-body)', lineHeight: 1.7, color: 'var(--fg2)' }}
            >
              <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeHighlight]}>
                {meeting.rich_content ?? ''}
              </ReactMarkdown>
            </div>
          ) : (
            <SafeHtml html={meeting.rich_content ?? ''} />
          )}
        </div>
      )}
    </Modal>
  )
}

function SafeHtml({ html }: { html: string }) {
  return (
    <div
      style={{ font: 'var(--text-body)', lineHeight: 1.7, color: 'var(--fg2)' }}
      dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(html) }}
    />
  )
}

function TabButton({
  label,
  active,
  onClick,
}: {
  label: string
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      style={{
        background: 'none',
        border: 'none',
        borderBottom: active ? '2px solid var(--brand)' : '2px solid transparent',
        color: active ? 'var(--brand)' : 'var(--fg2)',
        cursor: 'pointer',
        padding: '8px 14px',
        font: 'var(--text-h4)',
        fontWeight: active ? 600 : 400,
        marginBottom: -1,
        transition: 'color 0.15s ease, border-color 0.15s ease',
      }}
      onMouseEnter={e => {
        if (!active) e.currentTarget.style.color = 'var(--fg1)'
      }}
      onMouseLeave={e => {
        if (!active) e.currentTarget.style.color = 'var(--fg2)'
      }}
    >
      {label}
    </button>
  )
}

function MetaItem({
  label,
  value,
  small,
}: {
  label: string
  value: string
  small?: boolean
}) {
  return (
    <div
      style={{
        background: 'var(--bg-sunken)',
        borderRadius: 'var(--radius-md)',
        padding: 12,
      }}
    >
      <div className="label" style={{ marginBottom: 3 }}>{label}</div>
      <div
        style={{
          fontWeight: 600,
          font: small ? 'var(--text-caption)' : 'var(--text-body)',
          color: 'var(--fg1)',
          wordBreak: 'break-all',
        }}
      >
        {value}
      </div>
    </div>
  )
}
