import { useState } from 'react'
import type { CSSProperties } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { Modal, Field } from './Modal'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'
import { createTask } from '../../api/client'
import { useToast } from '../../hooks/useToast'

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

export function NewTaskModal() {
  const { projects, meetings } = useAppState()
  const { closeModal, loadAll } = useAppActions()
  const { pushToast } = useToast()

  const [form, setForm] = useState({
    title: '',
    description: '',
    owner: '',
    priority: 'medium',
    status: 'todo',
    due_date: '',
    project_id: '',
    meeting_id: '',
  })

  async function handleSubmit() {
    if (!form.title.trim()) { pushToast('Title is required', true); return }
    try {
      await createTask({
        title: form.title.trim(),
        description: form.description,
        owner: form.owner,
        priority: form.priority as never,
        status: form.status as never,
        due_date: form.due_date,
        project_id: parseInt(form.project_id) || null,
        meeting_id: parseInt(form.meeting_id) || null,
      })
      pushToast('Task created')
      closeModal()
      await loadAll()
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'Error', true)
    }
  }

  return (
    <Modal
      title="New Task"
      footer={
        <>
          <Button variant="ghost" onClick={closeModal}>Cancel</Button>
          <Button variant="primary" onClick={handleSubmit}>Create Task</Button>
        </>
      }
    >
      <Field label="Title *">
        <Input value={form.title} onChange={e => setForm(f => ({ ...f, title: e.target.value }))} placeholder="What needs to be done..." />
      </Field>
      <Field label="Description">
        <textarea
          style={textareaStyle}
          value={form.description}
          onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
          placeholder="Context and details..."
        />
      </Field>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
        <Field label="Owner">
          <Input value={form.owner} onChange={e => setForm(f => ({ ...f, owner: e.target.value }))} placeholder="Name" />
        </Field>
        <Field label="Priority">
          <select
            style={selectStyle}
            value={form.priority}
            onChange={e => setForm(f => ({ ...f, priority: e.target.value }))}
          >
            <option value="low">Low</option>
            <option value="medium">Medium</option>
            <option value="high">High</option>
            <option value="critical">Critical</option>
          </select>
        </Field>
        <Field label="Status">
          <select
            style={selectStyle}
            value={form.status}
            onChange={e => setForm(f => ({ ...f, status: e.target.value }))}
          >
            <option value="todo">To Do</option>
            <option value="in_progress">In Progress</option>
            <option value="blocked">Blocked</option>
          </select>
        </Field>
        <Field label="Due Date">
          <Input type="date" value={form.due_date} onChange={e => setForm(f => ({ ...f, due_date: e.target.value }))} />
        </Field>
      </div>
      <Field label="Project">
        <select
          style={selectStyle}
          value={form.project_id}
          onChange={e => setForm(f => ({ ...f, project_id: e.target.value }))}
        >
          <option value="">— no project —</option>
          {projects.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
        </select>
      </Field>
      <Field label="Source Meeting (optional)">
        <select
          style={selectStyle}
          value={form.meeting_id}
          onChange={e => setForm(f => ({ ...f, meeting_id: e.target.value }))}
        >
          <option value="">— none —</option>
          {meetings.map(m => <option key={m.id} value={m.id}>{m.date} — {m.title}</option>)}
        </select>
      </Field>
    </Modal>
  )
}
