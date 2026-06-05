import { useState } from 'react'
import type { CSSProperties } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { Modal, Field } from './Modal'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'
import { importMeeting } from '../../api/client'
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

export function ImportMeetingModal() {
  const { projects } = useAppState()
  const { closeModal, loadAll } = useAppActions()
  const { pushToast } = useToast()

  const [form, setForm] = useState({ path: '', project_id: '', summary: '' })

  async function handleSubmit() {
    if (!form.path.trim()) { pushToast('File path is required', true); return }
    try {
      const result = await importMeeting({
        path: form.path.trim(),
        project_id: parseInt(form.project_id) || null,
        summary: form.summary,
      })
      pushToast(`Meeting imported: ${result.title}`)
      closeModal()
      await loadAll()
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'Error', true)
    }
  }

  return (
    <Modal
      title="Import Meeting"
      footer={
        <>
          <Button variant="ghost" onClick={closeModal}>Cancel</Button>
          <Button variant="primary" onClick={handleSubmit}>Import</Button>
        </>
      }
    >
      <Field label="File path (.vtt or .txt)">
        <Input value={form.path} onChange={e => setForm(f => ({ ...f, path: e.target.value }))} placeholder="E:\path\to\meeting.vtt" />
      </Field>
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
      <Field label="Summary (optional)">
        <textarea
          style={textareaStyle}
          value={form.summary}
          onChange={e => setForm(f => ({ ...f, summary: e.target.value }))}
          placeholder="Key points from the meeting..."
        />
      </Field>
    </Modal>
  )
}
