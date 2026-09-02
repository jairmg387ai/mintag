import { useState, useEffect, useCallback, Fragment } from 'react'
import { ChevronLeft, ChevronRight, Plus, Upload, CheckCircle, RotateCcw, RefreshCw, Pencil, Trash2, Settings, Eye, FileDown } from 'lucide-react'
import type { DailyActivity, ActivityCatalog, UploadResult, AzureTimeLogConfigStatus, AzureActivity } from '../../types'
import {
  listActivities,
  approveActivity,
  unapproveActivity,
  uploadActivities,
  getActivityCatalog,
  deleteActivity,
  getAzureTimeLogConfig,
  saveAzureTimeLogConfig,
  clearAzureTimeLogConfig,
  startAzureDeviceAuth,
  completeAzureDeviceAuth,
  listAzureActivities,
} from '../../api/client'
import { ActivityStatusBadge, ActivitySourceBadge } from './ActivityStatusBadge'
import { AzureActivityReference } from './AzureActivityLink'
import { NewActivityModal } from './NewActivityModal'
import { EditActivityModal } from './EditActivityModal'
import { CatalogManagementModal } from './CatalogManagementModal'
import { ActivityDetailModal } from './ActivityDetailModal'
import { ExportActivitiesModal } from './ExportActivitiesModal'
import { useAppActions } from '../../store/AppContext'

