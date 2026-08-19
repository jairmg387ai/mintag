import type { AzureActivity } from '../../types'

// Shared by resolveAzureActivityLabel below and by the "Use default (...)"
// option in NewActivityModal/EditActivityModal, so there's one place that
// decides what counts as "the default" instead of each caller re-deriving
// it with its own .find(a => a.is_default).
export function findDefaultAzureActivity(azureActivities: AzureActivity[]): AzureActivity | undefined {
  return azureActivities.find(a => a.is_default)
}

export function azureWorkItemUrl(a: Pick<AzureActivity, 'org' | 'work_item_id'>): string {
  const orgPath = a.org
    .trim()
    .split('/')
    .map(segment => encodeURIComponent(segment))
    .join('/')
  return `https://dev.azure.com/${orgPath}/_workitems/edit/${a.work_item_id}`
}

// Shared formatting so every combo/display shows the same "label (#work
// item id)" shape — the label alone doesn't identify the Azure work item.
export function formatAzureActivityLabel(a: AzureActivity): string {
  return `${a.label} (#${a.work_item_id})`
}

// Case-insensitive substring match against the same "label (#work item id)"
// string the user sees (formatAzureActivityLabel), so one comparison covers
// matching the title, the bare id ("4521"), and the "#4521" display form.
// Inactive items are defensively excluded even though the server-side
// catalog list already returns active-only rows.
export function azureActivityMatches(a: AzureActivity, query: string): boolean {
  if (a.is_active === false) return false
  const q = query.trim().toLowerCase()
  if (!q) return true
  return formatAzureActivityLabel(a).toLowerCase().includes(q)
}

export function filterAzureActivities(list: AzureActivity[], query: string): AzureActivity[] {
  return list.filter(a => azureActivityMatches(a, query))
}

export function resolveAzureActivity(
  azureActivityId: number | null | undefined,
  azureActivities: AzureActivity[],
): AzureActivity | undefined {
  if (azureActivityId == null) return findDefaultAzureActivity(azureActivities)
  return azureActivities.find(a => a.id === azureActivityId)
}

// Resolves the display label for a daily activity's assigned Azure work
// item. A null/undefined azure_activity_id means the activity uploads to
// whichever azure_activities row is currently the default at upload time.
export function resolveAzureActivityLabel(
  azureActivityId: number | null | undefined,
  azureActivities: AzureActivity[],
): string {
  const match = resolveAzureActivity(azureActivityId, azureActivities)
  if (match) return formatAzureActivityLabel(match)
  return azureActivityId == null ? 'Predeterminada' : `#${azureActivityId}`
}

// Most azure_activities store-layer errors are already user-appropriate
// domain messages (e.g. "org is required", "azure activity 5 is inactive
// and cannot be assigned"). The one path that isn't is the rare race where a
// unique-constraint violation (concurrent inserts both trying to become the
// first/default row) surfaces the raw SQLite driver error. Catch that shape
// specifically instead of passing every backend error straight through to
// the UI, so a request that reaches the network fails with a message the
// user can act on either way.
export function friendlyCatalogErrorMessage(e: unknown, fallback: string): string {
  const message = e instanceof Error ? e.message : ''
  if (/sqlite|constraint|no such table|UNIQUE/i.test(message)) {
    return fallback
  }
  return message || fallback
}
