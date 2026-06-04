import { useEffect, useState } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { Modal, Field, Input, Textarea, Select, BtnPrimary, BtnGhost } from './Modal'
import { Timeline } from '../shared/Timeline'
import { updateTask, getTask, getTaskHistory, getMeeting } from '../../api/client'
import type { Task, TaskHistory, Meeting, Status, Priority } from '../../types'
import { useToast } from '../../hooks/useToast'

function fmt(dt: string) {
  if (!dt) return '—'
  try {
    return new Date(dt).toLocaleString('en-US', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch { return dt }
}

export function TaskDetailModal() {
  const { editingTaskId, meetings } = useAppState()
  const { closeModal, loadAll } = useAppActions()
  const { pushToast } = useToast()

  const [task, setTask] = useState<Task | null>(null)
  const [history, setHistory] = useState<TaskHistory[]>([])
  const [meetingCtx, setMeetingCtx] = useState<Meeting | null>(null)
  const [form, setForm] = useState({ status: '', priority: '', owner: '', due_date: '', description: '', note: '', author: '', source_meeting_id: '' })

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
    }).catch(e => { pushToast(e.message, true); closeModal() })
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
      footer={
        <>
          <BtnGhost onClick={closeModal}>Close</BtnGhost>
          <BtnPrimary onClick={handleSave}>Save Changes</BtnPrimary>
        </>
      }
    >
      {/* Meeting context */}
      {meetingCtx && (
        <div style={{ background: 'rgba(59,130,246,0.12)', border: '1px solid var(--color-blue-dim)', borderRadius: 8, padding: '12px 14px', marginBottom: 16 }}>
          <div style={{ fontSize: '0.72em', textTransform: 'uppercase', letterSpacing: '0.5px', color: 'var(--color-blue)', marginBottom: 6, fontWeight: 600 }}>Meeting Context</div>
          <div style={{ fontWeight: 600, fontSize: '0.9em', color: 'var(--color-text)', marginBottom: 2 }}>{meetingCtx.title}</div>
          <div style={{ fontSize: '0.75em', color: 'var(--color-text3)', marginBottom: 6 }}>{meetingCtx.date}</div>
          {(meetingCtx.summary || meetingCtx.raw_content) && (
            <div style={{ fontSize: '0.82em', color: 'var(--color-text2)', lineHeight: 1.5 }}>
              {(meetingCtx.summary || meetingCtx.raw_content || '').slice(0, 300)}{(meetingCtx.summary || meetingCtx.raw_content || '').length > 300 ? '…' : ''}
            </div>
          )}
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginBottom: 16 }}>
        <Field label="Status">
          <Select value={form.status} onChange={e => setForm(f => ({ ...f, status: e.target.value }))}>
            {(['todo', 'in_progress', 'blocked', 'done', 'cancelled'] as Status[]).map(s => (
              <option key={s} value={s}>{s === 'in_progress' ? 'In Progress' : s.charAt(0).toUpperCase() + s.slice(1)}</option>
            ))}
          </Select>
        </Field>
        <Field label="Priority">
          <Select value={form.priority} onChange={e => setForm(f => ({ ...f, priority: e.target.value }))}>
            {(['low', 'medium', 'high', 'critical'] as Priority[]).map(p => (
              <option key={p} value={p}>{p.charAt(0).toUpperCase() + p.slice(1)}</option>
            ))}
          </Select>
        </Field>
        <Field label="Owner">
          <Input value={form.owner} onChange={e => setForm(f => ({ ...f, owner: e.target.value }))} placeholder="Name" />
        </Field>
        <Field label="Due Date">
          <Input type="date" value={form.due_date} onChange={e => setForm(f => ({ ...f, due_date: e.target.value }))} />
        </Field>
      </div>

      <Field label="Description">
        <Textarea value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))} />
      </Field>
      <Field label="Update Note">
        <Input value={form.note} onChange={e => setForm(f => ({ ...f, note: e.target.value }))} placeholder="Why is this changing / what was discussed..." />
      </Field>
      <Field label="Meeting that triggered this change">
        <Select value={form.source_meeting_id} onChange={e => setForm(f => ({ ...f, source_meeting_id: e.target.value }))}>
          <option value="">— none —</option>
          {meetings.map(m => <option key={m.id} value={m.id}>📅 {m.date} — {m.title}</option>)}
        </Select>
      </Field>
      <Field label="Updated by">
        <Input value={form.author} onChange={e => setForm(f => ({ ...f, author: e.target.value }))} placeholder="Your name..." />
      </Field>

      {history.length > 0 && (
        <div style={{ marginTop: 20 }}>
          <div style={{ fontSize: '0.8em', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.5px', color: 'var(--color-text3)', marginBottom: 12 }}>Change History</div>
          <Timeline history={history} />
        </div>
      )}

      <div style={{ marginTop: 16, padding: 12, background: 'var(--color-surface2)', borderRadius: 8, fontSize: '0.8em', color: 'var(--color-text3)' }}>
        Created {fmt(task.created_at)} · Project: {task.project_name ?? '—'} · Source meeting: {task.meeting_title ?? '—'}
      </div>
    </Modal>
  )
}
