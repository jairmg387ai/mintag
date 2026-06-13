import { useEffect, useState } from 'react'
import type { CSSProperties } from 'react'
import { Calendar } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useAppState, useAppActions } from '../../store/AppContext'
import { Modal, Field } from './Modal'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'

const selectStyle: CSSProperties = {
  width: '100%',
  padding: '8px 12px',
  border: '1px solid var(--border-strong)',
  borderRadius: 'var(--radius-md)',
  font: 'var(--text-body)',
  color: 'var(--fg1)',
  background: 'var(--bg-sunken)',
  outline: 'none',
  boxSizing: 'border-box',
}

const textareaStyle: CSSProperties = {
  width: '100%',
  padding: '8px 12px',
  border: '1px solid var(--border-strong)',
  borderRadius: 'var(--radius-md)',
  font: 'var(--text-body)',
  color: 'var(--fg1)',
  background: 'var(--bg-sunken)',
  outline: 'none',
  resize: 'vertical',
  minHeight: 80,
  fontFamily: 'inherit',
  boxSizing: 'border-box',
}
import { Timeline } from '../shared/Timeline'
import { updateTask, getTask, getTaskHistory, getMeeting } from '../../api/client'
import type { Task, TaskHistory, Meeting, Status, Priority } from '../../types'
import { useToast } from '../../hooks/useToast'

const STATUSES: { value: Status; label: string }[] = [
  { value: 'todo', label: 'Todo' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'blocked', label: 'Blocked' },
  { value: 'in_testing', label: 'In Testing' },
  { value: 'done', label: 'Done' },
  { value: 'cancelled', label: 'Cancelled' },
]

const PRIORITIES: { value: Priority; label: string }[] = [
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
  { value: 'critical', label: 'Critical' },
]

