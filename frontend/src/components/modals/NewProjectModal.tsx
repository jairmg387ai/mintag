import { useState } from 'react'
import { useAppActions } from '../../store/AppContext'
import { Modal, Field, Input, BtnPrimary, BtnGhost } from './Modal'
import { createProject } from '../../api/client'
import { useToast } from '../../hooks/useToast'

export function NewProjectModal() {
  const { closeModal, loadAll } = useAppActions()
  const { pushToast } = useToast()

  const [form, setForm] = useState({ name: '', description: '', color: '#3b82f6' })

  async function handleSubmit() {
    if (!form.name.trim()) { pushToast('Name is required', true); return }
    try {
      await createProject({ name: form.name.trim(), description: form.description, color: form.color })
      pushToast('Project created')
      closeModal()
      await loadAll()
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'Error', true)
    }
  }

  return (
    <Modal
      title="New Project"
      footer={
        <>
          <BtnGhost onClick={closeModal}>Cancel</BtnGhost>
          <BtnPrimary onClick={handleSubmit}>Create</BtnPrimary>
        </>
      }
    >
      <Field label="Name *">
        <Input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} placeholder="My Project" />
      </Field>
      <Field label="Description">
        <Input value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))} placeholder="Brief description..." />
      </Field>
      <Field label="Color">
        <Input type="color" value={form.color} onChange={e => setForm(f => ({ ...f, color: e.target.value }))} style={{ height: 38, padding: '2px 6px' }} />
      </Field>
    </Modal>
  )
}
