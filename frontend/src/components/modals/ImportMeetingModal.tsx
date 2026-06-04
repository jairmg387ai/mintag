import { useState } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { Modal, Field, Input, Textarea, Select, BtnPrimary, BtnGhost } from './Modal'
import { importMeeting } from '../../api/client'
import { useToast } from '../../hooks/useToast'

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
          <BtnGhost onClick={closeModal}>Cancel</BtnGhost>
          <BtnPrimary onClick={handleSubmit}>Import</BtnPrimary>
        </>
      }
    >
      <Field label="File path (.vtt or .txt)">
        <Input value={form.path} onChange={e => setForm(f => ({ ...f, path: e.target.value }))} placeholder="E:\path\to\meeting.vtt" />
      </Field>
      <Field label="Project">
        <Select value={form.project_id} onChange={e => setForm(f => ({ ...f, project_id: e.target.value }))}>
          <option value="">— no project —</option>
          {projects.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
        </Select>
      </Field>
      <Field label="Summary (optional)">
        <Textarea value={form.summary} onChange={e => setForm(f => ({ ...f, summary: e.target.value }))} placeholder="Key points from the meeting..." />
      </Field>
    </Modal>
  )
}
