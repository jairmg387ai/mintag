import { useRef, type CSSProperties } from 'react'
import { X } from 'lucide-react'
import type { DailyActivity, AzureActivity } from '../../types'
import { ActivityStatusBadge, ActivitySourceBadge } from './ActivityStatusBadge'
import { AzureActivityReference } from './AzureActivityLink'

interface ActivityDetailModalProps {
  activity: DailyActivity | null
  open: boolean
  onClose: () => void
  azureActivities: AzureActivity[]
}

const valueStyle: CSSProperties = {
  font: 'var(--text-body)',
  color: 'var(--fg1)',
}

function DetailField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 14 }}>
      <div
        style={{
          font: 'var(--text-label)',
          color: 'var(--fg3)',
          textTransform: 'uppercase',
          letterSpacing: 'var(--tracking-label)',
          marginBottom: 6,
        }}
      >
        {label}
      </div>
      {children}
    </div>
  )
}

export function ActivityDetailModal({ activity, open, onClose, azureActivities }: ActivityDetailModalProps) {
  const pressedOnOverlay = useRef(false)

  if (!open || !activity) return null

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
        onClick={e => e.stopPropagation()}
        className="card"
        style={{
          borderRadius: 'var(--radius-xl)',
          boxShadow: 'var(--shadow-xl)',
          width: '100%',
          maxWidth: 580,
          maxHeight: '90vh',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: '16px 20px',
            borderBottom: '1px solid var(--border)',
            display: 'flex',
            alignItems: 'center',
            gap: 10,
          }}
        >
          <h2 style={{ font: 'var(--text-h3)', color: 'var(--fg1)', flex: 1, margin: 0 }}>
            Detalle del registro
          </h2>
          <button
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--fg3)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              padding: 4,
              borderRadius: 'var(--radius-md)',
            }}
            aria-label="Cerrar"
          >
            <X size={18} strokeWidth={1.75} />
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: '20px 22px', overflowY: 'auto', flex: 1 }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <DetailField label="ID">
              <div style={{ ...valueStyle, font: 'var(--text-mono)' }}>{activity.id}</div>
            </DetailField>
            <DetailField label="Fecha">
              <div style={valueStyle}>{activity.date}</div>
            </DetailField>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <DetailField label="Proyecto">
              <div style={valueStyle}>{activity.project}</div>
            </DetailField>
            <DetailField label="Horas">
              <div style={{ ...valueStyle, font: 'var(--text-mono)' }}>{activity.hours.toFixed(2)}</div>
            </DetailField>
          </div>

          <DetailField label="Categoría">
            <div style={valueStyle}>{activity.category}</div>
          </DetailField>

          <DetailField label="Registro diario">
            <div
              style={{
                ...valueStyle,
                whiteSpace: 'normal',
                wordBreak: 'break-word',
                padding: '10px 12px',
                background: 'var(--bg-sunken)',
                border: '1px solid var(--border)',
                borderRadius: 'var(--radius-md)',
              }}
            >
              {activity.registro_diario}
            </div>
          </DetailField>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <DetailField label="Origen">
              <ActivitySourceBadge source={activity.source} />
            </DetailField>
            <DetailField label="Estado">
              <ActivityStatusBadge status={activity.status} />
            </DetailField>
          </div>

          <DetailField label="Actividad de Azure">
            <div style={valueStyle}>
              <AzureActivityReference
                azureActivityId={activity.azure_activity_id}
                azureActivities={azureActivities}
              />
            </div>
          </DetailField>

          <DetailField label="Creado">
            <div style={valueStyle}>{new Date(activity.created_at).toLocaleString()}</div>
          </DetailField>

          {activity.uploaded_at && (
            <DetailField label="Subido">
              <div style={valueStyle}>{new Date(activity.uploaded_at).toLocaleString()}</div>
            </DetailField>
          )}

          {activity.azure_document_id && (
            <DetailField label="ID de documento Azure">
              <div style={{ ...valueStyle, font: 'var(--text-mono)' }}>{activity.azure_document_id}</div>
            </DetailField>
          )}
        </div>

        {/* Footer */}
        <div
          style={{
            padding: '14px 22px',
            borderTop: '1px solid var(--border)',
            display: 'flex',
            gap: 8,
            justifyContent: 'flex-end',
          }}
        >
          <button onClick={onClose} className="btn btn-ghost">
            Cerrar
          </button>
        </div>
      </div>
    </div>
  )
}
