import { useState, useEffect, useCallback, Fragment } from 'react'
import { ChevronLeft, ChevronRight, Plus, Upload, CheckCircle, RotateCcw, RefreshCw, Pencil, Trash2, Settings } from 'lucide-react'
import type { DailyActivity, ActivityCatalog, UploadResult } from '../../types'
import {
  listActivities,
  approveActivity,
  unapproveActivity,
  uploadActivities,
  getActivityCatalog,
  deleteActivity,
} from '../../api/client'
import { ActivityStatusBadge, ActivitySourceBadge } from './ActivityStatusBadge'
import { NewActivityModal } from './NewActivityModal'
import { EditActivityModal } from './EditActivityModal'
import { CatalogManagementModal } from './CatalogManagementModal'

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

export function ActivitiesView() {
  const [date, setDate] = useState<string>(() => toYMD(new Date()))
  const [activities, setActivities] = useState<DailyActivity[]>([])
  const [catalog, setCatalog] = useState<ActivityCatalog | null>(null)
  const [loading, setLoading] = useState(false)
  const [fetchError, setFetchError] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)
  const [uploadResult, setUploadResult] = useState<UploadResult | null>(null)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [showNewModal, setShowNewModal] = useState(false)
  const [editingActivity, setEditingActivity] = useState<DailyActivity | null>(null)
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({})
  const [showCatalogModal, setShowCatalogModal] = useState(false)

  const fetchActivities = useCallback(async (d: string) => {
    setLoading(true)
    setFetchError(null)
    try {
      const data = await listActivities(d)
      setActivities(data)
    } catch (e: unknown) {
      setFetchError(e instanceof Error ? e.message : 'Failed to load activities')
    } finally {
      setLoading(false)
    }
  }, [])

  // Catalog loads once on mount
  useEffect(() => {
    getActivityCatalog().then(setCatalog).catch(() => setCatalog(null))
  }, [])

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
        [id]: e instanceof Error ? e.message : 'Approval failed',
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
        [id]: e instanceof Error ? e.message : 'Unapprove failed',
      }))
    }
  }

  async function handleDelete(id: number) {
    setRowErrors(prev => { const n = { ...prev }; delete n[id]; return n })
    try {
      await deleteActivity(id)
      await fetchActivities(date)
    } catch (e: unknown) {
      setRowErrors(prev => ({
        ...prev,
        [id]: e instanceof Error ? e.message : 'Delete failed',
      }))
    }
  }

  async function handleApproveAll() {
    const ids = pending.map(a => a.id)
    const results = await Promise.allSettled(ids.map(id => approveActivity(id)))
    const errs: Record<number, string> = {}
    results.forEach((r, i) => {
      if (r.status === 'rejected') {
        errs[ids[i]] = r.reason instanceof Error ? r.reason.message : 'Approval failed'
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
      const msg = e instanceof Error ? e.message : 'Upload failed'
      if (msg.includes('503') || msg.includes('MINTAG_AZURE_TIMELOG_PAT')) {
        setUploadError('MINTAG_AZURE_TIMELOG_PAT not configured')
      } else {
        setUploadError(msg)
      }
    } finally {
      setUploading(false)
    }
  }

  const [_y, _m, _d] = date.split('-').map(Number)
  const displayDate = new Date(_y, _m - 1, _d).toLocaleDateString(undefined, {
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
          aria-label="Previous day"
        >
          <ChevronLeft size={16} strokeWidth={1.75} />
        </button>

        <h2 style={{ font: 'var(--text-h2)', color: 'var(--fg1)', margin: 0, minWidth: 260, textAlign: 'center' }}>
          {displayDate}
        </h2>

        <button
          className="btn btn-ghost btn-sm"
          onClick={() => setDate(d => shiftDate(d, 1))}
          aria-label="Next day"
        >
          <ChevronRight size={16} strokeWidth={1.75} />
        </button>

        <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
          {pending.length > 1 && (
            <button
              className="btn btn-ghost btn-sm"
              onClick={handleApproveAll}
              title="Approve all pending activities"
            >
              <CheckCircle size={15} strokeWidth={1.75} />
              Approve All
            </button>
          )}
          <button
            className="btn btn-ghost btn-sm"
            onClick={() => setShowCatalogModal(true)}
            title="Manage catalog"
            aria-label="Manage catalog"
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
        <SummaryItem label="Pending" value={pending.length} cls="chip-pending" />
        <SummaryItem label="Approved" value={approved.length} cls="chip-prog" />
        <SummaryItem label="Uploaded" value={uploaded.length} cls="chip-done" />
        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 6 }}>
          <span style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>
            Total Hours
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
            <div style={{ marginTop: 8, font: 'var(--text-body)' }}>Loading activities...</div>
          </div>
        ) : fetchError ? (
          <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--block-solid)' }}>
            <div style={{ font: 'var(--text-body)', marginBottom: 12 }}>{fetchError}</div>
            <button className="btn btn-secondary btn-sm" onClick={() => fetchActivities(date)}>
              Retry
            </button>
          </div>
        ) : activities.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--fg3)', font: 'var(--text-body)' }}>
            No activities for this day. Add one with "Nueva Actividad".
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
                      <span style={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {a.registro_diario}
                      </span>
                    </td>
                    <td><ActivitySourceBadge source={a.source} /></td>
                    <td><ActivityStatusBadge status={a.status} /></td>
                    <td>
                      <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                        {a.status === 'pending' && (
                          <>
                            <button
                              className="btn btn-ghost btn-sm"
                              onClick={() => setEditingActivity(a)}
                              title="Edit"
                              aria-label="Edit activity"
                            >
                              <Pencil size={15} strokeWidth={1.75} />
                            </button>
                            <button
                              className="btn btn-ghost btn-sm"
                              onClick={() => handleApprove(a.id)}
                              title="Approve"
                              aria-label="Approve activity"
                            >
                              <CheckCircle size={15} strokeWidth={1.75} />
                            </button>
                            <button
                              className="btn btn-ghost btn-sm"
                              onClick={() => handleDelete(a.id)}
                              title="Delete"
                              aria-label="Delete activity"
                            >
                              <Trash2 size={15} strokeWidth={1.75} />
                            </button>
                          </>
                        )}
                        {a.status === 'approved' && (
                          <>
                            <button
                              className="btn btn-ghost btn-sm"
                              onClick={() => handleUnapprove(a.id)}
                              title="Unapprove"
                              aria-label="Unapprove activity"
                            >
                              <RotateCcw size={15} strokeWidth={1.75} />
                            </button>
                            <button
                              className="btn btn-ghost btn-sm"
                              onClick={() => handleDelete(a.id)}
                              title="Delete"
                              aria-label="Delete activity"
                            >
                              <Trash2 size={15} strokeWidth={1.75} />
                            </button>
                          </>
                        )}
                      </div>
                    </td>
                  </tr>
                  {rowErrors[a.id] && (
                    <tr>
                      <td colSpan={7} style={{ padding: '4px 16px 10px', color: 'var(--block-solid)', font: 'var(--text-caption)' }}>
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
            {uploading ? 'Uploading...' : 'Subir a Azure'}
          </button>
          <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
            {approved.length} approved {approved.length === 1 ? 'entry' : 'entries'} ready to upload
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
            <span>{uploadResult.uploaded_count} {uploadResult.uploaded_count === 1 ? 'activity' : 'activities'} uploaded</span>
            {uploadResult.failed_ids.length > 0 && (
              <div style={{ marginTop: 4 }}>
                Failed IDs: {uploadResult.failed_ids.join(', ')} — {uploadResult.errors.join(', ')}
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
      />

      <NewActivityModal
        open={showNewModal}
        onClose={() => setShowNewModal(false)}
        onCreated={() => {
          setShowNewModal(false)
          fetchActivities(date)
        }}
        catalog={catalog}
        defaultDate={date}
      />

      <CatalogManagementModal
        open={showCatalogModal}
        onClose={() => setShowCatalogModal(false)}
        catalog={catalog}
        onCatalogChanged={() => {
          getActivityCatalog().then(setCatalog).catch(() => setCatalog(null))
        }}
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