function fmt(dt: string) {
  if (!dt) return '—'
  try {
    return new Date(dt).toLocaleString('en-US', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return dt
  }
}

export function TaskDetailModal() {
  const { editingTaskId, meetings } = useAppState()
  const { closeModal, loadAll } = useAppActions()
  const { pushToast } = useToast()

  const [task, setTask] = useState<Task | null>(null)
  const [history, setHistory] = useState<TaskHistory[]>([])
  const [meetingCtx, setMeetingCtx] = useState<Meeting | null>(null)
  const [descPreview, setDescPreview] = useState(false)
  const [form, setForm] = useState({
    status: '',
    priority: '',
    owner: '',
    due_date: '',
    description: '',
    note: '',
    author: '',
    source_meeting_id: '',
  })

  useEffect(() => {
    if (!editingTaskId) return
    Promise.all([getTask(editingTaskId), getTaskHistory(editingTaskId)]).then(([t, h]) => {
      setTask(t)
      setHistory(h)
      setForm({
        status: t.status,
        priority: t.priority,
        owner: t.owner ?? '',
        due_date: t.due_date ?? '',
        description: t.description ?? '',
        note: '',
        author: '',
        source_meeting_id: '',
      })
      if (t.meeting_id) {
        getMeeting(t.meeting_id).then(setMeetingCtx).catch(() => setMeetingCtx(null))
      }
    }).catch(e => {
      pushToast(e.message, true)
      closeModal()
    })
  }, [editingTaskId])

  async function handleSave() {
    if (!task) return
    try {
      await updateTask(task.id, {
        status: form.status as Status,
        priority: form.priority as Priority,
        owner: form.owner,
        due_date: form.due_date,
        description: form.description,
        note: form.note,
        author: form.author,
        source_meeting_id: parseInt(form.source_meeting_id) || null,
      })
      pushToast('Task updated')
      closeModal()
      await loadAll()
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'Error', true)
    }
  }

  if (!task) return null

  return (
    <Modal
      title={task.title}
      maxWidth={700}
      footer={
        <>
          <Button variant="ghost" onClick={closeModal}>Cancel</Button>
          <Button variant="primary" onClick={handleSave}>Save Changes</Button>
        </>
      }
    >
      {/* Meeting context banner */}
      {meetingCtx && (
        <div
          className="card"
          style={{
            padding: '12px 14px',
            marginBottom: 16,
            borderLeft: '3px solid var(--brand)',
            borderRadius: 'var(--radius-md)',
          }}
        >
          <div className="label" style={{ marginBottom: 6 }}>Meeting Context</div>
          <div style={{ font: 'var(--text-h4)', color: 'var(--fg1)', marginBottom: 2 }}>
            {meetingCtx.title}
          </div>
          <div
            style={{
              font: 'var(--text-mono)',
              fontSize: 11,
              color: 'var(--fg3)',
              marginBottom: 6,
              display: 'flex',
              alignItems: 'center',
              gap: 5,
            }}
          >
            <Calendar size={12} strokeWidth={1.75} />
            {meetingCtx.date}
          </div>
          {(meetingCtx.summary || meetingCtx.raw_content) && (
            <div style={{ font: 'var(--text-sm)', color: 'var(--fg2)', lineHeight: 1.5 }}>
              {(meetingCtx.summary || meetingCtx.raw_content || '').slice(0, 300)}
              {(meetingCtx.summary || meetingCtx.raw_content || '').length > 300 ? '…' : ''}
            </div>
          )}
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
        {/* Left column: form */}
        <div style={{ gridColumn: '1 / -1' }}>
          {/* Status chips */}
          <Field label="Status">
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              {STATUSES.map(s => {
                const isActive = form.status === s.value
                return (
                  <button
                    key={s.value}
                    onClick={() => setForm(f => ({ ...f, status: s.value }))}
                    style={{
                      padding: '5px 10px',
                      borderRadius: 'var(--radius-md)',
                      font: 'var(--text-caption)',
                      fontWeight: 600,
                      border: '1px solid ' + (isActive ? 'var(--brand)' : 'var(--border)'),
                      background: isActive ? 'color-mix(in srgb, var(--brand) 15%, var(--bg-surface))' : 'var(--bg-surface)',
                      color: isActive ? 'var(--brand)' : 'var(--fg2)',
                      cursor: 'pointer',
                    }}
                  >
                    {s.label}
                  </button>
                )
              })}
            </div>
          </Field>
        </div>

        {/* Priority row */}
        <div style={{ gridColumn: '1 / -1' }}>
          <Field label="Priority">
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              {PRIORITIES.map(p => {
                const isActive = form.priority === p.value
                return (
                  <button
                    key={p.value}
                    onClick={() => setForm(f => ({ ...f, priority: p.value }))}
                    style={{
                      padding: '4px 10px',
                      borderRadius: 'var(--radius-md)',
                      font: 'var(--text-caption)',
                      fontWeight: 600,
                      border: '1px solid ' + (isActive ? 'var(--brand)' : 'var(--border)'),
                      background: isActive ? 'color-mix(in srgb, var(--brand) 15%, var(--bg-surface))' : 'var(--bg-surface)',
                      color: isActive ? 'var(--brand)' : 'var(--fg2)',
                      cursor: 'pointer',
                    }}
                  >
                    {p.label}
                  </button>
                )
              })}
            </div>
          </Field>
        </div>

        {/* Owner + Due date */}
        <Field label="Owner">
          <Input
            value={form.owner}
            onChange={e => setForm(f => ({ ...f, owner: e.target.value }))}
            placeholder="Name"
          />
        </Field>
        <Field label="Due Date">
          <Input
            type="date"
            value={form.due_date}
            onChange={e => setForm(f => ({ ...f, due_date: e.target.value }))}
          />
        </Field>
      </div>

      {/* Description */}
      <Field label="Description">
        <div style={{ display: 'flex', gap: 6, marginBottom: 6 }}>
          <button
            type="button"
            onClick={() => setDescPreview(false)}
            style={{
              padding: '3px 10px',
              borderRadius: 'var(--radius-md)',
              font: 'var(--text-caption)',
              fontWeight: descPreview ? 400 : 600,
              border: '1px solid ' + (descPreview ? 'var(--border)' : 'var(--brand)'),
              background: descPreview ? 'var(--bg-surface)' : 'color-mix(in srgb, var(--brand) 15%, var(--bg-surface))',
              color: descPreview ? 'var(--fg2)' : 'var(--brand)',
              cursor: 'pointer',
            }}
          >
            Edit
          </button>
          <button
            type="button"
            onClick={() => setDescPreview(true)}
            style={{
              padding: '3px 10px',
              borderRadius: 'var(--radius-md)',
              font: 'var(--text-caption)',
              fontWeight: descPreview ? 600 : 400,
              border: '1px solid ' + (descPreview ? 'var(--brand)' : 'var(--border)'),
              background: descPreview ? 'color-mix(in srgb, var(--brand) 15%, var(--bg-surface))' : 'var(--bg-surface)',
              color: descPreview ? 'var(--brand)' : 'var(--fg2)',
              cursor: 'pointer',
            }}
          >
            Preview
          </button>
        </div>
        {descPreview ? (
          <div
            className="prose prose-invert max-w-none"
            style={{
              padding: '8px 12px',
              border: '1px solid var(--border-strong)',
              borderRadius: 'var(--radius-md)',
              background: 'var(--bg-sunken)',
              minHeight: 80,
              font: 'var(--text-body)',
              color: 'var(--fg1)',
              lineHeight: 1.6,
            }}
          >
            {form.description
              ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{form.description}</ReactMarkdown>
              : <span style={{ color: 'var(--fg3)' }}>Nothing to preview</span>
            }
          </div>
        ) : (
          <textarea
            style={textareaStyle}
            value={form.description}
            onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
          />
        )}
      </Field>

      {/* Update note */}
      <Field label="Update Note">
        <Input
          value={form.note}
          onChange={e => setForm(f => ({ ...f, note: e.target.value }))}
          placeholder="Why is this changing / what was discussed..."
        />
      </Field>

      {/* Meeting trigger */}
      <Field label="Meeting that triggered this change">
        <select
          style={selectStyle}
          value={form.source_meeting_id}
          onChange={e => setForm(f => ({ ...f, source_meeting_id: e.target.value }))}
        >
          <option value="">— none —</option>
          {meetings.map(m => (
            <option key={m.id} value={m.id}>
              {m.date} — {m.title}
            </option>
          ))}
        </select>
      </Field>

      {/* Author */}
      <Field label="Updated by">
        <Input
          value={form.author}
          onChange={e => setForm(f => ({ ...f, author: e.target.value }))}
          placeholder="Your name..."
        />
      </Field>

      {/* Activity timeline */}
      {history.length > 0 && (
        <div style={{ marginTop: 20 }}>
          <div className="label" style={{ marginBottom: 12 }}>Activity</div>
          <Timeline history={history} />
        </div>
      )}

      {/* Metadata footer */}
      <div
        style={{
          marginTop: 16,
          padding: 12,
          background: 'var(--bg-sunken)',
          borderRadius: 'var(--radius-md)',
          font: 'var(--text-caption)',
          color: 'var(--fg3)',
        }}
      >
        Created {fmt(task.created_at)} · Project: {task.project_name ?? '—'} · Source meeting: {task.meeting_title ?? '—'}
      </div>
    </Modal>
  )
}