function toYMD(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${dd}`
}

function shiftDate(ymd: string, days: number): string {
  const [y, m, d] = ymd.split('-').map(Number)
  const date = new Date(y, m - 1, d)
  date.setDate(date.getDate() + days)
  return toYMD(date)
}

const REGISTRO_DIARIO_PREVIEW_LIMIT = 150

function truncateRegistroDiario(value: string): string {
  if (value.length <= REGISTRO_DIARIO_PREVIEW_LIMIT) return value
  return `${value.slice(0, REGISTRO_DIARIO_PREVIEW_LIMIT).trimEnd()}…`
}

export function ActivitiesView() {
  const { setAzureConfig: setGlobalAzureConfig } = useAppActions()
  const [date, setDate] = useState<string>(() => toYMD(new Date()))
  const [activities, setActivities] = useState<DailyActivity[]>([])
  const [catalog, setCatalog] = useState<ActivityCatalog | null>(null)
  const [azureActivities, setAzureActivities] = useState<AzureActivity[]>([])
  const [loading, setLoading] = useState(false)
  const [fetchError, setFetchError] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)
  const [uploadResult, setUploadResult] = useState<UploadResult | null>(null)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [showNewModal, setShowNewModal] = useState(false)
  const [editingActivity, setEditingActivity] = useState<DailyActivity | null>(null)
  const [viewingActivity, setViewingActivity] = useState<DailyActivity | null>(null)
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({})
  const [showCatalogModal, setShowCatalogModal] = useState(false)
  const [showExportModal, setShowExportModal] = useState(false)
  const [azureConfig, setAzureConfig] = useState<AzureTimeLogConfigStatus | null>(null)
  const [azureToken, setAzureToken] = useState('')
  const [azureAuthMode, setAzureAuthMode] = useState<'bearer' | 'basic'>('bearer')
  const [azureConfigMessage, setAzureConfigMessage] = useState<string | null>(null)
  const [deviceAuth, setDeviceAuth] = useState<{ device_code: string; user_code: string; verification_uri: string; message: string; expires_in: number; interval: number } | null>(null)
  const [deviceAuthLoading, setDeviceAuthLoading] = useState(false)

  // Returns the fetched rows (or undefined on failure) so callers that need
  // to react to the result — e.g. the day-complete auto-advance below — can,
  // without every other existing call site (which just discards the return
  // value) having to change.
  const fetchActivities = useCallback(async (d: string): Promise<DailyActivity[] | undefined> => {
    setLoading(true)
    setFetchError(null)
    try {
      const data = await listActivities(d)
      setActivities(data)
      return data
    } catch (e: unknown) {
      setFetchError(e instanceof Error ? e.message : 'No se pudieron cargar las actividades')
      return undefined
    } finally {
      setLoading(false)
    }
  }, [])

  const refreshAzureActivities = useCallback(() => {
    listAzureActivities().then(setAzureActivities).catch(() => setAzureActivities([]))
  }, [])

  // Catalogs load once on mount
  useEffect(() => {
    getActivityCatalog().then(setCatalog).catch(() => setCatalog(null))
    getAzureTimeLogConfig().then(cfg => {
      setAzureConfig(cfg)
      setGlobalAzureConfig(cfg)
      setAzureAuthMode(cfg.auth_mode === 'basic' ? 'basic' : 'bearer')
    }).catch(() => setAzureConfig(null))
    refreshAzureActivities()
  }, [refreshAzureActivities, setGlobalAzureConfig])

  // Re-fetch on date change (also covers initial load)
  useEffect(() => {
    fetchActivities(date)
    setUploadResult(null)
    setUploadError(null)
  }, [date, fetchActivities])

  const pending = activities.filter(a => a.status === 'pending')
  const approved = activities.filter(a => a.status === 'approved')
  const uploaded = activities.filter(a => a.status === 'uploaded')
  const totalHours = activities.reduce((s, a) => s + a.hours, 0)

  async function handleApprove(id: number) {
    setRowErrors(prev => { const n = { ...prev }; delete n[id]; return n })
    try {
      await approveActivity(id)
      await fetchActivities(date)
    } catch (e: unknown) {
      setRowErrors(prev => ({
        ...prev,
        [id]: e instanceof Error ? e.message : 'No se pudo aprobar la actividad',
      }))
    }
  }

  async function handleUnapprove(id: number) {
    setRowErrors(prev => { const n = { ...prev }; delete n[id]; return n })
    try {
      await unapproveActivity(id)
      await fetchActivities(date)
    } catch (e: unknown) {
      setRowErrors(prev => ({
        ...prev,
        [id]: e instanceof Error ? e.message : 'No se pudo revertir la aprobación',
      }))
    }
  }

  async function handleDelete(activity: DailyActivity) {
    const message = activity.status === 'uploaded'
      ? 'Esta actividad ya fue subida. Se eliminará de Azure TimeLog y también de Mintag. ¿Continuar?'
      : 'Se eliminará esta actividad de Mintag. ¿Continuar?'
    if (!window.confirm(message)) return

    const id = activity.id
    setRowErrors(prev => { const n = { ...prev }; delete n[id]; return n })
    try {
      await deleteActivity(id)
      await fetchActivities(date)
    } catch (e: unknown) {
      setRowErrors(prev => ({
        ...prev,
        [id]: e instanceof Error ? e.message : 'No se pudo eliminar la actividad',
      }))
    }
  }

  async function handleApproveAll() {
    const ids = pending.map(a => a.id)
    const results = await Promise.allSettled(ids.map(id => approveActivity(id)))
    const errs: Record<number, string> = {}
    results.forEach((r, i) => {
      if (r.status === 'rejected') {
        errs[ids[i]] = r.reason instanceof Error ? r.reason.message : 'No se pudo aprobar la actividad'
      }
    })
    setRowErrors(prev => ({ ...prev, ...errs }))
    await fetchActivities(date)
  }

  async function handleUpload() {
    setUploading(true)
    setUploadResult(null)
    setUploadError(null)
    try {
      const result = await uploadActivities(date)
      setUploadResult(result)
      await fetchActivities(date)
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'No se pudo subir a Azure'
      if (msg.includes('503') || msg.includes('Azure TimeLog token')) {
        setUploadError('El token de Azure TimeLog no está configurado')
      } else {
        setUploadError(msg)
      }
    } finally {
      setUploading(false)
    }
  }

  async function handleSaveAzureConfig() {
    setAzureConfigMessage(null)
    try {
      const cfg = await saveAzureTimeLogConfig({ token: azureToken, auth_mode: azureAuthMode })
      setAzureConfig(cfg)
      setGlobalAzureConfig(cfg)
      setAzureToken('')
      setAzureConfigMessage('Token de Azure guardado. El token queda oculto después de guardar.')
    } catch (e: unknown) {
      setAzureConfigMessage(e instanceof Error ? e.message : 'No se pudo guardar la configuración de Azure')
    }
  }

  async function handleClearAzureConfig() {
    setAzureConfigMessage(null)
    try {
      const cfg = await clearAzureTimeLogConfig()
      setAzureConfig(cfg)
      setGlobalAzureConfig(cfg)
      setAzureToken('')
      setAzureAuthMode(cfg.auth_mode === 'basic' ? 'basic' : 'bearer')
      setDeviceAuth(null)
      setAzureConfigMessage('Token local de Azure borrado.')
    } catch (e: unknown) {
      setAzureConfigMessage(e instanceof Error ? e.message : 'No se pudo borrar la configuración de Azure')
    }
  }

  async function handleStartDeviceAuth() {
    setAzureConfigMessage(null)
    setDeviceAuthLoading(true)
    try {
      const auth = await startAzureDeviceAuth()
      setDeviceAuth(auth)
      setAzureConfigMessage('Inicio de sesión con dispositivo iniciado. Abre la URL de verificación, ingresa el código y completa el inicio de sesión aquí.')
    } catch (e: unknown) {
      setAzureConfigMessage(e instanceof Error ? e.message : 'No se pudo iniciar sesión con Microsoft')
    } finally {
      setDeviceAuthLoading(false)
    }
  }

  async function handleCompleteDeviceAuth() {
    if (!deviceAuth) return
    setAzureConfigMessage(null)
    setDeviceAuthLoading(true)
    try {
      const result = await completeAzureDeviceAuth(deviceAuth.device_code)
      if (result.status === 'pending') {
        setAzureConfigMessage('La autorización sigue pendiente. Termina el inicio de sesión en el navegador e intenta completar de nuevo.')
        return
      }
      if (result.status !== 'complete') {
        setAzureConfigMessage(`Inicio de sesión con Microsoft: ${result.status}. Inicia de nuevo para obtener un código nuevo.`)
        return
      }
      const cfg = await getAzureTimeLogConfig()
      setAzureConfig(cfg)
      setGlobalAzureConfig(cfg)
      setDeviceAuth(null)
      setAzureConfigMessage('Inicio de sesión con Microsoft conectado. Mintag renovará el acceso automáticamente.')
    } catch (e: unknown) {
      setAzureConfigMessage(e instanceof Error ? e.message : 'No se pudo completar el inicio de sesión con Microsoft')
    } finally {
      setDeviceAuthLoading(false)
    }
  }

  const [_y, _m, _d] = date.split('-').map(Number)
  const displayDate = new Date(_y, _m - 1, _d).toLocaleDateString('es-CO', {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })

  return (
    <div className="content-pad">
      {/* Header row */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20 }}>
        <button
          className="btn btn-ghost btn-sm"
          onClick={() => setDate(d => shiftDate(d, -1))}
          aria-label="Día anterior"
        >
          <ChevronLeft size={16} strokeWidth={1.75} />
        </button>

        <h2 style={{ font: 'var(--text-h2)', color: 'var(--fg1)', margin: 0, minWidth: 260, textAlign: 'center' }}>
          {displayDate}
        </h2>

        <button
          className="btn btn-ghost btn-sm"
          onClick={() => setDate(d => shiftDate(d, 1))}
          aria-label="Día siguiente"
        >
          <ChevronRight size={16} strokeWidth={1.75} />
        </button>

        <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
          {pending.length > 1 && (
            <button
              className="btn btn-ghost btn-sm"
              onClick={handleApproveAll}
              title="Aprobar todas las actividades pendientes"
            >
              <CheckCircle size={15} strokeWidth={1.75} />
              Aprobar todo
            </button>
          )}
          <button
            className="btn btn-ghost btn-sm"
            onClick={() => setShowExportModal(true)}
            title="Exportar actividades a Excel"
            aria-label="Exportar actividades a Excel"
          >
            <FileDown size={15} strokeWidth={1.75} />
            Exportar
          </button>
          <button
            className="btn btn-ghost btn-sm"
            onClick={() => setShowCatalogModal(true)}
            title="Gestionar catálogo"
            aria-label="Gestionar catálogo"
          >
            <Settings size={15} strokeWidth={1.75} />
          </button>
          <button
            className="btn btn-primary btn-sm"
            onClick={() => setShowNewModal(true)}
          >
            <Plus size={15} strokeWidth={1.75} />
            Nueva Actividad
          </button>
        </div>
      </div>

      {/* Summary bar */}
      <div
        style={{
          display: 'flex',
          gap: 24,
          padding: '12px 16px',
          background: 'var(--bg-surface)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--radius-lg)',
          marginBottom: 20,
        }}
      >
        <SummaryItem label="Pendientes" value={pending.length} cls="chip-pending" />
        <SummaryItem label="Aprobadas" value={approved.length} cls="chip-prog" />
        <SummaryItem label="Subidas" value={uploaded.length} cls="chip-done" />
        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 6 }}>
          <span style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>
            Total de horas
          </span>
          <span style={{ font: 'var(--text-h3)', color: 'var(--fg1)' }}>
            {totalHours.toFixed(1)}
          </span>
        </div>
      </div>

      {/* Table area */}
      <div className="card" style={{ marginBottom: 20 }}>
        {loading ? (
          <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--fg3)' }}>
            <RefreshCw size={24} strokeWidth={1.75} style={{ animation: 'spin 1s linear infinite', display: 'inline-block' }} />
            <div style={{ marginTop: 8, font: 'var(--text-body)' }}>Cargando actividades...</div>
          </div>
        ) : fetchError ? (
          <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--block-solid)' }}>
            <div style={{ font: 'var(--text-body)', marginBottom: 12 }}>{fetchError}</div>
            <button className="btn btn-secondary btn-sm" onClick={() => fetchActivities(date)}>
              Reintentar
            </button>
          </div>
        ) : activities.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--fg3)', font: 'var(--text-body)' }}>
            No hay actividades para este día. Agrega una con "Nueva Actividad".
          </div>
        ) : (
          <table className="mt-table">
            <thead>
              <tr>
                <th>PROYECTO</th>
                <th>HRS</th>
                <th>CATEGORÍA</th>
                <th>REGISTRO DIARIO</th>
                <th>ORIGEN</th>
                <th>AZURE</th>
                <th>ESTADO</th>
                <th>ACCIONES</th>
              </tr>
            </thead>
            <tbody>
              {activities.map(a => (
                <Fragment key={a.id}>
                  <tr>
                    <td style={{ font: 'var(--text-h4)', color: 'var(--fg1)' }}>{a.project}</td>
                    <td style={{ font: 'var(--text-mono)', color: 'var(--fg1)' }}>{a.hours.toFixed(2)}</td>
                    <td style={{ color: 'var(--fg2)' }}>{a.category}</td>
                    <td style={{ color: 'var(--fg2)', maxWidth: 320 }}>
                      <span title={a.registro_diario} style={{ display: 'block', whiteSpace: 'normal', wordBreak: 'break-word' }}>
                        {truncateRegistroDiario(a.registro_diario)}
                      </span>
                    </td>
                    <td><ActivitySourceBadge source={a.source} /></td>
                    <td>
                      <AzureActivityReference
                        azureActivityId={a.azure_activity_id}
                        azureActivities={azureActivities}
                        style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}
                      />
                    </td>
                    <td><ActivityStatusBadge status={a.status} /></td>
                    <td>
                      <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                        <button
                          className="btn btn-ghost btn-sm"
                          onClick={() => setViewingActivity(a)}
                          title="Ver detalle"
                          aria-label="Ver detalle de la actividad"
                        >
                          <Eye size={15} strokeWidth={1.75} />
                        </button>
                        {a.status !== 'uploaded' && (
                          <button
                            className="btn btn-ghost btn-sm"
                            onClick={() => setEditingActivity(a)}
                            title="Editar"
                            aria-label="Editar actividad"
                          >
                            <Pencil size={15} strokeWidth={1.75} />
                          </button>
                        )}
                        {a.status === 'pending' && (
                          <>
                            <button
                              className="btn btn-ghost btn-sm"
                              onClick={() => handleApprove(a.id)}
                              title="Aprobar"
                              aria-label="Aprobar actividad"
                            >
                              <CheckCircle size={15} strokeWidth={1.75} />
                            </button>
                          </>
                        )}
                        {a.status === 'approved' && (
                          <>
                            <button
                              className="btn btn-ghost btn-sm"
                              onClick={() => handleUnapprove(a.id)}
                              title="Revertir aprobación"
                              aria-label="Revertir aprobación de la actividad"
                            >
                              <RotateCcw size={15} strokeWidth={1.75} />
                            </button>
                          </>
                        )}
                        <button
                          className="btn btn-ghost btn-sm"
                          onClick={() => handleDelete(a)}
                          title={a.status === 'uploaded' ? 'Eliminar en Azure y Mintag' : 'Eliminar'}
                          aria-label={a.status === 'uploaded' ? 'Eliminar actividad en Azure y Mintag' : 'Eliminar actividad'}
                        >
                          <Trash2 size={15} strokeWidth={1.75} />
                        </button>
                      </div>
                    </td>
                  </tr>
                  {rowErrors[a.id] && (
                    <tr>
                      <td colSpan={8} style={{ padding: '4px 16px 10px', color: 'var(--block-solid)', font: 'var(--text-caption)' }}>
                        {rowErrors[a.id]}
                      </td>
                    </tr>
                  )}
                </Fragment>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Upload section */}
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          gap: 12,
          padding: '16px 20px',
          background: 'var(--bg-surface)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--radius-lg)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <button
            className="btn btn-secondary"
            onClick={handleUpload}
            disabled={approved.length === 0 || uploading}
          >
            {uploading ? (
              <RefreshCw size={16} strokeWidth={1.75} style={{ animation: 'spin 1s linear infinite' }} />
            ) : (
              <Upload size={16} strokeWidth={1.75} />
            )}
            {uploading ? 'Subiendo...' : 'Subir a Azure'}
          </button>
          <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
            {approved.length} {approved.length === 1 ? 'registro aprobado listo' : 'registros aprobados listos'} para subir
          </span>
        </div>

        {uploadResult && (
          <div
            style={{
              padding: '10px 14px',
              background: uploadResult.failed_ids.length > 0 ? 'var(--amber-50)' : 'var(--done-bg)',
              color: uploadResult.failed_ids.length > 0 ? 'var(--amber-700)' : 'var(--done-fg)',
              borderRadius: 'var(--radius-md)',
              font: 'var(--text-sm)',
            }}
          >
            <span>{uploadResult.uploaded_count} {uploadResult.uploaded_count === 1 ? 'actividad subida' : 'actividades subidas'}</span>
            {uploadResult.failed_ids.length > 0 && (
              <div style={{ marginTop: 4 }}>
                IDs con error: {uploadResult.failed_ids.join(', ')} — {uploadResult.errors.join(', ')}
              </div>
            )}
          </div>
        )}

        {uploadError && (
          <div
            style={{
              padding: '10px 14px',
              background: 'var(--amber-50)',
              color: 'var(--amber-700)',
              borderRadius: 'var(--radius-md)',
              font: 'var(--text-sm)',
            }}
          >
            {uploadError}
          </div>
        )}

        <div style={{ borderTop: '1px solid var(--border)', paddingTop: 12, display: 'flex', flexDirection: 'column', gap: 10 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
            <span style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>
              Autenticación de Azure
            </span>
            <span className={`chip ${azureConfig?.configured ? 'chip-done' : 'chip-pending'}`}>
              {azureConfig?.configured ? `Configurada (${azureConfig.auth_mode})` : 'No configurada'}
            </span>
            {azureConfig?.oauth_connected && (
              <span className="chip chip-done">Microsoft conectado</span>
            )}
            {azureConfig?.source && azureConfig.source !== 'none' && (
              <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>Origen: {azureConfig.source}</span>
            )}
            {azureConfig?.oauth_access_token_expires_at && (
              <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
                Acceso válido hasta: {new Date(azureConfig.oauth_access_token_expires_at).toLocaleString()}
              </span>
            )}
          </div>
          <div style={{ padding: '12px', border: '1px solid var(--border)', borderRadius: 'var(--radius-md)', display: 'flex', flexDirection: 'column', gap: 10 }}>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
              <strong style={{ font: 'var(--text-body)', color: 'var(--fg1)' }}>Conectar con Microsoft</strong>
              <button className="btn btn-secondary btn-sm" onClick={handleStartDeviceAuth} disabled={deviceAuthLoading}>
                {deviceAuthLoading ? 'Procesando...' : 'Iniciar sesión con dispositivo'}
              </button>
              {deviceAuth && (
                <button className="btn btn-primary btn-sm" onClick={handleCompleteDeviceAuth} disabled={deviceAuthLoading}>
                  Completar inicio de sesión
                </button>
              )}
            </div>
            <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
              Recomendado para Mintag. Guarda las credenciales de renovación localmente y no muestra tokens.
            </div>
            {deviceAuth && (
              <div style={{ padding: '10px 12px', background: 'var(--bg2)', borderRadius: 'var(--radius-md)', display: 'grid', gap: 6 }}>
                <div style={{ font: 'var(--text-caption)', color: 'var(--fg2)' }}>{deviceAuth.message}</div>
                <div style={{ font: 'var(--text-body)', color: 'var(--fg1)' }}>
                  URL: <a href={deviceAuth.verification_uri} target="_blank" rel="noreferrer">{deviceAuth.verification_uri}</a>
                </div>
                <div style={{ font: 'var(--text-h3)', color: 'var(--fg1)' }}>Código: {deviceAuth.user_code}</div>
                <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
                  Expira en {Math.round(deviceAuth.expires_in / 60)} minutos. Intervalo de consulta: {deviceAuth.interval}s.
                </div>
              </div>
            )}
          </div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
            <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)', width: '100%' }}>Respaldo con token manual</span>
            <select
              className="input"
              value={azureAuthMode}
              onChange={e => setAzureAuthMode(e.target.value as 'bearer' | 'basic')}
              style={{ width: 120 }}
            >
              <option value="bearer">Bearer</option>
              <option value="basic">PAT básico</option>
            </select>
            <input
              className="input"
              type="password"
              placeholder="Pega un token nuevo"
              value={azureToken}
              onChange={e => setAzureToken(e.target.value)}
              style={{ minWidth: 280 }}
            />
            <button className="btn btn-secondary btn-sm" onClick={handleSaveAzureConfig} disabled={azureToken.trim() === ''}>
              Guardar token
            </button>
            <button className="btn btn-ghost btn-sm" onClick={handleClearAzureConfig}>
              Borrar token local
            </button>
          </div>
          {azureAuthMode === 'bearer' && (
            <div style={{ font: 'var(--text-caption)', color: 'var(--amber-700)' }}>
              Los tokens Bearer expiran. Si Azure solicita iniciar sesión, reemplaza este token por uno vigente.
            </div>
          )}
          {azureConfigMessage && (
            <div style={{ font: 'var(--text-caption)', color: azureConfigMessage.includes('Failed') ? 'var(--block-solid)' : 'var(--fg3)' }}>
              {azureConfigMessage}
            </div>
          )}
        </div>
      </div>

      <EditActivityModal
        activity={editingActivity}
        open={editingActivity !== null}
        onClose={() => setEditingActivity(null)}
        onSaved={() => {
          setEditingActivity(null)
          fetchActivities(date)
        }}
        catalog={catalog}
        azureActivities={azureActivities}
      />

      <NewActivityModal
        open={showNewModal}
        onClose={() => setShowNewModal(false)}
        onCreated={async () => {
          setShowNewModal(false)
          await fetchActivities(date)
        }}
        catalog={catalog}
        defaultDate={date}
        azureActivities={azureActivities}
      />

      <CatalogManagementModal
        open={showCatalogModal}
        onClose={() => setShowCatalogModal(false)}
        catalog={catalog}
        onCatalogChanged={() => {
          getActivityCatalog().then(setCatalog).catch(() => setCatalog(null))
        }}
      />

      <ActivityDetailModal
        activity={viewingActivity}
        open={viewingActivity !== null}
        onClose={() => setViewingActivity(null)}
        azureActivities={azureActivities}
      />

      <ExportActivitiesModal
        open={showExportModal}
        onClose={() => setShowExportModal(false)}
        defaultDate={date}
      />

    </div>
  )
}

function SummaryItem({ label, value, cls }: { label: string; value: number; cls: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <span style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>
        {label}
      </span>
      <span className={`chip ${cls}`} style={{ minWidth: 28, justifyContent: 'center' }}>
        {value}
      </span>
    </div>
  )
}
