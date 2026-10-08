import { useEffect, useRef, useState, type CSSProperties } from 'react'
import { AlertTriangle, ExternalLink, X } from 'lucide-react'
import type { ActivityCatalog, BugCorrectionTaskDraft, CreatedBugCorrectionTaskResponse } from '../../types'
import {
  BugEvidenceApiError,
  createBugCorrectionTask,
  fetchSubareaAllowedValues,
  getBugCorrectionTaskDraft,
} from '../../api/client'
import { azureWorkItemUrl } from '../activities/azureActivity'
import { ClassificationTreePicker } from './ClassificationTreePicker'

interface CreateBugCorrectionTaskModalProps {
  open: boolean
  bugId: number | null
  onClose: () => void
  onCreated: (result: CreatedBugCorrectionTaskResponse) => void
  catalog?: ActivityCatalog | null
  // Display name of the connected Azure identity; the catalog checkbox
  // defaults to checked only when the bug's assignee is this person.
  currentUserDisplayName?: string
}

const fieldStyle: CSSProperties = {
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
  ...fieldStyle,
  resize: 'vertical',
  minHeight: 70,
  fontFamily: 'inherit',
}

function Field({ label, htmlFor, children }: { label: string; htmlFor?: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 14 }}>
      <label className="label" htmlFor={htmlFor} style={{ marginBottom: 6, display: 'block' }}>
        {label}
      </label>
      {children}
    </div>
  )
}

// errorMessage turns an API failure into user-facing text: structured
// {"code": ...} errors from the correction-task routes carry an optional
// "message"; anything else falls back to the raw error text.
function errorMessage(e: unknown, fallback: string): string {
  if (e instanceof BugEvidenceApiError) {
    if (e.code === 'not_a_bug') return 'El work item no es un Bug'
    const msg = e.extra?.message
    return typeof msg === 'string' && msg ? msg : e.code
  }
  return e instanceof Error && e.message ? e.message : fallback
}

