import type { CSSProperties, ReactNode } from 'react'
import type { AzureActivity } from '../../types'
import {
  azureWorkItemUrl,
  formatAzureActivityLabel,
  resolveAzureActivity,
  resolveAzureActivityLabel,
} from './azureActivity'

interface AzureWorkItemLinkProps {
  activity: AzureActivity
  children?: ReactNode
  className?: string
  style?: CSSProperties
}

export function AzureWorkItemLink({ activity, children, className, style }: AzureWorkItemLinkProps) {
  const label = formatAzureActivityLabel(activity)
  return (
    <a
      href={azureWorkItemUrl(activity)}
      target="_blank"
      rel="noreferrer"
      className={className}
      style={{ color: 'inherit', textDecoration: 'underline', textUnderlineOffset: 2, ...style }}
      title={`Abrir work item de Azure #${activity.work_item_id}`}
      aria-label={`Abrir work item de Azure #${activity.work_item_id} en una pestaña nueva`}
    >
      {children ?? label}
    </a>
  )
}

interface AzureActivityReferenceProps {
  azureActivityId: number | null | undefined
  azureActivities: AzureActivity[]
  style?: CSSProperties
}

export function AzureActivityReference({ azureActivityId, azureActivities, style }: AzureActivityReferenceProps) {
  const activity = resolveAzureActivity(azureActivityId, azureActivities)
  if (activity) {
    return (
      <AzureWorkItemLink activity={activity} style={style}>
        {formatAzureActivityLabel(activity)}
      </AzureWorkItemLink>
    )
  }

  return <span style={style}>{resolveAzureActivityLabel(azureActivityId, azureActivities)}</span>
}
