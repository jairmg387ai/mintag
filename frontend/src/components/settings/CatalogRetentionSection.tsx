import { useEffect, useState } from 'react'
import { getCatalogRetention, updateCatalogRetention } from '../../api/client'

// Mirrors CategorySection's description-field save UX in
// CatalogManagementModal.tsx (save on blur, no explicit button) rather than
// MenuOptionsSection's immediate-on-change toggle — a debounced/blur save
// fits a free-typed number input better than an instant save on every
// keystroke.
export function CatalogRetentionSection() {
  const [bugDays, setBugDays] = useState('')
  const [projectDays, setProjectDays] = useState('')
  const [loading, setLoading] = useState(true)
  const [savingBug, setSavingBug] = useState(false)
  const [savingProject, setSavingProject] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    getCatalogRetention()
      .then(settings => {
        setBugDays(settings.bug_retention_days != null ? String(settings.bug_retention_days) : '')
        setProjectDays(settings.project_retention_days != null ? String(settings.project_retention_days) : '')
      })
      .catch(() => setError('No se pudo cargar la configuración de retención'))
      .finally(() => setLoading(false))
  }, [])

  function parseDays(value: string): number | null {
    const trimmed = value.trim()
    if (trimmed === '') return null
    const n = parseInt(trimmed, 10)
    return isNaN(n) || n <= 0 ? null : n
  }

  async function handleBlur(field: 'bug' | 'project') {
    const setSaving = field === 'bug' ? setSavingBug : setSavingProject
    setSaving(true)
    setError('')
    setSaved(false)
    try {
      const payload = {
        bug_retention_days: parseDays(bugDays),
        project_retention_days: parseDays(projectDays),
      }
      const result = await updateCatalogRetention(payload)
      setBugDays(result.bug_retention_days != null ? String(result.bug_retention_days) : '')
      setProjectDays(result.project_retention_days != null ? String(result.project_retention_days) : '')
      setSaved(true)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo guardar la configuración de retención')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
        Cargando…
      </div>
    )
  }

  const inputStyle = {
    width: 120,
    padding: '8px 12px',
    border: '1px solid var(--border-strong)',
    borderRadius: 'var(--radius-md)',
    font: 'var(--text-body)',
    color: 'var(--fg1)',
    background: 'var(--bg-sunken)',
    outline: 'none',
    boxSizing: 'border-box' as const,
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        <label htmlFor="bug-retention-days" style={{ font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
          Retención de bugs (días)
        </label>
        <input
          id="bug-retention-days"
          type="number"
          min={1}
          value={bugDays}
          onChange={e => setBugDays(e.target.value)}
          onBlur={() => handleBlur('bug')}
          disabled={savingBug}
          placeholder="Sin límite"
          style={inputStyle}
        />
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        <label htmlFor="project-retention-days" style={{ font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
          Retención de proyectos (días)
        </label>
        <input
          id="project-retention-days"
          type="number"
          min={1}
          value={projectDays}
          onChange={e => setProjectDays(e.target.value)}
          onBlur={() => handleBlur('project')}
          disabled={savingProject}
          placeholder="Sin límite"
          style={inputStyle}
        />
      </div>

      <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
        Solo afecta work items de tipo Bug en el catálogo de Azure y a los proyectos del catálogo de actividades — nunca a los de tipo Task.
        Deja el campo vacío para desactivar la retención automática de ese catálogo. Las entradas que se desactivan por vencimiento
        no se eliminan: se marcan como inactivas y conservan su historial.
      </div>

      {error && (
        <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)' }}>
          {error}
        </div>
      )}
      {!error && saved && (
        <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
          Guardado.
        </div>
      )}
    </div>
  )
}
