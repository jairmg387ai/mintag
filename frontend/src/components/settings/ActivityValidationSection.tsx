import { useEffect, useState } from 'react'
import type { ActivityValidationSettings } from '../../types'
import { getActivityValidationSettings, updateActivityValidationSettings } from '../../api/client'

const TOGGLES: Array<{ key: keyof ActivityValidationSettings; label: string; caption: string }> = [
  {
    key: 'max_hours_per_entry',
    label: 'Máximo 8 horas por registro',
    caption: 'Rechaza un registro de actividad individual con más de 8 horas.',
  },
  {
    key: 'weekend_confirm',
    label: 'Confirmar fin de semana',
    caption: 'Pide confirmación antes de registrar horas en sábado o domingo.',
  },
  {
    key: 'block_closed_work_item',
    label: 'Bloquear work item cerrado',
    caption: 'Impide vincular horas a un work item de Azure que ya está Cerrado.',
  },
]

// Mirrors MenuOptionsSection's immediate-on-change toggle pattern (save on
// every change, no explicit button) — fits a boolean checkbox better than
// CatalogRetentionSection's blur-save-number UX.
export function ActivityValidationSection() {
  const [settings, setSettings] = useState<ActivityValidationSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [busyKey, setBusyKey] = useState<string | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getActivityValidationSettings()
      .then(setSettings)
      .catch(() => setError('No se pudo cargar la configuración de validaciones'))
      .finally(() => setLoading(false))
  }, [])

  async function handleToggle(key: keyof ActivityValidationSettings, checked: boolean) {
    if (!settings) return
    setBusyKey(key)
    setError('')
    const next = { ...settings, [key]: checked }
    try {
      const result = await updateActivityValidationSettings(next)
      setSettings(result)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo guardar la configuración de validaciones')
    } finally {
      setBusyKey(null)
    }
  }

  if (loading) {
    return (
      <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
        Cargando…
      </div>
    )
  }

  if (!settings) {
    return (
      <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)' }}>
        {error || 'No se pudo cargar la configuración de validaciones'}
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      {TOGGLES.map(t => (
        <label
          key={t.key}
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: 10,
            padding: '8px 10px',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border)',
            background: 'var(--bg-sunken)',
            cursor: busyKey === t.key ? 'default' : 'pointer',
          }}
        >
          <input
            type="checkbox"
            checked={settings[t.key]}
            disabled={busyKey === t.key}
            onChange={e => handleToggle(t.key, e.target.checked)}
            aria-label={t.label}
            style={{ marginTop: 3 }}
          />
          <span style={{ flex: 1 }}>
            <span style={{ display: 'block', font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
              {t.label}
            </span>
            <span style={{ display: 'block', font: 'var(--text-caption)', color: 'var(--fg3)' }}>
              {t.caption}
            </span>
          </span>
        </label>
      ))}

      {error && (
        <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)' }}>
          {error}
        </div>
      )}
    </div>
  )
}