// CreateBugCorrectionTaskModal creates a Bug's correction Task ("Atención
// y/o Corrección del defecto <bugId>") in the bug's own team project, linked
// to the bug as parent, as the CMMI team does today. Subárea has no default
// on purpose: the TL must pick it. Form state is not reset between bugs:
// callers mount one instance per bug (key={bugId}) so each opens fresh.
export function CreateBugCorrectionTaskModal({
  open,
  bugId,
  onClose,
  onCreated,
  catalog = null,
  currentUserDisplayName,
}: CreateBugCorrectionTaskModalProps) {
  const pressedOnOverlay = useRef(false)
  const [draft, setDraft] = useState<BugCorrectionTaskDraft | null>(null)
  const [loadError, setLoadError] = useState('')
  const [subareas, setSubareas] = useState<string[]>([])
  const [subareaError, setSubareaError] = useState('')

  const [title, setTitle] = useState('')
  const [subarea, setSubarea] = useState('')
  const [estimate, setEstimate] = useState('')
  const [iterationPath, setIterationPath] = useState('')
  const [areaPath, setAreaPath] = useState('')
  const [assignedTo, setAssignedTo] = useState('')
  const [description, setDescription] = useState('')
  // null until the user toggles the checkbox; until then the default is
  // derived from the connected identity so a late-arriving identity still
  // applies without overwriting an explicit choice.
  const [addToCatalogChoice, setAddToCatalogChoice] = useState<boolean | null>(null)
  const [project, setProject] = useState('')
  const [category, setCategory] = useState('')

  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')
  const [created, setCreated] = useState<CreatedBugCorrectionTaskResponse | null>(null)

  useEffect(() => {
    if (!open || bugId === null) return
    let cancelled = false
    getBugCorrectionTaskDraft(bugId)
      .then(d => {
        if (cancelled) return
        setLoadError('')
        setDraft(d)
        setTitle(d.suggested_title)
        setAreaPath(d.area_path)
        setAssignedTo(d.assigned_to.unique_name)
        fetchSubareaAllowedValues(d.team_project)
          .then(values => { if (!cancelled) setSubareas(values) })
          .catch((e: unknown) => { if (!cancelled) setSubareaError(errorMessage(e, 'No se pudieron cargar las subáreas')) })
      })
      .catch((e: unknown) => {
        if (!cancelled) setLoadError(errorMessage(e, 'No se pudo cargar el bug'))
      })
    return () => { cancelled = true }
  }, [open, bugId])

  const me = currentUserDisplayName?.trim()
  const assigneeIsMe = !!draft && !!me && draft.assigned_to.display_name.trim() === me
  const addToCatalog = addToCatalogChoice ?? assigneeIsMe

  if (!open || bugId === null) return null

  const estimateValue = parseFloat(estimate)
  const isValid =
    !!draft &&
    title.trim() !== '' &&
    subarea !== '' &&
    !isNaN(estimateValue) && estimateValue > 0 &&
    iterationPath !== '' &&
    assignedTo.trim() !== ''

  async function handleSubmit() {
    if (!isValid || bugId === null) return
    setSubmitting(true)
    setSubmitError('')
    try {
      const categoryId = addToCatalog && category ? catalog?.categories.find(c => c.name === category)?.id : undefined
      const result = await createBugCorrectionTask(bugId, {
        title: title.trim(),
        ...(description.trim() ? { description: description.trim() } : {}),
        subarea,
        original_estimate: estimateValue,
        area_path: areaPath,
        iteration_path: iterationPath,
        assigned_to: assignedTo.trim(),
        add_to_catalog: addToCatalog,
        ...(addToCatalog && project ? { project } : {}),
        ...(categoryId !== undefined ? { category_id: categoryId } : {}),
      })
      setCreated(result)
      onCreated(result)
    } catch (e: unknown) {
      setSubmitError(errorMessage(e, 'No se pudo crear la tarea de corrección'))
    } finally {
      setSubmitting(false)
    }
  }

  const org = draft?.org ?? ''

  return (
    <div
      onMouseDown={e => { pressedOnOverlay.current = e.target === e.currentTarget }}
      onClick={e => {
        if (pressedOnOverlay.current && e.target === e.currentTarget) onClose()
        pressedOnOverlay.current = false
      }}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(15,23,42,0.5)',
        zIndex: 100,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 20,
      }}
    >
      <div
        role="dialog"
        aria-label="Crear tarea de corrección"
        onClick={e => e.stopPropagation()}
        className="card"
        style={{
          borderRadius: 'var(--radius-xl)',
          boxShadow: 'var(--shadow-xl)',
          width: '100%',
          maxWidth: 600,
          maxHeight: '90vh',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        <div style={{ padding: '16px 20px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: 10 }}>
          <h2 style={{ font: 'var(--text-h3)', color: 'var(--fg1)', flex: 1, margin: 0 }}>
            Tarea de corrección del bug
          </h2>
          <button
            onClick={onClose}
            style={{ background: 'none', border: 'none', color: 'var(--fg3)', cursor: 'pointer', display: 'flex', alignItems: 'center', padding: 4, borderRadius: 'var(--radius-md)' }}
            aria-label="Cerrar"
          >
            <X size={18} strokeWidth={1.75} />
          </button>
        </div>

        <div style={{ padding: '20px 22px', overflowY: 'auto', flex: 1 }}>
          {loadError && (
            <div style={{ padding: '10px 14px', background: 'var(--rose-50)', color: 'var(--block-solid)', borderRadius: 'var(--radius-md)', font: 'var(--text-sm)' }}>
              {loadError}
            </div>
          )}
          {!draft && !loadError && (
            <div style={{ font: 'var(--text-body)', color: 'var(--fg3)' }}>Cargando bug #{bugId}...</div>
          )}

          {draft && (
            <>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 14, font: 'var(--text-body)', color: 'var(--fg1)' }}>
                <span>#{draft.bug_id} — {draft.bug_title}</span>
                {org && (
                  <a
                    href={azureWorkItemUrl({ org, work_item_id: draft.bug_id })}
                    target="_blank"
                    rel="noreferrer"
                    title="Abrir bug en Azure DevOps"
                    aria-label={`Abrir bug #${draft.bug_id} en Azure DevOps`}
                    style={{ color: 'var(--fg3)', display: 'inline-flex' }}
                  >
                    <ExternalLink size={13} strokeWidth={1.75} />
                  </a>
                )}
                <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>{draft.team_project}</span>
              </div>

              {draft.existing_tasks_error && (
                <div
                  role="alert"
                  title={draft.existing_tasks_error}
                  style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '10px 14px', background: 'var(--amber-50, var(--bg-sunken))', border: '1px solid var(--border)', borderRadius: 'var(--radius-md)', font: 'var(--text-sm)', color: 'var(--fg1)', marginBottom: 14 }}
                >
                  <AlertTriangle size={14} strokeWidth={1.75} />
                  No se pudo verificar si ya existe una tarea de corrección
                </div>
              )}

              {draft.existing_correction_tasks.length > 0 && (
                <div
                  role="alert"
                  style={{ padding: '10px 14px', background: 'var(--amber-50, var(--bg-sunken))', border: '1px solid var(--border)', borderRadius: 'var(--radius-md)', font: 'var(--text-sm)', color: 'var(--fg1)', marginBottom: 14 }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                    <AlertTriangle size={14} strokeWidth={1.75} />
                    Este bug ya tiene tareas de corrección. Puedes crear otra si es necesario.
                  </div>
                  <ul style={{ margin: 0, paddingLeft: 18 }}>
                    {draft.existing_correction_tasks.map(t => (
                      <li key={t.id}>#{t.id} — {t.title} ({t.state})</li>
                    ))}
                  </ul>
                </div>
              )}

              {created ? (
                <div style={{ padding: '10px 14px', background: 'var(--bg-sunken)', border: '1px solid var(--border)', borderRadius: 'var(--radius-md)', font: 'var(--text-body)', color: 'var(--fg1)' }}>
                  Tarea #{created.id} creada ({created.state}).{' '}
                  {org && (
                    <a href={azureWorkItemUrl({ org, work_item_id: created.id })} target="_blank" rel="noreferrer">
                      Abrir #{created.id} en Azure DevOps
                    </a>
                  )}
                  {created.catalog_error && (
                    <div style={{ color: 'var(--block-solid)', font: 'var(--text-sm)', marginTop: 6 }}>
                      No se pudo agregar al catálogo: {created.catalog_error}
                    </div>
                  )}
                </div>
              ) : (
                <>
                  <Field label="Título *" htmlFor="bct-title">
                    <input id="bct-title" type="text" value={title} onChange={e => setTitle(e.target.value)} style={fieldStyle} />
                  </Field>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                    <Field label="Subárea *" htmlFor="bct-subarea">
                      <select id="bct-subarea" aria-label="Subárea *" value={subarea} onChange={e => setSubarea(e.target.value)} style={fieldStyle}>
                        <option value="">Selecciona la subárea</option>
                        {subareas.map(v => <option key={v} value={v}>{v}</option>)}
                      </select>
                      {subareaError && <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)', marginTop: 4 }}>{subareaError}</div>}
                    </Field>

                    <Field label="Estimado en horas *" htmlFor="bct-estimate">
                      <input id="bct-estimate" type="number" min="0" step="0.5" value={estimate} onChange={e => setEstimate(e.target.value)} placeholder="Ej: 6" style={fieldStyle} />
                    </Field>
                  </div>

                  <Field label="Iteración *">
                    <ClassificationTreePicker
                      kind="iterations"
                      ariaLabel="Iteración *"
                      value={iterationPath}
                      onChange={setIterationPath}
                      inputStyle={fieldStyle}
                      teamProject={draft.team_project}
                    />
                  </Field>

                  <Field label="Área">
                    <ClassificationTreePicker
                      kind="areas"
                      ariaLabel="Área"
                      value={areaPath}
                      onChange={setAreaPath}
                      inputStyle={fieldStyle}
                      teamProject={draft.team_project}
                    />
                  </Field>

                  <Field label="Asignado a *" htmlFor="bct-assignee">
                    <input
                      id="bct-assignee"
                      type="text"
                      value={assignedTo}
                      onChange={e => setAssignedTo(e.target.value)}
                      placeholder="usuario@dominio.com"
                      style={fieldStyle}
                    />
                    {draft.assigned_to.display_name && (
                      <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', marginTop: 4 }}>
                        Asignado del bug: {draft.assigned_to.display_name}
                      </div>
                    )}
                  </Field>

                  <Field label="Descripción" htmlFor="bct-description">
                    <textarea id="bct-description" style={textareaStyle} value={description} onChange={e => setDescription(e.target.value)} placeholder="Descripción (opcional)..." />
                  </Field>

                  <label style={{ display: 'flex', alignItems: 'center', gap: 8, font: 'var(--text-body)', color: 'var(--fg1)', marginBottom: 12 }}>
                    <input type="checkbox" checked={addToCatalog} onChange={e => setAddToCatalogChoice(e.target.checked)} />
                    Agregar al catálogo
                  </label>

                  {addToCatalog && catalog !== null && (
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                      <Field label="Proyecto">
                        <select aria-label="Proyecto" style={fieldStyle} value={project} onChange={e => setProject(e.target.value)}>
                          <option value="">— Sin proyecto —</option>
                          {catalog.projects.map(p => <option key={p.name} value={p.name}>{p.name}</option>)}
                        </select>
                      </Field>
                      <Field label="Categoría">
                        <select aria-label="Categoría" style={fieldStyle} value={category} onChange={e => setCategory(e.target.value)}>
                          <option value="">— Sin categoría —</option>
                          {catalog.categories.map(c => <option key={c.id} value={c.name}>{c.name}</option>)}
                        </select>
                      </Field>
                    </div>
                  )}

                  {submitError && (
                    <div style={{ padding: '10px 14px', background: 'var(--rose-50)', color: 'var(--block-solid)', borderRadius: 'var(--radius-md)', font: 'var(--text-sm)', marginTop: 8 }}>
                      {submitError}
                    </div>
                  )}
                </>
              )}
            </>
          )}
        </div>

        <div style={{ padding: '14px 22px', borderTop: '1px solid var(--border)', display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
          <button onClick={onClose} className="btn btn-ghost" disabled={submitting}>
            {created ? 'Cerrar' : 'Cancelar'}
          </button>
          {!created && (
            <button onClick={handleSubmit} className="btn btn-primary" disabled={!isValid || submitting}>
              {submitting ? 'Creando...' : 'Crear tarea de corrección'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
