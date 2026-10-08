import { useEffect, useState } from 'react'
import type { WorkItemEffort } from '../types'
import { fetchWorkItemEffort } from '../api/client'

export interface WorkItemEffortState {
  // Effort keyed by work item id; empty while loading or after a failure.
  byId: Record<number, WorkItemEffort>
  loading: boolean
  // Set when the call failed.
  error: string
}

interface FetchResult {
  // The request (ids + reloadKey) this result answers.
  requestKey: string
  byId: Record<number, WorkItemEffort>
  error: string
}

const NO_RESULT: FetchResult = { requestKey: '', byId: {}, error: '' }

// useWorkItemEffort loads effort for the given work item ids, refetching when
// the set of ids or reloadKey changes. A result is only exposed while it still
// matches the current request, so stale responses never leak into the UI.
export function useWorkItemEffort(ids: number[], reloadKey: number = 0): WorkItemEffortState {
  const idsKey = ids.filter(id => id > 0).join(',')
  const requestKey = idsKey ? `${idsKey}#${reloadKey}` : ''
  const [result, setResult] = useState<FetchResult>(NO_RESULT)

  useEffect(() => {
    if (!idsKey) return
    let cancelled = false
    fetchWorkItemEffort(idsKey.split(',').map(Number))
      .then(res => {
        if (cancelled) return
        const byId: Record<number, WorkItemEffort> = {}
        for (const item of res?.items ?? []) byId[item.id] = item
        setResult({ requestKey, byId, error: '' })
      })
      .catch((e: unknown) => {
        if (cancelled) return
        setResult({ requestKey, byId: {}, error: e instanceof Error ? e.message : 'error' })
      })
    return () => { cancelled = true }
  }, [idsKey, requestKey])

  if (!requestKey) return { byId: {}, loading: false, error: '' }
  if (result.requestKey !== requestKey) return { byId: {}, loading: true, error: '' }
  return { byId: result.byId, loading: false, error: result.error }
}
